import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { api, subscribeBoard } from "./api/client";
import { Board } from "./components/Board";
import { CreateBoardForm } from "./components/CreateBoardForm";

export default function App() {
  const qc = useQueryClient();
  const boardsQ = useQuery({ queryKey: ["boards"], queryFn: api.listBoards });
  const [activeId, setActiveId] = useState<number | null>(null);

  useEffect(() => {
    if (activeId == null && boardsQ.data && boardsQ.data.length > 0) {
      setActiveId(boardsQ.data[0].id);
    }
  }, [boardsQ.data, activeId]);

  useEffect(() => {
    if (activeId == null) return;
    return subscribeBoard(activeId, () => {
      qc.invalidateQueries({ queryKey: ["board", activeId] });
    });
  }, [activeId, qc]);

  return (
    <div className="flex h-full flex-col">
      <header className="flex items-center gap-4 border-b border-zinc-800 px-4 py-2">
        <h1 className="text-lg font-semibold">Kanban</h1>
        <select
          className="rounded bg-zinc-900 px-2 py-1 text-sm"
          value={activeId ?? ""}
          onChange={(e) => setActiveId(Number(e.target.value))}
        >
          <option value="">— select board —</option>
          {(boardsQ.data ?? []).map((b) => (
            <option key={b.id} value={b.id}>
              {b.name}
            </option>
          ))}
        </select>
        <div className="ml-auto">
          <CreateBoardForm
            onCreated={(b) => {
              qc.invalidateQueries({ queryKey: ["boards"] });
              setActiveId(b.id);
            }}
          />
        </div>
      </header>
      <main className="min-h-0 flex-1 overflow-hidden">
        {activeId != null ? <Board boardId={activeId} /> : <p className="p-4 text-sm text-zinc-400">No board selected.</p>}
      </main>
    </div>
  );
}
