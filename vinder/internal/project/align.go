package project

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"

	"github.com/Masterminds/semver/v3"

	"github.com/jmelahman/vinder/internal/pkgjson"
)

// dependencyFields are the package.json fields that declare the project's
// own dependencies.
var dependencyFields = []string{"dependencies", "devDependencies", "optionalDependencies"}

// plainSpec matches a dependency spec vinder will rewrite: an exact
// version, optionally behind ^ or ~. Anything else (ranges, tags, file:,
// git and npm: specs) is left to the person who wrote it.
var plainSpec = regexp.MustCompile(`^([~^]?)(\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?)$`)

// overridesPath is where the package manager reads overrides from in
// package.json: pnpm keeps them under its own key and Yarn calls them
// resolutions. Nub, outside a pnpm project, reads the top-level overrides
// like npm and bun.
func (pm PackageManager) overridesPath() []string {
	switch pm.Name {
	case "pnpm":
		return []string{"pnpm", "overrides"}
	case "yarn":
		return []string{"resolutions"}
	}
	return []string{"overrides"}
}

// aligned returns the overrides vinder added in an earlier alignment that
// package.json still has, unchanged.
func (p *Project) aligned(pm PackageManager) (map[string]string, error) {
	old, err := ReadManifest(filepath.Join(p.OutDir(), ManifestFile))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil //nolint:nilnil // nil map means "no prior alignment"
	}
	if err != nil {
		return nil, err
	}
	pj, err := pkgjson.Read(filepath.Join(p.Dir, "package.json"))
	if err != nil {
		return nil, err
	}
	overrides := pj.Strings(pm.overridesPath()...)
	var aligned map[string]string
	for name, version := range old.Aligned {
		if overrides[name] == version {
			if aligned == nil {
				aligned = map[string]string{}
			}
			aligned[name] = version
		}
	}
	return aligned, nil
}

// rootRequires returns the ranges the project's package.json asks for: its
// dependencies, and its own overrides as exact requirements. Overrides from
// an earlier alignment are not the project's own and are left out, so a
// new alignment can move them.
func (p *Project) rootRequires(pm PackageManager, aligned map[string]string) (map[string][]string, error) {
	pj, err := pkgjson.Read(filepath.Join(p.Dir, "package.json"))
	if err != nil {
		return nil, err
	}
	requires := map[string][]string{}
	for _, field := range dependencyFields {
		for name, r := range pj.Strings(field) {
			requires[name] = append(requires[name], r)
		}
	}
	for name, r := range pj.Strings(pm.overridesPath()...) {
		if _, ok := aligned[name]; !ok {
			requires[name] = append(requires[name], r)
		}
	}
	return requires, nil
}

// Alignment is what Align did about one package vendored at several
// versions.
type Alignment struct {
	// Moved maps each project directory moved to Version to the version it
	// was on.
	Moved map[string]string
	Name  string
	// Version is the one the projects now share, or "" if no one version
	// satisfies them all and they were left as they were.
	Version string
}

// Align brings projects onto one version of every package more than one of
// them vendors. It syncs each project that has not been synced already,
// as upgrade's have, and for each package vendored at
// several versions picks the highest of them that satisfies every range
// asked of it, in every project. A project on another version is moved to
// it: by its dependency spec if the package is a direct dependency, and
// otherwise by an override, which the manifest records as vinder's. The
// projects it changed are synced again.
func Align(ctx context.Context, projects []*Project) ([]Alignment, error) {
	for _, p := range projects {
		if p.synced {
			continue
		}
		if err := p.Sync(ctx); err != nil {
			return nil, fmt.Errorf("%s: %w", p.Dir, err)
		}
	}
	skews, err := FindSkew(projects)
	if err != nil {
		return nil, err
	}

	byDir := map[string]*Project{}
	for _, p := range projects {
		byDir[p.Dir] = p
	}
	var report []Alignment
	changed := map[*Project]bool{}
	for _, s := range skews {
		target := pickVersion(s, byDir)
		a := Alignment{Name: s.Name, Version: target, Moved: map[string]string{}}
		report = append(report, a)
		if target == "" {
			continue
		}
		for _, version := range sortedKeys(s.Versions) {
			if version == target {
				continue
			}
			for _, dir := range s.Versions[version] {
				p := byDir[dir]
				if err := p.pin(s.Name, version, target); err != nil {
					return nil, fmt.Errorf("%s: %w", p.Dir, err)
				}
				changed[p] = true
				a.Moved[dir] = version
			}
		}
	}

	for _, p := range projects {
		if !changed[p] {
			continue
		}
		if err := p.Sync(ctx); err != nil {
			return nil, fmt.Errorf("%s: %w", p.Dir, err)
		}
	}
	return report, nil
}

