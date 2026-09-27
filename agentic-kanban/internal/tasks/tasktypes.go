package tasks

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// resolveCommand turns a tasks.json entry into the command line to run.
// "shell"/"process" tasks (and untyped ones) carry it in command/args; the
// task-provider types VS Code extensions contribute instead describe what to
// run (an npm script, a gulp task, a cargo subcommand), so rebuild the command
// the provider would have. ok is false for entries with nothing runnable,
// like a dependsOn-only compound task. worktreePath is the host-side root the
// tasks.json was read from, used to sniff the package manager.
func resolveCommand(t vsTaskRaw, worktreePath string) (command string, args []string, cwd string, ok bool, err error) {
	cwd = t.Options.Cwd
	switch t.Type {
	case "", "shell", "process":
		return t.Command, t.Args, cwd, t.Command != "", nil
	case "npm":
		// VS Code's npm provider runs `<pm> run <script>` from `path`, the
		// package.json's directory relative to the workspace folder.
		if t.Script == "" {
			return "", nil, "", false, errors.New("npm task has no script")
		}
		if cwd == "" {
			cwd = t.Path
		}
		// Sniff from where the script actually runs, which options.cwd can
		// move away from path.
		pm := detectPackageManager(worktreePath, cwd)
		if t.Script == "install" {
			return pm, append([]string{"install"}, t.Args...), cwd, true, nil
		}
		return pm, append([]string{"run", t.Script}, t.Args...), cwd, true, nil
	case "typescript":
		if t.Tsconfig == "" {
			return "", nil, "", false, errors.New("typescript task has no tsconfig")
		}
		args = []string{"tsc", "-p", t.Tsconfig}
		if t.Option == "watch" {
			args = append(args, "--watch")
		}
		return "npx", args, cwd, true, nil
	case "gulp", "grunt", "jake":
		// Node task runners, resolved from node_modules/.bin like VS Code does.
		if t.Task == "" {
			return "", nil, "", false, fmt.Errorf("%s task has no task", t.Type)
		}
		return "npx", append([]string{t.Type, t.Task}, t.Args...), cwd, true, nil
	case "rake":
		if t.Task == "" {
			return "", nil, "", false, errors.New("rake task has no task")
		}
		return "rake", append([]string{t.Task}, t.Args...), cwd, true, nil
	case "cargo", "go", "deno":
		// rust-analyzer, vscode-go and the Deno extension put the subcommand
		// in command (e.g. "build"), not the full command line.
		if t.Command == "" {
			return "", nil, "", false, fmt.Errorf("%s task has no command", t.Type)
		}
		return t.Type, append([]string{t.Command}, t.Args...), cwd, true, nil
	default:
		return "", nil, "", false, fmt.Errorf("unsupported type %q", t.Type)
	}
}

// lockfiles maps each package manager to the lockfiles that identify it,
// checked in order.
var lockfiles = []struct{ pm, file string }{
	{"bun", "bun.lock"},
	{"bun", "bun.lockb"},
	{"pnpm", "pnpm-lock.yaml"},
	{"yarn", "yarn.lock"},
	{"npm", "package-lock.json"},
}

// detectPackageManager picks the package manager for the package.json at
// worktreePath/rel, walking up to worktreePath so a workspace package finds
// the monorepo root's lockfile. A lockfile wins over package.json's
// "packageManager" field; with neither, it falls back to npm.
func detectPackageManager(worktreePath, rel string) string {
	root := filepath.Clean(worktreePath)
	dir := root
	if rel != "" && !filepath.IsAbs(rel) && !strings.Contains(rel, "${") {
		dir = filepath.Join(root, rel)
	}
	for {
		for _, l := range lockfiles {
			if _, err := os.Stat(filepath.Join(dir, l.file)); err == nil {
				return l.pm
			}
		}
		if pm := packageManagerField(dir); pm != "" {
			return pm
		}
		if dir == root || !strings.HasPrefix(dir, root+string(filepath.Separator)) {
			return "npm"
		}
		dir = filepath.Dir(dir)
	}
}

// packageManagerField reads the Corepack "packageManager" field (e.g.
// "pnpm@9.1.0") from dir/package.json.
func packageManagerField(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return ""
	}
	var pkg struct {
		PackageManager string `json:"packageManager"`
	}
	if json.Unmarshal(data, &pkg) != nil {
		return ""
	}
	name, _, _ := strings.Cut(pkg.PackageManager, "@")
	switch name {
	case "npm", "yarn", "pnpm", "bun":
		return name
	}
	return ""
}

// defaultLabel names a typed task that has no label the way VS Code shows it
// in the Run Task picker (e.g. "npm: dev", "tsc: watch - tsconfig.json").
func defaultLabel(t vsTaskRaw) string {
	switch t.Type {
	case "npm":
		if t.Script == "" {
			return ""
		}
		if t.Path != "" {
			return fmt.Sprintf("npm: %s - %s", t.Script, strings.TrimSuffix(t.Path, "/"))
		}
		return "npm: " + t.Script
	case "typescript":
		if t.Tsconfig == "" {
			return ""
		}
		mode := "build"
		if t.Option == "watch" {
			mode = "watch"
		}
		return fmt.Sprintf("tsc: %s - %s", mode, t.Tsconfig)
	case "gulp", "grunt", "jake", "rake":
		if t.Task == "" {
			return ""
		}
		return fmt.Sprintf("%s: %s", t.Type, t.Task)
	case "cargo", "go", "deno":
		if t.Command == "" {
			return ""
		}
		return fmt.Sprintf("%s: %s", t.Type, t.Command)
	}
	return ""
}
