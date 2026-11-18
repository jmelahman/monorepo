# opencode-usage-summaries

An OpenCode plugin that shows rolling hourly, weekly, and monthly usage
summaries in the bottom footer status bar:

```
~/code/monorepo:master  1h 5% · 7d 41% · 30d 91%
```

The plugin is a **TUI part**. It claims the built-in footer status slots
(`home.footer.status` and `prompt.footer.status`), so its text appears after
OpenCode's health indicators — on the home screen and inside sessions —
without replacing the rest of the UI.

By default it also **removes the animated running strip** from the prompt
footer: the row of purple blocks that sweeps left and right while a session
runs. The prompt footer's status area is replaced with a plain location label
plus the summaries, so while a session runs the footer stays quiet:

- Default (strip removed): `~/code/monorepo:master  1h 5% · 7d 41% · 30d 91%`
- With `"hideRunningStrip": false`: OpenCode's animated strip renders while a
  session runs, and the summaries append to the idle footer instead.

Subscription meters are **colored by how close they are to their limit**:
muted normally, the theme's warning color from 70%, and the error color from
90% (or whenever the plan is limiting requests). In the example above the
`30d 91%` renders red. Computed costs stay muted.

The location label is **dropped on terminals narrower than 90 columns** so it
cannot crowd out the usage summaries and the footer's key hints; it also
truncates when a long path just barely fits.

Replacing the status area takes over everything OpenCode put in it, so with
the strip removed you also lose, while a session runs:

- the `esc interrupt` hint (pressing `esc` still interrupts; the hint is gone)
- the worktree-switch progress indicators (rare)
- the click-to-move menu on the location label (the label is now plain text)

Set `"hideRunningStrip": false` to restore all of it.

## Sources

- **`subscription` (default)** shows your actual plan usage, as percentages of
  the subscription meters the server reports (`experimental.usage.limits`):
  the rolling, weekly, and monthly meter for each window. This is the usage
  OpenCode counts against your plan, across every project and device — not
  just the open session. When the server reports no subscription (anonymous or
  BYO-key use), the footer falls back to `cost` values.
- **`cost`** shows computed USD cost, summed over all projects on this
  machine, from the server's `experimental.session.stats` endpoint
  (`tools: "none"`). The windows are rolling: the last 1 hour, last 7 days,
  and last 30 days from right now.

The summaries refresh when a session reports new usage (`session.usage.updated`,
debounced to one second) and once a minute so the numbers stay current when you
are not prompting. A failed fetch keeps the last-known values instead of
blanking the footer.

Cost amounts stay compact: three decimals below ten cents, two decimals below a
thousand dollars, and grouped thousands above (`$1,235`). Subscription meters
are shown as whole percentages (`5%`, `90%`).

## Enable

This repository loads the plugin through `.opencode/opencode.json`:

```jsonc
{
  "$schema": "https://opencode.ai/config.json",
  "plugins": [
    "../extensions/opencode-usage-summaries"
  ]
}
```

Configuration changes reload without restarting OpenCode. To use it from any
project, add the entry (or an absolute-path form) to the global
`~/.config/opencode/opencode.jsonc` instead:

```jsonc
{
  "plugins": ["/home/jamison/code/monorepo/extensions/opencode-usage-summaries"]
}
```

## Options

Pass options through the object form of the plugin entry:

```jsonc
{
  "plugins": [
    {
      "package": "../extensions/opencode-usage-summaries",
      "options": {
        "windows": ["hour", "week", "month"],
        "separator": " · ",
        "labels": { "hour": "1h", "week": "7d", "month": "30d" }
      }
    }
  ]
}
```

| Option       | Default                    | Description                                  |
| ------------ | -------------------------- | -------------------------------------------- |
| `windows`    | `["hour","week","month"]`  | Which summaries to show, in this order.     |
| `separator`  | `" · "`                    | Text between the summaries.                  |
| `labels`     | `1h` / `7d` / `30d`         | Label overrides per window.                  |

Invalid window names and non-string labels are dropped; a `windows` list that
ends up empty falls back to the default order.

## Notes

- The server part (`src/index.ts`) is a no-op; it exists so the package follows
  the usual `.` + `./tui` layout and loads automatically from `plugins`. All
  the work happens in the TUI part against the client connection.
- Costs are USD. Models without price data (free or local ones) contribute
  `$0`, so a session on the free tier shows `1h $0` even while tokens are
  burning.
- The endpoint is marked `experimental` upstream. If OpenCode renames it, the
  plugin keeps showing the last-known values and logs nothing in the footer.

## Development

```sh
cd extensions/opencode-usage-summaries
bun install
bun test
bun run typecheck
```

The TUI plugin source is `src/tui.tsx`; the window definitions, option
parsing, and formatting live in `src/summary.ts` with tests in
`test/summary.test.ts`.
