import {
  closestCorners,
  DndContext,
  type DragEndEvent,
  type DragOverEvent,
  DragOverlay,
  type DragStartEvent,
  KeyboardSensor,
  PointerSensor,
  TouchSensor,
  useDroppable,
  useSensor,
  useSensors,
} from "@dnd-kit/core";
import {
  arrayMove,
  SortableContext,
  sortableKeyboardCoordinates,
  useSortable,
  verticalListSortingStrategy,
} from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useMemo, useState } from "react";
import { api, type Goal, type Lane, type Step } from "@/api/client";
import CompleteDialog from "@/components/CompleteDialog";
import { Button, EnergyDots, ErrorText, inputClass } from "@/components/ui";

const lanes: { id: Lane; label: string; hint: string }[] = [
  { id: "someday", label: "Someday", hint: "Ideas, no pressure" },
  { id: "week", label: "This week", hint: "What this sprint is about" },
  { id: "today", label: "Today", hint: "Just a few" },
  { id: "done", label: "Done", hint: "Every one counts" },
];

type Columns = Record<Lane, number[]>;

function group(steps: Step[]): Columns {
  const cols: Columns = { someday: [], week: [], today: [], done: [], let_go: [] };
  for (const s of [...steps].sort((a, b) => a.sort_order - b.sort_order || a.id - b.id)) {
    cols[s.lane].push(s.id);
  }
  return cols;
}

export default function Board() {
  const qc = useQueryClient();
  const steps = useQuery({
    queryKey: ["steps", "board"],
    queryFn: () => api.steps(["someday", "week", "today", "done"]),
  });
  const goals = useQuery({ queryKey: ["goals"], queryFn: api.goals });
  const [cols, setCols] = useState<Columns | null>(null);
  const [active, setActive] = useState<number | null>(null);
  const [completing, setCompleting] = useState<Step | null>(null);

  const byId = useMemo(() => new Map((steps.data ?? []).map((s) => [s.id, s])), [steps.data]);
  const goalsById = useMemo(() => new Map((goals.data ?? []).map((g) => [g.id, g])), [goals.data]);

  // Server data is the source of truth; local columns only exist mid-drag.
  useEffect(() => {
    if (steps.data) setCols(group(steps.data));
  }, [steps.data]);

  const move = useMutation({
    mutationFn: ({ id, lane, index }: { id: number; lane: Lane; index?: number }) =>
      api.moveStep(id, lane, index),
    onSettled: () => qc.invalidateQueries(),
  });

  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 6 } }),
    useSensor(TouchSensor, { activationConstraint: { delay: 200, tolerance: 6 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  );

  if (!cols) return <ErrorText error={steps.error} />;

  const laneOf = (id: number | string): Lane | undefined => {
    if (typeof id === "string") return id as Lane;
    return (Object.keys(cols) as Lane[]).find((l) => cols[l].includes(id));
  };

  function onDragStart(e: DragStartEvent) {
    setActive(e.active.id as number);
  }

  // Moving across lanes mid-drag lets the target lane open a gap.
  function onDragOver(e: DragOverEvent) {
    if (!e.over || !cols) return;
    const from = laneOf(e.active.id as number);
    const to = laneOf(e.over.id);
    if (!from || !to || from === to) return;
    const id = e.active.id as number;
    const overIdx = typeof e.over.id === "number" ? cols[to].indexOf(e.over.id) : cols[to].length;
    setCols({
      ...cols,
      [from]: cols[from].filter((x) => x !== id),
      [to]: [...cols[to].slice(0, overIdx), id, ...cols[to].slice(overIdx)],
    });
  }

  function onDragEnd(e: DragEndEvent) {
    setActive(null);
    if (!e.over || !cols) {
      if (steps.data) setCols(group(steps.data));
      return;
    }
    const id = e.active.id as number;
    const lane = laneOf(id) as Lane;
    const oldIdx = cols[lane].indexOf(id);
    const overIdx = typeof e.over.id === "number" ? cols[lane].indexOf(e.over.id) : -1;
    const ids = overIdx >= 0 ? arrayMove(cols[lane], oldIdx, overIdx) : cols[lane];
    const index = ids.indexOf(id);
    setCols({ ...cols, [lane]: ids });
    const step = byId.get(id);
    if (!step) return;
    if (lane === "done" && step.lane !== "done") {
      // Finishing via drag still asks how it felt.
      setCompleting(step);
      return;
    }
    const serverIdx = steps.data ? group(steps.data)[lane].indexOf(id) : -1;
    if (lane !== step.lane || index !== serverIdx) {
      move.mutate({ id, lane, index });
    }
  }

  const activeStep = active === null ? undefined : byId.get(active);

  return (
    <div className="space-y-3">
      <div className="flex items-baseline justify-between">
        <h1 className="text-xl font-semibold">Board</h1>
        <p className="text-xs text-fg-muted">Drag cards between lanes, or use Move.</p>
      </div>
      <ErrorText error={move.error} />
      <DndContext
        sensors={sensors}
        collisionDetection={closestCorners}
        onDragStart={onDragStart}
        onDragOver={onDragOver}
        onDragEnd={onDragEnd}
        onDragCancel={() => {
          setActive(null);
          if (steps.data) setCols(group(steps.data));
        }}
      >
        <div className="-mx-4 flex snap-x gap-3 overflow-x-auto px-4 pb-2 md:mx-0 md:grid md:grid-cols-4 md:px-0">
          {lanes.map((l) => (
            <LaneColumn
              key={l.id}
              lane={l}
              ids={cols[l.id]}
              byId={byId}
              goalsById={goalsById}
              onMove={(id, lane) =>
                lane === "done" ? setCompleting(byId.get(id) ?? null) : move.mutate({ id, lane })
              }
            />
          ))}
        </div>
        <DragOverlay>
          {activeStep && (
            <CardBody step={activeStep} goal={goalsById.get(activeStep.goal_id ?? -1)} lifted />
          )}
        </DragOverlay>
      </DndContext>
      <CompleteDialog
        step={completing}
        onClose={() => {
          setCompleting(null);
          if (steps.data) setCols(group(steps.data));
          qc.invalidateQueries();
        }}
      />
    </div>
  );
}

function LaneColumn({
  lane,
  ids,
  byId,
  goalsById,
  onMove,
}: {
  lane: (typeof lanes)[number];
  ids: number[];
  byId: Map<number, Step>;
  goalsById: Map<number, Goal>;
  onMove: (id: number, lane: Lane) => void;
}) {
  const { setNodeRef, isOver } = useDroppable({ id: lane.id });
  return (
    <section
      aria-label={lane.label}
      className={`flex w-[78vw] max-w-xs shrink-0 snap-start flex-col rounded-2xl border bg-surface p-3 md:w-auto md:max-w-none ${isOver ? "border-accent-500" : "border-border"}`}
    >
      <header className="mb-2">
        <h2 className="text-sm font-semibold">
          {lane.label} <span className="font-normal text-fg-muted">{ids.length}</span>
        </h2>
        <p className="text-xs text-fg-muted">{lane.hint}</p>
      </header>
      <SortableContext id={lane.id} items={ids} strategy={verticalListSortingStrategy}>
        <ul ref={setNodeRef} className="flex min-h-16 flex-1 flex-col gap-2">
          {ids.map((id) => {
            const s = byId.get(id);
            return s ? (
              <SortableCard
                key={id}
                step={s}
                goal={goalsById.get(s.goal_id ?? -1)}
                onMove={onMove}
              />
            ) : null;
          })}
        </ul>
      </SortableContext>
      {lane.id !== "done" && <AddCard lane={lane.id} />}
    </section>
  );
}

function SortableCard({
  step,
  goal,
  onMove,
}: {
  step: Step;
  goal?: Goal;
  onMove: (id: number, lane: Lane) => void;
}) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({
    id: step.id,
  });
  return (
    <li
      ref={setNodeRef}
      style={{ transform: CSS.Transform.toString(transform), transition }}
      className={isDragging ? "opacity-40" : ""}
      data-testid={`step-${step.id}`}
    >
      <CardBody step={step} goal={goal} handle={{ ...attributes, ...listeners }} onMove={onMove} />
    </li>
  );
}

