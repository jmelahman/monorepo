use std::process::{Command, Stdio};

use anyhow::{Context, Result};

use crate::semver::{self, Version};

/// Builds the glob `git` uses to narrow tags down to plausible semver ones.
/// The results still need parsing, since globs cannot express semver exactly.
fn tag_pattern(prefix: &str, suffix: &str) -> String {
    let mut pattern = String::from("v[0-9]*.[0-9]*.[0-9]*");
    if !suffix.is_empty() {
        pattern = format!("{pattern}-{suffix}*");
    }
    if !prefix.is_empty() {
        pattern = format!("{prefix}/{pattern}");
    }
    pattern
}

/// The `v0.0.0` tag to fall back on when a repository has no matching tags yet.
fn zero_tag(prefix: &str) -> String {
    if prefix.is_empty() {
        "v0.0.0".to_string()
    } else {
        format!("{prefix}/v0.0.0")
    }
}

fn expected_prefix(prefix: &str) -> String {
    if prefix.is_empty() {
        String::new()
    } else {
        format!("{prefix}/")
    }
}

fn git(args: &[&str]) -> Command {
    let mut cmd = Command::new("git");
    cmd.args(args);
    cmd
}

/// Runs a git command, capturing stdout and letting stderr through to the user.
fn capture(args: &[&str]) -> Result<Option<String>> {
    capture_with(args, Stdio::inherit())
}

/// Like [`capture`], but discards stderr. Used for probes whose failure is an
/// expected, handled outcome rather than something to report to the user.
fn capture_quiet(args: &[&str]) -> Result<Option<String>> {
    capture_with(args, Stdio::null())
}

fn capture_with(args: &[&str], stderr: Stdio) -> Result<Option<String>> {
    let output = git(args)
        .stderr(stderr)
        .output()
        .with_context(|| format!("failed to run `git {}`", args.join(" ")))?;
    if !output.status.success() {
        return Ok(None);
    }
    Ok(Some(
        String::from_utf8_lossy(&output.stdout).trim().to_string(),
    ))
}

/// Runs a git command for its exit status alone, distinguishing "no" (exit 1)
/// from an actual failure the way git's query commands report them.
fn succeeded(args: &[&str]) -> Result<bool> {
    let status = git(args)
        .stderr(Stdio::inherit())
        .status()
        .with_context(|| format!("failed to run `git {}`", args.join(" ")))?;
    match status.code() {
        Some(0) => Ok(true),
        Some(1) => Ok(false),
        _ => anyhow::bail!("`git {}` failed: {status}", args.join(" ")),
    }
}

fn lines(output: &str) -> Vec<String> {
    output
        .lines()
        .map(str::trim)
        .filter(|line| !line.is_empty())
        .map(str::to_string)
        .collect()
}

/// Returns the largest tag in `tags` that satisfies `accept`, if any.
fn largest_tag<F>(tags: &[String], accept: F) -> Option<String>
where
    F: Fn(&Version) -> bool,
{
    let mut largest: Option<(String, Version)> = None;

    for tag in tags {
        let version = match semver::parse(tag) {
            Ok(version) => version,
            Err(err) => {
                log::debug!("largest_tag: failed to parse tag {tag}: {err}");
                continue;
            }
        };
        if !accept(&version) {
            log::debug!("largest_tag: skipping tag {tag}");
            continue;
        }
        if largest.as_ref().is_none_or(|(_, best)| version.gt(best)) {
            log::debug!("largest_tag: {tag} is the largest so far");
            largest = Some((tag.clone(), version));
        }
    }

    largest.map(|(tag, _)| tag)
}

pub fn get_latest_semver_tag(prefix: &str, suffix: &str) -> Result<String> {
    let pattern = tag_pattern(prefix, suffix);
    log::debug!("get_latest_semver_tag: pattern={pattern} prefix={prefix} suffix={suffix}");

    // A repository with no matching tags is an ordinary case, not an error.
    let matched = capture_quiet(&["describe", "--tags", "--abbrev=0", "--match", &pattern])?;
    let Some(matched_tag) = matched.filter(|tag| !tag.is_empty()) else {
        log::debug!("get_latest_semver_tag: no tags found matching pattern, returning v0.0.0");
        return Ok(zero_tag(prefix));
    };
    log::debug!("get_latest_semver_tag: git describe matched tag {matched_tag}");

    // `git describe` reports only one of possibly several tags on that commit,
    // so re-examine every tag there and pick the largest.
    let Some(tags_at) = list_tags_at(&matched_tag)? else {
        log::debug!("get_latest_semver_tag: error listing tags at {matched_tag}");
        return Ok(zero_tag(prefix));
    };
    log::debug!(
        "get_latest_semver_tag: found {} tags at {matched_tag}",
        tags_at.len()
    );

    let want_prefix = expected_prefix(prefix);
    let latest = largest_tag(&tags_at, |version| {
        version.prefix == want_prefix && (suffix.is_empty() || version.pre_release == suffix)
    })
    .unwrap_or_default();

    log::debug!("get_latest_semver_tag: returning latest tag {latest}");
    Ok(latest)
}

