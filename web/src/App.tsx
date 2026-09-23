import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { api } from "@/api/client";

export default function App() {
  const queryClient = useQueryClient();
  const [title, setTitle] = useState("");

  const health = useQuery({ queryKey: ["health"], queryFn: api.health });
  const items = useQuery({ queryKey: ["items"], queryFn: api.listItems });

  const invalidateItems = () => queryClient.invalidateQueries({ queryKey: ["items"] });
  const create = useMutation({
    mutationFn: api.createItem,
    onSuccess: () => {
      setTitle("");
      invalidateItems();
    },
  });
  const remove = useMutation({ mutationFn: api.deleteItem, onSuccess: invalidateItems });

  return (
    <div className="flex min-h-screen flex-col items-center bg-bg px-4 py-16 text-fg">
      <main className="w-full max-w-md space-y-6">
        <header>
          <h1 className="text-2xl font-semibold">Fullstack Template</h1>
          <p className="text-sm text-fg-muted">
            Go + SQLite backend, React + Tailwind frontend, one binary.
          </p>
        </header>

        <form
          className="flex gap-2"
          onSubmit={(e) => {
            e.preventDefault();
            if (title.trim()) create.mutate(title.trim());
          }}
        >
          <input
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            placeholder="Add an item…"
            className="flex-1 rounded-md border border-border bg-surface px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-accent-500"
          />
          <button
            type="submit"
            disabled={!title.trim() || create.isPending}
            className="rounded-md bg-accent-600 px-4 py-2 text-sm font-medium text-white hover:bg-accent-500 disabled:opacity-50"
          >
            Add
          </button>
        </form>

        <ul className="divide-y divide-border rounded-md border border-border bg-surface">
          {items.data?.length === 0 && (
            <li className="px-3 py-6 text-center text-sm text-fg-muted">No items yet.</li>
          )}
          {items.data?.map((item) => (
            <li key={item.id} className="flex items-center justify-between px-3 py-2 text-sm">
              <span>{item.title}</span>
              <button
                type="button"
                onClick={() => remove.mutate(item.id)}
                aria-label={`Delete ${item.title}`}
                className="px-1 text-fg-muted hover:text-danger"
              >
                ×
              </button>
            </li>
          ))}
        </ul>

        {items.error && <p className="text-sm text-danger">{String(items.error)}</p>}

        <footer className="text-xs text-fg-muted">
          {health.data ? `server ${health.data.version}` : "server unreachable"}
        </footer>
      </main>
    </div>
  );
}
