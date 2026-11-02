const ACTIVE_BOARD_KEY = "kanban.activeBoardId";

export function readActiveBoardId(): number | null {
  try {
    const raw = localStorage.getItem(ACTIVE_BOARD_KEY);
    if (!raw) return null;
    const id = Number(raw);
    return Number.isInteger(id) && id > 0 ? id : null;
  } catch {
    return null;
  }
}

export function writeActiveBoardId(id: number): void {
  try {
    localStorage.setItem(ACTIVE_BOARD_KEY, String(id));
  } catch {
    // ignore quota / disabled storage
  }
}

const RELEASES_VIEW_ALL_KEY = "kanban.releases.viewAll";

export function readReleasesViewAll(): boolean {
  try {
    return localStorage.getItem(RELEASES_VIEW_ALL_KEY) === "true";
  } catch {
    return false;
  }
}

export function writeReleasesViewAll(viewAll: boolean): void {
  try {
    localStorage.setItem(RELEASES_VIEW_ALL_KEY, String(viewAll));
  } catch {
    // ignore quota / disabled storage
  }
}

const RELEASES_FILTER_KEY = "kanban.releases.filter";

// Only the subtrees toggled by hand, so the rest keep following the default.
export type ReleasesFilter = Record<string, boolean>;

export function readReleasesFilter(): ReleasesFilter {
  try {
    const parsed: unknown = JSON.parse(localStorage.getItem(RELEASES_FILTER_KEY) ?? "{}");
    return parsed && typeof parsed === "object" ? (parsed as ReleasesFilter) : {};
  } catch {
    return {};
  }
}

export function writeReleasesFilter(filter: ReleasesFilter): void {
  try {
    localStorage.setItem(RELEASES_FILTER_KEY, JSON.stringify(filter));
  } catch {
    // ignore quota / disabled storage
  }
}

// Per-session remembered tab (e.g. "agent", "diff"). One key per session id,
// so these must be cleaned up when a session is dropped (archive/delete) or
// they accumulate forever — see clearSessionTab, called from the store's
// orphan-session reconciliation in fetchBoardStructure.
const SESSION_TAB_KEY_PREFIX = "sessionPane.tab.";

export function readSessionTab(sessionId: number): string | null {
  try {
    return localStorage.getItem(SESSION_TAB_KEY_PREFIX + sessionId);
  } catch {
    return null;
  }
}

export function writeSessionTab(sessionId: number, tab: string): void {
  try {
    localStorage.setItem(SESSION_TAB_KEY_PREFIX + sessionId, tab);
  } catch {
    // ignore quota / disabled storage
  }
}

export function clearSessionTab(sessionId: number): void {
  try {
    localStorage.removeItem(SESSION_TAB_KEY_PREFIX + sessionId);
  } catch {
    // ignore disabled storage
  }
}
