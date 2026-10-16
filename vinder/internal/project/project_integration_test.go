package project_test

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/jmelahman/vinder/internal/project"
)

// These run real package managers against packages packed into local
// tarballs, so they need no network. TestSync runs under every supported
// package manager on PATH; the rest use the first one found.

const appConfig = `
[deps.a]
package = "lib-a"
embeds = ["inner@0.1.0"]
bundle = { "a.js" = "./src/entry.js", "a.css" = "lib-a/style.css" }
copy = { "LICENSE" = "./LICENSE.a", "docs/README.md" = "lib-a/README.md" }
`

// lockfiles are the supported package managers and the lockfile each
// writes for the fixture.
var lockfiles = map[string]string{
	"bun":  "bun.lock",
	"npm":  "package-lock.json",
	"nub":  "nub.lock",
	"pnpm": "pnpm-lock.yaml",
	"yarn": "yarn.lock",
}

// packageManagers returns the supported package managers on PATH, sorted,
// skipping the test if there are none.
func packageManagers(t *testing.T) []string {
	t.Helper()
	var found []string
	for _, pm := range slices.Sorted(maps.Keys(lockfiles)) {
		if _, err := exec.LookPath(pm); err == nil {
			found = append(found, pm)
		}
	}
	if len(found) == 0 {
		t.Skip("no package manager on PATH")
	}
	return found
}

// newApp lays out two packed packages and a project depending on both that
// declares pm as its package manager, and returns the project's directory.
// With pm empty it uses the first package manager on PATH.
func newApp(t *testing.T, pm string) string {
	t.Helper()
	if pm == "" {
		pm = packageManagers(t)[0]
	}
	// Declared as name@version, as corepack writes it; Yarn 1 refuses a bare
	// name.
	version, err := exec.Command(pm, "--version").Output()
	if err != nil {
		t.Fatalf("%s --version: %v", pm, err)
	}
	pm += "@" + strings.TrimPrefix(strings.TrimSpace(string(version)), "v")

	root := t.TempDir()
	pack(t, filepath.Join(root, "lib-a.tgz"), map[string]string{
		"package.json": `{"name": "lib-a", "version": "1.2.3", "main": "index.js"}`,
		"index.js":     `export { a } from "./dist/a.js";`,
		// A subdirectory marker that names no package: its files still
		// belong to lib-a.
		"dist/package.json": `{"type": "module"}`,
		"dist/a.js":         `export const a = (s) => "a:" + s;`,
		"style.css":         ".a { color: red; background: url(./dot.png) }\n",
		"dot.png":           "not really a png",
		"README.md":         "# lib-a\n",
	})
	pack(t, filepath.Join(root, "lib-b.tgz"), map[string]string{
		"package.json": `{"name": "lib-b", "version": "0.4.0", "main": "index.js"}`,
		"index.js":     `export const b = (s) => "b:" + s;`,
		"unused.js":    `export const never = 1;`,
	})

	app := filepath.Join(root, "app")
	write(t, filepath.Join(app, "package.json"), `{"private": true, "packageManager": "`+pm+`", "dependencies": {"lib-a": "file:../lib-a.tgz", "lib-b": "file:../lib-b.tgz"}}`)
	write(t, filepath.Join(app, project.ConfigFile), appConfig)
	write(t, filepath.Join(app, "src/entry.js"), "import { a } from \"lib-a\";\nimport { b } from \"lib-b\";\nwindow.demo = { a, b };\n")
	write(t, filepath.Join(app, "LICENSE.a"), "MIT\n")
	return app
}

