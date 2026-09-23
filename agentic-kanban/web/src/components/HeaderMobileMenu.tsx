import { type ReactNode, useRef, useState } from "react";
import { useClickOutside } from "@/hooks/useClickOutside";
import { ArchiveIcon, CogIcon, HelpIcon, MenuIcon, MoreIcon, PlusIcon } from "@/icons";
import { Button } from "./Button";

type MenuAction = (() => void) | null;

export function HeaderMobileMenu({
  onNewBoard,
  onArchived,
  onBoardSettings,
  onAppSettings,
  suggestRepoLink,
}: {
  onNewBoard: () => void;
  onArchived: MenuAction;
  onBoardSettings: MenuAction;
  onAppSettings: () => void;
  suggestRepoLink: boolean;
}) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement | null>(null);
  useClickOutside(ref, () => setOpen(false), { enabled: open });

  const close = () => setOpen(false);

  return (
    <div ref={ref} className="relative inline-flex">
      <Button
        variant="neutral"
        size="icon"
        onClick={() => setOpen((v) => !v)}
        aria-haspopup="menu"
        aria-expanded={open}
        aria-label="Menu"
        title="Menu"
      >
        <MoreIcon />
      </Button>
      {suggestRepoLink && !open && (
        <span
          aria-hidden
          className="pointer-events-none absolute -right-0.5 -top-0.5 h-2 w-2 rounded-full bg-accent-500 ring-2 ring-bg"
        />
      )}
      {open && (
        <div
          role="menu"
          className="absolute right-0 top-full z-20 mt-1 w-56 overflow-hidden rounded border border-border bg-surface text-sm shadow-lg"
        >
          <MenuItem
            icon={<PlusIcon />}
            label="New board"
            onClick={() => {
              close();
              onNewBoard();
            }}
          />
          {onArchived && (
            <MenuItem
              icon={<ArchiveIcon />}
              label="Archived tickets"
              onClick={() => {
                close();
                onArchived();
              }}
            />
          )}
          {onBoardSettings && (
            <MenuItem
              icon={<CogIcon />}
              label="Board settings"
              badge={suggestRepoLink}
              onClick={() => {
                close();
                onBoardSettings();
              }}
            />
          )}
          <MenuItem
            as="a"
            icon={<HelpIcon />}
            label="Help"
            href="https://jamison.lahman.dev/agentic-kanban/guide/"
            onClick={close}
          />
          <MenuItem
            icon={<MenuIcon />}
            label="App settings"
            onClick={() => {
              close();
              onAppSettings();
            }}
          />
        </div>
      )}
    </div>
  );
}

function MenuItem({
  as = "button",
  icon,
  label,
  badge,
  onClick,
  href,
}: {
  as?: "button" | "a";
  icon: ReactNode;
  label: string;
  badge?: boolean;
  onClick?: () => void;
  href?: string;
}) {
  const className = "flex w-full items-center gap-2 px-3 py-2 text-left text-fg hover:bg-surface-2";
  const content = (
    <>
      <span className="inline-flex h-4 w-4 items-center justify-center text-fg-muted">{icon}</span>
      <span className="flex-1 truncate">{label}</span>
      {badge && <span aria-hidden className="h-2 w-2 rounded-full bg-accent-500" />}
    </>
  );
  if (as === "a") {
    return (
      <a
        role="menuitem"
        href={href}
        target="_blank"
        rel="noopener noreferrer"
        onClick={onClick}
        className={className}
      >
        {content}
      </a>
    );
  }
  return (
    <button role="menuitem" type="button" onClick={onClick} className={className}>
      {content}
    </button>
  );
}
