//! End-to-end tests driving the real binary against temporary repositories.

use std::fs;
use std::os::unix::fs::symlink;
use std::path::{Path, PathBuf};
use std::process::{Command, Output};
use std::sync::atomic::{AtomicU32, Ordering};

const EXE: &str = env!("CARGO_BIN_EXE_undot");

/// A throwaway directory, removed on drop. It lives under the system temp dir
/// rather than `CARGO_TARGET_TMPDIR`: the latter sits inside this checkout, so
/// the "not a repo" test would find the real `.git`.
struct Repo {
    dir: PathBuf,
}

impl Repo {
    /// A fake repository: a temp dir with a `.git` entry.
    fn new() -> Repo {
        let repo = Repo::bare();
        repo.mkdir(".git");
        repo
    }

    fn bare() -> Repo {
        // Unique per process and test, so tests can run in parallel.
        static COUNTER: AtomicU32 = AtomicU32::new(0);
        let id = COUNTER.fetch_add(1, Ordering::Relaxed);
        let name = format!("undot-test-{}-{id}", std::process::id());
        let dir = std::env::temp_dir().join(name);
        fs::create_dir_all(&dir).unwrap();
        // Resolve symlinked temp dirs (macOS), for tests that compare paths.
        Repo {
            dir: dir.canonicalize().unwrap(),
        }
    }

    fn path(&self, name: &str) -> PathBuf {
        self.dir.join(name)
    }

    fn mkdir(&self, name: &str) -> &Repo {
        fs::create_dir_all(self.path(name)).unwrap();
        self
    }

    fn write(&self, name: &str, content: &str) -> &Repo {
        fs::write(self.path(name), content).unwrap();
        self
    }

    fn read(&self, name: &str) -> String {
        fs::read_to_string(self.path(name)).unwrap_or_default()
    }

    /// Runs the binary in `cwd`, with the user-level config lookup pointed into
    /// the repo so a developer's real undot.conf can't leak into tests.
    fn output_in(&self, cwd: &Path, args: &[&str]) -> Output {
        Command::new(EXE)
            .args(args)
            .current_dir(cwd)
            .env("XDG_CONFIG_HOME", &self.dir)
            .output()
            .unwrap()
    }

    fn output(&self, args: &[&str]) -> Output {
        self.output_in(&self.dir, args)
    }

    fn run(&self, args: &[&str]) -> i32 {
        self.output(args).status.code().unwrap()
    }

    fn stderr(&self, args: &[&str]) -> String {
        String::from_utf8(self.output(args).stderr).unwrap()
    }

    fn stdout(&self, args: &[&str]) -> String {
        String::from_utf8(self.output(args).stdout).unwrap()
    }

    fn is_link_to(&self, name: &str, want: &str) -> bool {
        fs::read_link(self.path(name)).is_ok_and(|dest| dest == Path::new(want))
    }

    fn is_real_dir(&self, name: &str) -> bool {
        fs::symlink_metadata(self.path(name)).is_ok_and(|meta| meta.is_dir())
    }

    fn exists(&self, name: &str) -> bool {
        fs::symlink_metadata(self.path(name)).is_ok()
    }
}

impl Drop for Repo {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.dir);
    }
}

// Link.

#[test]
fn link_without_config_dir_is_a_noop() {
    assert_eq!(Repo::new().run(&["link"]), 0);
}

#[test]
fn link_creates_and_repairs_links() {
    let repo = Repo::new();
    repo.mkdir(".config/.claude")
        .write(".config/.myrc", "x=1\n");
    symlink("somewhere/else", repo.path(".myrc")).unwrap();

    assert_eq!(repo.run(&["link"]), 0);
    assert!(repo.is_link_to(".claude", ".config/.claude"));
    assert!(repo.is_link_to(".myrc", ".config/.myrc"));

    // Re-running changes nothing and stays quiet about it.
    assert_eq!(repo.stdout(&["link"]), "");
    assert_eq!(repo.run(&["link"]), 0);
}

