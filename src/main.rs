mod completion;
mod git;
mod logger;
mod semver;

use std::io::{self, Write};
use std::process::ExitCode;
use std::sync::LazyLock;

use anyhow::{bail, Result};
use clap::{ArgAction, CommandFactory, Parser, Subcommand};

use completion::Shell;

/// Injected at build time by the release pipeline; `dev`/`none` locally.
static VERSION: LazyLock<String> = LazyLock::new(|| {
    format!(
        "{}\ncommit {}",
        option_env!("TAG_VERSION").unwrap_or("dev"),
        option_env!("TAG_COMMIT").unwrap_or("none"),
    )
});

#[derive(Parser, Debug)]
#[command(
    name = "tag",
    about = "Calculate the next semantic version tag",
    version = VERSION.as_str(),
    disable_version_flag = true,
)]
struct Cli {
    /// increment the major version
    #[arg(long)]
    major: bool,

    /// increment the minor version
    #[arg(long)]
    minor: bool,

    /// increment the patch version
    #[arg(long)]
    patch: bool,

    /// create and push the tag to remote
    #[arg(long)]
    push: bool,

    /// print the next tag and exit
    #[arg(long = "print-only")]
    print_only: bool,

    /// validate that the tag at HEAD has its previous version as an ancestor
    #[arg(long)]
    check: bool,

    /// skip fetching tags from remote
    #[arg(long = "no-fetch")]
    no_fetch: bool,

    /// allow HEAD to be untagged when using --check
    #[arg(long = "allow-untagged")]
    allow_untagged: bool,

    /// enable debug logging
    #[arg(long)]
    debug: bool,

    /// set a prefix for the tag
    #[arg(long, default_value = "")]
    prefix: String,

    /// set the pre-release suffix (e.g., rc, alpha, beta)
    #[arg(long, default_value = "")]
    suffix: String,

    /// set the build metadata
    #[arg(long, default_value = "")]
    metadata: String,

    /// remote repository to push tag to
    #[arg(long, default_value = "origin")]
    remote: String,

    /// version for tag
    #[arg(short = 'v', long = "version", action = ArgAction::Version)]
    version: (),

    #[command(subcommand)]
    command: Option<Command>,
}

#[derive(Subcommand, Debug)]
enum Command {
    /// Generate completion script
    #[command(long_about = completion::LONG_HELP.as_str())]
    Completion {
        #[arg(value_name = "shell")]
        shell: Shell,
    },
}

fn main() -> ExitCode {
    let cli = Cli::parse();

    logger::init(cli.debug);

    match run(&cli) {
        Ok(code) => code,
        Err(err) => {
            // Matches the Go implementation, which reported errors on stdout.
            println!("Error: {err}");
            ExitCode::FAILURE
        }
    }
}

fn run(cli: &Cli) -> Result<ExitCode> {
    if let Some(Command::Completion { shell }) = &cli.command {
        completion::generate(*shell, &mut Cli::command());
        return Ok(ExitCode::SUCCESS);
    }

    if [cli.major, cli.minor, cli.patch]
        .iter()
        .filter(|set| **set)
        .count()
        > 1
    {
        bail!(
            "only one version increment flag (--major, --minor, or --patch) \
             can be used at a time"
        );
    }

    log::debug!(
        "Configuration (prefix={} suffix={} remote={} major={} minor={} patch={} check={} \
         noFetch={} allowUntagged={})",
        cli.prefix,
        cli.suffix,
        cli.remote,
        cli.major,
        cli.minor,
        cli.patch,
        cli.check,
        cli.no_fetch,
        cli.allow_untagged,
    );

    if !cli.no_fetch {
        git::fetch_semver_tags(&cli.remote, &cli.prefix, &cli.suffix)?;
    }

    if cli.check {
        return check(cli);
    }

    next(cli)
}

