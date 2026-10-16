package project_test

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jmelahman/vinder/internal/pkgjson"
	"github.com/jmelahman/vinder/internal/project"
	"github.com/jmelahman/vinder/internal/registry"
	"github.com/jmelahman/vinder/internal/registry/registrytest"
)

// overridesPaths are where each package manager reads overrides from.
var overridesPaths = map[string][]string{
	"bun":  {"overrides"},
	"npm":  {"overrides"},
	"nub":  {"overrides"},
	"pnpm": {"pnpm", "overrides"},
	"yarn": {"resolutions"},
}

// registryApp lays out a project that installs lib from srv under pm and
// bundles it, and returns its directory.
func registryApp(t *testing.T, pm string, srv *registrytest.Server, libSpec string) string {
	t.Helper()
	out, err := exec.Command(pm, "--version").Output()
	if err != nil {
		t.Fatalf("%s --version: %v", pm, err)
	}
	version := strings.TrimPrefix(strings.TrimSpace(string(out)), "v")

	app := t.TempDir()
	data, err := json.Marshal(map[string]any{
		"private":        true,
		"packageManager": pm + "@" + version,
		"dependencies":   map[string]string{"lib": libSpec},
	})
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(app, "package.json"), string(data))
	write(t, filepath.Join(app, ".npmrc"), "registry="+srv.URL+"/\n")
	if pm == "yarn" && !strings.HasPrefix(version, "1.") {
		write(t, filepath.Join(app, ".yarnrc.yml"), "npmRegistryServer: \""+srv.URL+"\"\nunsafeHttpWhitelist: [\"127.0.0.1\"]\n")
	}
	write(t, filepath.Join(app, project.ConfigFile), "[deps.l]\npackage = \"lib\"\nbundle = { \"l.js\" = \"lib\" }\n")
	return app
}

func lib(version, depRange string) registrytest.Package {
	return registrytest.Package{
		Name:         "lib",
		Version:      version,
		Dependencies: map[string]string{"dep": depRange},
		Files:        map[string]string{"index.js": "import { d } from \"dep\";\nexport const l = () => \"lib " + version + " \" + d;\n"},
	}
}

func dep(version string) registrytest.Package {
	return registrytest.Package{
		Name:    "dep",
		Version: version,
		Files:   map[string]string{"index.js": "export const d = \"dep " + version + "\";\n"},
	}
}

func manifest(t *testing.T, p *project.Project) *project.Manifest {
	t.Helper()
	m, err := project.ReadManifest(filepath.Join(p.OutDir(), project.ManifestFile))
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func assertCurrent(t *testing.T, p *project.Project) {
	t.Helper()
	diffs, err := p.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(diffs) > 0 {
		t.Errorf("Check(%s) = %q, want none", p.Dir, diffs)
	}
}

// TestAlignAndUpgrade aligns two projects that locked different versions
// of a transitive dependency, then upgrades one past the alignment.
func TestAlignAndUpgrade(t *testing.T) {
	t.Parallel()

	for _, pm := range packageManagers(t) {
		t.Run(pm, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			// app1 locks dep 1.0.0; 1.1.0 is out by the time app2 locks.
			srv := registrytest.New(t, lib("1.0.0", "^1.0.0"), dep("1.0.0"))
			p1 := syncApp(t, registryApp(t, pm, srv, "^1.0.0"))
			srv.Publish(t, dep("1.1.0"))
			p2 := syncApp(t, registryApp(t, pm, srv, "^1.0.0"))

			got, err := project.Align(ctx, []*project.Project{p1, p2})
			if err != nil {
				t.Fatal(err)
			}
			want := []project.Alignment{{Name: "dep", Version: "1.1.0", Moved: map[string]string{p1.Dir: "1.0.0"}}}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("Align() = %+v, want %+v", got, want)
			}
			pj, err := pkgjson.Read(filepath.Join(p1.Dir, "package.json"))
			if err != nil {
				t.Fatal(err)
			}
			path := append(overridesPaths[pm], "dep")
			if got, _ := pj.String(path...); got != "1.1.0" {
				t.Errorf("package.json %s = %q, want 1.1.0", strings.Join(path, "."), got)
			}
			if got := manifest(t, p1).Aligned; !reflect.DeepEqual(got, map[string]string{"dep": "1.1.0"}) {
				t.Errorf("manifest Aligned = %v, want dep 1.1.0", got)
			}
			if skews, err := project.FindSkew([]*project.Project{p1, p2}); err != nil || len(skews) > 0 {
				t.Errorf("FindSkew() after Align = %+v, %v; want none", skews, err)
			}
			assertCurrent(t, p1)
			if again, err := project.Align(ctx, []*project.Project{p1, p2}); err != nil || len(again) > 0 {
				t.Errorf("second Align() = %+v, %v; want nothing to do", again, err)
			}

			// Upgrade bumps lib across a major, drops the alignment's
			// override, and re-resolves dep to its newest 1.x.
			srv.Publish(t, lib("2.0.0", "^1.0.0"))
			srv.Publish(t, dep("1.2.0"))
			before := manifest(t, p1)
			bumps, err := p1.Upgrade(ctx, (&registry.Client{URL: srv.URL}).Latest)
			if err != nil {
				t.Fatal(err)
			}
			if want := []project.Change{{Name: "lib", From: "1.0.0", To: "2.0.0"}}; !reflect.DeepEqual(bumps, want) {
				t.Errorf("Upgrade() = %+v, want %+v", bumps, want)
			}
			pj, err = pkgjson.Read(filepath.Join(p1.Dir, "package.json"))
			if err != nil {
				t.Fatal(err)
			}
			if got, _ := pj.String("dependencies", "lib"); got != "^2.0.0" {
				t.Errorf("package.json dependencies.lib = %q, want ^2.0.0", got)
			}
			if _, ok := pj.String(path...); ok {
				t.Errorf("package.json %s survived Upgrade", strings.Join(path, "."))
			}
			after := manifest(t, p1)
			if after.Aligned != nil {
				t.Errorf("manifest Aligned after Upgrade = %v, want none", after.Aligned)
			}
			wantChanges := []project.Change{
				{Name: "dep", From: "1.1.0", To: "1.2.0"},
				{Name: "lib", From: "1.0.0", To: "2.0.0"},
			}
			if got := project.Changes(before, after); !reflect.DeepEqual(got, wantChanges) {
				t.Errorf("Changes() = %+v, want %+v", got, wantChanges)
			}
			js, err := os.ReadFile(filepath.Join(p1.OutDir(), "l/l.js"))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(js), "dep 1.2.0") {
				t.Errorf("l.js does not bundle dep 1.2.0:\n%s", js)
			}
			assertCurrent(t, p1)
		})
	}
}
