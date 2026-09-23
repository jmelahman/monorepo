import { useEffect, useState } from "react";

export interface PreferenceHookConfig<T> {
  /** localStorage key. */
  key: string;
  /** Custom event name dispatched after `set` and listened to inside the hook. */
  event: string;
  /** Parse a raw localStorage value (or null when unset) into T. Must never throw. */
  parse: (raw: string | null) => T;
  /** Serialize T to the string stored in localStorage. Defaults to `String(value)`. */
  serialize?: (value: T) => string;
  /** Optional DOM side effect applied when the value mounts, updates, or is set. */
  apply?: (value: T) => void;
  /** Fires once at the end of `set`, after the main custom event is dispatched. */
  onAfterSet?: () => void;
}

/**
 * makePreferenceHook builds a (use, set) pair around a localStorage-backed
 * preference that broadcasts changes via a CustomEvent and survives cross-tab
 * `storage` updates. Used by useAccent, useContrast, and useTerminalOrientation.
 */
export function makePreferenceHook<T>(cfg: PreferenceHookConfig<T>): {
  use: () => T;
  set: (value: T) => void;
} {
  const { key, event, parse, serialize = String, apply, onAfterSet } = cfg;

  const read = (): T => {
    try {
      return parse(localStorage.getItem(key));
    } catch {
      return parse(null);
    }
  };

  const set = (value: T): void => {
    try {
      localStorage.setItem(key, serialize(value));
    } catch {
      // ignore quota / disabled storage
    }
    apply?.(value);
    window.dispatchEvent(new CustomEvent<T>(event, { detail: value }));
    onAfterSet?.();
  };

  const use = (): T => {
    const [value, setState] = useState<T>(() => read());
    useEffect(() => {
      apply?.(value);
    }, [value]);
    useEffect(() => {
      const onCustom = (e: Event) => setState((e as CustomEvent<T>).detail);
      const onStorage = (e: StorageEvent) => {
        if (e.key === key) setState(read());
      };
      window.addEventListener(event, onCustom);
      window.addEventListener("storage", onStorage);
      return () => {
        window.removeEventListener(event, onCustom);
        window.removeEventListener("storage", onStorage);
      };
    }, []);
    return value;
  };

  return { use, set };
}
