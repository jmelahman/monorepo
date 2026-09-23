//! End-to-end tests driving the real binary against real git repositories.
//!
//! These cover the git and CLI layers, which unit tests cannot reach: tag
//! discovery, `--check` validation, and actually creating and pushing a tag.

use std::path::{Path, PathBuf};
use std::process::Command;
use std::sync::atomic::{AtomicU32, Ordering};

const EXE: &str = env!("CARGO_BIN_EXE_tag");

/// A throwaway git repository, optionally wired to a bare "remote".
struct Repo {
    dir: PathBuf,
}

impl Repo {
    fn new(name: &str) -> Repo {
        // Keep names unique so tests can run in parallel and be re-run cleanly.
        static COUNTER: AtomicU32 = AtomicU32::new(0);
        let id = COUNTER.fetch_add(1, Ordering::Relaxed);
        let dir = Path::new(env!("CARGO_TARGET_TMPDIR")).join(format!("{name}-{id}"));

        let _ = std::fs::remove_dir_all(&dir);
        std::fs::create_dir_all(&dir).unwrap();

        let repo = Repo { dir };
        repo.git(&["init", "-q", "--initial-branch", "main", "."]);
        repo.git(&["config", "user.email", "test@example.com"]);
        repo.git(&["config", "user.name", "Test"]);
        repo
    }

    /// Runs `program` inside the repository, isolated from the developer's own
    /// git configuration (signing, hooks, aliases) so results are deterministic.
    fn command(&self, program: &str) -> Command {
        let mut cmd = Command::new(program);
        cmd.current_dir(&self.dir)
            .env("GIT_CONFIG_GLOBAL", self.dir.join("no-global-gitconfig"))
            .env("GIT_CONFIG_NOSYSTEM", "1");
        cmd
    }

    fn git(&self, args: &[&str]) -> String {
        let output = self.command("git").args(args).output().unwrap();
        assert!(output.status.success(), "git {args:?} failed: {output:?}");
        String::from_utf8(output.stdout).unwrap()
    }

    /// Creates a commit, so that tags have distinct places to point at.
    fn commit(&self, name: &str) -> &Repo {
        std::fs::write(self.dir.join(name), name).unwrap();
        self.git(&["add", "-A"]);
        self.git(&["commit", "-qm", name]);
        self
    }

    fn tag(&self, tag: &str) -> &Repo {
        self.git(&["tag", tag]);
        self
    }

    /// Adds a bare repository as `origin`, for exercising fetch and push.
    fn with_remote(&self) -> &Repo {
        let remote = self.dir.join("remote.git");
        self.git(&["init", "-q", "--bare", remote.to_str().unwrap()]);
        self.git(&["remote", "add", "origin", remote.to_str().unwrap()]);
        // Push tags too: fetching prunes local tags the remote does not have.
        self.git(&["push", "-q", "--tags", "origin", "HEAD"]);
        self
    }

    /// Runs the binary and returns its trimmed stdout plus exit code.
    fn run(&self, args: &[&str]) -> (String, i32) {
        let output = self.command(EXE).args(args).output().unwrap();
        let stdout = String::from_utf8(output.stdout).unwrap().trim().to_string();
        (stdout, output.status.code().unwrap())
    }

    /// Runs the binary offline, which is what almost every test wants.
    fn tag_cmd(&self, args: &[&str]) -> (String, i32) {
        let mut all = vec!["--no-fetch"];
        all.extend_from_slice(args);
        self.run(&all)
    }

    fn tags(&self) -> Vec<String> {
        self.git(&["tag", "-l"])
            .lines()
            .map(str::to_string)
            .collect()
    }
}

#[test]
fn increments_the_patch_version_by_default() {
    let repo = Repo::new("default");
    repo.commit("a").tag("v1.0.0").commit("b");

    assert_eq!(repo.tag_cmd(&["--print-only"]), ("v1.0.1".into(), 0));
}

#[test]
fn increments_the_requested_component() {
    let repo = Repo::new("increments");
    repo.commit("a").tag("v1.2.3").commit("b");

    assert_eq!(
        repo.tag_cmd(&["--patch", "--print-only"]),
        ("v1.2.4".into(), 0)
    );
    assert_eq!(
        repo.tag_cmd(&["--minor", "--print-only"]),
        ("v1.3.0".into(), 0)
    );
    assert_eq!(
        repo.tag_cmd(&["--major", "--print-only"]),
        ("v2.0.0".into(), 0)
    );
}

