package project_test

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/jmelahman/vinder/internal/project"
)

func TestFindSkew(t *testing.T) {
	t.Parallel()

	// Each project vendors one key, "ed", that includes these packages.
	tests := []struct {
		projects map[string][]project.Package
		want     map[string]map[string][]string // name -> version -> project names
		name     string
	}{
		{
			name: "same versions",
			projects: map[string][]project.Package{
				"a": {{Name: "ed", Version: "3.2.2"}, {Name: "model", Version: "1.19.0"}},
				"b": {{Name: "ed", Version: "3.2.2"}, {Name: "model", Version: "1.19.0"}},
			},
		},
		{
			name: "vendored package at two versions",
			projects: map[string][]project.Package{
				"a": {{Name: "ed", Version: "3.2.2"}},
				"b": {{Name: "ed", Version: "3.1.0"}},
				"c": {{Name: "ed", Version: "3.2.2"}},
			},
			want: map[string]map[string][]string{"ed": {"3.1.0": {"b"}, "3.2.2": {"a", "c"}}},
		},
		{
			name: "bundled transitive dependency at two versions",
			projects: map[string][]project.Package{
				"a": {{Name: "ed", Version: "3.2.2"}, {Name: "model", Version: "1.19.0"}},
				"b": {{Name: "ed", Version: "3.2.2"}, {Name: "model", Version: "1.25.12"}},
			},
			want: map[string]map[string][]string{"model": {"1.19.0": {"a"}, "1.25.12": {"b"}}},
		},
		{
			name: "project without a manifest is skipped",
			projects: map[string][]project.Package{
				"a": {{Name: "ed", Version: "3.2.2"}},
				"b": nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			var projects []*project.Project
			for name, includes := range tt.projects {
				dir := filepath.Join(root, name)
				write(t, filepath.Join(dir, "package.json"), "{}")
				write(t, filepath.Join(dir, project.ConfigFile), "[deps.ed]\npackage = \"ed\"\ncopy = { a = \"ed/a\" }\n")
				if includes != nil {
					m, err := json.Marshal(project.Manifest{Deps: map[string]project.Entry{"ed": {Package: "ed", Includes: includes}}})
					if err != nil {
						t.Fatal(err)
					}
					write(t, filepath.Join(dir, "vendor", project.ManifestFile), string(m))
				}
				p, err := project.Load(dir)
				if err != nil {
					t.Fatal(err)
				}
				projects = append(projects, p)
			}

			skews, err := project.FindSkew(projects)
			if err != nil {
				t.Fatal(err)
			}
			got := map[string]map[string][]string{}
			for _, s := range skews {
				got[s.Name] = map[string][]string{}
				for version, dirs := range s.Versions {
					for _, dir := range dirs {
						got[s.Name][version] = append(got[s.Name][version], filepath.Base(dir))
					}
				}
			}
			if tt.want == nil {
				tt.want = map[string]map[string][]string{}
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FindSkew() = %v, want %v", got, tt.want)
			}
		})
	}
}
