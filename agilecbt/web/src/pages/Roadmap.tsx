import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import {
  api,
  type Goal,
  type GoalStatus,
  type Health,
  type Lane,
  type Step,
  type Value,
} from "@/api/client";
import Chat from "@/components/Chat";
import ChatPage from "@/components/ChatPage";
import { Button, Card, EnergyDots, ErrorText, inputClass } from "@/components/ui";

const statusLabel: Record<GoalStatus, string> = {
  active: "Active",
  resting: "Resting",
  done: "Done",
};

// Roadmap is Values → Goals → Steps: every step traces back to why it matters.
// With a coach, the page is a full-height conversation and the coach does the
// data entry; the roadmap itself is a read-only outline beside it. Without
// one, the outline has the forms.
export default function Roadmap({ health }: { health: Health }) {
  const values = useQuery({ queryKey: ["values"], queryFn: api.values });
  const goals = useQuery({ queryKey: ["goals"], queryFn: api.goals });
  const steps = useQuery({
    queryKey: ["steps", "roadmap"],
    queryFn: () => api.steps(["someday", "week", "today", "done"]),
  });
  if (!values.data || !goals.data) return <ErrorText error={values.error ?? goals.error} />;
  const coach = health.llm.available;
  const outline = (
    <Outline values={values.data} goals={goals.data} steps={steps.data ?? []} editable={!coach} />
  );

  const title = (
    <div className="shrink-0">
      <h1 className="text-xl font-semibold">Roadmap</h1>
      <p className="text-sm text-fg-muted">
        What matters to you, and the goals that grow from it. Resting goals are fine.
      </p>
    </div>
  );
  if (!coach) {
    return (
      <div className="space-y-4">
        {title}
        {outline}
      </div>
    );
  }
  return (
    <ChatPage
      top={title}
      title="Your roadmap"
      label={`Your roadmap (${count(values.data.length, "value")}, ${count(goals.data.length, "goal")})`}
      side={outline}
    >
      <RoadmapChat empty={values.data.length === 0 && goals.data.length === 0} />
    </ChatPage>
  );
}

function count(n: number, noun: string) {
  return `${n} ${noun}${n === 1 ? "" : "s"}`;
}

// RoadmapChat is today's roadmap conversation. The coach's opening line shows
// at once and is saved when the conversation starts, on the first message.
function RoadmapChat({ empty }: { empty: boolean }) {
  const conv = useQuery({
    queryKey: ["conversation", "roadmap"],
    queryFn: api.roadmapConversation,
  });
  if (conv.isPending) return null;
  const greeting = empty
    ? "What matters to you? Tell me in your own words, and I'll turn it into values, goals, and small steps."
    : "What would you like to change on your roadmap?";
  return (
    <Chat
      fill
      checkinId={conv.data?.id ?? null}
      start={async () =>
        (
          await api.createCheckin({
            kind: "adhoc",
            topic: "roadmap",
            intro: [{ role: "assistant", text: greeting }],
          })
        ).id
      }
      greeting={greeting}
      suggestions={
        empty
          ? ["Help me figure out what matters to me", "I already have a goal in mind"]
          : [
              "Break a goal into small steps",
              "Something doesn't fit anymore",
              "I want to add something new",
            ]
      }
      placeholder="Talk about your roadmap…"
    />
  );
}

function Outline({
  values,
  goals,
  steps,
  editable,
}: {
  values: Value[];
  goals: Goal[];
  steps: Step[];
  editable: boolean;
}) {
  const loose = goals.filter((g) => g.value_id === null);
  return (
    <div className="space-y-4">
      {values.length === 0 && editable && (
        <Card>
          <p className="text-sm">
            Start with a value: a direction you care about, like <em>Health</em>,{" "}
            <em>Connection</em>, or <em>Creativity</em>.
          </p>
        </Card>
      )}
      {values.length === 0 && loose.length === 0 && !editable && (
        <p className="text-sm text-fg-muted">Nothing here yet.</p>
      )}
      {values.map((v) => (
        <ValueCard
          key={v.id}
          value={v}
          goals={goals.filter((g) => g.value_id === v.id)}
          steps={steps}
          editable={editable}
        />
      ))}
      {loose.length > 0 && (
        <ValueCard value={null} goals={loose} steps={steps} editable={editable} />
      )}
      {editable && <AddValue />}
    </div>
  );
}

function ValueCard({
  value,
  goals,
  steps,
  editable,
}: {
  value: Value | null;
  goals: Goal[];
  steps: Step[];
  editable: boolean;
}) {
  const order: GoalStatus[] = ["active", "resting", "done"];
  const sorted = [...goals].sort((a, b) => order.indexOf(a.status) - order.indexOf(b.status));
  return (
    <Card>
      <div className="mb-3 flex items-start gap-2">
        <span
          className="mt-1.5 h-2.5 w-2.5 shrink-0 rounded-full"
          style={{ background: value?.color || "var(--color-accent-500)" }}
        />
        <div>
          <h2 className="font-semibold">{value?.name ?? "Other goals"}</h2>
          {value?.description && <p className="text-sm text-fg-muted">{value.description}</p>}
        </div>
      </div>
      {sorted.length === 0 && !editable && <p className="text-sm text-fg-muted">No goals yet.</p>}
      <ul className="space-y-3">
        {sorted.map((g) => (
          <GoalItem
            key={g.id}
            goal={g}
            steps={steps.filter((s) => s.goal_id === g.id)}
            editable={editable}
          />
        ))}
      </ul>
      {value && editable && <AddGoal valueId={value.id} />}
    </Card>
  );
}

