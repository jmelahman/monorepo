use std::fs;

use crate::util::read_optional;

const HOOK_REPO_URL: &str = "https://github.com/jmelahman/undot";
const HOOK_REV: &str = concat!("v", env!("CARGO_PKG_VERSION"));
const PRE_COMMIT_YAML: &str = ".pre-commit-config.yaml";
const PRE_COMMIT_YML: &str = ".pre-commit-config.yml";

const HOOK_TYPES_KEY: &str = "default_install_hook_types";

/// The stages that recreate the symlinks after an operation rewrites the
/// worktree.
const REQUIRED_STAGES: [&str; 3] = ["post-checkout", "post-merge", "post-rewrite"];

/// Plain `pre-commit install` only installs the pre-commit stage, hence the
/// explicit hook types.
const HOOK_TYPES_LINE: &str =
    "default_install_hook_types: [pre-commit, post-checkout, post-merge, post-rewrite]";

/// Registers the undot hook in the repository's pre-commit config, creating the
/// file if needed. Edits are plain text: the repo block is inserted before the
/// first existing `- repo:` entry, reusing its indentation.
pub fn ensure_pre_commit_config(dry_run: bool) -> Result<(), String> {
    let existing = match read_optional(PRE_COMMIT_YAML)? {
        Some(content) => Some((PRE_COMMIT_YAML, content)),
        None => read_optional(PRE_COMMIT_YML)?.map(|content| (PRE_COMMIT_YML, content)),
    };
    let Some((path, content)) = existing else {
        if dry_run {
            say!("undot: [dry-run] would create {PRE_COMMIT_YAML} with the undot hook");
            return Ok(());
        }
        let fresh = format!("{HOOK_TYPES_LINE}\nrepos:\n{}", repo_block("  "));
        fs::write(PRE_COMMIT_YAML, fresh)
            .map_err(|err| format!("cannot write {PRE_COMMIT_YAML}: {err}"))?;
        say!("undot: created {PRE_COMMIT_YAML}");
        return Ok(());
    };

    warn_about_hook_types(path, &content);
    if is_registered(&content) {
        return Ok(());
    }
    let updated = with_hook(&content).map_err(|err| format!("{path}: {err}"))?;
    if dry_run {
        say!("undot: [dry-run] would add the undot hook to {path}");
        return Ok(());
    }
    fs::write(path, updated).map_err(|err| format!("cannot write {path}: {err}"))?;
    say!("undot: added the undot hook to {path}");
    Ok(())
}

/// An existing `default_install_hook_types` is the user's to edit, but if it
/// lacks a stage the symlinks silently won't come back after that operation.
fn warn_about_hook_types(path: &str, content: &str) {
    let Some(hook_types) = hook_types_value(content) else {
        return;
    };
    let missing: Vec<&str> = REQUIRED_STAGES
        .into_iter()
        .filter(|stage| !hook_types.contains(stage))
        .collect();
    if !missing.is_empty() {
        complain!(
            "{path}: add {} to {HOOK_TYPES_KEY}, or undot won't restore symlinks then",
            missing.join(", ")
        );
    }
}

/// A line without its comment. Good enough for the keys read here, whose values
/// never contain ` #`.
fn code(line: &str) -> &str {
    if line.trim_start().starts_with('#') {
        return "";
    }
    line.split(" #").next().unwrap_or(line)
}

/// Whether the config already has the undot repo, or one of its hooks (a local
/// or mirrored copy), rather than merely mentioning them.
fn is_registered(content: &str) -> bool {
    content.lines().any(|line| {
        let code = code(line).trim();
        let item = code.strip_prefix('-').unwrap_or(code).trim_start();
        let value = |key: &str| {
            let value = item.strip_prefix(key)?.strip_prefix(':')?;
            Some(value.trim().trim_matches(['"', '\'']))
        };
        value("repo").is_some_and(|repo| repo.strip_suffix(".git").unwrap_or(repo) == HOOK_REPO_URL)
            || value("id").is_some_and(|id| id == "undot" || id == "undot-system")
    })
}

/// The value of the top-level hook types key: an inline list, or the block list
/// on the lines after it.
fn hook_types_value(content: &str) -> Option<String> {
    let mut lines = content.lines().map(code);
    let mut value = lines
        .find_map(|line| line.strip_prefix(HOOK_TYPES_KEY)?.strip_prefix(':'))?
        .to_string();
    for line in lines.filter(|line| !line.trim().is_empty()) {
        if !line.starts_with([' ', '\t', '-']) {
            break;
        }
        value.push(' ');
        value.push_str(line);
    }
    Some(value)
}

