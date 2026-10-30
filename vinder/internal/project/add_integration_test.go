package project_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jmelahman/vinder/internal/pkgjson"
	"github.com/jmelahman/vinder/internal/project"
	"github.com/jmelahman/vinder/internal/registry"
	"github.com/jmelahman/vinder/internal/registry/registrytest"
)

// point has a project's package managers install from srv.
func point(t *testing.T, dir string, srv *registrytest.Server) {
	t.Helper()
	write(t, filepath.Join(dir, ".npmrc"), "registry="+srv.URL+"/\n")
	// Yarn 1 reads .npmrc too, and doesn't read this.
	write(t, filepath.Join(dir, ".yarnrc.yml"), "npmRegistryServer: \""+srv.URL+"\"\nunsafeHttpWhitelist: [\"127.0.0.1\"]\n")
}

// TestAdd starts a project from nothing under the package manager Add picks,
// with a package that has only an exports map and a scoped one whose name
// doesn't make a bare key, then adds to it.
func TestAdd(t *testing.T) {
	t.Parallel()
	packageManagers(t)
	ctx := context.Background()

	srv := registrytest.New(t,
		registrytest.Package{Name: "modern", Version: "1.2.0", Files: map[string]string{
			"package.json":  `{"name": "modern", "version": "1.2.0", "type": "module", "exports": {".": "./dist/index.js"}}`,
			"dist/index.js": "export const modern = \"modern\";\n",
		}},
		registrytest.Package{Name: "@scope/some.lib", Version: "3.0.0", Files: map[string]string{"index.js": "export const s = \"scoped\";\n"}},
		lib("1.0.0"), dep("1.0.0"),
	)
	reg := &registry.Client{URL: srv.URL}

	dir := t.TempDir()
	point(t, dir, srv)
	p, got, err := project.Add(ctx, dir, []string{"modern", "@scope/some.lib@3.0.0"}, reg.Resolve)
	if err != nil {
		t.Fatal(err)
	}
	want := []project.Addition{
		{Name: "modern", Version: "1.2.0", Key: "modern", Files: []string{"vendor/modern/modern.mjs"}},
		{Name: "@scope/some.lib", Version: "3.0.0", Key: "scope-some-lib", Files: []string{"vendor/scope-some-lib/scope-some-lib.mjs"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Add() = %+v, want %+v", got, want)
	}
	assertCurrent(t, p)

	bundle, err := os.ReadFile(filepath.Join(dir, "vendor/modern/modern.mjs"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(bundle), `"modern"`) || !strings.Contains(string(bundle), "export") {
		t.Errorf("modern.mjs = %q, want the package's code as an ES module", bundle)
	}
	pj, err := pkgjson.Read(filepath.Join(dir, "package.json"))
	if err != nil {
		t.Fatal(err)
	}
	if got := pj.Strings("dependencies"); !reflect.DeepEqual(got, map[string]string{"modern": "^1.2.0", "@scope/some.lib": "3.0.0"}) {
		t.Errorf("dependencies = %v", got)
	}
	if _, ok := pj.String("packageManager"); !ok {
		t.Error("package.json declares no packageManager")
	}

	// A second add keeps what the first wrote, and what a person did.
	configFile := filepath.Join(dir, project.ConfigFile)
	config, err := os.ReadFile(configFile)
	if err != nil {
		t.Fatal(err)
	}
	write(t, configFile, "# Mine.\n"+strings.TrimSuffix(string(config), "\n"))
	p, _, err = project.Add(ctx, dir, []string{"lib"}, reg.Resolve)
	if err != nil {
		t.Fatal(err)
	}
	assertCurrent(t, p)
	if keys := p.Keys(); !reflect.DeepEqual(keys, []string{"lib", "modern", "scope-some-lib"}) {
		t.Errorf("Keys() = %v", keys)
	}
	config, err = os.ReadFile(configFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(config), "# Mine.\n") {
		t.Errorf("vinder.toml lost its comment:\n%s", config)
	}
}

// TestAddExisting vendors a dependency the project already has, at the
// version it already asks for and without rewriting how it asks: "1" is a
// spec Add would resolve if it had been given it.
func TestAddExisting(t *testing.T) {
	t.Parallel()

	for _, pm := range packageManagers(t) {
		t.Run(pm, func(t *testing.T) {
			t.Parallel()

			srv := registrytest.New(t, lib("1.0.0"), lib("2.0.0"), dep("1.0.0"))
			dir := registryApp(t, pm, srv, "1")
			if err := os.Remove(filepath.Join(dir, project.ConfigFile)); err != nil {
				t.Fatal(err)
			}
			p, got, err := project.Add(context.Background(), dir, []string{"lib"}, func(context.Context, string, string) (string, error) {
				return "", errors.New("looked up a release")
			})
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != 1 || got[0].Version != "1.0.0" {
				t.Errorf("Add() = %+v, want lib 1.0.0", got)
			}
			pj, err := pkgjson.Read(filepath.Join(dir, "package.json"))
			if err != nil {
				t.Fatal(err)
			}
			if spec, _ := pj.String("dependencies", "lib"); spec != "1" {
				t.Errorf("package.json asks for lib %q, want it left at 1", spec)
			}
			assertCurrent(t, p)
		})
	}
}

// TestAddSpecs checks what goes in package.json for each way of asking for a
// version, and that the install takes it.
func TestAddSpecs(t *testing.T) {
	t.Parallel()
	packageManagers(t)

	release := func(version string, tags ...string) registrytest.Package {
		return registrytest.Package{Name: "spec-lib", Version: version, Tags: tags, Files: map[string]string{"index.js": "export const v = \"" + version + "\";\n"}}
	}
	srv := registrytest.New(t, release("1.0.0"), release("1.5.0"), release("2.0.0"), release("3.0.0-beta.1", "next"))
	reg := &registry.Client{URL: srv.URL}

	tests := []struct {
		name        string
		spec        string
		wantSpec    string
		wantVersion string
	}{
		{name: "none", spec: "spec-lib", wantSpec: "^2.0.0", wantVersion: "2.0.0"},
		{name: "latest", spec: "spec-lib@latest", wantSpec: "^2.0.0", wantVersion: "2.0.0"},
		{name: "other tag", spec: "spec-lib@next", wantSpec: "^3.0.0-beta.1", wantVersion: "3.0.0-beta.1"},
		{name: "major", spec: "spec-lib@1", wantSpec: "^1.5.0", wantVersion: "1.5.0"},
		{name: "x range", spec: "spec-lib@1.x", wantSpec: "^1.5.0", wantVersion: "1.5.0"},
		{name: "exact", spec: "spec-lib@1.0.0", wantSpec: "1.0.0", wantVersion: "1.0.0"},
		{name: "caret", spec: "spec-lib@^1.0.0", wantSpec: "^1.0.0", wantVersion: "1.5.0"},
		{name: "range", spec: "spec-lib@>=1 <2", wantSpec: ">=1 <2", wantVersion: "1.5.0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			point(t, dir, srv)
			_, got, err := project.Add(context.Background(), dir, []string{tt.spec}, reg.Resolve)
			if err != nil {
				t.Fatal(err)
			}
			if got[0].Version != tt.wantVersion {
				t.Errorf("Add(%q) vendored %s, want %s", tt.spec, got[0].Version, tt.wantVersion)
			}
			pj, err := pkgjson.Read(filepath.Join(dir, "package.json"))
			if err != nil {
				t.Fatal(err)
			}
			if spec, _ := pj.String("dependencies", "spec-lib"); spec != tt.wantSpec {
				t.Errorf("Add(%q) wrote %q to package.json, want %q", tt.spec, spec, tt.wantSpec)
			}
		})
	}

	t.Run("unknown tag", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		point(t, dir, srv)
		if _, _, err := project.Add(context.Background(), dir, []string{"spec-lib@nope"}, reg.Resolve); err == nil {
			t.Fatal("Add() = nil, want an error")
		}
		if _, err := os.Stat(filepath.Join(dir, "package.json")); !errors.Is(err, os.ErrNotExist) {
			t.Error("package.json left behind")
		}
	})
}

// TestAddStaleKey takes back a directory vinder wrote for a key since removed
// from vinder.toml, which is not someone else's to protect.
func TestAddStaleKey(t *testing.T) {
	t.Parallel()
	pm := packageManagers(t)[0]

	srv := registrytest.New(t, lib("1.0.0"), dep("1.0.0"))
	dir := registryApp(t, pm, srv, "^1.0.0")
	reg := &registry.Client{URL: srv.URL}
	syncApp(t, dir)
	write(t, filepath.Join(dir, project.ConfigFile), "")
	if err := os.Rename(filepath.Join(dir, "vendor/l"), filepath.Join(dir, "vendor/lib")); err != nil {
		t.Fatal(err)
	}
	m, err := os.ReadFile(filepath.Join(dir, "vendor", project.ManifestFile))
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, "vendor", project.ManifestFile), strings.Replace(string(m), `"l": {`, `"lib": {`, 1))

	p, _, err := project.Add(context.Background(), dir, []string{"lib"}, reg.Resolve)
	if err != nil {
		t.Fatal(err)
	}
	assertCurrent(t, p)
}