#[test]
fn link_never_touches_a_real_directory() {
    let repo = Repo::new();
    repo.mkdir(".config/.claude").mkdir(".claude");
    repo.write(".claude/precious.txt", "keep me\n");

    assert_eq!(repo.run(&["link"]), 1);
    assert_eq!(repo.read(".claude/precious.txt"), "keep me\n");
}

#[test]
fn link_outside_a_repository_fails() {
    let dir = Repo::bare();
    assert_eq!(dir.run(&["link"]), 1);
    assert!(dir
        .stderr(&["link"])
        .contains("not inside a git repository"));
}

#[test]
fn link_runs_from_the_repository_root_when_in_a_subdirectory() {
    let repo = Repo::new();
    repo.mkdir(".config/.claude").mkdir("deep/down");

    let output = repo.output_in(&repo.path("deep/down"), &["link"]);
    assert!(output.status.success());
    assert!(repo.is_link_to(".claude", ".config/.claude"));
    assert!(!repo.exists("deep/down/.claude"));
}

#[test]
fn link_finds_a_worktree_style_git_file() {
    let repo = Repo::bare();
    repo.write(".git", "gitdir: elsewhere\n")
        .mkdir(".config/.claude");

    assert_eq!(repo.run(&["link"]), 0);
    assert!(repo.is_link_to(".claude", ".config/.claude"));
}

#[test]
fn link_dry_run_changes_nothing() {
    let repo = Repo::new();
    repo.mkdir(".config/.claude");
    symlink("somewhere/else", repo.path(".myrc")).unwrap();
    repo.write(".config/.myrc", "x=1\n");

    let stdout = repo.stdout(&["link", "--dry-run"]);
    assert!(stdout.contains("[dry-run] would link .claude -> .config/.claude"));
    assert!(stdout.contains("[dry-run] would link .myrc -> .config/.myrc"));
    assert!(!repo.exists(".claude"));
    assert!(repo.is_link_to(".myrc", "somewhere/else"));
}

#[test]
fn link_rejects_a_non_directory_config() {
    let repo = Repo::new();
    repo.write(".config", "not a directory\n");
    assert_eq!(repo.run(&["link"]), 1);
}

#[test]
fn link_handles_more_entries_than_the_old_fixed_limit() {
    let repo = Repo::new();
    repo.mkdir(".config");
    for i in 0..300 {
        repo.write(&format!(".config/.rc{i}"), "");
    }

    assert_eq!(repo.run(&["migrate"]), 0);
    assert!(repo.is_link_to(".rc299", ".config/.rc299"));
    assert!(repo.read(".gitignore").contains("/.rc299\n"));
}

// macOS filesystems refuse non-UTF-8 names, so these can't arise there.
#[cfg(target_os = "linux")]
#[test]
fn non_utf8_config_entries_are_reported_not_fatal() {
    use std::os::unix::ffi::OsStrExt;

    let repo = Repo::new();
    repo.mkdir(".config/.claude");
    let name = std::ffi::OsStr::from_bytes(b".config/.bad\xff");
    fs::write(repo.dir.join(name), "").unwrap();

    assert_eq!(repo.run(&["link"]), 1);
    assert!(repo.stderr(&["link"]).contains("not valid UTF-8"));
    assert!(repo.is_link_to(".claude", ".config/.claude"));
}

#[cfg(target_os = "linux")]
#[test]
fn non_utf8_non_dotfiles_do_not_matter_to_the_scan() {
    use std::os::unix::ffi::OsStrExt;

    let repo = Repo::new();
    repo.mkdir(".claude");
    fs::write(repo.dir.join(std::ffi::OsStr::from_bytes(b"bad\xff")), "").unwrap();

    assert_eq!(repo.run(&["migrate"]), 0);
    assert!(repo.is_link_to(".claude", ".config/.claude"));
}

// Migrate.

