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
}

// Install writes every bundled skill to <skillsDir>/<name>/SKILL.md. A file
// that differs from the bundled copy may hold the user's edits, so it's only
// replaced with force. Conflicts are found before anything is written, so a
// refused install changes nothing.
func Install(skillsDir string, force bool) ([]Result, error) {
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
		results = append(results, Result{Name: name, Path: dest, Unchanged: unchanged})
		contents = append(contents, content)
	}
	for i, r := range results {
		if r.Unchanged {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(r.Path), 0o755); err != nil {
			return nil, fmt.Errorf("create skill directory: %w", err)
		}
		if err := os.WriteFile(r.Path, contents[i], 0o644); err != nil {
			return nil, fmt.Errorf("write skill: %w", err)
		}
	}
	return results, nil
}