/// Returns the latest stable (non-pre-release) semver tag.
pub fn get_latest_stable_semver_tag(prefix: &str) -> Result<String> {
    let pattern = tag_pattern(prefix, "");
    log::debug!("get_latest_stable_semver_tag: pattern={pattern} prefix={prefix}");

    let Some(output) = capture(&["tag", "-l", &pattern])? else {
        log::debug!("get_latest_stable_semver_tag: error listing tags");
        return Ok(zero_tag(prefix));
    };

    let tags = lines(&output);
    log::debug!("get_latest_stable_semver_tag: found {} tags", tags.len());

    let want_prefix = expected_prefix(prefix);
    let latest = largest_tag(&tags, |version| {
        version.prefix == want_prefix && version.pre_release.is_empty()
    })
    .unwrap_or_else(|| zero_tag(prefix));

    log::debug!("get_latest_stable_semver_tag: returning latest stable tag {latest}");
    Ok(latest)
}

pub fn list_tags(prefix: &str, suffix: &str) -> Result<Vec<String>> {
    let pattern = tag_pattern(prefix, suffix);
    log::debug!("list_tags: pattern={pattern} prefix={prefix} suffix={suffix}");

    let output = capture(&["tag", "-l", &pattern])?.context("failed to list tags")?;
    let tags = lines(&output);
    log::debug!(
        "list_tags: git returned {} tags (before filtering)",
        tags.len()
    );

    if suffix.is_empty() {
        return Ok(tags);
    }

    // The glob cannot pin the suffix exactly, so filter it properly here.
    let filtered: Vec<String> = tags
        .into_iter()
        .filter(|tag| match semver::parse(tag) {
            Ok(version) => version.pre_release == suffix,
            Err(err) => {
                log::debug!("list_tags: failed to parse tag {tag}: {err}");
                false
            }
        })
        .collect();
    log::debug!("list_tags: {} tags match suffix {suffix}", filtered.len());

    Ok(filtered)
}

/// Returns the tags pointing at `git_ref`, or `None` if the ref cannot be read.
fn list_tags_at(git_ref: &str) -> Result<Option<Vec<String>>> {
    Ok(capture(&["tag", "--points-at", git_ref])?.map(|output| lines(&output)))
}

/// Returns the semver tags at HEAD matching `prefix`/`suffix`.
fn tags_at_head(prefix: &str, suffix: &str) -> Result<Vec<String>> {
    let pattern = tag_pattern(prefix, suffix);
    log::debug!("tags_at_head: pattern={pattern} prefix={prefix} suffix={suffix}");

    let output = capture(&["tag", "--points-at", "HEAD", "--list", &pattern])?
        .context("failed to check tags for HEAD")?;
    Ok(lines(&output))
}

/// Returns the largest semver tag at HEAD, or `None` if there is none.
pub fn get_tag_at_head(prefix: &str, suffix: &str) -> Result<Option<String>> {
    let tags = tags_at_head(prefix, suffix)?;
    if tags.is_empty() {
        log::debug!("get_tag_at_head: no tags found at HEAD");
        return Ok(None);
    }

    let tag = largest_tag(&tags, |version| {
        suffix.is_empty() || version.pre_release == suffix
    });
    log::debug!("get_tag_at_head: returning tag at HEAD {tag:?}");
    Ok(tag)
}

pub fn is_head_already_tagged(prefix: &str, suffix: &str) -> Result<bool> {
    Ok(get_tag_at_head(prefix, suffix)?.is_some())
}

pub fn tag_exists(tag: &str) -> Result<bool> {
    log::debug!("tag_exists: checking if tag {tag} exists");
    let tag_ref = format!("refs/tags/{tag}");
    let exists = succeeded(&["show-ref", "--tags", "--quiet", &tag_ref])?;
    log::debug!("tag_exists: {tag} exists={exists}");
    Ok(exists)
}

/// Checks whether `ancestor_ref` is an ancestor of `descendant_ref`.
pub fn is_ancestor(ancestor_ref: &str, descendant_ref: &str) -> Result<bool> {
    log::debug!("is_ancestor: checking if {ancestor_ref} is an ancestor of {descendant_ref}");
    succeeded(&["merge-base", "--is-ancestor", ancestor_ref, descendant_ref])
}

pub fn create_and_push_tag(tag: &str, remote: &str) -> Result<()> {
    log::debug!("create_and_push_tag: creating tag {tag}");
    let status = git(&["tag", tag]).stderr(Stdio::inherit()).status()?;
    if !status.success() {
        anyhow::bail!("failed to create tag");
    }

    log::debug!("create_and_push_tag: pushing tag {tag} to {remote}");
    let status = git(&["push", "--quiet", remote, tag])
        .stderr(Stdio::inherit())
        .status()?;
    if !status.success() {
        anyhow::bail!("failed to push tag to {remote}");
    }

    log::debug!("create_and_push_tag: successfully created and pushed tag {tag}");
    Ok(())
}

pub fn fetch_semver_tags(remote: &str, prefix: &str, suffix: &str) -> Result<()> {
    // Refspecs cannot express a wildcard in the middle (`v*-suffix*`), so fetch
    // every version tag greedily and filter by suffix later in the process.
    let refspec = if prefix.is_empty() {
        "refs/tags/v*:refs/tags/v*".to_string()
    } else {
        format!("refs/tags/{prefix}/v*:refs/tags/{prefix}/v*")
    };
    log::debug!(
        "fetch_semver_tags: fetching from {remote} (refspec={refspec}, suffix={suffix} \
         will be filtered later)"
    );

    let status = git(&["fetch", "--quiet", "--prune", remote, &refspec])
        .stderr(Stdio::inherit())
        .status()?;
    if !status.success() {
        anyhow::bail!("failed to fetch tags from {remote}");
    }

    log::debug!("fetch_semver_tags: successfully fetched tags");
    Ok(())
}
