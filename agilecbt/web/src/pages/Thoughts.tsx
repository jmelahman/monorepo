import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { api, type Emotion, type ThoughtRecord } from "@/api/client";
import { Button, Card, ErrorText, inputClass } from "@/components/ui";

// Keys match what the curator's create_thought_record tool uses.
export const distortions: { key: string; label: string; hint: string }[] = [
  {
    key: "all_or_nothing",
    label: "All-or-nothing",
    hint: "Seeing it as total success or total failure.",
  },
  {
    key: "catastrophizing",
    label: "Catastrophizing",
    hint: "Jumping to the worst possible outcome.",
  },
  {
    key: "mind_reading",
    label: "Mind reading",
    hint: "Assuming you know what others think of you.",
  },
  { key: "fortune_telling", label: "Fortune telling", hint: "Predicting things will go badly." },
  {
    key: "should_statements",
    label: "Should statements",
    hint: "Rigid rules: I should, I must, I have to.",
  },
  { key: "labeling", label: "Labeling", hint: "“I'm a failure” instead of “I made a mistake.”" },
  {
    key: "personalization",
    label: "Personalization",
    hint: "Blaming yourself for things outside your control.",
  },
  {
    key: "overgeneralization",
    label: "Overgeneralization",
    hint: "One event becomes “always” or “never.”",
  },
  { key: "mental_filter", label: "Mental filter", hint: "Only noticing the negatives." },
  { key: "discounting_positives", label: "Discounting positives", hint: "“That doesn't count.”" },
  {
    key: "emotional_reasoning",
    label: "Emotional reasoning",
    hint: "“I feel it, so it must be true.”",
  },
];
const distortionLabel = new Map(distortions.map((d) => [d.key, d.label]));

const commonEmotions = [
  "anxious",
  "sad",
  "angry",
  "ashamed",
  "guilty",
  "hopeless",
  "lonely",
  "overwhelmed",
];

const stepsMeta = [
  { title: "Situation", prompt: "What happened? Just the facts — where, when, who." },
  { title: "Feelings", prompt: "What did you feel, and how strongly (0–100)?" },
  { title: "Thought", prompt: "What went through your mind? Any thinking traps?" },
  { title: "Evidence", prompt: "What supports the thought? What doesn't?" },
  { title: "Balance", prompt: "A more balanced way to see it — then re-rate how you feel." },
];

export default function Thoughts() {
  const thoughts = useQuery({ queryKey: ["thoughts"], queryFn: api.thoughts });
  const [writing, setWriting] = useState(false);
  return (
    <div className="space-y-4">
      <div className="flex items-start justify-between gap-2">
        <div>
          <h1 className="text-xl font-semibold">Thoughts</h1>
          <p className="text-sm text-fg-muted">
            Slow a sticky thought down and look at it together.
          </p>
        </div>
        {!writing && <Button onClick={() => setWriting(true)}>New thought record</Button>}
      </div>
      {writing && <ThoughtForm onDone={() => setWriting(false)} />}
      <ErrorText error={thoughts.error} />
      {thoughts.data?.length === 0 && !writing && (
        <Card>
          <p className="text-sm text-fg-muted">
            No thought records yet. When a thought keeps pulling you down, write it out here.
          </p>
        </Card>
      )}
      <ul className="space-y-3">
        {thoughts.data?.map((t) => (
          <ThoughtItem key={t.id} t={t} />
        ))}
      </ul>
    </div>
  );
}

