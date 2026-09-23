import { api, waitForSession, isAttachable } from "./fixtures/api";
import { DOCKER_AVAILABLE } from "./fixtures/docker";
import { test, expect } from "./fixtures/seed";
import { BoardPage } from "./pages/BoardPage";

test.skip(!DOCKER_AVAILABLE, "requires a reachable docker daemon");

// Initial-load scrollback path: a session has been running with output,
// the user loads the page, the terminal WS opens, and the broker
// immediately replays the ring buffer (broker.go:257) so historical
// output is visible before any new bytes arrive. Asserts the UI's first
// shell WS receives our pre-seeded marker.
test("first UI WS connect replays prior shell output", async ({
  seed,
  page,
}) => {
  // 1) Start the session and let it become attachable.
  const session = await api.ensureSession(seed.ticket.id);
  await api.startSession(session.id);
  const ready = await waitForSession(seed.board.id, session.id, isAttachable, {
    timeoutMs: 60_000,
  });
  expect(isAttachable(ready)).toBe(true);

  // 2) Seed the active board, then navigate to the app so we have a
  // real origin for the throwaway WebSocket.
  const board = new BoardPage(page);
  await board.goto(seed.board.id);

  // 3) Inject a known marker via a throwaway shell WS, then close it.
  // The broker keeps the last ~64 KiB in its ring buffer (broker.go:19)
  // so the next connect — the one the UI is about to make after reload
  // — will replay it.
  const marker = `SCROLLBACK_${Math.random().toString(36).slice(2, 10)}`;
  await page.evaluate(
    async ({ sessionId, marker }) => {
      const proto = location.protocol === "https:" ? "wss:" : "ws:";
      const ws = new WebSocket(`${proto}//${location.host}/ws/sessions/${sessionId}/shell`);
      ws.binaryType = "arraybuffer";
      const frames: string[] = [];
      const dec = new TextDecoder();
      ws.addEventListener("message", (e) => {
        if (typeof e.data === "string") frames.push(e.data);
        else frames.push(dec.decode(new Uint8Array(e.data)));
      });
      await new Promise<void>((res, rej) => {
        ws.addEventListener("open", () => res());
        ws.addEventListener("error", () => rej(new Error("ws error")));
      });
      ws.send(JSON.stringify({ type: "resize", cols: 80, rows: 24 }));
      ws.send(`echo ${marker}\r`);
      const deadline = Date.now() + 10_000;
      while (Date.now() < deadline && !frames.join("").includes(marker)) {
        await new Promise((r) => setTimeout(r, 50));
      }
      ws.close();
      await new Promise((r) => setTimeout(r, 200));
    },
    { sessionId: session.id, marker },
  );

  // 4) Reload to wipe the page's WS state. After this, every shell WS
  // is a *fresh* connection that should trigger broker replay.
  const dec = new TextDecoder();
  const shellAggregates = new Map<string, string>();
  page.on("websocket", (ws) => {
    if (!ws.url().includes(`/ws/sessions/${session.id}/shell`)) return;
    const key = `${ws.url()}#${shellAggregates.size}`;
    shellAggregates.set(key, "");
    ws.on("framereceived", (data) => {
      const payload = data.payload;
      const text =
        typeof payload === "string" ? payload : dec.decode(new Uint8Array(payload));
      shellAggregates.set(key, (shellAggregates.get(key) ?? "") + text);
    });
  });
  await board.reload();

  // 5) Drive the UI to open the shell WS: pick the ticket, switch to
  // the "terminal" tab.
  await board.ticketCard(seed.ticket.title).click();
  await board.sessionPane.openTerminalTab();

  // 6) Any of the captured shell WS aggregations should contain the
  // replayed marker.
  await expect
    .poll(() => [...shellAggregates.values()].join("\n---\n"), {
      timeout: 15_000,
    })
    .toContain(marker);
});
