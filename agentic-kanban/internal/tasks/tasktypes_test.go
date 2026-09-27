package tasks

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDiscoverTaskTypes(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "web", "pnpm-lock.yaml"), "")
	writeFile(t, filepath.Join(root, "api", "yarn.lock"), "")
	writeFile(t, filepath.Join(root, ".vscode", "tasks.json"), `{
  "version": "2.0.0",
  "tasks": [
    // Commented JSONC, as VS Code writes it.
    {"label": "Run", "type": "npm", "script": "dev", "isBackground": true},
    {"type": "npm", "script": "build", "path": "web"},
    {"label": "Install", "type": "npm", "script": "install"},
    {"label": "Cwd wins", "type": "npm", "script": "dev", "path": "web", "options": {"cwd": "api"}},
    {"label": "Shell", "type": "shell", "command": "make", "args": ["all"], "options": {"cwd": "sub"}},
    {"label": "Untyped", "command": "echo hi"},
    {"type": "typescript", "tsconfig": "tsconfig.json", "option": "watch"},
    {"label": "Gulp", "type": "gulp", "task": "default"},
    {"label": "Rake", "type": "rake", "task": "spec"},
    {"label": "Cargo", "type": "cargo", "command": "build", "args": ["--release"]},
    {"label": "Go", "type": "go", "command": "test", "args": ["./..."]},
    {"label": "Deno", "type": "deno", "command": "run", "args": ["main.ts"]},
    {"label": "All", "dependsOn": ["Run", "Shell"]},
    {"label": "Docker", "type": "docker-build"},
    {"type": "docker-run"},
    {"type": "npm"},
    {"label": "--- Section ---", "type": "npm", "script": "x"}
  ]
}`)

	got, warnings, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []VSCodeTask{
		{Label: "Run", Command: "npm", Args: []string{"run", "dev"}},
		{Label: "npm: build - web", Command: "pnpm", Args: []string{"run", "build"}, Cwd: "web"},
		{Label: "Install", Command: "npm", Args: []string{"install"}},
		{Label: "Cwd wins", Command: "yarn", Args: []string{"run", "dev"}, Cwd: "api"},
		{Label: "Shell", Command: "make", Args: []string{"all"}, Cwd: "sub"},
		{Label: "Untyped", Command: "echo hi"},
		{Label: "tsc: watch - tsconfig.json", Command: "npx", Args: []string{"tsc", "-p", "tsconfig.json", "--watch"}},
		{Label: "Gulp", Command: "npx", Args: []string{"gulp", "default"}},
		{Label: "Rake", Command: "rake", Args: []string{"spec"}},
		{Label: "Cargo", Command: "cargo", Args: []string{"build", "--release"}},
		{Label: "Go", Command: "go", Args: []string{"test", "./..."}},
		{Label: "Deno", Command: "deno", Args: []string{"run", "main.ts"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Discover tasks:\n got %+v\nwant %+v", got, want)
	}
	wantWarn := []string{
		`.vscode/tasks.json: task "Docker": unsupported type "docker-build"`,
		`.vscode/tasks.json: task #15: unsupported type "docker-run"`,
		`.vscode/tasks.json: task #16: npm task has no script`,
	}
	if !reflect.DeepEqual(warnings, wantWarn) {
		t.Errorf("warnings = %q; want %q", warnings, wantWarn)
	}
}

func TestDetectPackageManager(t *testing.T) {
	cases := []struct {
		name  string
		files map[string]string
		rel   string
		want  string
	}{
		{name: "nothing falls back to npm", want: "npm"},
		{name: "bun lockfile", files: map[string]string{"bun.lock": ""}, want: "bun"},
		{name: "legacy bun lockfile", files: map[string]string{"bun.lockb": ""}, want: "bun"},
		{name: "yarn lockfile", files: map[string]string{"yarn.lock": ""}, want: "yarn"},
		{name: "root lockfile found from workspace package", files: map[string]string{"pnpm-lock.yaml": "", "pkgs/a/package.json": "{}"}, rel: "pkgs/a", want: "pnpm"},
		{name: "package lockfile beats root", files: map[string]string{"pnpm-lock.yaml": "", "web/yarn.lock": ""}, rel: "web", want: "yarn"},
		{name: "packageManager field", files: map[string]string{"package.json": `{"packageManager": "pnpm@9.1.0"}`}, want: "pnpm"},
		{name: "unknown packageManager ignored", files: map[string]string{"package.json": `{"packageManager": "foo@1"}`}, want: "npm"},
		{name: "variable path uses root", files: map[string]string{"yarn.lock": ""}, rel: "${workspaceFolder}/web", want: "yarn"},
		{name: "escaping path stops at root", files: map[string]string{"yarn.lock": ""}, rel: "../..", want: "npm"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			parent := t.TempDir()
			root := filepath.Join(parent, "repo")
			if err := os.Mkdir(root, 0o755); err != nil {
				t.Fatal(err)
			}
			// A lockfile outside the worktree must never be picked up.
			writeFile(t, filepath.Join(parent, "bun.lock"), "")
			for name, content := range tc.files {
				writeFile(t, filepath.Join(root, name), content)
			}
			if got := detectPackageManager(root, tc.rel); got != tc.want {
				t.Errorf("detectPackageManager(%q) = %q; want %q", tc.rel, got, tc.want)
			}
		})
	}
}
