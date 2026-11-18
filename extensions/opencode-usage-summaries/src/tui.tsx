import { Plugin } from "@opencode/plugin/tui"
import { useTerminalDimensions } from "@opentui/solid"
import type { RGBA } from "@opentui/core"
import { For, Show } from "solid-js"
import { formatLocationLabel, summarySegments, METER_KEYS, rangeFor, readOptions, type ColorBand, type SummaryValue } from "./summary.js"

/** How often the rolling windows are refetched, so they roll without usage. */
const REFRESH_INTERVAL_MS = 60_000
/** Coalesces bursts of usage events into one refetch. */
const EVENT_DEBOUNCE_MS = 1_000
/**
 * The prompt footer also carries the right-side key hints, so the location
 * label is dropped below this terminal width instead of crowding them out.
 */
const LABEL_MIN_WIDTH = 90

interface State {
  /** The configured windows' usage, from the last successful refresh. */
  values?: readonly SummaryValue[]
}

/** Theme tokens for each urgency band. */
function bandColor(band: ColorBand, theme: Plugin.Context["theme"]): RGBA {
  if (band === "error") return theme.text.feedback.error.base
  if (band === "warning") return theme.text.feedback.warning.base
  return theme.text.muted
}

export default Plugin.define({
  id: "usage-summaries.tui",
  async setup(context) {
    const options = readOptions(context.options)
    const [state, mutate] = context.storage.memory<State>("summaries", { initial: {} })

    const costValues = async (): Promise<readonly SummaryValue[]> => {
      const now = Date.now()
      // One stats call per window; the ranges are independent, so parallel.
      return Promise.all(
        options.windows.map(async (id) => {
          const range = rangeFor(id, now)
          const stats = await context.client.experimental.session.stats({
            from: range.from,
            to: range.to,
            tools: "none",
          })
          return { id, cost: stats.cost }
        }),
      )
    }

    const subscriptionValues = async (): Promise<readonly SummaryValue[] | undefined> => {
      const limits = await context.client.experimental.usage.limits()
      const go = limits.data?.go
      if (!go) return undefined
      return options.windows.map((id) => {
        const meter = go[METER_KEYS[id]]
        return { id, percent: meter.percent, limited: meter.limited }
      })
    }

    let refreshing = false
    let queued = false
    const refresh = async (): Promise<void> => {
      if (refreshing) {
        queued = true
        return
      }
      refreshing = true
      try {
        // Subscription meters first; without a subscription the server
        // reports no meters and the footer falls back to computed cost.
        const values =
          options.source === "subscription" ? ((await subscriptionValues()) ?? (await costValues())) : await costValues()
        mutate((draft) => {
          draft.values = values
        })
      } catch {
        // A failed fetch keeps the last-known values; the next event or
        // interval tick retries.
      } finally {
        refreshing = false
        if (queued) {
          queued = false
          void refresh()
        }
      }
    }

    let debounce: ReturnType<typeof setTimeout> | undefined
    const schedule = () => {
      clearTimeout(debounce)
      debounce = setTimeout(() => void refresh(), EVENT_DEBOUNCE_MS)
    }

    const render = () => {
      const segments = summarySegments(state.values ?? [], options)
      return (
        <Show when={segments !== undefined}>
          <text fg={context.theme.text.muted}>
            <For each={segments}>
              {(segment) => <span style={{ fg: bandColor(segment.band, context.theme) }}>{segment.text}</span>}
            </For>
          </text>
        </Show>
      )
    }

    const disposeHome = context.ui.slot({ append: "home.footer.status", render })

    // With `hideRunningStrip` the prompt footer's status area is replaced
    // rather than appended to: the host's animated running strip is
    // suppressed, and a plain location label plus the summaries render in
    // its place. The host's click-to-move menu, interrupt hint, and
    // worktree-move progress do not come along; the label is plain text,
    // and is dropped on narrow terminals so it cannot crowd out the hints.
    let disposePrompt: () => void
    if (options.hideRunningStrip) {
      disposePrompt = context.ui.slot({
        replace: "prompt.footer.status",
        render: (input) => {
          // A hook: the slot render runs as a component inside the app tree,
          // so the renderer context exists only here, not in setup.
          const dimensions = useTerminalDimensions()
          return (
            <box flexDirection="row" gap={2} flexGrow={1} flexShrink={1} minWidth={0}>
              <Show when={dimensions().width >= LABEL_MIN_WIDTH}>
                <text fg={context.theme.text.muted} wrapMode="none" truncate flexShrink={1} minWidth={0}>
                  {promptLocationLabel(input, context)}
                </text>
              </Show>
              {render()}
            </box>
          )
        },
      })
    } else {
      disposePrompt = context.ui.slot({ append: "prompt.footer.status", render })
    }
    // The VCS branch is read reactively; prime the cache so the label has a
    // branch on first render even if the host has not synced it yet.
    void context.data.location.vcs.sync(context.location).catch(() => {})
    const stopUsage = context.data.on("session.usage.updated", schedule)
    const interval = setInterval(() => void refresh(), REFRESH_INTERVAL_MS)
    void refresh()

    return () => {
      clearInterval(interval)
      clearTimeout(debounce)
      stopUsage()
      disposeHome()
      disposePrompt()
    }
  },
})

/**
 * The location the prompt footer should describe: the open session's own
 * location while one exists, otherwise where the next session would start.
 * Reads the reactive caches, so it must run inside a tracked render.
 */
function promptLocationLabel(
  input: { readonly sessionID?: string },
  context: Plugin.Context,
): string {
  const session = input.sessionID ? context.data.session.get(input.sessionID) : undefined
  const location = session?.location ?? context.location ?? context.data.location.default()
  return formatLocationLabel(
    location.directory,
    context.data.location.vcs.info(location)?.branch.current,
    (value) => context.ui.format.path(value),
  )
}
