import { useEffect, useMemo, useState } from "react";
import { ACTIONS } from "./registry";
import { formatBinding } from "./serialize";
import {
  KEYBINDINGS_EVENT,
  getBinding,
  resetAll as resetEverything,
  resetBinding as resetOne,
  setBinding as writeBinding,
} from "./storage";
import type { Action, ActionId, Binding } from "./types";

const STORAGE_KEY = "kanban.keybindings";

export interface UseKeybindings {
  actions: Action[];
  bindings: Record<ActionId, Binding | null>;
  conflicts: Set<ActionId>;
  setBinding: (id: ActionId, binding: Binding | null) => void;
  resetBinding: (id: ActionId) => void;
  resetAll: () => void;
}

export function useKeybindings(): UseKeybindings {
  const [tick, setTick] = useState(0);

  useEffect(() => {
    const onChange = () => setTick((t) => t + 1);
    const onStorage = (e: StorageEvent) => {
      if (e.key === STORAGE_KEY) onChange();
    };
    window.addEventListener(KEYBINDINGS_EVENT, onChange);
    window.addEventListener("storage", onStorage);
    return () => {
      window.removeEventListener(KEYBINDINGS_EVENT, onChange);
      window.removeEventListener("storage", onStorage);
    };
  }, []);

  // `tick` is a re-key signal driven by storage/event subscriptions; it isn't
  // read inside, but bumping it is how we force a fresh read of `getBinding`.
  // biome-ignore lint/correctness/useExhaustiveDependencies: see above.
  const bindings = useMemo(() => {
    const map: Record<ActionId, Binding | null> = {};
    for (const action of ACTIONS) {
      map[action.id] = getBinding(action.id);
    }
    return map;
  }, [tick]);

  const conflicts = useMemo(() => {
    const seen = new Map<string, string[]>();
    for (const action of ACTIONS) {
      const b = bindings[action.id];
      if (!b) continue;
      const key = formatBinding(b);
      const list = seen.get(key) ?? [];
      list.push(action.id);
      seen.set(key, list);
    }
    const out = new Set<ActionId>();
    for (const list of seen.values()) {
      if (list.length > 1) for (const id of list) out.add(id);
    }
    return out;
  }, [bindings]);

  return {
    actions: ACTIONS,
    bindings,
    conflicts,
    setBinding: writeBinding,
    resetBinding: resetOne,
    resetAll: resetEverything,
  };
}
