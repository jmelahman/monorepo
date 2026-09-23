import { makePreferenceHook } from "@/hooks/makePreferenceHook";

// Persisted state for the developer toolbar widget: whether it's open, which
// screen corner it pins to, and which metric sections are shown. Stored as one
// JSON blob in localStorage and broadcast cross-tab via makePreferenceHook.

export type DevToolbarCorner = "br" | "bl" | "tr" | "tl";

export type DevToolbarSectionKey = "fps" | "memory" | "domReact" | "appInternals";

export type DevToolbarPrefs = {
  open: boolean;
  corner: DevToolbarCorner;
  sections: Record<DevToolbarSectionKey, boolean>;
};

const DEFAULTS: DevToolbarPrefs = {
  open: false,
  corner: "br",
  sections: { fps: true, memory: true, domReact: true, appInternals: true },
};

// Clockwise cycle order used by the corner button.
export const DEV_TOOLBAR_CORNERS: readonly DevToolbarCorner[] = ["br", "bl", "tl", "tr"];

// parse must never throw (makePreferenceHook contract) and merges onto DEFAULTS
// so an older stored blob missing a newer section key still gets a sane value.
function parse(raw: string | null): DevToolbarPrefs {
  if (!raw) return DEFAULTS;
  try {
    const obj = JSON.parse(raw) as Partial<DevToolbarPrefs>;
    const corner = DEV_TOOLBAR_CORNERS.includes(obj.corner as DevToolbarCorner)
      ? (obj.corner as DevToolbarCorner)
      : DEFAULTS.corner;
    return {
      open: typeof obj.open === "boolean" ? obj.open : DEFAULTS.open,
      corner,
      sections: { ...DEFAULTS.sections, ...(obj.sections ?? {}) },
    };
  } catch {
    return DEFAULTS;
  }
}

const pref = makePreferenceHook<DevToolbarPrefs>({
  key: "dev.toolbar.prefs",
  event: "dev-toolbar-prefs",
  parse,
  serialize: JSON.stringify,
});

export const useDevToolbarPrefs = pref.use;
export const setDevToolbarPrefs = pref.set;
