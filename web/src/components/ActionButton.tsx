import type { ComponentProps, ReactNode } from "react";
import { Button, Spinner } from "./Button";

type Variant = NonNullable<ComponentProps<typeof Button>["variant"]>;

type Props = {
  // When true render as an icon-only button; when false render with `idleLabel`.
  // The caller decides (typically via a header overflow measurement).
  compact: boolean;
  variant?: Variant;
  pending: boolean;
  onClick: () => void;
  // Glyph used in compact mode (replaced by a Spinner while pending).
  icon: ReactNode;
  idleLabel: ReactNode;
  pendingLabel: ReactNode;
  // Required: icon-only buttons need an accessible label.
  ariaLabel: string;
  // Tooltip for the labeled (non-compact) variant. Omit for no tooltip.
  title?: string;
  // Tooltip for the compact variant. Defaults to `ariaLabel`.
  compactTitle?: string;
};

export function ActionButton({
  compact,
  variant = "neutral",
  pending,
  onClick,
  icon,
  idleLabel,
  pendingLabel,
  ariaLabel,
  title,
  compactTitle,
}: Props) {
  if (compact) {
    return (
      <Button
        variant={variant}
        size="icon"
        onClick={onClick}
        disabled={pending}
        aria-label={ariaLabel}
        title={compactTitle ?? ariaLabel}
      >
        {pending ? <Spinner /> : icon}
      </Button>
    );
  }
  return (
    <Button
      variant={variant}
      onClick={onClick}
      pending={pending}
      idleLabel={idleLabel}
      pendingLabel={pendingLabel}
      title={title}
    />
  );
}
