import { makePreferenceHook } from "./makePreferenceHook";
import { APPEARANCE_EVENT } from "./useThemeMode";

export type Contrast = "normal" | "high";

const { use, set } = makePreferenceHook<Contrast>({
  key: "kanban.contrast",
  event: "kanban:contrast",
  parse: (raw) => (raw === "high" ? "high" : "normal"),
  apply: (value) => {
    if (value === "high") document.documentElement.dataset.contrast = "high";
    else delete document.documentElement.dataset.contrast;
  },
  onAfterSet: () => window.dispatchEvent(new Event(APPEARANCE_EVENT)),
});

export const useContrast = use;
export const setContrast = set;