function ThoughtForm({ onDone }: { onDone: () => void }) {
  const qc = useQueryClient();
  const [step, setStep] = useState(0);
  const [situation, setSituation] = useState("");
  const [emotions, setEmotions] = useState<Emotion[]>([]);
  const [automatic, setAutomatic] = useState("");
  const [picked, setPicked] = useState<string[]>([]);
  const [evidenceFor, setEvidenceFor] = useState("");
  const [evidenceAgainst, setEvidenceAgainst] = useState("");
  const [balanced, setBalanced] = useState("");
  const [rerated, setRerated] = useState<Record<string, number>>({});
  const [custom, setCustom] = useState("");

  const save = useMutation({
    mutationFn: () =>
      api.createThought({
        situation,
        emotions,
        automatic_thought: automatic,
        distortions: picked,
        evidence_for: evidenceFor,
        evidence_against: evidenceAgainst,
        balanced_thought: balanced,
        rerated_emotions: emotions.map((e) => ({
          name: e.name,
          intensity: rerated[e.name] ?? e.intensity,
        })),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["thoughts"] });
      onDone();
    },
  });

  const addEmotion = (name: string) => {
    const n = name.trim().toLowerCase();
    if (n && !emotions.some((e) => e.name === n))
      setEmotions([...emotions, { name: n, intensity: 50 }]);
  };
  const canNext = [situation.trim(), emotions.length > 0, automatic.trim(), true, true][step];

  return (
    <Card>
      <ol className="mb-4 flex gap-1" aria-label="Progress">
        {stepsMeta.map((s, i) => (
          <li
            key={s.title}
            className={`h-1 flex-1 rounded-full ${i <= step ? "bg-accent-500" : "bg-border"}`}
            aria-current={i === step ? "step" : undefined}
          />
        ))}
      </ol>
      <h2 className="font-semibold">
        {step + 1}. {stepsMeta[step].title}
      </h2>
      <p className="mb-3 text-sm text-fg-muted">{stepsMeta[step].prompt}</p>

      {step === 0 && (
        <textarea
          value={situation}
          onChange={(e) => setSituation(e.target.value)}
          aria-label="Situation"
          rows={3}
          className={inputClass}
        />
      )}

      {step === 1 && (
        <div className="space-y-3">
          <div className="flex flex-wrap gap-1.5">
            {commonEmotions
              .filter((n) => !emotions.some((e) => e.name === n))
              .map((n) => (
                <button
                  key={n}
                  type="button"
                  onClick={() => addEmotion(n)}
                  className="rounded-full border border-border px-2.5 py-0.5 text-xs hover:border-accent-500"
                >
                  + {n}
                </button>
              ))}
          </div>
          <form
            className="flex gap-2"
            onSubmit={(e) => {
              e.preventDefault();
              addEmotion(custom);
              setCustom("");
            }}
          >
            <input
              value={custom}
              onChange={(e) => setCustom(e.target.value)}
              placeholder="Another feeling…"
              aria-label="Another feeling"
              className={inputClass}
            />
            <Button type="submit" variant="soft" disabled={!custom.trim()}>
              Add
            </Button>
          </form>
          {emotions.map((e, i) => (
            <IntensityRow
              key={e.name}
              name={e.name}
              value={e.intensity}
              onChange={(v) =>
                setEmotions(emotions.map((x, j) => (j === i ? { ...x, intensity: v } : x)))
              }
              onRemove={() => setEmotions(emotions.filter((_, j) => j !== i))}
            />
          ))}
        </div>
      )}

      {step === 2 && (
        <div className="space-y-3">
          <textarea
            value={automatic}
            onChange={(e) => setAutomatic(e.target.value)}
            aria-label="Automatic thought"
            placeholder="e.g. “I'm going to mess this up and everyone will see.”"
            rows={3}
            className={inputClass}
          />
          <fieldset>
            <legend className="mb-2 text-sm font-medium">
              Any of these thinking traps? (optional)
            </legend>
            <div className="grid gap-1.5 sm:grid-cols-2">
              {distortions.map((d) => {
                const on = picked.includes(d.key);
                return (
                  <label
                    key={d.key}
                    className={`flex cursor-pointer gap-2 rounded-xl border p-2 text-sm ${on ? "border-accent-500 bg-accent-500/10" : "border-border"}`}
                  >
                    <input
                      type="checkbox"
                      checked={on}
                      onChange={() =>
                        setPicked(on ? picked.filter((k) => k !== d.key) : [...picked, d.key])
                      }
                      className="mt-0.5 accent-accent-600"
                    />
                    <span>
                      <span className="font-medium">{d.label}</span>
                      <span className="block text-xs text-fg-muted">{d.hint}</span>
                    </span>
                  </label>
                );
              })}
            </div>
          </fieldset>
        </div>
      )}

      {step === 3 && (
        <div className="space-y-3">
          <label className="block text-sm">
            <span className="font-medium">Evidence for the thought</span>
            <textarea
              value={evidenceFor}
              onChange={(e) => setEvidenceFor(e.target.value)}
              rows={2}
              className={`${inputClass} mt-1`}
            />
          </label>
          <label className="block text-sm">
            <span className="font-medium">Evidence against it</span>
            <textarea
              value={evidenceAgainst}
              onChange={(e) => setEvidenceAgainst(e.target.value)}
              rows={2}
              placeholder="What would you tell a friend who thought this?"
              className={`${inputClass} mt-1`}
            />
          </label>
        </div>
      )}

      {step === 4 && (
        <div className="space-y-3">
          <textarea
            value={balanced}
            onChange={(e) => setBalanced(e.target.value)}
            aria-label="Balanced thought"
            rows={3}
            className={inputClass}
          />
          <p className="text-sm font-medium">How strong are the feelings now?</p>
          {emotions.map((e) => (
            <IntensityRow
              key={e.name}
              name={e.name}
              before={e.intensity}
              value={rerated[e.name] ?? e.intensity}
              onChange={(v) => setRerated({ ...rerated, [e.name]: v })}
            />
          ))}
        </div>
      )}

      <ErrorText error={save.error} />
      <div className="mt-4 flex gap-2">
        {step > 0 ? (
          <Button variant="soft" onClick={() => setStep(step - 1)}>
            Back
          </Button>
        ) : (
          <Button variant="ghost" onClick={onDone}>
            Cancel
          </Button>
        )}
        <div className="flex-1" />
        {step < stepsMeta.length - 1 ? (
          <Button onClick={() => setStep(step + 1)} disabled={!canNext}>
            Next
          </Button>
        ) : (
          <Button onClick={() => save.mutate()} disabled={save.isPending}>
            Save
          </Button>
        )}
      </div>
    </Card>
  );
}

