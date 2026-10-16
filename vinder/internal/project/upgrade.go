package project

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Masterminds/semver/v3"

	"github.com/jmelahman/vinder/internal/pkgjson"
)

// Upgrade moves the project to the newest releases it can take. Every
// direct dependency with a plain spec (1.2.3, ^1.2.3 or ~1.2.3) is raised to
// the version latest returns for it, keeping its ^ or ~; the overrides an
// earlier alignment added are dropped, so the next one starts afresh; and
// the lockfile is resolved again from nothing, which moves every transitive
// dependency to the newest release its ranges allow. Then it syncs. If any
// of that fails, package.json and the lockfile are put back as they were.
func (p *Project) Upgrade(ctx context.Context, latest func(ctx context.Context, name string) (string, error)) (bumps []Change, err error) {
	pm, err := p.packageManager()
	if err != nil {
		return nil, err
	}
	pjFile := filepath.Join(p.Dir, "package.json")
	pjData, err := os.ReadFile(pjFile)
	if err != nil {
		return nil, err
	}
	var lockFile string
	var lockData []byte
	if name := pm.lockfile(p.Dir); name != "" {
		lockFile = filepath.Join(p.Dir, name)
		if lockData, err = os.ReadFile(lockFile); err != nil {
			return nil, err
		}
	}
	defer func() {
		if err == nil {
			return
		}
		restore := os.WriteFile(pjFile, pjData, 0o644)
		if lockFile != "" {
			restore = errors.Join(restore, os.WriteFile(lockFile, lockData, 0o644))
		}
		if restore != nil {
			err = errors.Join(err, fmt.Errorf("restoring package.json and the lockfile: %w", restore))
		}
	}()

	pj, err := pkgjson.Read(pjFile)
	if err != nil {
		return nil, err
	}
	for _, field := range dependencyFields {
		for _, name := range sortedKeys(pj.Strings(field)) {
			spec, _ := pj.String(field, name)
			m := plainSpec.FindStringSubmatch(spec)
			if m == nil {
				continue
			}
			to, err := latest(ctx, name)
			if err != nil {
				return nil, err
			}
			cur, err1 := semver.NewVersion(m[2])
			next, err2 := semver.NewVersion(to)
			if err1 != nil || err2 != nil || !next.GreaterThan(cur) {
				continue
			}
			if err := pj.SetString(m[1]+to, field, name); err != nil {
				return nil, err
			}
			bumps = append(bumps, Change{Name: name, From: m[2], To: to})
		}
	}

	aligned, err := p.aligned(pm)
	if errors.Is(err, ErrBadManifest) {
		// Upgrade regenerates the manifest; without it, the aligned
		// overrides count as the project's own.
		aligned, err = nil, nil
	}
	if err != nil {
		return nil, err
	}
	for name := range aligned {
		if err := pj.Delete(append(pm.overridesPath(), name)...); err != nil {
			return nil, err
		}
	}
	if err := pj.Write(); err != nil {
		return nil, err
	}

	if lockFile != "" {
		if err := os.Remove(lockFile); err != nil {
			return nil, err
		}
	}
	if err := p.Sync(ctx); err != nil {
		return nil, err
	}
	return bumps, nil
}

// Change is a package that moved from one version to another: a direct
// dependency Upgrade bumped, or a vendored package two manifests disagree
// on. From is "" for a package that is new, and To "" for one that is
// gone.
type Change struct {
	Name string
	From string
	To   string
}

// Changes compares what two manifests vendor, across all their deps. A nil
// manifest vendors nothing.
func Changes(old, cur *Manifest) []Change {
	before, after := versions(old), versions(cur)
	names := map[string]bool{}
	for name := range before {
		names[name] = true
	}
	for name := range after {
		names[name] = true
	}
	var changes []Change
	for _, name := range sortedKeys(names) {
		from, to := before[name], after[name]
		if from != to {
			changes = append(changes, Change{Name: name, From: from, To: to})
		}
	}
	return changes
}

// versions returns each vendored package's versions, joined, by name.
func versions(m *Manifest) map[string]string {
	if m == nil {
		return nil
	}
	set := map[string]map[string]bool{}
	for _, e := range m.Deps {
		for _, pkg := range e.Includes {
			if set[pkg.Name] == nil {
				set[pkg.Name] = map[string]bool{}
			}
			set[pkg.Name][pkg.Version] = true
		}
	}
	out := map[string]string{}
	for name, vs := range set {
		out[name] = strings.Join(sortedKeys(vs), ", ")
	}
	return out
}
