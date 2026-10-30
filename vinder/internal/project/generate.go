package project

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/evanw/esbuild/pkg/api"
)

// Package is an npm package at one version.
type Package struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// Entry is what the manifest records about one vendored dependency.
type Entry struct {
	Package string `json:"package"`
	Version string `json:"version"`
	// Files are the files vinder wrote to the dependency's directory.
	Files []string `json:"files"`
	// Includes are the packages whose code is in those files: the package
	// itself, every package a bundle pulled in, and the declared embeds.
	Includes []Package `json:"includes"`
}

// Manifest is the contents of <out>/vinder.json.
type Manifest struct {
	// Deps is keyed like [deps].
	Deps map[string]Entry `json:"deps"`
	// Aligned are the overrides vinder added to package.json to keep this
	// project on the same versions as the others, by package name. upgrade
	// drops and recomputes them; overrides not listed here are the
	// project's own, and are left alone.
	Aligned map[string]string `json:"aligned,omitempty"`
}

// ErrBadManifest is returned by ReadManifest for a vinder.json that does not
// parse, such as one left with merge conflict markers. Sync and upgrade
// regenerate it; everything else fails on it.
var ErrBadManifest = errors.New("invalid " + ManifestFile + "; run vinder sync to regenerate it")

// ReadManifest reads a vinder.json.
func ReadManifest(file string) (*Manifest, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("%s: %w: %w", file, ErrBadManifest, err)
	}
	return &m, nil
}

// Generate installs the project's dependencies with its package manager and
// writes the vendor tree it describes, manifest included, to dest. With
// frozen set the install fails rather than change the lockfile.
// node_modules is removed afterwards unless it was already there.
func (p *Project) Generate(ctx context.Context, dest string, frozen bool) (*Manifest, error) {
	pm, err := p.packageManager()
	if err != nil {
		return nil, err
	}
	aligned, err := p.aligned(pm)
	if errors.Is(err, ErrBadManifest) && !frozen {
		// The manifest being regenerated is what records the alignment;
		// without it, the aligned overrides count as the project's own.
		aligned, err = nil, nil
	}
	if err != nil {
		return nil, err
	}
	nodeModules := filepath.Join(p.Dir, "node_modules")
	if _, err := os.Stat(nodeModules); errors.Is(err, os.ErrNotExist) {
		defer os.RemoveAll(nodeModules)
	}
	if err := pm.install(ctx, p.Dir, frozen); err != nil {
		return nil, err
	}

	p.requires, err = p.rootRequires(pm, aligned)
	if err != nil {
		return nil, err
	}
	owners := map[string]*installed{}
	m := &Manifest{Deps: map[string]Entry{}, Aligned: aligned}
	for _, key := range p.Keys() {
		dep := p.Config.Deps[key]
		depDir := filepath.Join(dest, key)
		top, err := readPackage(filepath.Join(p.Dir, "node_modules", filepath.FromSlash(dep.Package)))
		if err != nil {
			return nil, fmt.Errorf("deps.%s: %w", key, err)
		}
		if top == nil {
			return nil, fmt.Errorf("deps.%s: package %s is not installed; is it in package.json?", key, dep.Package)
		}
		p.addRequires(top)
		includes := map[Package]bool{top.Package: true}
		var files []string

		for _, out := range sortedKeys(dep.Bundle) {
			outputs, inputs, err := p.bundle(p.bundleEntry(dep.Bundle[out]), filepath.Join(depDir, filepath.FromSlash(out)))
			if err != nil {
				return nil, fmt.Errorf("deps.%s.bundle %s: %w", key, out, err)
			}
			for _, in := range inputs {
				owner, err := p.owner(filepath.Dir(in), owners)
				if err != nil {
					return nil, err
				}
				if owner != nil && !includes[owner.Package] {
					p.addRequires(owner)
					includes[owner.Package] = true
				}
			}
			for _, o := range outputs {
				rel, err := filepath.Rel(depDir, o)
				if err != nil {
					return nil, err
				}
				files = append(files, filepath.ToSlash(rel))
			}
		}

		for _, out := range sortedKeys(dep.Copy) {
			data, err := os.ReadFile(p.sourcePath(dep.Copy[out]))
			if err != nil {
				return nil, fmt.Errorf("deps.%s.copy %s: %w", key, out, err)
			}
			outFile := filepath.Join(depDir, filepath.FromSlash(out))
			if err := os.MkdirAll(filepath.Dir(outFile), 0o755); err != nil {
				return nil, err
			}
			if err := os.WriteFile(outFile, data, 0o644); err != nil {
				return nil, err
			}
			files = append(files, out)
		}

		for _, e := range dep.Embeds {
			pkg, _ := parseNameVersion(e) // validated by Load
			includes[pkg] = true
		}

		// A bundle can emit an asset that another bundle, or a copy,
		// also writes.
		sort.Strings(files)
		files = slices.Compact(files)
		m.Deps[key] = Entry{Package: top.Name, Version: top.Version, Files: files, Includes: sortedPackages(includes)}
	}

	if err := os.MkdirAll(dest, 0o755); err != nil {
		return nil, err
	}
	return m, writeManifest(m, filepath.Join(dest, ManifestFile))
}

func writeManifest(m *Manifest, file string) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(file, append(data, '\n'), 0o644)
}

// assetLoaders copies the files a stylesheet's url()s point at next to the
// bundle, under a content hash, rather than failing on them.
var assetLoaders = map[string]api.Loader{}

