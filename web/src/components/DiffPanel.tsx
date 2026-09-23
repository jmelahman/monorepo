import {
  type DiffLineAnnotation,
  type DiffsThemeNames,
  getFiletypeFromFileName,
  parseDiffFromFile,
  parsePatchFiles,
  preloadHighlighter,
  type FileDiffMetadata,
  type SelectedLineRange,
  type SupportedLanguages,
} from "@pierre/diffs";
import { File, FileDiff } from "@pierre/diffs/react";
import { useQuery } from "@tanstack/react-query";
import {
  memo,
  type ReactNode,
  useCallback,
  useDeferredValue,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { api, type Session, type Ticket } from "@/api/client";
import { queryKeys } from "@/api/keys";
import { useContrast } from "@/hooks/useContrast";
import { useCopyFeedback } from "@/hooks/useCopyFeedback";
import { useSessionDiff } from "@/hooks/useSessionDiff";
import { useThemeMode } from "@/hooks/useThemeMode";
import { CheckIcon, CopyIcon } from "@/icons";
import { ScalarStore, useScalarSelector, useTicket } from "@/store";
import {
  InlineCommentBox,
  lineRangeLabel,
  type ReviewAnchor,
  type ReviewComment,
  type ReviewSide,
} from "./InlineCommentBox";

const SIDEBAR_COLLAPSED_KEY = "diff.sidebar.collapsed";
// Per-session prefix for the "Viewed" (GitHub-style) set. We persist a
// `{ path: signature }` map rather than a bare list so a viewed file can be
// auto-unmarked once its diff changes (see `fileSignature`).
const VIEWED_KEY_PREFIX = "diff.viewed.";

// Stable empty list so `useDeferredValue`'s initial value never changes identity
// and the first-mount defer below fires reliably.
const EMPTY_FILES: FileDiffMetadata[] = [];

// A file's full-contents fetch (pairQ in DiffFileEntry) is gated on this set:
// only files that have scrolled within REVEAL_MARGIN of the viewport reveal and
// fetch. Opening a large diff therefore doesn't fire one whole-file fetch (and a
// second Shiki highlight pass) for every changed file at once — just the ones
// the user actually scrolls near. Sticky: a revealed path never drops, so
// scrolling a file out and back never re-fetches or re-highlights it.
const EMPTY_REVEALED: ReadonlySet<string> = new Set();
// Reveal a file ~1.5 screens before it enters view, so its full-context fetch
// and re-highlight usually finish before the user reaches it.
const REVEAL_MARGIN = "1200px 0px";

// Lines revealed per click on a collapsed-context "expand" separator. The
// library defaults to 100, which reveals most gaps in a single click and shoves
// the button far down the viewport; a small step keeps the control in view so
// you can fan it open a bit at a time (shift-click still expands the whole gap).
const EXPAND_CONTEXT_LINES = 10;

// Re-skin @pierre/diffs to match the app. The library renders into shadow DOM
// and themes itself through `--diffs-*` custom properties; this CSS is injected
// into its top-priority `unsafe` layer. Custom properties inherit across the
// shadow boundary, so `var(--color-*)` resolves against the app's own tokens
// and automatically follows the active `data-theme`. Everything the library
// draws (line tints, gutters, separators) is color-mixed from `--diffs-bg`, so
// pinning that to `--color-bg` recolors the whole surface. Syntax token colors
// still come from the Shiki theme passed via `options.theme`.
const DIFF_UNSAFE_CSS = `:host{
  --diffs-bg: var(--color-bg);
  --diffs-font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace;
  --diffs-header-font-family: ui-sans-serif, system-ui, sans-serif;
  --diffs-addition-color: #3fb950;
  --diffs-deletion-color: var(--color-danger);
  --diffs-modified-color: #58a6ff;
}
[data-diffs-header],
[data-diffs-header][data-sticky]{
  background-color: var(--color-surface);
  border-bottom: 1px solid var(--color-border);
}
/* "Viewed" collapse: hide the body (diff/blob rows and the rename banner) but
   keep the library's own header, so collapsing never changes the header's font
   or height. Toggled by a class on the shadow host (see DiffFileEntry), which
   leaves the options object untouched, so the file is not re-highlighted. */
:host(.diff-collapsed) [data-diff],
:host(.diff-collapsed) [data-file],
:host(.diff-collapsed) [data-file-info]{
  display: none;
}`;

function loadSidebarCollapsed(): boolean {
  try {
    return localStorage.getItem(SIDEBAR_COLLAPSED_KEY) === "1";
  } catch {
    return false;
  }
}

// The persisted "Viewed" set for a session: a map of changed-file path to the
// signature the file had when it was marked viewed. Surviving across tab
// switches and reloads is the whole point — DiffPanel unmounts whenever the
// Diff tab loses focus, so in-component state alone wouldn't last.
function loadViewed(sessionId: number): Record<string, string> {
  try {
    const raw = localStorage.getItem(VIEWED_KEY_PREFIX + sessionId);
    const parsed = raw ? JSON.parse(raw) : null;
    return parsed && typeof parsed === "object" ? (parsed as Record<string, string>) : {};
  } catch {
    return {};
  }
}

function saveViewed(sessionId: number, viewed: Record<string, string>): void {
  try {
    localStorage.setItem(VIEWED_KEY_PREFIX + sessionId, JSON.stringify(viewed));
  } catch {
    // ignore (private mode / quota)
  }
}

// FNV-1a (32-bit). Cheap, dependency-free, and only ever run over a handful of
// files (the viewed set), so collision resistance is a non-issue here.
function hashString(s: string): number {
  let h = 0x811c9dc5;
  for (let i = 0; i < s.length; i++) {
    h ^= s.charCodeAt(i);
    h = Math.imul(h, 0x01000193);
  }
  return h >>> 0;
}

// A signature that changes whenever the file's diff changes. `additionLines` /
// `deletionLines` hold the patch's actual content lines, so an in-place edit
// (even one that keeps the +/- counts identical) shifts the hash — which is how
// a "Viewed" mark gets cleared the moment the agent touches the file again.
function fileSignature(f: FileDiffMetadata): string {
  const body = `${f.type}\u0000${f.prevName ?? ""}\u0000${f.additionLines.join("\n")}\u0000${f.deletionLines.join("\n")}`;
  return hashString(body).toString(36);
}

// ---------------------------------------------------------------------------
// Review comments
//
// A lightweight, GitHub-style review layer: the user attaches freeform comments
// to diff lines, then copies them all (with the code they reference) as one
// plaintext block to paste into an agent. Comments persist per-session in
// localStorage, like the "Viewed" set above.
// ---------------------------------------------------------------------------

const COMMENTS_KEY_PREFIX = "diff.review.";
// Stable empties so per-file selectors / annotation arrays keep referential
// identity when a file has nothing — see the perf notes on `commentStore`.
const EMPTY_COMMENTS: ReviewComment[] = [];
const EMPTY_ANNOTATIONS: DiffLineAnnotation[] = [];

// sessionId -> (file path -> its comments). Grouping by path is deliberate: a
// per-path selector returns the *same array reference* for files that didn't
// change, so `useScalarSelector` (which bails via `Object.is`) re-renders only
// the one DiffFileEntry whose comments changed — never the others, and never
// the memoized DiffStack. This is what keeps adding a comment on file A from
// re-rendering (and re-highlighting) file B.
type ReviewState = Record<number, Record<string, ReviewComment[]>>;
const commentStore = new ScalarStore<ReviewState>({});

function loadReview(sessionId: number): Record<string, ReviewComment[]> {
  try {
    const raw = localStorage.getItem(COMMENTS_KEY_PREFIX + sessionId);
    const parsed = raw ? JSON.parse(raw) : null;
    return parsed && typeof parsed === "object" ? (parsed as Record<string, ReviewComment[]>) : {};
  } catch {
    return {};
  }
}

function saveReview(sessionId: number, byPath: Record<string, ReviewComment[]>): void {
  try {
    localStorage.setItem(COMMENTS_KEY_PREFIX + sessionId, JSON.stringify(byPath));
  } catch {
    // ignore (private mode / quota)
  }
}

// Load a session's persisted comments into the store. Idempotent — runs on
// mount and whenever the panel is reused for a new session id.
function hydrateReview(sessionId: number): void {
  commentStore.set((prev) => ({ ...prev, [sessionId]: loadReview(sessionId) }));
}

function addReviewComment(sessionId: number, comment: ReviewComment): void {
  commentStore.set((prev) => {
    const session = prev[sessionId] ?? {};
    const path = comment.anchor.path;
    const next = { ...session, [path]: [...(session[path] ?? EMPTY_COMMENTS), comment] };
    saveReview(sessionId, next);
    return { ...prev, [sessionId]: next };
  });
}

// Rebuild a session's path map, touching only the path that owns `id` so every
// other path's array keeps its identity (and its DiffFileEntry stays put).
function mutateReview(
  sessionId: number,
  id: string,
  fn: (list: ReviewComment[]) => ReviewComment[],
): void {
  commentStore.set((prev) => {
    const session = prev[sessionId];
    if (!session) return prev;
    let changed = false;
    const next: Record<string, ReviewComment[]> = {};
    for (const [path, list] of Object.entries(session)) {
      if (list.some((c) => c.id === id)) {
        const updated = fn(list);
        if (updated.length > 0) next[path] = updated;
        changed = true;
      } else {
        next[path] = list;
      }
    }
    if (!changed) return prev;
    saveReview(sessionId, next);
    return { ...prev, [sessionId]: next };
  });
}

function updateReviewComment(sessionId: number, id: string, body: string): void {
  mutateReview(sessionId, id, (list) => list.map((c) => (c.id === id ? { ...c, body } : c)));
}

function deleteReviewComment(sessionId: number, id: string): void {
  mutateReview(sessionId, id, (list) => list.filter((c) => c.id !== id));
}

// How many of a range's code lines to show before eliding the middle, so a
// comment on a 200-line range doesn't paste an unreadable wall of code.
const SNIPPET_HEAD = 12;
const SNIPPET_TAIL = 8;

// Render code lines [start, end] (1-based, inclusive) with line-number prefixes,
// matching the fenced block in the copied review. Long ranges are elided.
function snippetForRange(lines: string[], start: number, end: number): string {
  const lo = Math.max(1, start);
  const hi = Math.min(lines.length, end);
  if (hi < lo) return "";
  const fmt = (i: number) => `${i}  ${lines[i - 1]}`;
  const total = hi - lo + 1;
  if (total <= SNIPPET_HEAD + SNIPPET_TAIL + 1) {
    const out: string[] = [];
    for (let i = lo; i <= hi; i++) out.push(fmt(i));
    return out.join("\n");
  }
  const head: string[] = [];
  for (let i = lo; i < lo + SNIPPET_HEAD; i++) head.push(fmt(i));
  const tail: string[] = [];
  for (let i = hi - SNIPPET_TAIL + 1; i <= hi; i++) tail.push(fmt(i));
  return [...head, `… (${total - SNIPPET_HEAD - SNIPPET_TAIL} more lines)`, ...tail].join("\n");
}

// buildReviewText turns the session's comments into the single plaintext block
// copied to the clipboard. One block per comment, sorted by path then line, with
// the referenced code fenced inline and a trailing instruction for the agent.
function buildReviewText(
  session: Session,
  ticket: Ticket | undefined,
  comments: ReviewComment[],
): string {
  const sorted = [...comments].sort((a, b) => {
    const byPath = a.anchor.path.localeCompare(b.anchor.path);
    return byPath !== 0 ? byPath : a.anchor.startLine - b.anchor.startLine;
  });
  const header = [
    `Code review — ${ticket?.title ?? "(untitled)"}`,
    `Branch: ${session.branch_name}`,
  ];
  const blocks = sorted.map((c) => {
    const parts = [`Path: ${c.anchor.path}`, `Line: ${lineRangeLabel(c.anchor)}`, ""];
    if (c.snippet) parts.push("```", c.snippet, "```", "");
    parts.push(c.body);
    return parts.join("\n");
  });
  const footer = "How can I resolve these? If you propose a fix, please make it concise.";
  return [header.join("\n"), ...blocks, footer].join("\n\n---\n\n");
}

// Per-file change-type marker. The app's token palette has no green/yellow/blue,
// so additions/renames use fixed hexes that read on both light and dark themes;
// deletions reuse the theme-aware `text-danger` token.
function changeMarker(type: FileDiffMetadata["type"]): { label: string; cls: string } {
  switch (type) {
    case "new":
      return { label: "A", cls: "text-[#3fb950]" };
    case "deleted":
      return { label: "D", cls: "text-danger" };
    case "rename-pure":
    case "rename-changed":
      return { label: "R", cls: "text-[#58a6ff]" };
    default:
      return { label: "M", cls: "text-[#d29922]" };
  }
}

function fileStats(f: FileDiffMetadata): { adds: number; dels: number } {
  let adds = 0;
  let dels = 0;
  for (const h of f.hunks) {
    adds += h.additionLines;
    dels += h.deletionLines;
  }
  return { adds, dels };
}

function displayPath(f: FileDiffMetadata): string {
  return f.prevName ? `${f.prevName} → ${f.name}` : f.name;
}

type Leaf = { kind: "file"; name: string; path: string; file: FileDiffMetadata };
type Dir = { kind: "dir"; name: string; path: string; children: TreeNode[] };
type TreeNode = Leaf | Dir;

// buildTree turns the flat list of changed file paths into a nested
// folder/file tree (split on "/"), with directories sorted before files.
function buildTree(files: FileDiffMetadata[]): TreeNode[] {
  const root: Dir = { kind: "dir", name: "", path: "", children: [] };
  for (const file of files) {
    const parts = file.name.split("/");
    let cur = root;
    parts.forEach((part, i) => {
      if (i === parts.length - 1) {
        cur.children.push({ kind: "file", name: part, path: file.name, file });
        return;
      }
      let next = cur.children.find((c): c is Dir => c.kind === "dir" && c.name === part);
      if (!next) {
        next = { kind: "dir", name: part, path: parts.slice(0, i + 1).join("/"), children: [] };
        cur.children.push(next);
      }
      cur = next;
    });
  }
  const sort = (nodes: TreeNode[]) => {
    nodes.sort((a, b) => {
      if (a.kind !== b.kind) return a.kind === "dir" ? -1 : 1;
      return a.name.localeCompare(b.name);
    });
    for (const n of nodes) if (n.kind === "dir") sort(n.children);
  };
  sort(root.children);
  return root.children;
}

// flattenLeaves walks the sorted tree depth-first so the stacked diffs render
// in the same order the sidebar lists them.
function flattenLeaves(nodes: TreeNode[], out: FileDiffMetadata[] = []): FileDiffMetadata[] {
  for (const n of nodes) {
    if (n.kind === "file") out.push(n.file);
    else flattenLeaves(n.children, out);
  }
  return out;
}

export function DiffPanel({ session }: { session: Session }) {
  const { resolved } = useThemeMode();
  // Syntax colors come from the Shiki theme; the surrounding surface, gutters,
  // borders and accents are remapped to app tokens in DIFF_UNSAFE_CSS, which
  // already track `data-contrast` because the tokens themselves do. Pick the
  // high-contrast Shiki variant so the code text is HC-tuned to match.
  const highContrast = useContrast() === "high";
  const theme = `github-${resolved}${highContrast ? "-high-contrast" : ""}`;

  // Polls every 4s while mounted (the diff tab is open). SessionView keeps this
  // same cache entry warm in the background on other tabs, so opening the tab
  // usually renders from cache rather than waiting on a cold fetch.
  const diffQ = useSessionDiff(session.id);

  const files = useMemo<FileDiffMetadata[]>(() => {
    const patch = diffQ.data?.patch ?? "";
    if (!patch.trim()) return [];
    try {
      return parsePatchFiles(patch).flatMap((p) => p.files);
    } catch {
      return [];
    }
  }, [diffQ.data?.patch]);

  const tree = useMemo(() => buildTree(files), [files]);
  const orderedFiles = useMemo(() => flattenLeaves(tree), [tree]);

  // The distinct Shiki languages in this diff, as a stable comma-joined key.
  // `files` gets a fresh array identity on every 4s poll even when unchanged, so
  // keying the preload effect on this string (compared by value) keeps it from
  // re-firing each poll — it only runs when the set of languages actually shifts.
  const langKey = useMemo(() => {
    const set = new Set<SupportedLanguages>();
    for (const f of files) set.add(getFiletypeFromFileName(f.name));
    return Array.from(set).sort().join(",");
  }, [files]);

  // Warm the Shiki grammar + theme (and the Oniguruma WASM) for every language
  // in this diff as soon as the file list is known — in parallel, before the
  // deferred highlight pass below needs them. Otherwise each ~170 KB language
  // grammar chunk loads lazily mid-highlight, serially gating first paint of
  // highlighted code on the slowest grammar. The shared highlighter dedupes, so
  // this fetches each grammar once and the FileDiff renders reuse it.
  useEffect(() => {
    if (!langKey) return;
    const langs = langKey.split(",") as SupportedLanguages[];
    void preloadHighlighter({ themes: [theme as DiffsThemeNames], langs });
  }, [langKey, theme]);

  // Path → current metadata, for resolving a file by path when (un)marking it
  // viewed and when reconciling the persisted viewed set against a fresh poll.
  const byPath = useMemo(() => {
    const m = new Map<string, FileDiffMetadata>();
    for (const f of orderedFiles) m.set(f.name, f);
    return m;
  }, [orderedFiles]);
  // Keep the latest map reachable from `toggleViewed` without making that
  // callback depend on `byPath` — `byPath` gets a new identity every 4s poll,
  // and a changing `onToggleViewed` prop would defeat DiffStack's memo.
  const byPathRef = useRef(byPath);
  byPathRef.current = byPath;

  // Highlighting every file (Shiki, on the main thread — `disableWorkerPool`) is
  // the expensive part of this panel. Defer it so the shell (sidebar + a
  // "Loading diff…" line) paints first and the diffs fill in via a non-blocking
  // transition. The `EMPTY_FILES` initial value forces the defer even on the
  // first mount when the patch is already warm in cache; otherwise the first
  // open would highlight every file synchronously and jank the tab switch. On
  // later polls `deferredFiles` holds the previous list, so a changed patch
  // re-renders in the background without flashing the placeholder.
  const deferredFiles = useDeferredValue(orderedFiles, EMPTY_FILES);

  // Paths the user has switched from the diff to a whole-file ("View file") view,
  // GitHub-style. Ephemeral (not persisted) and per-path, so toggling one file
  // leaves the rest as diffs. Each viewed file lazily fetches its full contents.
  const [viewFilePaths, setViewFilePaths] = useState<ReadonlySet<string>>(() => new Set());

  // The GitHub-style "Viewed" set: a `{ path: signature }` map persisted per
  // session. A viewed file hides its body but keeps its header. We keep the
  // signature so a later poll can tell whether the file changed since it was
  // marked, and auto-unmark it if so (the effect below).
  const [viewed, setViewed] = useState<Record<string, string>>(() => loadViewed(session.id));
  // Latest viewed map reachable from the stable `toggleViewed` callback, so it
  // can tell collapse from expand without taking `viewed` as a dependency.
  const viewedRef = useRef(viewed);
  viewedRef.current = viewed;

  // The panel is reused (not remounted) across sessions within a ticket — e.g.
  // a restart swaps in a new session id — so re-hydrate the viewed set from its
  // new key. Declared before the reconcile effect so that effect validates the
  // freshly-loaded set against the current diff in the same commit.
  const prevSessionRef = useRef(session.id);
  useEffect(() => {
    if (prevSessionRef.current === session.id) return;
    prevSessionRef.current = session.id;
    setViewed(loadViewed(session.id));
  }, [session.id]);

  // Auto-unmark: drop any viewed file that has since changed (signature differs)
  // or vanished from the diff. Runs on every poll, but only hashes the viewed
  // files, so it stays cheap.
  useEffect(() => {
    setViewed((prev) => {
      const entries = Object.entries(prev);
      if (entries.length === 0) return prev;
      let changed = false;
      const next: Record<string, string> = {};
      for (const [path, sig] of entries) {
        const f = byPath.get(path);
        if (f && fileSignature(f) === sig) next[path] = sig;
        else changed = true;
      }
      if (!changed) return prev;
      saveViewed(session.id, next);
      return next;
    });
  }, [byPath, session.id]);

  const viewedPaths = useMemo<ReadonlySet<string>>(() => new Set(Object.keys(viewed)), [viewed]);

  const [collapsed, setCollapsed] = useState(loadSidebarCollapsed);
  // The file currently scrolled to the top of the viewport, highlighted in the
  // sidebar. Tracked separately from the heavy diff stack so scroll updates
  // only re-render the sidebar.
  const [activePath, setActivePath] = useState<string | null>(null);

  const scrollRef = useRef<HTMLDivElement | null>(null);
  const fileRefs = useRef<Map<string, HTMLElement>>(new Map());
  const rafRef = useRef(0);

  // Paths that have scrolled near the viewport; gates each file's full-contents
  // fetch (see EMPTY_REVEALED / REVEAL_MARGIN). A ScalarStore (not state) so a
  // reveal re-renders only the one DiffFileEntry that just came into view — via
  // its useScalarSelector — and never the memoized DiffStack. Survives the
  // cross-session panel reuse; a stale path just means a same-named file in the
  // next session reveals eagerly (harmless — pairQ is keyed by session+signature).
  const [revealStore] = useState(() => new ScalarStore<ReadonlySet<string>>(EMPTY_REVEALED));
  const revealObserverRef = useRef<IntersectionObserver | null>(null);

  const toggleCollapsed = () => {
    setCollapsed((prev) => {
      const next = !prev;
      try {
        localStorage.setItem(SIDEBAR_COLLAPSED_KEY, next ? "1" : "0");
      } catch {
        // ignore
      }
      return next;
    });
  };

  // A single stable ref callback keyed off `data-path`, so the memoized diff
  // stack never sees a changing prop. Stale entries for removed files are never
  // read (we only iterate the current `orderedFiles`). `revealStore` is stable
  // (useState), so depending on it keeps this callback's identity stable too.
  const registerRef = useCallback(
    (el: HTMLElement | null) => {
      const path = el?.dataset.path;
      if (!el || !path) return;
      fileRefs.current.set(path, el);
      // Watch newly-mounted wrappers for viewport entry. Already-revealed paths
      // are sticky (the observer unobserves on first hit), so skip re-observing.
      if (!revealStore.get().has(path)) revealObserverRef.current?.observe(el);
    },
    [revealStore],
  );

  // One IntersectionObserver per panel, rooted at the scroll container, that
  // flips a file's path into `revealStore` the first time it nears the
  // viewport. registerRef observes wrappers as they mount; this also sweeps any
  // already mounted when the effect runs, since the deferred file stack can
  // commit on either side of it. Sticky: unobserve on first intersection so a
  // file scrolled out and back never re-fetches.
  useEffect(() => {
    const root = scrollRef.current;
    if (!root) return;
    const obs = new IntersectionObserver(
      (entries) => {
        const cur = revealStore.get();
        let next: Set<string> | null = null;
        for (const entry of entries) {
          if (!entry.isIntersecting) continue;
          const el = entry.target as HTMLElement;
          const path = el.dataset.path;
          if (!path) continue;
          obs.unobserve(el);
          if (cur.has(path)) continue;
          if (!next) next = new Set(cur);
          next.add(path);
        }
        if (next) revealStore.set(next);
      },
      { root, rootMargin: REVEAL_MARGIN },
    );
    revealObserverRef.current = obs;
    for (const el of fileRefs.current.values()) {
      if (!revealStore.get().has(el.dataset.path ?? "")) obs.observe(el);
    }
    return () => {
      obs.disconnect();
      revealObserverRef.current = null;
    };
  }, [revealStore]);

  // Mark the last file whose top has reached (or passed) the viewport top as
  // active — same behavior as GitHub's "Files changed" sidebar.
  const updateActive = useCallback(() => {
    const container = scrollRef.current;
    if (!container) return;
    const top = container.getBoundingClientRect().top;
    let current: string | null = null;
    for (const f of orderedFiles) {
      const el = fileRefs.current.get(f.name);
      if (!el) continue;
      if (el.getBoundingClientRect().top - top <= 8) current = f.name;
      else break;
    }
    setActivePath(current ?? orderedFiles[0]?.name ?? null);
  }, [orderedFiles]);

  const onScroll = () => {
    if (rafRef.current) return;
    rafRef.current = requestAnimationFrame(() => {
      rafRef.current = 0;
      updateActive();
    });
  };

  // Default to the first file and recompute once the diffs have laid out (file
  // heights aren't known until the first paint after a diff changes).
  useEffect(() => {
    setActivePath(orderedFiles[0]?.name ?? null);
    const id = requestAnimationFrame(() => updateActive());
    return () => cancelAnimationFrame(id);
  }, [orderedFiles, updateActive]);

  const scrollToFile = useCallback((path: string, behavior: ScrollBehavior = "smooth") => {
    const container = scrollRef.current;
    const el = fileRefs.current.get(path);
    if (!container || !el) return;
    const top =
      el.getBoundingClientRect().top - container.getBoundingClientRect().top + container.scrollTop;
    container.scrollTo({ top, behavior });
    setActivePath(path);
  }, []);

  // Toggle a file between its diff and whole-file ("View file") view. Swapping
  // the body changes the file's height — collapsing a long blob back to its
  // diff pulls everything below it up — so once the new layout commits (next
  // frame) re-anchor the toggled file's header to the top, rather than leaving
  // the user scrolled into whatever shifted into view. The jump is instant, not
  // smooth: the body teleports in place, so there's no motion to animate along.
  const toggleFileView = useCallback(
    (path: string) => {
      setViewFilePaths((prev) => {
        const next = new Set(prev);
        if (next.has(path)) next.delete(path);
        else next.add(path);
        return next;
      });
      requestAnimationFrame(() => scrollToFile(path, "instant"));
    },
    [scrollToFile],
  );

  // Mark/unmark a file "Viewed". Marking hides its body (and captures its
  // current signature so it auto-unmarks on later edits); unmarking reveals
  // whatever view — diff or blob — it had.
  //
  // Re-anchor the file's header to the top only when *collapsing*: the body
  // above the fold vanishes, so without this the content below would jump up
  // under the cursor. When *expanding* we deliberately don't scroll — the newly
  // revealed diff should grow in place below its header, leaving the rest of the
  // page where it was. (Scrolling there fought the user when un-viewing several
  // files: each reveal yanked the viewport away and shoved the others off-screen.)
  const toggleViewed = useCallback(
    (path: string) => {
      const collapsing = !(path in viewedRef.current);
      setViewed((prev) => {
        const next = { ...prev };
        if (path in next) {
          delete next[path];
        } else {
          const f = byPathRef.current.get(path);
          if (!f) return prev;
          next[path] = fileSignature(f);
        }
        saveViewed(session.id, next);
        return next;
      });
      if (collapsing) requestAnimationFrame(() => scrollToFile(path, "instant"));
    },
    [scrollToFile, session.id],
  );

  // Review comments for this session, grouped by path. Read here only to drive
  // the sidebar's "Copy review" button + count; the per-file comment boxes
  // subscribe independently inside DiffFileEntry, so a comment change never
  // re-renders the memoized DiffStack.
  const reviewByPath = useScalarSelector(
    commentStore,
    useCallback((v: ReviewState) => v[session.id], [session.id]),
  );
  const allComments = useMemo(
    () => (reviewByPath ? Object.values(reviewByPath).flat() : EMPTY_COMMENTS),
    [reviewByPath],
  );

  // Load persisted comments into the store on mount and whenever the panel is
  // reused for a new session (mirrors the "Viewed" re-hydrate above).
  useEffect(() => {
    hydrateReview(session.id);
  }, [session.id]);

  // Stable across polls (depend only on session.id, like toggleViewed) so
  // threading them through DiffStack never busts its memo.
  const onAddComment = useCallback(
    (c: ReviewComment) => addReviewComment(session.id, c),
    [session.id],
  );
  const onUpdateComment = useCallback(
    (id: string, body: string) => updateReviewComment(session.id, id, body),
    [session.id],
  );
  const onDeleteComment = useCallback(
    (id: string) => deleteReviewComment(session.id, id),
    [session.id],
  );

  if (diffQ.isLoading) {
    return <p className="p-4 text-sm text-fg-muted">Loading diff…</p>;
  }
  if (diffQ.error) {
    return <p className="p-4 text-sm text-danger">Couldn't load the diff.</p>;
  }
  if (orderedFiles.length === 0) {
    return <p className="p-4 text-sm text-fg-muted">No changes yet.</p>;
  }

  return (
    <div className="flex h-full min-h-0">
      {collapsed ? (
        <button
          type="button"
          onClick={toggleCollapsed}
          title="Show changed files"
          aria-label="Show changed files"
          aria-expanded={false}
          className="flex h-full w-6 shrink-0 items-center justify-center border-r border-border bg-bg text-fg-muted hover:bg-surface-2 hover:text-fg"
        >
          <span aria-hidden>▶</span>
        </button>
      ) : (
        <aside className="flex w-64 shrink-0 flex-col border-r border-border bg-bg text-sm">
          <div className="sticky top-0 z-10 flex min-h-11 items-center border-b border-border bg-bg px-3">
            <h2 className="text-[13px] font-semibold uppercase tracking-wide text-fg-muted">
              Changed files
            </h2>
            <span className="ml-2 text-xs text-fg-muted">{orderedFiles.length}</span>
            <div className="ml-auto flex items-center gap-1">
              {allComments.length > 0 && (
                <CopyReviewButton session={session} comments={allComments} />
              )}
              <button
                type="button"
                onClick={toggleCollapsed}
                title="Hide changed files"
                aria-label="Hide changed files"
                aria-expanded={true}
                className="rounded px-1 text-fg-muted hover:bg-surface-2 hover:text-fg"
              >
                <span aria-hidden>◀</span>
              </button>
            </div>
          </div>
          <div className="flex-1 overflow-y-auto [scrollbar-gutter:stable]">
            <TreeRows
              nodes={tree}
              depth={0}
              selected={activePath}
              viewedPaths={viewedPaths}
              onSelect={scrollToFile}
            />
          </div>
        </aside>
      )}
      <div
        ref={scrollRef}
        onScroll={onScroll}
        className="min-w-0 flex-1 overflow-auto [scrollbar-gutter:stable]"
      >
        {deferredFiles.length === 0 ? (
          <p className="p-4 text-sm text-fg-muted">Loading diff…</p>
        ) : (
          <DiffStack
            files={deferredFiles}
            sessionId={session.id}
            theme={theme}
            themeType={resolved}
            viewFilePaths={viewFilePaths}
            viewedPaths={viewedPaths}
            onToggleView={toggleFileView}
            onToggleViewed={toggleViewed}
            onAddComment={onAddComment}
            onUpdateComment={onUpdateComment}
            onDeleteComment={onDeleteComment}
            registerRef={registerRef}
            revealStore={revealStore}
          />
        )}
      </div>
    </div>
  );
}

// DiffStack renders every changed file stacked vertically. It is memoized on
// its props so the active-file highlight — which changes on every scroll frame
// (held in DiffPanel, not here) — never re-renders the (expensive) file list.
// `viewFilePaths` / `viewedPaths` only change on a "View file" / "Viewed"
// toggle, so a toggle re-renders the stack but React reconciles by `key`,
// leaving the other files' FileDiff instances (and their Shiki highlight)
// untouched. Both `onToggle*` callbacks are stable across polls, so a fresh
// diff poll on its own never re-renders the stack.
// Stable callbacks for the review layer. Kept separate from FileEntryProps so
// FileBlob (which has no comment UI) doesn't inherit them.
type ReviewCallbacks = {
  onAddComment: (comment: ReviewComment) => void;
  onUpdateComment: (id: string, body: string) => void;
  onDeleteComment: (id: string) => void;
};

const DiffStack = memo(function DiffStack({
  files,
  sessionId,
  theme,
  themeType,
  viewFilePaths,
  viewedPaths,
  onToggleView,
  onToggleViewed,
  onAddComment,
  onUpdateComment,
  onDeleteComment,
  registerRef,
  revealStore,
}: {
  files: FileDiffMetadata[];
  sessionId: number;
  theme: string;
  themeType: "light" | "dark";
  viewFilePaths: ReadonlySet<string>;
  viewedPaths: ReadonlySet<string>;
  onToggleView: (path: string) => void;
  onToggleViewed: (path: string) => void;
  registerRef: (el: HTMLElement | null) => void;
  revealStore: ScalarStore<ReadonlySet<string>>;
} & ReviewCallbacks) {
  return (
    <>
      {files.map((f) => (
        <div
          key={f.name}
          data-path={f.name}
          ref={registerRef}
          className="border-b border-border last:border-b-0"
        >
          <DiffFileEntry
            file={f}
            sessionId={sessionId}
            theme={theme}
            themeType={themeType}
            viewFile={viewFilePaths.has(f.name)}
            viewed={viewedPaths.has(f.name)}
            onToggleView={onToggleView}
            onToggleViewed={onToggleViewed}
            onAddComment={onAddComment}
            onUpdateComment={onUpdateComment}
            onDeleteComment={onDeleteComment}
            revealStore={revealStore}
          />
        </div>
      ))}
    </>
  );
});

type FileEntryProps = {
  file: FileDiffMetadata;
  sessionId: number;
  theme: string;
  themeType: "light" | "dark";
  viewFile: boolean;
  onToggleView: (path: string) => void;
  onToggleViewed: (path: string) => void;
};

// DiffFileEntry renders one file as either its diff or — when toggled — its
// whole contents (the "View file" blob view). A blob view is only offered for
// text files that still exist on the new side: binary/metadata-only changes
// (no hunks) and deletions have nothing to show as a file. When the file is
// marked "Viewed" the library's header stays exactly as-is and only the body is
// hidden (via the `diff-collapsed` host class), so the header's font and height
// never shift between states.
function DiffFileEntry({
  file,
  sessionId,
  theme,
  themeType,
  viewFile,
  viewed,
  onToggleView,
  onToggleViewed,
  onAddComment,
  onUpdateComment,
  onDeleteComment,
  revealStore,
}: FileEntryProps & {
  viewed: boolean;
  revealStore: ScalarStore<ReadonlySet<string>>;
} & ReviewCallbacks) {
  const canView = file.type !== "deleted" && file.hunks.length > 0;
  const viewedToggle = <ViewedToggle viewed={viewed} onToggle={() => onToggleViewed(file.name)} />;

  // Has this file scrolled near the viewport yet? Gates the full-contents fetch
  // below so a large diff doesn't fetch (and re-highlight) every file on open.
  // The selector returns a boolean, so this re-renders only on the open→revealed
  // flip — once — and never for reveals of other files.
  const revealed = useScalarSelector(
    revealStore,
    useCallback((v: ReadonlySet<string>) => v.has(file.name), [file.name]),
  );

  // A modified (or renamed-and-modified) text file has content on both sides, so
  // we can rebuild its diff from the full base+head contents and hand the library
  // every line — that's what backs its GitHub-style expand-up/down separators.
  // New/deleted files are entirely one-sided (nothing to expand into) and
  // binary/no-hunks files have no diff, so both keep the cheap patch-derived
  // metadata. The fetch is keyed by the file's signature (see fileSignature), so
  // it only re-runs when the file actually changes — not on every 4s diff poll —
  // and is gated to the live diff view (and to files scrolled near the viewport,
  // via `revealed`) so a blob/collapsed/far-offscreen file never pays the fetch
  // + whole-file highlight cost. Until it lands (or if it fails) we render the
  // patch metadata, so the diff still shows immediately, just without expansion.
  const expandable = file.hunks.length > 0 && file.type !== "new" && file.type !== "deleted";
  const oldName = file.prevName ?? file.name;
  const pairQ = useQuery({
    queryKey: queryKeys.sessionFileDiff(sessionId, file.name, fileSignature(file)),
    queryFn: () => api.getSessionFileDiff(sessionId, file.name, file.prevName ?? undefined),
    enabled: expandable && !viewFile && !viewed && revealed,
    staleTime: Number.POSITIVE_INFINITY,
  });
  const enriched = useMemo(() => {
    if (!pairQ.data) return null;
    try {
      const meta = parseDiffFromFile(
        { name: oldName, contents: pairQ.data.old_contents },
        { name: file.name, contents: pairQ.data.new_contents },
      );
      // A no-hunk result means the contents matched (e.g. a stale/odd read) —
      // fall back to the patch metadata rather than render an empty file.
      return meta.hunks.length > 0 ? meta : null;
    } catch {
      return null;
    }
  }, [pairQ.data, oldName, file.name]);

  // --- Review comments (live diff view only) ---
  // This file's comments, read from the shared store with a per-path selector.
  // The selector returns a stable array reference for files that didn't change,
  // so a comment edit on another file never re-renders (or re-highlights) this
  // one. EMPTY_COMMENTS keeps the no-comment case referentially stable too.
  const fileComments = useScalarSelector(
    commentStore,
    useCallback(
      (v: ReviewState) => v[sessionId]?.[file.name] ?? EMPTY_COMMENTS,
      [sessionId, file.name],
    ),
  );
  // The pending new-comment anchor (gutter click/drag). Local + ephemeral, so it
  // never touches the store or re-renders sibling files.
  const [draft, setDraft] = useState<ReviewAnchor | null>(null);

  // Controlled line-selection highlight. We drive it ourselves (rather than
  // letting the library own it) so the highlight can be cleared the moment the
  // drag ends — otherwise the blue selection would linger and never "blur" when
  // you click elsewhere. The library only renders the highlight from this
  // controlled value, so we feed `onLineSelectionChange` back into it for live
  // feedback during the drag, then clear it on release (see commitDraft).
  const [selection, setSelection] = useState<SelectedLineRange | null>(null);

  // One annotation per (side, line) carrying a comment or the draft. Memoized so
  // the array identity is stable across unrelated re-renders (e.g. the 4s poll);
  // a fresh array would make @pierre/diffs treat annotations as "changed" and
  // needlessly re-lay-out the file.
  const annotations = useMemo<DiffLineAnnotation[]>(() => {
    const seen = new Set<string>();
    const out: DiffLineAnnotation[] = [];
    const add = (side: ReviewSide, lineNumber: number) => {
      const key = `${side}:${lineNumber}`;
      if (seen.has(key)) return;
      seen.add(key);
      out.push({ side, lineNumber });
    };
    // Anchor at the range's end line so the box renders after the last selected
    // line (GitHub-style), instead of splitting the selection in the middle.
    for (const c of fileComments) add(c.anchor.side, c.anchor.endLine);
    if (draft) add(draft.side, draft.endLine);
    return out.length > 0 ? out : EMPTY_ANNOTATIONS;
  }, [fileComments, draft]);

  // Capture the code under a range at save time so the copied review carries the
  // exact lines reviewed. Prefer the full base/head contents already fetched for
  // expansion (pairQ) or derived (enriched); fall back to the one-sided patch
  // arrays for new/deleted files.
  const captureSnippet = useCallback(
    (anchor: ReviewAnchor): string => {
      const lines =
        anchor.side === "additions"
          ? pairQ.data
            ? pairQ.data.new_contents.split("\n")
            : (enriched?.additionLines ?? (file.type === "new" ? file.additionLines : null))
          : pairQ.data
            ? pairQ.data.old_contents.split("\n")
            : (enriched?.deletionLines ?? (file.type === "deleted" ? file.deletionLines : null));
      return lines ? snippetForRange(lines, anchor.startLine, anchor.endLine) : "";
    },
    [pairQ.data, enriched, file.type, file.additionLines, file.deletionLines],
  );

  // Open a draft comment over a selected range. Shared by both entry points:
  // the gutter "+" (click a line, or drag the gutter) and dragging across the
  // line-number column (which the library highlights live, since
  // `enableLineSelection` is on). Stable across polls (depends only on
  // file.name) so the FileDiff `options` object stays shallow-equal and never
  // forces a re-highlight.
  const openDraftFromRange = useCallback(
    (range: SelectedLineRange | null) => {
      if (!range) return;
      const side: ReviewSide = range.side ?? range.endSide ?? "additions";
      setDraft({
        path: file.name,
        side,
        startLine: Math.min(range.start, range.end),
        endLine: Math.max(range.start, range.end),
      });
    },
    [file.name],
  );

  // End of a selection gesture (line-number drag or gutter "+"): open the draft
  // and keep the range highlighted while the comment box is open, so you can see
  // exactly which lines you're commenting on. The highlight is cleared when the
  // draft closes (see the save/cancel handlers in renderAnnotation).
  const commitDraft = useCallback(
    (range: SelectedLineRange | null) => {
      setSelection(range);
      openDraftFromRange(range);
    },
    [openDraftFromRange],
  );

  // Close the draft and drop its highlight together.
  const closeDraft = useCallback(() => {
    setDraft(null);
    setSelection(null);
  }, []);

  // Renders the inline comment box(es) for one annotated line: any saved comments
  // there, plus the draft editor if the draft sits on this line.
  const renderAnnotation = useCallback(
    (a: DiffLineAnnotation) => {
      const here = fileComments.filter(
        (c) => c.anchor.side === a.side && c.anchor.endLine === a.lineNumber,
      );
      const draftHere =
        draft && draft.side === a.side && draft.endLine === a.lineNumber ? draft : null;
      return (
        <>
          {here.map((c) => (
            <InlineCommentBox
              key={c.id}
              comment={c}
              anchor={c.anchor}
              onSave={(body) => onUpdateComment(c.id, body)}
              onDelete={() => onDeleteComment(c.id)}
            />
          ))}
          {draftHere && (
            <InlineCommentBox
              key="draft"
              comment={null}
              anchor={draftHere}
              onSave={(body) => {
                onAddComment({
                  id: crypto.randomUUID(),
                  anchor: draftHere,
                  body,
                  snippet: captureSnippet(draftHere),
                  createdAt: Date.now(),
                });
                closeDraft();
              }}
              onCancel={closeDraft}
            />
          )}
        </>
      );
    },
    [
      fileComments,
      draft,
      onAddComment,
      onUpdateComment,
      onDeleteComment,
      captureSnippet,
      closeDraft,
    ],
  );

  if (file.hunks.length === 0) {
    return (
      <div>
        <MetaHeaderBar file={file}>{viewedToggle}</MetaHeaderBar>
        {!viewed && (
          <div className="px-4 py-2 text-sm text-fg-muted">
            No textual diff to display (binary file or metadata-only change).
          </div>
        )}
      </div>
    );
  }

  if (viewFile && canView) {
    return (
      <FileBlob
        file={file}
        sessionId={sessionId}
        theme={theme}
        themeType={themeType}
        viewed={viewed}
        onToggleView={onToggleView}
        onToggleViewed={onToggleViewed}
      />
    );
  }

  return (
    <FileDiff
      fileDiff={enriched ?? file}
      className={viewed ? "diff-collapsed" : undefined}
      lineAnnotations={annotations}
      renderAnnotation={renderAnnotation}
      selectedLines={selection}
      options={{
        diffStyle: "split",
        theme,
        themeType,
        stickyHeader: true,
        expansionLineCount: EXPAND_CONTEXT_LINES,
        unsafeCSS: DIFF_UNSAFE_CSS,
        // Two ways to start a comment: the gutter "+" (click a line, or drag the
        // gutter), and dragging across the line-number column. enableLineSelection
        // gives the drag a live highlight; it only starts on the number column, so
        // selecting code text to copy still works. We control the selection
        // (selectedLines above) so it can be cleared on release: onLineSelectionChange
        // feeds the live highlight, commitDraft opens the box and clears it.
        enableGutterUtility: true,
        onGutterUtilityClick: commitDraft,
        enableLineSelection: true,
        onLineSelectionChange: setSelection,
        onLineSelectionEnd: commitDraft,
      }}
      renderHeaderMetadata={() => (
        <div className="flex items-center gap-1">
          {canView && <ViewToggle viewing={false} onClick={() => onToggleView(file.name)} />}
          {viewedToggle}
        </div>
      )}
      disableWorkerPool
    />
  );
}

// FileBlob fetches the file's current working-tree contents and renders them as
// a plain, syntax-highlighted file (no diff coloring), like GitHub's "View
// file". The fetch is cached per (session, path); it is a point-in-time
// snapshot and does not poll, so heavy in-progress edits won't thrash it.
function FileBlob({
  file,
  sessionId,
  theme,
  themeType,
  viewed,
  onToggleView,
  onToggleViewed,
}: Omit<FileEntryProps, "viewFile"> & { viewed: boolean }) {
  const back = () => onToggleView(file.name);
  const markViewed = () => onToggleViewed(file.name);
  const q = useQuery({
    queryKey: queryKeys.sessionFile(sessionId, file.name),
    queryFn: () => api.getSessionFile(sessionId, file.name),
    staleTime: 10_000,
  });

  if (q.isPending) {
    return (
      <BlobBar file={file} viewed={viewed} onToggleView={back} onToggleViewed={markViewed}>
        <span className="text-fg-muted">Loading file…</span>
      </BlobBar>
    );
  }
  if (q.isError || !q.data) {
    return (
      <BlobBar file={file} viewed={viewed} onToggleView={back} onToggleViewed={markViewed}>
        <span className="text-danger">Couldn't load file.</span>
      </BlobBar>
    );
  }

  return (
    <File
      file={{ name: file.name, contents: q.data.contents }}
      className={viewed ? "diff-collapsed" : undefined}
      options={{
        theme,
        themeType,
        stickyHeader: true,
        unsafeCSS: DIFF_UNSAFE_CSS,
      }}
      renderHeaderMetadata={() => (
        <div className="flex items-center gap-1">
          <ViewToggle viewing onClick={back} />
          <ViewedToggle viewed={viewed} onToggle={markViewed} />
        </div>
      )}
      disableWorkerPool
    />
  );
}

// MetaHeaderBar reproduces the @pierre/diffs file header for the two cases the
// library draws no header of its own — the no-hunks notice and the blob
// loading/error states. It matches the library's metrics (16px inline padding,
// a 44px min-height = 1lh@20px + 24px, a sans 13px title) plus our surface /
// border skin from DIFF_UNSAFE_CSS, so the "Viewed" toggle and the filename
// line up with every other file's header instead of sitting slightly inset.
function MetaHeaderBar({ file, children }: { file: FileDiffMetadata; children: ReactNode }) {
  return (
    <div className="sticky top-0 z-10 flex min-h-11 items-center gap-2 border-b border-border bg-surface px-4">
      <span
        className="min-w-0 flex-1 truncate font-sans text-[13px] leading-5 text-fg"
        title={displayPath(file)}
      >
        {displayPath(file)}
      </span>
      {children}
    </div>
  );
}

// BlobBar is a stand-in for the blob view's loading/error states, so the "View
// diff" / "Viewed" toggles stay reachable before the File component (which draws
// its own header) has any contents to render. It uses MetaHeaderBar so this
// placeholder header is identical to the real one that replaces it on load.
function BlobBar({
  file,
  viewed,
  onToggleView,
  onToggleViewed,
  children,
}: {
  file: FileDiffMetadata;
  viewed: boolean;
  onToggleView: () => void;
  onToggleViewed: () => void;
  children: ReactNode;
}) {
  return (
    <div>
      <MetaHeaderBar file={file}>
        <div className="flex items-center gap-1">
          <ViewToggle viewing onClick={onToggleView} />
          <ViewedToggle viewed={viewed} onToggle={onToggleViewed} />
        </div>
      </MetaHeaderBar>
      {!viewed && <div className="px-4 py-2 text-sm">{children}</div>}
    </div>
  );
}

// ViewToggle is the per-file "View file" / "View diff" switch. The library
// projects it (light DOM) into the file header's metadata slot, so it styles
// with the app's own tokens and its onClick behaves like any React handler.
function ViewToggle({ viewing, onClick }: { viewing: boolean; onClick: () => void }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="rounded px-1.5 py-0.5 text-[11px] text-fg-muted hover:bg-surface-2 hover:text-fg"
    >
      {viewing ? "View diff" : "View file"}
    </button>
  );
}

