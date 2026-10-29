#!/usr/bin/env python3
"""Draws the monorepo as a skyline: one building per top-level project.

A building's height is the square root of its tracked, non-blank lines of
code, and its floors are its languages, largest at the bottom. The root
README shows the light and dark drawings through a <picture>, the way
5-wild shows its wordmark: a prefers-color-scheme query inside one SVG
follows the OS rather than GitHub's theme, so it can be wrong on the page.

The square root is deliberate. Lines run from about a hundred to over forty
thousand, so a linear scale leaves most of the repo as a row of slivers,
while a log scale flattens pkglint to barely twice the height of connections.
The square root keeps every building visible and still lets size read as
size, and the caption says so.

Counts come from `git ls-files`, so untracked and ignored files never count.
Anything git attributes as linguist-generated or linguist-vendored is
skipped, as GitHub's own language bar skips it, along with vendor/ and
third_party/ trees and the SKIP files below.

Usage: tools/scripts/skyline.py   (rewrites media/skyline-{light,dark}.svg)
"""

from __future__ import annotations

from collections import defaultdict
import hashlib
from html import escape
import math
import os
import subprocess
from typing import NamedTuple

ROOT = os.path.normpath(os.path.join(os.path.dirname(__file__), "..", ".."))
OUT = os.path.join(ROOT, "media")

# Languages fold into five groups, in the categorical order the colors are
# assigned in. A sixth group would be a hue nobody validated, so a new
# language joins "Other" rather than getting one.
GROUPS = ("Go", "Web", "Rust", "Python", "Other")
EXTENSIONS = {
    ".go": "Go",
    **dict.fromkeys(
        (".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs", ".html", ".css", ".vue", ".svelte"),
        "Web",
    ),
    ".rs": "Rust",
    ".py": "Python",
    **dict.fromkeys(
        (".sh", ".bash", ".hcl", ".tf", ".gd", ".java", ".kt", ".swift"),
        "Other",
    ),
    **dict.fromkeys(
        (".c", ".h", ".lua", ".sql"),
        "Other",
    ),
}
SKIP_DIRS = frozenset({"vendor", "third_party", "node_modules", "dist"})
# Generated, but not marked linguist-generated in .gitattributes.
SKIP = frozenset({"agilecbt/docs/public/benchmarks/report.html"})
# Below this a project is a shed, not a building.
MIN_LINES = 100


# The dataviz reference palette's first five categorical slots, validated
# against GitHub's own page colors (#ffffff, #0d1117) for protan, deutan and
# tritan separation. Three light hues sit under 3:1 on white, which is why
# every building carries its name and the legend is always drawn.
class Theme(NamedTuple):
    series: tuple[str, ...]
    ink: str
    muted: str
    ground: str
    window: str  # The page behind the drawing, which unlit windows show.
    lit: str
    sky: str | None  # Star color; None draws a day with no stars.


THEMES = {
    "light": Theme(
        series=("#2a78d6", "#eb6834", "#1baf7a", "#eda100", "#e87ba4"),
        ink="#0b0b0b",
        muted="#52514e",
        ground="#d0d7de",
        window="#ffffff",
        lit="#ffffff",
        sky=None,
    ),
    "dark": Theme(
        series=("#3987e5", "#d95926", "#199e70", "#c98500", "#d55181"),
        ink="#f0efec",
        muted="#c3c2b7",
        ground="#30363d",
        window="#0d1117",
        lit="#ffe7a3",
        sky="#f0efec",
    ),
}

WIDTH = 960
TOP = 92  # Room for the title and legend above the tallest roof.
TALLEST = 230
GROUND = TOP + TALLEST
LABELS = 136  # The rotated names hang below the ground line.
HEIGHT = GROUND + LABELS
MARGIN = 24
GAP = 6
FLOOR_GAP = 2


class Project(NamedTuple):
    name: str
    lines: dict[str, int]

    @property
    def total(self) -> int:
        return sum(self.lines.values())


def git(*args: str, stdin: bytes | None = None) -> bytes:
    return subprocess.run(  # noqa: S603
        ["git", *args], cwd=ROOT, input=stdin, capture_output=True, check=True
    ).stdout