#[test]
fn migrate_moves_links_ignores_and_registers_the_hook() {
    let repo = Repo::new();
    repo.mkdir(".claude")
        .write(".claude/settings.json", "{}\n")
        .write(".myrc", "x=1\n")
        .mkdir(".github")
        .write(".gitignore", "node_modules/\n");

    assert_eq!(repo.run(&["migrate"]), 0);

    assert!(repo.is_real_dir(".config/.claude"));
    assert!(repo.is_link_to(".claude", ".config/.claude"));
    assert!(repo.is_link_to(".myrc", ".config/.myrc"));
    assert!(repo.is_real_dir(".github"));
    assert_eq!(repo.read(".config/.claude/settings.json"), "{}\n");

    let ignore = repo.read(".gitignore");
    assert!(ignore.contains("node_modules/\n"));
    assert!(ignore.contains("/.claude\n"));
    assert!(ignore.contains("/.myrc\n"));

    let config = repo.read(".pre-commit-config.yaml");
    assert!(config.contains("id: undot"));
    assert!(config.contains("default_install_hook_types:"));
    assert!(config.contains(concat!("rev: v", env!("CARGO_PKG_VERSION"))));
}

#[test]
fn migrate_is_idempotent() {
    let repo = Repo::new();
    repo.mkdir(".claude").write(".claude/settings.json", "{}\n");

    assert_eq!(repo.run(&["migrate"]), 0);
    assert_eq!(repo.run(&["migrate"]), 0);

    assert_eq!(repo.read(".gitignore").matches("/.claude\n").count(), 1);
    assert_eq!(
        repo.read(".pre-commit-config.yaml")
            .matches("id: undot")
            .count(),
        1
    );
    assert!(repo.is_link_to(".claude", ".config/.claude"));
}

#[test]
fn migrate_explicit_overrides_the_skip_list_and_leaves_the_rest() {
    let repo = Repo::new();
    repo.write(".env", "SECRET=1\n").mkdir(".claude");

    assert_eq!(repo.run(&["migrate", ".env"]), 0);
    assert!(repo.is_link_to(".env", ".config/.env"));
    assert!(repo.is_real_dir(".claude"));
}

#[test]
fn migrate_normalizes_entry_arguments() {
    let repo = Repo::new();
    repo.mkdir(".claude");

    assert_eq!(repo.run(&["migrate", "./.claude/", ".claude"]), 0);
    assert!(repo.is_link_to(".claude", ".config/.claude"));
}

#[test]
fn migrate_rejects_paths_and_missing_entries() {
    let repo = Repo::new();
    repo.mkdir(".a/b");

    assert_eq!(repo.run(&["migrate", ".a/b"]), 1);
    assert_eq!(repo.run(&["migrate", ".."]), 1);
    assert_eq!(repo.run(&["migrate", ".missing"]), 1);
    assert!(repo.is_real_dir(".a/b"));
}

#[test]
fn migrate_scan_skips_the_default_list() {
    let repo = Repo::new();
    repo.write(".env", "SECRET=1\n")
        .mkdir(".mypy_cache")
        .mkdir(".claude");

    assert_eq!(repo.run(&["migrate"]), 0);
    assert!(!repo.exists(".config/.env"));
    assert!(!repo.exists(".config/.mypy_cache"));
    assert!(repo.is_link_to(".claude", ".config/.claude"));
}

#[test]
fn migrate_scan_leaves_git_files_alone() {
    let repo = Repo::new();
    repo.write(".gitlab-ci.yml", "x\n")
        .write(".git-blame-ignore-revs", "x\n")
        .write(".gitignore", "")
        .mkdir(".claude");

    assert_eq!(repo.run(&["migrate"]), 0);
    assert!(!repo.exists(".config/.gitlab-ci.yml"));
    assert!(!repo.exists(".config/.git-blame-ignore-revs"));
    assert!(!repo.exists(".config/.gitignore"));
    assert!(!repo.is_link_to(".gitlab-ci.yml", ".config/.gitlab-ci.yml"));

    // Still possible on request, unlike the files git itself needs.
    assert_eq!(repo.run(&["migrate", ".git-blame-ignore-revs"]), 0);
    assert_eq!(repo.run(&["migrate", ".gitignore"]), 1);
}

#[test]
fn migrate_dry_run_changes_nothing() {
    let repo = Repo::new();
    repo.mkdir(".claude");

    assert_eq!(repo.run(&["migrate", "--dry-run"]), 0);
    assert!(repo.is_real_dir(".claude"));
    assert!(!repo.exists(".config"));
    assert!(!repo.exists(".pre-commit-config.yaml"));
    assert!(!repo.exists(".gitignore"));
}

