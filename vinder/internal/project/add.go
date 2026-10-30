package project

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/jmelahman/vinder/internal/pkgjson"
)

var (
	// ErrBadPackageSpec is returned by Add for an argument that is not
	// name or name@version.
	ErrBadPackageSpec = errors.New("not a package: want name or name@version")
	// ErrAlreadyVendored is returned by Add for a package vinder.toml already
	// has, or one whose key it already uses for another.
	ErrAlreadyVendored = errors.New("already vendored")
	// ErrUnmanagedOutput is returned by Add for a package whose directory
	// under the output directory exists and is not one vinder wrote, which
	// the sync would replace.
	ErrUnmanagedOutput = errors.New("exists and vinder did not write it; move it out of the way first")
)

// namePattern is an npm package name, in characters vinder.toml and paths can
// safely hold.
const namePattern = `(?:@[A-Za-z0-9][A-Za-z0-9._~-]*/)?[A-Za-z0-9][A-Za-z0-9._~-]*`

var (
	packageName = regexp.MustCompile(`^` + namePattern + `$`)
	// packageSpec is a package name and an optional version.
	packageSpec = regexp.MustCompile(`^(` + namePattern + `)(?:@(.+))?$`)
)

// resolvedSpec matches the specs Add turns into a version, as npm does: a
// dist-tag, or a range that only names part of a version. Other ranges, exact
// versions and specs like npm: and file: say what the person wants as written.
var resolvedSpec = regexp.MustCompile(`^(?:[A-Za-z*][A-Za-z0-9._-]*|\d+(?:\.(?:\d+|[xX*]))?(?:\.[xX*])?)$`)

// keyReplacer turns a package name into a bare TOML key and a directory
// name: a dot would otherwise nest the table.
var keyReplacer = strings.NewReplacer("@", "", "/", "-", ".", "-", "~", "-")

// Addition is a package Add vendored.
type Addition struct {
	// Files are the files written, as slash-separated paths relative to the
	// project.
	Files   []string
	Name    string
	Version string
	Key     string
}

// Add vendors more packages in the project in dir, creating package.json and
// vinder.toml if it has neither. Each spec is a package, whose bundled ES
// module becomes <out>/<key>/<key>.mjs, and optionally the version to depend
// on. Without one, a dependency package.json already has stays as it is and
// a new one is resolved as latest. A dist-tag or a partial version goes in at
// ^ the version resolve returns for it, so that upgrade can move it; anything
// else goes in as written.
//
// A project with no package manager yet gets the first one on PATH, recorded
// in package.json. If anything fails, package.json, the lockfile and
// vinder.toml are put back as they were.
func Add(ctx context.Context, dir string, specs []string, resolve func(ctx context.Context, name, spec string) (string, error)) (_ *Project, _ []Addition, err error) {
	dir, err = filepath.Abs(dir)
	if err != nil {
		return nil, nil, err
	}
	configFile := filepath.Join(dir, ConfigFile)
	configData, err := os.ReadFile(configFile)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, nil, err
	}
	var cfg Config
	if _, err := toml.Decode(string(configData), &cfg); err != nil {
		return nil, nil, fmt.Errorf("%s: %w: %w", configFile, ErrBadConfig, err)
	}

	adds, err := parseAdds(specs, cfg)
	if err != nil {
		return nil, nil, err
	}
	if err := checkOutputs(dir, cfg, adds); err != nil {
		return nil, nil, err
	}

	// The lockfile isn't known until the package manager is, and every one
	// it could be has to go back to not existing.
	saved := []string{configFile, filepath.Join(dir, "package.json")}
	for _, pm := range managers {
		for _, name := range pm.Lockfiles {
			saved = append(saved, filepath.Join(dir, name))
		}
	}
	restore, err := snapshot(saved)
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		if err == nil {
			return
		}
		if rerr := restore(); rerr != nil {
			err = errors.Join(err, fmt.Errorf("restoring package.json, the lockfile and %s: %w", ConfigFile, rerr))
		}
	}()

	pj, err := readOrCreatePackageJSON(filepath.Join(dir, "package.json"))
	if err != nil {
		return nil, nil, err
	}
	if err := declarePackageManager(ctx, dir, pj); err != nil {
		return nil, nil, err
	}

	config := configData
	if len(config) > 0 && config[len(config)-1] != '\n' {
		config = append(config, '\n')
	}
	for _, a := range adds {
		if err := addDependency(ctx, pj, a, resolve); err != nil {
			return nil, nil, err
		}

		if len(config) > 0 {
			config = append(config, '\n')
		}
		config = fmt.Appendf(config, "[deps.%s]\npackage = %q\n\n[deps.%s.bundle]\n%q = %q\n", a.key, a.name, a.key, a.key+".mjs", a.name)
	}
	if err := pj.Write(); err != nil {
		return nil, nil, err
	}
	if err := os.WriteFile(configFile, config, 0o644); err != nil {
		return nil, nil, err
	}

	p, err := Load(dir)
	if err != nil {
		return nil, nil, err
	}
	if err := p.Sync(ctx); errors.Is(err, errSwap) {
		return nil, nil, fmt.Errorf("%w\npackage.json, the lockfile and %s were put back, but %s may be partly changed", err, ConfigFile, p.Config.Out)
	} else if err != nil {
		return nil, nil, fmt.Errorf("%w\nnothing was added; a package with no JavaScript entry point, such as a stylesheet, needs its %s entry written by hand", err, ConfigFile)
	}
	m, err := ReadManifest(filepath.Join(p.OutDir(), ManifestFile))
	if err != nil {
		return nil, nil, err
	}
	additions := make([]Addition, len(adds))
	for i, a := range adds {
		entry := m.Deps[a.key]
		files := make([]string, len(entry.Files))
		for j, f := range entry.Files {
			files[j] = p.Config.Out + "/" + a.key + "/" + f
		}
		additions[i] = Addition{Name: a.name, Version: entry.Version, Key: a.key, Files: files}
	}
	return p, additions, nil
}