function CardBody({
  step,
  goal,
  handle,
  onMove,
  lifted,
}: {
  step: Step;
  goal?: Goal;
  handle?: object;
  onMove?: (id: number, lane: Lane) => void;
  lifted?: boolean;
}) {
  return (
    <div
      className={`rounded-xl border border-border bg-bg p-2.5 text-sm ${lifted ? "rotate-1 shadow-lg" : ""}`}
    >
      <div className="flex items-start gap-2">
        <div {...handle} className="flex-1 cursor-grab touch-manipulation active:cursor-grabbing">
          <p className={step.lane === "done" ? "text-fg-muted" : ""}>{step.title}</p>
          <div className="mt-1 flex items-center gap-2 text-xs text-fg-muted">
            <EnergyDots cost={step.energy_cost} />
            {goal && <span className="truncate">{goal.title}</span>}
            {step.lane === "done" && (step.mastery !== null || step.pleasure !== null) && (
              <span>
                M {step.mastery ?? "–"} · P {step.pleasure ?? "–"}
              </span>
            )}
          </div>
        </div>
        {onMove && (
          <select
            aria-label={`Move ${step.title}`}
            value=""
            onChange={(e) => onMove(step.id, e.target.value as Lane)}
            className="w-14 rounded-lg border border-border bg-surface px-1 py-0.5 text-xs text-fg-muted"
          >
            <option value="" disabled>
              Move
            </option>
            {lanes
              .filter((l) => l.id !== step.lane)
              .map((l) => (
                <option key={l.id} value={l.id}>
                  {l.label}
                </option>
              ))}
            <option value="let_go">Let go</option>
          </select>
        )}
      </div>
    </div>
  );
}

function AddCard({ lane }: { lane: Lane }) {
  const qc = useQueryClient();
  const [open, setOpen] = useState(false);
  const [title, setTitle] = useState("");
  const add = useMutation({
    mutationFn: () => api.createStep(lane, { title: title.trim() }),
    onSuccess: () => {
      setTitle("");
      qc.invalidateQueries();
    },
  });
  if (!open) {
    return (
      <Button variant="ghost" className="mt-2 w-full text-left" onClick={() => setOpen(true)}>
        + Add
      </Button>
    );
  }
  return (
    <form
      className="mt-2 space-y-2"
      onSubmit={(e) => {
        e.preventDefault();
        if (title.trim()) add.mutate();
      }}
    >
      <input
        value={title}
        onChange={(e) => setTitle(e.target.value)}
        onKeyDown={(e) => e.key === "Escape" && setOpen(false)}
        placeholder="A small, concrete step…"
        aria-label={`New step in ${lane}`}
        className={inputClass}
        // biome-ignore lint/a11y/noAutofocus: opened on request
        autoFocus
      />
      <div className="flex gap-2">
        <Button type="submit" disabled={!title.trim() || add.isPending} className="flex-1">
          Add
        </Button>
        <Button variant="ghost" onClick={() => setOpen(false)}>
          Cancel
        </Button>
      </div>
    </form>
  );
}
