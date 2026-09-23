// Runtime reporting wires three sources of automatic frontend error capture
// on top of the React ErrorBoundary / React Query reporting handled in
// main.tsx:
//
//   1. window "error" — uncaught exceptions outside React's render tree
//      (event handlers, async timers, native callbacks).
//   2. window "unhandledrejection" — rejected promises with no .catch.
//   3. PerformanceObserver("longtask") — main-thread tasks blocking the UI
//      above LONG_TASK_THRESHOLD_MS. This is what surfaces the
//      "switching to the terminal tab freezes the app" class of regression:
//      the freeze itself throws nothing, but the long task is reported when
//      the main thread eventually unblocks.
//
// All paths funnel through postError which caps concurrent requests and
// silently swallows transport failures, so this module can never amplify an
// existing problem.
//
// Title strings are stable per (source, kind, location.pathname) so the
// backend's fingerprint dedup collapses repeated occurrences into a single
// ticket with "Seen again at ..." entries.

import { postError } from "@/api/client";

const LONG_TASK_THRESHOLD_MS = 2000;
// Coarsely bucket durations so each magnitude maps to one ticket instead of a
// new ticket per millisecond. Keep these stable — they are part of the title
// and therefore the fingerprint.
const DURATION_BUCKETS_MS = [2000, 5000, 10000, 30000, 60000];

let installed = false;

export function installRuntimeReporting(): void {
  if (installed) return;
  installed = true;

  window.addEventListener("error", (event) => {
    const err = event.error;
    const message =
      (err instanceof Error ? err.message : null) || event.message || "Unknown window error";
    const stack = err instanceof Error ? (err.stack ?? "") : "";
    void postError({
      message,
      stack,
      source: "window",
      meta: {
        filename: event.filename ?? "",
        lineno: String(event.lineno ?? ""),
        colno: String(event.colno ?? ""),
      },
    });
  });

  window.addEventListener("unhandledrejection", (event) => {
    const reason = event.reason;
    let message: string;
    let stack = "";
    if (reason instanceof Error) {
      message = reason.message || "Unhandled rejection";
      stack = reason.stack ?? "";
    } else if (typeof reason === "string") {
      message = reason;
    } else {
      try {
        message = `Unhandled rejection: ${JSON.stringify(reason)}`;
      } catch {
        message = "Unhandled rejection (non-serializable)";
      }
    }
    void postError({ message, stack, source: "unhandledrejection" });
  });

  installLongTaskObserver();
}

function installLongTaskObserver(): void {
  if (typeof PerformanceObserver === "undefined") return;
  const supported = PerformanceObserver.supportedEntryTypes;
  if (!supported?.includes("longtask")) return;

  // Collapse a callback's batch to its worst entry. With `buffered: true`
  // a fresh observer can flush a dozen tasks at once, all mapping to the
  // same (bucket, pathname) fingerprint — sending them individually would
  // burn the postError in-flight budget without adding signal.
  const callback = (list: PerformanceObserverEntryList) => {
    let max = 0;
    for (const entry of list.getEntries()) {
      if (entry.duration < LONG_TASK_THRESHOLD_MS) continue;
      if (entry.duration > max) max = entry.duration;
    }
    if (max >= LONG_TASK_THRESHOLD_MS) reportLongTask(max);
  };

  try {
    const observer = new PerformanceObserver(callback);
    observer.observe({ type: "longtask", buffered: true });
  } catch {
    // Older Safari throws on { type, buffered } — fall back to the legacy form.
    try {
      const observer = new PerformanceObserver(callback);
      observer.observe({ entryTypes: ["longtask"] });
    } catch {
      // No longtask support — skip silently.
    }
  }
}

function reportLongTask(duration: number): void {
  const bucket = bucketDuration(duration);
  const pathname = window.location.pathname || "/";
  void postError({
    message: `Long task ≥${bucket}ms on ${pathname}`,
    // Synthetic stack: the first line has a ":" so it survives the backend's
    // top-3-frame fingerprint filter while staying stable per pathname.
    stack: `longtask: ${pathname}`,
    source: "longtask",
    meta: {
      duration_ms: String(Math.round(duration)),
      threshold_ms: String(LONG_TASK_THRESHOLD_MS),
      pathname,
      document_title: typeof document !== "undefined" ? document.title : "",
    },
  });
}

function bucketDuration(ms: number): number {
  let chosen = DURATION_BUCKETS_MS[0];
  for (const b of DURATION_BUCKETS_MS) {
    if (ms >= b) chosen = b;
  }
  return chosen;
}
