package main

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jmelahman/vinder/internal/registry/registrytest"
)

// TestUpgrade runs the upgrade command against a local registry with the
// first package manager on PATH and checks the report it prints.
func TestUpgrade(t *testing.T) {
	t.Parallel()

	var pm string
	for _, name := range []string{"bun", "npm", "nub", "pnpm", "yarn"} {
		if _, err := exec.LookPath(name); err == nil {
			pm = name
			break
		}
	}
	if pm == "" {
		t.Skip("no package manager on PATH")
	}
	out, err := exec.Command(pm, "--version").Output()
	if err != nil {
		t.Fatalf("%s --version: %v", pm, err)
	}
	if pm == "yarn" && !strings.HasPrefix(string(out), "1.") {
		t.Skip("Yarn 2+ reads its registry from .yarnrc.yml")
	}

	lib := func(version string) registrytest.Package {
		return registrytest.Package{
			Name: "lib", Version: version,
			Dependencies: map[string]string{"dep": "^1.0.0"},
			Files:        map[string]string{"index.js": "import { d } from \"dep\";\nexport const l = d;\n"},
		}
	}
	dep := func(version string) registrytest.Package {
		return registrytest.Package{Name: "dep", Version: version, Files: map[string]string{"index.js": "export const d = \"" + version + "\";\n"}}
	}
	srv := registrytest.New(t, lib("1.0.0"), dep("1.0.0"))

	app := t.TempDir()
	writeFile(t, filepath.Join(app, "package.json"), `{"private": true, "packageManager": "`+pm+"@"+strings.TrimPrefix(strings.TrimSpace(string(out)), "v")+`", "dependencies": {"lib": "^1.0.0"}}`)
	writeFile(t, filepath.Join(app, ".npmrc"), "registry="+srv.URL+"/\n")
	writeFile(t, filepath.Join(app, "vinder.toml"), "[deps.l]\npackage = \"lib\"\nbundle = { \"l.js\" = \"lib\" }\n")

	var stdout, stderr bytes.Buffer
	if err := run([]string{"sync", app}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}

	stdout.Reset()
	if err := run([]string{"upgrade", "--registry", srv.URL, app}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if got := stdout.String(); got != "Nothing vendored changed.\n" {
		t.Errorf("upgrade with nothing new printed:\n%s", got)
	}

	srv.Publish(t, lib("2.0.0"))
	srv.Publish(t, dep("1.1.0"))
	stdout.Reset()
	if err := run([]string{"upgrade", "--registry", srv.URL, app}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"**package.json**\n\n- `lib` 1.0.0 → 2.0.0\n",
		"**vendored**\n\n- `dep` 1.0.0 → 1.1.0\n- `lib` 1.0.0 → 2.0.0\n",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("upgrade printed:\n%s\nwant it to contain:\n%s", stdout.String(), want)
		}
	}
}
