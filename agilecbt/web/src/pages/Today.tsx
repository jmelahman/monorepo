import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import {
  api,
  type Checkin,
  type CheckinKind,
  type Health,
  type Lane,
  type Snapshot,
  type Step,
  type TodayStep,
} from "@/api/client";
import Chat from "@/components/Chat";
import CompleteDialog from "@/components/CompleteDialog";
import MoodChart from "@/components/MoodChart";
import {
  Button,
  Card,
  EnergyDots,
  ErrorText,
  inputClass,
  SectionTitle,
  Slider,
} from "@/components/ui";

// activeKind picks which check-in the page is about right now, honoring the
// morning/evening/both preference.
function activeKind(pref: string | undefined, snap: Snapshot): CheckinKind {
  if (pref === "morning") return "morning";
  if (pref === "evening") return "evening";
  const hour = new Date().getHours();
  return hour >= 17 || (snap.morning && hour >= 15) ? "evening" : "morning";
}

// energyHint turns reported energy into a gentle size for today's plan.
function energyHint(energy: number | null): string {
  if (energy === null) return "";
  if (energy <= 3) return "Low energy — one small step is plenty today.";
  if (energy <= 6) return "Medium energy — two or three steps, mostly light ones.";
  return "Good energy — a fuller day is okay, but leave room to rest.";
}

export default function Today({ health }: { health: Health }) {
  const snap = useQuery({ queryKey: ["today"], queryFn: api.today });
  const settings = useQuery({ queryKey: ["settings"], queryFn: api.settings });
  const mood = useQuery({ queryKey: ["mood", 14], queryFn: () => api.mood(14) });
  const [completing, setCompleting] = useState<Step | null>(null);

  if (!snap.data) return <ErrorText error={snap.error} />;
  const s = snap.data;
  const kind = activeKind(settings.data?.checkin_times, s);
  const checkin = kind === "morning" ? s.morning : s.evening;
  const latest = s.evening ?? s.morning;
  const dateLabel = new Date(`${s.date}T12:00:00`).toLocaleDateString(undefined, {
    weekday: "long",
    month: "long",
    day: "numeric",
  });

  return (
    <div className="space-y-4">
      <div>
        <p className="text-sm text-fg-muted">{dateLabel}</p>
        {s.week.intention && (
          <p className="text-sm">
            <span className="text-fg-muted">This week: </span>
            {s.week.intention}
          </p>
        )}
      </div>

      <CheckinCard kind={kind} checkin={checkin} />

      {checkin && health.llm.available && (
        <Card>
          <SectionTitle>Curator</SectionTitle>
          <Chat
            checkinId={checkin.id}
            prompt={
              kind === "morning"
                ? "Say hi, or tell your curator what's on your mind. It can help pick today's steps."
                : "Want to look back on the day together?"
            }
          />
        </Card>
      )}
      {checkin && !health.llm.available && health.llm.backend !== "none" && (
        <p className="text-xs text-fg-muted">Curator offline: {health.llm.detail}</p>
      )}

      <TodaySteps snap={s} energy={latest?.energy ?? null} onComplete={setCompleting} />

      <Card>
        <MoodChart days={mood.data ?? []} title="Last two weeks" />
      </Card>

      <CompleteDialog step={completing} onClose={() => setCompleting(null)} />
    </div>
  );
}

function CheckinCard({ kind, checkin }: { kind: CheckinKind; checkin: Checkin | null }) {
  const [editing, setEditing] = useState(false);
  if (checkin && !editing) {
    return (
      <Card className="flex items-center justify-between gap-3">
        <div className="text-sm">
          <p className="font-medium">{kind === "morning" ? "Morning" : "Evening"} check-in done</p>
          <p className="text-fg-muted">
            Mood {checkin.mood ?? "–"} · Energy {checkin.energy ?? "–"} · Anxiety{" "}
            {checkin.anxiety ?? "–"}
          </p>
        </div>
        <Button variant="ghost" onClick={() => setEditing(true)}>
          Edit
        </Button>
      </Card>
    );
  }
  return <CheckinForm kind={kind} checkin={checkin} onDone={() => setEditing(false)} />;
}

function CheckinForm({
  kind,
  checkin,
  onDone,
}: {
  kind: CheckinKind;
  checkin: Checkin | null;
  onDone: () => void;
}) {
  const qc = useQueryClient();
  const [mood, setMood] = useState(checkin?.mood ?? null);
  const [energy, setEnergy] = useState(checkin?.energy ?? null);
  const [anxiety, setAnxiety] = useState(checkin?.anxiety ?? null);
  const [note, setNote] = useState(checkin?.note ?? "");
  const save = useMutation({
    mutationFn: () => {
      const body = { mood, energy, anxiety, note };
      return checkin ? api.updateCheckin(checkin.id, body) : api.createCheckin({ ...body, kind });
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["today"] });
      qc.invalidateQueries({ queryKey: ["mood"] });
      onDone();
    },
  });
  return (
    <Card>
      <SectionTitle>
        {kind === "morning" ? "Good morning. How are you arriving?" : "How did today go?"}
      </SectionTitle>
      <form
        className="space-y-4"
        onSubmit={(e) => {
          e.preventDefault();
          save.mutate();
        }}
      >
        <Slider label="Mood" value={mood} onChange={setMood} low="very low" high="great" />
        <Slider label="Energy" value={energy} onChange={setEnergy} low="empty" high="full" />
        <Slider label="Anxiety" value={anxiety} onChange={setAnxiety} low="calm" high="intense" />
        <textarea
          value={note}
          onChange={(e) => setNote(e.target.value)}
          placeholder={
            kind === "morning"
              ? "Anything on your mind? (optional)"
              : "A word about today (optional)"
          }
          aria-label="Note"
          rows={2}
          className={inputClass}
        />
        <ErrorText error={save.error} />
        <Button type="submit" disabled={save.isPending} className="w-full">
          {checkin ? "Update check-in" : "Check in"}
        </Button>
      </form>
    </Card>
  );
}

