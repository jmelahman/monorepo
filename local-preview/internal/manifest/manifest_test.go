package manifest

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

const valid = `
[frontend]
path  = "web"
build = [["npm", "ci"], ["npm", "run", "build"]]
dist  = "dist"

[backend]
path        = "."
exclude     = ["docs/", "*.md"]
build       = [["go", "build", "-o", "bin/server", "."]]
run         = ["./bin/server", "--addr", ":{port}", "--data-dir", "{state_dir}"]
health_path = "/api/health"
`

func TestParseValid(t *testing.T) {
	m, err := Parse([]byte(valid))
	if err != nil {
		t.Fatal(err)
	}
	if m.Frontend.Path != "web" || m.Frontend.Dist != "dist" || m.Backend.Path != "." {
		t.Fatalf("unexpected manifest: %+v", m)
	}
	if time.Duration(m.Backend.StartTimeout) != DefaultStartTimeout {
		t.Fatalf("start_timeout default = %v", m.Backend.StartTimeout)
	}
	if time.Duration(m.Backend.IdleTimeout) != DefaultIdleTimeout {
		t.Fatalf("idle_timeout default = %v", m.Backend.IdleTimeout)
	}
}

func TestParseDevcontainerToggle(t *testing.T) {
	m, err := Parse([]byte(valid))
	if err != nil {
		t.Fatal(err)
	}
	if !m.DevcontainerEnabled() {
		t.Fatal("discovery should default to enabled")
	}
	m, err = Parse([]byte("devcontainer = false\n" + valid))
	if err != nil {
		t.Fatal(err)
	}
	if m.DevcontainerEnabled() {
		t.Fatal("devcontainer = false should disable discovery")
	}
	m, err = Parse([]byte("devcontainer = true\n" + valid))
	if err != nil {
		t.Fatal(err)
	}
	if !m.DevcontainerEnabled() {
		t.Fatal("devcontainer = true should keep discovery on")
	}
}

func TestParseTimeouts(t *testing.T) {
	src := valid + "\nstart_timeout = \"5s\"\nidle_timeout = \"1h\"\n"
	m, err := Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if time.Duration(m.Backend.StartTimeout) != 5*time.Second {
		t.Fatalf("start_timeout = %v, want 5s", m.Backend.StartTimeout)
	}
	if time.Duration(m.Backend.IdleTimeout) != time.Hour {
		t.Fatalf("idle_timeout = %v, want 1h", m.Backend.IdleTimeout)
	}
}

func TestParseInit(t *testing.T) {
	src := valid + "\ninit = [[\"alembic\", \"upgrade\", \"head\"], [\"./seed\", \"{state_dir}\"]]\ninit_timeout = \"90s\"\n"
	m, err := Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Backend.Init) != 2 || m.Backend.Init[0][0] != "alembic" {
		t.Fatalf("init = %+v", m.Backend.Init)
	}
	if time.Duration(m.Backend.InitTimeout) != 90*time.Second {
		t.Fatalf("init_timeout = %v, want 90s", m.Backend.InitTimeout)
	}

	// Init is optional, and its timeout defaults when omitted.
	m, err = Parse([]byte(valid))
	if err != nil {
		t.Fatal(err)
	}
	if m.Backend.Init != nil {
		t.Fatalf("init = %+v, want none", m.Backend.Init)
	}
	if time.Duration(m.Backend.InitTimeout) != DefaultInitTimeout {
		t.Fatalf("init_timeout default = %v", m.Backend.InitTimeout)
	}
}

func TestParseArtifacts(t *testing.T) {
	src := valid + `
[artifacts.cli]
path    = "cli"
exclude = ["*.md"]
build   = [["go", "build", "-o", "bin/mycli", "."]]
files   = ["./bin/mycli", "bin/checksums.txt"]
`
	m, err := Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	a, ok := m.Artifacts["cli"]
	if !ok {
		t.Fatalf("artifacts = %+v, want cli entry", m.Artifacts)
	}
	if a.Path != "cli" || len(a.Build) != 1 {
		t.Fatalf("unexpected artifact: %+v", a)
	}
	// File paths are cleaned at parse time.
	if a.Files[0] != "bin/mycli" || a.Files[1] != "bin/checksums.txt" {
		t.Fatalf("files = %v", a.Files)
	}

	// Artifacts are optional.
	m, err = Parse([]byte(valid))
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Artifacts) != 0 {
		t.Fatalf("artifacts = %+v, want none", m.Artifacts)
	}
}