// ViewedToggle is the GitHub-style "Viewed" checkbox. Like ViewToggle it's
// projected (light DOM) into a file header's metadata slot, so it styles with
// the app's tokens and its onChange behaves like any React handler. Checking it
// collapses the file; the checkbox uses the addition green to read as "done".
function ViewedToggle({ viewed, onToggle }: { viewed: boolean; onToggle: () => void }) {
  return (
    <label
      className="flex shrink-0 cursor-pointer select-none items-center gap-1 rounded px-1.5 py-0.5 text-[11px] text-fg-muted hover:bg-surface-2 hover:text-fg"
      title={viewed ? "Marked viewed — click to expand" : "Collapse and mark viewed"}
    >
      <input
        type="checkbox"
        checked={viewed}
        onChange={onToggle}
        className="h-3 w-3 accent-[#3fb950]"
      />
      Viewed
    </label>
  );
}

// CopyReviewButton copies every review comment in the session — each with its
// path, line range, the referenced code, and a trailing instruction — as one
// plaintext block to paste into an agent. Lives in the sidebar header and only
// renders when the session has at least one comment.
function CopyReviewButton({ session, comments }: { session: Session; comments: ReviewComment[] }) {
  const { copied, markCopied } = useCopyFeedback(1500);
  const ticket = useTicket(session.ticket_id);
  const onCopy = () => {
    navigator.clipboard.writeText(buildReviewText(session, ticket, comments));
    markCopied();
  };
  const plural = comments.length === 1 ? "" : "s";
  return (
    <button
      type="button"
      onClick={onCopy}
      title={copied ? "Copied!" : `Copy review (${comments.length} comment${plural})`}
      aria-label={`Copy review (${comments.length} comment${plural})`}
      className="flex items-center gap-1 rounded px-1.5 py-0.5 text-[11px] text-fg-muted hover:bg-surface-2 hover:text-fg"
    >
      {copied ? <CheckIcon size={12} /> : <CopyIcon size={12} />}
      <span>{comments.length}</span>
    </button>
  );
}

