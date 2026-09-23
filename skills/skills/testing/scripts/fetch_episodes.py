#!/usr/bin/env python3
"""Fetch Google Testing Blog posts listed in references/INDEX.md.

    fetch_episodes.py                 rebuild all of references/episodes/
    fetch_episodes.py <title words>   fetch ONE episode and print it to stdout
                                      (e.g. `fetch_episodes.py damp`)

Downloads each post, extracts the article body from the Blogger page, and
converts it to clean markdown — flattening Blogger's code tables into fenced
blocks labeled **Bad:** / **Good:** (TotT's red/green convention) and
stripping boilerplate. Converted episodes are cached under
references/episodes/ and raw HTML under references/episodes/.raw/,
so repeated lookups are offline. Existing converted files are never
overwritten — delete a file (or the episodes/ tree) to force reconversion.

The episode texts are Google's copyright, so they are not committed; this
script makes the local reference corpus reproducible.

Requires: python3 with beautifulsoup4. Uses pandoc when available; falls back
to a plain-text conversion (code blocks and Bad/Good labels survive, inline
formatting and links are dropped).
"""
import re
import subprocess
import sys
import time
import urllib.request
from pathlib import Path

from bs4 import BeautifulSoup, NavigableString

BASE = Path(__file__).resolve().parent.parent
INDEX = BASE / "references" / "INDEX.md"
OUT = BASE / "references" / "episodes"
RAW = OUT / ".raw"

LINK_RE = re.compile(r"^- \[([^\]]+)\]\((https?://[^\)]+)\)")

# TotT color convention: red background = bad example, green = good example
RED_HEXES = ("f4cccc", "ea9999", "f4c7c3", "fbe5e1", "fce8e6")
GREEN_HEXES = ("d9ead3", "b6d7a8", "e2f3eb", "e6f4ea", "d9ead2")
MONO_RE = re.compile(r"courier|consolas|monospace", re.I)


def parse_index():
    episodes = []
    for line in INDEX.read_text().splitlines():
        m = LINK_RE.match(line)
        if m:
            episodes.append({"title": m.group(1), "url": m.group(2)})
    return episodes


def slugify(title):
    s = title.lower()
    s = re.sub(r"['’]", "", s)
    return re.sub(r"[^a-z0-9]+", "-", s).strip("-")


def fetch(url, dest):
    if dest.exists() and dest.stat().st_size > 0:
        return True
    req = urllib.request.Request(url, headers={"User-Agent": "Mozilla/5.0 (episode-archiver)"})
    try:
        with urllib.request.urlopen(req, timeout=30) as r:
            dest.write_bytes(r.read())
        return True
    except Exception as e:
        print(f"  FETCH FAILED {url}: {e}", file=sys.stderr)
        return False


def cell_color(cell):
    """Classify a table cell as bad/good/None from background colors."""
    styles = [cell.get("style", ""), cell.get("bgcolor", "")]
    for d in cell.find_all(["pre", "div", "span"]):
        styles.append(d.get("style", "") + " " + d.get("bgcolor", ""))
    blob = " ".join(styles).lower()
    if any(h in blob for h in RED_HEXES):
        return "bad"
    if any(h in blob for h in GREEN_HEXES):
        return "good"
    return None


def cell_is_code(cell):
    if cell.find("pre") or cell.find("code"):
        return True
    blob = " ".join(d.get("style", "") for d in cell.find_all(True)).lower()
    return "monospace" in blob or "courier" in blob or "consolas" in blob


def cell_text(cell):
    """Extract text preserving line structure."""
    for br in cell.find_all("br"):
        br.replace_with("\n")
    # block-level elements imply a line break (e.g. one <p> per code line)
    for block in cell.find_all(["p", "div", "li", "tr"]):
        block.append(NavigableString("\n"))
    text = cell.get_text().replace("\xa0", " ")
    lines = [ln.rstrip() for ln in text.split("\n")]
    while lines and not lines[0].strip():
        lines.pop(0)
    while lines and not lines[-1].strip():
        lines.pop()
    return "\n".join(lines)


def flatten_tables(soup, body):
    """Replace each table with labeled <pre> blocks / paragraphs."""
    for table in body.find_all("table"):
        replacements = []
        for row in table.find_all("tr"):
            for cell in row.find_all(["td", "th"]):
                text = cell_text(cell)
                if not text.strip():
                    continue
                if cell_is_code(cell):
                    label = cell_color(cell)
                    if label:
                        p = soup.new_tag("p")
                        strong = soup.new_tag("strong")
                        strong.string = "Bad:" if label == "bad" else "Good:"
                        p.append(strong)
                        replacements.append(p)
                    pre = soup.new_tag("pre")
                    pre.string = text
                    replacements.append(pre)
                else:
                    p = soup.new_tag("p")
                    p.string = text
                    replacements.append(p)
        for el in reversed(replacements):
            table.insert_after(el)
        table.decompose()


def _mono_chars(el):
    """Chars of text whose ancestor chain includes a monospace-styled element."""
    mono = total = 0
    for s in el.descendants:
        if not isinstance(s, NavigableString):
            continue
        n = len(s.strip())
        if not n:
            continue
        total += n
        p = s.parent
        while p is not None and p is not el.parent:
            if MONO_RE.search(p.get("style", "") or ""):
                mono += n
                break
            p = getattr(p, "parent", None)
    return mono, total


