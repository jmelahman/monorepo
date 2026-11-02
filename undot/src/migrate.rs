use std::fs;
use std::os::unix::ffi::OsStrExt;
use std::process::ExitCode;

use crate::conf::{Conf, CONF_FILE};
use crate::gitignore::ensure_gitignore;
use crate::link::{ensure_link, remove_nolink_link};
use crate::precommit::ensure_pre_commit_config;
use crate::util::{config_path, read_names, utf8_names};
use crate::{CONFIG_DIR, EXIT_USAGE};

/// These make the symlink scheme itself work: git reads them directly from the
/// worktree before anything else can run. They are never migratable.
const HARD_PROTECTED: &[&str] = &[
    ".git",
    ".gitignore",
    ".gitattributes",
    ".gitmodules",
    ".config",
];

/// These bootstrap the repository on a fresh clone before any hook can recreate
/// symlinks: pre-commit needs its config at the root to run `undot link` at all,
/// and GitHub reads .github/ from the tree. Migrating one requires naming it
/// explicitly AND passing --force.
const GUARDED: &[&str] = &[
    ".github",
    ".pre-commit-config.yaml",
    ".pre-commit-config.yml",
    ".pre-commit-hooks.yaml",
];

/// Entries the automatic scan leaves alone: machine state and secrets rather
/// than shareable configuration, plus configs that CI services read from
/// hook-less checkouts (a migrated .goreleaser.yaml is invisible to
/// goreleaser-action unless the workflow runs `undot link`). Naming one
/// explicitly (`undot migrate .env`) still migrates it, and an undot.conf rule
/// like `!.env` opts it back into the scan.
const SKIP_BY_DEFAULT: &[&str] = &[
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
];

/// Whether the scan skips `name` absent a config rule. `.git*` covers files git
/// hosts read from hook-less checkouts (.gitlab-ci.yml) on top of the list.
pub fn skipped_by_default(name: &str) -> bool {
    name.starts_with(".git") || SKIP_BY_DEFAULT.contains(&name)
}

/// Moves root config entries into .config/, links them back, gitignores the
/// links, and registers the pre-commit hook. With explicit entries it migrates
/// exactly those; otherwise it scans the root for dotfiles that aren't
/// protected, skipped, or already symlinks.
pub fn cmd_migrate(explicit: &[String], dry_run: bool, force: bool) -> ExitCode {
    if force && explicit.is_empty() {
        complain!("--force requires naming the entries to migrate");
        return ExitCode::from(EXIT_USAGE);
    }
    match migrate(explicit, dry_run, force) {
        Ok(fails) if fails > 0 => {
            complain!("completed with {fails} problems");
            ExitCode::FAILURE
        }
        Ok(_) => ExitCode::SUCCESS,
        Err(err) => {
            complain!("{err}");
            ExitCode::FAILURE
        }
    }
}

/// Returns the number of problems that didn't stop the run, each already
/// reported; `Err` is one that did.
fn migrate(explicit: &[String], dry_run: bool, force: bool) -> Result<usize, String> {
    ensure_config_dir(dry_run)?;
    let conf = Conf::load();

    let mut fails = 0;
    let mut migrated = Vec::new();
    let candidates = match explicit {
        [] => scan_root(&conf, &mut fails)?,
        names => names.to_vec(),
    };
    for name in candidates {
        match migrate_entry(&name, dry_run, !explicit.is_empty(), force) {
            Ok(true) => migrated.push(name),
            Ok(false) => {}
            Err(err) => {
                complain!("{err}");
                fails += 1;
            }
        }
    }

    // The ignore and link passes cover everything in .config/, not just this
    // run's moves, so a partially migrated repo self-heals.
    let mut names = Vec::new();
    match read_names(CONFIG_DIR) {
        Ok(entries) => {
            for name in utf8_names(entries, &mut fails) {
                if name == CONF_FILE || conf.is_nolink(&name) {
                    if let Err(err) = remove_nolink_link(&name, dry_run) {
                        complain!("{err}");
                        fails += 1;
                    }
                } else {
                    names.push(name);
                }
            }
        }
        // In a dry run .config/ may simply not exist yet.
        Err(_) if dry_run => {}
        Err(err) => return Err(format!("cannot read {CONFIG_DIR}: {err}")),
    }
    if dry_run {
        // The moves didn't happen, so add them by hand.
        for name in &migrated {
            if !conf.is_nolink(name) && !names.contains(name) {
                names.push(name.clone());
            }
        }
    }

    if names.is_empty() && migrated.is_empty() {
        if fails == 0 {
            say!("undot: nothing to migrate");
        }
        return Ok(fails);
    }

    if let Err(err) = ensure_gitignore(&names, dry_run) {
        complain!("{err}");
        fails += 1;
    }
    for name in &names {
        // In a dry run the moves above didn't happen, so entries planned for
        // migration still exist at the root; a real run links them after the
        // move, so don't report them as conflicts here.
        if dry_run && migrated.contains(name) {
            say!(
                "undot: [dry-run] would link {name} -> {}",
                config_path(name)
            );
            continue;
        }
        if let Err(err) = ensure_link(name, dry_run) {
            complain!("{err}");
            fails += 1;
        }
    }
    if let Err(err) = ensure_pre_commit_config(dry_run) {
        complain!("{err}");
        fails += 1;
    }

    if fails > 0 {
        return Ok(fails);
    }
    if dry_run {
        say!("undot: dry run complete; re-run without --dry-run to apply");
        return Ok(0);
    }
    moved_file_hint(&migrated);
    say!(
        "undot: done. Suggested follow-up:\n  \
         git add -A && git commit\n  \
         pre-commit install  # installs post-checkout/post-merge hooks too"
    );
    Ok(0)
}