// wanted is a package Add was asked for.
type wanted struct{ name, version, key string }

// parseAdds parses Add's specs, and refuses what cfg already has or what
// would collide among them.
func parseAdds(specs []string, cfg Config) ([]wanted, error) {
	adds := make([]wanted, 0, len(specs))
	for _, spec := range specs {
		m := packageSpec.FindStringSubmatch(spec)
		if m == nil {
			return nil, fmt.Errorf("%q: %w", spec, ErrBadPackageSpec)
		}
		key := keyReplacer.Replace(m[1])
		if dep, ok := cfg.Deps[key]; ok {
			return nil, fmt.Errorf("%s: %w: deps.%s is %s", m[1], ErrAlreadyVendored, key, dep.Package)
		}
		for other, dep := range cfg.Deps {
			if dep.Package == m[1] {
				return nil, fmt.Errorf("%s: %w as deps.%s", m[1], ErrAlreadyVendored, other)
			}
		}
		for _, a := range adds {
			if a.key == key {
				return nil, fmt.Errorf("%s: %w: %s was also given, as deps.%s", m[1], ErrAlreadyVendored, a.name, key)
			}
		}
		adds = append(adds, wanted{m[1], m[2], key})
	}
	return adds, nil
}

// checkOutputs refuses a package whose directory under the output directory
// exists, unless the manifest says vinder wrote it.
func checkOutputs(dir string, cfg Config, adds []wanted) error {
	out := cfg.Out
	if out == "" {
		out = defaultOut
	}
	outDir := filepath.Join(dir, filepath.FromSlash(out))
	var owned map[string]Entry
	if m, err := ReadManifest(filepath.Join(outDir, ManifestFile)); err == nil {
		owned = m.Deps
	}
	for _, a := range adds {
		if _, ok := owned[a.key]; !ok && exists(filepath.Join(outDir, a.key)) {
			return fmt.Errorf("%s: %s/%s %w", a.name, out, a.key, ErrUnmanagedOutput)
		}
	}
	return nil
}

// addDependency puts a's dependency in pj. See Add for the version it gets.
func addDependency(ctx context.Context, pj *pkgjson.File, a wanted, resolve func(ctx context.Context, name, spec string) (string, error)) error {
	field := "dependencies"
	for _, f := range dependencyFields {
		if _, ok := pj.String(f, a.name); ok {
			if a.version == "" {
				return nil
			}
			field = f
			break
		}
	}
	spec := a.version
	if spec == "" {
		spec = "latest"
	}
	if resolvedSpec.MatchString(spec) {
		version, err := resolve(ctx, a.name, spec)
		if err != nil {
			return err
		}
		spec = "^" + version
	}
	return pj.SetString(spec, field, a.name)
}

// declarePackageManager sets pj's packageManager to the first one on PATH,
// if nothing in dir says which the project uses.
func declarePackageManager(ctx context.Context, dir string, pj *pkgjson.File) error {
	_, err := DetectPackageManager(dir)
	if !errors.Is(err, ErrNoPackageManager) {
		return err
	}
	declared, err := firstPackageManager(ctx, dir)
	if err != nil {
		return err
	}
	return pj.SetString(declared, "packageManager")
}

// snapshot reads files, any of which may not exist, and returns a function
// that puts each back as it was, removing those that did not exist.
func snapshot(files []string) (func() error, error) {
	data := make(map[string][]byte, len(files))
	for _, file := range files {
		d, err := os.ReadFile(file)
		switch {
		case errors.Is(err, os.ErrNotExist):
		case err != nil:
			return nil, err
		default:
			data[file] = d
		}
	}
	return func() error {
		var errs []error
		for _, file := range files {
			if d, ok := data[file]; ok {
				errs = append(errs, os.WriteFile(file, d, 0o644))
			} else if err := os.Remove(file); err != nil && !errors.Is(err, os.ErrNotExist) {
				errs = append(errs, err)
			}
		}
		return errors.Join(errs...)
	}, nil
}

func readOrCreatePackageJSON(file string) (*pkgjson.File, error) {
	if !exists(file) {
		if err := os.WriteFile(file, []byte("{\n  \"private\": true\n}\n"), 0o644); err != nil {
			return nil, err
		}
	}
	return pkgjson.Read(file)
}

// firstPackageManager returns the first supported package manager on PATH
// as name@version, how package.json declares one: Yarn 1 refuses a bare
// name. It asks from dir, since a version manager's pin can depend on where
// it is asked.
func firstPackageManager(ctx context.Context, dir string) (string, error) {
	names := make([]string, len(managers))
	for i, pm := range managers {
		names[i] = pm.Name
		bin, err := exec.LookPath(pm.Name)
		if err != nil {
			continue
		}
		cmd := exec.CommandContext(ctx, bin, "--version")
		cmd.Dir = dir
		out, err := cmd.Output()
		if err != nil {
			return "", fmt.Errorf("%s --version: %w", pm.Name, err)
		}
		return pm.Name + "@" + strings.TrimPrefix(strings.TrimSpace(string(out)), "v"), nil
	}
	return "", fmt.Errorf("%w: install one of %s", ErrNoPackageManagerBinary, strings.Join(names, ", "))
}
