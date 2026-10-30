// Package project loads a project's vinder.toml and generates, syncs and
// checks the vendor directory it describes.
package project

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

// ConfigFile is the name of the file that marks a project vinder manages.
const ConfigFile = "vinder.toml"

// ManifestFile is the name of the manifest vinder writes at the root of the
// output directory.
const ManifestFile = "vinder.json"

// defaultOut is the output directory when a vinder.toml sets none.
const defaultOut = "vendor"

var (
	// ErrNoConfig is returned by Load for a directory without a vinder.toml.
	ErrNoConfig = errors.New("no " + ConfigFile)
	// ErrBadConfig is returned by Load for a vinder.toml that does not
	// describe a valid vendor directory.
	ErrBadConfig = errors.New("invalid " + ConfigFile)
)

// Dep is one vendored dependency: the files of the directory
// <out>/<key>/ and where they come from.
type Dep struct {
	// Bundle maps an output file to the entry esbuild bundles it from.
	Bundle map[string]string `toml:"bundle"`
	// Copy maps an output file to the file it is copied from.
	Copy map[string]string `toml:"copy"`
	// Ignore maps an advisory ID, or a name@version to ignore every advisory
	// of, to the reason the audit should not report it.
	Ignore map[string]string `toml:"ignore"`
	// Package is the npm package being vendored.
	Package string `toml:"package"`
	// Embeds lists name@version pairs a package inlines in its own files,
	// which the lockfile cannot see but the audit should.
	Embeds []string `toml:"embeds"`
}

// Config is the contents of a vinder.toml.
type Config struct {
	Deps map[string]Dep `toml:"deps"`
	// Out is the output directory, relative to the project; "vendor" when
	// unset.
	Out string `toml:"out"`
}

// Project is a directory holding a package.json and a vinder.toml.
type Project struct {
	// requires holds, after a Generate, the version ranges asked of each
	// vendored package by name: by the project's package.json and its own
	// overrides, and by every vendored package that depends on it.
	requires map[string][]string
	// pm is the package manager, once detected. It is kept because what it
	// was detected from can go: upgrade deletes the lockfile.
	pm     *PackageManager
	Config Config
	Dir    string
	// Cooldown is how old a release must be for Sync to resolve to it. Zero
	// allows any.
	Cooldown time.Duration
	// synced is set once Sync has run, leaving the vendor tree on disk and
	// requires current.
	synced bool
}

// Load reads and validates the vinder.toml in dir.
func Load(dir string) (*Project, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(dir, ConfigFile))
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("%s: %w", dir, ErrNoConfig)
	}
	if err != nil {
		return nil, err
	}
	var cfg Config
	md, err := toml.Decode(string(data), &cfg)
	if err != nil {
		return nil, fmt.Errorf("%s: %w: %w", filepath.Join(dir, ConfigFile), ErrBadConfig, err)
	}
	if undecoded := md.Undecoded(); len(undecoded) > 0 {
		return nil, fmt.Errorf("%s: %w: unknown key %q", filepath.Join(dir, ConfigFile), ErrBadConfig, undecoded[0].String())
	}
	if cfg.Out == "" {
		cfg.Out = defaultOut
	}
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("%s: %w: %w", filepath.Join(dir, ConfigFile), ErrBadConfig, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "package.json")); err != nil {
		return nil, fmt.Errorf("%s: %w: package.json: %w", dir, ErrBadConfig, err)
	}
	return &Project{Dir: dir, Config: cfg}, nil
}

// OutDir is the absolute path of the project's output directory.
func (p *Project) OutDir() string {
	return filepath.Join(p.Dir, filepath.FromSlash(p.Config.Out))
}

