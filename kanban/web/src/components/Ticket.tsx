import { useDraggable } from "@dnd-kit/core";
import { Session, Ticket as TicketType } from "../api/client";

const STATUS_COLOR: Record<string, string> = {
  stopped: "text-zinc-500",
  starting: "text-amber-400",
  idle: "text-emerald-400",
  working: "text-sky-400",
  awaiting_perm: "text-yellow-400",
  error: "text-red-400",
};

export function Ticket({
  ticket,
  session,
  active,
  onSelect,
}: {
  ticket: TicketType;
  session: Session | null;
  active: boolean;
  onSelect: () => void;
}) {
  const { attributes, listeners, setNodeRef, transform, isDragging } = useDraggable({ id: ticket.id });
  const style: React.CSSProperties = {
    transform: transform ? `translate3d(${transform.x}px, ${transform.y}px, 0)` : undefined,
    opacity: isDragging ? 0.5 : 1,
  };
  const status = session?.status ?? "stopped";
  return (
    <div
      ref={setNodeRef}
      {...attributes}
      {...listeners}
      style={style}
      onClick={onSelect}
      className={`cursor-pointer rounded bg-zinc-800 p-2 text-sm hover:bg-zinc-700 ${active ? "ring-2 ring-red-500" : ""}`}
    >
      <div className="flex items-center justify-between">
        <span className="font-medium">{ticket.title}</span>
        <span className={`text-xs ${STATUS_COLOR[status] ?? "text-zinc-500"}`}>{status}</span>
      </div>
      {ticket.body && <p className="mt-1 text-xs text-zinc-400 line-clamp-2">{ticket.body}</p>}
    </div>
  );
}
