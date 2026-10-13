# The benchmark

The game doubles as a benchmark for language models: a model plays a full run
blind to the answer, over the same seeds as everyone else, and a browser tab can
watch it happen in the real game. The repo contains no model client. The
harness brings the model (Claude Code, any MCP-speaking agent) and the host
keeps everything else fixed: the seeds, the rules text, the caps and the
scoring. A score therefore measures model and harness together, and the label
a run is filed under should name both.

[LLMs play 5 Wild](https://jamison.lahman.dev/blog/llms-play-5-wild.html) is the
write-up: what the models scored, what a run cost, and films of them playing.
Its charts and tables are drawn by `bench:charts`, below.

This directory is the host and everything that happens to a run after it is
played: the MCP server a model plays through, the reference bot, the report, and
the tools that turn finished episodes into bundles, films and charts. Nothing in
it is scored logic. The observation, the command grammar, the caps, the scoring
and the replay check are `src/bench/`, which the browser's spectator also loads;
this is the part that touches a disk, a port or a network. The reasoning behind
each tool is in the comment at the head of its file, and that comment, not this
page, is where a decision is recorded.

```
play                      check                 publish
────                      ─────                 ───────
mcp.ts ──────┐                                  export.ts ── bench/exports/<label>.json
baseline.ts ─┼─▶ bench/results/<label>/ ──┬──▶ report.ts ── the table, --json, --csv
opencode.ts ─┘     seed-N-aA.json         ├──▶ record.ts ── bench/videos/*.mp4 ──▶ upload.ts ── YouTube
                                          └──▶ charts.ts ── the blog post's SVG and tables
```

Everything to the right of `bench/results/` reads episodes and replays them
against this checkout's engine, so run it on the commit the episodes were played
on. Everything under `bench/` except the suite files is gitignored.

## Playing a run

The host is `mcp.ts`, an MCP server over stdio, registered in `.mcp.json` as
`5wild`, so Claude Code opened at the root offers it (after the one-time prompt
to approve project servers). Any other harness launches the same command,
`bun tools/bench/mcp.ts`, from the root. To watch:

```sh
bun run dev    # then open http://localhost:5173/?watch=http://localhost:7777
```

Then ask the agent to play: `new_run` with a label, then `act` until the run
ends. The tools are `rules` (the system prompt: how the game works and how to
act), `new_run {label, seed?, ascension?, format?}`, `observe {format?}` and
`act {command}`, which returns the next observation, so a turn is one call. The
observation is text by default and the player's JSON view with `format: "json"`.
Either way the answer is absent from the structure it is drawn from, not merely
unprinted, and `test/bench/observe.test.ts` walks every observation of three
runs to hold that. Commands are the small grammar in `src/bench/commands.ts`
(`guess crane`, `buy 2`, `place e`, `next`...), and every observation lists the
ones legal right now. The engine's refusals still decide what happens: the
list is a hint, not a second rulebook. A refused command changes nothing, and
25 in a row, or 3000 accepted steps, end the run as `stalled`.

| Flag | Default | |
| --- | --- | --- |
| `--suite <path>` | `bench/suite.json` | Which seeds, at which ascension. |
| `--port <n>` | 7777 | The spectator feed. |
| `--no-feed` | | Serve no feed. |
| `--format text\|json` | the suite's | The observation a `new_run` gets when it names none. |
| `--label <name>` | `unlabelled` | The label a `new_run` gets when it names none. |

stdout is the protocol and nothing else may write to it. Everything a person
should see goes to stderr.

### The suite

`bench/suite.json` is twenty seeds at ascension 0, and `suite-a1.json`,
`suite-a9.json` and `suite-a10.json` beside it are the same seeds higher up the
ladder; every tool takes one with `--suite`. A `new_run` with no seed deals the
label's next unplayed suite seed, and each finished run is written to
`bench/results/<label>/seed-N-aA.json`. That layout is the cursor, so a label
cannot play a seed twice and keep the better one. A label's episodes for every
ascension share one directory and are told apart by the `aA`.

A run is saved after every command in `in-progress.json` beside it, and a
harness that drops mid-run gets the same run back, replayed from its seed, on
its next `new_run` under the same label. Hanging up is not a way out of a bad
deal. A `new_run` naming a seed is ad hoc: it goes under `adhoc/` and never
counts toward the suite.

### An episode

An episode is `{content, commit, promptHash, label, suite, seed, ascension,
steps, result, ...}`, where `steps` are telemetry `Step`s, so the replay code
that validates player runs validates these unchanged. `commit` carries
`-dirty` when the tree was, and `promptHash` covers the rules text and
`CONTENT_VERSION`, so a report never averages across a rules change. Play on a
clean tree: `-dirty` is not a commit anyone can check out.

### The baseline

```sh
bun run bench:baseline                    # the blind bot through the same host, filed as a suite
bun run bench:baseline --policy farmer --seeds 1..5 --watch
```

The baseline is `test/helpers/blind.ts` driven through the benchmark's
`Session` rather than straight into `reduce`, so it is also the host's smoke
test, and it needs no model. On the standard suite at `CONTENT_VERSION` 27 the
solver clears 10.85 rounds a run (bootstrap 95% interval 8.65 to 13.25) and wins
1 of 20, and the farmer clears 9.75 and wins none. A model below the solver is
losing to a pool filter with a shop script.

| Flag | Default | |
| --- | --- | --- |
| `--policy solver\|farmer` | `solver` | Filed as `baseline-<policy>`. |
| `--seeds 1..5` or `1,4,9` | the suite's | Named seeds go under `adhoc/`, so a smoke test does not leave the suite looking part done. |
| `--watch`, `--pause <ms>` | off, 1500 | Serve the feed and wait between moves. Without it the suite runs in about a second. |
| `--suite <path>`, `--port <n>` | as the host's | |

### OpenCode

OpenCode is the one harness the repo drives itself, because a suite across a
subscription's worth of models is forty sessions nobody wants to start by hand:

```sh
bun run bench:opencode --models opencode-go/kimi-k3,opencode-go/glm-5.3   # as `opencode models` spells them
bun run bench:opencode --models opencode-go/kimi-k3 --runs 3 --parallel 1
```

Each model is filed as `<model> / <provider>` and plays from an empty directory
under the system's temp folder, with a config that allows the `5wild` server
and denies every other tool, so the word lists are out of reach rather than
merely out of bounds. The briefing it is sent is
`.agents/skills/benchmark/SKILL.md`, the same text a model driven by hand
reads. A session that stops before `RUN ENDED` is started again and handed the
same run, up to `--attempts` times; a seed still unfinished after that stops
the label there. What OpenCode printed, tokens and cost included, is kept in
`bench/logs/<label>/`. See the head of `opencode.ts` for why each of those is
so.

| Flag | Default | |
| --- | --- | --- |
| `--models <provider/model,...>` | required | |
| `--runs <n>` | the suite's length | How many seeds each model plays. |
| `--parallel <n>` | 2 | Models playing at once. |
| `--attempts <n>` | 6 | Sessions a single run may take. |
| `--timeout <minutes>` | 90 | |
| `--tag <text>` | | Lets one model play several seeds at once; see the comment on `tag`. |
| `--suite <path>` | `bench/suite.json` | |

Any other harness needs nothing here: it launches the host and plays.

## Reading the results

```sh
bun run bench:report                      # every label, this suite, this prompt hash
bun run bench:report --suite bench/suite-a10.json
bun run bench:export                      # one portable, replayable file per label
bun run bench:replay bench/results/<label>/seed-3-a0.json
```

The report ranks by mean rounds cleared, the one measure that separates runs
which all die early, with a bootstrap interval over the seeds. Twenty seeds give
a wide interval, and two labels whose intervals overlap have not been separated.
It keeps only the suite's own runs, at the suite's ascension, on the current
prompt hash, and counts what it left out underneath.

**Many models** are best run side by side, one worktree or devcontainer each,
under distinct labels, since a host plays one run at a time and two hosts under
one label would both deal the suite's first seed. Each place then exports its
labels, and the bundles are gathered anywhere and reported together:

```sh
bun run bench:export                      # bench/exports/<label>.json, one per label
bun run bench:export "kimi-k3 / opencode-go" --out ~/5wild-results
bun run bench:report ~/5wild-results/ --json results.json --csv results.csv
```

A bundle is the label's suite runs whole, steps and all, under the suite, the
content version, the prompt hash and the commits they were played on. The
report takes bundles, results directories or a folder of either, and trusts
none of it: every run is replayed from its seed and its result computed again,
so an edited file, or a worktree on a different engine behind the same prompt
hash, is named and left out rather than averaged in. The same run arriving
twice counts once. The same label and seed arriving as two different plays is a
label reused on two machines, and both are left out, since the table has no
way to know which to believe. `--csv` is one row per run, the shape a plotting
tool wants, and `--json` is the summaries with the runs beneath them. Run the
report on the commit the bundles were played on: a bundle made elsewhere
replays only against the engine that dealt it.

## The spectator

The spectator is `?watch=<feed>` on any build. The host publishes the seed and
then every accepted step over server-sent events on port 7777, and the tab deals
the same run from the same seed and plays the steps through the ordinary `App`,
at animation pace, sound and all. No state and no answer crosses the wire.
Watching touches no storage (no save, no record, no telemetry), so a player's
own run survives a spectator opened in the same browser. A step the host
accepted that the tab's engine refuses means the two are not playing the same
game, and the banner says so and stops rather than drawing a run that is not
the one being scored. `bench:replay` serves a finished episode on the same feed,
so the spectator has one input whether the run is live or recorded.

## Films

A video of a finished run is the spectator with a headless browser watching:

```sh
bunx playwright-core install chromium-headless-shell    # once
bun run bench:record bench/results/<label>/seed-3-a0.json
bun run bench:record bench/results/*/seed-3-a0.json --size 540x960 --skin tabletop
```

It writes `bench/videos/<label>-seed-N-aA.mp4` (a silent WebM without `ffmpeg`),
with the game's own sound unless `--silent`, at the game's own ×2 speed unless
`--speed` says otherwise. It films this checkout's engine playing the episode's
steps, so it refuses an episode from another `CONTENT_VERSION` and fails on one
that diverges, and a video is therefore as much a replay check as `bench:report`
is.

| Flag | Default | |
| --- | --- | --- |
| `--out <dir>` | `bench/videos` | |
| `--size <WxH>` | `1920x1080` | |
| `--speed 1\|2\|3` | 2 | The game's own speed setting, not a sped-up encode. |
| `--skin <name>` | `smoke` | `smoke`, `tabletop`, `classic-dark` or `classic-light`. |
| `--webm` | | Keep the WebM beside the MP4. |
| `--silent` | | Leave the sound out. |

**Uploading** them is `bench:upload`, because a run's film is tens of megabytes
and belongs in no repo:

```sh
bun run bench:upload --login                      # once, in a browser
bun run bench:upload bench/videos/*.mp4 --dry     # the titles it would write
bun run bench:upload bench/videos/*.mp4           # unlisted unless --privacy
bun run bench:upload --retitle bench/videos/*.mp4 # titles and descriptions only
```

It wants an OAuth client of type "Desktop app", from a Google Cloud project
with the YouTube Data API v3 enabled, saved as
`~/.config/5wild-bench/youtube-client.json`; the token `--login` earns is kept
beside it, outside the checkout. Titles and descriptions come from the episode
a video was filmed from, and `bench/videos/uploaded.json` remembers what went
up as which id, so a second run sends only what is new. `--retitle` rewrites
what is already up from that ledger and sends no bytes.

This is the one tool here that leaves the machine, so run it with `--dry`
first. Since `--retitle` its token is wide enough to delete videos, which the
tool never asks of it; see the head of `upload.ts` for why no smaller scope
would do, and for the two Google limits worth knowing before blaming the file.

## Charts

`bench:charts` redraws every chart and table in
[the benchmark blog post](https://jamison.lahman.dev/blog/llms-play-5-wild.html)
from the episodes:

```sh
bun run bench:charts --dry --preview /tmp/charts.html   # look before writing
bun run bench:charts
```

**It writes into another repository.** Its default `--post` is
`../jmelahman.github.io/blog/llms-play-5-wild.md`, the site checked out beside
this one. It replaces each chart and table where it stands and touches nothing
else, so the prose around them still has to be read after a rerun: a sentence
quoting a median does not follow the median.

Nothing in it names a model. A player is a results directory with the first
`--seeds` seeds (five) filed at one ascension; one with fewer is listed on the
way out and drawn nowhere, and the baselines are left to `bench:report`.
Everything but cost and speed is read by replaying the episode. Those two an
episode cannot give, and they come from `measured.ts`.

| Flag | Default | |
| --- | --- | --- |
| `--post <file.md>` | the blog post | |
| `--preview <file.html>` | | Every chart on one page. |
| `--seeds <n>` | 5 | |
| `--dry` | | Write nothing to the post. |

## The shared modules

Not scripts. Each is its own file because two tools need it, and several of the
tools run on import, so a tool that needs something from another gets it from a
module both can import.

| File | What it holds |
| --- | --- |
| `host.ts` | Where episodes go, which seed a label plays next, the commit an episode is stamped with, and the `in-progress.json` that lets a dropped harness resume its run. |
| `feed.ts` | The spectator feed: server-sent events on a local port. Also the `--replay` entry point. |
| `bundle.ts` | The bundle format, and `loadEpisodes`, the one reader behind the report and the export. |
| `words.ts` | The shipped word lists off disk, shared with `test/telemetry-report.test.ts`. |
| `names.ts` | A model's id as a title: `gpt-6-luna` is GPT-6 Luna. Read by the charts and the film titles. |
| `measured.ts` | Cost and seconds per round, for the labels somebody measured. |

`names.ts` and `measured.ts` are the two tables kept by hand, and a new model
wants a line in each. One absent from `names.ts` is titled by its id. One
absent from `measured.ts` is left off the cost chart and has its speed
estimated from wall time, and `bench:charts` names both kinds on its way out. The figures in
`measured.ts` were read out of `bench/logs/` by hand; nothing computes them.

## What is written where

| Path | Written by | |
| --- | --- | --- |
| `bench/suite*.json` | by hand | The only files under `bench/` that are tracked. |
| `bench/results/<label>/seed-N-aA.json` | the host, the baseline | One episode. |
| `bench/results/<label>/in-progress.json` | the host | The run being played, rewritten after every command. |
| `bench/results/<label>/adhoc/` | the host, the baseline | Runs that named their seed. Never counted. |
| `bench/exports/<label>.json` | `export.ts` | Bundles. |
| `bench/logs/<label>/` | `opencode.ts` | What OpenCode printed. |
| `bench/videos/*.mp4` | `record.ts` | Films. |
| `bench/videos/uploaded.json` | `upload.ts` | File name to video id. The charts read their film links from it. |
