#!/usr/bin/env python3
"""Validate the skills in this repository and the manifests that distribute them.

Checks that every skill under skills/ is discoverable by Claude Code, the
skills.sh CLI, and anything else that follows the Agent Skills spec, and that
the plugin and marketplace manifests agree with each other.

Run locally with `python3 .github/validate.py`. For a second opinion from the
authoritative implementation, run `claude plugin validate .claude-plugin/plugin.json`.
"""

import json
import re
import sys
from pathlib import Path

import yaml

REPO = Path(__file__).resolve().parent.parent
SKILLS_DIR = REPO / "skills"
PLUGIN_MANIFEST = REPO / ".claude-plugin" / "plugin.json"
MARKETPLACE_MANIFEST = REPO / ".claude-plugin" / "marketplace.json"

KEBAB = re.compile(r"^[a-z0-9]+(-[a-z0-9]+)*$")
FRONTMATTER = re.compile(r"\A---\n(.*?)\n---\n?", re.DOTALL)
MARKDOWN_LINK = re.compile(r"\[[^\]]*\]\(([^)]+)\)")

# The Agent Skills spec (https://agentskills.io) defines these six fields, and
# they are the only ones every distribution path accepts. Claude Code supports
# more, but a skill that stays inside this set also loads from claude.ai, the
# Skills API, and other agents.
SPEC_FIELDS = {"name", "description", "license", "compatibility", "metadata", "allowed-tools"}

# Fields Claude Code understands beyond the spec. Using them is fine; it just
# means the skill is Claude Code-specific.
CLAUDE_CODE_FIELDS = {
    "when_to_use",
    "disable-model-invocation",
    "user-invocable",
    "allowed-tools",
    "disallowed-tools",
    "arguments",
    "argument-hint",
    "model",
    "paths",
    "context",
    "agent",
}

# The skill listing truncates description + when_to_use at this many characters.
DESCRIPTION_LIMIT = 1536

errors: list[str] = []
warnings: list[str] = []


def error(where: Path, message: str) -> None:
    errors.append(f"{where.relative_to(REPO)}: {message}")


def warn(where: Path, message: str) -> None:
    warnings.append(f"{where.relative_to(REPO)}: {message}")


def load_json(path: Path) -> dict | None:
    if not path.exists():
        error(path, "missing")
        return None
    try:
        return json.loads(path.read_text())
    except json.JSONDecodeError as exc:
        error(path, f"invalid JSON: {exc}")
        return None


def validate_skill(skill_dir: Path) -> None:
    if not KEBAB.match(skill_dir.name):
        error(skill_dir, "directory name must be kebab-case; it becomes the slash command")

    skill_md = skill_dir / "SKILL.md"
    if not skill_md.exists():
        error(skill_dir, "no SKILL.md; this directory will not be discovered as a skill")
        return

    text = skill_md.read_text()
    if not text.strip():
        error(skill_md, "file is empty")
        return

    match = FRONTMATTER.match(text)
    if not match:
        error(skill_md, "no YAML frontmatter block between --- delimiters")
        return

    try:
        front = yaml.safe_load(match.group(1)) or {}
    except yaml.YAMLError as exc:
        error(skill_md, f"invalid YAML frontmatter: {exc}")
        return
    if not isinstance(front, dict):
        error(skill_md, "frontmatter must be a YAML mapping")
        return

    description = front.get("description")
    if not description:
        error(skill_md, "frontmatter is missing `description`; agents use it to decide when to load the skill")
    elif not isinstance(description, str):
        error(skill_md, "`description` must be a string")

    name = front.get("name")
    if name is not None:
        if not isinstance(name, str) or not KEBAB.match(name):
            error(skill_md, f"`name` must be kebab-case, got {name!r}")
        elif name != skill_dir.name:
            # In a plugin skill, `name` replaces the directory name in the
            # command, so a mismatch silently renames the command.
            error(skill_md, f"`name` is {name!r} but the directory is {skill_dir.name!r}")

    listing = f"{description or ''}{front.get('when_to_use') or ''}"
    if len(listing) > DESCRIPTION_LIMIT:
        warn(skill_md, f"description + when_to_use is {len(listing)} chars, truncated at {DESCRIPTION_LIMIT}")

    unknown = set(front) - SPEC_FIELDS - CLAUDE_CODE_FIELDS
    if unknown:
        error(skill_md, f"unknown frontmatter field(s): {', '.join(sorted(unknown))}")
    claude_only = set(front) & CLAUDE_CODE_FIELDS - SPEC_FIELDS
    if claude_only:
        warn(skill_md, f"Claude Code-only frontmatter field(s): {', '.join(sorted(claude_only))}")

    body_lines = text[match.end():].splitlines()
    if len(body_lines) > 500:
        warn(skill_md, f"body is {len(body_lines)} lines; move detail into references/ and link to it")

    for link in MARKDOWN_LINK.findall(text):
        target = link.split("#", 1)[0]
        if not target or "://" in target or target.startswith("mailto:"):
            continue
        if not (skill_dir / target).exists():
            error(skill_md, f"broken relative link: {link}")


