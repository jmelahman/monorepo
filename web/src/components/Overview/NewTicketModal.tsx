import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";
import { api } from "@/api/client";
import type { Board } from "@/api/client";
import { queryKeys } from "@/api/keys";
import type { BoardStructure } from "@/store";
import { fetchBoardStructure } from "@/store";
import { Button } from "@/components/Button";
import { FormField } from "@/components/FormField";
import { Modal } from "@/components/Modal";

// Mounted only while open. We deliberately read board metadata via
// `qc.getQueryData` rather than `useQuery` so we don't add cache observers
// during render — those subscriptions can fire notifications that cascade
// into TerminalsRoot's `useSyncExternalStore` subscriber while this
// component is still rendering. BoardTree keeps the relevant queries warm
// in the cache, so a passive read is enough.
export function NewTicketModal({
  defaultBoardId,
  onClose,
  onCreated,
}: {
  defaultBoardId: number | null;
  onClose: () => void;
  onCreated: (ticketId: number, boardId: number) => void;
}) {
  const qc = useQueryClient();
  const boards = qc.getQueryData<Board[]>(queryKeys.boards) ?? [];

  const [title, setTitle] = useState("");
  const [boardId, setBoardId] = useState<number | null>(
    () => defaultBoardId ?? boards[0]?.id ?? null,
  );
  const [structure, setStructure] = useState<BoardStructure | null>(() =>
    boardId != null ? (qc.getQueryData<BoardStructure>(queryKeys.board(boardId)) ?? null) : null,
  );
  const [columnId, setColumnId] = useState<number | null>(() => structure?.columns[0]?.id ?? null);
  const titleInputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    titleInputRef.current?.focus();
  }, []);

  // When the user changes the selected board, re-read the structure from
  // the cache (kept warm by BoardTree). If the cache is empty for that
  // board, fall back to a one-off fetch.
  useEffect(() => {
    if (boardId == null) {
      setStructure(null);
      setColumnId(null);
      return;
    }
    const cached = qc.getQueryData<BoardStructure>(queryKeys.board(boardId));
    if (cached) {
      setStructure(cached);
      setColumnId(cached.columns[0]?.id ?? null);
      return;
    }
    let cancelled = false;
    qc.fetchQuery({
      queryKey: queryKeys.board(boardId),
      queryFn: () => fetchBoardStructure(boardId),
    }).then((s) => {
      if (cancelled) return;
      setStructure(s);
      setColumnId(s.columns[0]?.id ?? null);
    });
    return () => {
      cancelled = true;
    };
  }, [boardId, qc]);

  const columns = structure?.columns ?? [];

  const createMut = useMutation({
    mutationFn: (input: { boardId: number; columnId: number; title: string }) =>
      api.createTicket(input.boardId, { column_id: input.columnId, title: input.title }),
    // Wait for the refetch so the new ticket is in the cache before we tell
    // PanelCanvas to open it — otherwise its isLiveTicket guard filters the
    // freshly-added panel out on the next cache notification.
    onSuccess: async (ticket, vars) => {
      await qc.refetchQueries({ queryKey: queryKeys.board(vars.boardId) });
      onCreated(ticket.id, vars.boardId);
      onClose();
    },
  });

  const canSubmit =
    title.trim().length > 0 && boardId != null && columnId != null && !createMut.isPending;

  function submit() {
    if (!canSubmit || boardId == null || columnId == null) return;
    createMut.mutate({ boardId, columnId, title: title.trim() });
  }

  return (
    <Modal open onClose={onClose} title="New ticket" busy={createMut.isPending}>
      <form
        onSubmit={(e) => {
          e.preventDefault();
          submit();
        }}
        className="flex flex-col gap-3 p-3 text-sm"
      >
        <FormField label="Title">
          <input
            ref={titleInputRef}
            className="rounded bg-surface-2 px-2 py-1"
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            disabled={createMut.isPending}
            placeholder="ticket title"
          />
        </FormField>
        <FormField label="Board">
          <select
            aria-label="Board"
            className="rounded bg-surface-2 px-2 py-1"
            value={boardId ?? ""}
            onChange={(e) => setBoardId(e.target.value ? Number(e.target.value) : null)}
            disabled={createMut.isPending || boards.length === 0}
          >
            {boards.length === 0 && <option value="">Loading…</option>}
            {boards.map((b) => (
              <option key={b.id} value={b.id}>
                {b.name}
              </option>
            ))}
          </select>
        </FormField>
        <FormField label="Column">
          <select
            aria-label="Column"
            className="rounded bg-surface-2 px-2 py-1"
            value={columnId ?? ""}
            onChange={(e) => setColumnId(e.target.value ? Number(e.target.value) : null)}
            disabled={createMut.isPending || columns.length === 0}
          >
            {columns.length === 0 && <option value="">Loading…</option>}
            {columns.map((c) => (
              <option key={c.id} value={c.id}>
                {c.name}
              </option>
            ))}
          </select>
        </FormField>
        <div className="flex justify-end gap-2 pt-1">
          <Button type="button" variant="ghost" onClick={onClose} disabled={createMut.isPending}>
            cancel
          </Button>
          <Button
            type="submit"
            variant="primary"
            disabled={!canSubmit}
            pending={createMut.isPending}
            idleLabel="create"
            pendingLabel="creating…"
          />
        </div>
      </form>
    </Modal>
  );
}
