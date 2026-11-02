use std::env;
use std::fs;

const NOT_A_REPO: &str = "not inside a git repository (no .git found)";

/// Walks up from the current directory to the nearest `.git` entry (a
/// directory, or a file for worktrees and submodules) and makes that directory
/// the working directory. Every other path in the program is relative to it.
pub fn chdir_repo_root() -> Result<(), String> {
    let cwd = env::current_dir().map_err(|_| NOT_A_REPO)?;
    let root = cwd
        .ancestors()
        .find(|dir| fs::symlink_metadata(dir.join(".git")).is_ok())
        .ok_or(NOT_A_REPO)?;
    env::set_current_dir(root).map_err(|err| format!("cannot enter {}: {err}", root.display()))
}
