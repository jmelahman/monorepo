import type { ButtonHTMLAttributes, ReactNode } from "react";

type Variant = "primary" | "secondary" | "neutral" | "danger" | "ghost" | "dashed";
type Size = "sm" | "md" | "lg" | "icon";

type Props = Omit<ButtonHTMLAttributes<HTMLButtonElement>, "children"> & {
  variant?: Variant;
  size?: Size;
  pending?: boolean;
  idleLabel?: ReactNode;
  pendingLabel?: ReactNode;
  children?: ReactNode;
};

const BASE =
  "rounded transition-colors duration-150 disabled:opacity-50 disabled:cursor-not-allowed";

const VARIANTS: Record<Variant, string> = {
  primary: "bg-accent-700 text-white hover:bg-accent-600",
  secondary: "bg-surface-3 text-fg hover:bg-surface-4",
  neutral: "bg-surface-2 text-fg hover:bg-surface-3",
  danger: "bg-danger-bg text-danger hover:bg-danger-border hover:text-fg",
  ghost: "text-fg-muted hover:text-fg",
  dashed:
    "border border-dashed border-border text-fg-muted hover:bg-surface-2 hover:border-fg-muted",
};

const SIZES: Record<Size, string> = {
  sm: "px-2 py-0.5 text-xs",
  md: "px-2 py-1",
  lg: "px-3 py-1",
  icon: "inline-flex h-7 w-7 items-center justify-center",
};

export function Button({
  variant = "neutral",
  size = "md",
  pending = false,
  idleLabel,
  pendingLabel,
  disabled,
  className = "",
  children,
  ...rest
}: Props) {
  const content =
    pending && pendingLabel ? (
      <span className="inline-flex items-center gap-1.5">
        <Spinner />
        {pendingLabel}
      </span>
    ) : (
      (idleLabel ?? children)
    );
  return (
    <button
      {...rest}
      disabled={pending || disabled}
      className={`${BASE} ${VARIANTS[variant]} ${SIZES[size]} ${className}`}
    >
      {content}
    </button>
  );
}

export function Spinner({ className = "" }: { className?: string }) {
  return (
    <span
      role="status"
      aria-label="loading"
      className={`inline-block h-3 w-3 animate-spin rounded-full border border-current border-r-transparent align-[-1px] ${className}`}
    />
  );
}
