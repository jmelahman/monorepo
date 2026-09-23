import { type KeyboardEvent, useEffect, useRef, useState } from "react";
import { PencilIcon, TrashIcon } from "@/icons";

// The side a comment is anchored to. Mirrors @pierre/diffs' AnnotationSide
// ("additions" = the new/right column, "deletions" = the old/left column) so a
// comment captured from a click maps straight onto the right column of code.
export type ReviewSide = "additions" | "deletions";

// Where a review comment is attached. `startLine`/`endLine` are file line
// numbers on `side`; they're equal for a single-line comment. The inline box is
// rendered after `endLine` (see DiffPanel's annotation wiring).
export type ReviewAnchor = {
  path: string;
  side: ReviewSide;
  startLine: number;
  endLine: number;
};

// One review comment. `snippet` is the code that was under the comment when it
// was written (line-number prefixed), captured at save time so the copied
// review is self-contained and survives later edits to the file.
export type ReviewComment = {
  id: string;
  anchor: ReviewAnchor;
  body: string;
  snippet: string;
  createdAt: number;
};

export function lineRangeLabel(anchor: ReviewAnchor): string {
  return anchor.startLine === anchor.endLine
    ? `${anchor.startLine}`
    : `${anchor.startLine}-${anchor.endLine}`;
}

// InlineCommentBox renders a single review comment — either an existing one
// (view mode, with edit/delete on hover) or a brand-new draft (edit mode). The
// library projects it into a light-DOM slot inserted after the anchored line, so
// it styles with the app's own Tailwind tokens like any other element.
export function InlineCommentBox({
  comment,
  anchor,
  onSave,
  onDelete,
  onCancel,
}: {
  comment: ReviewComment | null;
  anchor: ReviewAnchor;
  onSave: (body: string) => void;
  onDelete?: () => void;
  onCancel?: () => void;
}) {
  // A null comment is a fresh draft, so it opens straight into edit mode.
  const [editing, setEditing] = useState(comment === null);
  const [text, setText] = useState(comment?.body ?? "");
  const textareaRef = useRef<HTMLTextAreaElement | null>(null);

  useEffect(() => {
    if (editing) textareaRef.current?.focus();
  }, [editing]);

  const save = () => {
    const body = text.trim();
    if (!body) {
      // Saving an empty draft is the same as cancelling it.
      onCancel?.();
      return;
    }
    onSave(body);
    setEditing(false);
  };

  const cancel = () => {
    if (comment === null) {
      onCancel?.();
      return;
    }
    setText(comment.body);
    setEditing(false);
  };

  const onKeyDown = (e: KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) {
      e.preventDefault();
      save();
    } else if (e.key === "Escape") {
      e.preventDefault();
      cancel();
    }
  };

  return (
    <div className="my-1 ml-12 mr-4 rounded border border-border bg-surface-2 text-sm text-fg shadow-sm">
      <div className="flex items-center gap-2 border-b border-border px-3 py-1 text-[11px] text-fg-muted">
        <span className="font-mono">
          {anchor.side === "deletions" ? "old" : "new"} L{lineRangeLabel(anchor)}
        </span>
        {comment !== null && !editing && (
          <div className="ml-auto flex items-center gap-1">
            <button
              type="button"
              onClick={() => setEditing(true)}
              title="Edit comment"
              aria-label="Edit comment"
              className="rounded p-0.5 hover:bg-surface-3 hover:text-fg"
            >
              <PencilIcon size={12} />
            </button>
            {onDelete && (
              <button
                type="button"
                onClick={onDelete}
                title="Delete comment"
                aria-label="Delete comment"
                className="rounded p-0.5 hover:bg-surface-3 hover:text-danger"
              >
                <TrashIcon size={12} />
              </button>
            )}
          </div>
        )}
      </div>
      {editing ? (
        <div className="p-2">
          <textarea
            ref={textareaRef}
            value={text}
            onChange={(e) => setText(e.target.value)}
            onKeyDown={onKeyDown}
            placeholder="Leave a review comment…"
            className="min-h-[4rem] w-full resize-y rounded bg-surface px-2 py-1 font-sans text-sm outline-none ring-1 ring-border focus:ring-accent-500"
          />
          <div className="mt-2 flex items-center justify-end gap-2">
            <button
              type="button"
              onClick={cancel}
              className="rounded px-2 py-0.5 text-xs text-fg-muted hover:bg-surface-3 hover:text-fg"
            >
              Cancel
            </button>
            <button
              type="button"
              onClick={save}
              className="rounded bg-accent-600 px-2 py-0.5 text-xs text-white hover:bg-accent-700"
            >
              {comment === null ? "Add comment" : "Save"}
            </button>
          </div>
        </div>
      ) : (
        <p className="whitespace-pre-wrap px-3 py-2 font-sans">{comment?.body}</p>
      )}
    </div>
  );
}
