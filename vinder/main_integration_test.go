package main

import (
	"bytes"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jmelahman/vinder/internal/project"
	"github.com/jmelahman/vinder/internal/registry/registrytest"
)

// TestAdd runs the add command in an empty directory and checks what it
// prints. Yarn 2+ reads its registry from .yarnrc.yml, which Yarn 1 ignores.
func TestAdd(t *testing.T) {
	t.Parallel()

	found := false
	for _, name := range []string{"bun", "npm", "nub", "pnpm", "yarn"} {
		_, err := exec.LookPath(name)
		found = found || err == nil
	}
	if !found {
		t.Skip("no package manager on PATH")
	}
	srv := registrytest.New(t, registrytest.Package{Name: "added-lib", Version: "1.0.0", Files: map[string]string{"index.js": "export const l = 1;\n"}})
	app := t.TempDir()
	writeFile(t, filepath.Join(app, ".npmrc"), "registry="+srv.URL+"/\n")
	writeFile(t, filepath.Join(app, ".yarnrc.yml"), "npmRegistryServer: \""+srv.URL+"\"\nunsafeHttpWhitelist: [\"127.0.0.1\"]\n")

	var stdout, stderr bytes.Buffer
	if err := run([]string{"add", "--dir", app, "--registry", srv.URL, "added-lib"}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if want := "added added-lib 1.0.0\n  vendor/added-lib/added-lib.mjs\n"; !strings.HasPrefix(stdout.String(), want) {
		t.Errorf("add printed:\n%s\nwant it to start with:\n%s", stdout.String(), want)
	}
	stdout.Reset()
	if err := run([]string{"check", app}, &stdout, &stderr); err != nil {
		t.Errorf("check after add: %v\n%s", err, stdout.String())
	}
}

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

	// Too new to take with a cooldown.
	fresh := lib("3.0.0")
	fresh.Published = time.Now()
	srv.Publish(t, fresh)
	stdout.Reset()
	err = run([]string{"upgrade", "--registry", srv.URL, "--cooldown-days", "7", app}, &stdout, &stderr)
	if errors.Is(err, project.ErrNoCooldown) {
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	if got := stdout.String(); got != "Nothing vendored changed.\n" {
		t.Errorf("upgrade with a cooldown printed:\n%s", got)
	}
}
