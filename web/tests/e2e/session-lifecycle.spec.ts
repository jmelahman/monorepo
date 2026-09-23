import { api, waitForSession, isAttachable } from "./fixtures/api";
import { DOCKER_AVAILABLE } from "./fixtures/docker";
import { test, expect } from "./fixtures/seed";
import { BoardPage } from "./pages/BoardPage";

test.skip(!DOCKER_AVAILABLE, "requires a reachable docker daemon");

// Full UI walk through a session: open the board, click a ticket, watch
// the auto-ensure + auto-start flow drive the status pill through
// stopped → starting → idle, terminal mounts, then stop returns it to
// stopped. Pins down `SessionView.tsx` auto-start + board-SSE consumer.
test("ticket click ensures, starts, and stops a session via UI", async ({
  page,
  seed,
}) => {
  const board = new BoardPage(page);

  const wsOpens: string[] = [];
  page.on("websocket", (ws) => wsOpens.push(ws.url()));

  await board.goto(seed.board.id);

  const ticketCard = board.ticketCard(seed.ticket.title);
  await ticketCard.expectVisible();
  await ticketCard.click();

  // Wait for the session to flip into an attachable state. The UI
  // status pill mirrors the SSE-driven sessionStore.
  const sessionId = await firstSessionId(seed.board.id, seed.ticket.id);
  const session = await waitForSession(seed.board.id, sessionId, isAttachable, {
    timeoutMs: 60_000,
  });
  expect(["idle", "working"]).toContain(session.status);

  // Status text on the ticket card matches the backend.
  await ticketCard.expectStatus(session.status);

  // The agent terminal mounts whenever a session is attachable.
  await board.sessionPane.expectTerminalMounted();

  // A WebSocket to /ws/sessions/{id}/pty opened during the attach.
  await expect
    .poll(() => wsOpens.some((u) => u.includes(`/ws/sessions/${session.id}/pty`)))
    .toBe(true);

  // Stop via the header action button (aria-label="stop").
  await board.sessionPane.stop();

  await waitForSession(seed.board.id, session.id, (s) => s.status === "stopped", {
    timeoutMs: 30_000,
  });
  await ticketCard.expectStatus("stopped");
  await board.sessionPane.expectTerminalAbsent();
});

// Helper: poll boardState until a session exists for this ticket.
async function firstSessionId(boardId: number, ticketId: number): Promise<number> {
  const deadline = Date.now() + 20_000;
  while (Date.now() < deadline) {
    const state = await api.boardState(boardId);
    const found = state.sessions.find((s) => s.ticket_id === ticketId);
    if (found) return found.id;
    await new Promise((r) => setTimeout(r, 150));
  }
  throw new Error(`no session created for ticket ${ticketId} within 20s`);
}