function GoalItem({ goal, steps, editable }: { goal: Goal; steps: Step[]; editable: boolean }) {
  const qc = useQueryClient();
  const [open, setOpen] = useState(goal.status === "active");
  const update = useMutation({
    mutationFn: (status: GoalStatus) => api.updateGoal(goal.id, { status }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["goals"] }),
  });
  const open_ = steps.filter((s) => s.lane !== "done");
  const done = steps.length - open_.length;
  return (
    <li
      className={`rounded-xl border border-border p-3 ${goal.status !== "active" ? "opacity-70" : ""}`}
    >
      <div className="flex items-start gap-2">
        <button
          type="button"
          className="flex-1 text-left"
          onClick={() => setOpen((o) => !o)}
          aria-expanded={open}
        >
          <p className="text-sm font-medium">{goal.title}</p>
          {goal.why && <p className="text-xs text-fg-muted">Why: {goal.why}</p>}
          <p className="mt-1 text-xs text-fg-muted">
            {open_.length} open · {done} done{goal.horizon ? ` · ${goal.horizon}` : ""}
          </p>
        </button>
        {editable ? (
          <select
            aria-label={`Status of ${goal.title}`}
            value={goal.status}
            onChange={(e) => update.mutate(e.target.value as GoalStatus)}
            className="rounded-lg border border-border bg-surface px-1 py-0.5 text-xs"
          >
            {(Object.keys(statusLabel) as GoalStatus[]).map((s) => (
              <option key={s} value={s}>
                {statusLabel[s]}
              </option>
            ))}
          </select>
        ) : (
          goal.status !== "active" && (
            <span className="text-xs text-fg-muted">{statusLabel[goal.status]}</span>
          )
        )}
      </div>
      {open && (open_.length > 0 || editable) && (
        <div className="mt-2 border-t border-border pt-2">
          <ul className="space-y-1">
            {open_.map((s) => (
              <li key={s.id} className="flex items-center gap-2 text-sm">
                <EnergyDots cost={s.energy_cost} />
                <span className="flex-1">{s.title}</span>
                <span className="text-xs text-fg-muted">{laneName(s.lane)}</span>
              </li>
            ))}
          </ul>
          {editable && <AddStep goalId={goal.id} />}
        </div>
      )}
    </li>
  );
}

function laneName(l: Lane) {
  return { someday: "someday", week: "this week", today: "today", done: "done", let_go: "let go" }[
    l
  ];
}

function AddValue() {
  const qc = useQueryClient();
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const add = useMutation({
    mutationFn: () => api.createValue({ name: name.trim(), description: description.trim() }),
    onSuccess: () => {
      setName("");
      setDescription("");
      qc.invalidateQueries({ queryKey: ["values"] });
    },
  });
  return (
    <form
      className="space-y-2 rounded-2xl border border-dashed border-border p-4"
      onSubmit={(e) => {
        e.preventDefault();
        if (name.trim()) add.mutate();
      }}
    >
      <p className="text-sm font-medium">Add a value</p>
      <input
        value={name}
        onChange={(e) => setName(e.target.value)}
        placeholder="e.g. Health"
        aria-label="Value name"
        className={inputClass}
      />
      <input
        value={description}
        onChange={(e) => setDescription(e.target.value)}
        placeholder="What it means to you (optional)"
        aria-label="Value description"
        className={inputClass}
      />
      <ErrorText error={add.error} />
      <Button type="submit" disabled={!name.trim() || add.isPending}>
        Add value
      </Button>
    </form>
  );
}

function AddGoal({ valueId }: { valueId: number }) {
  const qc = useQueryClient();
  const [open, setOpen] = useState(false);
  const [title, setTitle] = useState("");
  const [why, setWhy] = useState("");
  const add = useMutation({
    mutationFn: () => api.createGoal({ title: title.trim(), why: why.trim(), value_id: valueId }),
    onSuccess: () => {
      setTitle("");
      setWhy("");
      setOpen(false);
      qc.invalidateQueries({ queryKey: ["goals"] });
    },
  });
  if (!open) {
    return (
      <Button variant="ghost" className="mt-2" onClick={() => setOpen(true)}>
        + Goal
      </Button>
    );
  }
  return (
    <form
      className="mt-3 space-y-2"
      onSubmit={(e) => {
        e.preventDefault();
        if (title.trim()) add.mutate();
      }}
    >
      <input
        value={title}
        onChange={(e) => setTitle(e.target.value)}
        placeholder="Goal, e.g. Sleep better"
        aria-label="Goal title"
        className={inputClass}
      />
      <input
        value={why}
        onChange={(e) => setWhy(e.target.value)}
        placeholder="Why does it matter? (optional)"
        aria-label="Why it matters"
        className={inputClass}
      />
      <ErrorText error={add.error} />
      <div className="flex gap-2">
        <Button type="submit" disabled={!title.trim() || add.isPending}>
          Add goal
        </Button>
        <Button variant="ghost" onClick={() => setOpen(false)}>
          Cancel
        </Button>
      </div>
    </form>
  );
}

function AddStep({ goalId }: { goalId: number }) {
  const qc = useQueryClient();
  const [title, setTitle] = useState("");
  const add = useMutation({
    mutationFn: () => api.createStep("someday", { title: title.trim(), goal_id: goalId }),
    onSuccess: () => {
      setTitle("");
      qc.invalidateQueries({ queryKey: ["steps"] });
    },
  });
  return (
    <form
      className="mt-2 flex gap-2"
      onSubmit={(e) => {
        e.preventDefault();
        if (title.trim()) add.mutate();
      }}
    >
      <input
        value={title}
        onChange={(e) => setTitle(e.target.value)}
        placeholder="Add a step (goes to Someday)…"
        aria-label="New step for this goal"
        className={`${inputClass} py-1.5`}
      />
      <Button type="submit" variant="soft" disabled={!title.trim() || add.isPending}>
        Add
      </Button>
    </form>
  );
}
