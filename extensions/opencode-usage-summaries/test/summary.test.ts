import { describe, expect, test } from "bun:test"
import {
  DEFAULT_SEPARATOR,
  DEFAULT_SOURCE,
  DEFAULT_WINDOWS,
  ERROR_AT,
  METER_KEYS,
  WARN_AT,
  WINDOWS,
  colorBand,
  formatCost,
  formatLocationLabel,
  formatPercent,
  formatSummaries,
  rangeFor,
  readOptions,
  summarySegments,
} from "../src/summary"

const now = 1_800_000_000_000

describe("readOptions", () => {
  test("defaults to every window, in order, from the subscription", () => {
    const options = readOptions(undefined)
    expect(options.windows).toEqual(DEFAULT_WINDOWS)
    expect(options.separator).toBe(DEFAULT_SEPARATOR)
    expect(options.labels).toEqual({})
    expect(options.source).toBe(DEFAULT_SOURCE)
  })

  test("keeps configured windows and their order", () => {
    expect(readOptions({ windows: ["month", "hour"] }).windows).toEqual(["month", "hour"])
  })

  test("drops unknown windows and falls back when none are left", () => {
    expect(readOptions({ windows: ["hour", "day", "year"] }).windows).toEqual(["hour"])
    expect(readOptions({ windows: ["day"] }).windows).toEqual(DEFAULT_WINDOWS)
    expect(readOptions({ windows: "hour" }).windows).toEqual(DEFAULT_WINDOWS)
  })

  test("accepts a custom separator", () => {
    expect(readOptions({ separator: "  " }).separator).toBe("  ")
  })

  test("keeps string labels and drops the rest", () => {
    const options = readOptions({ labels: { hour: "H", week: 7, month: null } })
    expect(options.labels).toEqual({ hour: "H" })
  })

  test("accepts only the known sources", () => {
    expect(readOptions({ source: "cost" }).source).toBe("cost")
    expect(readOptions({ source: "sessions" }).source).toBe(DEFAULT_SOURCE)
  })

  test("replaces the running strip unless explicitly disabled", () => {
    expect(readOptions(undefined).hideRunningStrip).toBe(true)
    expect(readOptions({ hideRunningStrip: false }).hideRunningStrip).toBe(false)
    expect(readOptions({ hideRunningStrip: "false" }).hideRunningStrip).toBe(true)
  })
})

describe("METER_KEYS", () => {
  test("maps every window onto a subscription meter", () => {
    expect(METER_KEYS).toEqual({ hour: "rolling", week: "weekly", month: "monthly" })
  })
})

describe("rangeFor", () => {
  test("reaches back exactly one span", () => {
    expect(rangeFor("hour", now)).toEqual({ from: now - WINDOWS.hour.span, to: now })
    expect(rangeFor("week", now).from).toBe(now - 7 * 86_400_000)
    expect(rangeFor("month", now).from).toBe(now - 30 * 86_400_000)
  })
})

describe("formatCost", () => {
  test("rounds sub-milli amounts to zero", () => {
    expect(formatCost(0)).toBe("$0")
    expect(formatCost(0.0004)).toBe("$0")
  })

  test("keeps three decimals for small hourly amounts", () => {
    expect(formatCost(0.0042)).toBe("$0.004")
    expect(formatCost(0.042)).toBe("$0.042")
  })

  test("uses two decimals from ten cents up", () => {
    expect(formatCost(0.1)).toBe("$0.10")
    expect(formatCost(12.3)).toBe("$12.30")
    expect(formatCost(999.999)).toBe("$1000.00")
  })

  test("groups thousands without cents", () => {
    expect(formatCost(1234.5)).toBe("$1,235")
    expect(formatCost(12_345.67)).toBe("$12,346")
  })

  test("marks amounts it cannot show", () => {
    expect(formatCost(Number.NaN)).toBe("$?")
    expect(formatCost(-1)).toBe("$?")
    expect(formatCost(Number.POSITIVE_INFINITY)).toBe("$?")
  })
})

