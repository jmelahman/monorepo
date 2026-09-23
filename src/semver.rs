use std::fmt;

use anyhow::{anyhow, Result};

#[derive(Debug, Default, Clone, PartialEq, Eq)]
pub struct Version {
    pub prefix: String,
    pub major: u64,
    pub minor: u64,
    pub patch: u64,
    pub pre_release: String,
    pub pre_release_num: u64,
}

impl fmt::Display for Version {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(
            f,
            "{}v{}.{}.{}",
            self.prefix, self.major, self.minor, self.patch
        )?;
        if !self.pre_release.is_empty() {
            write!(f, "-{}", self.pre_release)?;
            if self.pre_release_num > 0 {
                write!(f, ".{}", self.pre_release_num)?;
            }
        }
        Ok(())
    }
}

impl Version {
    /// The base version with any pre-release information stripped.
    pub fn base(&self) -> Version {
        Version {
            prefix: self.prefix.clone(),
            major: self.major,
            minor: self.minor,
            patch: self.patch,
            ..Default::default()
        }
    }

    /// Reports whether `self` is strictly greater than `other`.
    ///
    /// Note this is not a total order: two versions sharing a base but carrying
    /// differently-named pre-releases always compare as `false` in both
    /// directions, since there is no meaningful precedence between, say,
    /// `-alpha` and `-rc`.
    pub fn gt(&self, other: &Version) -> bool {
        let lhs = (self.major, self.minor, self.patch);
        let rhs = (other.major, other.minor, other.patch);
        if lhs != rhs {
            return lhs > rhs;
        }

        match (self.pre_release.is_empty(), other.pre_release.is_empty()) {
            // A stable release outranks a pre-release of the same base version.
            (true, false) => true,
            (false, true) => false,
            _ if self.pre_release == other.pre_release => {
                self.pre_release_num > other.pre_release_num
            }
            _ => false,
        }
    }
}

/// Splits `s` into the leading run of ASCII digits and the rest.
fn take_digits(s: &str) -> Option<(u64, &str)> {
    let end = s.find(|c: char| !c.is_ascii_digit()).unwrap_or(s.len());
    if end == 0 {
        return None;
    }
    // A run of digits too long for a u64 saturates rather than failing the
    // match, so an absurd version still parses as a very large version.
    Some((s[..end].parse().unwrap_or(u64::MAX), &s[end..]))
}

/// Splits `s` into the leading run of ASCII letters and the rest.
fn take_letters(s: &str) -> Option<(&str, &str)> {
    let end = s
        .find(|c: char| !c.is_ascii_alphabetic())
        .unwrap_or(s.len());
    if end == 0 {
        return None;
    }
    Some((&s[..end], &s[end..]))
}

/// Matches `v<major>.<minor>.<patch>[-<pre_release>[.<num>]]` at the start of
/// `s`, ignoring any trailing text such as `+<build metadata>`.
fn match_core(s: &str) -> Option<Version> {
    let rest = s.strip_prefix('v')?;
    let (major, rest) = take_digits(rest)?;
    let (minor, rest) = take_digits(rest.strip_prefix('.')?)?;
    let (patch, rest) = take_digits(rest.strip_prefix('.')?)?;

    let mut version = Version {
        major,
        minor,
        patch,
        ..Default::default()
    };

    // The pre-release is optional, and so is its trailing number; neither
    // failing to match invalidates the version parsed so far.
    if let Some((pre_release, rest)) = rest.strip_prefix('-').and_then(take_letters) {
        version.pre_release = pre_release.to_string();
        if let Some((num, _)) = rest.strip_prefix('.').and_then(take_digits) {
            version.pre_release_num = num;
        }
    }

    Some(version)
}

/// Matches a version at `s`, optionally preceded by a `<prefix>/` path.
///
/// The prefix is greedy, so `a/b/v1.2.3` yields the prefix `a/b/`.
fn match_at(s: &str) -> Option<Version> {
    // Prefer the longest prefix, falling back to shorter ones and finally to
    // no prefix at all.
    for (offset, _) in s.rmatch_indices('/') {
        if offset == 0 {
            break;
        }
        if let Some(mut version) = match_core(&s[offset + 1..]) {
            version.prefix = s[..=offset].to_string();
            return Some(version);
        }
    }
    match_core(s)
}