#[test]
fn starts_from_v0_0_1_when_untagged() {
    let repo = Repo::new("untagged");
    repo.commit("a");

    assert_eq!(repo.tag_cmd(&["--print-only"]), ("v0.0.1".into(), 0));
}

#[test]
fn rejects_multiple_increment_flags() {
    let repo = Repo::new("conflict");
    repo.commit("a");

    let (output, code) = repo.tag_cmd(&["--major", "--minor", "--print-only"]);
    assert!(
        output.contains("only one version increment flag"),
        "{output}"
    );
    assert_eq!(code, 1);
}

#[test]
fn refuses_to_tag_an_already_tagged_head() {
    let repo = Repo::new("already-tagged");
    repo.commit("a").tag("v1.0.0");

    let (output, code) = repo.tag_cmd(&["--print-only"]);
    assert_eq!(output, "Error: Current HEAD is already tagged");
    assert_eq!(code, 1);
}

#[test]
fn appends_build_metadata() {
    let repo = Repo::new("metadata");
    repo.commit("a").tag("v1.0.0").commit("b");

    assert_eq!(
        repo.tag_cmd(&["--metadata", "deadbeef", "--print-only"]),
        ("v1.0.1+deadbeef".into(), 0),
    );
}

#[test]
fn numbers_successive_pre_releases() {
    let repo = Repo::new("suffix");
    repo.commit("a");

    // The first pre-release of a version carries no number...
    assert_eq!(
        repo.tag_cmd(&["--suffix", "rc", "--print-only"]),
        ("v0.0.0-rc".into(), 0)
    );

    // ...and later ones count up from it.
    repo.tag("v0.0.1-rc").commit("b");
    assert_eq!(
        repo.tag_cmd(&["--suffix", "rc", "--print-only"]),
        ("v0.0.1-rc.1".into(), 0)
    );

    repo.tag("v0.0.1-rc.1").commit("c");
    assert_eq!(
        repo.tag_cmd(&["--suffix", "rc", "--print-only"]),
        ("v0.0.1-rc.2".into(), 0)
    );

    // Switching to a suffix that has never been used restarts from the latest
    // *stable* version, and there is none here, so it falls back to v0.0.0.
    assert_eq!(
        repo.tag_cmd(&["--suffix", "beta", "--print-only"]),
        ("v0.0.0-beta".into(), 0)
    );

    // Dropping the suffix promotes to the next stable version.
    assert_eq!(repo.tag_cmd(&["--print-only"]), ("v0.0.2".into(), 0));
}

#[test]
fn bases_pre_releases_on_the_latest_stable_version() {
    let repo = Repo::new("stable-base");
    // The stable v0.0.1 outranks the older v0.0.0-rc, so it is the base.
    repo.commit("a").tag("v0.0.0-rc").tag("v0.0.1").commit("b");

    assert_eq!(
        repo.tag_cmd(&["--suffix", "alpha", "--print-only"]),
        ("v0.0.1-alpha".into(), 0)
    );
}

#[test]
fn isolates_prefixed_tags() {
    let repo = Repo::new("prefix");
    repo.commit("a").tag("v1.0.0").tag("org/v2.3.4").commit("b");

    assert_eq!(
        repo.tag_cmd(&["--prefix", "org", "--print-only"]),
        ("org/v2.3.5".into(), 0)
    );
    assert_eq!(
        repo.tag_cmd(&["--prefix", "org", "--minor", "--print-only"]),
        ("org/v2.4.0".into(), 0)
    );
    // The unprefixed lookup must ignore the prefixed tag entirely.
    assert_eq!(repo.tag_cmd(&["--print-only"]), ("v1.0.1".into(), 0));
}

#[test]
fn check_accepts_a_tag_following_its_predecessor() {
    let repo = Repo::new("check-ok");
    repo.commit("a").tag("v1.0.0").commit("b").tag("v1.0.1");

    let (output, code) = repo.tag_cmd(&["--check"]);
    assert_eq!(
        output,
        "Tag 'v1.0.1' is valid (previous version 'v1.0.0' is an ancestor)"
    );
    assert_eq!(code, 0);
}

