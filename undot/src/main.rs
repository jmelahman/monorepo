//! undot keeps a repository's root directory free of tool configuration
//! clutter: config files and directories (.claude/, .cursor/, .yarn/, ...) are
//! stored under .config/ and symlinked back into the root. The symlinks are
//! gitignored and recreated on checkout by the `undot link` pre-commit hook, so
//! only .config/ is tracked.

use std::env;
use std::process::ExitCode;

/// Prints a line to stdout. Unlike `println!`, a closed pipe (`undot | head`)
/// is not a panic: output is informational, and the work must still finish.
macro_rules! say {
    ($($arg:tt)*) => {{
        use std::io::Write;
        let _ = writeln!(std::io::stdout(), $($arg)*);
    }};
}

/// Prints an `undot: `-prefixed line to stderr, without `eprintln!`'s panic.
macro_rules! complain {
    ($($arg:tt)*) => {{
        use std::io::Write;
        let _ = writeln!(std::io::stderr(), "undot: {}", format_args!($($arg)*));
    }};
}

mod conf;
mod gitignore;
mod glob;
mod link;
mod migrate;
mod precommit;
mod repo;
mod util;

/// Where configuration entries live, relative to the repo root.
const CONFIG_DIR: &str = ".config";

const USAGE: &str = "\
undot: declutter a repository root by storing configs in .config/
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
  \"!name\" opts an entry back in (overrides earlier rules and built-in
  defaults; the last matching rule wins). \"nolink name\" keeps an entry in
  .config/ without a root symlink (repository file only). \"#\" starts a comment.
";

const EXIT_USAGE: u8 = 2;

fn main() -> ExitCode {
    let mut args = Vec::new();
    for arg in env::args_os().skip(1) {
        let Ok(arg) = arg.into_string() else {
            complain!("arguments must be valid UTF-8");
            return ExitCode::from(EXIT_USAGE);
        };
        args.push(arg);
    }
    run(args)
}

fn run(args: Vec<String>) -> ExitCode {
    let mut command = None;
    let mut entries = Vec::new();
    let mut dry_run = false;
    let mut force = false;

    for arg in args {
        match arg.as_str() {
            "-n" | "--dry-run" => dry_run = true,
            "-f" | "--force" => force = true,
            "-h" | "--help" => command = Some("help".to_string()),
            "-V" | "--version" => command = Some("version".to_string()),
            flag if flag.starts_with('-') => {
                complain!("unknown flag {flag}");
                return ExitCode::from(EXIT_USAGE);
            }
            _ if command.is_none() => command = Some(arg),
            _ => {
                let entry = clean_entry_arg(&arg).to_string();
                if !entries.contains(&entry) {
                    entries.push(entry);
                }
            }
        }
    }

    match command.as_deref() {
        None => {
            let _ = std::io::Write::write_all(&mut std::io::stderr(), USAGE.as_bytes());
            ExitCode::from(EXIT_USAGE)
        }
        Some("help") => {
            say!("{}", USAGE.trim_end());
            ExitCode::SUCCESS
        }
        Some("version") => {
            say!("undot {}", env!("CARGO_PKG_VERSION"));
            ExitCode::SUCCESS
        }
        Some("link") => in_repo_root(|| link::cmd_link(dry_run)),
        Some("migrate") => in_repo_root(|| migrate::cmd_migrate(&entries, dry_run, force)),
        Some(other) => {
            complain!("unknown command {other} (try `undot help`)");
            ExitCode::from(EXIT_USAGE)
        }
    }
}

fn in_repo_root(command: impl FnOnce() -> ExitCode) -> ExitCode {
    match repo::chdir_repo_root() {
        Ok(()) => command(),
        Err(err) => {
            complain!("{err}");
            ExitCode::FAILURE
        }
    }
}

/// Normalizes a user-supplied entry path like `./.claude/` to `.claude`.
fn clean_entry_arg(arg: &str) -> &str {
    let arg = arg.strip_prefix("./").unwrap_or(arg);
    arg.strip_suffix('/').unwrap_or(arg)
}
