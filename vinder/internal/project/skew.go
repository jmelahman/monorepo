package project

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
)

// Skew is a package vendored at more than one version across projects.
type Skew struct {
	// Versions maps each version to the directories of the projects that
	// vendor it, sorted.
	Versions map[string][]string
	Name     string
}

// FindSkew reads the manifests of projects and returns, sorted by name,
// every package they vendor at more than one version, counting what each
// bundle pulled in and the declared embeds as well as the vendored packages
// themselves. A project without a manifest is skipped; Check reports it.
func FindSkew(projects []*Project) ([]Skew, error) {
	seen := map[string]map[string]map[string]bool{} // name -> version -> project dirs
	for _, p := range projects {
		m, err := ReadManifest(filepath.Join(p.OutDir(), ManifestFile))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		for _, key := range p.Keys() {
			for _, pkg := range m.Deps[key].Includes {
				if seen[pkg.Name] == nil {
					seen[pkg.Name] = map[string]map[string]bool{}
				}
				if seen[pkg.Name][pkg.Version] == nil {
					seen[pkg.Name][pkg.Version] = map[string]bool{}
				}
				seen[pkg.Name][pkg.Version][p.Dir] = true
			}
		}
	}

	var skews []Skew
	for _, name := range sortedKeys(seen) {
		if len(seen[name]) < 2 {
			continue
		}
		s := Skew{Name: name, Versions: map[string][]string{}}
		for version, dirs := range seen[name] {
			s.Versions[version] = sortedKeys(dirs)
		}
		skews = append(skews, s)
	}
	sort.Slice(skews, func(i, j int) bool { return skews[i].Name < skews[j].Name })
	return skews, nil
}