func pack(t *testing.T, file string, files map[string]string) {
	t.Helper()
	f, err := os.Create(file)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	for name, content := range files {
		hdr := &tar.Header{Name: "package/" + name, Mode: 0o644, Size: int64(len(content))}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
}

func syncApp(t *testing.T, app string) *project.Project {
	t.Helper()
	p, err := project.Load(app)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestSync(t *testing.T) {
	t.Parallel()

	for _, pm := range packageManagers(t) {
		t.Run(pm, func(t *testing.T) {
			t.Parallel()

			app := newApp(t, pm)
			write(t, filepath.Join(app, "vendor/keep.txt"), "not vinder's\n")
			p := syncApp(t, app)

			for _, f := range []string{lockfiles[pm], "vendor/keep.txt", "vendor/a/LICENSE", "vendor/a/docs/README.md", "vendor/a/a.css"} {
				if _, err := os.Stat(filepath.Join(app, f)); err != nil {
					t.Errorf("after sync: %v", err)
				}
			}
			js, err := os.ReadFile(filepath.Join(app, "vendor/a/a.js"))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(js), "window.demo") || strings.Contains(string(js), "never") {
				t.Errorf("vendor/a/a.js is not the minified, tree-shaken bundle:\n%s", js)
			}
			if _, err := os.Stat(filepath.Join(app, "node_modules")); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("node_modules left behind: %v", err)
			}
			if leftovers, _ := filepath.Glob(filepath.Join(app, ".vinder-*")); len(leftovers) > 0 {
				t.Errorf("temporary directories left behind: %q", leftovers)
			}

			m, err := project.ReadManifest(filepath.Join(p.OutDir(), project.ManifestFile))
			if err != nil {
				t.Fatal(err)
			}
			got := m.Deps["a"]
			// The image the stylesheet points at is copied beside it under
			// a content hash.
			got.Files = slices.DeleteFunc(got.Files, func(f string) bool {
				return strings.HasPrefix(f, "dot-") && strings.HasSuffix(f, ".png")
			})
			if len(got.Files) != len(m.Deps["a"].Files)-1 {
				t.Errorf("manifest[a].Files = %q, want one dot-*.png", m.Deps["a"].Files)
			}
			want := project.Entry{
				Package: "lib-a",
				Version: "1.2.3",
				Files:   []string{"LICENSE", "a.css", "a.js", "docs/README.md"},
				Includes: []project.Package{
					{Name: "inner", Version: "0.1.0"},
					{Name: "lib-a", Version: "1.2.3"},
					{Name: "lib-b", Version: "0.4.0"},
				},
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("manifest[a] = %+v, want %+v", got, want)
			}

			// The frozen install reproduces what sync wrote.
			diffs, err := p.Check(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if len(diffs) > 0 {
				t.Errorf("Check() after sync = %q, want none", diffs)
			}
		})
	}
}

func TestCheck(t *testing.T) {
	t.Parallel()

	tests := []struct {
		edit func(t *testing.T, app string)
		name string
		want []string
	}{
		{name: "up to date", edit: func(*testing.T, string) {}},
		{
			name: "hand edit",
			edit: func(t *testing.T, app string) { write(t, filepath.Join(app, "vendor/a/a.js"), "patched") },
			want: []string{"modified vendor/a/a.js"},
		},
		{
			name: "extra and missing files",
			edit: func(t *testing.T, app string) {
				write(t, filepath.Join(app, "vendor/a/extra.js"), "")
				if err := os.Remove(filepath.Join(app, "vendor/a/LICENSE")); err != nil {
					t.Fatal(err)
				}
			},
			want: []string{"missing  vendor/a/LICENSE", "extra    vendor/a/extra.js"},
		},
		{
			name: "source changed since sync",
			edit: func(t *testing.T, app string) { write(t, filepath.Join(app, "LICENSE.a"), "Apache-2.0\n") },
			want: []string{"modified vendor/a/LICENSE"},
		},
		{
			name: "files outside vinder's directories are ignored",
			edit: func(t *testing.T, app string) { write(t, filepath.Join(app, "vendor/other/x.js"), "") },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			app := newApp(t, "")
			p := syncApp(t, app)
			tt.edit(t, app)

			got, err := p.Check(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("Check() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCheckWithoutLock(t *testing.T) {
	t.Parallel()

	p, err := project.Load(newApp(t, ""))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Check(context.Background()); !errors.Is(err, project.ErrNoLock) {
		t.Errorf("Check() error = %v, want %v", err, project.ErrNoLock)
	}
}

func TestSyncRemovesStaleKeys(t *testing.T) {
	t.Parallel()

	app := newApp(t, "")
	syncApp(t, app)
	write(t, filepath.Join(app, project.ConfigFile), strings.Replace(appConfig, "[deps.a]", "[deps.renamed]", 1))
	syncApp(t, app)

	if _, err := os.Stat(filepath.Join(app, "vendor/a")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("vendor/a after its key was renamed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(app, "vendor/renamed/a.js")); err != nil {
		t.Error(err)
	}
}