// pickVersion returns the highest of the versions in s that every range
// asked of the package satisfies, in every project vendoring it, or "". A
// declared embed is code inlined in another package's files, which no spec
// or override can move, so it counts as an exact requirement.
func pickVersion(s Skew, byDir map[string]*Project) string {
	var candidates []*semver.Version
	for v := range s.Versions {
		if sv, err := semver.NewVersion(v); err == nil {
			candidates = append(candidates, sv)
		}
	}
	slices.SortFunc(candidates, func(a, b *semver.Version) int { return b.Compare(a) })

	var constraints []*semver.Constraints
	for _, dirs := range s.Versions {
		for _, dir := range dirs {
			for _, r := range byDir[dir].requires[s.Name] {
				// Specs that are not ranges (file:, git, npm: aliases, tags)
				// say nothing about versions.
				if c, err := semver.NewConstraint(r); err == nil {
					constraints = append(constraints, c)
				}
			}
			for _, dep := range byDir[dir].Config.Deps {
				for _, e := range dep.Embeds {
					pkg, _ := parseNameVersion(e) // validated by Load
					if pkg.Name != s.Name {
						continue
					}
					if c, err := semver.NewConstraint("=" + pkg.Version); err == nil {
						constraints = append(constraints, c)
					}
				}
			}
		}
	}
	for _, v := range candidates {
		if !slices.ContainsFunc(constraints, func(c *semver.Constraints) bool { return !c.Check(v) }) {
			return v.Original()
		}
	}
	return ""
}

// pin moves the project from one version of name to another: through its
// dependency spec if it is a direct dependency, keeping the spec's ^ or ~
// unless the range would still admit from, which the lockfile would then
// keep; and otherwise with an override recorded in the manifest as
// vinder's.
func (p *Project) pin(name, from, version string) error {
	pm, err := p.packageManager()
	if err != nil {
		return err
	}
	pj, err := pkgjson.Read(filepath.Join(p.Dir, "package.json"))
	if err != nil {
		return err
	}
	for _, field := range dependencyFields {
		spec, ok := pj.String(field, name)
		if !ok {
			continue
		}
		prefix := ""
		if m := plainSpec.FindStringSubmatch(spec); m != nil {
			prefix = m[1]
		}
		if prefix != "" {
			c, err := semver.NewConstraint(prefix + version)
			v, verr := semver.NewVersion(from)
			if err == nil && verr == nil && c.Check(v) {
				prefix = ""
			}
		}
		if err := pj.SetString(prefix+version, field, name); err != nil {
			return err
		}
		return pj.Write()
	}

	if err := pj.SetString(version, append(pm.overridesPath(), name)...); err != nil {
		return err
	}
	if err := pj.Write(); err != nil {
		return err
	}
	manifest := filepath.Join(p.OutDir(), ManifestFile)
	m, err := ReadManifest(manifest)
	if err != nil {
		return err
	}
	if m.Aligned == nil {
		m.Aligned = map[string]string{}
	}
	m.Aligned[name] = version
	return writeManifest(m, manifest)
}
