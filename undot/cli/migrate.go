package cli

import (
	"solod.dev/so/fmt"
	"solod.dev/so/mem"
	"solod.dev/so/os"
	"solod.dev/so/strings"
)

// hardProtected entries make the symlink scheme itself work: git reads
// them directly from the worktree before anything else can run. They are
// never migratable.
var hardProtected = []string{
	".git",
	".gitignore",
	".gitattributes",
	".gitmodules",
	".config",
}

// guardedDefaults bootstrap the repository on a fresh clone before any
// hook can recreate symlinks: pre-commit needs its config at the root to
// run `undot link` at all, and GitHub reads .github/ from the tree.
// Migrating one requires naming it explicitly AND passing --force.
var guardedDefaults = []string{
	".github",
	".pre-commit-config.yaml",
	".pre-commit-config.yml",
	".pre-commit-hooks.yaml",
}

// skipByDefault holds entries the automatic scan leaves alone: machine
// state and secrets rather than shareable configuration, plus configs that
// CI services read from hook-less checkouts (a migrated .goreleaser.yaml
// is invisible to goreleaser-action unless the workflow runs
// `undot link`). Naming one explicitly (`undot migrate .env`)
// still migrates it, and a undot.conf rule like "!.env" opts it back
// into the scan.
var skipByDefault = []string{
	".goreleaser.yaml",
	".goreleaser.yml",
	".golangci.yaml",
	".golangci.yml",
	".codecov.yml",
	".readthedocs.yaml",
	".env",
	".venv",
	".direnv",
	".DS_Store",
	".cache",
	".mypy_cache",
	".pytest_cache",
	".ruff_cache",
	".tox",
	".nox",
	".coverage",
	".hypothesis",
	".terraform",
	".gradle",
	".next",
	".nuxt",
	".turbo",
	".parcel-cache",
	".angular",
	".dart_tool",
	".eggs",
	".ipynb_checkpoints",
	".sass-cache",
	".history",
	".vs",
	".swiftpm",
	".build",
}

type moveResult int

const (
	moveMoved moveResult = iota
	moveSkipped
	moveFailed
)

// cmdMigrate moves root config entries into .config/, links them back,
// gitignores the links, and registers the pre-commit hook. With explicit
// entries it migrates exactly those; otherwise it scans the root for
// dotfiles that aren't protected, skipped, or already symlinks.
func cmdMigrate(a mem.Allocator, explicit []string, dryRun bool, force bool) int {
	loadIgnoreRules(a)
	if force && len(explicit) == 0 {
		fmt.Fprintf(os.Stderr, "undot: --force requires naming the entries to migrate\n")
		return 2
	}
	fi, err := os.Lstat(configDir)
	if err != nil {
		if dryRun {
			fmt.Printf("undot: [dry-run] would create %s/\n", configDir)
		} else {
			if merr := os.Mkdir(configDir, 0o755); merr != nil {
				fmt.Fprintf(os.Stderr, "undot: cannot create %s: %s\n", configDir, merr.Error())
				return 1
			}
			fmt.Printf("undot: created %s/\n", configDir)
		}
	} else if !fi.IsDir() {
		fmt.Fprintf(os.Stderr, "undot: %s exists but is not a directory\n", configDir)
		return 1
	}

	var migrated [maxEntries]string
	nMigrated := 0
	fails := 0

	if len(explicit) > 0 {
		for _, name := range explicit {
			r := migrateEntry(name, dryRun, true, force)
			if r == moveFailed {
				fails++
			} else if r == moveMoved && nMigrated < maxEntries {
				migrated[nMigrated] = name
				nMigrated++
			}
		}
	} else {
		entries, rerr := os.ReadDir(a, ".")
		if rerr != nil {
			fmt.Fprintf(os.Stderr, "undot: cannot read repository root: %s\n", rerr.Error())
			return 1
		}
		for _, e := range entries {
			if !strings.HasPrefix(e.Name, ".") {
				continue
			}
			if e.Type&os.ModeSymlink != 0 {
				continue
			}
			if containsString(hardProtected, e.Name) || containsString(guardedDefaults, e.Name) {
				continue
			}
			if isIgnored(e.Name) {
				continue
			}
			r := migrateEntry(e.Name, dryRun, false, false)
			if r == moveFailed {
				fails++
			} else if r == moveMoved && nMigrated < maxEntries {
				migrated[nMigrated] = e.Name
				nMigrated++
			}
		}
	}

	// The ignore and link passes cover everything in .config/, not just this
	// run's moves, so a partially migrated repo self-heals. In dry-run mode
	// the moves didn't happen, so add them by hand.
	var names [maxEntries]string
	nNames := 0
	entries, rerr := os.ReadDir(a, configDir)
	if rerr == nil {
		for _, e := range entries {
			if e.Name == confFileName || isNolink(e.Name) {
				if !removeNolinkLink(e.Name, dryRun) {
					fails++
				}
				continue
			}
			if nNames < maxEntries {
				names[nNames] = e.Name
				nNames++
			}
		}
	} else if !dryRun {
		fmt.Fprintf(os.Stderr, "undot: cannot read %s: %s\n", configDir, rerr.Error())
		return 1
	}
	if dryRun {
		for i := range nMigrated {
			if isNolink(migrated[i]) {
				continue
			}
			if !containsString(names[:nNames], migrated[i]) && nNames < maxEntries {
				names[nNames] = migrated[i]
				nNames++
			}
		}
	}

	if nNames == 0 && nMigrated == 0 {
		if fails > 0 {
			fmt.Fprintf(os.Stderr, "undot: completed with %d problems\n", fails)
			return 1
		}
		fmt.Printf("undot: nothing to migrate\n")
		return 0
	}

	if ensureGitignore(a, names[:nNames], dryRun) < 0 {
		fails++
	}
	for i := range nNames {
		// In a dry run the moves above didn't happen, so entries planned for
		// migration still exist at the root; a real run links them after the
		// move, so don't report them as conflicts here.
		if dryRun && containsString(migrated[:nMigrated], names[i]) {
			fmt.Printf("undot: [dry-run] would link %s -> %s/%s\n", names[i], configDir, names[i])
			continue
		}
		r := ensureLink(names[i], dryRun)
		if r == linkConflict || r == linkError {
			fails++
		}
	}
	if ensurePreCommitConfig(a, dryRun) < 0 {
		fails++
	}

	if fails > 0 {
		fmt.Fprintf(os.Stderr, "undot: completed with %d problems\n", fails)
		return 1
	}
	if dryRun {
		fmt.Printf("undot: dry run complete; re-run without --dry-run to apply\n")
		return 0
	}
	// git keeps tracking a previously tracked FILE at its root path even
	// though the path is now an ignored symlink (ignore patterns don't
	// apply to tracked paths); directories are fine because their tracked
	// paths moved away. Point at the affected entries.
	movedFileHint(nMigrated, migrated[:])
	fmt.Print(`undot: done. Suggested follow-up:
  git add -A && git commit
  pre-commit install  # installs post-checkout/post-merge hooks too
`)
	return 0
}

