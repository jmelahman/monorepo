import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { api, type DayMood, type Health, type Week, type WeekReview } from "@/api/client";
import MoodChart from "@/components/MoodChart";
import { Button, Card, ErrorText, inputClass, Markdownish, SectionTitle } from "@/components/ui";

function addDays(date: string, n: number) {
  const d = new Date(`${date}T12:00:00`);
  d.setDate(d.getDate() + n);
  return d.toISOString().slice(0, 10);
}

// weekMood averages each day's check-ins into one point per metric.
function weekMood(r: WeekReview): DayMood[] {
  return Array.from({ length: 7 }, (_, i) => {
    const date = addDays(r.week.start_date, i);
    const day = r.checkins.filter((c) => c.date === date);
    const avg = (k: "mood" | "energy" | "anxiety") => {
      const vs = day.map((c) => c[k]).filter((v): v is number => v !== null);
      return vs.length ? Math.round((vs.reduce((a, b) => a + b, 0) / vs.length) * 10) / 10 : null;
    };
    return { date, mood: avg("mood"), energy: avg("energy"), anxiety: avg("anxiety") };
  });
}

function mean(xs: (number | null)[]) {
  const vs = xs.filter((v): v is number => v !== null);
  return vs.length ? (vs.reduce((a, b) => a + b, 0) / vs.length).toFixed(1) : "–";
}

export default function Retro({ health }: { health: Health }) {
  const review = useQuery({ queryKey: ["week", "current"], queryFn: api.currentWeek });
  if (!review.data) return <ErrorText error={review.error} />;
  const r = review.data;
  const fmt = (d: string) =>
    new Date(`${d}T12:00:00`).toLocaleDateString(undefined, { month: "short", day: "numeric" });

  const topPleasure = [...r.completed]
    .filter((s) => s.pleasure !== null)
    .sort((a, b) => (b.pleasure ?? 0) - (a.pleasure ?? 0))
    .slice(0, 3);

  return (
    <div className="space-y-4">
      <div>
        <h1 className="text-xl font-semibold">Weekly retro</h1>
        <p className="text-sm text-fg-muted">
          {fmt(r.week.start_date)} – {fmt(r.end_date)} · Look back with curiosity, not judgment.
        </p>
      </div>

      <Intention week={r.week} />

      <Card>
        <MoodChart days={weekMood(r)} title="This week" />
      </Card>

      <Card>
        <SectionTitle>What happened</SectionTitle>
        <dl className="grid grid-cols-3 gap-2 text-center">
          <Stat label="steps done" value={String(r.completed.length)} />
          <Stat label="avg mastery" value={mean(r.completed.map((s) => s.mastery))} />
          <Stat label="avg pleasure" value={mean(r.completed.map((s) => s.pleasure))} />
        </dl>
        {topPleasure.length > 0 && (
          <div className="mt-3 text-sm">
            <p className="text-fg-muted">Most enjoyable:</p>
            <ul className="list-inside list-disc">
              {topPleasure.map((s) => (
                <li key={s.id}>
                  {s.title} <span className="text-fg-muted">(pleasure {s.pleasure})</span>
                </li>
              ))}
            </ul>
          </div>
        )}
        <p className="mt-3 text-sm text-fg-muted">
          {r.checkins.length} check-ins · {r.thought_records.length} thought records
        </p>
      </Card>

      <RetroForm review={r} canDraft={health.llm.available} />
    </div>
  );
}

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-xl bg-surface-2 p-2">
      <dd className="text-xl font-semibold tabular-nums">{value}</dd>
      <dt className="text-xs text-fg-muted">{label}</dt>
    </div>
  );
}

