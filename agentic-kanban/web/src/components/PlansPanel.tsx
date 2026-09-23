import { useQuery } from "@tanstack/react-query";
import { useEffect, useMemo, useState } from "react";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { api, type Session } from "@/api/client";
import { queryKeys } from "@/api/keys";
import { MARKDOWN_COMPONENTS } from "./markdownComponents";

const SIDEBAR_COLLAPSED_KEY = "plans.sidebar.collapsed";

function loadSidebarCollapsed(): boolean {
  try {
    return localStorage.getItem(SIDEBAR_COLLAPSED_KEY) === "1";
  } catch {
    return false;
  }
}

export function PlansPanel({ session }: { session: Session }) {
  const plansQ = useQuery({
    queryKey: queryKeys.sessionPlans(session.id),
    queryFn: () => api.listSessionPlans(session.id),
    refetchInterval: 5000,
  });

  const sorted = useMemo(() => {
    const list = plansQ.data ?? [];
    return [...list].sort((a, b) => (b.mod_time > a.mod_time ? 1 : -1));
  }, [plansQ.data]);

  const [selected, setSelected] = useState<string | null>(null);
  const [collapsed, setCollapsed] = useState(loadSidebarCollapsed);

  const toggleCollapsed = () => {
    setCollapsed((prev) => {
      const next = !prev;
      try {
        localStorage.setItem(SIDEBAR_COLLAPSED_KEY, next ? "1" : "0");
      } catch {
        // ignore
      }
      return next;
    });
  };

  // Re-pin the default selection whenever the list changes — if the
  // previously selected file disappears, fall back to the newest one.
  useEffect(() => {
    if (sorted.length === 0) {
      setSelected(null);
      return;
    }
    if (selected != null && sorted.some((p) => p.name === selected)) return;
    setSelected(sorted[0].name);
  }, [sorted, selected]);

  if (plansQ.isLoading) {
    return <p className="p-4 text-sm text-fg-muted">Loading plans…</p>;
  }
  if (sorted.length === 0) {
    return <p className="p-4 text-sm text-fg-muted">No plan files found.</p>;
  }

  return (
    <div className="flex h-full min-h-0">
      {collapsed ? (
        <button
          type="button"
          onClick={toggleCollapsed}
          title="Show plans sidebar"
          aria-label="Show plans sidebar"
          aria-expanded={false}
          className="flex h-full w-6 shrink-0 items-center justify-center border-r border-border bg-bg text-fg-muted hover:bg-surface-2 hover:text-fg"
        >
          <span aria-hidden>▶</span>
        </button>
      ) : (
        <aside className="flex w-56 shrink-0 flex-col border-r border-border bg-surface text-sm">
          <div className="sticky top-0 z-10 flex items-center border-b border-border bg-surface px-3 py-2">
            <h2 className="text-xs font-semibold uppercase tracking-wide text-fg-muted">Plans</h2>
            <button
              type="button"
              onClick={toggleCollapsed}
              title="Hide plans sidebar"
              aria-label="Hide plans sidebar"
              aria-expanded={true}
              className="ml-auto rounded px-1 text-fg-muted hover:bg-surface-2 hover:text-fg"
            >
              <span aria-hidden>◀</span>
            </button>
          </div>
          <ul className="flex-1 overflow-y-auto [scrollbar-gutter:stable]">
            {sorted.map((p) => (
              <li key={p.name}>
                <button
                  type="button"
                  onClick={() => setSelected(p.name)}
                  className={`block w-full truncate px-3 py-1.5 text-left hover:bg-bg ${
                    selected === p.name ? "bg-bg font-medium" : ""
                  }`}
                  title={p.name}
                >
                  {p.name.replace(/\.md$/i, "")}
                </button>
              </li>
            ))}
          </ul>
        </aside>
      )}
      <div className="min-w-0 flex-1 overflow-y-auto [scrollbar-gutter:stable]">
        {selected && <PlanContent sessionId={session.id} name={selected} />}
      </div>
    </div>
  );
}

function PlanContent({ sessionId, name }: { sessionId: number; name: string }) {
  const q = useQuery({
    queryKey: queryKeys.sessionPlan(sessionId, name),
    queryFn: () => api.getSessionPlan(sessionId, name),
  });

  if (q.isLoading) {
    return <p className="p-4 text-sm text-fg-muted">Loading…</p>;
  }
  if (q.error) {
    return <p className="p-4 text-sm text-danger">Couldn't load {name}</p>;
  }

  return (
    <article className="p-4 text-sm leading-relaxed text-fg">
      <ReactMarkdown remarkPlugins={[remarkGfm]} components={MARKDOWN_COMPONENTS}>
        {q.data ?? ""}
      </ReactMarkdown>
    </article>
  );
}