#[test]
fn flags_may_precede_the_command() {
    let repo = Repo::new();
    repo.mkdir(".claude");

    assert_eq!(repo.run(&["-n", "migrate"]), 0);
    assert!(repo.is_real_dir(".claude"));
}

#[test]
fn migrate_adds_companions_for_patterns_inside_moved_entries() {
    let repo = Repo::new();
    repo.mkdir(".yarn").mkdir(".claude").write(
        ".gitignore",
        ".yarn/cache\n!.yarn/patches\n**/.claude/tmp\nnode_modules/\n",
    );

    assert_eq!(repo.run(&["migrate"]), 0);
    let ignore = repo.read(".gitignore");
    assert!(ignore.contains("/.config/.yarn/cache\n"));
    assert!(ignore.contains("!/.config/.yarn/patches\n"));
    assert!(!ignore.contains(".config/**"));
    assert_eq!(ignore.matches("node_modules/").count(), 1);

    assert_eq!(repo.run(&["migrate"]), 0);
    assert_eq!(
        repo.read(".gitignore")
            .matches("/.config/.yarn/cache\n")
            .count(),
        1
    );
}

#[test]
fn migrate_refuses_to_overwrite_an_existing_config_entry() {
    let repo = Repo::new();
    repo.mkdir(".claude").mkdir(".config/.claude");

    assert_eq!(repo.run(&["migrate"]), 1);
    assert!(repo.is_real_dir(".claude"));
}

#[test]
fn migrate_hints_at_untracking_moved_files() {
    let repo = Repo::new();
    repo.write(".myrc", "x\n").mkdir(".claude");

    let stdout = repo.stdout(&["migrate"]);
    assert!(stdout.contains("git rm --cached .myrc"));
    assert!(!stdout.contains("git rm --cached .claude"));
}

#[test]
fn migrate_keeps_a_gitignore_it_cannot_decode() {
    let repo = Repo::new();
    repo.mkdir(".claude");
    fs::write(repo.path(".gitignore"), b"# caf\xe9\nsecret\n").unwrap();

    assert_eq!(repo.run(&["migrate"]), 1);
    assert_eq!(
        fs::read(repo.path(".gitignore")).unwrap(),
        b"# caf\xe9\nsecret\n"
    );
}

#[test]
fn migrate_keeps_a_pre_commit_config_it_cannot_decode() {
    let repo = Repo::new();
    repo.mkdir(".claude");
    fs::write(repo.path(".pre-commit-config.yaml"), b"# caf\xe9\n").unwrap();

    assert_eq!(repo.run(&["migrate"]), 1);
    assert_eq!(
        fs::read(repo.path(".pre-commit-config.yaml")).unwrap(),
        b"# caf\xe9\n"
    );
}

// Pre-commit config.

#[test]
fn migrate_inserts_the_hook_into_an_existing_config() {
    let repo = Repo::new();
    repo.mkdir(".claude").write(
        ".pre-commit-config.yaml",
        "repos:\n  - repo: https://github.com/psf/black\n    rev: 24.1.0\n    hooks:\n      - id: black\n",
    );

    assert_eq!(repo.run(&["migrate"]), 0);
    let config = repo.read(".pre-commit-config.yaml");
    assert!(config.contains("id: black"));
    assert!(config.contains("id: undot"));
    assert!(config.starts_with("default_install_hook_types:"));
    assert!(config.find("jmelahman/undot") < config.find("psf/black"));
}

#[test]
fn migrate_edits_a_yml_config_when_that_is_what_exists() {
    let repo = Repo::new();
    repo.mkdir(".claude")
        .write(".pre-commit-config.yml", "repos:\n");

    assert_eq!(repo.run(&["migrate"]), 0);
    assert!(repo.read(".pre-commit-config.yml").contains("id: undot"));
    assert!(!repo.exists(".pre-commit-config.yaml"));
}

