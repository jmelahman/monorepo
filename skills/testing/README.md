# testing

Software-testing rules distilled from 75 [Google Testing Blog](https://testing.googleblog.com/)
posts (2007–2026), mostly Testing on the Toilet (TotT) episodes.

## Layout

- [`SKILL.md`](SKILL.md): the skill: a write/review loop, rules grouped by
  topic, and a pre-ship checklist. This is all an agent normally needs.
- [`references/patterns.md`](references/patterns.md): original Bad/Good
  worked examples for the rules most often misapplied.
- [`references/INDEX.md`](references/INDEX.md): every source post,
  chronological. Doubles as the manifest for the fetch script.
- `references/episodes/`: cleaned local copies of the posts. **Not
  committed**: the posts are Google's copyright (the blog carries no license
  grant), so the corpus is kept local and reproducible instead.
- [`scripts/fetch_episodes.py`](scripts/fetch_episodes.py): rebuilds the
  corpus from `INDEX.md`.

## Rebuilding the reference corpus

```sh
scripts/fetch_episodes.py         # fetch and convert all 75 posts
scripts/fetch_episodes.py damp    # fetch one post by title words, print it
```

Requires python3 with beautifulsoup4; uses pandoc when available, falling
back to a plain-text conversion otherwise. Raw HTML is cached under
`references/episodes/.raw/`, and existing converted files are never
overwritten — delete a file (or the directory) to force reconversion.

