import { makePreferenceHook } from "./makePreferenceHook";

export type TerminalOrientation = "vertical" | "horizontal";

const { use, set } = makePreferenceHook<TerminalOrientation>({
  key: "sessionPane.orientation",
  event: "kanban:terminal-orientation",
  parse: (raw) => (raw === "horizontal" ? "horizontal" : "vertical"),
});

export const useTerminalOrientation = use;
export const setTerminalOrientation = set;
