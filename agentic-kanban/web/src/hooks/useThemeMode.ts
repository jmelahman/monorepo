import { useEffect, useState } from "react";

export type ThemeMode = "system" | "light" | "dark";
export type ResolvedTheme = "light" | "dark";

const STORAGE_KEY = "kanban.themeMode";
const EVENT = "kanban:theme-mode";
export const APPEARANCE_EVENT = "kanban:appearance-changed";

function readMode(): ThemeMode {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (raw === "light" || raw === "dark" || raw === "system") return raw;
    return "system";
  } catch {
    return "system";
  }
}

function prefersDark(): boolean {
  return typeof matchMedia !== "undefined" && matchMedia("(prefers-color-scheme: dark)").matches;
}

function resolve(mode: ThemeMode): ResolvedTheme {
  if (mode === "system") return prefersDark() ? "dark" : "light";
  return mode;
}

function apply(mode: ThemeMode): void {
  document.documentElement.dataset.theme = resolve(mode);
}

export function setThemeMode(value: ThemeMode): void {
  try {
    localStorage.setItem(STORAGE_KEY, value);
  } catch {
    // ignore
  }
  apply(value);
  window.dispatchEvent(new CustomEvent<ThemeMode>(EVENT, { detail: value }));
  window.dispatchEvent(new Event(APPEARANCE_EVENT));
}

export function useThemeMode(): { mode: ThemeMode; resolved: ResolvedTheme } {
  const [mode, setMode] = useState<ThemeMode>(() => readMode());
  const [resolved, setResolved] = useState<ResolvedTheme>(() => resolve(readMode()));

  useEffect(() => {
    apply(mode);
    setResolved(resolve(mode));
  }, [mode]);

  useEffect(() => {
    const onCustom = (e: Event) => setMode((e as CustomEvent<ThemeMode>).detail);
    const onStorage = (e: StorageEvent) => {
      if (e.key === STORAGE_KEY) setMode(readMode());
    };
    const mq = matchMedia("(prefers-color-scheme: dark)");
    const onMedia = () => {
      if (mode === "system") {
        apply("system");
        setResolved(resolve("system"));
        window.dispatchEvent(new Event(APPEARANCE_EVENT));
      }
    };
    window.addEventListener(EVENT, onCustom);
    window.addEventListener("storage", onStorage);
    mq.addEventListener("change", onMedia);
    return () => {
      window.removeEventListener(EVENT, onCustom);
      window.removeEventListener("storage", onStorage);
      mq.removeEventListener("change", onMedia);
    };
  }, [mode]);

  return { mode, resolved };
}
