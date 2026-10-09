import { api, isAttachable, waitForSession } from "./fixtures/api";
import { DOCKER_AVAILABLE } from "./fixtures/docker";
import { test } from "./fixtures/seed";
import { OverviewPage } from "./pages/OverviewPage";

test.skip(!DOCKER_AVAILABLE, "requires a reachable docker daemon");

// The no-session half of the tree filter lives in overview-board-tree.spec.ts;
// this covers the half that needs a real container: a running ticket stays
// listed, and its board drops out of the filtered tree when the session stops.
test("running filter keeps only tickets whose session is running", async ({ seed, page }) => {
  const idleTicket = await seed.addTicket("never started");
  const session = await api.ensureSession(seed.ticket.id);
  await api.startSession(session.id);
  await waitForSession(seed.board.id, session.id, isAttachable, { timeoutMs: 60_000 });

  const overview = new OverviewPage(page);
  await overview.goto();
  await overview.sidebar.cycleTreeFilterTo("running");

  const node = overview.boardNode(seed.board.id);
  await overview.treeTicket(seed.ticket.id).expectVisible();
  await overview.treeTicket(idleTicket.id).expectHidden();
  await node.expectTotalTickets(1);

  await api.stopSession(session.id);
  await waitForSession(seed.board.id, session.id, (s) => s.status === "stopped");
  await node.expectHidden();
});
