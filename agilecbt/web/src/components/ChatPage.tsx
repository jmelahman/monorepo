import { type ReactNode, useState } from "react";

// ChatPage is a coach conversation filling the screen. On wide screens the
// chat stays centered under the header, with a side panel in the right-hand
// gutter; on phones the panel is a one-line toggle above the chat, so the
// conversation keeps the room.
export default function ChatPage({
  top,
  title,
  label,
  side,
  children,
}: {
  top: ReactNode;
  title: string;
  label: string;
  side: ReactNode;
  children: ReactNode;
}) {
  const [open, setOpen] = useState(false);
  return (
    <div className="flex h-full flex-col gap-3 xl:grid xl:grid-cols-[minmax(16rem,1fr)_minmax(0,48rem)_minmax(16rem,1fr)] xl:grid-rows-[minmax(0,1fr)] xl:gap-6">
      <div className="flex min-h-0 min-w-0 flex-1 flex-col gap-3 xl:col-start-2">
        {top}
        <div className="shrink-0 xl:hidden">
          <button
            type="button"
            className="text-sm text-fg-muted hover:text-fg"
            onClick={() => setOpen((v) => !v)}
            aria-expanded={open}
          >
            {open ? "▾" : "▸"} {label}
          </button>
          {open && <div className="mt-2 max-h-[40dvh] overflow-y-auto">{side}</div>}
        </div>
        <div className="min-h-0 flex-1">{children}</div>
      </div>
      <aside className="hidden max-w-xs overflow-y-auto xl:col-start-3 xl:block">
        <h2 className="mb-2 text-sm font-semibold">{title}</h2>
        {side}
      </aside>
    </div>
  );
}