function TodaySteps({
  snap,
  energy,
  onComplete,
}: {
  snap: Snapshot;
  energy: number | null;
  onComplete: (s: Step) => void;
}) {
  const qc = useQueryClient();
  const [title, setTitle] = useState("");
  const [cost, setCost] = useState(1);
  const [showWeek, setShowWeek] = useState(false);
  const refresh = () => qc.invalidateQueries();
  const add = useMutation({
    mutationFn: () => api.createStep("today", { title: title.trim(), energy_cost: cost }),
    onSuccess: () => {
      setTitle("");
      refresh();
    },
  });
  const move = useMutation({
    mutationFn: ({ id, lane }: { id: number; lane: Lane }) => api.moveStep(id, lane),
    onSuccess: refresh,
  });
  const planned = snap.today.reduce((n, st) => n + st.energy_cost, 0);

  return (
    <Card>
      <SectionTitle
        aside={
          snap.today.length > 0 && (
            <span className="text-xs text-fg-muted">{planned} energy planned</span>
          )
        }
      >
        Today
      </SectionTitle>
      {energy !== null && <p className="mb-3 text-xs text-fg-muted">{energyHint(energy)}</p>}

      <ul className="space-y-2">
        {snap.today.length === 0 && (
          <li className="text-sm text-fg-muted">
            Nothing planned yet. Pick something small from this week, or add one below.
          </li>
        )}
        {snap.today.map((st) => (
          <TodayRow
            key={st.id}
            step={st}
            onComplete={() => onComplete(st)}
            onMove={(lane) => move.mutate({ id: st.id, lane })}
          />
        ))}
      </ul>

      <form
        className="mt-3 flex gap-2"
        onSubmit={(e) => {
          e.preventDefault();
          if (title.trim()) add.mutate();
        }}
      >
        <input
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          placeholder="Add a step for today…"
          aria-label="New step for today"
          className={inputClass}
        />
        <select
          value={cost}
          onChange={(e) => setCost(Number(e.target.value))}
          aria-label="Energy cost"
          className="rounded-xl border border-border bg-bg px-2 text-sm"
        >
          <option value={1}>light</option>
          <option value={2}>medium</option>
          <option value={3}>heavy</option>
        </select>
        <Button type="submit" disabled={!title.trim() || add.isPending}>
          Add
        </Button>
      </form>
      <ErrorText error={add.error ?? move.error} />

      {snap.week_steps.length > 0 && (
        <div className="mt-4">
          <button
            type="button"
            className="text-sm text-fg-muted hover:text-fg"
            onClick={() => setShowWeek((v) => !v)}
            aria-expanded={showWeek}
          >
            {showWeek ? "▾" : "▸"} From this week ({snap.week_steps.length})
          </button>
          {showWeek && (
            <ul className="mt-2 space-y-1">
              {[...snap.week_steps]
                .sort((a, b) => a.energy_cost - b.energy_cost)
                .map((st) => (
                  <li key={st.id} className="flex items-center gap-2 text-sm">
                    <EnergyDots cost={st.energy_cost} />
                    <span className="flex-1">{st.title}</span>
                    <Button
                      variant="soft"
                      className="px-2 py-1 text-xs"
                      onClick={() => move.mutate({ id: st.id, lane: "today" })}
                      aria-label={`Move ${st.title} to today`}
                    >
                      Today →
                    </Button>
                  </li>
                ))}
            </ul>
          )}
        </div>
      )}

      {snap.done_today.length > 0 && (
        <div className="mt-4 rounded-xl bg-calm-soft p-3">
          <p className="mb-1 text-sm font-medium">Done today</p>
          <ul className="space-y-1 text-sm">
            {snap.done_today.map((st) => (
              <li key={st.id} className="flex items-center gap-2">
                <span className="text-calm">✓</span>
                <span className="flex-1">{st.title}</span>
                {(st.mastery !== null || st.pleasure !== null) && (
                  <span className="text-xs text-fg-muted">
                    M {st.mastery ?? "–"} · P {st.pleasure ?? "–"}
                  </span>
                )}
              </li>
            ))}
          </ul>
        </div>
      )}
    </Card>
  );
}

function TodayRow({
  step,
  onComplete,
  onMove,
}: {
  step: TodayStep;
  onComplete: () => void;
  onMove: (lane: Lane) => void;
}) {
  return (
    <li className="rounded-xl border border-border p-2">
      <div className="flex items-center gap-3">
        <button
          type="button"
          onClick={onComplete}
          aria-label={`Complete ${step.title}`}
          className="h-6 w-6 shrink-0 rounded-full border-2 border-accent-500 hover:bg-accent-500/20"
        />
        <span className="flex-1 text-sm">{step.title}</span>
        <EnergyDots cost={step.energy_cost} />
      </div>
      {step.carried_over && (
        <div className="mt-2 flex flex-wrap items-center gap-1 pl-9 text-xs text-fg-muted">
          <span className="mr-1">Still here from before — that's okay.</span>
          <Button variant="soft" className="px-2 py-0.5 text-xs" onClick={() => onMove("today")}>
            Keep for today
          </Button>
          <Button variant="soft" className="px-2 py-0.5 text-xs" onClick={() => onMove("week")}>
            Back to this week
          </Button>
          <Button variant="ghost" className="px-2 py-0.5 text-xs" onClick={() => onMove("let_go")}>
            Let go
          </Button>
        </div>
      )}
    </li>
  );
}