pub fn parse(tag: &str) -> Result<Version> {
    // Scan forward for the leftmost position that yields a version, matching
    // the unanchored search the original regex performed.
    tag.char_indices()
        .find_map(|(i, _)| match_at(&tag[i..]))
        .ok_or_else(|| anyhow!("invalid semver tag: {tag}"))
}

/// Returns the expected immediate predecessor version, or `None` when the tag
/// is the first possible version and so has no predecessor.
///
/// For `v1.2.3-rc.2`, this returns `v1.2.3-rc.1`
/// For `v1.2.3-rc.1`, this returns `v1.2.3-rc`
/// For `v1.2.3-rc`, this returns `v1.2.2` (pre-release with no num falls back to stable decrement)
/// For `v1.1.2`, this returns `v1.1.1`
/// For `v1.1.0`, this returns `v1.0.0` (expects previous minor to exist)
/// For `v1.0.0`, this returns `v0.0.0` (expects previous major to exist)
/// For `v0.0.0`, this returns `None` (no predecessor expected)
pub fn expected_predecessor(current_tag: &str) -> Result<Option<String>> {
    let mut pred = parse(current_tag)?;

    if pred.pre_release_num > 0 {
        pred.pre_release_num -= 1;
        return Ok(Some(pred.to_string()));
    }

    // No pre-release num: strip the pre-release and decrement the base version.
    pred.pre_release = String::new();

    if pred.patch > 0 {
        pred.patch -= 1;
    } else if pred.minor > 0 {
        pred.minor -= 1;
    } else if pred.major > 0 {
        pred.major -= 1;
    } else {
        return Ok(None);
    }

    Ok(Some(pred.to_string()))
}

/// Returns all stable version tags that are greater than the given tag.
pub fn find_larger_versions(current_tag: &str, all_tags: &[String]) -> Result<Vec<String>> {
    let current = parse(current_tag)?;
    let current_stable = current.base();

    let larger = all_tags
        .iter()
        .filter(|tag| !tag.is_empty() && tag.as_str() != current_tag)
        .filter(|tag| match parse(tag) {
            Ok(version) => {
                // Only stable tags sharing the current prefix are comparable.
                version.prefix == current.prefix
                    && version.pre_release.is_empty()
                    && version.gt(&current_stable)
            }
            Err(err) => {
                log::debug!("find_larger_versions: failed to parse tag {tag}: {err}");
                false
            }
        })
        .cloned()
        .collect();

    Ok(larger)
}

pub fn calculate_next_version(
    tag: &str,
    all_tags: &[String],
    inc_major: bool,
    inc_minor: bool,
    inc_patch: bool,
    suffix: &str,
) -> Result<String> {
    log::debug!(
        "Calculating next version (latest={tag} major={inc_major} minor={inc_minor} \
         patch={inc_patch} suffix={suffix})"
    );
    let mut version = parse(tag)?;

    if inc_major {
        version.major += 1;
        version.minor = 0;
        version.patch = 0;
        version.pre_release_num = 0;
    } else if inc_minor {
        version.minor += 1;
        version.patch = 0;
        version.pre_release_num = 0;
    } else if inc_patch || suffix.is_empty() {
        version.patch += 1;
        version.pre_release_num = 0;
    } else if version.pre_release != suffix {
        // Switching to a different pre-release identifier on the same base
        // version: continue from the highest existing number for that suffix.
        // TODO: This should probably only consider tags with HEAD as an ancestor.
        let largest = all_tags
            .iter()
            .filter_map(|existing| parse(existing).ok())
            .filter(|existing| existing.pre_release == suffix && existing.base() == version.base())
            .map(|existing| existing.pre_release_num)
            .max()
            .unwrap_or(0);
        if largest > 0 {
            version.pre_release_num = largest + 1;
        }
    } else {
        version.pre_release_num += 1;
    }

    version.pre_release = suffix.to_string();

    let next_version = version.to_string();
    log::debug!("Calculated next version: {next_version}");
    Ok(next_version)
}

#[cfg(test)]
mod tests {
    use super::*;

    fn version(major: u64, minor: u64, patch: u64) -> Version {
        Version {
            major,
            minor,
            patch,
            ..Default::default()
        }
    }

