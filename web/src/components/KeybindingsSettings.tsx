import { useState } from "react";
import { bindingFromEvent, formatBinding } from "@/keys/serialize";
import type { Action } from "@/keys/types";
import { useKeybindings } from "@/keys/useKeybindings";
import { Button } from "./Button";

export function KeybindingsSettings() {
  const { actions, bindings, conflicts, setBinding, resetBinding, resetAll } = useKeybindings();
  const [recording, setRecording] = useState<string | null>(null);

  const groups = new Map<string, Action[]>();
  for (const a of actions) {
    const list = groups.get(a.group) ?? [];
    list.push(a);
    groups.set(a.group, list);
  }

  return (
    <fieldset className="flex flex-col gap-2">
      {[...groups.entries()].map(([group, list]) => (
        <div key={group} className="flex flex-col gap-1.5">
          <div className="text-xs text-fg-muted">{group}</div>
          {list.map((a) => {
            const binding = bindings[a.id];
            const isRecording = recording === a.id;
            const conflict = conflicts.has(a.id);
            return (
              <div key={a.id} className="flex items-center gap-2">
                <span className="flex-1 text-sm">{a.label}</span>
                {conflict && !isRecording && (
                  <span
                    className="rounded bg-amber-900 px-1 py-0.5 text-[10px] uppercase tracking-wide text-amber-100"
                    title="Another action uses the same shortcut"
                  >
                    conflict
                  </span>
                )}
                <button
                  type="button"
                  data-keybind-capture={isRecording ? "true" : undefined}
                  onClick={() => setRecording(isRecording ? null : a.id)}
                  onBlur={() => isRecording && setRecording(null)}
                  onKeyDown={(e) => {
                    if (!isRecording) return;
                    e.preventDefault();
                    e.stopPropagation();
                    if (e.key === "Escape") {
                      setRecording(null);
                      return;
                    }
                    if (e.key === "Backspace" || e.key === "Delete") {
                      setBinding(a.id, null);
                      setRecording(null);
                      return;
                    }
                    const next = bindingFromEvent(e.nativeEvent);
                    if (!next) return;
                    setBinding(a.id, next);
                    setRecording(null);
                  }}
                  className={`min-w-[10rem] rounded border bg-surface px-2 py-1 text-left font-mono text-xs ${
                    isRecording
                      ? "border-accent-700 ring-1 ring-accent-700"
                      : conflict
                        ? "border-amber-700"
                        : "border-border"
                  }`}
                >
                  {isRecording ? "press a key…" : binding ? formatBinding(binding) : "—"}
                </button>
                <Button type="button" variant="ghost" size="sm" onClick={() => resetBinding(a.id)}>
                  reset
                </Button>
              </div>
            );
          })}
        </div>
      ))}
      <div className="flex justify-end">
        <Button type="button" variant="ghost" size="sm" onClick={resetAll}>
          reset all to defaults
        </Button>
      </div>
      <span className="text-xs text-fg-muted">
        Click a shortcut and press the new key combination. Backspace clears the binding; Esc
        cancels.
      </span>
    </fieldset>
  );
}
