import { api, waitForSession, isAttachable } from "./fixtures/api";
import { DOCKER_AVAILABLE } from "./fixtures/docker";
import { test, expect } from "./fixtures/seed";

test.skip(!DOCKER_AVAILABLE, "requires a reachable docker daemon");

// Server-contract test for the broker's ring-buffer replay. Frontend
// auto-reconnect for terminal WS is a known gap (PtyTerminal.tsx:89 just
// prints "[disconnected]") — we'll wire that up in the streaming refactor.
// This test pins down the server behavior the refactor must keep working:
// every fresh WS connection receives `ESC c` (RIS) followed by the ring
// buffer snapshot before any live output. See `broker.go:257`.
test("shell WS replays ESC c + ring buffer on reconnect", async ({
  seed,
  page,
}) => {
  const session = await api.ensureSession(seed.ticket.id);
  await api.startSession(session.id);
  const ready = await waitForSession(seed.board.id, session.id, isAttachable, {
    timeoutMs: 60_000,
  });
  expect(isAttachable(ready)).toBe(true);

  // Navigate to the frontend root: the page only needs a real origin
  // for the WS client to attach to, and using the dev server lets us
  // hit the WS endpoint through the Vite proxy (same path as the UI
  // does), so we exercise the proxy alongside the broker.
  await page.goto("/");

  const marker = `MARKER_${Math.random().toString(36).slice(2, 10)}`;

  type WsResult = { frames: string[]; closed: boolean };

  // First connection: write the marker and close.
  const first: WsResult = await page.evaluate(
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
      // Echo the marker so it lands in the broker's ring buffer.
      ws.send(`echo ${marker}\r`);
      // Wait until the marker shows up in returned frames (server echoed it).
      const deadline = Date.now() + 10_000;
      while (Date.now() < deadline && !frames.join("").includes(marker)) {
        await new Promise((r) => setTimeout(r, 50));
      }
      ws.close();
      await new Promise((r) => setTimeout(r, 200));
      return { frames, closed: true };
    },
    { sessionId: session.id, marker },
  );
  expect(first.frames.join("")).toContain(marker);

  // Second connection: a fresh WS should receive a hard reset (`ESC c`)
  // followed by the ring buffer snapshot, which still contains our marker.
  const second: WsResult = await page.evaluate(
    async ({ sessionId }) => {
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
      // Give the broker a moment to ship the snapshot.
      await new Promise((r) => setTimeout(r, 500));
      ws.close();
      return { frames, closed: true };
    },
    { sessionId: session.id },
  );

  const joined = second.frames.join("");
  expect(joined).toContain("\x1bc"); // RIS — broker reset prefix
  expect(joined).toContain(marker); // replayed from ring buffer
});
