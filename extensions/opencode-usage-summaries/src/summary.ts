export type WindowId = "hour" | "week" | "month";

/** One rolling usage window: how far back it reaches and how it is labelled. */
export interface SummaryWindow {
  readonly id: WindowId;
  /** Rolling span, in milliseconds. */
  readonly span: number;
  /** Default footer label. */
  readonly label: string;
}

export const WINDOWS: Readonly<Record<WindowId, SummaryWindow>> = {
  hour: { id: "hour", span: 3_600_000, label: "1h" },
  week: { id: "week", span: 7 * 86_400_000, label: "7d" },
  month: { id: "month", span: 30 * 86_400_000, label: "30d" },
};

export const DEFAULT_WINDOWS: readonly WindowId[] = ["hour", "week", "month"];
export const DEFAULT_SEPARATOR = " · ";

/** Which usage numbers to show: subscription meters or computed session cost. */
export type Source = "subscription" | "cost";
export const DEFAULT_SOURCE: Source = "subscription";

/** The subscription meter that backs each window. */
export const METER_KEYS: Readonly<Record<WindowId, "rolling" | "weekly" | "monthly">> = {
  hour: "rolling",
  week: "weekly",
  month: "monthly",
};

export interface Options {
  /** Which windows to show, in order. Defaults to hour, week, month. */
  readonly windows: readonly WindowId[];
  /** Text between summaries. Defaults to `" · "`. */
  readonly separator: string;
  /** Footer label overrides per window. */
  readonly labels: Readonly<Partial<Record<WindowId, string>>>;
  /**
   * Where the numbers come from. `"subscription"` (default) shows the plan's
   * usage meters as percentages; `"cost"` shows computed session cost. When
   * the server reports no subscription meters, `"subscription"` falls back to
   * `"cost"` values.
   */
  readonly source: Source;
  /**
   * Replace the prompt footer's status area instead of appending to it:
   * OpenCode's animated running strip is removed, and a plain location label
   * plus the summaries render in its place. The home footer is unaffected.
   * Defaults to true; pass `false` to keep the built-in strip.
   */
  readonly hideRunningStrip: boolean;
}

/** A window's usage, as fetched for one refresh. */
export interface SummaryValue {
  readonly id: WindowId;
  /** The plan meter's percentage, for the subscription source. */
  readonly percent?: number;
  /** Whether the plan meter has hit its limit, for the subscription source. */
  readonly limited?: boolean;
  /** Computed cost in USD, for the cost source. */
  readonly cost?: number;
}

/** How urgent a meter is: drives the footer color. */
export type ColorBand = "muted" | "warning" | "error";

/** A meter at or above this percentage turns warning-colored. */
export const WARN_AT = 70;
/** A meter at or above this percentage, or when limited, turns error-colored. */
export const ERROR_AT = 90;

/**
 * The color band for a subscription meter: muted normally, warning as it
 * fills, error once it is nearly exhausted or the plan is limiting requests.
 */
export function colorBand(percent: number, limited = false): ColorBand {
  if (limited || percent >= ERROR_AT) return "error";
  if (percent >= WARN_AT) return "warning";
  return "muted";
}

/** One colored run of the footer text. */
export interface SummarySegment {
  readonly text: string;
  readonly band: ColorBand;
}

/**
 * The footer text as colored segments: each window's `1h 5%` carries its own
 * color band, separators stay muted. Undefined when there is nothing to
 * show.
 */
export function summarySegments(
  values: readonly SummaryValue[],
  options: Options,
): readonly SummarySegment[] | undefined {
  if (values.length === 0) return undefined;
  const segments: SummarySegment[] = [];
  for (const [index, value] of values.entries()) {
    if (index > 0) segments.push({ text: options.separator, band: "muted" });
    segments.push({
      text: `${labelFor(value.id, options)} ${amountFor(value)}`,
      band: value.percent !== undefined ? colorBand(value.percent, value.limited) : "muted",
    });
  }
  return segments;
}

/**
 * Parses and validates the plugin options. Invalid `windows` entries and
 * non-string labels are dropped; an empty or missing `windows` falls back to
 * the default order. Called once per `setup`, including config reloads.
 */
export function readOptions(options: Readonly<Record<string, any>> | undefined): Options {
  const rawWindows = options?.windows;
  const windows = Array.isArray(rawWindows)
    ? rawWindows.filter((id): id is WindowId => typeof id === "string" && id in WINDOWS)
    : [];
  const rawLabels = options?.labels;
  const labels: Partial<Record<WindowId, string>> = {};
  if (rawLabels !== null && typeof rawLabels === "object") {
    for (const id of Object.keys(WINDOWS) as WindowId[]) {
      const label = rawLabels[id];
      if (typeof label === "string") labels[id] = label;
    }
  }
  return {
    windows: windows.length > 0 ? windows : DEFAULT_WINDOWS,
    separator: typeof options?.separator === "string" ? options.separator : DEFAULT_SEPARATOR,
    labels,
    source: isSource(options?.source) ? options.source : DEFAULT_SOURCE,
    hideRunningStrip: options?.hideRunningStrip !== false,
  };
}

function isSource(value: unknown): value is Source {
  return value === "subscription" || value === "cost";
}

/** The `[from, to)` range a window covers, as epoch milliseconds. */
export function rangeFor(
  id: WindowId,
  now = Date.now(),
): { readonly from: number; readonly to: number } {
  return { from: now - WINDOWS[id].span, to: now };
}

/**
 * A compact USD amount. Precision grows for small amounts so an hourly total
 * is not rounded to `$0.00`, and drops beyond a thousand dollars to keep the
 * footer narrow.
 */
export function formatCost(value: number): string {
  if (!Number.isFinite(value) || value < 0) return "$?";
  if (value < 0.0005) return "$0";
  if (value < 0.1) return `$${value.toFixed(3)}`;
  if (value < 1000) return `$${value.toFixed(2)}`;
  return `$${new Intl.NumberFormat("en-US", { maximumFractionDigits: 0 }).format(value)}`;
}

/**
 * A compact percentage of the plan's usage meter, rounded to a whole number
 * like `5%` or `90%`.
 */
export function formatPercent(value: number): string {
  if (!Number.isFinite(value) || value < 0) return "?%";
  return `${Math.round(value)}%`;
}

/**
 * Joins the windows' usage into the footer text, for example
 * `1h 5% · 7d 90% · 30d 40%` for subscription meters or
 * `1h $0.42 · 7d $12.30 · 30d $45.10` for computed cost. Undefined when
 * there is nothing to show, so the footer renders nothing until the first
 * fetch resolves.
 */
export function formatSummaries(
  values: readonly SummaryValue[],
  options: Options,
): string | undefined {
  return summarySegments(values, options)
    ?.map((segment) => segment.text)
    .join("");
}

function amountFor(value: SummaryValue): string {
  return value.percent !== undefined ? formatPercent(value.percent) : formatCost(value.cost ?? 0);
}

/**
 * The prompt footer's location label: the abbreviated working directory, with
 * the VCS branch appended after a colon. `formatPath` is the host's
 * home-abbreviating formatter (`context.ui.format.path`).
 */
export function formatLocationLabel(
  directory: string,
  branch: string | undefined,
  formatPath: (value: string) => string,
): string {
  const location = formatPath(directory);
  return branch ? `${location}:${branch}` : location;
}

function labelFor(id: WindowId, options: Options): string {
  return options.labels[id] ?? WINDOWS[id].label;
}