fn ensure_config_dir(dry_run: bool) -> Result<(), String> {
    match fs::symlink_metadata(CONFIG_DIR) {
        Ok(meta) if meta.is_dir() => Ok(()),
        Ok(_) => Err(format!("{CONFIG_DIR} exists but is not a directory")),
        Err(_) if dry_run => {
            say!("undot: [dry-run] would create {CONFIG_DIR}/");
            Ok(())
        }
        Err(_) => {
            fs::create_dir(CONFIG_DIR)
                .map_err(|err| format!("cannot create {CONFIG_DIR}: {err}"))?;
            say!("undot: created {CONFIG_DIR}/");
            Ok(())
        }
    }
}

/// Finds the root dotfiles the automatic scan should try to migrate.
fn scan_root(conf: &Conf, fails: &mut usize) -> Result<Vec<String>, String> {
    let mut entries =
        read_names(".").map_err(|err| format!("cannot read repository root: {err}"))?;
    // Filter before the UTF-8 check: the name of a non-dotfile is none of our
    // business, however it is spelled.
    entries.retain(|name| name.as_bytes().starts_with(b"."));
    let mut names = utf8_names(entries, fails);
    names.retain(|name| {
        !HARD_PROTECTED.contains(&name.as_str())
            && !GUARDED.contains(&name.as_str())
            && !conf.is_ignored(name)
    });
    Ok(names)
}

/// Moves one root entry into .config/, returning whether it moved (or would
/// have, in a dry run). The symlink back is created later by the shared link
/// pass.
fn migrate_entry(name: &str, dry_run: bool, explicit: bool, force: bool) -> Result<bool, String> {
    if name == "." || name == ".." || name.contains('/') {
        return Err(format!("{name} is not a root-level entry name"));
    }
    if name == CONF_FILE {
        return Err(format!("{name} is undot's own configuration; skipping"));
    }
    if HARD_PROTECTED.contains(&name) {
        return Err(format!(
            "{name} is required at the repository root and cannot be migrated"
        ));
    }
    if GUARDED.contains(&name) && !force {
        return Err(format!(
            "{name} bootstraps the repository before any hook can run; \
             pass --force to migrate it anyway"
        ));
    }
    let meta = match fs::symlink_metadata(name) {
        Ok(meta) => meta,
        Err(_) if explicit => return Err(format!("{name} not found")),
        Err(_) => return Ok(false),
    };
    if meta.is_symlink() {
        return Ok(false);
    }
    let dest = config_path(name);
    if fs::symlink_metadata(&dest).is_ok() {
        return Err(format!("both {name} and {dest} exist; resolve manually"));
    }
    if dry_run {
        say!("undot: [dry-run] would move {name} -> {dest}");
        return Ok(true);
    }
    fs::rename(name, &dest).map_err(|err| format!("cannot move {name} -> {dest}: {err}"))?;
    say!("undot: moved {name} -> {dest}");
    Ok(true)
}

/// git keeps tracking a previously tracked FILE at its root path even though the
/// path is now an ignored symlink (ignore patterns don't apply to tracked
/// paths); directories are fine because their tracked paths moved away. Point at
/// the affected entries.
fn moved_file_hint(migrated: &[String]) {
    let files: Vec<&String> = migrated
        .iter()
        .filter(|name| fs::symlink_metadata(config_path(name)).is_ok_and(|meta| !meta.is_dir()))
        .collect();
    if files.is_empty() {
        return;
    }
    say!("undot: if these files were tracked by git, also untrack the root symlinks:");
    for name in files {
        say!("  git rm --cached {name}");
    }
}
