import { api } from "./fixtures/api";
import { test, expect } from "./fixtures/seed";
import { OverviewPage } from "./pages/OverviewPage";

// BoardTree (sidebar) coverage for the Overview view.
//
// What's NOT covered here:
//   - Sidebar resizing / collapse  → overview-sidebar.spec.ts
//   - Panel chrome / layout        → overview-panel-canvas.spec.ts
//   - Mobile drilldown             → overview-mobile.spec.ts

test.describe("Overview / BoardTree", () => {
  test("renders the seeded board, columns, and ticket counts", async ({ page, seed }) => {
    const overview = new OverviewPage(page);
    await overview.goto();

    const boardState = await api.boardState(seed.board.id);
    const backlog = boardState.columns.find((c) => c.name === "Backlog")!;

    const node = overview.boardNode(seed.board.id);
    await node.expectVisible();
    await expect(node.toggleButton).toContainText(seed.board.name);
    await node.expectTotalTickets(1);

    // Backlog column lists the seeded ticket. Other columns exist but are
    // empty — we only assert the one with content so a future default-
    // column rename doesn't fail the whole suite.
    const backlogCol = node.column(backlog.id, "Backlog");
    await expect(backlogCol.root).toBeVisible();
    await overview.treeTicket(seed.ticket.id).expectVisible();
  });

  test("collapse hides the columns and persists across reload", async ({ page, seed }) => {
    const overview = new OverviewPage(page);
    await overview.goto();

    const node = overview.boardNode(seed.board.id);
    await node.expectExpanded();

    await node.toggle();
    await node.expectCollapsed();

    await overview.reload();
    await overview.boardNode(seed.board.id).expectCollapsed();

    await overview.boardNode(seed.board.id).toggle();
    await overview.boardNode(seed.board.id).expectExpanded();
  });

  test("clicking a ticket opens a single panel and marks the row active", async ({ page, seed }) => {
    const overview = new OverviewPage(page);
    await overview.goto();
    await expect(overview.emptyState).toBeVisible();

    const treeTicket = overview.treeTicket(seed.ticket.id);
    await treeTicket.expectInactive();
    await treeTicket.click();

    await overview.panel(seed.ticket.id).expectMounted();
    await treeTicket.expectActive();
    await expect(overview.emptyState).toBeHidden();

    // Re-clicking the same row must not spawn a duplicate panel; the
    // existing one is brought to front.
    await treeTicket.click();
    await overview.expectPanelCount(1);
  });

  test("opening multiple tickets stacks panels in the canvas", async ({ page, seed }) => {
    const ticketB = await seed.addTicket("second ticket");
    const overview = new OverviewPage(page);
    await overview.goto();

    await overview.treeTicket(seed.ticket.id).click();
    await overview.treeTicket(ticketB.id).click();

    await overview.expectPanelCount(2);

    // The second panel cascades by CASCADE_STEP (28px) from the first.
    const boxA = await overview.panel(seed.ticket.id).box();
    const boxB = await overview.panel(ticketB.id).box();
    expect(boxB.x).toBeGreaterThan(boxA.x);
    expect(boxB.y).toBeGreaterThan(boxA.y);
  });

  test("add-ticket form creates a new tree row in the chosen column", async ({ page, seed }) => {
    const overview = new OverviewPage(page);
    await overview.goto();

    const state = await api.boardState(seed.board.id);
    const review = state.columns.find((c) => c.name === "Review")!;
    const node = overview.boardNode(seed.board.id);
    const reviewCol = node.column(review.id, "Review");

    await reviewCol.startAdd();
    await reviewCol.submit("from tree");

    // The new ticket should appear in the Review column. We can't predict
    // its id without polling the API, but only one ticket on this board
    // has the title we just typed.
    const newTicketRow = page.locator("[data-tree-ticket]").filter({ hasText: "from tree" });
    await expect(newTicketRow).toBeVisible();
    await expect(reviewCol.root).toContainText("from tree");
    await node.expectTotalTickets(2);
  });

  test("dragging a tree ticket onto another column moves it server-side", async ({ page, seed }) => {
    const overview = new OverviewPage(page);
    await overview.goto();

    const state = await api.boardState(seed.board.id);
    const inProgress = state.columns.find((c) => c.name === "In Progress")!;

    const node = overview.boardNode(seed.board.id);
    const sourceTicket = overview.treeTicket(seed.ticket.id);
    const targetCol = node.column(inProgress.id, "In Progress");

    await sourceTicket.dragTo(targetCol);

    // The optimistic update + server roundtrip should land the ticket in
    // the new column. Poll the API rather than the DOM — DOM order on the
    // tree is the same outcome but settles a tick later via query refetch.
    await expect
      .poll(async () => {
        const fresh = await api.boardState(seed.board.id);
        return fresh.tickets.find((t) => t.id === seed.ticket.id)?.column_id;
      }, { timeout: 10_000 })
      .toBe(inProgress.id);

    // And the row should render under the In Progress column in the tree.
    await expect(targetCol.root).toContainText(seed.ticket.title);
  });
});