function TreeRows({
  nodes,
  depth,
  selected,
  viewedPaths,
  onSelect,
}: {
  nodes: TreeNode[];
  depth: number;
  selected: string | null;
  viewedPaths: ReadonlySet<string>;
  onSelect: (path: string) => void;
}) {
  return (
    <>
      {nodes.map((n) =>
        n.kind === "dir" ? (
          <DirRow
            key={n.path}
            dir={n}
            depth={depth}
            selected={selected}
            viewedPaths={viewedPaths}
            onSelect={onSelect}
          />
        ) : (
          <FileRow
            key={n.path}
            leaf={n}
            depth={depth}
            selected={selected}
            viewed={viewedPaths.has(n.path)}
            onSelect={onSelect}
          />
        ),
      )}
    </>
  );
}

function DirRow({
  dir,
  depth,
  selected,
  viewedPaths,
  onSelect,
}: {
  dir: Dir;
  depth: number;
  selected: string | null;
  viewedPaths: ReadonlySet<string>;
  onSelect: (path: string) => void;
}) {
  const [open, setOpen] = useState(true);
  return (
    <div>
      <button
        type="button"
        onClick={() => setOpen((o) => !o)}
        style={{ paddingLeft: depth * 12 + 8 }}
        className="flex w-full items-center gap-1 py-1 pr-2 text-left text-fg-muted hover:bg-surface-2"
        title={dir.path}
      >
        <span aria-hidden className="w-3 shrink-0 text-center text-[10px]">
          {open ? "▾" : "▸"}
        </span>
        <span className="min-w-0 flex-1 truncate">{dir.name}</span>
      </button>
      {open && (
        <TreeRows
          nodes={dir.children}
          depth={depth + 1}
          selected={selected}
          viewedPaths={viewedPaths}
          onSelect={onSelect}
        />
      )}
    </div>
  );
}

