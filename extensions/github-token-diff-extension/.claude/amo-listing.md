# AMO listing draft — PR Token Diff for GitHub

## Name
PR Token Diff for GitHub

## Summary (max 250 chars)
Shows the estimated token diff of a GitHub pull request — tokens added and
removed — next to GitHub's line-count diffstat, with per-file badges in the
Files changed view. Works on private repos using your existing GitHub session.

## Description
Line counts don't tell you how much LLM context a pull request consumes.
This extension adds an estimated **token diff** wherever GitHub shows its
line-count diffstat:

<b>What you get</b>
• A header badge on every PR: +12,345 −6,789 tokens, right beside GitHub's own +/− line counts
• Per-file token badges in the Files changed view
• Hover any badge for the exact added/removed estimate

<b>How it works</b>
The extension fetches the PR's .diff through your existing GitHub session —
no API token, no OAuth, works on private repos you can already see. The
unified diff is parsed locally (renames, binaries, and mode changes handled)
and token counts are estimated per file, then cached with ETag revalidation
so repeat visits are instant.

Counts are a fast heuristic estimate (identifier sub-word splitting blended
with a chars-per-token ratio), not an exact BPE tokenization — accurate
enough to judge whether a PR fits in a model's context window.

<b>Privacy</b>
• No data collection, no analytics, no external services
• Talks only to github.com / patch-diff.githubusercontent.com
• Zero runtime dependencies

Source code: https://github.com/jmelahman/github-token-diff-extension

## Categories
Web Development (primary); no secondary

## Tags
github, code review, llm, ai, tokens, developer tools

## Homepage
https://github.com/jmelahman/github-token-diff-extension

## Support email
lahmanja@gmail.com

## License
GNU General Public License v3.0 (matches repo LICENSE)

## Privacy policy
None needed — manifest declares data_collection_permissions: none
(select "This add-on doesn't collect any data" in the Developer Hub)

## Release notes — 0.1.2
• Header token badge now sits to the left of GitHub's line-count diffstat
• Token icon moved to the right of the numbers; +/− counts are now bold
• Per-file badges align flush right in the file header
