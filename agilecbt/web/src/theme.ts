// Theme is three independent choices, each stored per browser and applied as a
// data attribute on <html> (see index.css for what each value means):
//
//   data-theme  light | dark        resolved from the mode (system follows the OS)
//   data-style  warm | drafting     neutrals, type, corners
//   data-accent apricot | signal…   the one hue used for actions and focus
//
// index.html repeats the read-and-apply step inline so the first paint is
// already themed; keep the keys and defaults here in sync with it.

export type ThemeMode = "system" | "light" | "dark";
export type ThemeStyle = "warm" | "drafting";
export type ThemeAccent = "apricot" | "signal" | "sage" | "sky" | "violet";

export const THEME_MODES: { value: ThemeMode; label: string }[] = [
  { value: "system", label: "System" },
  { value: "light", label: "Light" },
  { value: "dark", label: "Dark" },
];

export const THEME_STYLES: { value: ThemeStyle; label: string }[] = [
  { value: "warm", label: "Warm" },
  { value: "drafting", label: "Drafting" },
];

export const THEME_ACCENTS: { value: ThemeAccent; label: string }[] = [
  { value: "apricot", label: "Apricot" },
  { value: "signal", label: "Signal red" },
  { value: "sage", label: "Sage" },
  { value: "sky", label: "Sky" },
  { value: "violet", label: "Violet" },
];

export type Theme = { mode: ThemeMode; style: ThemeStyle; accent: ThemeAccent };

export const DEFAULT_THEME: Theme = { mode: "system", style: "warm", accent: "apricot" };

const KEYS: Record<keyof Theme, string> = {
  mode: "agilecbt.themeMode",
  style: "agilecbt.themeStyle",
  accent: "agilecbt.themeAccent",
};

const ALLOWED: { [K in keyof Theme]: readonly Theme[K][] } = {
  mode: THEME_MODES.map((m) => m.value),
  style: THEME_STYLES.map((s) => s.value),
  accent: THEME_ACCENTS.map((a) => a.value),
};

function read<K extends keyof Theme>(key: K): Theme[K] {
  try {
    const v = localStorage.getItem(KEYS[key]) as Theme[K] | null;
    if (v && ALLOWED[key].includes(v)) return v;
  } catch {
    // private mode: fall through to the default
  }
  return DEFAULT_THEME[key];
}

export function loadTheme(): Theme {
  return { mode: read("mode"), style: read("style"), accent: read("accent") };
}

export function saveTheme(theme: Theme) {
  for (const key of Object.keys(KEYS) as (keyof Theme)[]) {
    try {
      localStorage.setItem(KEYS[key], theme[key]);
    } catch {
      // private mode: the theme still applies for this visit
    }
  }
  applyTheme(theme);
}

export function applyTheme({ mode, style, accent }: Theme) {
  const root = document.documentElement;
  const dark =
    mode === "dark" || (mode === "system" && matchMedia("(prefers-color-scheme: dark)").matches);
  root.dataset.theme = dark ? "dark" : "light";
  root.dataset.style = style;
  root.dataset.accent = accent;
  // Keep the browser chrome (mobile address bar, PWA title bar) on the page ground.
  const bg = getComputedStyle(root).getPropertyValue("--color-bg").trim();
  if (bg) document.querySelector('meta[name="theme-color"]')?.setAttribute("content", bg);
}