def validate_manifests() -> None:
    plugin = load_json(PLUGIN_MANIFEST)
    marketplace = load_json(MARKETPLACE_MANIFEST)
    if plugin is None or marketplace is None:
        return

    for key in ("name", "owner", "plugins"):
        if key not in marketplace:
            error(MARKETPLACE_MANIFEST, f"missing required field `{key}`")
    if not KEBAB.match(marketplace.get("name", "")):
        error(MARKETPLACE_MANIFEST, "`name` must be kebab-case")
    if not (marketplace.get("owner") or {}).get("name"):
        error(MARKETPLACE_MANIFEST, "`owner.name` is required")

    if not KEBAB.match(plugin.get("name", "")):
        error(PLUGIN_MANIFEST, "`name` must be kebab-case")

    entries = marketplace.get("plugins") or []
    for entry in entries:
        for key in ("name", "source"):
            if key not in entry:
                error(MARKETPLACE_MANIFEST, f"plugin entry {entry.get('name', '?')!r} missing `{key}`")

    # `claude plugin tag` requires plugin.json and its enclosing marketplace
    # entry to agree, so keep them in sync here too.
    root_entries = [e for e in entries if e.get("source") in ("./", ".")]
    if not root_entries:
        error(MARKETPLACE_MANIFEST, 'no plugin entry with source "./" for this repository')
        return
    for entry in root_entries:
        for key in ("name", "version"):
            if entry.get(key) != plugin.get(key):
                error(
                    MARKETPLACE_MANIFEST,
                    f"`{key}` is {entry.get(key)!r} but plugin.json says {plugin.get(key)!r}",
                )


def main() -> int:
    validate_manifests()

    # An empty repository is vacuously valid, so having no skills yet is only
    # worth a nudge. A skill that exists but is malformed is an error.
    if not SKILLS_DIR.is_dir():
        warn(SKILLS_DIR, "missing; skills belong in skills/<name>/SKILL.md")
    else:
        skill_dirs = sorted(d for d in SKILLS_DIR.iterdir() if d.is_dir())
        if not skill_dirs:
            warn(SKILLS_DIR, "contains no skills yet")
        for skill_dir in skill_dirs:
            validate_skill(skill_dir)
        print(f"Checked {len(skill_dirs)} skill(s) in {SKILLS_DIR.relative_to(REPO)}/")

    for warning in warnings:
        print(f"warning: {warning}")
    sys.stdout.flush()
    for err in errors:
        print(f"error: {err}", file=sys.stderr)

    if errors:
        print(f"\n{len(errors)} error(s), {len(warnings)} warning(s)", file=sys.stderr)
        return 1
    print(f"OK ({len(warnings)} warning(s))")
    return 0


if __name__ == "__main__":
    sys.exit(main())