// Keys returns the configured dependency keys, sorted.
func (p *Project) Keys() []string {
	keys := make([]string, 0, len(p.Config.Deps))
	for k := range p.Config.Deps {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// validKey reports whether key names a directory directly under the output
// directory, which is all vinder may create or remove there.
func validKey(key string) bool {
	return key != "" && key != "." && key != ".." && !strings.ContainsAny(key, `/\`) && key != ManifestFile
}

func (c *Config) validate() error {
	// vinder replaces <out>/<key>/ wholesale, so out must be a directory
	// of its own rather than the project root.
	if !filepath.IsLocal(filepath.FromSlash(c.Out)) || path.Clean(c.Out) == "." {
		return fmt.Errorf("out %q must be a directory inside the project", c.Out)
	}
	if len(c.Deps) == 0 {
		return errors.New("no [deps] entries")
	}
	for key, dep := range c.Deps {
		if !validKey(key) {
			return fmt.Errorf("deps.%s: key must be a plain directory name", key)
		}
		if dep.Package == "" {
			return fmt.Errorf("deps.%s: package is required", key)
		}
		if len(dep.Bundle)+len(dep.Copy) == 0 {
			return fmt.Errorf("deps.%s: needs at least one bundle or copy entry", key)
		}
		for out, entry := range dep.Bundle {
			if err := checkOutput(out, dep.Copy); err != nil {
				return fmt.Errorf("deps.%s.bundle: %w", key, err)
			}
			switch path.Ext(out) {
			case ".js", ".mjs", ".css":
			default:
				return fmt.Errorf("deps.%s.bundle: %q: output must end in .js, .mjs or .css", key, out)
			}
			if err := checkSource(entry); err != nil {
				return fmt.Errorf("deps.%s.bundle: %w", key, err)
			}
		}
		for out, src := range dep.Copy {
			if err := checkOutput(out, nil); err != nil {
				return fmt.Errorf("deps.%s.copy: %w", key, err)
			}
			if err := checkSource(src); err != nil {
				return fmt.Errorf("deps.%s.copy: %w", key, err)
			}
		}
		for k, reason := range dep.Ignore {
			if strings.TrimSpace(reason) == "" {
				return fmt.Errorf("deps.%s.ignore: %q needs a reason", key, k)
			}
		}
		for _, e := range dep.Embeds {
			if _, err := parseNameVersion(e); err != nil {
				return fmt.Errorf("deps.%s.embeds: %w", key, err)
			}
		}
	}
	return nil
}

func checkOutput(out string, others map[string]string) error {
	if !filepath.IsLocal(filepath.FromSlash(out)) {
		return fmt.Errorf("output %q must be a relative path inside the dependency's directory", out)
	}
	if _, dup := others[out]; dup {
		return fmt.Errorf("output %q is both bundled and copied", out)
	}
	return nil
}

func checkSource(src string) error {
	if !filepath.IsLocal(filepath.FromSlash(strings.TrimPrefix(src, "./"))) {
		return fmt.Errorf("source %q must be a path inside the project (./...) or node_modules", src)
	}
	return nil
}

// sourcePath resolves a bundle entry or copy source: "./x" is relative to
// the project and anything else to its node_modules.
func (p *Project) sourcePath(src string) string {
	if rest, ok := strings.CutPrefix(src, "./"); ok {
		return filepath.Join(p.Dir, filepath.FromSlash(rest))
	}
	return filepath.Join(p.Dir, "node_modules", filepath.FromSlash(src))
}

// bundleEntry is the entry point esbuild gets for a bundle source. A bare
// package name stays one, so that esbuild resolves it as an import would,
// through the package's exports; as a path, it would read only main and
// module, and a package with just exports would not resolve.
func (p *Project) bundleEntry(src string) string {
	if packageName.MatchString(src) {
		return src
	}
	return p.sourcePath(src)
}

// Ignored reports whether the audit should leave out advisory id for pkg,
// a package vendored as the dependency key.
func (p *Project) Ignored(key string, pkg Package, id string) bool {
	ignore := p.Config.Deps[key].Ignore
	_, all := ignore[pkg.Name+"@"+pkg.Version]
	_, one := ignore[id]
	return all || one
}

func parseNameVersion(s string) (Package, error) {
	// The first character may be the @ of a scope, so look past it.
	i := strings.LastIndex(s, "@")
	if i <= 0 || i == len(s)-1 {
		return Package{}, fmt.Errorf("%q must be name@version", s)
	}
	return Package{Name: s[:i], Version: s[i+1:]}, nil
}