#[test]
fn migrate_warns_when_existing_hook_types_miss_a_stage() {
    let repo = Repo::new();
    repo.mkdir(".claude").write(
        ".pre-commit-config.yaml",
        "default_install_hook_types: [pre-commit, post-checkout]\nrepos:\n",
    );

    let stderr = repo.stderr(&["migrate"]);
    assert!(stderr.contains("add post-merge, post-rewrite to default_install_hook_types"));
    assert!(!stderr.contains("post-checkout,"));
}

#[test]
fn migrate_reports_an_inline_repos_value_without_changing_it() {
    let repo = Repo::new();
    repo.mkdir(".claude")
        .write(".pre-commit-config.yaml", "repos: []\n");

    assert_eq!(repo.run(&["migrate"]), 1);
    assert_eq!(repo.read(".pre-commit-config.yaml"), "repos: []\n");
}

// Configuration.

#[test]
fn conf_ignores_entries_until_named_explicitly() {
    let repo = Repo::new();
    repo.mkdir(".claude")
        .mkdir(".cursor")
        .mkdir(".config")
        .write(".config/undot.conf", "# ignore claude\n.claude\n");

    assert_eq!(repo.run(&["migrate"]), 0);
    assert!(repo.is_real_dir(".claude"));
    assert!(!repo.exists(".config/.claude"));
    assert!(repo.is_link_to(".cursor", ".config/.cursor"));

    assert_eq!(repo.run(&["migrate", ".claude"]), 0);
    assert!(repo.is_link_to(".claude", ".config/.claude"));
}

#[test]
fn conf_negation_and_globs() {
    let repo = Repo::new();
    repo.write(".env", "SECRET=1\n")
        .mkdir(".cursor")
        .write(".cursorignore", "x\n")
        .mkdir(".config")
        .write(".config/undot.conf", "!.env\n.cursor*\n");

    assert_eq!(repo.run(&["migrate"]), 0);
    assert!(repo.is_link_to(".env", ".config/.env"));
    assert!(!repo.exists(".config/.cursor"));
    assert!(!repo.exists(".config/.cursorignore"));
}

#[test]
fn conf_last_matching_rule_wins() {
    let repo = Repo::new();
    repo.mkdir(".claude")
        .mkdir(".config")
        .write(".config/undot.conf", ".cl*\n!.claude\n");

    assert_eq!(repo.run(&["migrate"]), 0);
    assert!(repo.is_link_to(".claude", ".config/.claude"));
}

#[test]
fn conf_can_opt_git_files_back_in() {
    let repo = Repo::new();
    repo.write(".git-blame-ignore-revs", "x\n")
        .mkdir(".config")
        .write(".config/undot.conf", "!.git-blame-ignore-revs\n");

    assert_eq!(repo.run(&["migrate"]), 0);
    assert!(repo.is_link_to(".git-blame-ignore-revs", ".config/.git-blame-ignore-revs"));
}

#[test]
fn conf_reports_bad_rules_and_keeps_going() {
    let repo = Repo::new();
    repo.mkdir(".claude")
        .mkdir(".config")
        .write(".config/undot.conf", "[bad\nsub/dir\nnolink !.claude\n!\n");

    let output = repo.output(&["migrate"]);
    let stderr = String::from_utf8(output.stderr).unwrap();
    assert!(output.status.success());
    assert!(stderr.contains("malformed pattern [bad"));
    assert!(stderr.contains("not paths"));
    assert!(stderr.contains("cannot be negated"));
    assert!(repo.is_link_to(".claude", ".config/.claude"));
}

#[test]
fn user_conf_applies_and_the_repository_conf_overrides_it() {
    let repo = Repo::new();
    repo.mkdir("undot")
        .write("undot/undot.conf", ".claude\n")
        .mkdir(".claude")
        .mkdir(".cursor");

    assert_eq!(repo.run(&["migrate"]), 0);
    assert!(!repo.exists(".config/.claude"));
    assert!(repo.is_link_to(".cursor", ".config/.cursor"));

    repo.write(".config/undot.conf", "!.claude\n");
    assert_eq!(repo.run(&["migrate"]), 0);
    assert!(repo.is_link_to(".claude", ".config/.claude"));
}

