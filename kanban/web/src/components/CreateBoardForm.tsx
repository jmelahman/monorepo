import { useState } from "react";
import { api, Board } from "../api/client";

export function CreateBoardForm({ onCreated }: { onCreated: (b: Board) => void }) {
  const [open, setOpen] = useState(false);
  const [name, setName] = useState("");
  const [repo, setRepo] = useState("");
  const [base, setBase] = useState("main");
  const [busy, setBusy] = useState(false);

  if (!open) {
    return (
      <button className="rounded bg-zinc-800 px-3 py-1 text-sm hover:bg-zinc-700" onClick={() => setOpen(true)}>
        + new board
      </button>
    );
  }

  return (
    <form
      className="flex items-center gap-2 text-sm"
      onSubmit={async (e) => {
        e.preventDefault();
        setBusy(true);
        try {
          const board = await api.createBoard({ name, source_repo_path: repo, base_branch: base });
          onCreated(board);
          setOpen(false);
          setName("");
          setRepo("");
        } finally {
          setBusy(false);
        }
      }}
    >
      <input className="rounded bg-zinc-900 px-2 py-1" placeholder="name" value={name} onChange={(e) => setName(e.target.value)} required />
      <input className="rounded bg-zinc-900 px-2 py-1 w-72" placeholder="/host/path/to/repo" value={repo} onChange={(e) => setRepo(e.target.value)} required />
      <input className="rounded bg-zinc-900 px-2 py-1 w-28" placeholder="base branch" value={base} onChange={(e) => setBase(e.target.value)} />
      <button disabled={busy} className="rounded bg-emerald-700 px-3 py-1 text-white disabled:opacity-50">
        create
      </button>
      <button type="button" className="text-zinc-400" onClick={() => setOpen(false)}>
        cancel
      </button>
    </form>
  );
}