func init() {
	for _, ext := range []string{".avif", ".bmp", ".cur", ".eot", ".gif", ".ico", ".jpeg", ".jpg", ".otf", ".png", ".svg", ".ttf", ".webp", ".woff", ".woff2"} {
		assetLoaders[ext] = api.LoaderFile
	}
}

// bundle builds entry into outFile with esbuild: minified, for the browser,
// as an IIFE for .js, an ES module for .mjs, and a stylesheet for .css. It
// returns every file written (outFile, and any assets a stylesheet pulled
// in) and the absolute paths of the source files that went into them.
func (p *Project) bundle(entry, outFile string) (outputs, inputs []string, err error) {
	opts := api.BuildOptions{
		EntryPoints:       []string{entry},
		Outfile:           outFile,
		AbsWorkingDir:     p.Dir,
		Bundle:            true,
		Write:             true,
		Metafile:          true,
		MinifyWhitespace:  true,
		MinifyIdentifiers: true,
		MinifySyntax:      true,
		Platform:          api.PlatformBrowser,
		Loader:            assetLoaders,
		AssetNames:        "[name]-[hash]",
		LogLevel:          api.LogLevelSilent,
	}
	switch path.Ext(outFile) {
	case ".js":
		opts.Format = api.FormatIIFE
	case ".mjs":
		opts.Format = api.FormatESModule
	}
	result := api.Build(opts)
	if len(result.Errors) > 0 {
		msgs := api.FormatMessages(result.Errors, api.FormatMessagesOptions{Kind: api.ErrorMessage})
		return nil, nil, errors.New(strings.TrimSpace(strings.Join(msgs, "")))
	}

	var meta struct {
		Inputs  map[string]json.RawMessage `json:"inputs"`
		Outputs map[string]struct {
			EntryPoint string `json:"entryPoint"`
		} `json:"outputs"`
	}
	if err := json.Unmarshal([]byte(result.Metafile), &meta); err != nil {
		return nil, nil, fmt.Errorf("esbuild metafile: %w", err)
	}
	// Both are relative to AbsWorkingDir. An input in a namespace, like a
	// module the browser field disables, is not a file and has no owner.
	for _, in := range sortedKeys(meta.Inputs) {
		if strings.Contains(in, ":") {
			continue
		}
		inputs = append(inputs, filepath.Join(p.Dir, filepath.FromSlash(in)))
	}
	for _, o := range sortedKeys(meta.Outputs) {
		// esbuild writes a stylesheet to whatever name it is given, so a
		// package whose entry turns out to be one would land in a .mjs.
		if entry := meta.Outputs[o].EntryPoint; entry != "" && (path.Ext(entry) == ".css") != (path.Ext(outFile) == ".css") {
			return nil, nil, fmt.Errorf("%s: a stylesheet's output must end in .css, and only a stylesheet's", entry)
		}
		outputs = append(outputs, filepath.Join(p.Dir, filepath.FromSlash(o)))
	}
	return outputs, inputs, nil
}

// owner finds the package a file in dir belongs to: the nearest
// package.json above it with a name and a version, stopping at the project
// itself, whose own files belong to no package.
func (p *Project) owner(dir string, cache map[string]*installed) (*installed, error) {
	if pkg, ok := cache[dir]; ok {
		return pkg, nil
	}
	var pkg *installed
	if dir != p.Dir && filepath.Dir(dir) != dir {
		var err error
		if pkg, err = readPackage(dir); err != nil {
			return nil, err
		}
		if pkg == nil {
			if pkg, err = p.owner(filepath.Dir(dir), cache); err != nil {
				return nil, err
			}
		}
	}
	cache[dir] = pkg
	return pkg, nil
}

// installed is a package in node_modules.
type installed struct {
	// Requires are the ranges it asks of its dependencies, peers included.
	Requires map[string]string
	Package
}

// addRequires records the ranges pkg asks of other packages.
func (p *Project) addRequires(pkg *installed) {
	for name, r := range pkg.Requires {
		p.requires[name] = append(p.requires[name], r)
	}
}

// readPackage reads dir/package.json. It returns nil, without an error, when
// there is none or it names no package, as with the {"type": "module"}
// markers some packages put in subdirectories.
func readPackage(dir string) (*installed, error) {
	data, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil //nolint:nilnil // documented: nil package, no error, means there is none
	}
	if err != nil {
		return nil, err
	}
	var pj struct {
		Dependencies         map[string]string `json:"dependencies"`
		PeerDependencies     map[string]string `json:"peerDependencies"`
		OptionalDependencies map[string]string `json:"optionalDependencies"`
		Package
	}
	if err := json.Unmarshal(data, &pj); err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Join(dir, "package.json"), err)
	}
	if pj.Name == "" || pj.Version == "" {
		return nil, nil //nolint:nilnil // documented: nil package, no error, means it names no package
	}
	pkg := &installed{Package: pj.Package, Requires: map[string]string{}}
	for _, deps := range []map[string]string{pj.Dependencies, pj.PeerDependencies, pj.OptionalDependencies} {
		maps.Copy(pkg.Requires, deps)
	}
	return pkg, nil
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedPackages(set map[Package]bool) []Package {
	pkgs := make([]Package, 0, len(set))
	for pkg := range set {
		pkgs = append(pkgs, pkg)
	}
	sort.Slice(pkgs, func(i, j int) bool {
		if pkgs[i].Name != pkgs[j].Name {
			return pkgs[i].Name < pkgs[j].Name
		}
		return pkgs[i].Version < pkgs[j].Version
	})
	return pkgs
}
