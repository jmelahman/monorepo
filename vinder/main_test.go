package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jmelahman/vinder/internal/project"
)

func TestRun(t *testing.T) {
	t.Parallel()

	tests := []struct {
		wantErr    error
		name       string
		wantOut    string
		args       []string
		wantAnyErr bool
	}{
		{name: "help", args: []string{"-h"}, wantOut: "Usage:"},
		{name: "version", args: []string{"--version"}, wantOut: "version"},
		{name: "unknown command", args: []string{"nope"}, wantAnyErr: true},
		{name: "all with directories", args: []string{"check", "--all", "x"}, wantAnyErr: true},
		{name: "negative cooldown", args: []string{"upgrade", "--cooldown-days", "-1"}, wantAnyErr: true},
		{name: "directory without config", args: []string{"check", t.TempDir()}, wantErr: project.ErrNoConfig},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr strings.Builder
			err := run(tt.args, &stdout, &stderr)
			switch {
			case tt.wantAnyErr:
				if err == nil {
					t.Fatalf("run(%q) = nil, want an error", tt.args)
				}
			case !errors.Is(err, tt.wantErr):
				t.Fatalf("run(%q) error = %v, want %v", tt.args, err, tt.wantErr)
			}
			if !strings.Contains(stdout.String(), tt.wantOut) {
				t.Errorf("run(%q) stdout = %q, want it to contain %q", tt.args, stdout.String(), tt.wantOut)
			}
		})
	}
}

func TestAudit(t *testing.T) {
	t.Parallel()

	// An OSV stand-in that reports one advisory for dompurify and none for
	// anything else.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Queries []struct {
				Package struct{ Name string } `json:"package"`
			} `json:"queries"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		results := make([]map[string]any, len(req.Queries))
		for i, q := range req.Queries {
			results[i] = map[string]any{}
			if q.Package.Name == "dompurify" {
				results[i]["vulns"] = []map[string]string{{"id": "GHSA-test"}}
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"results": results})
	}))
	t.Cleanup(srv.Close)

	tests := []struct {
		wantErr  error
		name     string
		wantOut  string
		ignore   string
		includes []project.Package
	}{
		{
			name:     "clean",
			includes: []project.Package{{Name: "lib", Version: "1.0.0"}},
		},
		{
			name:     "vulnerable embed",
			includes: []project.Package{{Name: "dompurify", Version: "2.3.3"}, {Name: "lib", Version: "1.0.0"}},
			wantErr:  errVulnerable,
			wantOut:  "\tdep\tdompurify@2.3.3\tGHSA-test\n",
		},
		{
			name:     "package ignored",
			includes: []project.Package{{Name: "dompurify", Version: "2.3.3"}},
			ignore:   `"dompurify@2.3.3" = "never called"`,
		},
		{
			name:     "advisory ignored",
			includes: []project.Package{{Name: "dompurify", Version: "2.3.3"}},
			ignore:   `GHSA-test = "not exploitable here"`,
		},
		{
			name:     "other version not ignored",
			includes: []project.Package{{Name: "dompurify", Version: "2.3.3"}},
			ignore:   `"dompurify@2.5.9" = "never called"`,
			wantErr:  errVulnerable,
			wantOut:  "\tdep\tdompurify@2.3.3\tGHSA-test\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			writeFile(t, filepath.Join(dir, "package.json"), "{}")
			writeFile(t, filepath.Join(dir, project.ConfigFile), "[deps.dep]\npackage = \"lib\"\ncopy = { \"a.js\" = \"lib/a.js\" }\nignore = { "+tt.ignore+" }\n")
			m, err := json.Marshal(project.Manifest{Deps: map[string]project.Entry{"dep": {Package: "lib", Version: "1.0.0", Includes: tt.includes}}})
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(dir, "vendor", project.ManifestFile), string(m))

			var stdout, stderr strings.Builder
			err = run([]string{"audit", "--osv-url", srv.URL, dir}, &stdout, &stderr)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("audit error = %v, want %v", err, tt.wantErr)
			}
			if !strings.HasSuffix(stdout.String(), tt.wantOut) {
				t.Errorf("audit stdout = %q, want it to end with %q", stdout.String(), tt.wantOut)
			}
		})
	}
}

func writeFile(t *testing.T, file, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