function Intention({ week }: { week: Week }) {
  const qc = useQueryClient();
  const [text, setText] = useState(week.intention);
  useEffect(() => setText(week.intention), [week.intention]);
  const weeks = useQuery({ queryKey: ["weeks"], queryFn: api.weeks, enabled: !week.intention });
  const prev = weeks.data
    ?.filter((w) => w.start_date < week.start_date)
    .sort((a, b) => b.start_date.localeCompare(a.start_date))[0];
  const prevReview = useQuery({
    queryKey: ["week", prev?.id],
    queryFn: () => api.weekReview((prev as Week).id),
    enabled: !!prev,
  });
  const suggestion = prevReview.data?.retro?.try_next;
  const save = useMutation({
    mutationFn: (intention: string) => api.setIntention(week.id, intention),
    onSuccess: () => qc.invalidateQueries(),
  });
  return (
    <Card>
      <SectionTitle>This week's intention</SectionTitle>
      {!week.intention && suggestion && (
        <div className="mb-3 rounded-xl bg-surface-2 p-3 text-sm">
          <p className="text-fg-muted">Last retro, you wanted to try:</p>
          <p className="my-1">{suggestion}</p>
          <Button
            variant="soft"
            className="px-2 py-1 text-xs"
            onClick={() => save.mutate(suggestion)}
          >
            Use as this week's intention
          </Button>
        </div>
      )}
      <form
        className="flex gap-2"
        onSubmit={(e) => {
          e.preventDefault();
          save.mutate(text.trim());
        }}
      >
        <input
          value={text}
          onChange={(e) => setText(e.target.value)}
          placeholder="One gentle focus for the week"
          aria-label="Week intention"
          className={inputClass}
        />
        <Button
          type="submit"
          variant="soft"
          disabled={save.isPending || text.trim() === week.intention}
        >
          Save
        </Button>
      </form>
      <ErrorText error={save.error} />
    </Card>
  );
}

function RetroForm({ review, canDraft }: { review: WeekReview; canDraft: boolean }) {
  const qc = useQueryClient();
  const weekId = review.week.id;
  const [wentWell, setWentWell] = useState(review.retro?.went_well ?? "");
  const [wasHard, setWasHard] = useState(review.retro?.was_hard ?? "");
  const [tryNext, setTryNext] = useState(review.retro?.try_next ?? "");
  const [aiDraft, setAiDraft] = useState(review.retro?.ai_draft ?? "");
  const [saved, setSaved] = useState(false);

  const save = useMutation({
    mutationFn: () =>
      api.putRetro(weekId, { went_well: wentWell, was_hard: wasHard, try_next: tryNext }),
    onSuccess: () => {
      setSaved(true);
      qc.invalidateQueries({ queryKey: ["week"] });
    },
  });
  const draft = useMutation({
    mutationFn: async () => {
      // Save first so the curator keeps anything already written.
      await api.putRetro(weekId, { went_well: wentWell, was_hard: wasHard, try_next: tryNext });
      return api.draftRetro(weekId);
    },
    onSuccess: (r) => {
      setWentWell(r.went_well);
      setWasHard(r.was_hard);
      setTryNext(r.try_next);
      setAiDraft(r.ai_draft);
      qc.invalidateQueries({ queryKey: ["week"] });
    },
  });

  const field = (label: string, value: string, set: (v: string) => void, placeholder: string) => (
    <label className="block text-sm">
      <span className="font-medium">{label}</span>
      <textarea
        value={value}
        onChange={(e) => {
          set(e.target.value);
          setSaved(false);
        }}
        placeholder={placeholder}
        rows={3}
        className={`${inputClass} mt-1`}
      />
    </label>
  );

  return (
    <Card>
      <SectionTitle
        aside={
          canDraft && (
            <Button variant="soft" onClick={() => draft.mutate()} disabled={draft.isPending}>
              {draft.isPending ? "Drafting…" : "Draft with AI"}
            </Button>
          )
        }
      >
        Reflection
      </SectionTitle>
      <div className="space-y-3">
        {field("What helped", wentWell, setWentWell, "Moments, people or habits that lifted you")}
        {field("What was hard", wasHard, setWasHard, "Name it kindly")}
        {field("One thing to try next week", tryNext, setTryNext, "Small and specific")}
        {aiDraft && (
          <details className="rounded-xl bg-surface-2 p-3 text-sm">
            <summary className="cursor-pointer font-medium">Coach's notes</summary>
            <Markdownish text={aiDraft} className="mt-2" />
          </details>
        )}
        <ErrorText error={save.error ?? draft.error} />
        <div className="flex items-center gap-3">
          <Button onClick={() => save.mutate()} disabled={save.isPending}>
            Save retro
          </Button>
          {saved && <span className="text-sm text-calm">Saved</span>}
        </div>
      </div>
    </Card>
  );
}