    fn pre_release(major: u64, minor: u64, patch: u64, name: &str, num: u64) -> Version {
        Version {
            major,
            minor,
            patch,
            pre_release: name.to_string(),
            pre_release_num: num,
            ..Default::default()
        }
    }

    /// Locks in the exact parsing behavior, including the quirks of the
    /// unanchored, greedy-prefix pattern this parser replaced. Every
    /// expectation here was verified against the original regex.
    #[test]
    fn parses_edge_cases() {
        // (tag, Some((prefix, major, minor, patch, pre_release, pre_release_num)))
        let cases = [
            ("v1.2.3", Some(("", 1, 2, 3, "", 0))),
            ("org/v1.2.3", Some(("org/", 1, 2, 3, "", 0))),
            // The prefix is greedy, so it swallows every leading path segment.
            ("a/b/v1.2.3", Some(("a/b/", 1, 2, 3, "", 0))),
            (
                "prefix/with/slashes/v9.9.9-beta.7",
                Some(("prefix/with/slashes/", 9, 9, 9, "beta", 7)),
            ),
            ("//v1.2.3", Some(("//", 1, 2, 3, "", 0))),
            ("v1.2.3/v4.5.6", Some(("v1.2.3/", 4, 5, 6, "", 0))),
            // A prefix needs at least one character before the slash.
            ("/v1.2.3", Some(("", 1, 2, 3, "", 0))),
            // The search is unanchored, so leading junk is skipped...
            ("xxv1.2.3", Some(("", 1, 2, 3, "", 0))),
            ("-v1.2.3", Some(("", 1, 2, 3, "", 0))),
            // ...and trailing junk, including build metadata, is ignored.
            ("v1.2.3+21AF26D3", Some(("", 1, 2, 3, "", 0))),
            ("v1.2.3.4", Some(("", 1, 2, 3, "", 0))),
            ("v1.2.3-rc.1-extra", Some(("", 1, 2, 3, "rc", 1))),
            // A pre-release that does not match in full is dropped, not fatal.
            ("v1.2.3-rc", Some(("", 1, 2, 3, "rc", 0))),
            ("v1.2.3-rc.x", Some(("", 1, 2, 3, "rc", 0))),
            ("v1.2.3-1", Some(("", 1, 2, 3, "", 0))),
            ("v1.2.3-RC.2", Some(("", 1, 2, 3, "RC", 2))),
            // Leading zeroes are accepted and normalized away.
            ("v01.02.03", Some(("", 1, 2, 3, "", 0))),
            ("v1.2.3-rc.01", Some(("", 1, 2, 3, "rc", 1))),
            ("invalid-tag", None),
            ("v1.2", None),
            ("v-1.2.3", None),
        ];

        for (tag, expected) in cases {
            let expected = expected.map(|(prefix, major, minor, patch, pre, num)| Version {
                prefix: prefix.to_string(),
                major,
                minor,
                patch,
                pre_release: pre.to_string(),
                pre_release_num: num,
            });
            assert_eq!(parse(tag).ok(), expected, "mismatch for input {tag:?}");
        }
    }

    #[test]
    fn parses_tags() {
        let cases = [
            ("standard version", "v1.2.3", version(1, 2, 3)),
            (
                "prefixed version",
                "org/v1.2.3",
                Version {
                    prefix: "org/".into(),
                    ..version(1, 2, 3)
                },
            ),
            (
                "pre-release version",
                "v1.2.3-rc",
                pre_release(1, 2, 3, "rc", 0),
            ),
            (
                "prefixed pre-release version",
                "org/v1.2.3-rc.1",
                Version {
                    prefix: "org/".into(),
                    ..pre_release(1, 2, 3, "rc", 1)
                },
            ),
            (
                "build metadata is ignored",
                "v1.2.3+21AF26D3",
                version(1, 2, 3),
            ),
        ];

        for (name, tag, expected) in cases {
            assert_eq!(parse(tag).unwrap(), expected, "{name}");
        }
    }

    #[test]
    fn rejects_invalid_tags() {
        assert!(parse("invalid-tag").is_err());
        assert!(parse("not-a-version").is_err());
    }

    #[test]
    fn formats_round_trip() {
        for tag in ["v1.2.3", "org/v1.2.3", "v1.2.3-rc", "org/v1.2.3-rc.1"] {
            assert_eq!(parse(tag).unwrap().to_string(), tag);
        }
    }

