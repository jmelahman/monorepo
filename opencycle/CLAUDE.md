# OpenCycle — Claude Notes

A free, open cycling sim in Unreal Engine 5.8. Everything is in `unreal/`, and
`unreal/README.md` is the reference: where things live, how the coast ride is built, the
settings, and the pitfalls. The Godot version was removed; it is in git history.

## Commands

Run inside the `unreal` devcontainer, from `/workspace/ue` (the working project, outside git,
which reaches the source through symlinks):

- Build the module: see `unreal/README.md`; always `-NoUBA`.
- Test without the engine: `unreal/tests/run.sh`.
- Ride offscreen and take stills: `bash scripts/ride_test.sh WATTS SHOT_SECONDS START_METRES [args]`.
- Package: `bash scripts/package.sh`. Play the package: `unreal/scripts/game.sh`.

## Keeping the expensive work cheap

Use a Sonnet subagent (`Agent` with `model: sonnet`) for work that eats tokens but needs little
context: QA of stills and clips, comparing batches of renders, and long log or transcript searches.
Tell it the shots, what to look for and what to report back, and keep design decisions here.

## Gotchas

- Run one Unreal at a time: imports take about 17 GB of the machine's 30.
- Never run a generator script as a "check"; it rewrites what it generates.
- The frame-rate cap in `/workspace/ue/local.env` makes local fps readings meaningless.
- Slate takes a box's colour from the draw's tint, not the brush's, and drops a text run's
  leading space.
