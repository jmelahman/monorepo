use std::fs;

use crate::util::read_optional;
use crate::CONFIG_DIR;

const GITIGNORE: &str = ".gitignore";
const HEADER: &str = "# Symlinks into .config/, recreated by `undot link`.";

/// Makes sure every root symlink is ignored via an anchored `/name` pattern.
/// Anchored, because the tracked copy lives at .config/name; without a trailing
/// slash, because a symlink never matches directory-only patterns like `name/`.
pub fn ensure_gitignore(names: &[String], dry_run: bool) -> Result<(), String> {
    let old = read_optional(GITIGNORE)?.unwrap_or_default();
    let missing = missing_patterns(&old, names);
    if missing.is_empty() {
        return Ok(());
    }
    if dry_run {
        for pattern in &missing {
            say!("undot: [dry-run] would add {pattern} to {GITIGNORE}");
        }
        return Ok(());
    }

    let mut new = old.clone();
    if !old.is_empty() && !old.ends_with('\n') {
        new.push('\n');
    }
    if !has_line(&old, HEADER) {
        if !old.is_empty() {
            new.push('\n');
        }
        new.push_str(HEADER);
        new.push('\n');
    }
    for pattern in &missing {
        new.push_str(pattern);
        new.push('\n');
    }
    fs::write(GITIGNORE, new).map_err(|err| format!("cannot write {GITIGNORE}: {err}"))?;
    say!("undot: added {} entries to {GITIGNORE}", missing.len());
    Ok(())
}

fn missing_patterns(gitignore: &str, names: &[String]) -> Vec<String> {
    let mut missing: Vec<String> = names
        .iter()
        .map(|name| format!("/{name}"))
        .filter(|pattern| !has_line(gitignore, pattern))
        .collect();

    // Pre-existing patterns that reach inside a migrated entry (".yarn/cache")
    // contain a slash and are therefore root-anchored, so after the move they
    // no longer match the content at .config/. Add a companion pattern for each
    // so `git add -A` doesn't start tracking previously ignored files.
    for line in gitignore.lines() {
        let line = line.trim();
        if line.is_empty() || line.starts_with('#') {
            continue;
        }
        let (bang, pattern) = match line.strip_prefix('!') {
            Some(rest) => ("!", rest),
            None => ("", line),
        };
        let pattern = pattern.strip_prefix('/').unwrap_or(pattern);
        if !refers_inside_any(pattern, names) {
            continue;
        }
        let companion = format!("{bang}/{CONFIG_DIR}/{pattern}");
        if !has_line(gitignore, &companion) && !missing.contains(&companion) {
            missing.push(companion);
        }
    }
    missing
}

/// Whether a gitignore pattern (leading `!` and `/` already stripped) points
/// inside one of the managed entries, like `.yarn/cache` does for `.yarn`.
fn refers_inside_any(pattern: &str, names: &[String]) -> bool {
    names.iter().any(|name| {
        pattern
            .strip_prefix(name.as_str())
            .is_some_and(|rest| rest.len() > 1 && rest.starts_with('/'))
    })
}

/// Whether any line equals `wanted` once trimmed of whitespace.
fn has_line(content: &str, wanted: &str) -> bool {
    content.lines().any(|line| line.trim() == wanted)
}