func TestParseArtifactErrors(t *testing.T) {
	cases := map[string]string{
		"bad name": `
[artifacts."My CLI"]
path  = "cli"
build = [["true"]]
files = ["mycli"]
`,
		"missing files": `
[artifacts.cli]
path  = "cli"
build = [["true"]]
`,
		"dot file": `
[artifacts.cli]
path  = "cli"
build = [["true"]]
files = ["."]
`,
		"escaping file": `
[artifacts.cli]
path  = "cli"
build = [["true"]]
files = ["../secret"]
`,
		"duplicate base name": `
[artifacts.cli]
path  = "cli"
build = [["true"]]
files = ["linux/mycli", "darwin/mycli"]
`,
		"missing build": `
[artifacts.cli]
path  = "cli"
files = ["mycli"]
`,
		"unknown key (no run)": `
[artifacts.cli]
path  = "cli"
build = [["true"]]
files = ["mycli"]
run   = ["./mycli"]
`,
	}
	for name, section := range cases {
		if _, err := Parse([]byte(valid + section)); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestParseAt(t *testing.T) {
	// The manifest hosted as [previews.*] tables inside a larger config
	// file, surrounded by foreign tables.
	hosted := "[harness]\nname = \"claude\"\n" +
		strings.ReplaceAll(valid, "\n[", "\n[previews.") +
		"\n[other]\nkey = true\n"

	m, err := ParseAt([]byte(hosted), "previews")
	if err != nil {
		t.Fatal(err)
	}
	if m.Frontend.Path != "web" || m.Backend.HealthPath != "/api/health" {
		t.Fatalf("unexpected manifest: %+v", m)
	}

	// Empty table name = whole-file manifest.
	if _, err := ParseAt([]byte(valid), ""); err != nil {
		t.Fatal(err)
	}

	// Missing table is ErrNoManifest (caller tries the next source).
	if _, err := ParseAt([]byte("[sync]\nallow_rebase = true\n"), "previews"); !errors.Is(err, ErrNoManifest) {
		t.Fatalf("err = %v, want ErrNoManifest", err)
	}

	// Unknown keys inside the table are an error; outside it they're not
	// ours to police.
	bad := hosted + "\n[previews.typo]\nx = 1\n"
	if _, err := ParseAt([]byte(bad), "previews"); err == nil || !strings.Contains(err.Error(), "unknown keys") {
		t.Fatalf("err = %v, want unknown-keys error", err)
	}
}

const processMode = `
networks = ["deps_default"]

[frontend]
path        = "web"
build       = [["bun", "run", "build"]]
run         = ["node", "server.js"]
run_image   = "node:24-slim"
health_path = "/"

[frontend.env]
PORT         = "{port}"
INTERNAL_URL = "{backend_url}"

[backend]
path             = "backend"
build            = [["uv", "sync"]]
run              = ["uvicorn", "app:app", "--port", "{port}"]
run_image        = "python:3.13-slim"
health_path      = "/health"
strip_api_prefix = true
extra_routes     = ["/openapi.json", "/auth/saml"]

[backend.env]
POSTGRES_DB = "preview_{hash}"
`

func TestParseProcessMode(t *testing.T) {
	m, err := Parse([]byte(processMode))
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Frontend.Run) == 0 || m.Frontend.Dist != "" {
		t.Fatalf("frontend: %+v", m.Frontend)
	}
	if time.Duration(m.Frontend.StartTimeout) != DefaultStartTimeout {
		t.Fatalf("frontend start_timeout default = %v", m.Frontend.StartTimeout)
	}
	if !m.Backend.StripAPIPrefix || len(m.Backend.ExtraRoutes) != 2 {
		t.Fatalf("backend: %+v", m.Backend)
	}
	if m.Networks[0] != "deps_default" {
		t.Fatalf("networks: %v", m.Networks)
	}
}

func TestParseProcessModeErrors(t *testing.T) {
	cases := map[string]struct {
		mutate func(string) string
		want   string
	}{
		"run without health_path": {
			func(s string) string { return strings.Replace(s, `health_path = "/"`, "", 1) },
			"frontend.health_path is required",
		},
		"static frontend still needs dist": {
			func(s string) string {
				s = strings.Replace(s, `run         = ["node", "server.js"]`, "", 1)
				return strings.Replace(s, `INTERNAL_URL = "{backend_url}"`, "", 1)
			},
			"frontend.dist is required",
		},
		"unknown env placeholder": {
			func(s string) string { return strings.Replace(s, `"preview_{hash}"`, `"preview_{short_sha}"`, 1) },
			"unknown placeholder {short_sha}",
		},
		"state_dir not a frontend placeholder": {
			func(s string) string { return strings.Replace(s, `PORT         = "{port}"`, `PORT = "{state_dir}"`, 1) },
			"unknown placeholder {state_dir}",
		},
		"backend_url needs run": {
			func(s string) string {
				return strings.Replace(s, `run         = ["node", "server.js"]`, `dist = "out"`, 1)
			},
			"{backend_url} requires frontend.run",
		},
		"backend_url needs matching runtimes": {
			func(s string) string { return strings.Replace(s, `run_image        = "python:3.13-slim"`, "", 1) },
			"same runtime",
		},
		"relative extra route": {
			func(s string) string { return strings.Replace(s, `"/openapi.json"`, `"openapi.json"`, 1) },
			"extra_routes",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := Parse([]byte(tc.mutate(processMode)))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want containing %q", err, tc.want)
			}
		})
	}
}

