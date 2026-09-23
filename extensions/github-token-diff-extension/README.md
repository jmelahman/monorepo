# github-token-diff-extension

Firefox extension that shows the **estimated token difference** of a GitHub pull request alongside GitHub's line-count diffstat — because line counts don't tell you how much LLM context a PR consumes.

- A badge next to the PR tab bar: `+12,345 −6,789 tokens` (added / removed, mirroring GitHub's line stats)
- Per-file token badges next to each file's diffstat in the **Files changed** view
- Works on private repos with no API token — it fetches the PR's `.diff` using your existing GitHub session
- Zero runtime dependencies, no bundler, no data collection

Token counts are a fast heuristic estimate (identifier sub-word splitting blended with a chars-per-token ratio), not a real BPE tokenization. The estimator lives in [`background/token-estimator.js`](background/token-estimator.js) behind a single `estimateTokens(text)` function, so a real tokenizer can be swapped in later.

## How it works

The background script fetches `https://github.com/{owner}/{repo}/pull/{n}.diff`, parses the unified diff (renames, binaries, mode changes handled), estimates tokens for added and removed lines per file, and caches the result (`ETag`-revalidated, `storage.session`). The content script only *places* badges in the page — numbers never come from the DOM, so GitHub redesigns can at worst move the badge, never corrupt the counts.

## Development

```bash
npm install

# Run unit tests (diff parser, token estimator, stats)
npm test

# Lint the extension
npm run lint

# Launch Firefox with the extension loaded
npm run start -- --start-url "https://github.com/facebook/react/pull/28000/files"

# Package a .zip for AMO
npm run build
```

To load it manually instead: open `about:debugging#/runtime/this-firefox` → **Load Temporary Add-on…** → select `manifest.json`.

## Layout

```
manifest.json            MV3, Firefox 115+
background/              fetch → parse → estimate → cache (ESM, Node-testable)
content/                 URL gating + badge injection (fail-soft selectors)
test/                    node:test suites + diff fixtures
```