#[test]
fn conf_file_is_never_linked() {
    let repo = Repo::new();
    repo.mkdir(".claude")
        .mkdir(".config")
        .write(".config/undot.conf", "# nothing ignored\n");

    assert_eq!(repo.run(&["migrate"]), 0);
    assert_eq!(repo.run(&["link"]), 0);
    assert!(!repo.exists("undot.conf"));

    // A stray root symlink left by an older undot gets cleaned up.
    symlink(".config/undot.conf", repo.path("undot.conf")).unwrap();
    assert_eq!(repo.run(&["link"]), 0);
    assert!(!repo.exists("undot.conf"));

    assert!(!repo.read(".gitignore").contains("undot.conf"));
    assert_eq!(repo.run(&["migrate", "undot.conf"]), 1);
}

#[test]
fn guarded_entries_require_force() {
    let repo = Repo::new();
    repo.mkdir(".github").write(".github/f.yml", "x\n");

    assert_eq!(repo.run(&["migrate", ".github"]), 1);
    assert!(repo.is_real_dir(".github"));

    assert_eq!(repo.run(&["migrate", "--force", ".github"]), 0);
    assert!(repo.is_link_to(".github", ".config/.github"));

    assert_eq!(Repo::new().run(&["migrate", "--force"]), 2);
}

#[test]
fn ci_configs_are_skipped_until_named() {
    let repo = Repo::new();
    repo.write(".goreleaser.yaml", "version: 2\n")
        .mkdir(".claude");

    assert_eq!(repo.run(&["migrate"]), 0);
    assert!(!repo.exists(".config/.goreleaser.yaml"));

    assert_eq!(repo.run(&["migrate", ".goreleaser.yaml"]), 0);
    assert!(repo.is_link_to(".goreleaser.yaml", ".config/.goreleaser.yaml"));
}

// Nolink.

#[test]
fn nolink_removes_the_link_and_stops_creating_it() {
    let repo = Repo::new();
    repo.mkdir(".claude").write(".myrc", "x=1\n");

    assert_eq!(repo.run(&["migrate"]), 0);
    assert!(repo.is_link_to(".myrc", ".config/.myrc"));

    repo.write(".config/undot.conf", "nolink .myrc\n");
    assert_eq!(repo.run(&["link"]), 0);
    assert!(!repo.exists(".myrc"));
    assert!(repo.exists(".config/.myrc"));
    assert!(repo.is_link_to(".claude", ".config/.claude"));
    assert_eq!(repo.run(&["link"]), 0);

    // A real file at the root path is never touched.
    repo.write(".myrc", "local override\n");
    assert_eq!(repo.run(&["link"]), 0);
    assert_eq!(repo.read(".myrc"), "local override\n");
}

#[test]
fn nolink_entries_migrate_without_a_link_or_ignore_rule() {
    let repo = Repo::new();
    repo.mkdir(".config")
        .write(".config/undot.conf", "nolink .goreleaser.yaml\n")
        .write(".goreleaser.yaml", "version: 2\n")
        .mkdir(".claude");

    assert_eq!(repo.run(&["migrate", ".goreleaser.yaml"]), 0);
    assert!(!repo.exists(".goreleaser.yaml"));
    assert!(repo.exists(".config/.goreleaser.yaml"));
    assert!(!repo.read(".gitignore").contains("/.goreleaser.yaml"));
}

#[test]
fn user_level_nolink_is_rejected() {
    let repo = Repo::new();
    repo.mkdir("undot")
        .write("undot/undot.conf", "nolink .claude\n")
        .mkdir(".claude");

    assert_eq!(repo.run(&["migrate"]), 0);
    assert!(repo.is_link_to(".claude", ".config/.claude"));
}

// Command line.

#[test]
fn usage_errors_exit_2() {
    let repo = Repo::new();
    assert_eq!(repo.run(&[]), 2);
    assert_eq!(repo.run(&["bogus"]), 2);
    assert_eq!(repo.run(&["link", "--bogus"]), 2);
}

