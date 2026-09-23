import type { Action } from "./types";

export const ACTIONS: Action[] = [
  {
    id: "tab.next",
    label: "Next tab",
    group: "Session",
    defaultBinding: { ctrl: true, alt: true, shift: false, meta: false, key: "PageDown" },
  },
  {
    id: "tab.prev",
    label: "Previous tab",
    group: "Session",
    defaultBinding: { ctrl: true, alt: true, shift: false, meta: false, key: "PageUp" },
  },
  {
    id: "session.fullscreen",
    label: "Toggle fullscreen",
    group: "Session",
    defaultBinding: { ctrl: false, alt: false, shift: false, meta: false, key: "F" },
  },
  {
    id: "board.create",
    label: "New board",
    group: "Navigation",
    defaultBinding: { ctrl: false, alt: false, shift: false, meta: false, key: "B" },
  },
  {
    id: "board.next",
    label: "Next board",
    group: "Navigation",
    defaultBinding: { ctrl: true, alt: true, shift: true, meta: false, key: "PageDown" },
  },
  {
    id: "board.prev",
    label: "Previous board",
    group: "Navigation",
    defaultBinding: { ctrl: true, alt: true, shift: true, meta: false, key: "PageUp" },
  },
  {
    id: "ticket.prev",
    label: "Previous ticket",
    group: "Navigation",
    defaultBinding: { ctrl: true, alt: true, shift: false, meta: false, key: "ArrowUp" },
  },
  {
    id: "ticket.next",
    label: "Next ticket",
    group: "Navigation",
    defaultBinding: { ctrl: true, alt: true, shift: false, meta: false, key: "ArrowDown" },
  },
  {
    id: "column.prev",
    label: "Previous column",
    group: "Navigation",
    defaultBinding: { ctrl: true, alt: true, shift: false, meta: false, key: "ArrowLeft" },
  },
  {
    id: "column.next",
    label: "Next column",
    group: "Navigation",
    defaultBinding: { ctrl: true, alt: true, shift: false, meta: false, key: "ArrowRight" },
  },
  {
    id: "ticket.create",
    label: "New ticket",
    group: "Ticket",
    defaultBinding: { ctrl: false, alt: false, shift: false, meta: false, key: "N" },
  },
  {
    id: "ticket.archive",
    label: "Archive ticket",
    group: "Ticket",
    defaultBinding: { ctrl: false, alt: false, shift: false, meta: false, key: "A" },
  },
];

export const ACTIONS_BY_ID: Record<string, Action> = Object.fromEntries(
  ACTIONS.map((a) => [a.id, a]),
);
