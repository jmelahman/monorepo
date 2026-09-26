import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { type ReactNode, useEffect, useRef, useState } from "react";
import { type Action, api, chat } from "@/api/client";
import { Button, ErrorText, inputClass, Markdownish, Shimmer } from "@/components/ui";

type Item =
  | { kind: "msg"; key: string; role: "user" | "assistant"; text: string; at: string }
  | { kind: "action"; key: string; action: Action; at: string };

// Chat is a coach conversation (a check-in, or a topic chat like the
// Roadmap's). Replies stream in; any change the coach makes shows up as a
// chip with Undo.
//
// With checkinId null, start() creates the conversation on the first send
// and gets that first message. greeting, if set, is shown as the coach's
// opening line while the conversation is empty (the owner saves it in
// start), with suggestions as quick replies; otherwise prompt and
// suggestions are shown as openers. fill makes the chat take its parent's
// full height, with only the transcript scrolling. readOnly shows the
// transcript without a composer (no coach available).
export default function Chat({
  checkinId,
  start,
  greeting,
  prompt,
  suggestions = [],
  placeholder = "Reply to your coach…",
  fill = false,
  readOnly = false,
}: {
  checkinId: number | null;
  start?: (first: string) => Promise<number>;
  greeting?: string;
  prompt?: string;
  suggestions?: string[];
  placeholder?: string;
  fill?: boolean;
  readOnly?: boolean;
}) {
  const qc = useQueryClient();
  const detail = useQuery({
    queryKey: ["checkin", checkinId],
    queryFn: () => api.checkin(checkinId as number),
    enabled: checkinId !== null,
  });
  const [live, setLive] = useState<Item[]>([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  // The transcript as it was when a send began. While a reply streams, the
  // saved copy may refetch (or first load) with the same messages as live.
  const [frozen, setFrozen] = useState<Item[] | null>(null);
  const bottom = useRef<HTMLDivElement>(null);

  const history: Item[] = [
    ...(detail.data?.messages ?? []).map(
      (m): Item => ({ kind: "msg", key: `m${m.id}`, role: m.role, text: m.text, at: m.created_at }),
    ),
    ...(detail.data?.actions ?? []).map(
      (a): Item => ({ kind: "action", key: `a${a.id}`, action: a, at: a.created_at }),
    ),
  ].sort((a, b) => a.at.localeCompare(b.at));
  const shown = frozen ?? history;
  const tail = live[live.length - 1];
  const streaming = tail?.kind === "msg" && tail.role === "assistant";
  const items = [...shown, ...live];
  const opening = !!greeting && shown.length === 0;

  // biome-ignore lint/correctness/useExhaustiveDependencies: scroll when the transcript grows
  useEffect(() => {
    bottom.current?.scrollIntoView({ block: "nearest", behavior: "smooth" });
  }, [items.length, live]);

  const waiting = busy && !streaming;

  async function send(msg: string) {
    setBusy(true);
    setError(null);
    setFrozen(history);
    const now = new Date().toISOString();
    setLive([{ kind: "msg", key: "live-user", role: "user", text: msg, at: now }]);
    let id = checkinId;
    try {
      id ??= start ? await start(msg) : null;
      if (id === null) throw new Error("no conversation to send to");
      await chat(id, msg, (ev) => {
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
          // The coach changed something: refresh the rest of the page, but
          // not the transcript (or which one to show), which would duplicate
          // the live messages.
          qc.invalidateQueries({
            predicate: (q) => q.queryKey[0] !== "checkin" && q.queryKey[0] !== "conversation",
          });
        } else if (ev.event === "error") {
          setError(ev.data.error);
        }
      });
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
    // Load the saved transcript before dropping the live copy, so a newly
    // started conversation doesn't flash empty.
    if (id !== null) {
      const cid = id;
      await qc
        .fetchQuery({ queryKey: ["checkin", cid], queryFn: () => api.checkin(cid), staleTime: 0 })
        .catch(() => {});
    }
    await qc.invalidateQueries({ predicate: (q) => q.queryKey[0] !== "checkin" });
    setLive([]);
    setFrozen(null);
    setBusy(false);
  }

  return (
    <div className={fill ? "flex h-full min-h-0 flex-col gap-3" : "space-y-3"}>
      <div
        className={`space-y-3 overflow-y-auto ${fill ? "min-h-0 flex-1" : "max-h-[60vh]"}`}
        aria-live="polite"
      >
        {opening && (
          <Bubble from="assistant">
            <p className="text-base font-medium">{greeting}</p>
          </Bubble>
        )}
        {!greeting && items.length === 0 && !busy && (
          <div className="space-y-3">
            {prompt && <p className="text-sm text-fg-muted">{prompt}</p>}
            {suggestions.length > 0 && (
              <div className="flex flex-wrap gap-2">
                {suggestions.map((sg) => (
                  <Button key={sg} variant="soft" className="text-left" onClick={() => send(sg)}>
                    {sg}
                  </Button>
                ))}
              </div>
            )}
          </div>
        )}
        {items.map((it) =>
          it.kind === "msg" ? (
            <Bubble key={it.key} from={it.role}>
              <Markdownish text={it.text} />
            </Bubble>
          ) : (
            <ActionChip key={it.key} action={it.action} />
          ),
        )}
        {waiting && (
          <p className="text-sm">
            <Shimmer>Thinking…</Shimmer>
          </p>
        )}
        <div ref={bottom} />
      </div>
      <ErrorText error={error} />
      {!readOnly && (
        <Composer
          busy={busy}
          placeholder={placeholder}
          onSend={send}
          quickReplies={opening && !busy ? suggestions : []}
        />
      )}
    </div>
  );
}

// Bubble is one chat message: the person's on the right, the coach's as
// plain text on the left.
export function Bubble({ from, children }: { from: "user" | "assistant"; children: ReactNode }) {
  return (
    <div
      className={
        from === "user"
          ? "ml-8 rounded-2xl rounded-br-md bg-accent-600/15 px-3 py-2 text-sm"
          : "mr-4 text-sm leading-relaxed"
      }
    >
      {children}
    </div>
  );
}

// Composer is the one place to answer: a message box, plus quick replies for
// when typing is too much. Enter sends; Shift+Enter adds a line.
export function Composer({
  onSend,
  busy = false,
  placeholder,
  quickReplies = [],
}: {
  onSend: (text: string) => void;
  busy?: boolean;
  placeholder: string;
  quickReplies?: string[];
}) {
  const [text, setText] = useState("");
  return (
    <div className="space-y-2">
      {quickReplies.length > 0 && (
        <div className="flex flex-wrap justify-end gap-2">
          {quickReplies.map((r) => (
            <Button
              key={r}
              variant="soft"
              className="px-3 py-1 text-sm"
              disabled={busy}
              onClick={() => onSend(r)}
            >
              {r}
            </Button>
          ))}
        </div>
      )}
      <form
        className="flex gap-2"
        onSubmit={(e) => {
          e.preventDefault();
          const msg = text.trim();
          if (msg && !busy) {
            setText("");
            onSend(msg);
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
          placeholder={placeholder}
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
          className="rounded-lg px-2 py-0.5 font-medium text-accent-ink hover:bg-surface-2"
          aria-label={`Undo: ${action.summary}`}
        >
          Undo
        </button>
      )}
      {undo.error && <span className="text-danger">{undo.error.message}</span>}
    </div>
  );
}
