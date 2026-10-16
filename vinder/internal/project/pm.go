package project

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var (
	// ErrNoPackageManager is returned for a project that neither declares a
	// package manager nor has a lockfile that names one.
	ErrNoPackageManager = errors.New(`no package manager: set "packageManager" in package.json, or commit a lockfile`)
	// ErrAmbiguousPackageManager is returned for a project with lockfiles
	// from more than one package manager and nothing to choose between them.
	ErrAmbiguousPackageManager = errors.New(`lockfiles from more than one package manager: set "packageManager" in package.json to choose`)
	// ErrUnknownPackageManager is returned for a declared package manager
	// vinder does not know how to drive.
	ErrUnknownPackageManager = errors.New("unsupported package manager")
	// ErrNoPackageManagerBinary is returned when the project's package
	// manager is not on PATH.
	ErrNoPackageManagerBinary = errors.New("package manager not found on PATH")
)

// PackageManager is the tool that installs a project's node_modules.
type PackageManager struct {
	// Name is the command: bun, npm, nub, pnpm or yarn.
	Name string
	// Lockfiles are the lockfiles it writes, any one of which pins the
	// project's versions.
	Lockfiles []string
	// berry is set for Yarn 2 and later, whose install flags differ from
	// Yarn 1's.
	berry bool
}

// managers lists the supported package managers and their lockfiles, in
// the order lockfile detection tries them.
var managers = []PackageManager{
	{Name: "bun", Lockfiles: []string{"bun.lock", "bun.lockb"}},
	{Name: "nub", Lockfiles: []string{"nub.lock"}},
	{Name: "pnpm", Lockfiles: []string{"pnpm-lock.yaml"}},
	{Name: "npm", Lockfiles: []string{"package-lock.json", "npm-shrinkwrap.json"}},
	{Name: "yarn", Lockfiles: []string{"yarn.lock"}},
}

// DetectPackageManager works out which package manager a project uses the
// way corepack and its successors do: the packageManager field of
// package.json (or devEngines.packageManager) when there is one, and
// otherwise the one lockfile present.
func DetectPackageManager(dir string) (PackageManager, error) {
	data, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return PackageManager{}, err
	}
	var pj struct {
		DevEngines struct {
			PackageManager json.RawMessage `json:"packageManager"`
		} `json:"devEngines"`
		PackageManager string `json:"packageManager"`
	}
	if err := json.Unmarshal(data, &pj); err != nil {
		return PackageManager{}, fmt.Errorf("%s: %w", filepath.Join(dir, "package.json"), err)
	}

	declared := pj.PackageManager
	if declared == "" && len(pj.DevEngines.PackageManager) > 0 {
		// One {name, version} object, or a list of them in order of
		// preference.
		var one struct{ Name, Version string }
		var many []struct{ Name, Version string }
		if json.Unmarshal(pj.DevEngines.PackageManager, &one) == nil && one.Name != "" {
			declared = one.Name + "@" + one.Version
		} else if json.Unmarshal(pj.DevEngines.PackageManager, &many) == nil && len(many) > 0 {
			declared = many[0].Name + "@" + many[0].Version
		}
	}
	if declared != "" {
		// "pnpm@9.1.0+sha512.…", or a bare name.
		name, version, _ := strings.Cut(declared, "@")
		for _, pm := range managers {
			if pm.Name != name {
				continue
			}
			if name == "yarn" && version != "" {
				pm.berry = !strings.HasPrefix(version, "1.")
			} else if name == "yarn" {
				pm.berry = exists(filepath.Join(dir, ".yarnrc.yml"))
			}
			return pm, nil
		}
		return PackageManager{}, fmt.Errorf("%w: %q", ErrUnknownPackageManager, declared)
	}

	var found []PackageManager
	for _, pm := range managers {
		if pm.lockfile(dir) != "" {
			if pm.Name == "yarn" {
				pm.berry = exists(filepath.Join(dir, ".yarnrc.yml"))
			}
			found = append(found, pm)
		}
	}
	switch len(found) {
	case 0:
		return PackageManager{}, ErrNoPackageManager
	case 1:
		return found[0], nil
	}
	names := make([]string, len(found))
	for i, pm := range found {
		names[i] = pm.Name
	}
	return PackageManager{}, fmt.Errorf("%w (%s)", ErrAmbiguousPackageManager, strings.Join(names, ", "))
}

// lockfile returns the name of the package manager's lockfile in dir, or
// "" if there is none.
func (pm PackageManager) lockfile(dir string) string {
	for _, name := range pm.Lockfiles {
		if exists(filepath.Join(dir, name)) {
			return name
		}
	}
	return ""
}

// install populates dir/node_modules without running any package's
// lifecycle scripts. With frozen set it fails rather than change the
// lockfile.
func (pm PackageManager) install(ctx context.Context, dir string, frozen bool) error {
	bin, err := exec.LookPath(pm.Name)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrNoPackageManagerBinary, pm.Name)
	}
	var args, env []string
	switch {
	case pm.Name == "npm" && frozen:
		args = []string{"ci", "--ignore-scripts", "--no-audit", "--no-fund"}
	case pm.Name == "npm":
		args = []string{"install", "--ignore-scripts", "--no-audit", "--no-fund"}
	case pm.berry:
		// Yarn 2+ has no --ignore-scripts, and by default installs Plug'n'Play
		// with no node_modules to bundle from.
		args = []string{"install"}
		if frozen {
			args = append(args, "--immutable")
		}
		env = []string{"YARN_ENABLE_SCRIPTS=false", "YARN_NODE_LINKER=node-modules"}
	default:
		// bun, nub, pnpm and Yarn 1 spell it the same way.
		args = []string{"install", "--ignore-scripts"}
		if frozen {
			args = append(args, "--frozen-lockfile")
		}
		// Bun trusts its cached package metadata for minutes, so a resolve
		// could miss a release the registry already has, one upgrade has
		// just asked for. --no-cache skips only that metadata; tarballs
		// still come from the cache.
		if pm.Name == "bun" && !frozen {
			args = append(args, "--no-cache")
		}
	}

	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %w\n%s", pm.Name, strings.Join(args, " "), err, bytes.TrimSpace(out.Bytes()))
	}
	return nil
}

// packageManager detects the project's package manager the first time it
// is asked for, and returns the same one after.
func (p *Project) packageManager() (PackageManager, error) {
	if p.pm == nil {
		pm, err := DetectPackageManager(p.Dir)
		if err != nil {
			return PackageManager{}, err
		}
		p.pm = &pm
	}
	return *p.pm, nil
}

func exists(file string) bool {
	_, err := os.Stat(file)
	return err == nil
}
