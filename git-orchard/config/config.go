// Package config reads the subtree manifest.
//
// The manifest is a committed file at the repository root, either
// .gitsubtrees or .config/git-orchard/subtrees, in gitconfig syntax like
// .gitmodules:
//
//	[orchard]
//		squash = true
//	[subtree "pre-commit-hooks/go-pre-commit-hooks"]
//		remote = git@github.com:jmelahman/go-pre-commit-hooks.git
//		branch = master
//		shared = base
//		shared = go
//
// Each shared value names a profile, a directory under orchard.sharedDir
// (.config/git-orchard/shared by default) whose files sync copies into the
// subtree.
//
// The same keys in git's own configuration (e.g. .git/config) override it, so
// a clone can point a subtree somewhere else without touching the manifest.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/jmelahman/git-orchard/git"
)

// Manifests are the paths the manifest may have, relative to the repository
// root. A repository without one gets the first.
var Manifests = []string{".gitsubtrees", ".config/git-orchard/subtrees"}

// DefaultBranch is the upstream branch of a subtree that doesn't name one.
const DefaultBranch = "master"

// DefaultSharedDir is where profiles live when orchard.sharedDir isn't set,
// relative to the repository root.
const DefaultSharedDir = ".config/git-orchard/shared"

// Subtree is one subtree and the upstream it mirrors.
type Subtree struct {
	Prefix string
	Remote string
	Branch string
	// Shared names the profiles synced into the subtree, in manifest order.
	Shared []string
}

// Config is the parsed manifest.
type Config struct {
	// Manifest is the manifest's path relative to the repository root,
	// whether or not it exists yet.
	Manifest string
	// Squash makes pull and add squash upstream history into one commit.
	Squash bool
	// SharedDir is the directory holding the profiles subtrees share,
	// relative to the repository root.
	SharedDir string
	Subtrees  []Subtree
}

// Lookup returns the subtree at prefix.
func (c Config) Lookup(prefix string) (Subtree, bool) {
	prefix = Clean(prefix)
	for _, s := range c.Subtrees {
		if s.Prefix == prefix {
			return s, true
		}
	}
	return Subtree{}, false
}

// Select returns the subtrees at prefixes, or every subtree if there are none.
func (c Config) Select(prefixes []string) ([]Subtree, error) {
	if len(prefixes) == 0 {
		return c.Subtrees, nil
	}
	selected := make([]Subtree, 0, len(prefixes))
	for _, p := range prefixes {
		s, ok := c.Lookup(p)
		if !ok {
			return nil, fmt.Errorf("%s is not a subtree listed in %s", p, c.Manifest)
		}
		selected = append(selected, s)
	}
	return selected, nil
}

// Clean normalizes a prefix as typed on the command line ("./tag/" → "tag").
func Clean(prefix string) string {
	return filepath.ToSlash(filepath.Clean(prefix))
}

// FindManifest returns the path of the manifest in the repository at root,
// relative to it, or the default path if there is none. Having more than one
// is an error, since it would be ambiguous which one to edit.
func FindManifest(root string) (path string, exists bool, err error) {
	var found []string
	for _, m := range Manifests {
		if _, err := os.Stat(filepath.Join(root, m)); err == nil {
			found = append(found, m)
		} else if !os.IsNotExist(err) {
			return "", false, err
		}
	}
	switch len(found) {
	case 0:
		return Manifests[0], false, nil
	case 1:
		return found[0], true, nil
	}
	return "", false, fmt.Errorf("found both %s; keep one", strings.Join(found, " and "))
}

// Load reads the manifest of repo, with git's own configuration layered on
// top. A missing manifest is an empty one.
func Load(repo git.Repo) (Config, error) {
	b := newBuilder()

	manifest, exists, err := FindManifest(repo.Dir)
	if err != nil {
		return Config{}, err
	}
	if exists {
		out, err := repo.Output("config", "--file", filepath.Join(repo.Dir, manifest), "--null", "--list")
		if err != nil {
			return Config{}, err
		}
		if err := b.apply(out); err != nil {
			return Config{}, fmt.Errorf("%s: %w", manifest, err)
		}
	}

	// Exit status 1 means no key matched.
	out, err := repo.Output("config", "--null", "--get-regexp", `^(orchard|subtree)\.`)
	if err != nil && git.ExitCode(err) != 1 {
		return Config{}, err
	}
	if err := b.apply(out); err != nil {
		return Config{}, fmt.Errorf("git config: %w", err)
	}

	c, err := b.build()
	c.Manifest = manifest
	return c, err
}

