locals {
  # Every repository a subtree mirrors, plus the monorepo itself. Adding a
  # subtree with `git orchard add` brings its repository under management.
  subtree_remotes = regexall(
    "remote = git@github.com:jmelahman/([^\\s]+)",
    file("${path.module}/../../.config/git-orchard/subtrees"),
  )
  repository_names = toset(concat(
    [for m in local.subtree_remotes : trimsuffix(m[0], ".git")],
    ["monorepo"],
  ))

  # Settings shared by every repository.
  defaults = {
    description                 = ""
    homepage_url                = ""
    topics                      = []
    is_template                 = false
    has_issues                  = true
    has_projects                = false
    has_wiki                    = false
    has_discussions             = false
    allow_merge_commit          = true
    allow_squash_merge          = true
    allow_rebase_merge          = true
    allow_auto_merge            = true
    allow_update_branch         = true
    delete_branch_on_merge      = true
    squash_merge_commit_title   = "PR_TITLE"
    squash_merge_commit_message = "PR_BODY"
    merge_commit_title          = "MERGE_MESSAGE"
    merge_commit_message        = "PR_TITLE"
    # Dependabot and secret scanning. Mirrors' alerts would duplicate the
    # monorepo's, where their code is actually developed.
    security = false
  }

  # Per-repository settings, merged over the defaults.
  overrides = {
    "5-wild" = {
      description  = "A word-guessing roguelike."
      homepage_url = "https://5-wild.com/"
      topics       = ["android", "balatro", "free", "game", "rougelike", "wordle"]
    }
    agentic-kanban = {
      description  = "Kanban board for managing AI agent sessions"
      homepage_url = "http://jamison.lahman.dev/agentic-kanban/"
    }
    AgileCBT = {
      description  = "Use Agile software development techniques for personal development"
      homepage_url = "http://jamison.lahman.dev/AgileCBT/"
    }
    check-symlinks = {
      description = "Check for broken symlinks"
    }
    connections = {
      description = "A command-line client for the NYT Connections game"
      topics      = ["cli", "game", "games", "go", "golang", "nyt-games", "nytimes", "tui"]
    }
    connections-ssh = {
      description  = "Play NYT Connections in a terminal over SSH"
      homepage_url = "ssh://connections.lahman.dev"
    }
    github-token-diff-extension = {
      description = "Firefox extension to estimate the token delta of a given pull request"
    }
    git-orchard = {
      description = "A command-line utility for managing git worktrees and subtrees."
    }
    go-bin = {
      description  = "Prebuilt golang binary packaged as a Python wheel"
      homepage_url = "https://pypi.org/project/go-bin/"
      topics       = ["build", "building", "go", "golang", "packaging", "pip", "pypi", "python", "toolchain", "tools"]
    }
    local-preview = {
      homepage_url = "https://jmelahman.github.io/local-preview"
    }
    manygo = {
      description  = "python library with helpers for packaging golang source code"
      homepage_url = "https://pypi.org/project/manygo/"
      topics       = ["go", "golang", "packaging", "platform", "python"]
    }
    nature-sounds = {
      description = "A lightweight, nature sounds player for the command-line."
      topics      = ["cli", "go", "golang", "lightweight", "nature-sounds"]
    }
    notebook = {
      description = "A local NotebookLM: sources to podcast episodes with Claude Code"
    }
    pkglint = {
      description  = "Linter for Arch Linux PKGBUILDs"
      homepage_url = "http://jamison.lahman.dev/pkglint/"
      topics       = ["archlinux", "archlinux-package", "developer-tools", "devtools", "linter"]
    }
    go-pre-commit-hooks = {
      description = "Hooks for running standard golang tools without system dependencies"
      topics      = ["ci", "golang", "pre-commit", "pre-commit-hooks", "tooling"]
    }
    skills = {
      description = "Personal Skills for AI Agents"
    }
    smart-lights = {
      description = "A simple terminal interface for controlling the DayBetter LED strip lights."
    }
    tag = {
      description = "Create semantic version git tags"
      topics      = ["cli", "go", "golang", "packaging", "semver", "tools"]
    }
    PKGBUILDs-template = {
      description = "Template repository for hosting Arch Linux PKGBUILDs"
      is_template = true
    }
    fullstack-template = {
      description  = "A template repository for fullstack applications: a Go backend and CLI with an embedded SQLite database serving an embedded React frontend"
      homepage_url = "https://jamison.lahman.dev/fullstack-template/"
      is_template  = true
    }
    godot-template = {
      is_template = true
    }
    golang-template = {
      is_template = true
    }
    typesafe-sdk-go = {
      description = "Unofficial Golang library for the TypeSafe API"
      topics      = ["jev", "typesafe-ai"]
    }
    undot = {
      description  = "Declutter a project's root directory by taming configuration file sprawl"
      homepage_url = "http://jamison.lahman.dev/undot/"
      topics       = ["clutter", "config", "configuration-files", "hygiene", "pre-commit-hooks", "repository-management"]
    }
    work = {
      description = "A stupid simple time tracker"
    }
    dotfiles = {
      has_issues = false
    }
    "jmelahman.github.io" = {
      homepage_url = "https://jamison.lahman.dev/"
      has_issues   = false
    }
    draw = {
      homepage_url = "http://jamison.lahman.dev/draw/"
    }
    write = {
      homepage_url = "http://jamison.lahman.dev/write/"
    }
    vinder = {
      description = "Package manager for vendored dependencies"
    }
    monorepo = {
      description  = "If I'm doing anything interesting, it's here."
      homepage_url = "https://jamison.lahman.dev/projects"
      topics       = ["automation", "golang", "monorepo", "monorepo-tooling", "python", "tooling"]
      security     = true
    }
  }

  repositories = {
    for name in local.repository_names :
    name => merge(local.defaults, lookup(local.overrides, name, {}))
  }
}

