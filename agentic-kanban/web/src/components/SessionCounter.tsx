import { useQuery } from "@tanstack/react-query";
import { useRef, useState } from "react";
import { api, type SessionSummary } from "@/api/client";
import { useClickOutside } from "@/hooks/useClickOutside";
import { STATUS_BG } from "./Ticket";

// Status breakdown rendered inline in the header. `working`/`awaiting_perm`/`idle`
// always render; `starting` only appears while something is spinning up.
const ROWS: { key: keyof SessionSummary; status: string; label: string; always: boolean }[] = [
  { key: "working", status: "working", label: "working", always: true },
  { key: "awaiting_perm", status: "awaiting_perm", label: "awaiting", always: true },
  { key: "idle", status: "idle", label: "idle", always: true },
  { key: "starting", status: "starting", label: "starting", always: false },
];

// Highest-signal status wins the single dot in compact mode: a working session
// is more interesting than one awaiting perms, which beats starting, then idle.
const DOMINANT_ORDER: (keyof SessionSummary)[] = ["working", "awaiting_perm", "starting", "idle"];

// SessionCounter is a header applet showing the instance-wide number of running
// containers, polling the cheap /api/sessions/summary aggregate every few
// seconds. It has two responsive forms that share one query and swap via CSS at
// the `lg` breakpoint — the same point the rest of the header gains text labels:
//
//   - below `lg` (mobile + medium widths): a single button — one dot tinted by
//     the dominant status plus the total running count — so the header stays on
//     one line. Tapping it opens a popover with the same per-status breakdown
//     (the desktop `title` tooltip is unreachable on touch) plus a "View all
//     sessions" action wired to `onActivate`, which the header points at the
//     Overview.
//   - at `lg` and up: the full per-status breakdown rendered inline, a colored
//     dot + count for each status, once the header has room to be verbose.
export function SessionCounter({ onActivate }: { onActivate?: () => void } = {}) {
  const { data } = useQuery({
    queryKey: ["session-summary"],
    queryFn: api.sessionSummary,
    refetchInterval: 3000,
  });

  const [open, setOpen] = useState(false);
  const popRef = useRef<HTMLDivElement | null>(null);
  useClickOutside(popRef, () => setOpen(false), { enabled: open });

  const running = data?.running ?? 0;
  const title =
    running === 0
      ? "No running sessions"
      : `${running} running — ${data?.working ?? 0} working, ${data?.awaiting_perm ?? 0} awaiting, ${data?.idle ?? 0} idle`;
  const dominant = DOMINANT_ORDER.find((key) => (data?.[key] ?? 0) > 0);
  const rows = ROWS.filter((row) => row.always || (data?.[row.key] ?? 0) > 0);

  return (
    <>
      <div ref={popRef} className="relative inline-flex lg:hidden">
        <button
          type="button"
          onClick={() => setOpen((v) => !v)}
          aria-expanded={open}
          aria-label={title}
          className="inline-flex items-center gap-1.5 rounded bg-surface-2 px-2 py-1 text-xs tabular-nums text-fg transition-colors duration-150 hover:bg-surface-3"
        >
          <span
            aria-hidden
            className={`h-2 w-2 rounded-full ${dominant ? STATUS_BG[dominant] : "bg-fg-muted opacity-40"}`}
          />
          {running}
        </button>
        {open && (
          <div className="absolute right-0 top-full z-20 mt-1 w-44 overflow-hidden rounded border border-border bg-surface text-sm shadow-lg">
            <div className="px-3 py-2">
              <div className="mb-1.5 text-xs font-semibold text-fg-muted">
                {running === 0 ? "No running sessions" : `${running} running`}
              </div>
              <ul className="flex flex-col gap-1 tabular-nums">
                {rows.map((row) => {
                  const count = data?.[row.key] ?? 0;
                  return (
                    <li
                      key={row.key}
                      className={`flex items-center justify-between gap-2 ${
                        count === 0 ? "text-fg-muted" : "text-fg"
                      }`}
                    >
                      <span className="inline-flex items-center gap-2">
                        <span
                          aria-hidden
                          className={`h-2 w-2 rounded-full ${STATUS_BG[row.status] ?? "bg-fg-muted"} ${
                            count === 0 ? "opacity-40" : ""
                          }`}
                        />
                        {row.label}
                      </span>
                      {count}
                    </li>
                  );
                })}
              </ul>
            </div>
            {onActivate && (
              <button
                type="button"
                onClick={() => {
                  setOpen(false);
                  onActivate();
                }}
                className="flex w-full items-center border-t border-border px-3 py-2 text-left text-fg hover:bg-surface-2"
              >
                View all sessions
              </button>
            )}
          </div>
        )}
      </div>
      <div
        className="mr-2 hidden items-center gap-2.5 text-xs tabular-nums lg:inline-flex"
        title={title}
      >
        {rows.map((row) => {
          const count = data?.[row.key] ?? 0;
          return (
            <span
              key={row.key}
              className={`inline-flex items-center gap-1 ${count === 0 ? "text-fg-muted" : "text-fg"}`}
            >
              <span
                aria-hidden
                className={`h-2 w-2 rounded-full ${STATUS_BG[row.status] ?? "bg-fg-muted"} ${
                  count === 0 ? "opacity-40" : ""
                }`}
              />
              <span className="sr-only">{row.label} </span>
              {count}
            </span>
          );
        })}
      </div>
    </>
  );
}