/// Validates that the tag at HEAD follows on from its predecessor: no larger
/// version is already an ancestor, and the version immediately before it exists
/// and is an ancestor.
fn check(cli: &Cli) -> Result<ExitCode> {
    let Some(current_tag) = git::get_tag_at_head(&cli.prefix, &cli.suffix)? else {
        if cli.allow_untagged {
            println!("HEAD is not tagged (allowed)");
            return Ok(ExitCode::SUCCESS);
        }
        println!("Error: HEAD is not tagged");
        return Ok(ExitCode::FAILURE);
    };

    let all_tags = git::list_tags(&cli.prefix, &cli.suffix)?;

    // Nothing larger than the tag at HEAD may already be in its history.
    for larger_tag in semver::find_larger_versions(&current_tag, &all_tags)? {
        if git::is_ancestor(&larger_tag, "HEAD")? {
            println!(
                "Error: Larger tag '{larger_tag}' is an ancestor of current tag '{current_tag}'"
            );
            return Ok(ExitCode::FAILURE);
        }
    }

    // The expected predecessor must exist, so that no version was skipped.
    let Some(expected_pred) = semver::expected_predecessor(&current_tag)? else {
        println!("Tag '{current_tag}' is valid (first version)");
        return Ok(ExitCode::SUCCESS);
    };

    if !git::tag_exists(&expected_pred)? {
        println!(
            "Error: Expected predecessor tag '{expected_pred}' does not exist (version skipped)"
        );
        return Ok(ExitCode::FAILURE);
    }

    if !git::is_ancestor(&expected_pred, "HEAD")? {
        println!(
            "Error: Previous tag '{expected_pred}' is not an ancestor of current tag \
             '{current_tag}'"
        );
        return Ok(ExitCode::FAILURE);
    }

    println!("Tag '{current_tag}' is valid (previous version '{expected_pred}' is an ancestor)");
    Ok(ExitCode::SUCCESS)
}

/// Calculates the next tag and, unless only printing, creates and pushes it.
fn next(cli: &Cli) -> Result<ExitCode> {
    if git::is_head_already_tagged(&cli.prefix, &cli.suffix)? {
        println!("Error: Current HEAD is already tagged");
        return Ok(ExitCode::FAILURE);
    }

    let mut latest_tag = git::get_latest_semver_tag(&cli.prefix, &cli.suffix)?;

    // When building a pre-release, prefer the latest stable tag as the base so
    // the new pre-release is never derived from an older version than it.
    if !cli.suffix.is_empty() {
        if let Some(stable_tag) = stable_base(&cli.prefix, &latest_tag) {
            log::debug!(
                "Using stable tag as base (higher or equal base version) \
                 (stableTag={stable_tag} preReleaseTag={latest_tag})"
            );
            latest_tag = stable_tag;
        }
    }

    let all_tags = git::list_tags(&cli.prefix, &cli.suffix)?;

    let mut next_version = semver::calculate_next_version(
        &latest_tag,
        &all_tags,
        cli.major,
        cli.minor,
        cli.patch,
        &cli.suffix,
    )?;

    if !cli.metadata.is_empty() {
        next_version = format!("{next_version}+{}", cli.metadata);
    }

    if git::tag_exists(&next_version)? {
        println!("Next tag '{next_version}' already exists.");
        return Ok(ExitCode::FAILURE);
    }

    if cli.print_only {
        println!("{next_version}");
        return Ok(ExitCode::SUCCESS);
    }

    if !cli.push && !confirm(&next_version, &cli.remote)? {
        return Ok(ExitCode::SUCCESS);
    }

    git::create_and_push_tag(&next_version, &cli.remote)?;
    println!("Tag '{next_version}' created and pushed to {}.", cli.remote);
    Ok(ExitCode::SUCCESS)
}

/// Returns the latest stable tag if its base version is at least as high as
/// `latest_tag`'s, meaning it should be used as the base instead.
fn stable_base(prefix: &str, latest_tag: &str) -> Option<String> {
    let stable_tag = git::get_latest_stable_semver_tag(prefix).ok()?;
    if stable_tag.is_empty() {
        return None;
    }

    let stable = semver::parse(&stable_tag).ok()?.base();
    let current = semver::parse(latest_tag).ok()?.base();

    (stable.gt(&current) || stable == current).then_some(stable_tag)
}

/// Asks whether to push the tag. An empty response accepts.
fn confirm(next_version: &str, remote: &str) -> Result<bool> {
    print!("Push tag '{next_version}' to {remote}? (y/N): ");
    io::stdout().flush()?;

    let mut response = String::new();
    io::stdin().read_line(&mut response)?;

    let response = response.trim().to_lowercase();
    Ok(matches!(response.as_str(), "" | "y" | "yes"))
}