func movedFileHint(nMigrated int, migrated []string) {
	printed := false
	for i := range nMigrated {
		dest := configDir + "/" + migrated[i]
		fi, err := os.Lstat(dest)
		if err != nil || fi.IsDir() {
			continue
		}
		if !printed {
			fmt.Printf("undot: if these files were tracked by git, also untrack the root symlinks:\n")
			printed = true
		}
		fmt.Printf("  git rm --cached %s\n", migrated[i])
	}
}

// migrateEntry moves one root entry into .config/. The symlink back is
// created later by the shared link pass.
func migrateEntry(name string, dryRun bool, explicit bool, force bool) moveResult {
	_ = explicit
	if name == "." || name == ".." || strings.IndexByte(name, '/') >= 0 {
		fmt.Fprintf(os.Stderr, "undot: %s is not a root-level entry name\n", name)
		return moveFailed
	}
	if name == confFileName {
		fmt.Fprintf(os.Stderr, "undot: %s is undot's own configuration; skipping\n", name)
		return moveFailed
	}
	if containsString(hardProtected, name) {
		fmt.Fprintf(os.Stderr, "undot: %s is required at the repository root and cannot be migrated\n", name)
		return moveFailed
	}
	if containsString(guardedDefaults, name) && !force {
		fmt.Fprintf(os.Stderr, "undot: %s bootstraps the repository before any hook can run; pass --force to migrate it anyway\n", name)
		return moveFailed
	}
	fi, err := os.Lstat(name)
	if err != nil {
		if explicit {
			fmt.Fprintf(os.Stderr, "undot: %s not found\n", name)
			return moveFailed
		}
		return moveSkipped
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return moveSkipped
	}
	dest := configDir + "/" + name
	if _, derr := os.Lstat(dest); derr == nil {
		fmt.Fprintf(os.Stderr, "undot: both %s and %s exist; resolve manually\n", name, dest)
		return moveFailed
	}
	if dryRun {
		fmt.Printf("undot: [dry-run] would move %s -> %s\n", name, dest)
		return moveMoved
	}
	if rerr := os.Rename(name, dest); rerr != nil {
		fmt.Fprintf(os.Stderr, "undot: cannot move %s -> %s: %s\n", name, dest, rerr.Error())
		return moveFailed
	}
	fmt.Printf("undot: moved %s -> %s\n", name, dest)
	return moveMoved
}
