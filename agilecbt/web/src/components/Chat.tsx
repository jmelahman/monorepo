import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";
import { type Action, api, chat } from "@/api/client";
import { Button, ErrorText, inputClass, Markdownish } from "@/components/ui";

type Item =
  | { kind: "msg"; key: string; role: "user" | "assistant"; text: string; at: string }
  | { kind: "action"; key: string; action: Action; at: string };

// Chat is the curator conversation for one check-in. Replies stream in; any
// change the curator makes shows up as a chip with Undo.
export default function Chat({ checkinId, prompt }: { checkinId: number; prompt: string }) {
  const qc = useQueryClient();
  const detail = useQuery({
    queryKey: ["checkin", checkinId],
    queryFn: () => api.checkin(checkinId),
  });
  const [text, setText] = useState("");
  const [live, setLive] = useState<Item[]>([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const bottom = useRef<HTMLDivElement>(null);

  const history: Item[] = [
    ...(detail.data?.messages ?? []).map(
      (m): Item => ({ kind: "msg", key: `m${m.id}`, role: m.role, text: m.text, at: m.created_at }),
    ),
    ...(detail.data?.actions ?? []).map(
      (a): Item => ({ kind: "action", key: `a${a.id}`, action: a, at: a.created_at }),
    ),
  ].sort((a, b) => a.at.localeCompare(b.at));
  const tail = live[live.length - 1];
  const streaming = tail?.kind === "msg" && tail.role === "assistant";
  const items = busy || live.length ? [...history, ...live] : history;

  // biome-ignore lint/correctness/useExhaustiveDependencies: scroll when the transcript grows
  useEffect(() => {
    bottom.current?.scrollIntoView({ block: "nearest", behavior: "smooth" });
  }, [items.length, live]);

  async function send(msg: string) {
    setBusy(true);
    setError(null);
    const now = new Date().toISOString();
    setLive([{ kind: "msg", key: "live-user", role: "user", text: msg, at: now }]);
    try {
      await chat(checkinId, msg, (ev) => {
        if (ev.event === "text") {
          setLive((cur) => {
            const last = cur[cur.length - 1];
            if (last?.kind === "msg" && last.role === "assistant") {
              return [...cur.slice(0, -1), { ...last, text: last.text + ev.data.text }];
            }
            return [
              ...cur,
              {
                kind: "msg",
                key: `live-${cur.length}`,
                role: "assistant",
                text: ev.data.text,
                at: now,
              },
            ];
          });
        } else if (ev.event === "action") {
          setLive((cur) => [
            ...cur,
            { kind: "action", key: `live-a${ev.data.id}`, action: ev.data, at: now },
          ]);
          // The curator changed something: refresh the rest of the page.
          qc.invalidateQueries({ queryKey: ["today"] });
          qc.invalidateQueries({ queryKey: ["steps"] });
        } else if (ev.event === "error") {
          setError(ev.data.error);
        }
      });
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
    await qc.invalidateQueries({ queryKey: ["checkin", checkinId] });
    await qc.invalidateQueries({ queryKey: ["today"] });
    setLive([]);
    setBusy(false);
  }

  return (
    <div className="space-y-3">
      <div className="max-h-[60vh] space-y-3 overflow-y-auto" aria-live="polite">
        {items.length === 0 && <p className="text-sm text-fg-muted">{prompt}</p>}
        {items.map((it) =>
          it.kind === "msg" ? (
            <div
              key={it.key}
              className={
                it.role === "user"
                  ? "ml-8 rounded-2xl rounded-br-md bg-accent-600/15 px-3 py-2 text-sm"
                  : "mr-4 text-sm leading-relaxed"
              }
            >
              <Markdownish text={it.text} />
            </div>
          ) : (
            <ActionChip key={it.key} action={it.action} />
          ),
        )}
        {busy && !streaming && <p className="text-sm text-fg-muted">thinking…</p>}
        <div ref={bottom} />
      </div>
      <ErrorText error={error} />
      <form
        className="flex gap-2"
        onSubmit={(e) => {
          e.preventDefault();
          const msg = text.trim();
          if (msg && !busy) {
            setText("");
            void send(msg);
          }
        }}
      >
        <textarea
          value={text}
          onChange={(e) => setText(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter" && !e.shiftKey) {
              e.preventDefault();
              e.currentTarget.form?.requestSubmit();
            }
          }}
          rows={1}
          placeholder="Reply to your curator…"
          aria-label="Message"
          className={`${inputClass} resize-none`}
        />
        <Button type="submit" disabled={busy || !text.trim()}>
          Send
        </Button>
      </form>
    </div>
  );
}

function ActionChip({ action }: { action: Action }) {
  const qc = useQueryClient();
  const [undone, setUndone] = useState(action.undone_at !== null);
  const undo = useMutation({
    mutationFn: () => api.undo(action.id),
    onSuccess: () => {
      setUndone(true);
      qc.invalidateQueries();
    },
  });
  return (
    <div
      className="flex items-center gap-2 rounded-xl border border-dashed border-border px-3 py-1.5 text-xs"
      data-testid="action-chip"
    >
      <span aria-hidden>✎</span>
      <span className={`flex-1 ${undone ? "text-fg-muted line-through" : ""}`}>
        {action.summary}
      </span>
      {undone ? (
        <span className="text-fg-muted">undone</span>
      ) : (
        <button
          type="button"
          onClick={() => undo.mutate()}
          disabled={undo.isPending}
          className="rounded-lg px-2 py-0.5 font-medium text-accent-500 hover:bg-surface-2"
          aria-label={`Undo: ${action.summary}`}
        >
          Undo
        </button>
      )}
      {undo.error && <span className="text-danger">{undo.error.message}</span>}
    </div>
  );
}