def codify_divs(soup, body):
    """Old posts (2007-09) put code in styled divs/paragraphs of monospace
    spans + <br>. Convert any div/p dominated by monospace text into a <pre>."""
    for div in body.find_all(["div", "p"]):
        if div.find("table") or div.find("pre") or div.find("div"):
            continue
        mono, total = _mono_chars(div)
        if total and mono / total >= 0.5:
            pre = soup.new_tag("pre")
            pre.string = cell_text(div)
            div.replace_with(pre)


def extract_body(html):
    soup = BeautifulSoup(html, "html.parser")
    body = soup.find("div", class_="post-body")
    if body is None:
        return None, None
    date = None
    pub = soup.find("span", class_="publishdate")
    if pub:
        date = pub.get_text(strip=True)
    if not date:
        meta = soup.find("abbr", class_="published") or soup.find("time", class_="published")
        if meta:
            date = meta.get("title") or meta.get("datetime") or meta.get_text(strip=True)
    for sel in body.find_all(["script", "style", "iframe"]):
        sel.decompose()
    flatten_tables(soup, body)
    codify_divs(soup, body)
    return str(body), date


def to_markdown(html_fragment):
    try:
        p = subprocess.run(
            ["pandoc", "-f", "html", "-t", "gfm-raw_html", "--wrap=none"],
            input=html_fragment.encode(), capture_output=True,
        )
    except FileNotFoundError:
        # No pandoc: plain-text fallback. <pre> blocks become indented code so
        # they still read as code in the output.
        soup = BeautifulSoup(html_fragment, "html.parser")
        for pre in soup.find_all("pre"):
            indented = "\n".join("    " + ln for ln in cell_text(pre).split("\n"))
            pre.replace_with(NavigableString("\n" + indented + "\n"))
        return cell_text(soup)
    if p.returncode != 0:
        raise RuntimeError(p.stderr.decode()[:500])
    return p.stdout.decode()


BOILERPLATE = [
    # "This article was adapted from a [Google Testing on the Toilet] (TotT)
    # episode. You can download a [printer-friendly version] ... post it in your
    # office." — phrasing/link placement varies by year; match the whole span.
    r"\*?This article was adapted from a[\s\S]{0,400}?(?:post it )?in your office\.\*?\s*",
    r"\*?You can download a[\s\S]{0,300}?printer-friendly[\s\S]{0,300}?office\.\*?\s*",
    r"This is another installment[^.]*\.\s*",
]


def strip_hard_breaks(md):
    """Remove pandoc's trailing-backslash hard breaks outside code fences."""
    out = []
    in_fence = False
    for line in md.split("\n"):
        if re.match(r"^\s*(```|~~~)", line):
            in_fence = not in_fence
            out.append(line)
            continue
        if not in_fence:
            if line.strip() == "\\":
                line = ""
            elif line.endswith("\\") and not line.endswith("\\\\"):
                line = line[:-1].rstrip()
        out.append(line)
    return "\n".join(out)


def clean_markdown(md):
    for b in BOILERPLATE:
        md = re.sub(b, "", md)
    md = strip_hard_breaks(md)
    # image-only lines (blogger assets not downloaded)
    md = re.sub(r"^\s*!\[[^\]]*\]\([^)]*\)\s*$", "", md, flags=re.M)
    # empty list items from hollow <li> tags
    md = re.sub(r"^\s*(?:[-*]|\d+\.)\s*$\n?", "", md, flags=re.M)
    md = re.sub(r"\[\]\([^)]*\)", "", md)
    return re.sub(r"\n{3,}", "\n\n", md).strip()


def convert_episode(ep):
    """Fetch and convert one episode; returns its output path or an error str."""
    slug = slugify(ep["title"])
    out_path = OUT / f"{slug}.md"
    if out_path.exists():
        return out_path
    RAW.mkdir(parents=True, exist_ok=True)
    raw_path = RAW / f"{slug}.html"
    if not fetch(ep["url"], raw_path):
        return "fetch failed"
    body, date = extract_body(raw_path.read_text(errors="replace"))
    if body is None:
        return "no post body found"
    try:
        md = clean_markdown(to_markdown(body))
    except RuntimeError as e:
        return f"pandoc failed: {e}"
    out_path.parent.mkdir(parents=True, exist_ok=True)
    meta = f"> Source: {ep['url']}" + (f" ({date})" if date else "")
    out_path.write_text(f"# {ep['title']}\n\n{meta}\n\n{md}\n")
    return out_path


def fetch_all(episodes):
    print(f"{len(episodes)} episodes listed in {INDEX.name}")
    failures = []
    for ep in episodes:
        result = convert_episode(ep)
        if isinstance(result, str):
            failures.append((ep, result))
        time.sleep(0.1)
    print(f"done: {len(episodes) - len(failures)}/{len(episodes)} ok")
    for ep, why in failures:
        print(f"  {why}: {ep['title']} {ep['url']}", file=sys.stderr)
    return 1 if failures else 0


def fetch_one(episodes, query):
    q = slugify(query)
    matches = [ep for ep in episodes if q in slugify(ep["title"])]
    if not matches:
        print(f"no episode title matches {query!r}; see references/INDEX.md", file=sys.stderr)
        return 1
    if len(matches) > 1:
        print(f"{query!r} is ambiguous:", file=sys.stderr)
        for ep in matches:
            print(f"  {slugify(ep['title'])}", file=sys.stderr)
        return 2
    result = convert_episode(matches[0])
    if isinstance(result, str):
        print(f"{result}: {matches[0]['title']} {matches[0]['url']}", file=sys.stderr)
        return 1
    print(result.read_text())
    return 0


def main(argv):
    episodes = parse_index()
    if argv:
        return fetch_one(episodes, " ".join(argv))
    return fetch_all(episodes)


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
