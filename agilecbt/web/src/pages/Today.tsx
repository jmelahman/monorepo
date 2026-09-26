import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Fragment, type ReactNode, useEffect, useRef, useState } from "react";
import {
  api,
  type Checkin,
  type CheckinDetail,
  type CheckinKind,
  type Health,
  type Lane,
  type Snapshot,
  type Step,
  type TodayStep,
} from "@/api/client";
import ChatPage from "@/components/ChatPage";
import Chat, { Bubble, Composer } from "@/components/Chat";
import CompleteDialog from "@/components/CompleteDialog";
import MoodChart from "@/components/MoodChart";
import {
  Button,
  Card,
  Collapsible,
  EnergyDots,
  ErrorText,
  inputClass,
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

// Without a coach, a standup is a few short questions asked in the chat, one
// at a time. The first also takes the mood/energy/anxiety readings. Each has
// quick replies for when typing is too much, and all but the first can be
// skipped.
// With a coach, only the first question's quick replies are used; the coach
// takes it from there.
interface Question {
  text: string;
  hint: string;
  replies: string[];
}

const SKIP = "Skip";

const questions: Record<"morning" | "evening", Question[]> = {
  morning: [
    {
      text: "How are you doing?",
      hint: "In a few words, if you like.",
      replies: ["Okay", "Not great", "Pretty good"],
    },
    {
      text: "What would you like to get done today?",
      hint: "Even one small thing counts.",
      replies: ["Not sure yet", "Just rest"],
    },
    {
      text: "Anything that might get in the way?",
      hint: "Worries, low energy, a busy day.",
      replies: ["Nothing comes to mind"],
    },
  ],
  evening: [
    {
      text: "How did today go?",
      hint: "In a few words, if you like.",
      replies: ["Okay", "Hard", "Pretty good"],
    },
    {
      text: "What went okay, even a little?",
      hint: "A shower counts. So does resting.",
      replies: ["Not much, honestly"],
    },
    {
      text: "Anything still on your mind?",
      hint: "You can leave it here for tonight.",
      replies: ["Nothing, I'm done for today"],
    },
  ],
};

export default function Today({ health }: { health: Health }) {
  const snap = useQuery({ queryKey: ["today"], queryFn: api.today });
  const settings = useQuery({ queryKey: ["settings"], queryFn: api.settings });
  const [completing, setCompleting] = useState<Step | null>(null);
  const [editing, setEditing] = useState(false);

  if (!snap.data) return <ErrorText error={snap.error} />;
  const s = snap.data;
  const kind = activeKind(settings.data?.checkin_times, s);
  const checkin = kind === "morning" ? s.morning : s.evening;
  const coach = health.llm.available;
  const dateLabel = new Date(`${s.date}T12:00:00`).toLocaleDateString(undefined, {
    weekday: "long",
    month: "long",
    day: "numeric",
  });
  const header = (
    <div className="shrink-0">
      <p className="text-sm text-fg-muted">{dateLabel}</p>
      {s.week.intention && (
        <p className="text-sm">
          <span className="text-fg-muted">This week: </span>
          {s.week.intention}
        </p>
      )}
    </div>
  );
  const complete = <CompleteDialog step={completing} onClose={() => setCompleting(null)} />;

  if (coach) {
    return (
      <>
        <CoachStandup
          key={kind}
          kind={kind === "evening" ? "evening" : "morning"}
          snap={s}
          checkin={checkin}
          header={header}
          onComplete={setCompleting}
        />
        {complete}
      </>
    );
  }

  return (
    <div className="space-y-4">
      {header}
      {!checkin ? (
        <Standup kind={kind === "evening" ? "evening" : "morning"} />
      ) : (
        <Card className="space-y-4">
          {editing ? (
            <Readings checkin={checkin} onClose={() => setEditing(false)} />
          ) : (
            <div className="flex items-center justify-between gap-3 text-sm">
              <p className="text-fg-muted">
                {[
                  `${kind === "morning" ? "Morning" : "Evening"} check-in`,
                  ...readingKeys.flatMap((k) => (checkin[k] == null ? [] : [`${k} ${checkin[k]}`])),
                ].join(" · ")}
              </p>
              <Button variant="ghost" className="px-2 py-1" onClick={() => setEditing(true)}>
                Edit
              </Button>
            </div>
          )}

          <Plan snap={s} coach={false} onComplete={setCompleting} />

          <Collapsible label="Your answers">
            <Chat checkinId={checkin.id} prompt="No answers saved." readOnly />
          </Collapsible>
          {health.llm.backend !== "none" && (
            <p className="text-xs text-fg-muted">Coach offline: {health.llm.detail}</p>
          )}
        </Card>
      )}
      <MoodHistory />
      {complete}
    </div>
  );
}

const greetings: Record<"morning" | "evening", string> = {
  morning: "Morning. How are you doing?",
  evening: "Evening. How did today go?",
};

// CoachStandup is the check-in as one conversation filling the screen. The
// greeting shows right away, and the check-in is created (with the greeting
// as its first message) when the person first answers; from there the coach
// leads. Readings sit in a slim strip above, and the plan beside or above.
function CoachStandup({
  kind,
  snap,
  checkin,
  header,
  onComplete,
}: {
  kind: "morning" | "evening";
  snap: Snapshot;
  checkin: Checkin | null;
  header: ReactNode;
  onComplete: (s: Step) => void;
}) {
  const [draft, setDraft] = useState<Values>({ mood: null, energy: null, anxiety: null });
  const [stripOpen, setStripOpen] = useState(checkin === null);
  const greeting = greetings[kind];

  async function start(first: string) {
    setStripOpen(false);
    const c = await api.createCheckin({
      kind,
      ...draft,
      note: first,
      intro: [{ role: "assistant", text: greeting }],
    });
    return c.id;
  }

  const n = snap.today.length;
  return (
    <ChatPage
      top={
        <>
          {header}
          <ReadingsStrip
            checkin={checkin}
            draft={draft}
            setDraft={setDraft}
            open={stripOpen}
            setOpen={setStripOpen}
          />
        </>
      }
      title="Today's plan"
      label={`Today's plan${n > 0 ? ` (${n})` : ""}`}
      side={<PlanPanel snap={snap} onComplete={onComplete} />}
    >
      <Chat
        fill
        checkinId={checkin?.id ?? null}
        start={start}
        greeting={greeting}
        suggestions={questions[kind][0].replies}
        placeholder="Talk to your coach…"
      />
    </ChatPage>
  );
}

type Values = Pick<Checkin, "mood" | "energy" | "anxiety">;
const readingKeys = ["mood", "energy", "anxiety"] as const;

// ReadingsStrip is the optional mood/energy/anxiety sliders, folded into one
// line once set. Before the check-in exists they're a draft saved with it;
// after, changes save on their own. The coach can fill them in too.
function ReadingsStrip({
  checkin,
  draft,
  setDraft,
  open,
  setOpen,
}: {
  checkin: Checkin | null;
  draft: Values;
  setDraft: (v: Values) => void;
  open: boolean;
  setOpen: (v: boolean) => void;
}) {
  const qc = useQueryClient();
  // Edits to a saved check-in show at once and save after a pause.
  const [pending, setPending] = useState<Values | null>(null);
  const timer = useRef<ReturnType<typeof setTimeout>>(undefined);
  const save = useMutation({
    mutationFn: ({ id, v }: { id: number; v: Values }) => api.updateCheckin(id, v),
    onSuccess: async () => {
      await Promise.all([
        qc.invalidateQueries({ queryKey: ["today"] }),
        qc.invalidateQueries({ queryKey: ["mood"] }),
      ]);
      setPending(null);
    },
  });
  useEffect(() => () => clearTimeout(timer.current), []);

  const values: Values = pending ?? checkin ?? draft;
  function set(k: (typeof readingKeys)[number], v: number) {
    const next = { mood: values.mood, energy: values.energy, anxiety: values.anxiety, [k]: v };
    if (!checkin) return setDraft(next);
    const id = checkin.id;
    setPending(next);
    clearTimeout(timer.current);
    timer.current = setTimeout(() => save.mutate({ id, v: next }), 400);
  }

  const summary = readingKeys.some((k) => values[k] !== null)
    ? readingKeys.map((k) => `${cap(k)} ${values[k] ?? "–"}`).join(" · ")
    : "How are you feeling?";

  return (
    <div className="shrink-0 rounded-xl bg-surface-2 px-3 py-2">
      <button
        type="button"
        className="flex w-full items-center justify-between gap-2 text-left text-sm text-fg-muted hover:text-fg"
        onClick={() => setOpen(!open)}
        aria-expanded={open}
      >
        <span>{summary}</span>
        <span aria-hidden>{open ? "▾" : "▸"}</span>
      </button>
      {open && (
        <div className="mt-2 grid gap-3 pb-1 sm:grid-cols-3">
          <Slider
            label="Mood"
            value={values.mood}
            onChange={(v) => set("mood", v)}
            low="very low"
            high="great"
          />
          <Slider
            label="Energy"
            value={values.energy}
            onChange={(v) => set("energy", v)}
            low="empty"
            high="full"
          />
          <Slider
            label="Anxiety"
            value={values.anxiety}
            onChange={(v) => set("anxiety", v)}
            low="calm"
            high="intense"
          />
        </div>
      )}
      <ErrorText error={save.error} />
    </div>
  );
}

function cap(s: string) {
  return s[0].toUpperCase() + s.slice(1);
}

// PlanPanel is today's plan and recent mood, beside the chat.
function PlanPanel({ snap, onComplete }: { snap: Snapshot; onComplete: (s: Step) => void }) {
  return (
    <div className="space-y-4">
      {snap.today.length === 0 && snap.done_today.length === 0 ? (
        <p className="text-sm text-fg-muted">
          Nothing planned yet. Your coach can help pick something small.
        </p>
      ) : (
        <Plan snap={snap} coach onComplete={onComplete} bare />
      )}
      <MoodHistory />
    </div>
  );
}

// Standup is the check-in without a coach: its questions as chat bubbles,
// one at a time, answered through a composer. Answers are saved with the
// check-in as its first messages, and the page moves on to the plan.
function Standup({ kind }: { kind: "morning" | "evening" }) {
  const qc = useQueryClient();
  const qs = questions[kind];
  const [answers, setAnswers] = useState<string[]>([]);
  const [mood, setMood] = useState<number | null>(null);
  const [energy, setEnergy] = useState<number | null>(null);
  const [anxiety, setAnxiety] = useState<number | null>(null);
  const bottom = useRef<HTMLDivElement>(null);

  const save = useMutation({
    mutationFn: async (all: string[]) => {
      const intro = qs.flatMap((q, i) => [
        { role: "assistant" as const, text: q.text },
        { role: "user" as const, text: all[i] },
      ]);
      const c = await api.createCheckin({ kind, mood, energy, anxiety, note: all[0], intro });
      // Seed the transcript so the chat picks up without a blank frame.
      const at = new Date().toISOString();
      qc.setQueryData<CheckinDetail>(["checkin", c.id], {
        ...c,
        messages: intro.map((m, i) => ({
          ...m,
          id: -1 - i,
          checkin_id: c.id,
          seq: i + 1,
          created_at: at,
        })),
        actions: [],
      });
      await Promise.all([
        qc.invalidateQueries({ queryKey: ["today"] }),
        qc.invalidateQueries({ queryKey: ["mood"] }),
      ]);
    },
  });

  const step = answers.length;
  const q = qs[Math.min(step, qs.length - 1)];
  const done = step === qs.length;

  useEffect(() => {
    if (step > 0) bottom.current?.scrollIntoView({ block: "nearest", behavior: "smooth" });
  }, [step]);

  function answer(text: string) {
    const next = [...answers, text];
    setAnswers(next);
    if (next.length === qs.length) save.mutate(next);
  }

  const readings = (["mood", "energy", "anxiety"] as const)
    .map((k) => [k, { mood, energy, anxiety }[k]] as const)
    .filter(([, v]) => v !== null)
    .map(([k, v]) => `${k} ${v}`)
    .join(" · ");

  return (
    <Card className="space-y-3">
      <p className="text-xs text-fg-muted">
        {kind === "morning" ? "Morning" : "Evening"} check-in · {qs.length} quick questions
      </p>
      <div className="space-y-3" aria-live="polite">
        {qs.slice(0, step + 1).map((qq, i) => (
          <Fragment key={qq.text}>
            <Bubble from="assistant">
              <p className={i === step ? "text-base font-medium" : undefined}>{qq.text}</p>
            </Bubble>
            {i === 0 && step === 0 && (
              <div className="space-y-4 rounded-xl bg-surface-2 p-3">
                <Slider label="Mood" value={mood} onChange={setMood} low="very low" high="great" />
                <Slider
                  label="Energy"
                  value={energy}
                  onChange={setEnergy}
                  low="empty"
                  high="full"
                />
                <Slider
                  label="Anxiety"
                  value={anxiety}
                  onChange={setAnxiety}
                  low="calm"
                  high="intense"
                />
              </div>
            )}
            {i < step && (
              <Bubble from="user">
                <p>{answers[i]}</p>
                {i === 0 && readings && <p className="text-xs text-fg-muted">{readings}</p>}
              </Bubble>
            )}
          </Fragment>
        ))}
        {save.isPending && <p className="text-sm text-fg-muted">saving…</p>}
        <div ref={bottom} />
      </div>
      <ErrorText error={save.error} />
      {save.isError ? (
        <Button onClick={() => save.mutate(answers)}>Try again</Button>
      ) : (
        !done && (
          <Composer
            key={step}
            onSend={answer}
            placeholder={q.hint}
            quickReplies={step === 0 ? q.replies : [...q.replies, SKIP]}
          />
        )
      )}
    </Card>
  );
}

// Readings edits the check-in's sliders after the fact.
function Readings({ checkin, onClose }: { checkin: Checkin; onClose: () => void }) {
  const qc = useQueryClient();
  const [mood, setMood] = useState(checkin.mood);
  const [energy, setEnergy] = useState(checkin.energy);
  const [anxiety, setAnxiety] = useState(checkin.anxiety);
  const save = useMutation({
    mutationFn: () => api.updateCheckin(checkin.id, { mood, energy, anxiety }),
    onSuccess: async () => {
      await Promise.all([
        qc.invalidateQueries({ queryKey: ["today"] }),
        qc.invalidateQueries({ queryKey: ["mood"] }),
      ]);
      onClose();
    },
  });
  return (
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
      <ErrorText error={save.error} />
      <div className="flex gap-2">
        <Button variant="ghost" onClick={onClose}>
          Cancel
        </Button>
        <Button type="submit" disabled={save.isPending} className="flex-1">
          Save
        </Button>
      </div>
    </form>
  );
}

// Plan is today's steps, with anything done today checked off. With a coach,
// the conversation handles adding and choosing steps, so the manual controls
// only show without one.
function Plan({
  snap,
  coach,
  onComplete,
  bare = false,
}: {
  snap: Snapshot;
  coach: boolean;
  onComplete: (s: Step) => void;
  bare?: boolean;
}) {
  const qc = useQueryClient();
  const move = useMutation({
    mutationFn: ({ id, lane }: { id: number; lane: Lane }) => api.moveStep(id, lane),
    onSuccess: () => qc.invalidateQueries(),
  });
  const empty = snap.today.length === 0 && snap.done_today.length === 0;

  return (
    <div>
      {!bare && <h2 className="mb-2 text-sm font-semibold">Today's plan</h2>}
      <ul className="space-y-2">
        {empty && (
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
        {snap.done_today.map((st) => (
          <li key={st.id} className="flex items-center gap-3 px-2 text-sm text-fg-muted">
            <span className="w-6 text-center text-calm" aria-hidden>
              ✓
            </span>
            <span className="flex-1 line-through">{st.title}</span>
            {(st.mastery !== null || st.pleasure !== null) && (
              <span className="text-xs">
                M {st.mastery ?? "–"} · P {st.pleasure ?? "–"}
              </span>
            )}
          </li>
        ))}
      </ul>
      <ErrorText error={move.error} />
      {!coach && <ManualPlanning snap={snap} onMove={(id, lane) => move.mutate({ id, lane })} />}
    </div>
  );
}

function ManualPlanning({
  snap,
  onMove,
}: {
  snap: Snapshot;
  onMove: (id: number, lane: Lane) => void;
}) {
  const qc = useQueryClient();
  const [title, setTitle] = useState("");
  const [cost, setCost] = useState(1);
  const [showWeek, setShowWeek] = useState(false);
  const add = useMutation({
    mutationFn: () => api.createStep("today", { title: title.trim(), energy_cost: cost }),
    onSuccess: () => {
      setTitle("");
      qc.invalidateQueries();
    },
  });
  return (
    <>
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
      <ErrorText error={add.error} />

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
                      onClick={() => onMove(st.id, "today")}
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
    </>
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
          <span className="mr-1">Still here from before. That's okay.</span>
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

// MoodHistory stays tucked away until asked for, so the page starts with one
// thing to do.
function MoodHistory() {
  const [open, setOpen] = useState(false);
  const mood = useQuery({ queryKey: ["mood", 14], queryFn: () => api.mood(14), enabled: open });
  return (
    <div>
      <button
        type="button"
        className="text-sm text-fg-muted hover:text-fg"
        onClick={() => setOpen((v) => !v)}
        aria-expanded={open}
      >
        {open ? "▾" : "▸"} Your last two weeks
      </button>
      {open && (
        <Card className="mt-2">
          <MoodChart days={mood.data ?? []} title="Last two weeks" />
        </Card>
      )}
    </div>
  );
}
