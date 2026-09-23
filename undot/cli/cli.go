// Package cli implements the undot command-line interface.
//
// undot keeps a repository's root directory free of tool configuration
// clutter: config files and directories (.claude/, .cursor/, .yarn/, ...) are
// stored under .config/ and symlinked back into the root. The symlinks are
// gitignored and recreated on checkout by the `undot link` pre-commit
// hook, so only .config/ is tracked.
package cli

import (
	"solod.dev/so/fmt"
	"solod.dev/so/mem"
	"solod.dev/so/os"
)

// Version is the undot release version.
const Version = "0.3.0"

// configDir is where configuration entries live, relative to the repo root.
const configDir = ".config"

// Package-level string constants can't be built by concatenation: the
// transpiler emits `+` as a runtime operation, which C static initializers
// don't allow. Keep them as single literals.
const usageText = `undot: declutter a repository root by storing configs in .config/
and symlinking them back into the root.

Usage:
  undot link                Create root symlinks for every entry in .config/.
                            Safe to re-run; intended as a post-checkout hook.
  undot migrate [entry...]  Move root config entries into .config/, link them
                            back, gitignore the links, and register the
                            pre-commit hook. With no arguments, migrates all
                            root dotfiles except git-owned and cache entries.
  undot version             Print the version.
  undot help                Print this help.

Flags:
  -n, --dry-run             Print planned actions without changing anything.
  -f, --force               With explicit entries: migrate bootstrap files that
                            are guarded by default (.github, .pre-commit-config.yaml).

Configuration:
  .config/undot.conf (repository) and
  ${XDG_CONFIG_HOME:-~/.config}/undot/undot.conf (user) list root-level
  names or globs the automatic scan should ignore, one per line.
  "!name" opts an entry back in (overrides earlier rules and built-in
  defaults; the last matching rule wins). "#" starts a comment.
`

// arenaBuf backs every allocation the program makes; freed wholesale on exit.
var arenaBuf [1 << 20]byte

// Run executes the CLI and returns the process exit code.
func Run(args []string) int {
	arena := mem.NewArena(arenaBuf[:])
	if len(args) < 2 {
		fmt.Print(usageText)
		return 2
	}
	cmd := args[1]
	dryRun := false
	force := false
	var extras [maxEntries]string
	nExtras := 0
	for _, arg := range args[2:] {
		if arg == "--dry-run" || arg == "-n" {
			dryRun = true
			continue
		}
		if arg == "--force" || arg == "-f" {
			force = true
			continue
		}
		if len(arg) > 0 && arg[0] == '-' {
			fmt.Fprintf(os.Stderr, "undot: unknown flag %s\n", arg)
			return 2
		}
		if nExtras >= maxEntries {
			fmt.Fprintf(os.Stderr, "undot: too many arguments\n")
			return 2
		}
		extras[nExtras] = cleanEntryArg(arg)
		nExtras++
	}

	switch cmd {
	case "help", "--help", "-h":
		fmt.Print(usageText)
		return 0
	case "version", "--version", "-V":
		fmt.Printf("undot %s\n", Version)
		return 0
	case "link":
		if err := chdirRepoRoot(); err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err.Error())
			return 1
		}
		return cmdLink(&arena, dryRun)
	case "migrate":
		if err := chdirRepoRoot(); err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err.Error())
			return 1
		}
		return cmdMigrate(&arena, extras[:nExtras], dryRun, force)
	}
	fmt.Fprintf(os.Stderr, "undot: unknown command %s (try `undot help`)\n", cmd)
	return 2
}
