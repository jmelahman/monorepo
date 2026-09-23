#!/usr/bin/env python3
"""Regenerates docs/benchmark-{light,dark}.svg from the measurements below.

The numbers come from hyperfine over a ~83.5k-file git worktree with the
page cache warm. Commands that need a pipeline are measured under bash so
hyperfine can calibrate the shell out; the two that don't are measured with
--shell=none:

    hyperfine -i -N --warmup 10 --runs 50 \\
      -n fd "fd --type symlink --exec sh -c 'test -e \\"\\$0\\"'" \\
      -n check-symlinks ./check-symlinks
    hyperfine -i --shell bash --warmup 3 --runs 30 \\
      -n check_symlinks.py "git ls-files | xargs python3 check_symlinks.py" \\
      -n "shell loop" 'while read f; do test -e "$f"; done < <(git ls-files)' \\
      -n find "find . -type l ! -exec test -e {} \\; -print0 |
               xargs --no-run-if-empty -0 git ls-files"
"""

from __future__ import annotations

from pathlib import Path

CAPTION = "checking a 83,533-file worktree, warm cache — lower is better"

# (label, mean ms, stddev ms, highlight)
RESULTS = [
    ("check-symlinks", 4.0, 0.7, True),
    ("fd", 15.6, 1.8, False),
    ("check_symlinks.py", 104.3, 5.5, False),
    ("find", 150.1, 3.4, False),
    ("shell loop", 225.4, 3.6, False),
]

THEMES = {
    "light": {"fg": "#1f2328", "muted": "#59636e", "bar": "#afb8c1", "hi": "#1a7f37"},
    "dark": {"fg": "#e6edf3", "muted": "#9198a1", "bar": "#4d5560", "hi": "#3fb950"},
}

WIDTH = 760
LABEL_W = 168
VALUE_W = 100  # room for the widest "225.4 ± 3.6 ms" label past the longest bar
ROW_H = 40
BAR_H = 24
PAD_TOP = 14
FONT = "ui-sans-serif, system-ui, -apple-system, Segoe UI, Helvetica, Arial, sans-serif"


def render(theme: dict[str, str]) -> str:
    bar_x = LABEL_W + 10
    bar_max = WIDTH - VALUE_W - bar_x
    scale = bar_max / max(r[1] for r in RESULTS)
    height = PAD_TOP + ROW_H * len(RESULTS) + 26

    out = [
        f'<svg xmlns="http://www.w3.org/2000/svg" width="{WIDTH}" height="{height}" '
        f'viewBox="0 0 {WIDTH} {height}" font-family="{FONT}" '
        f'role="img" aria-label="Benchmark: {CAPTION}">'
    ]
    for i, (label, mean, sigma, highlight) in enumerate(RESULTS):
        y = PAD_TOP + i * ROW_H
        mid = y + BAR_H / 2
        w = mean * scale
        fill = theme["hi"] if highlight else theme["bar"]
        weight = "600" if highlight else "400"
        colour = theme["fg"] if highlight else theme["muted"]
        out.append(
            f'<text x="{LABEL_W}" y="{mid}" text-anchor="end" dominant-baseline="central" '
            f'font-size="14" font-weight="{weight}" fill="{colour}">{label}</text>'
        )
        out.append(
            f'<rect x="{bar_x}" y="{y}" width="{w:.1f}" height="{BAR_H}" rx="3" fill="{fill}"/>'
        )
        out.append(
            f'<text x="{bar_x + w + 9:.1f}" y="{mid}" dominant-baseline="central" '
            f'font-size="13" fill="{theme["muted"]}">'
            f"{mean:.1f} ± {sigma:.1f} ms</text>"
        )
    out.append(
        f'<text x="{bar_x}" y="{height - 8}" font-size="12" fill="{theme["muted"]}">{CAPTION}</text>'
    )
    out.append("</svg>")
    return "\n".join(out) + "\n"


def main() -> None:
    docs = Path(__file__).resolve().parent.parent / "docs"
    docs.mkdir(exist_ok=True)
    for name, theme in THEMES.items():
        (docs / f"benchmark-{name}.svg").write_text(render(theme))
        print(f"wrote docs/benchmark-{name}.svg")


if __name__ == "__main__":
    main()
