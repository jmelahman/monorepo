#!/usr/bin/env python3
"""Fetch the configured RSS/Atom feeds and emit the rendered feed body on stdout.

The output is piped into pandoc by ./build. Pandoc only needs the body; the
chrome (head, nav, footer) comes from template.html.
"""

from __future__ import annotations

import concurrent.futures
import html
import sys
import urllib.request
import xml.etree.ElementTree as ET
from dataclasses import dataclass
from datetime import datetime, timezone
from email.utils import parsedate_to_datetime
from pathlib import Path

FEEDS = [
    ("Thorsten Ball", "https://registerspill.thorstenball.com/feed"),
    ("Uros Popovic", "https://popovicu.com/rss.xml"),
    ("Kyla Scanlon", "https://kyla.substack.com/feed"),
    ("Sam Harris", "https://wakingup.libsyn.com/rss"),
    ("Pragmatic Engineer", "https://newsletter.pragmaticengineer.com/feed"),
    ("Evil Martians", "https://evilmartians.com/chronicles.atom"),
    ("Matt Bruenig", "https://mattbruenig.com/feed"),
]
MAX_ITEMS = 50
TIMEOUT = 20
USER_AGENT = "jmelahman-feed-builder/1.0 (+https://jamison.lahman.dev)"
ATOM = "{http://www.w3.org/2005/Atom}"
DC = "{http://purl.org/dc/elements/1.1/}"
PLACEHOLDER = "<!-- FEED -->"


@dataclass
class Item:
    title: str
    feed: str
    link: str
    date: datetime


def parse_date(raw: str | None) -> datetime | None:
    if not raw:
        return None
    raw = raw.strip()
    try:
        return parsedate_to_datetime(raw)
    except (TypeError, ValueError):
        pass
    try:
        return datetime.fromisoformat(raw.replace("Z", "+00:00"))
    except ValueError:
        return None


def aware(d: datetime) -> datetime:
    return d if d.tzinfo else d.replace(tzinfo=timezone.utc)


def extract_link(entry: ET.Element) -> str:
    # Atom: prefer rel="alternate" or unspecified rel.
    alts = entry.findall(f"{ATOM}link")
    chosen = next((l for l in alts if (l.get("rel") or "alternate") == "alternate" and l.get("href")), None)
    if chosen is None and alts:
        chosen = alts[0]
    if chosen is not None:
        return (chosen.get("href") or (chosen.text or "")).strip()
    # RSS: <link>url</link>
    link = entry.find("link")
    if link is not None:
        return (link.get("href") or (link.text or "")).strip()
    return "#"


def parse_entries(name: str, root: ET.Element) -> list[Item]:
    items: list[Item] = []
    for it in list(root.iter("item")) + list(root.iter(f"{ATOM}entry")):
        title = (it.findtext("title") or it.findtext(f"{ATOM}title") or "(untitled)").strip()
        raw_date = (
            it.findtext("pubDate")
            or it.findtext(f"{DC}date")
            or it.findtext(f"{ATOM}published")
            or it.findtext(f"{ATOM}updated")
        )
        date = parse_date(raw_date)
        if date is None:
            continue
        items.append(Item(title=title, feed=name, link=extract_link(it), date=aware(date)))
    return items


def fetch(name: str, url: str) -> list[Item]:
    req = urllib.request.Request(url, headers={
        "User-Agent": USER_AGENT,
        "Accept": "application/rss+xml, application/atom+xml, application/xml;q=0.9, */*;q=0.5",
    })
    try:
        with urllib.request.urlopen(req, timeout=TIMEOUT) as resp:
            data = resp.read()
        return parse_entries(name, ET.fromstring(data))
    except Exception as err:
        print(f"warn: {name} <{url}>: {err}", file=sys.stderr)
        return []


def render_table(items: list[Item]) -> str:
    rows = []
    for item in items:
        date = item.date.astimezone().strftime("%m/%d/%Y")
        rows.append(
            f'    <tr><td class="date-col">{date}</td>'
            f'<td><a href="{html.escape(item.link, quote=True)}" target="_blank" rel="noopener noreferrer">{html.escape(item.title)}</a></td>'
            f'<td style="white-space: nowrap">{html.escape(item.feed)}</td></tr>'
        )
    return (
        '<table>\n'
        '    <colgroup><col><col><col></colgroup>\n'
        '    <tr><th class="date-col">Date</th><th>Title</th><th>Feed</th></tr>\n'
        + "\n".join(rows)
        + '\n</table>'
    )


def main() -> int:
    with concurrent.futures.ThreadPoolExecutor(max_workers=len(FEEDS)) as ex:
        results = list(ex.map(lambda f: fetch(*f), FEEDS))
    if not any(results):
        print("error: every feed failed; aborting", file=sys.stderr)
        return 1
    items = sorted((it for sub in results for it in sub), key=lambda it: it.date, reverse=True)[:MAX_ITEMS]
    body = Path(__file__).resolve().parent.parent.joinpath("feed/index.html.body").read_text()
    sys.stdout.write(body.replace(PLACEHOLDER, render_table(items)))
    return 0


if __name__ == "__main__":
    sys.exit(main())
