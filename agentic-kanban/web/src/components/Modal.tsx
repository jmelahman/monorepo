import { createContext, type ReactNode, useContext, useEffect, useId, useRef } from "react";
import { createPortal } from "react-dom";
import { XIcon } from "@/icons";
import { useEscapeClose } from "../hooks/useEscapeClose";
import { Button } from "./Button";

type Common = {
  open: boolean;
  onClose: () => void;
  title: string;
  children: ReactNode;
  // When true, backdrop click + close button are disabled (e.g. while a
  // mutation is pending). Escape is also suppressed.
  busy?: boolean;
};

// Carries the dialog's title id from DialogShell down to DialogHeader so the
// dialog can be labelled by its visible heading.
const TitleIdContext = createContext<string | undefined>(undefined);

// Modals default to form width. `wide` is for content that reads badly in a
// narrow column — log panes, diffs, tables.
export function Modal({
  open,
  onClose,
  title,
  children,
  busy = false,
  wide = false,
}: Common & { wide?: boolean }) {
  return (
    <DialogShell open={open} onClose={onClose} busy={busy} flavor="modal">
      <div
        className={`relative ${wide ? "w-[820px]" : "w-[520px]"} max-h-full max-w-full overflow-y-auto animate-modal-in rounded border border-border bg-bg shadow-lg`}
      >
        <DialogHeader title={title} onClose={onClose} busy={busy} />
        {children}
      </div>
    </DialogShell>
  );
}

export function Drawer({ open, onClose, title, children, busy = false }: Common) {
  return (
    <DialogShell open={open} onClose={onClose} busy={busy} flavor="drawer">
      <aside className="animate-drawer-in flex w-[480px] max-w-[calc(100vw-2rem)] flex-col border-l border-border bg-bg shadow-lg">
        <DialogHeader title={title} onClose={onClose} busy={busy} />
        {children}
      </aside>
    </DialogShell>
  );
}

type ConfirmModalProps = {
  open: boolean;
  onClose: () => void;
  title: string;
  // Main prompt; ReactNode so callers can embed <span className="font-medium">
  // emphasis or interpolated values.
  description: ReactNode;
  // Smaller secondary line describing consequences. Omit to skip it entirely.
  consequence?: ReactNode;
  onConfirm: () => void;
  confirmLabel: string;
  confirmPendingLabel: string;
  // Visual treatment of the confirm action. Defaults to "danger".
  confirmVariant?: "danger" | "primary";
  pending: boolean;
};

export function ConfirmModal({
  open,
  onClose,
  title,
  description,
  consequence,
  onConfirm,
  confirmLabel,
  confirmPendingLabel,
  confirmVariant = "danger",
  pending,
}: ConfirmModalProps) {
  return (
    <Modal open={open} onClose={onClose} title={title} busy={pending}>
      <div className="p-4">
        <p className="text-sm">{description}</p>
        {consequence != null && <p className="mt-2 text-xs text-fg-muted">{consequence}</p>}
        <div className="mt-4 flex justify-end gap-2">
          <Button type="button" variant="ghost" onClick={onClose} disabled={pending}>
            cancel
          </Button>
          <Button
            type="button"
            variant={confirmVariant}
            size="lg"
            onClick={onConfirm}
            disabled={pending}
            pending={pending}
            idleLabel={confirmLabel}
            pendingLabel={confirmPendingLabel}
          />
        </div>
      </div>
    </Modal>
  );
}

function DialogShell({
  open,
  onClose,
  busy,
  flavor,
  children,
}: {
  open: boolean;
  onClose: () => void;
  busy: boolean;
  flavor: "modal" | "drawer";
  children: ReactNode;
}) {
  useEscapeClose(open && !busy, onClose);
  useScrollLock(open);
  useRestoreFocus(open);
  const titleId = useId();
  const dialogRef = useRef<HTMLDivElement>(null);

  // Move focus into the dialog on open unless a child already claimed it
  // (e.g. an autoFocus input), so Tab starts inside rather than behind it.
  useEffect(() => {
    if (!open) return;
    const el = dialogRef.current;
    if (el && !el.contains(document.activeElement)) el.focus({ preventScroll: true });
  }, [open]);

  if (!open) return null;

  const layoutClass =
    flavor === "modal"
      ? "fixed inset-0 z-(--z-overlay) flex items-center justify-center p-4 outline-none"
      : "fixed inset-0 z-(--z-overlay) flex outline-none";
  const backdrop =
    flavor === "modal" ? (
      <button
        type="button"
        aria-label="Close"
        tabIndex={-1}
        className="animate-backdrop-in absolute inset-0 bg-black/50"
        onClick={busy ? undefined : onClose}
      />
    ) : (
      <button
        type="button"
        aria-label="Close"
        tabIndex={-1}
        className="animate-backdrop-in flex-1 bg-black/50"
        onClick={busy ? undefined : onClose}
      />
    );

  return createPortal(
    <div
      ref={dialogRef}
      className={layoutClass}
      role="dialog"
      aria-modal="true"
      aria-labelledby={titleId}
      tabIndex={-1}
    >
      {backdrop}
      <TitleIdContext.Provider value={titleId}>{children}</TitleIdContext.Provider>
    </div>,
    document.body,
  );
}

function DialogHeader({
  title,
  onClose,
  busy,
}: {
  title: string;
  onClose: () => void;
  busy: boolean;
}) {
  const titleId = useContext(TitleIdContext);
  return (
    <header className="flex items-center justify-between border-b border-border px-3 py-2">
      <h2 id={titleId} className="text-sm font-semibold">
        {title}
      </h2>
      <Button variant="neutral" size="icon" onClick={onClose} disabled={busy} aria-label="Close">
        <XIcon />
      </Button>
    </header>
  );
}

function useScrollLock(active: boolean) {
  useEffect(() => {
    if (!active) return;
    const prev = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => {
      document.body.style.overflow = prev;
    };
  }, [active]);
}

function useRestoreFocus(active: boolean) {
  const previousRef = useRef<Element | null>(null);
  useEffect(() => {
    if (!active) return;
    previousRef.current = document.activeElement;
    return () => {
      const el = previousRef.current;
      if (el instanceof HTMLElement) el.focus();
    };
  }, [active]);
}