/// Returns `content` with the hook types (unless already set) and the undot repo
/// block added.
fn with_hook(content: &str) -> Result<String, String> {
    let lines: Vec<&str> = content.split('\n').collect();

    // Where to insert default_install_hook_types: the top of the document,
    // unless the key is already there.
    let insert_types_at = if hook_types_value(content).is_some() {
        None
    } else if lines[0].trim() == "---" {
        Some(1)
    } else {
        Some(0)
    };

    // Where to insert the repo block: before the first list item, however it is
    // spelled, reusing its indentation, or right after a bare `repos:` key.
    let repos = lines.iter().position(|line| line.starts_with("repos:"));
    if let Some(repos) = repos {
        let value = lines[repos]["repos:".len()..].trim();
        if !value.is_empty() && !value.starts_with('#') {
            return Err(format!(
                "cannot edit an inline `repos: {value}`; add the undot hook by hand"
            ));
        }
    }
    let first_repo = repos.and_then(|repos| {
        lines[repos + 1..]
            .iter()
            .position(|line| !code(line).trim().is_empty())
            .map(|offset| repos + 1 + offset)
            .filter(|&i| lines[i].trim_start().starts_with('-'))
    });
    let insert_repo_at = first_repo.or(repos.map(|repos| repos + 1));
    let indent = first_repo.map_or("  ", |i| {
        let line = lines[i];
        &line[..line.len() - line.trim_start().len()]
    });

    let mut out = String::with_capacity(content.len() + 128);
    for (i, line) in lines.iter().enumerate() {
        if insert_types_at == Some(i) {
            out.push_str(HOOK_TYPES_LINE);
            out.push('\n');
        }
        if insert_repo_at == Some(i) {
            out.push_str(&repo_block(indent));
        }
        out.push_str(line);
        if i + 1 < lines.len() {
            out.push('\n');
        }
    }
    match insert_repo_at {
        // No repos: key anywhere; append one at the end.
        None => {
            if !content.is_empty() && !content.ends_with('\n') {
                out.push('\n');
            }
            out.push_str("repos:\n");
            out.push_str(&repo_block("  "));
        }
        // A bare `repos:` ending the file without a newline: the loop never got
        // that far.
        Some(i) if i == lines.len() => {
            out.push('\n');
            out.push_str(&repo_block(indent));
        }
        Some(_) => {}
    }
    Ok(out)
}

fn repo_block(indent: &str) -> String {
    format!(
        "{indent}- repo: {HOOK_REPO_URL}\n\
         {indent}  rev: {HOOK_REV}\n\
         {indent}  hooks:\n\
         {indent}    - id: undot\n"
    )
}

#[cfg(test)]
mod tests {
    use super::{hook_types_value, is_registered, repo_block, with_hook, HOOK_TYPES_LINE};

    #[test]
    fn inserts_before_the_first_repo_reusing_its_indent() {
        let updated = with_hook("repos:\n    - repo: x\n").unwrap();
        assert_eq!(
            updated,
            format!(
                "{HOOK_TYPES_LINE}\nrepos:\n{}    - repo: x\n",
                repo_block("    ")
            )
        );
    }

    #[test]
    fn unindented_repo_lists_stay_valid() {
        let updated = with_hook("repos:\n- repo: x\n").unwrap();
        assert!(updated.contains(&format!("repos:\n{}- repo: x\n", repo_block(""))));
    }

    #[test]
    fn keeps_a_document_marker_first() {
        let updated = with_hook("---\nrepos:\n").unwrap();
        assert!(updated.starts_with(&format!("---\n{HOOK_TYPES_LINE}\nrepos:\n")));
    }

    #[test]
    fn appends_repos_when_absent() {
        let updated = with_hook("fail_fast: true").unwrap();
        assert_eq!(
            updated,
            format!(
                "{HOOK_TYPES_LINE}\nfail_fast: true\nrepos:\n{}",
                repo_block("  ")
            )
        );
    }

    #[test]
    fn leaves_an_existing_hook_types_key_alone() {
        let content = "default_install_hook_types: [pre-push]\nrepos:\n";
        let updated = with_hook(content).unwrap();
        assert_eq!(updated.matches("default_install_hook_types").count(), 1);
    }

    #[test]
    fn inserts_after_a_final_repos_without_a_newline() {
        let updated = with_hook("repos:").unwrap();
        assert!(updated.ends_with(&format!("repos:\n{}", repo_block("  "))));
    }

    #[test]
    fn reuses_the_indent_of_any_list_item_spelling() {
        for item in ["    -   repo: x\n", "    - {repo: x, rev: y, hooks: []}\n"] {
            let updated = with_hook(&format!("repos:\n{item}")).unwrap();
            assert!(
                updated.contains(&format!("repos:\n{}{item}", repo_block("    "))),
                "{updated}"
            );
        }
    }

    #[test]
    fn a_key_after_an_empty_repos_is_not_a_list_item() {
        let updated = with_hook("repos:\nfail_fast: true\n").unwrap();
        assert!(updated.contains(&format!("repos:\n{}fail_fast", repo_block("  "))));
    }

    #[test]
    fn only_real_entries_count_as_registered() {
        for content in [
            "repos:\n  - repo: https://github.com/jmelahman/undot\n",
            "repos:\n  - repo: \"https://github.com/jmelahman/undot.git\"  # frozen\n",
            "      - id: undot\n",
            "      - id: undot-system\n",
            "      - id: undot # links\n",
        ] {
            assert!(is_registered(content), "{content}");
        }
        for content in [
            "# see https://github.com/jmelahman/undot\n",
            "  - repo: https://github.com/jmelahman/undot-fork\n",
            "      - id: undot-foo\n",
            "      # - id: undot\n",
        ] {
            assert!(!is_registered(content), "{content}");
        }
    }

    #[test]
    fn hook_types_are_read_from_the_key_only() {
        assert_eq!(
            hook_types_value("# default_install_hook_types: [a]\n"),
            None
        );
        let block = "default_install_hook_types:\n  - a # first\n- b\nrepos:\n  - c\n";
        let value = hook_types_value(block).unwrap();
        assert!(value.contains('a') && value.contains('b') && !value.contains('c'));
    }

    #[test]
    fn a_commented_out_hook_types_key_does_not_count() {
        let updated = with_hook("# default_install_hook_types: [a]\nrepos:\n").unwrap();
        assert!(updated.starts_with(HOOK_TYPES_LINE));
    }

    #[test]
    fn refuses_inline_repos() {
        assert!(with_hook("repos: []\n").is_err());
    }

    #[test]
    fn allows_a_comment_after_repos() {
        let updated = with_hook("repos: # hooks\n  - repo: x\n").unwrap();
        assert!(updated.contains("repos: # hooks\n  - repo: https"));
    }
}
