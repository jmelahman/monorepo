// Package skills bundles the agent skills that ship with kanban and installs
// them where coding agents discover them. Each skill is a <name>/SKILL.md
// following the Agent Skills format (https://agentskills.io).
package skills

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
)

// .dockerignore and pyproject.toml's sdist list both have to let these through,
// or the image and wheel builds fail on this embed.
//
//go:embed */SKILL.md
var bundled embed.FS

// Result reports what Install did with one skill.
type Result struct {
	Name string
	Path string
	// Unchanged is true when the installed file already matched.
	Unchanged bool
	// Link is the symlink Install created or replaced, if any.
	Link string
}

// Install writes every bundled skill to <skillsDir>/<name>/SKILL.md and, when
// linkDir is set, links <linkDir>/<name> to the skill's directory. A file that
// differs from the bundled copy may hold the user's edits, so it's only
// replaced with force. Conflicts are found before anything is written, so a
// refused install changes nothing.
//
// skillsDir and linkDir are the caller's to choose, but what's below them isn't
// trusted: session containers can write to ~/.claude, so a symlink there may
// have been planted to redirect a write on the host. Install never writes
// through one, even with force.
func Install(skillsDir, linkDir string, force bool) ([]Result, error) {
	if samePath(skillsDir, linkDir) {
		linkDir = ""
	}
	sources, err := fs.Glob(bundled, "*/SKILL.md")
	if err != nil {
		return nil, err
	}
	results := make([]Result, 0, len(sources))
	contents := make([][]byte, 0, len(sources))
	for _, src := range sources {
		content, err := bundled.ReadFile(src)
		if err != nil {
			return nil, err
		}
		name := path.Dir(src)
		dest := filepath.Join(skillsDir, name, "SKILL.md")
		existing, installed, err := readInstalled(dest)
		if err != nil {
			return nil, err
		}
		unchanged := installed && bytes.Equal(existing, content)
		if installed && !unchanged && !force {
			return nil, fmt.Errorf("%s already exists and differs from this version of kanban's; pass --force to replace it", dest)
		}
		result := Result{Name: name, Path: dest, Unchanged: unchanged}
		if linkDir != "" {
			link := filepath.Join(linkDir, name)
			needed, err := linkNeeded(link, filepath.Dir(dest), force)
			if err != nil {
				return nil, err
			}
			if needed {
				result.Link = link
			}
		}
		results = append(results, result)
		contents = append(contents, content)
	}
	for i, r := range results {
		if !r.Unchanged {
			if err := writeSkill(skillsDir, r.Name, contents[i]); err != nil {
				return nil, fmt.Errorf("write skill: %w", err)
			}
		}
		if r.Link != "" {
			if err := replaceLink(linkDir, r.Name, filepath.Dir(r.Path)); err != nil {
				return nil, fmt.Errorf("link skill: %w", err)
			}
		}
	}
	return results, nil
}

// readInstalled returns the skill installed at dest, if any. Anything other
// than a real directory holding a regular file is refused rather than followed.
func readInstalled(dest string) (content []byte, installed bool, err error) {
	dir := filepath.Dir(dest)
	info, err := os.Lstat(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("read installed skill: %w", err)
	}
	if !info.IsDir() {
		return nil, false, fmt.Errorf("%s isn't a real directory, so kanban won't write through it; remove it, or pass --dir its real location", dir)
	}
	info, err = os.Lstat(dest)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("read installed skill: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, false, fmt.Errorf("%s isn't a regular file, so kanban won't write through it; remove it, or pass --dir its real location", dest)
	}
	content, err = os.ReadFile(dest)
	if err != nil {
		return nil, false, fmt.Errorf("read installed skill: %w", err)
	}
	return content, true, nil
}

// writeSkill atomically replaces <skillsDir>/<name>/SKILL.md. Going through
// os.Root means a symlink swapped in after readInstalled's checks still can't
// send the write outside skillsDir.
func writeSkill(skillsDir, name string, content []byte) (err error) {
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		return err
	}
	root, err := os.OpenRoot(skillsDir)
	if err != nil {
		return err
	}
	defer root.Close()
	if err := root.MkdirAll(name, 0o755); err != nil {
		return err
	}
	tmp := filepath.Join(name, ".SKILL.md.tmp")
	if err := root.Remove(tmp); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	f, err := root.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = root.Remove(tmp)
		}
	}()
	if _, err := f.Write(content); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return root.Rename(tmp, filepath.Join(name, "SKILL.md"))
}

// samePath reports whether two directories are the same, following symlinks,
// so a skills directory that's already linked to linkDir doesn't get a link
// to itself.
func samePath(a, b string) bool {
	if a == b {
		return true
	}
	realA, errA := filepath.EvalSymlinks(a)
	realB, errB := filepath.EvalSymlinks(b)
	return errA == nil && errB == nil && realA == realB
}

// linkNeeded reports whether link has to be created or replaced to reach
// target. force only replaces another symlink: a real directory could be
// anything the user keeps there, so removing it is left to them.
func linkNeeded(link, target string, force bool) (bool, error) {
	info, err := os.Lstat(link)
	if errors.Is(err, fs.ErrNotExist) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("read skill link: %w", err)
	}
	if samePath(link, target) {
		return false, nil
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return false, fmt.Errorf("%s already exists and isn't a symlink; remove it to link it to %s", link, target)
	}
	if existing, err := os.Readlink(link); err == nil && existing == target {
		return false, nil
	}
	if !force {
		return false, fmt.Errorf("%s already links somewhere else; pass --force to link it to %s", link, target)
	}
	return true, nil
}

// replaceLink atomically points <linkDir>/<name> at target. Renaming over the
// old link replaces the link itself, never what it points to, and fails on a
// directory swapped in after linkNeeded's checks.
func replaceLink(linkDir, name, target string) error {
	if err := os.MkdirAll(linkDir, 0o755); err != nil {
		return err
	}
	root, err := os.OpenRoot(linkDir)
	if err != nil {
		return err
	}
	defer root.Close()
	tmp := "." + name + ".tmp"
	if err := root.Remove(tmp); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err := root.Symlink(target, tmp); err != nil {
		return err
	}
	if err := root.Rename(tmp, name); err != nil {
		_ = root.Remove(tmp)
		return err
	}
	return nil
}