def skipped(paths: list[str]) -> set[str]:
    """Paths git attributes as generated or vendored, by any means."""
    out = git(
        "check-attr",
        "-z",
        "--stdin",
        "linguist-generated",
        "linguist-vendored",
        stdin=b"\0".join(path.encode() for path in paths),
    )
    fields = out.split(b"\0")
    # -z output is path, attribute, value triples.
    return {
        fields[i].decode()
        for i in range(0, len(fields) - 2, 3)
        if fields[i + 2] in {b"set", b"true"}
    }


def count() -> list[Project]:
    paths = [
        path
        for path in git("ls-files", "-z").decode().split("\0")
        if "/" in path
        and not path.startswith(".")
        and os.path.splitext(path)[1] in EXTENSIONS
        and path not in SKIP
        and not SKIP_DIRS.intersection(path.split("/")[:-1])
    ]
    generated = skipped(paths)
    lines: dict[str, dict[str, int]] = defaultdict(lambda: defaultdict(int))
    for path in paths:
        if path in generated:
            continue
        try:
            with open(os.path.join(ROOT, path), "rb") as f:
                n = sum(1 for line in f if line.strip())
        except FileNotFoundError:  # Deleted but not yet staged.
            continue
        lines[path.split("/", 1)[0]][EXTENSIONS[os.path.splitext(path)[1]]] += n
    projects = [Project(name, dict(by_group)) for name, by_group in lines.items()]
    return sorted((p for p in projects if p.total >= MIN_LINES), key=lambda p: p.name)


def short(n: int) -> str:
    return f"{n / 1000:.1f}k" if n >= 1000 else str(n)  # noqa: PLR2004


def noise(*parts: object) -> float:
    """A stable number in [0, 1), so the windows light the same way every run."""
    digest = hashlib.sha256("/".join(map(str, parts)).encode()).digest()
    return int.from_bytes(digest[:4]) / 2**32


