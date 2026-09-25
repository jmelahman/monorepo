import { useState } from "react";
import type { DayMood } from "@/api/client";

// Mood, energy and anxiety share one 0–10 scale, so one axis is honest.
// Colors are the validated first three categorical slots (see index.css).
const series = [
  { key: "mood", label: "Mood", color: "var(--color-mood)" },
  { key: "energy", label: "Energy", color: "var(--color-energy)" },
  { key: "anxiety", label: "Anxiety", color: "var(--color-anxiety)" },
] as const;

const W = 320;
const H = 120;
const PAD = { l: 18, r: 58, t: 8, b: 16 };

function fmtDay(date: string) {
  return new Date(`${date}T12:00:00`).toLocaleDateString(undefined, {
    weekday: "short",
    month: "short",
    day: "numeric",
  });
}

export default function MoodChart({ days, title }: { days: DayMood[]; title: string }) {
  const [hover, setHover] = useState<number | null>(null);
  const [table, setTable] = useState(false);
  const logged = days.filter((d) => d.mood !== null || d.energy !== null || d.anxiety !== null);

  if (logged.length === 0) {
    return (
      <p className="text-sm text-fg-muted">
        Your mood trend will appear here after a couple of check-ins.
      </p>
    );
  }

  const x = (i: number) => PAD.l + (i * (W - PAD.l - PAD.r)) / Math.max(days.length - 1, 1);
  const y = (v: number) => PAD.t + ((10 - v) * (H - PAD.t - PAD.b)) / 10;

  // Split each series into runs so missing days show as gaps, not fake slopes.
  const paths = series.map((s) => {
    let d = "";
    let pen = false;
    days.forEach((day, i) => {
      const v = day[s.key];
      if (v === null) {
        pen = false;
        return;
      }
      d += `${pen ? "L" : "M"}${x(i).toFixed(1)},${y(v).toFixed(1)}`;
      pen = true;
    });
    return d;
  });

  // Direct label at each series' last logged value, nudged apart vertically.
  const lastIdx = days.indexOf(logged[logged.length - 1]);
  const labels = series
    .map((s) => ({ ...s, v: days[lastIdx][s.key] }))
    .filter((s): s is typeof s & { v: number } => s.v !== null)
    .map((s) => ({ ...s, ly: y(s.v) }))
    .sort((a, b) => a.ly - b.ly);
  for (let i = 1; i < labels.length; i++) {
    labels[i].ly = Math.max(labels[i].ly, labels[i - 1].ly + 11);
  }

  const hd = hover === null ? null : days[hover];

  return (
    <figure className="space-y-2">
      <figcaption className="flex items-baseline justify-between gap-2">
        <span className="text-sm font-medium">{title}</span>
        <button
          type="button"
          className="text-xs text-fg-muted underline-offset-2 hover:underline"
          onClick={() => setTable((t) => !t)}
        >
          {table ? "Show chart" : "Show table"}
        </button>
      </figcaption>

      {table ? (
        <MoodTable days={logged} />
      ) : (
        <div className="relative">
          <svg
            viewBox={`0 0 ${W} ${H}`}
            className="w-full touch-none select-none"
            role="img"
            aria-label={`${title}: mood, energy and anxiety over ${days.length} days`}
            onPointerLeave={() => setHover(null)}
            onPointerMove={(e) => {
              const r = e.currentTarget.getBoundingClientRect();
              const px = ((e.clientX - r.left) / r.width) * W;
              const i = Math.round(((px - PAD.l) / (W - PAD.l - PAD.r)) * (days.length - 1));
              setHover(Math.min(Math.max(i, 0), days.length - 1));
            }}
          >
            {[0, 5, 10].map((v) => (
              <g key={v}>
                <line
                  x1={PAD.l}
                  x2={W - PAD.r}
                  y1={y(v)}
                  y2={y(v)}
                  stroke="var(--color-border)"
                  strokeWidth={1}
                  strokeDasharray={v === 0 ? undefined : "2 3"}
                />
                <text
                  x={PAD.l - 4}
                  y={y(v) + 3}
                  textAnchor="end"
                  fontSize={8}
                  fill="var(--color-fg-muted)"
                >
                  {v}
                </text>
              </g>
            ))}
            {hover !== null && (
              <line
                x1={x(hover)}
                x2={x(hover)}
                y1={PAD.t}
                y2={H - PAD.b}
                stroke="var(--color-fg-muted)"
                strokeWidth={1}
              />
            )}
            {series.map((s, si) => (
              <path
                key={s.key}
                d={paths[si]}
                fill="none"
                stroke={s.color}
                strokeWidth={2}
                strokeLinecap="round"
                strokeLinejoin="round"
              />
            ))}
            {/* Isolated single days would be invisible as lines; mark every point. */}
            {series.map((s) =>
              days.map((d, i) =>
                d[s.key] === null ? null : (
                  <circle
                    key={`${s.key}-${d.date}`}
                    cx={x(i)}
                    cy={y(d[s.key] as number)}
                    r={hover === i ? 4 : 2.5}
                    fill={s.color}
                    stroke="var(--color-surface)"
                    strokeWidth={1.5}
                  />
                ),
              ),
            )}
            {labels.map((l) => (
              <text
                key={l.key}
                x={W - PAD.r + 6}
                y={l.ly + 3}
                fontSize={9}
                fill="var(--color-fg-muted)"
              >
                <tspan fill={l.color}>●</tspan> {l.label} {l.v}
              </text>
            ))}
            <text x={PAD.l} y={H - 3} fontSize={8} fill="var(--color-fg-muted)">
              {fmtDay(days[0].date)}
            </text>
            <text
              x={W - PAD.r}
              y={H - 3}
              fontSize={8}
              textAnchor="end"
              fill="var(--color-fg-muted)"
            >
              today
            </text>
          </svg>

          {hd && (
            <div
              className="pointer-events-none absolute top-0 rounded-lg border border-border bg-surface-2 px-2 py-1 text-xs shadow"
              style={{
                left: `${(x(hover as number) / W) * 100}%`,
                transform:
                  (hover as number) > days.length / 2 ? "translateX(-105%)" : "translateX(5%)",
              }}
            >
              <div className="font-medium">{fmtDay(hd.date)}</div>
              {series.map((s) => (
                <div key={s.key} className="flex items-center gap-1.5 text-fg-muted">
                  <span className="h-2 w-2 rounded-full" style={{ background: s.color }} />
                  {s.label}{" "}
                  <span className="ml-auto pl-2 tabular-nums text-fg">{hd[s.key] ?? "–"}</span>
                </div>
              ))}
            </div>
          )}

          <ul className="mt-1 flex gap-3 text-xs text-fg-muted" aria-label="Legend">
            {series.map((s) => (
              <li key={s.key} className="flex items-center gap-1">
                <span className="h-0.5 w-3 rounded" style={{ background: s.color }} />
                {s.label}
              </li>
            ))}
          </ul>
          <div className="sr-only">
            <MoodTable days={logged} />
          </div>
        </div>
      )}
    </figure>
  );
}

function MoodTable({ days }: { days: DayMood[] }) {
  return (
    <table className="w-full text-left text-xs">
      <thead className="text-fg-muted">
        <tr>
          <th className="py-1 font-normal">Day</th>
          {series.map((s) => (
            <th key={s.key} className="py-1 text-right font-normal">
              {s.label}
            </th>
          ))}
        </tr>
      </thead>
      <tbody>
        {days.map((d) => (
          <tr key={d.date} className="border-t border-border">
            <td className="py-1">{fmtDay(d.date)}</td>
            {series.map((s) => (
              <td key={s.key} className="py-1 text-right tabular-nums">
                {d[s.key] ?? "–"}
              </td>
            ))}
          </tr>
        ))}
      </tbody>
    </table>
  );
}