    #[test]
    fn compares_versions() {
        let cases = [
            (
                "v1 major version higher",
                version(2, 0, 0),
                version(1, 9, 9),
                true,
            ),
            (
                "v1 minor version higher",
                version(1, 2, 0),
                version(1, 1, 9),
                true,
            ),
            (
                "v1 patch version higher",
                version(1, 1, 2),
                version(1, 1, 1),
                true,
            ),
            (
                "v1 pre-release version higher",
                pre_release(1, 1, 1, "rc", 2),
                pre_release(1, 1, 1, "rc", 1),
                true,
            ),
            (
                "identical versions",
                version(1, 1, 1),
                version(1, 1, 1),
                false,
            ),
            (
                "major version lower",
                version(1, 9, 9),
                version(2, 0, 0),
                false,
            ),
            (
                "stable outranks pre-release",
                version(1, 1, 1),
                pre_release(1, 1, 1, "rc", 1),
                true,
            ),
            (
                "pre-release does not outrank stable",
                pre_release(1, 1, 1, "rc", 1),
                version(1, 1, 1),
                false,
            ),
            (
                "differently named pre-releases are incomparable",
                pre_release(1, 1, 1, "rc", 1),
                pre_release(1, 1, 1, "alpha", 9),
                false,
            ),
        ];

        for (name, v1, v2, expected) in cases {
            assert_eq!(v1.gt(&v2), expected, "{name}");
        }
    }

    #[test]
    fn finds_expected_predecessor() {
        let cases = [
            ("patch decrement", "v1.1.2", Some("v1.1.1")),
            ("minor decrement", "v1.1.0", Some("v1.0.0")),
            ("major decrement", "v1.0.0", Some("v0.0.0")),
            ("no predecessor for v0.0.0", "v0.0.0", None),
            (
                "pre-release num > 1 decrements num",
                "v1.2.3-rc.2",
                Some("v1.2.3-rc.1"),
            ),
            (
                "pre-release num 1 drops to base pre-release",
                "v1.2.3-rc.1",
                Some("v1.2.3-rc"),
            ),
            (
                "pre-release with no num falls back to stable",
                "v1.2.3-rc",
                Some("v1.2.2"),
            ),
            (
                "pre-release with no num on minor boundary",
                "v1.2.0-alpha",
                Some("v1.1.0"),
            ),
            (
                "pre-release with no num on major boundary",
                "v2.0.0-beta",
                Some("v1.0.0"),
            ),
            (
                "prefixed pre-release num decrement",
                "org/v1.0.0-rc.3",
                Some("org/v1.0.0-rc.2"),
            ),
            (
                "pre-release v0.0.0 has no predecessor",
                "v0.0.0-alpha",
                None,
            ),
        ];

        for (name, tag, expected) in cases {
            let actual = expected_predecessor(tag).unwrap();
            assert_eq!(actual.as_deref(), expected, "{name}");
        }
    }

    #[test]
    fn expected_predecessor_rejects_invalid_tags() {
        assert!(expected_predecessor("not-a-version").is_err());
    }

    #[test]
    fn finds_larger_versions() {
        let tags: Vec<String> = ["v1.0.0", "v1.2.3", "v1.3.0", "v2.0.0", "v1.3.0-rc.1"]
            .iter()
            .map(|s| s.to_string())
            .collect();

        assert_eq!(
            find_larger_versions("v1.2.3", &tags).unwrap(),
            vec!["v1.3.0".to_string(), "v2.0.0".to_string()],
        );
        assert!(find_larger_versions("v2.0.0", &tags).unwrap().is_empty());
    }

    #[test]
    fn find_larger_versions_ignores_other_prefixes() {
        let tags: Vec<String> = ["org/v2.0.0", "v3.0.0"]
            .iter()
            .map(|s| s.to_string())
            .collect();

        assert_eq!(
            find_larger_versions("org/v1.0.0", &tags).unwrap(),
            vec!["org/v2.0.0".to_string()],
        );
    }

    #[test]
    fn calculates_next_version() {
        /// (name, current tag, [major, minor, patch], suffix, all tags, expected)
        type Case = (
            &'static str,
            &'static str,
            [bool; 3],
            &'static str,
            &'static [&'static str],
            &'static str,
        );

