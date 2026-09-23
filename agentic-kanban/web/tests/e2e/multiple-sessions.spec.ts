import { api, waitForSession, isAttachable } from "./fixtures/api";
import { DOCKER_AVAILABLE } from "./fixtures/docker";
import { test, expect } from "./fixtures/seed";

test.skip(!DOCKER_AVAILABLE, "requires a reachable docker daemon");

// Two concurrent sessions on the same board: write distinct markers to
// each WS and assert each session's stream stays isolated. Catches
// session-id mix-ups in the broker registry / route handler — a regression
// that lost output to the wrong terminal would be invisible to single-
// session tests.
test("two sessions stream independently without cross-contamination", async ({
  seed,
  page,
}) => {
  const ticketA = seed.ticket;
  const ticketB = await seed.addTicket("session ticket B");

  const sessA = await api.ensureSession(ticketA.id);
  await api.startSession(sessA.id);
  const sessB = await api.ensureSession(ticketB.id);
  await api.startSession(sessB.id);

  await Promise.all([
    waitForSession(seed.board.id, sessA.id, isAttachable, { timeoutMs: 60_000 }),
    waitForSession(seed.board.id, sessB.id, isAttachable, { timeoutMs: 60_000 }),
  ]);

  // We need a real origin for the WebSocket constructor to attach to;
  // the frontend root is convenient and lets us hit the Vite WS proxy.
  await page.goto("/");

  const markerA = `A_${Math.random().toString(36).slice(2, 10)}`;
  const markerB = `B_${Math.random().toString(36).slice(2, 10)}`;

  // Drive both WS connections in parallel from the page context. Each
  // returns the frames it received so we can assert isolation.
  const result = await page.evaluate(
    async ({ aId, bId, mA, mB }) => {
      const run = async (sessionId: number, marker: string) => {
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
        return frames.join("");
      };
      const [a, b] = await Promise.all([run(aId, mA), run(bId, mB)]);
      return { a, b };
    },
    { aId: sessA.id, bId: sessB.id, mA: markerA, mB: markerB },
  );

  expect(result.a).toContain(markerA);
  expect(result.a).not.toContain(markerB);
  expect(result.b).toContain(markerB);
  expect(result.b).not.toContain(markerA);
});
