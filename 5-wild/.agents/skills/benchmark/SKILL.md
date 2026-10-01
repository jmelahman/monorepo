---
name: benchmark
description: Play a full run of 5 Wild through the `5wild` MCP server as a benchmark. Use when asked to play 5 Wild, run the benchmark, or play the next suite seed.
---

# Playing the 5 Wild benchmark

You are being measured. The `5wild` MCP server (`bun tools/bench/mcp.ts`,
registered in `.mcp.json`) deals a run of the game and scores how you play it.
Your score is what you do through that server and nothing else.

## How to play

1. **Call `rules` once** and read all of it. It is the whole briefing: how the
   game works, how scoring works and the command grammar. Nothing else you need
   is anywhere else.
2. **Call `new_run` with a `label`** naming your model and your harness, e.g.
   `claude-opus-5-5 / claude-code`. Results are filed under the label, so use
   the same one every time you play. Do not pass `seed`, `ascension` or
   `format` unless the user asked for them: a run with a chosen seed is ad hoc
   and does not count toward the suite.
   - If the label already has a suite run in progress, `new_run` returns you to
     it. That is expected; carry on from where it stands.
3. **Call `act` with one command at a time** (`guess crane`, `buy 2`,
   `collect`, `next`, ...). Each call returns the next observation, so a turn is
   one call. Every observation lists the commands legal right now; read it
   before acting.
4. `observe` re-reads the current state without acting, if you lose track.

A `REFUSED` response changed nothing. Read the reason and send something
different. 25 refusals in a row end the run as `stalled`, which is the worst
way to lose, so never resend a refused command unchanged.

## Do not stop until the run ends

A run is over only when an `act` response contains `RUN ENDED`. Until then:

- Keep calling `act`. Do not pause to summarise, ask the user whether to
  continue, or hand back a half-played run. A long run is many hundreds of
  calls, and that is normal.
- Do not end early because the run looks lost. Every round cleared counts, and
  the ranking is mean rounds cleared.
- Do not call `new_run` mid-run hoping for a better deal. A suite run cannot be
  swapped, and an ad-hoc one dropped this way is filed as a stall.
- If your context is getting long, keep notes terse rather than stopping.

When you see `RUN ENDED`, report the result line to the user (end reason,
rounds cleared, stage reached, score). Play the next run only if the user asked
for more than one; `new_run` with the same label deals the suite's next seed.

## Do not cheat

The point is to measure how well you play blind. The only information you may
use is the `rules` text and the observations the server returns, plus your own
reasoning and knowledge of English words. While playing, do **not**:

- read, grep or search anything in this repository, in particular `src/`,
  `public/words/`, `test/`, `bench/` (including `bench/results/` and any
  `in-progress.json`) or `tools/`;
- run the engine, the baseline bot, a solver or any script, or use Bash at all;
- try to recover the answer from the seed, the RNG or the word lists;
- edit any file, or touch the save, results or suite;
- search the web or spawn subagents to do any of the above for you;
- replay a seed you have already played, or play under another label to
  retry a deal.

If you are unsure whether something counts as cheating, it does. Use only the
four `5wild` tools: `rules`, `new_run`, `observe` and `act`.
