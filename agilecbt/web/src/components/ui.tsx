import { type ButtonHTMLAttributes, type ReactNode, useEffect, useId, useRef } from "react";

type Variant = "primary" | "soft" | "ghost";

const variants: Record<Variant, string> = {
  primary: "bg-accent-600 text-white hover:bg-accent-700",
  soft: "bg-surface-2 text-fg hover:bg-border",
  ghost: "text-fg-muted hover:text-fg hover:bg-surface-2",
};

export function Button({
  variant = "primary",
  className = "",
  ...props
}: ButtonHTMLAttributes<HTMLButtonElement> & { variant?: Variant }) {
  return (
    <button
      type="button"
      {...props}
      className={`rounded-xl px-4 py-2 text-sm font-medium transition-colors focus-visible:outline-2 focus-visible:outline-accent-500 disabled:opacity-50 ${variants[variant]} ${className}`}
    />
  );
}

export function Card({ children, className = "" }: { children: ReactNode; className?: string }) {
  return (
    <section className={`rounded-2xl border border-border bg-surface p-4 ${className}`}>
      {children}
    </section>
  );
}

export function SectionTitle({ children, aside }: { children: ReactNode; aside?: ReactNode }) {
  return (
    <div className="mb-3 flex items-baseline justify-between gap-2">
      <h2 className="text-base font-semibold">{children}</h2>
      {aside}
    </div>
  );
}

export const inputClass =
  "w-full rounded-xl border border-border bg-bg px-3 py-2 text-sm text-fg placeholder:text-fg-muted/70 outline-none focus:ring-2 focus:ring-accent-500";

// Slider is a labeled 0–max range with the current value shown.
export function Slider({
  label,
  value,
  onChange,
  max = 10,
  low,
  high,
}: {
  label: string;
  value: number | null;
  onChange: (v: number) => void;
  max?: number;
  low?: string;
  high?: string;
}) {
  const id = useId();
  return (
    <div>
      <div className="flex items-baseline justify-between text-sm">
        <label htmlFor={id} className="font-medium">
          {label}
        </label>
        <span className="tabular-nums text-fg-muted">{value ?? "–"}</span>
      </div>
      <input
        id={id}
        type="range"
        min={0}
        max={max}
        value={value ?? Math.round(max / 2)}
        onChange={(e) => onChange(Number(e.target.value))}
        className={`w-full ${value === null ? "opacity-50" : ""}`}
      />
      {(low || high) && (
        <div className="flex justify-between text-xs text-fg-muted">
          <span>{low}</span>
          <span>{high}</span>
        </div>
      )}
    </div>
  );
}

// Energy is shown as 1–3 small dots; it's a cost, not a judgment.
export function EnergyDots({ cost }: { cost: number }) {
  return (
    <span className="inline-flex gap-0.5" role="img" aria-label={`energy ${cost} of 3`}>
      {[1, 2, 3].map((n) => (
        <span
          key={n}
          className={`h-1.5 w-1.5 rounded-full ${n <= cost ? "bg-accent-500" : "bg-border"}`}
        />
      ))}
    </span>
  );
}

// Dialog is a native <dialog> that sits at the bottom on phones.
export function Dialog({
  open,
  onClose,
  title,
  children,
}: {
  open: boolean;
  onClose: () => void;
  title: string;
  children: ReactNode;
}) {
  const ref = useRef<HTMLDialogElement>(null);
  useEffect(() => {
    const d = ref.current;
    if (!d) return;
    if (open && !d.open) d.showModal();
    if (!open && d.open) d.close();
  }, [open]);
  return (
    <dialog
      ref={ref}
      onClose={onClose}
      aria-label={title}
      className="m-auto mb-0 w-full max-w-lg rounded-t-2xl border border-border bg-surface p-0 text-fg backdrop:bg-black/50 sm:mb-auto sm:rounded-2xl"
    >
      {open && (
        <div className="p-5">
          <div className="mb-4 flex items-center justify-between">
            <h2 className="text-lg font-semibold">{title}</h2>
            <button
              type="button"
              onClick={onClose}
              aria-label="Close"
              className="rounded-lg px-2 text-xl text-fg-muted hover:text-fg"
            >
              ×
            </button>
          </div>
          {children}
        </div>
      )}
    </dialog>
  );
}

export function ErrorText({ error }: { error: unknown }) {
  if (!error) return null;
  return (
    <p className="text-sm text-danger">{error instanceof Error ? error.message : String(error)}</p>
  );
}

// Markdownish renders plain text with paragraph breaks and **bold** runs,
// enough for curator replies and crisis resources without a markdown lib.
export function Markdownish({ text, className = "" }: { text: string; className?: string }) {
  return (
    <div className={`space-y-2 ${className}`}>
      {text.split(/\n{2,}/).map((para, i) => (
        // biome-ignore lint/suspicious/noArrayIndexKey: paragraphs have no identity
        <p key={i} className="whitespace-pre-wrap">
          {para.split(/(\*\*[^*]+\*\*)/).map((part, j) =>
            part.startsWith("**") && part.endsWith("**") ? (
              // biome-ignore lint/suspicious/noArrayIndexKey: inline runs have no identity
              <strong key={j}>{part.slice(2, -2)}</strong>
            ) : (
              part
            ),
          )}
        </p>
      ))}
    </div>
  );
}
