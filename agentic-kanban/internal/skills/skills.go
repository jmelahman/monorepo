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
		existing, err := os.ReadFile(dest)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("read installed skill: %w", err)
		}
		unchanged := err == nil && bytes.Equal(existing, content)
		if err == nil && !unchanged && !force {
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
			if err := os.MkdirAll(filepath.Dir(r.Path), 0o755); err != nil {
				return nil, fmt.Errorf("create skill directory: %w", err)
			}
			if err := os.WriteFile(r.Path, contents[i], 0o644); err != nil {
				return nil, fmt.Errorf("write skill: %w", err)
			}
		}
		if r.Link != "" {
			if err := replaceLink(r.Link, filepath.Dir(r.Path)); err != nil {
				return nil, err
			}
		}
	}
	return results, nil
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

// replaceLink points link at target. linkNeeded has already established that
// anything at link is a symlink.
func replaceLink(link, target string) error {
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		return fmt.Errorf("create skill link directory: %w", err)
	}
	if err := os.Remove(link); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("replace skill link: %w", err)
	}
	if err := os.Symlink(target, link); err != nil {
		return fmt.Errorf("link skill: %w", err)
	}
	return nil
}