def windows(name: str, box: tuple[float, float, float, float], theme: Theme) -> list[str]:
    """A grid of windows in box (x, y, w, h), a few lit. Decoration, so it stays faint."""
    x, y, w, h = box
    out = []
    cols = max(1, int((w - 4) // 7))
    rows = int((h - 6) // 9)
    pad = (w - cols * 7 + 3) / 2
    for row in range(rows):
        for col in range(cols):
            lit = noise(name, row, col) < 0.28  # noqa: PLR2004
            fill = theme.lit if lit else theme.window
            opacity = 0.85 if lit and theme.sky else 0.22
            out.append(
                f'<rect x="{x + pad + col * 7:.1f}" y="{y + 5 + row * 9:.1f}" width="4" '
                f'height="5" fill="{fill}" opacity="{opacity}"/>'
            )
    return out


def sky(theme: Theme) -> list[str]:
    """Stars and a moon at night; the day sky is left to the page."""
    if not theme.sky:
        return []
    out = []
    for i in range(60):
        x = MARGIN + noise("star", i, "x") * (WIDTH - 2 * MARGIN)
        y = TOP - 30 + noise("star", i, "y") * (TALLEST * 0.55)
        r = 0.6 + noise("star", i, "r") * 0.9
        out.append(
            f'<circle cx="{x:.1f}" cy="{y:.1f}" r="{r:.2f}" fill="{theme.sky}" opacity="0.5"/>'
        )
    # A crescent: one disc, with a page-colored disc over most of it.
    out.append(f'<circle cx="{WIDTH - 70}" cy="{TOP - 40}" r="16" fill="{theme.lit}"/>')
    out.append(f'<circle cx="{WIDTH - 62}" cy="{TOP - 46}" r="14" fill="{theme.window}"/>')
    return out


def legend(present: list[str], theme: Theme) -> list[str]:
    out = []
    x = MARGIN
    for group in present:
        color = theme.series[GROUPS.index(group)]
        out.append(f'<rect x="{x}" y="60" width="12" height="12" rx="2" fill="{color}"/>')
        out.append(
            f'<text x="{x + 18}" y="70.5" font-size="13" fill="{theme.muted}">{group}</text>'
        )
        x += 30 + 8 * len(group)
    return out


def draw(projects: list[Project], mode: str) -> str:
    theme = THEMES[mode]
    scale = TALLEST / math.sqrt(max(p.total for p in projects))
    width = (WIDTH - 2 * MARGIN - GAP * (len(projects) - 1)) / len(projects)
    # The three tallest carry their count, except beside one already labelled:
    # neighbours stand a building's width apart, narrower than "55.5k".
    labelled: list[int] = []
    for project in sorted(projects, key=lambda p: p.total, reverse=True)[:3]:
        at = projects.index(project)
        if all(abs(at - other) > 1 for other in labelled):
            labelled.append(at)
    present = [g for g in GROUPS if any(g in p.lines for p in projects)]
    total = sum(p.total for p in projects)

    out = [
        (
            f'<svg xmlns="http://www.w3.org/2000/svg" width="{WIDTH}" height="{HEIGHT}" '
            f'viewBox="0 0 {WIDTH} {HEIGHT}" role="img" aria-labelledby="title desc" '
            "font-family=\"-apple-system, BlinkMacSystemFont, 'Segoe UI', Helvetica, Arial, "
            'sans-serif">'
        ),
        '<title id="title">The monorepo skyline</title>',
        f'<desc id="desc">{len(projects)} projects, {total:,} lines of code. '
        + "; ".join(f"{p.name}: {p.total:,}" for p in projects)
        + "</desc>",
        *sky(theme),
        (
            f'<text x="{MARGIN}" y="30" font-size="20" font-weight="600" '
            f'fill="{theme.ink}">The monorepo skyline</text>'
        ),
        (
            f'<text x="{MARGIN}" y="49" font-size="13" fill="{theme.muted}">'
            f"{len(projects)} projects, {short(total)} lines. Height is √lines, so a building "
            "twice as tall holds four times the code.</text>"
        ),
        *legend(present, theme),
    ]

    for i, project in enumerate(projects):
        x = MARGIN + i * (width + GAP)
        height = max(6.0, math.sqrt(project.total) * scale)
        roof = GROUND - height
        # Floors split the building's height by each group's share of its lines,
        # largest at the bottom, with a gap of page color between them.
        floors = sorted(project.lines.items(), key=lambda kv: kv[1], reverse=True)
        y = GROUND
        for j, (group, n) in enumerate(floors):
            h = height * n / project.total
            if h < 1:
                continue
            top_floor = j == len(floors) - 1 or all(
                height * m / project.total < 1 for _, m in floors[j + 1 :]
            )
            y -= h
            color = theme.series[GROUPS.index(group)]
            # A floor a few pixels tall would be mostly gap and float free of
            # the building, so it sits directly on the one below instead.
            above = height * floors[j + 1][1] / project.total if j + 1 < len(floors) else 0
            gap = FLOOR_GAP if not top_floor and above >= 2 * FLOOR_GAP else 0
            # Round only the roof; the building stands flat on the ground.
            if top_floor:
                r = min(4.0, h / 2)
                path = (
                    f"M{x:.1f},{y + h:.1f} V{y + r:.1f} Q{x:.1f},{y:.1f} {x + r:.1f},{y:.1f} "
                    f"H{x + width - r:.1f} Q{x + width:.1f},{y:.1f} {x + width:.1f},{y + r:.1f} "
                    f"V{y + h:.1f} Z"
                )
                out.append(f'<path d="{path}" fill="{color}"/>')
            else:
                out.append(
                    f'<rect x="{x:.1f}" y="{y + gap:.1f}" width="{width:.1f}" '
                    f'height="{h - gap:.1f}" fill="{color}"/>'
                )
            out.extend(windows(f"{project.name}/{group}", (x, y, width, h), theme))
        if i in labelled:
            out.append(
                f'<text x="{x + width / 2:.1f}" y="{roof - 6:.1f}" font-size="11" '
                f'text-anchor="middle" fill="{theme.muted}">{short(project.total)}</text>'
            )
        out.append(
            f'<text transform="translate({x + width / 2 + 4:.1f},{GROUND + 10}) rotate(-90)" '
            f'font-size="12" text-anchor="end" fill="{theme.ink}">'
            f"{escape(project.name)}</text>"
        )

    out.append(
        f'<rect x="{MARGIN - 8}" y="{GROUND}" width="{WIDTH - 2 * MARGIN + 16}" height="2" '
        f'fill="{theme.ground}"/>'
    )
    out.append("</svg>")
    return "\n".join(out) + "\n"


def main() -> None:
    projects = count()
    os.makedirs(OUT, exist_ok=True)
    for mode in THEMES:
        with open(os.path.join(OUT, f"skyline-{mode}.svg"), "w") as f:
            f.write(draw(projects, mode))
    for p in sorted(projects, key=lambda p: p.total, reverse=True):
        print(f"{p.total:>7,}  {p.name}")


if __name__ == "__main__":
    main()
