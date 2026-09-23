import type { InputHTMLAttributes, ReactNode } from "react";

export function FormField({
  label,
  hint,
  children,
}: {
  label: string;
  hint?: ReactNode;
  children: ReactNode;
}) {
  return (
    // biome-ignore lint/a11y/noLabelWithoutControl: children slot holds the control at runtime.
    <label className="flex flex-col gap-1">
      <span className="text-xs text-fg-muted">{label}</span>
      {children}
      {hint && <span className="text-xs text-fg-muted">{hint}</span>}
    </label>
  );
}

export function FormInput({
  mono,
  className,
  ...props
}: InputHTMLAttributes<HTMLInputElement> & { mono?: boolean }) {
  const cls = ["rounded bg-surface px-2 py-1", mono && "font-mono", className]
    .filter(Boolean)
    .join(" ");
  return <input className={cls} {...props} />;
}