#[test]
fn only_migrate_takes_entries_and_force() {
    let repo = Repo::new();
    repo.mkdir(".config/.claude");

    assert_eq!(repo.run(&["link", ".claude"]), 2);
    assert_eq!(repo.run(&["link", "--force"]), 2);
    assert_eq!(repo.run(&["version", "extra"]), 2);
    assert!(!repo.exists(".claude"));
}

#[test]
fn double_dash_ends_the_flags() {
    let repo = Repo::new();
    repo.write("-weird", "x\n");

    assert_eq!(repo.run(&["migrate", "--", "-weird"]), 0);
    assert!(repo.is_link_to("-weird", ".config/-weird"));
}

#[test]
fn help_wins_wherever_it_appears() {
    let repo = Repo::new();
    repo.mkdir(".claude");

    for args in [
        &["migrate", "-h", ".claude"][..],
        &["-h", "migrate", ".claude"],
    ] {
        assert!(
            repo.stdout(args).starts_with("undot: declutter"),
            "{args:?}"
        );
    }
    assert!(repo.is_real_dir(".claude"));
}

// A real repository, since only git can say whether the patterns work.
#[test]
fn gitignore_patterns_match_names_with_special_characters() {
    let repo = Repo::bare();
    let names = [
        ".a b",
        ".star*",
        ".brack[et]",
        ".what?",
        ".back\\slash",
        ".trail ",
    ];
    assert!(Command::new("git")
        .args(["init", "-q"])
        .current_dir(&repo.dir)
        .status()
        .unwrap()
        .success());
    for name in names {
        repo.write(name, "x\n");
    }
    // Would be wrongly ignored by `/.star*` and `/.what?` if left unescaped.
    repo.mkdir(".config")
        .write(".starry", "x\n")
        .write(".whats", "x\n");

    let args = [&["migrate"][..], &names].concat();
    assert_eq!(repo.run(&args), 0);
    let before = repo.read(".gitignore");
    assert_eq!(repo.run(&args), 0);
    assert_eq!(repo.read(".gitignore"), before);

    for name in names {
        assert!(repo.is_link_to(name, &format!(".config/{name}")), "{name}");
    }
    let untracked = Command::new("git")
        .args(["ls-files", "--others", "--exclude-standard", "-z"])
        .current_dir(&repo.dir)
        .output()
        .unwrap();
    let untracked = String::from_utf8(untracked.stdout).unwrap();
    let untracked: Vec<&str> = untracked.split('\0').collect();
    for name in names {
        assert!(
            !untracked.contains(&name),
            "{name} is not ignored: {untracked:?}"
        );
    }
    assert!(untracked.contains(&".starry"), "{untracked:?}");
    assert!(untracked.contains(&".whats"), "{untracked:?}");
}

#[test]
fn newlines_in_names_are_reported_not_written_to_gitignore() {
    let repo = Repo::new();
    repo.mkdir(".config").write(".config/.a\nb", "");

    assert_eq!(repo.run(&["migrate"]), 1);
    assert!(!repo.read(".gitignore").contains(".a"));
}

#[test]
fn help_and_version_work_anywhere() {
    let dir = Repo::bare();
    let version = format!("undot {}\n", env!("CARGO_PKG_VERSION"));
    for args in [&["version"][..], &["--version"], &["-V"], &["link", "-V"]] {
        assert_eq!(dir.stdout(args), version, "{args:?}");
    }
    for args in [&["help"][..], &["--help"], &["-h"], &["migrate", "--help"]] {
        assert!(dir.stdout(args).starts_with("undot: declutter"), "{args:?}");
        assert_eq!(dir.run(args), 0, "{args:?}");
    }
}

#[test]
fn closed_stdout_does_not_abort_the_work() {
    use std::process::Stdio;

    let repo = Repo::new();
    repo.mkdir(".config/.claude");
    let mut child = Command::new(EXE)
        .arg("link")
        .current_dir(&repo.dir)
        .env("XDG_CONFIG_HOME", &repo.dir)
        .stdout(Stdio::piped())
        .spawn()
        .unwrap();
    drop(child.stdout.take());

    assert!(child.wait().unwrap().success());
    assert!(repo.is_link_to(".claude", ".config/.claude"));
}
