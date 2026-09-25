import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { NavLink, Outlet } from "react-router";
import { api } from "@/api/client";
import { Dialog, Markdownish } from "@/components/ui";

const tabs = [
  { to: "/", label: "Today", icon: "☀" },
  { to: "/board", label: "Board", icon: "▦" },
  { to: "/roadmap", label: "Roadmap", icon: "✦" },
  { to: "/thoughts", label: "Thoughts", icon: "❍" },
  { to: "/retro", label: "Retro", icon: "↺" },
];

export default function Layout() {
  const [helpOpen, setHelpOpen] = useState(false);
  return (
    <div className="min-h-dvh bg-bg text-fg">
      <header className="sticky top-0 z-10 border-b border-border bg-bg/90 backdrop-blur">
        <div className="mx-auto flex max-w-3xl items-center justify-between gap-2 px-4 py-3">
          <NavLink to="/" className="text-lg font-semibold tracking-tight">
            AgileCBT
          </NavLink>
          <div className="flex items-center gap-1">
            <button
              type="button"
              onClick={() => setHelpOpen(true)}
              className="rounded-full border border-danger/50 px-3 py-1 text-sm text-danger hover:bg-danger/10"
            >
              Need help now?
            </button>
            <NavLink
              to="/settings"
              aria-label="Settings"
              className={({ isActive }) =>
                `rounded-lg px-2 py-1 text-lg ${isActive ? "text-accent-500" : "text-fg-muted hover:text-fg"}`
              }
            >
              ⚙
            </NavLink>
          </div>
        </div>
      </header>

      <main className="mx-auto max-w-3xl px-4 pt-4 pb-28">
        <Outlet />
      </main>

      <nav
        aria-label="Main"
        className="fixed inset-x-0 bottom-0 z-10 border-t border-border bg-surface/95 pb-[env(safe-area-inset-bottom)] backdrop-blur"
      >
        <ul className="mx-auto flex max-w-3xl justify-around">
          {tabs.map((t) => (
            <li key={t.to} className="flex-1">
              <NavLink
                to={t.to}
                end={t.to === "/"}
                className={({ isActive }) =>
                  `flex flex-col items-center gap-0.5 py-2 text-xs ${isActive ? "text-accent-500" : "text-fg-muted hover:text-fg"}`
                }
              >
                <span aria-hidden className="text-lg leading-none">
                  {t.icon}
                </span>
                {t.label}
              </NavLink>
            </li>
          ))}
        </ul>
      </nav>

      <HelpDialog open={helpOpen} onClose={() => setHelpOpen(false)} />
    </div>
  );
}

function HelpDialog({ open, onClose }: { open: boolean; onClose: () => void }) {
  const settings = useQuery({ queryKey: ["settings"], queryFn: api.settings, enabled: open });
  return (
    <Dialog open={open} onClose={onClose} title="You don't have to do this alone">
      {settings.data ? (
        <Markdownish text={settings.data.crisis_resources} className="text-sm leading-relaxed" />
      ) : (
        <p className="text-sm leading-relaxed">
          If you might act on thoughts of harming yourself, call your local emergency number. In the
          US, call or text 988.
        </p>
      )}
    </Dialog>
  );
}