// TestLegacySectionJSONUnchanged pins the omitempty contract: a manifest
// using no new fields must marshal to the same section JSON as before, or
// every existing artifact hash would bust on upgrade (hashkey digests the
// section JSON).
func TestLegacySectionJSONUnchanged(t *testing.T) {
	m, err := Parse([]byte(valid))
	if err != nil {
		t.Fatal(err)
	}
	fe, _ := json.Marshal(m.Frontend)
	if want := `{"path":"web","build":[["npm","ci"],["npm","run","build"]],"dist":"dist"}`; string(fe) != want {
		t.Fatalf("frontend JSON changed:\n got %s\nwant %s", fe, want)
	}
	be, _ := json.Marshal(m.Backend)
	if strings.Contains(string(be), "run_image") || strings.Contains(string(be), "env") ||
		strings.Contains(string(be), "strip_api_prefix") || strings.Contains(string(be), "extra_routes") {
		t.Fatalf("backend JSON leaks new zero-value fields: %s", be)
	}
}

func TestParseErrors(t *testing.T) {
	cases := map[string]struct {
		mutate func(string) string
		want   string
	}{
		"unknown key": {
			func(s string) string { return s + "\ntypo_key = true\n" },
			"unknown keys",
		},
		"missing frontend path": {
			func(s string) string { return strings.Replace(s, `path  = "web"`, "", 1) },
			"frontend.path is required",
		},
		"absolute path": {
			func(s string) string { return strings.Replace(s, `path  = "web"`, `path = "/web"`, 1) },
			"relative path",
		},
		"escaping path": {
			func(s string) string { return strings.Replace(s, `path  = "web"`, `path = "../web"`, 1) },
			"relative path",
		},
		"missing run": {
			func(s string) string {
				return strings.Replace(s, `run         = ["./bin/server", "--addr", ":{port}", "--data-dir", "{state_dir}"]`, "", 1)
			},
			"backend.run is required",
		},
		"bad health path": {
			func(s string) string {
				return strings.Replace(s, `health_path = "/api/health"`, `health_path = "api/health"`, 1)
			},
			"health_path",
		},
		"empty init step": {
			func(s string) string { return s + "\ninit = [[]]\n" },
			"backend.init[0] must be a non-empty argv array",
		},
		"port in init": {
			func(s string) string { return s + "\ninit = [[\"./migrate\", \"--port\", \"{port}\"]]\n" },
			"{port} is not available during init",
		},
		"empty build step": {
			func(s string) string {
				return strings.Replace(s, `build       = [["go", "build", "-o", "bin/server", "."]]`, `build = [[]]`, 1)
			},
			"non-empty argv",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := Parse([]byte(tc.mutate(valid)))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want containing %q", err, tc.want)
			}
		})
	}
}
