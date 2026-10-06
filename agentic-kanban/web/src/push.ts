import { api, PUSH_EVENTS, type PushEvent } from "@/api/client";
import { openTicketRequestStore } from "@/store";

// Why this device can or can't receive notifications. Service workers and
// push only exist on HTTPS (or localhost), and iOS only offers push to a
// site that has been added to the home screen.
export type PushSupport = "ok" | "insecure" | "ios-needs-install" | "unsupported";

const EVENTS_KEY = "kanban.push.events";

function isIOS(): boolean {
  // iPadOS reports itself as a Mac; touch points tell them apart.
  return (
    /iPad|iPhone|iPod/.test(navigator.userAgent) ||
    (navigator.platform === "MacIntel" && navigator.maxTouchPoints > 1)
  );
}

function isStandalone(): boolean {
  return (
    window.matchMedia("(display-mode: standalone)").matches ||
    (navigator as Navigator & { standalone?: boolean }).standalone === true
  );
}

export function pushSupport(): PushSupport {
  if (!window.isSecureContext) return "insecure";
  if (isIOS() && !isStandalone()) return "ios-needs-install";
  if (!("serviceWorker" in navigator) || !("PushManager" in window) || !("Notification" in window))
    return "unsupported";
  return "ok";
}

// The opted-in kinds live on the server per subscription; this copy is what
// the settings form shows and what a resync sends back.
export function loadPushEvents(): PushEvent[] {
  try {
    const raw = localStorage.getItem(EVENTS_KEY);
    if (raw == null) return [...PUSH_EVENTS];
    const parsed: unknown = JSON.parse(raw);
    if (!Array.isArray(parsed)) return [...PUSH_EVENTS];
    return PUSH_EVENTS.filter((e) => parsed.includes(e));
  } catch {
    return [...PUSH_EVENTS];
  }
}

function savePushEvents(events: PushEvent[]): void {
  try {
    localStorage.setItem(EVENTS_KEY, JSON.stringify(events));
  } catch {
    // ignore
  }
}

function decodeKey(base64url: string): Uint8Array<ArrayBuffer> {
  const padded = base64url.replace(/-/g, "+").replace(/_/g, "/");
  const raw = atob(padded + "=".repeat((4 - (padded.length % 4)) % 4));
  const out = new Uint8Array(new ArrayBuffer(raw.length));
  for (let i = 0; i < raw.length; i++) out[i] = raw.charCodeAt(i);
  return out;
}

export async function currentPushSubscription(): Promise<PushSubscription | null> {
  if (pushSupport() !== "ok") return null;
  const reg = await navigator.serviceWorker.getRegistration();
  return (await reg?.pushManager.getSubscription()) ?? null;
}

const SW_READY_TIMEOUT_MS = 10_000;

// Asks for permission if needed, subscribes this browser and registers the
// subscription with the server. Throws with a user-facing message.
export async function enablePush(events: PushEvent[]): Promise<PushSubscription> {
  const permission = await Notification.requestPermission();
  if (permission !== "granted") {
    throw new Error(
      permission === "denied"
        ? "Notifications are blocked for this site. Allow them in the browser's site settings."
        : "Notification permission was not granted.",
    );
  }
  // `serviceWorker.ready` never settles when registration failed, which
  // would leave the settings toggle busy forever. Register here instead (a
  // no-op when installPush already did) and wait a bounded time for it.
  await navigator.serviceWorker.register("/sw.js");
  const reg = await Promise.race([
    navigator.serviceWorker.ready,
    new Promise<never>((_, reject) =>
      setTimeout(
        () =>
          reject(new Error("The notification service worker didn't start. Reload and try again.")),
        SW_READY_TIMEOUT_MS,
      ),
    ),
  ]);
  const { public_key } = await api.getVapidKey();
  const options = { userVisibleOnly: true, applicationServerKey: decodeKey(public_key) };
  let sub: PushSubscription;
  try {
    sub = await reg.pushManager.subscribe(options);
  } catch {
    // A subscription made against a different server key (another kanban
    // backend, or a recreated database) blocks a new one until it's dropped.
    await (await reg.pushManager.getSubscription())?.unsubscribe();
    sub = await reg.pushManager.subscribe(options);
  }
  await api.putPushSubscription(sub.toJSON(), events);
  savePushEvents(events);
  return sub;
}

export async function updatePushEvents(sub: PushSubscription, events: PushEvent[]): Promise<void> {
  await api.putPushSubscription(sub.toJSON(), events);
  savePushEvents(events);
}

export async function disablePush(sub: PushSubscription): Promise<void> {
  await api.deletePushSubscription(sub.endpoint);
  await sub.unsubscribe();
}

type OpenTicketMessage = { type: "open-ticket"; boardId: number; ticketId: number };

function isOpenTicketMessage(data: unknown): data is OpenTicketMessage {
  const m = data as Partial<OpenTicketMessage> | null;
  return m?.type === "open-ticket" && Number.isFinite(m.boardId) && Number.isFinite(m.ticketId);
}

// A notification tapped while no window was open launches
// /?board=…&ticket=…; turn that into the same request and tidy the URL.
function consumeDeepLink(): void {
  const params = new URLSearchParams(window.location.search);
  const boardId = Number(params.get("board"));
  const ticketId = Number(params.get("ticket"));
  if (!params.has("board") || !params.has("ticket")) return;
  params.delete("board");
  params.delete("ticket");
  const query = params.toString();
  window.history.replaceState(
    null,
    "",
    window.location.pathname + (query ? `?${query}` : "") + window.location.hash,
  );
  if (boardId > 0 && ticketId > 0) openTicketRequestStore.set({ boardId, ticketId });
}

// Registers the service worker and wires notification taps to the app.
// Called once at startup; a no-op for the worker where it can't exist.
export function installPush(): void {
  consumeDeepLink();
  if (!window.isSecureContext || !("serviceWorker" in navigator)) return;
  navigator.serviceWorker.addEventListener("message", (e) => {
    if (isOpenTicketMessage(e.data)) {
      openTicketRequestStore.set({ boardId: e.data.boardId, ticketId: e.data.ticketId });
    }
  });
  void (async () => {
    try {
      await navigator.serviceWorker.register("/sw.js");
      // Re-register an existing subscription on every load. The upsert is
      // idempotent, and it heals a server that lost the row (database
      // recreated) while this browser still believes it is subscribed.
      const sub = await currentPushSubscription();
      if (sub) await api.putPushSubscription(sub.toJSON(), loadPushEvents());
    } catch {
      // Best-effort: the app works without notifications.
    }
  })();
}