#[test]
fn check_accepts_the_first_version() {
    let repo = Repo::new("check-first");
    repo.commit("a").tag("v0.0.0");

    let (output, code) = repo.tag_cmd(&["--check"]);
    assert_eq!(output, "Tag 'v0.0.0' is valid (first version)");
    assert_eq!(code, 0);
}

#[test]
fn check_rejects_a_skipped_version() {
    let repo = Repo::new("check-skip");
    repo.commit("a").tag("v9.9.9");

    let (output, code) = repo.tag_cmd(&["--check"]);
    assert_eq!(
        output,
        "Error: Expected predecessor tag 'v9.9.8' does not exist (version skipped)"
    );
    assert_eq!(code, 1);
}

#[test]
fn check_rejects_a_larger_version_in_history() {
    let repo = Repo::new("check-larger");
    // v0.0.3 sits on top of the much larger v9.9.9, which is out of order.
    repo.commit("a")
        .tag("v9.9.9")
        .commit("b")
        .tag("v0.0.2")
        .tag("v0.0.3");

    let (output, code) = repo.tag_cmd(&["--check"]);
    assert!(
        output.contains("Larger tag 'v9.9.9' is an ancestor"),
        "{output}"
    );
    assert_eq!(code, 1);
}

#[test]
fn check_rejects_an_untagged_head_unless_allowed() {
    let repo = Repo::new("check-untagged");
    repo.commit("a");

    assert_eq!(
        repo.tag_cmd(&["--check"]),
        ("Error: HEAD is not tagged".into(), 1)
    );
    assert_eq!(
        repo.tag_cmd(&["--check", "--allow-untagged"]),
        ("HEAD is not tagged (allowed)".into(), 0),
    );
}

#[test]
fn check_requires_a_predecessor_that_is_an_ancestor() {
    let repo = Repo::new("check-ancestor");
    // v1.0.0 lives on a branch that v1.0.1 does not descend from.
    repo.commit("a");
    repo.git(&["checkout", "-qb", "side"]);
    repo.commit("side").tag("v1.0.0");
    repo.git(&["checkout", "-q", "main"]);
    repo.commit("b").tag("v1.0.1");

    let (output, code) = repo.tag_cmd(&["--check"]);
    assert!(
        output.contains("is not an ancestor of current tag"),
        "{output}"
    );
    assert_eq!(code, 1);
}

#[test]
fn push_creates_and_pushes_the_tag() {
    let repo = Repo::new("push");
    repo.commit("a").tag("v1.0.0").commit("b").with_remote();

    // Runs without --no-fetch, so this also exercises fetching from the remote.
    let (output, code) = repo.run(&["--push"]);
    assert_eq!(output, "Tag 'v1.0.1' created and pushed to origin.");
    assert_eq!(code, 0);

    assert!(repo.tags().contains(&"v1.0.1".to_string()));
    let pushed = repo.git(&["ls-remote", "--tags", "origin"]);
    assert!(pushed.contains("refs/tags/v1.0.1"), "{pushed}");
}

#[test]
fn print_only_does_not_create_a_tag() {
    let repo = Repo::new("print-only");
    repo.commit("a").tag("v1.0.0").commit("b");

    assert_eq!(repo.tag_cmd(&["--print-only"]), ("v1.0.1".into(), 0));
    assert_eq!(repo.tags(), vec!["v1.0.0".to_string()]);
}

#[test]
fn reports_when_the_next_tag_already_exists() {
    let repo = Repo::new("exists");
    // v1.0.1 already exists on a later commit, so it cannot be created again.
    repo.commit("a").tag("v1.0.0").commit("b").tag("v1.0.1");
    repo.git(&["checkout", "-q", "HEAD~1"]);
    repo.git(&["tag", "-d", "v1.0.0"]);
    repo.git(&["tag", "v1.0.0"]);
    repo.commit("c");

    let (output, code) = repo.tag_cmd(&["--print-only"]);
    assert_eq!(output, "Next tag 'v1.0.1' already exists.");
    assert_eq!(code, 1);
}

#[test]
fn generates_shell_completions() {
    let repo = Repo::new("completion");
    repo.commit("a");

    for shell in ["bash", "zsh", "fish", "powershell"] {
        let (output, code) = repo.run(&["completion", shell]);
        assert_eq!(code, 0, "{shell} exited non-zero");
        assert!(output.contains("tag"), "{shell} completion looks empty");
    }
}
