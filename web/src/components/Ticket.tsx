import { useSortable } from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import { useMutation } from "@tanstack/react-query";
import { useCallback, useEffect, useRef, useState } from "react";
import { api, type Session, type Ticket as TicketType } from "@/api/client";
import { activeTicketStore, ticketStore, useIsActiveTicket, useSession, useTicket } from "@/store";

export const STATUS_COLOR: Record<string, string> = {
  stopped: "text-fg-muted",
  starting: "text-amber-400",
  stopping: "text-amber-400",
  idle: "text-emerald-400",
  working: "text-sky-400",
  awaiting_perm: "text-yellow-400",
  error: "text-danger",
};

export const STATUS_BG: Record<string, string> = {
  stopped: "bg-fg-muted",
  starting: "bg-amber-400",
  stopping: "bg-amber-400",
  idle: "bg-emerald-400",
  working: "bg-sky-400",
  awaiting_perm: "bg-yellow-400",
  error: "bg-danger",
};

// Used when a ticket has no session at all yet.
export const STATUS_BG_NONE = "bg-border";

function TicketCard({
  ticket,
  session,
  active,
  onClick,
  onDoubleClick,
  className,
  innerRef,
  style,
  dragProps,
  interactive = true,
  titleSlot,
}: {
  ticket: TicketType;
  session: Session | null;
  active: boolean;
  onClick?: () => void;
  onDoubleClick?: React.MouseEventHandler<HTMLDivElement>;
  className?: string;
  innerRef?: (el: HTMLElement | null) => void;
  style?: React.CSSProperties;
  dragProps?: React.HTMLAttributes<HTMLDivElement>;
  interactive?: boolean;
  titleSlot?: React.ReactNode;
}) {
  const status = session?.status ?? "stopped";
  const interactiveCls = interactive ? "cursor-pointer hover:bg-surface-3" : "";
  const isInteractive = interactive && !!onClick;
  return (
    // biome-ignore lint/a11y/noStaticElementInteractions: role/tabIndex/onKeyDown applied conditionally below.
    <div
      ref={innerRef}
      {...dragProps}
      style={style}
      role={isInteractive ? "button" : undefined}
      tabIndex={isInteractive ? 0 : undefined}
      onClick={onClick}
      onDoubleClick={onDoubleClick}
      onKeyDown={
        isInteractive
          ? (e) => {
              if (e.key === "Enter" || e.key === " ") {
                e.preventDefault();
                onClick?.();
              }
            }
          : undefined
      }
      data-ticket-card="true"
      className={`rounded bg-surface-2 p-2 text-sm transition-colors duration-150 ${interactiveCls} ${active ? "ring-2 ring-accent-500" : ""} ${className ?? ""}`}
    >
      <div className="flex items-center justify-between gap-2">
        {titleSlot ?? <span className="font-medium">{ticket.title}</span>}
        <span className={`text-xs ${STATUS_COLOR[status] ?? "text-fg-muted"}`}>{status}</span>
      </div>
      {ticket.body && <p className="mt-1 text-xs text-fg-muted line-clamp-2">{ticket.body}</p>}
    </div>
  );
}

export function Ticket({ id, sessionId }: { id: number; sessionId: number | null }) {
  const ticket = useTicket(id);
  const session = useSession(sessionId);
  const active = useIsActiveTicket(id);
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(ticket?.title ?? "");
  const inputRef = useRef<HTMLInputElement>(null);

  const { attributes, listeners, setNodeRef, isDragging, transform, transition } = useSortable({
    id,
    disabled: editing,
  });
  const style: React.CSSProperties = {
    transform: CSS.Translate.toString(transform),
    transition,
    opacity: isDragging ? 0.5 : 1,
  };

  const renameMut = useMutation({
    mutationFn: (title: string) => api.updateTicket(id, { title }),
    onSuccess: (updated) => {
      setEditing(false);
      ticketStore.set(updated.id, updated);
    },
  });

  useEffect(() => {
    if (editing && ticket) {
      setDraft(ticket.title);
      requestAnimationFrame(() => {
        inputRef.current?.focus();
        inputRef.current?.select();
      });
    }
  }, [editing, ticket]);

  const onSelect = useCallback(() => activeTicketStore.set(id), [id]);

  if (!ticket) return null;

  const submit = () => {
    const next = draft.trim();
    if (!next) return;
    if (next === ticket.title) {
      setEditing(false);
      return;
    }
    renameMut.mutate(next);
  };

  return (
    <TicketCard
      ticket={ticket}
      session={session ?? null}
      active={active}
      onClick={editing ? undefined : onSelect}
      onDoubleClick={(e) => {
        e.stopPropagation();
        setEditing(true);
      }}
      innerRef={setNodeRef}
      style={style}
      dragProps={editing ? undefined : { ...attributes, ...listeners }}
      interactive={!editing}
      titleSlot={
        editing ? (
          <input
            ref={inputRef}
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            onClick={(e) => e.stopPropagation()}
            onKeyDown={(e) => {
              if (e.key === "Enter") {
                e.preventDefault();
                submit();
              } else if (e.key === "Escape") {
                e.preventDefault();
                setEditing(false);
              }
            }}
            onBlur={submit}
            disabled={renameMut.isPending}
            className="min-w-0 flex-1 rounded bg-surface px-1 py-0.5 text-sm font-medium outline-none ring-1 ring-border focus:ring-accent-500"
          />
        ) : undefined
      }
    />
  );
}

export function TicketDragPreview({
  ticket,
  session,
  active,
}: {
  ticket: TicketType;
  session: Session | null;
  active: boolean;
}) {
  return (
    <TicketCard
      ticket={ticket}
      session={session}
      active={active}
      className="shadow-2xl ring-1 ring-border"
    />
  );
}
