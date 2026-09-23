import { makePreferenceHook } from "./makePreferenceHook";
import { APPEARANCE_EVENT } from "./useThemeMode";

export const ACCENTS = [
  "red",
  "orange",
  "amber",
  "green",
  "teal",
  "blue",
  "indigo",
  "purple",
  "pink",
] as const;

export type Accent = (typeof ACCENTS)[number];

const { use, set } = makePreferenceHook<Accent>({
  key: "kanban.accent",
  event: "kanban:accent",
  parse: (raw) => (raw && (ACCENTS as readonly string[]).includes(raw) ? (raw as Accent) : "red"),
  apply: (value) => {
    document.documentElement.dataset.accent = value;
  },
  onAfterSet: () => window.dispatchEvent(new Event(APPEARANCE_EVENT)),
});

export const useAccent = use;
export const setAccent = set;