function FileRow({
  leaf,
  depth,
  selected,
  viewed,
  onSelect,
}: {
  leaf: Leaf;
  depth: number;
  selected: string | null;
  viewed: boolean;
  onSelect: (path: string) => void;
}) {
  const { adds, dels } = fileStats(leaf.file);
  const marker = changeMarker(leaf.file.type);
  const active = selected === leaf.path;
  return (
    <button
      type="button"
      onClick={() => onSelect(leaf.path)}
      title={leaf.file.prevName ? `${leaf.file.prevName} → ${leaf.path}` : leaf.path}
      style={{ paddingLeft: depth * 12 + 8 }}
      className={`flex w-full items-center gap-2 py-1 pr-2 text-left hover:bg-surface-2 ${
        active ? "bg-surface-2 font-medium" : ""
      }`}
    >
      <span className={`w-3 shrink-0 text-center text-[10px] font-bold ${marker.cls}`}>
        {marker.label}
      </span>
      <span className={`min-w-0 flex-1 truncate ${viewed ? "text-fg-muted line-through" : ""}`}>
        {leaf.name}
      </span>
      {viewed ? (
        <span aria-hidden title="Viewed" className="shrink-0 text-[10px] text-[#3fb950]">
          ✓
        </span>
      ) : (
        <>
          {adds > 0 && <span className="shrink-0 text-[10px] text-[#3fb950]">+{adds}</span>}
          {dels > 0 && <span className="shrink-0 text-[10px] text-danger">−{dels}</span>}
        </>
      )}
    </button>
  );
}