describe("formatPercent", () => {
  test("rounds to a whole percentage", () => {
    expect(formatPercent(0)).toBe("0%")
    expect(formatPercent(4.6)).toBe("5%")
    expect(formatPercent(90.4)).toBe("90%")
    expect(formatPercent(100)).toBe("100%")
  })

  test("marks amounts it cannot show", () => {
    expect(formatPercent(Number.NaN)).toBe("?%")
    expect(formatPercent(-1)).toBe("?%")
    expect(formatPercent(Number.POSITIVE_INFINITY)).toBe("?%")
  })
})

describe("formatLocationLabel", () => {
  const formatPath = (value: string) => value.replace("/home/jamison", "~")

  test("abbreviates the directory through the host formatter", () => {
    expect(formatLocationLabel("/home/jamison/code/monorepo", undefined, formatPath)).toBe("~/code/monorepo")
  })

  test("appends the branch after a colon", () => {
    expect(formatLocationLabel("/home/jamison/code/monorepo", "main", formatPath)).toBe("~/code/monorepo:main")
  })
})

describe("colorBand", () => {
  test("stays muted until the warning threshold", () => {
    expect(colorBand(0)).toBe("muted")
    expect(colorBand(WARN_AT - 1)).toBe("muted")
  })

  test("turns warning at the threshold and error near the limit", () => {
    expect(colorBand(WARN_AT)).toBe("warning")
    expect(colorBand(ERROR_AT - 1)).toBe("warning")
    expect(colorBand(ERROR_AT)).toBe("error")
    expect(colorBand(100)).toBe("error")
  })

  test("a limited meter is error-colored regardless of percentage", () => {
    expect(colorBand(0, true)).toBe("error")
  })
})

describe("summarySegments", () => {
  test("carries each meter's urgency and muted separators", () => {
    const segments = summarySegments(
      [
        { id: "hour", percent: 5 },
        { id: "week", percent: WARN_AT },
        { id: "month", percent: ERROR_AT, limited: true },
      ],
      readOptions(undefined),
    )
    expect(segments).toEqual([
      { text: "1h 5%", band: "muted" },
      { text: " · ", band: "muted" },
      { text: "7d 70%", band: "warning" },
      { text: " · ", band: "muted" },
      { text: "30d 90%", band: "error" },
    ])
  })

  test("computed cost stays muted", () => {
    expect(summarySegments([{ id: "hour", cost: 0.42 }], readOptions(undefined))).toEqual([
      { text: "1h $0.42", band: "muted" },
    ])
  })

  test("returns nothing when there is nothing to show", () => {
    expect(summarySegments([], readOptions(undefined))).toBeUndefined()
  })

  test("joins to the same text as formatSummaries", () => {
    const values = [
      { id: "hour" as const, percent: 5 },
      { id: "week" as const, percent: 41 },
    ]
    const options = readOptions(undefined)
    expect(summarySegments(values, options)?.map((segment) => segment.text).join("")).toBe(formatSummaries(values, options))
  })
})

describe("formatSummaries", () => {
  test("labels each window and joins with the separator", () => {
    const options = readOptions(undefined)
    const text = formatSummaries(
      [
        { id: "hour", cost: 0.42 },
        { id: "week", cost: 12.3 },
        { id: "month", cost: 45.1 },
      ],
      options,
    )
    expect(text).toBe("1h $0.42 · 7d $12.30 · 30d $45.10")
  })

  test("formats subscription meters as percentages", () => {
    const options = readOptions(undefined)
    const text = formatSummaries(
      [
        { id: "hour", percent: 4.6 },
        { id: "week", percent: 90.4 },
        { id: "month", percent: 40 },
      ],
      options,
    )
    expect(text).toBe("1h 5% · 7d 90% · 30d 40%")
  })

  test("honours label overrides and a custom separator", () => {
    const options = readOptions({ separator: " | ", labels: { hour: "hour", month: "month" } })
    const text = formatSummaries(
      [
        { id: "hour", percent: 0 },
        { id: "month", percent: 1 },
      ],
      options,
    )
    expect(text).toBe("hour 0% | month 1%")
  })

  test("returns nothing when there is nothing to show", () => {
    expect(formatSummaries([], readOptions(undefined))).toBeUndefined()
  })
})
