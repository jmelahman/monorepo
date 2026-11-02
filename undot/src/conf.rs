use std::env;
use std::fs;
use std::io;

use crate::glob::Glob;
use crate::migrate;
use crate::util::config_path;

/// The configuration read from .config/ in the repository and from
/// `${XDG_CONFIG_HOME:-~/.config}/undot/` for the user. It is never symlinked
/// into the repository root.
///
/// Format, one rule per line:
///   - a root-level entry name or shell glob (`.env`, `.cla*`) that `undot
///     migrate` should leave alone;
///   - `!name` to un-ignore an entry (overriding earlier rules and the built-in
///     skip list);
///   - `nolink name` for entries that live in .config/ but should not be
///     symlinked into the root (tools are pointed at the .config/ path
///     directly). Repository file only: a user-level nolink would silently
///     break repositories that expect their links.
///
/// `#` starts a comment. The user file loads first, then the repository file;
/// the last matching ignore rule wins.
pub const CONF_FILE: &str = "undot.conf";

struct IgnoreRule {
    glob: Glob,
    negated: bool,
}

#[derive(Default)]
pub struct Conf {
    ignore: Vec<IgnoreRule>,
    nolink: Vec<Glob>,
}

impl Conf {
    /// Reads the user-level and repository-level config files. Must be called
    /// with the repository root as the working directory.
    pub fn load() -> Conf {
        let mut conf = Conf::default();
        let base = env::var("XDG_CONFIG_HOME")
            .ok()
            .filter(|base| !base.is_empty())
            .or_else(|| {
                let home = env::var("HOME").ok().filter(|home| !home.is_empty())?;
                Some(format!("{home}/.config"))
            });
        if let Some(base) = base {
            conf.load_file(&format!("{base}/undot/{CONF_FILE}"), false);
        }
        conf.load_file(&config_path(CONF_FILE), true);
        conf
    }

    fn load_file(&mut self, path: &str, repo_level: bool) {
        let content = match fs::read_to_string(path) {
            Ok(content) => content,
            Err(err) if err.kind() == io::ErrorKind::NotFound => return,
            Err(err) => {
                complain!("cannot read {path}: {err}");
                return;
            }
        };
        for line in content.lines() {
            self.parse_line(line.trim(), path, repo_level);
        }
    }

    fn parse_line(&mut self, line: &str, path: &str, repo_level: bool) {
        if line.is_empty() || line.starts_with('#') {
            return;
        }
        let (nolink, rule) = match line.strip_prefix("nolink") {
            Some(rest) if rest.starts_with([' ', '\t']) => (true, rest.trim()),
            _ => (false, line),
        };
        if nolink && !repo_level {
            complain!("{path}: nolink is only honored in the repository config; ignoring {rule}");
            return;
        }
        let (negated, rule) = match rule.strip_prefix('!') {
            Some(rest) => (true, rest.trim()),
            None => (false, rule),
        };
        if rule.is_empty() {
            return;
        }
        if nolink && negated {
            complain!("{path}: nolink rules cannot be negated; ignoring !{rule}");
            return;
        }
        if rule.contains('/') {
            complain!("{path}: ignoring {rule} (rules match root-level names, not paths)");
            return;
        }
        let Some(glob) = Glob::new(rule) else {
            complain!("{path}: ignoring malformed pattern {rule}");
            return;
        };
        if nolink {
            self.nolink.push(glob);
        } else {
            self.ignore.push(IgnoreRule { glob, negated });
        }
    }

    /// Whether a .config/ entry should be left without a root symlink; tools
    /// are expected to reference the .config/ path directly.
    pub fn is_nolink(&self, name: &str) -> bool {
        self.nolink.iter().any(|glob| glob.matches(name))
    }

    /// Whether the automatic scan should skip a root entry. Config rules take
    /// precedence over the built-in skip list; among rules, the last match wins.
    pub fn is_ignored(&self, name: &str) -> bool {
        match self
            .ignore
            .iter()
            .rev()
            .find(|rule| rule.glob.matches(name))
        {
            Some(rule) => !rule.negated,
            None => migrate::skipped_by_default(name),
        }
    }
}
