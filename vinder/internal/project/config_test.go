package project_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/jmelahman/vinder/internal/project"
)

func TestLoad(t *testing.T) {
	t.Parallel()

	const valid = `
[deps.ed]
package = "@scope/ed"
embeds = ["@scope/inner@1.0.0", "dompurify@2.3.3"]
bundle = { "ed.js" = "./src/ed.js", "ed.css" = "@scope/ed/dist/ed.css" }
copy = { "LICENSE" = "./LICENSE.ed" }
ignore = { "dompurify@2.3.3" = "never called" }
`
	tests := []struct {
		wantErr     error
		name        string
		config      string
		wantOut     string
		noConfig    bool
		noPackage   bool
		wantAnyFail bool
	}{
		{name: "valid", config: valid, wantOut: "vendor"},
		{name: "custom out", config: "out = \"lib/third_party\"\n" + valid, wantOut: "lib/third_party"},
		{name: "no config", noConfig: true, wantErr: project.ErrNoConfig},
		{name: "no package.json", config: valid, noPackage: true, wantErr: project.ErrBadConfig},
		{name: "syntax error", config: "[deps", wantErr: project.ErrBadConfig},
		{name: "unknown key", config: valid + "bundel = {}\n", wantErr: project.ErrBadConfig},
		{name: "no deps", config: "out = \"vendor\"\n", wantErr: project.ErrBadConfig},
		{name: "missing package", config: "[deps.x]\ncopy = { a = \"x/a\" }\n", wantErr: project.ErrBadConfig},
		{name: "no outputs", config: "[deps.x]\npackage = \"x\"\n", wantErr: project.ErrBadConfig},
		{name: "escaping output", config: "[deps.x]\npackage = \"x\"\ncopy = { \"../a\" = \"x/a\" }\n", wantErr: project.ErrBadConfig},
		{name: "escaping source", config: "[deps.x]\npackage = \"x\"\ncopy = { a = \"./../a\" }\n", wantErr: project.ErrBadConfig},
		{name: "escaping out", config: "out = \"../v\"\n[deps.x]\npackage = \"x\"\ncopy = { a = \"x/a\" }\n", wantErr: project.ErrBadConfig},
		{name: "bad bundle extension", config: "[deps.x]\npackage = \"x\"\nbundle = { a.txt = \"x/a\" }\n", wantErr: project.ErrBadConfig},
		{name: "output both bundled and copied", config: "[deps.x]\npackage = \"x\"\nbundle = { \"a.js\" = \"x/a\" }\ncopy = { \"a.js\" = \"x/a\" }\n", wantErr: project.ErrBadConfig},
		{name: "bad embed", config: "[deps.x]\npackage = \"x\"\ncopy = { a = \"x/a\" }\nembeds = [\"dompurify\"]\n", wantErr: project.ErrBadConfig},
		{name: "ignore without reason", config: "[deps.x]\npackage = \"x\"\ncopy = { a = \"x/a\" }\nignore = { GHSA-x = \" \" }\n", wantErr: project.ErrBadConfig},
		{name: "manifest-named key", config: "[deps.\"vinder.json\"]\npackage = \"x\"\ncopy = { a = \"x/a\" }\n", wantErr: project.ErrBadConfig},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			if !tt.noConfig {
				write(t, filepath.Join(dir, project.ConfigFile), tt.config)
			}
			if !tt.noPackage {
				write(t, filepath.Join(dir, "package.json"), "{}")
			}

			p, err := project.Load(dir)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Load() error = %v, want %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if want := filepath.Join(dir, tt.wantOut); p.OutDir() != want {
				t.Errorf("OutDir() = %q, want %q", p.OutDir(), want)
			}
		})
	}
}

func TestDiscover(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	for _, dir := range []string{"a", "b/c", "node_modules/x", ".hidden", "d"} {
		write(t, filepath.Join(root, dir, "package.json"), "{}")
	}
	for _, dir := range []string{"a", "b/c", "node_modules/x", ".hidden"} {
		write(t, filepath.Join(root, dir, project.ConfigFile), "")
	}

	got, err := project.Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{filepath.Join(root, "a"), filepath.Join(root, "b/c")}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("Discover() = %q, want %q", got, want)
	}
}

func write(t *testing.T, file, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
