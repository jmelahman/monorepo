// Package share syncs the files of shared profiles into subtrees.
//
// A profile is a directory under the manifest's shared directory. Each file in
// it lands at the same relative path in every subtree that lists the profile.
// If that file in the subtree has a block marked for the profile,
//
//	# BEGIN orchard:go
//	...
//	# END orchard:go
//
// only the lines between the markers are replaced, and the rest of the file
// is the subtree's own. The marker lines are kept as they are, so they can use
// whatever comment syntax the file does. Without markers, the profile owns the
// whole file.
package share

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/jmelahman/git-orchard/config"
)

// Change is a file sync would write.
type Change struct {
	// Path is relative to the repository root, with forward slashes.
	Path string
	// Old is the current content, nil if the file doesn't exist.
	Old []byte
	New []byte
	// OldMode is the current permissions, 0 if the file doesn't exist.
	OldMode fs.FileMode
	// Mode is the permissions to write: the profile's for a file it owns
	// whole, the file's own when only blocks are shared.
	Mode fs.FileMode
}

// Plan returns the changes that would bring subtrees in the repository at
// root up to date with their profiles.
func Plan(root string, cfg config.Config, subtrees []config.Subtree) ([]Change, error) {
	var changes []Change
	for _, s := range subtrees {
		c, err := planSubtree(root, cfg.SharedDir, s)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", s.Prefix, err)
		}
		changes = append(changes, c...)
	}
	return changes, nil
}

// source is one profile's version of a file.
type source struct {
	profile string
	content []byte
	mode    fs.FileMode
}

func planSubtree(root, sharedDir string, s config.Subtree) ([]Change, error) {
	// Files in the order profiles first provide them, so output is stable.
	var paths []string
	sources := map[string][]source{}
	for _, profile := range s.Shared {
		dir := filepath.Join(root, sharedDir, profile)
		info, err := os.Stat(dir)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil, fmt.Errorf("profile %q: %s does not exist", profile, path.Join(sharedDir, profile))
			}
			return nil, err
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("profile %q: %s is not a directory", profile, path.Join(sharedDir, profile))
		}
		err = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			rel, err := filepath.Rel(dir, p)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			content, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			info, err := os.Stat(p)
			if err != nil {
				return err
			}
			if _, ok := sources[rel]; !ok {
				paths = append(paths, rel)
			}
			sources[rel] = append(sources[rel], source{profile: profile, content: content, mode: info.Mode().Perm()})
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	var changes []Change
	for _, rel := range paths {
		dest := path.Join(s.Prefix, rel)
		destPath := filepath.Join(root, filepath.FromSlash(dest))
		old, err := os.ReadFile(destPath)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
		exists := err == nil
		var oldMode fs.FileMode
		if exists {
			info, err := os.Stat(destPath)
			if err != nil {
				return nil, err
			}
			oldMode = info.Mode().Perm()
		}
		content, mode, err := render(old, oldMode, exists, sources[rel])
		if err != nil {
			return nil, fmt.Errorf("%s: %w", rel, err)
		}
		if exists && bytes.Equal(old, content) && oldMode == mode {
			continue
		}
		changes = append(changes, Change{Path: dest, Old: old, New: content, OldMode: oldMode, Mode: mode})
	}
	return changes, nil
}

// render returns the new content and permissions of a file from the profiles
// that provide it.
func render(old []byte, oldMode fs.FileMode, exists bool, srcs []source) ([]byte, fs.FileMode, error) {
	if !exists || !hasBlock(old, srcs[0].profile) {
		if len(srcs) > 1 {
			return nil, 0, fmt.Errorf("profiles %s all provide it; mark a block for each with BEGIN/END orchard:<profile>", profileNames(srcs))
		}
		return srcs[0].content, srcs[0].mode, nil
	}
	content := old
	for _, src := range srcs {
		var err error
		content, err = ReplaceBlock(content, src.profile, src.content)
		if err != nil {
			return nil, 0, err
		}
	}
	return content, oldMode, nil
}

func profileNames(srcs []source) string {
	names := make([]string, len(srcs))
	for i, s := range srcs {
		names[i] = s.profile
	}
	return strings.Join(names, ", ")
}

func hasBlock(content []byte, name string) bool {
	for _, line := range bytes.SplitAfter(content, []byte("\n")) {
		if marker(line, "BEGIN") == name {
			return true
		}
	}
	return false
}

// marker returns the name in a "<kind> orchard:<name>" marker on line, or ""
// if the line isn't one.
func marker(line []byte, kind string) string {
	token := kind + " orchard:"
	_, after, ok := strings.Cut(string(line), token)
	if !ok {
		return ""
	}
	name, _, _ := strings.Cut(after, " ")
	return strings.TrimRight(name, " \t\r\n")
}

// ReplaceBlock replaces the lines between the BEGIN and END markers of the
// block name in content with block.
func ReplaceBlock(content []byte, name string, block []byte) ([]byte, error) {
	lines := bytes.SplitAfter(content, []byte("\n"))
	begin, end := -1, -1
	for i, line := range lines {
		switch name {
		case marker(line, "BEGIN"):
			if begin >= 0 {
				return nil, fmt.Errorf("duplicate BEGIN orchard:%s on line %d", name, i+1)
			}
			begin = i
		case marker(line, "END"):
			if begin < 0 {
				return nil, fmt.Errorf("END orchard:%s on line %d has no BEGIN", name, i+1)
			}
			if end >= 0 {
				return nil, fmt.Errorf("duplicate END orchard:%s on line %d", name, i+1)
			}
			end = i
		}
	}
	if begin < 0 {
		return nil, fmt.Errorf("no BEGIN orchard:%s marker", name)
	}
	if end < 0 {
		return nil, fmt.Errorf("BEGIN orchard:%s on line %d has no END", name, begin+1)
	}

	var out bytes.Buffer
	for _, line := range lines[:begin+1] {
		out.Write(line)
	}
	if !bytes.HasSuffix(lines[begin], []byte("\n")) {
		out.WriteByte('\n')
	}
	out.Write(block)
	if len(block) > 0 && !bytes.HasSuffix(block, []byte("\n")) {
		out.WriteByte('\n')
	}
	for _, line := range lines[end:] {
		out.Write(line)
	}
	return out.Bytes(), nil
}

// Apply writes changes into the repository at root.
func Apply(root string, changes []Change) error {
	for _, c := range changes {
		dest := filepath.Join(root, filepath.FromSlash(c.Path))
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(dest, c.New, c.Mode); err != nil {
			return err
		}
		// WriteFile only applies the mode to a file it creates.
		if err := os.Chmod(dest, c.Mode); err != nil {
			return err
		}
	}
	return nil
}