        let cases: [Case; 21] = [
            (
                "patch increment",
                "v1.2.3",
                [false, false, false],
                "",
                &[],
                "v1.2.4",
            ),
            (
                "patch increment with incPatch",
                "v1.2.3",
                [false, false, true],
                "",
                &[],
                "v1.2.4",
            ),
            (
                "next tag already exists",
                "v1.2.3",
                [false, false, false],
                "",
                &["v1.2.3", "v1.3.0"],
                "v1.2.4",
            ),
            (
                "patch increment with suffix",
                "v1.2.3",
                [false, false, true],
                "alpha",
                &[],
                "v1.2.4-alpha",
            ),
            (
                "minor increment",
                "v1.2.3",
                [false, true, false],
                "",
                &[],
                "v1.3.0",
            ),
            (
                "minor increment with suffix",
                "v1.2.3",
                [false, true, false],
                "alpha",
                &[],
                "v1.3.0-alpha",
            ),
            (
                "major increment",
                "v1.2.3",
                [true, false, false],
                "",
                &[],
                "v2.0.0",
            ),
            (
                "major increment with suffix",
                "v1.2.3",
                [true, false, false],
                "alpha",
                &[],
                "v2.0.0-alpha",
            ),
            (
                "pre-release increment with pre-release version",
                "v1.2.3-rc.1",
                [false, false, false],
                "rc",
                &[],
                "v1.2.3-rc.2",
            ),
            (
                "pre-release increment with patch version",
                "v1.2.3-rc.1",
                [false, false, true],
                "rc",
                &[],
                "v1.2.4-rc",
            ),
            (
                "pre-release increment with minor version",
                "v1.2.3-rc.1",
                [false, true, false],
                "rc",
                &[],
                "v1.3.0-rc",
            ),
            (
                "pre-release increment with major version",
                "v1.2.3-rc.1",
                [true, false, false],
                "rc",
                &[],
                "v2.0.0-rc",
            ),
            (
                "add suffix to version",
                "v1.2.3",
                [false, false, false],
                "beta",
                &[],
                "v1.2.3-beta",
            ),
            (
                "add suffix to pre-release version",
                "v1.1.1-beta",
                [false, false, false],
                "beta",
                &[],
                "v1.1.1-beta.1",
            ),
            (
                "override existing pre-release keeps the current number",
                "v1.2.3-rc.1",
                [false, false, false],
                "beta",
                &[],
                "v1.2.3-beta.1",
            ),
            (
                "override existing pre-release continues numbering",
                "v1.2.3-rc.1",
                [false, false, false],
                "beta",
                &["v1.2.3-rc.1", "v1.2.3-beta.2"],
                "v1.2.3-beta.3",
            ),
            (
                "override existing pre-release with matching suffix",
                "v1.2.3-rc.1",
                [false, false, false],
                "rc",
                &["v1.2.3-rc.1", "v1.2.3-beta.2"],
                "v1.2.3-rc.2",
            ),
            (
                "override existing without suffix",
                "v1.2.3-rc.1",
                [false, false, false],
                "",
                &["v1.2.3-rc.1", "v1.2.3-beta.2"],
                "v1.2.4",
            ),
            (
                "suffix with stable version base (not old pre-release)",
                "v1.0.1",
                [false, false, false],
                "alpha",
                &["v1.0.0", "v1.0.0-alpha", "v1.0.1"],
                "v1.0.1-alpha",
            ),
            (
                "suffix with stable version base",
                "v1.0.1",
                [false, false, false],
                "alpha",
                &["v1.0.0", "v1.0.0-alpha.1", "v1.0.1"],
                "v1.0.1-alpha",
            ),
            (
                "prefix is preserved",
                "org/v1.2.3",
                [false, false, false],
                "",
                &[],
                "org/v1.2.4",
            ),
        ];

        for (name, current, [major, minor, patch], suffix, all_tags, expected) in cases {
            let all_tags: Vec<String> = all_tags.iter().map(|s| s.to_string()).collect();
            let actual =
                calculate_next_version(current, &all_tags, major, minor, patch, suffix).unwrap();
            assert_eq!(actual, expected, "{name}");
        }
    }

    #[test]
    fn calculate_next_version_rejects_invalid_tags() {
        assert!(calculate_next_version("invalid-tag", &[], false, false, false, "").is_err());
    }
}
