// Service worker for push notifications only. It deliberately has no fetch
// handler and caches nothing: the app is useless without its backend, and a
// cached bundle would keep serving a stale UI after an upgrade.

self.addEventListener("install", () => {
  self.skipWaiting();
});

self.addEventListener("activate", (event) => {
  event.waitUntil(self.clients.claim());
});

// Payload shape: internal/push Payload.
self.addEventListener("push", (event) => {
  let data = {};
  try {
    data = event.data ? event.data.json() : {};
  } catch {
    // A payload we can't parse still has to show something: browsers revoke
    // the subscription of a worker that receives pushes silently.
  }
  event.waitUntil(
    self.registration.showNotification(data.title || "Kanban", {
      body: data.body || "",
      icon: "/icon-192.png",
      // One notification per session: a newer state replaces the older one,
      // and renotify makes the replacement alert again.
      tag: data.tag,
      renotify: Boolean(data.tag),
      data: { boardId: data.board_id, ticketId: data.ticket_id },
    }),
  );
});

self.addEventListener("notificationclick", (event) => {
  event.notification.close();
  const { boardId, ticketId } = event.notification.data || {};
  const hasTarget = Boolean(boardId && ticketId);
  event.waitUntil(
    (async () => {
      const windows = await self.clients.matchAll({ type: "window", includeUncontrolled: true });
      const client = windows.find((c) => c.visibilityState === "visible") || windows[0];
      if (client) {
        if (hasTarget) client.postMessage({ type: "open-ticket", boardId, ticketId });
        try {
          await client.focus();
        } catch {
          // Some browsers refuse focus() here; the message still lands.
        }
        return;
      }
      await self.clients.openWindow(hasTarget ? `/?board=${boardId}&ticket=${ticketId}` : "/");
    })(),
  );
});