// Add records s in the manifest of repo at the path manifest, relative to the
// repository root.
func Add(repo git.Repo, manifest string, s Subtree) error {
	path := filepath.Join(repo.Dir, manifest)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	section := "subtree." + s.Prefix
	for _, kv := range [][2]string{{"remote", s.Remote}, {"branch", s.Branch}} {
		if kv[1] == "" {
			continue
		}
		if _, err := repo.Output("config", "--file", path, section+"."+kv[0], kv[1]); err != nil {
			return err
		}
	}
	return nil
}

type builder struct {
	squash    bool
	sharedDir string
	subtrees  map[string]*Subtree
}

func newBuilder() *builder {
	return &builder{squash: true, sharedDir: DefaultSharedDir, subtrees: map[string]*Subtree{}}
}

// apply merges `git config --null` output ("key\nvalue\0" per entry, or
// "key\0" for a key with no value) into b.
func (b *builder) apply(entries string) error {
	for _, entry := range strings.Split(entries, "\x00") {
		if entry == "" {
			continue
		}
		key, value, hasValue := strings.Cut(entry, "\n")
		// Section and variable names are case-insensitive (git lowercases
		// them); the subsection, here the prefix, is not.
		first := strings.Index(key, ".")
		last := strings.LastIndex(key, ".")
		section, name := key[:first], key[last+1:]

		switch section {
		case "orchard":
			switch name {
			case "squash":
				squash, err := parseBool(value, hasValue)
				if err != nil {
					return fmt.Errorf("%s: %w", key, err)
				}
				b.squash = squash
			case "shareddir":
				if value == "" {
					return fmt.Errorf("%s: needs a directory", key)
				}
				b.sharedDir = Clean(value)
			}
		case "subtree":
			if first == last {
				return fmt.Errorf("%s: subtree sections need a prefix, like [subtree \"path/to/dir\"]", key)
			}
			prefix := Clean(key[first+1 : last])
			s, ok := b.subtrees[prefix]
			if !ok {
				s = &Subtree{Prefix: prefix}
				b.subtrees[prefix] = s
			}
			switch name {
			case "remote":
				s.Remote = value
			case "branch":
				s.Branch = value
			case "shared":
				// Multi-valued, so a later layer adds profiles rather than
				// replacing them.
				if value == "" {
					return fmt.Errorf("%s: needs a profile name", key)
				}
				if !slices.Contains(s.Shared, value) {
					s.Shared = append(s.Shared, value)
				}
			}
		}
	}
	return nil
}

func (b *builder) build() (Config, error) {
	c := Config{Squash: b.squash, SharedDir: b.sharedDir}
	for _, s := range b.subtrees {
		if s.Remote == "" {
			return Config{}, fmt.Errorf("subtree %q has no remote", s.Prefix)
		}
		if s.Branch == "" {
			s.Branch = DefaultBranch
		}
		c.Subtrees = append(c.Subtrees, *s)
	}
	sort.Slice(c.Subtrees, func(i, j int) bool { return c.Subtrees[i].Prefix < c.Subtrees[j].Prefix })
	return c, nil
}

// parseBool parses a git boolean; a bare key ("squash" alone on a line) is
// true.
func parseBool(value string, hasValue bool) (bool, error) {
	if !hasValue {
		return true, nil
	}
	switch strings.ToLower(value) {
	case "yes", "on", "true", "1":
		return true, nil
	case "no", "off", "false", "0", "":
		return false, nil
	}
	if n, err := strconv.Atoi(value); err == nil {
		return n != 0, nil
	}
	return false, fmt.Errorf("invalid boolean %q", value)
}
