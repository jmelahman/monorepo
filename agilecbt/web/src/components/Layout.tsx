import {
  ArrowsClockwiseIcon,
  CompassIcon,
  GearSixIcon,
  KanbanIcon,
  NotepadIcon,
  type Icon,
  SunIcon,
} from "@phosphor-icons/react";
import { NavLink, Outlet, useLocation, useSearchParams } from "react-router";
import type { Health } from "@/api/client";
import Settings, { isSettingsTab, type SettingsTab } from "@/pages/Settings";

const tabs: { to: string; label: string; icon: Icon }[] = [
  { to: "/", label: "Today", icon: SunIcon },
  { to: "/board", label: "Board", icon: KanbanIcon },
  { to: "/roadmap", label: "Roadmap", icon: CompassIcon },
  { to: "/thoughts", label: "Thoughts", icon: NotepadIcon },
  { to: "/retro", label: "Retro", icon: ArrowsClockwiseIcon },
];

// The shell is a fixed-height column: header and nav stay put and pages
// scroll inside <main>, so a page (like Today's chat) can also fill the
// space between them.
export default function Layout({ health }: { health: Health }) {
  // With a coach, Today and Roadmap are full-height chats with a side panel
  // on wide screens (so their wrapper spans the width there and ChatPage
  // centers the chat itself).
  const path = useLocation().pathname;
  const full = health.llm.available && (path === "/" || path === "/roadmap");
  // Settings opens over the current page; its tab is in the URL so it can be
  // linked to and survives a reload.
  const [params, setParams] = useSearchParams();
  const raw = params.get("settings");
  const settingsTab = raw === null ? null : isSettingsTab(raw) ? raw : "coach";
  function openSettings(tab: SettingsTab | null) {
    setParams(
      (p) => {
        const next = new URLSearchParams(p);
        if (tab) next.set("settings", tab);
        else next.delete("settings");
        return next;
      },
      { replace: settingsTab !== null },
    );
  }
  return (
    <div className="flex h-dvh flex-col bg-bg text-fg">
      <header className="shrink-0 border-b border-border bg-bg">
        <div className="mx-auto flex max-w-3xl items-center justify-between gap-2 px-4 py-2">
          <NavLink to="/" className="ui-brand text-lg font-semibold tracking-tight">
            AgileCBT
          </NavLink>
          <button
            type="button"
            aria-label="Settings"
            onClick={() => openSettings("coach")}
            className={`-mr-2 grid size-11 place-items-center rounded-lg transition-colors focus-visible:outline-2 focus-visible:outline-accent-600 ${settingsTab ? "text-accent-ink" : "text-fg-muted hover:text-fg"}`}
          >
            <GearSixIcon size={22} aria-hidden />
          </button>
        </div>
      </header>

      <main className="min-h-0 flex-1 overflow-y-auto">
        <div className={`mx-auto max-w-3xl px-4 py-4 ${full ? "h-full xl:max-w-none" : ""}`}>
          <Outlet />
        </div>
      </main>

      <nav
        aria-label="Main"
        className="shrink-0 border-t border-border bg-surface pb-[env(safe-area-inset-bottom)]"
      >
        <ul className="mx-auto flex max-w-3xl justify-around">
          {tabs.map((t) => (
            <li key={t.to} className="flex-1">
              <NavLink
                to={t.to}
                end={t.to === "/"}
                className={({ isActive }) =>
                  `flex flex-col items-center gap-0.5 py-2 text-xs transition-colors focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-accent-600 ${isActive ? "text-accent-ink" : "text-fg-muted hover:text-fg"}`
                }
              >
                {({ isActive }) => (
                  <>
                    <t.icon size={22} weight={isActive ? "fill" : "regular"} aria-hidden />
                    <span className="ui-caps">{t.label}</span>
                  </>
                )}
              </NavLink>
            </li>
          ))}
        </ul>
      </nav>

      <Settings
        health={health}
        tab={settingsTab}
        onTab={openSettings}
        onClose={() => openSettings(null)}
      />
    </div>
  );
}