# The repositories predate this configuration; adopt rather than create them.
import {
  for_each = local.repositories
  to       = github_repository.repo[each.key]
  id       = each.key
}

resource "github_repository" "repo" {
  for_each = local.repositories

  name                        = each.key
  visibility                  = "public"
  description                 = each.value.description
  homepage_url                = each.value.homepage_url
  topics                      = each.value.topics
  is_template                 = each.value.is_template
  has_issues                  = each.value.has_issues
  has_projects                = each.value.has_projects
  has_wiki                    = each.value.has_wiki
  has_discussions             = each.value.has_discussions
  allow_merge_commit          = each.value.allow_merge_commit
  allow_squash_merge          = each.value.allow_squash_merge
  allow_rebase_merge          = each.value.allow_rebase_merge
  allow_auto_merge            = each.value.allow_auto_merge
  allow_update_branch         = each.value.allow_update_branch
  delete_branch_on_merge      = each.value.delete_branch_on_merge
  squash_merge_commit_title   = each.value.squash_merge_commit_title
  squash_merge_commit_message = each.value.squash_merge_commit_message
  merge_commit_title          = each.value.merge_commit_title
  merge_commit_message        = each.value.merge_commit_message

  security_and_analysis {
    secret_scanning {
      status = each.value.security ? "enabled" : "disabled"
    }
    secret_scanning_push_protection {
      status = each.value.security ? "enabled" : "disabled"
    }
  }

  # Removing a subtree should never delete its repository.
  archive_on_destroy = true

  lifecycle {
    prevent_destroy = true
    # Only meaningful when a repository is created from a template.
    ignore_changes = [template]
  }
}

import {
  for_each = local.repositories
  to       = github_repository_vulnerability_alerts.repo[each.key]
  id       = each.key
}

resource "github_repository_vulnerability_alerts" "repo" {
  for_each = local.repositories

  repository = github_repository.repo[each.key].name
  enabled    = each.value.security
}

import {
  for_each = local.repositories
  to       = github_repository_dependabot_security_updates.repo[each.key]
  id       = each.key
}

resource "github_repository_dependabot_security_updates" "repo" {
  for_each = local.repositories

  # Security updates need the alerts they act on.
  repository = github_repository_vulnerability_alerts.repo[each.key].repository
  enabled    = each.value.security
}