// TestAddSwapFailure says so when the output directory may have changed:
// here it can't be created, after everything else has succeeded. A dangling
// symlink reads as no manifest, so nothing fails until then.
func TestAddSwapFailure(t *testing.T) {
	t.Parallel()
	packageManagers(t)

	srv := registrytest.New(t, lib("1.0.0"), dep("1.0.0"))
	dir := t.TempDir()
	point(t, dir, srv)
	if err := os.Symlink("missing", filepath.Join(dir, "vendor")); err != nil {
		t.Fatal(err)
	}

	_, _, err := project.Add(context.Background(), dir, []string{"lib"}, (&registry.Client{URL: srv.URL}).Resolve)
	if err == nil {
		t.Fatal("Add() = nil, want an error")
	}
	if !strings.Contains(err.Error(), "may be partly changed") || strings.Contains(err.Error(), "nothing was added") {
		t.Errorf("Add() error = %v, want it to say vendor may have changed", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "package.json")); !errors.Is(err, os.ErrNotExist) {
		t.Error("package.json left behind")
	}
}

// TestAddFailure leaves the project as it found it when a package can't be
// bundled as an ES module: one has no entry point, and one's is a stylesheet.
func TestAddFailure(t *testing.T) {
	t.Parallel()
	packageManagers(t)

	srv := registrytest.New(t, registrytest.Package{Name: "no-entry", Version: "1.0.0", Files: map[string]string{
		"package.json": `{"name": "no-entry", "version": "1.0.0"}`,
		"style.css":    ".a { color: red }\n",
	}}, registrytest.Package{Name: "only-styles", Version: "1.0.0", Files: map[string]string{
		"package.json": `{"name": "only-styles", "version": "1.0.0", "main": "style.css"}`,
		"style.css":    ".a { color: red }\n",
	}})
	reg := &registry.Client{URL: srv.URL}

	for _, name := range []string{"no-entry", "only-styles"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			point(t, dir, srv)
			before := listDir(t, dir)
			if _, _, err := project.Add(context.Background(), dir, []string{name}, reg.Resolve); err == nil {
				t.Fatal("Add() = nil, want an error")
			}
			if after := listDir(t, dir); !reflect.DeepEqual(after, before) {
				t.Errorf("project now has %v, want %v", after, before)
			}
		})
	}

	t.Run("unknown package", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		point(t, dir, srv)
		original, err := json.Marshal(map[string]any{"name": "mine"})
		if err != nil {
			t.Fatal(err)
		}
		write(t, filepath.Join(dir, "package.json"), string(original))
		if _, _, err := project.Add(context.Background(), dir, []string{"nothing-here"}, reg.Resolve); err == nil {
			t.Fatal("Add() = nil, want an error")
		}
		got, err := os.ReadFile(filepath.Join(dir, "package.json"))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != string(original) {
			t.Errorf("package.json = %s, want %s", got, original)
		}
		if _, err := os.Stat(filepath.Join(dir, project.ConfigFile)); !errors.Is(err, os.ErrNotExist) {
			t.Error("vinder.toml left behind")
		}
	})
}

func listDir(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name()
	}
	return names
}