function IntensityRow({
  name,
  value,
  before,
  onChange,
  onRemove,
}: {
  name: string;
  value: number;
  before?: number;
  onChange: (v: number) => void;
  onRemove?: () => void;
}) {
  return (
    <div className="flex items-center gap-3 text-sm">
      <span className="w-24 capitalize">{name}</span>
      <input
        type="range"
        min={0}
        max={100}
        step={5}
        value={value}
        onChange={(e) => onChange(Number(e.target.value))}
        aria-label={`${name} intensity`}
        className="flex-1"
      />
      <span className="w-16 text-right tabular-nums text-fg-muted">
        {before !== undefined && before !== value ? `${before}→` : ""}
        {value}
      </span>
      {onRemove && (
        <button
          type="button"
          onClick={onRemove}
          aria-label={`Remove ${name}`}
          className="text-fg-muted"
        >
          ×
        </button>
      )}
    </div>
  );
}

function ThoughtItem({ t }: { t: ThoughtRecord }) {
  const [open, setOpen] = useState(false);
  const date = new Date(t.created_at).toLocaleDateString(undefined, {
    month: "short",
    day: "numeric",
  });
  return (
    <li>
      <Card>
        <button
          type="button"
          className="w-full text-left"
          onClick={() => setOpen(!open)}
          aria-expanded={open}
        >
          <p className="text-xs text-fg-muted">{date}</p>
          <p className="text-sm font-medium">“{t.automatic_thought || t.situation}”</p>
          {t.balanced_thought && (
            <p className="mt-1 text-sm text-fg-muted">→ {t.balanced_thought}</p>
          )}
        </button>
        {open && (
          <dl className="mt-3 space-y-2 border-t border-border pt-3 text-sm">
            <Field label="Situation" text={t.situation} />
            <Field
              label="Feelings"
              text={t.emotions
                .map((e) => {
                  const after = t.rerated_emotions.find((r) => r.name === e.name);
                  return `${e.name} ${e.intensity}${after && after.intensity !== e.intensity ? ` → ${after.intensity}` : ""}`;
                })
                .join(", ")}
            />
            <Field
              label="Thinking traps"
              text={t.distortions.map((d) => distortionLabel.get(d) ?? d).join(", ")}
            />
            <Field label="Evidence for" text={t.evidence_for} />
            <Field label="Evidence against" text={t.evidence_against} />
          </dl>
        )}
      </Card>
    </li>
  );
}

function Field({ label, text }: { label: string; text: string }) {
  if (!text) return null;
  return (
    <div>
      <dt className="text-xs text-fg-muted">{label}</dt>
      <dd className="whitespace-pre-wrap">{text}</dd>
    </div>
  );
}
