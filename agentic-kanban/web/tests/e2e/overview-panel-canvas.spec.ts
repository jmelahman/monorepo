import { test, expect } from "./fixtures/seed";
import { OverviewPage } from "./pages/OverviewPage";
import { api } from "./fixtures/api";

// PanelCanvas: open/close, focus tracking, drag, persistence, and the
// archived-ticket purge. Tile zones and the maximize toggle are covered
// separately in overview-panel-tile.spec.ts.

test.describe("Overview / PanelCanvas", () => {
  test("shows the empty state until the first ticket is opened", async ({ page, seed }) => {
    const overview = new OverviewPage(page);
    await overview.goto();
    await expect(overview.emptyState).toBeVisible();
    await overview.expectPanelCount(0);

    await overview.treeTicket(seed.ticket.id).click();

    await overview.panel(seed.ticket.id).expectMounted();
    await expect(overview.emptyState).toBeHidden();
  });

  test("re-clicking the same row brings the panel to the front without duplicating it", async ({ page, seed }) => {
    const ticketB = await seed.addTicket("ticket B");
    const overview = new OverviewPage(page);
    await overview.goto();

    await overview.treeTicket(seed.ticket.id).click();
    await overview.treeTicket(ticketB.id).click();
    await overview.expectPanelCount(2);

    // DOM order of [data-panel] mirrors the panels array — the last one
    // sits on top because of natural stacking.
    const panelsBefore = await overview.panels().evaluateAll((els) =>
      els.map((el) => Number((el as HTMLElement).dataset.panel)),
    );
    expect(panelsBefore[panelsBefore.length - 1]).toBe(ticketB.id);

    await overview.treeTicket(seed.ticket.id).click();
    await overview.expectPanelCount(2);
    const panelsAfter = await overview.panels().evaluateAll((els) =>
      els.map((el) => Number((el as HTMLElement).dataset.panel)),
    );
    expect(panelsAfter[panelsAfter.length - 1]).toBe(seed.ticket.id);
  });

  test("drag handle repositions the panel and the new layout persists", async ({ page, seed }) => {
    const overview = new OverviewPage(page);
    await overview.goto();
    await overview.treeTicket(seed.ticket.id).click();

    const panel = overview.panel(seed.ticket.id);
    await panel.expectMounted();
    const before = await panel.box();

    // Drag the panel ~200px to the right and down. Avoid the canvas edges
    // (within 24px) so we don't trip the tile-zone snap.
    const canvas = await overview.canvasBox();
    const targetX = Math.min(canvas.width - 200, before.width + 200);
    const targetY = Math.min(canvas.height - 200, before.height);
    await panel.dragTo(targetX, targetY);

    const after = await panel.box();
    expect(Math.abs(after.x - before.x)).toBeGreaterThan(20);
    expect(after.x).toBeGreaterThan(before.x);

    // Reload — react-query refetches boards, panels reload from
    // localStorage. The panel must come back at the dragged position.
    await overview.reload();
    const restored = await overview.panel(seed.ticket.id).box();
    expect(Math.abs(restored.x - after.x)).toBeLessThan(2);
    expect(Math.abs(restored.y - after.y)).toBeLessThan(2);
  });

  test("close button removes the panel and updates persistence", async ({ page, seed }) => {
    const overview = new OverviewPage(page);
    await overview.goto();
    await overview.treeTicket(seed.ticket.id).click();
    await overview.panel(seed.ticket.id).expectMounted();

    await overview.panel(seed.ticket.id).close();
    await overview.panel(seed.ticket.id).expectAbsent();
    await expect(overview.emptyState).toBeVisible();
    await overview.treeTicket(seed.ticket.id).expectInactive();

    await overview.reload();
    await overview.panel(seed.ticket.id).expectAbsent();
  });

  test("archiving a ticket removes its panel after the board structure refetches", async ({ page, seed }) => {
    const ticketB = await seed.addTicket("disposable");
    const overview = new OverviewPage(page);
    await overview.goto();

    await overview.treeTicket(seed.ticket.id).click();
    await overview.treeTicket(ticketB.id).click();
    await overview.expectPanelCount(2);

    // Archive ticketB on the server. The board-events SSE stream notifies
    // the open subscription, but we don't want the test to depend on SSE
    // delivery — reload forces fetchBoardStructure to drop the ticket,
    // and PanelCanvas's isLiveTicket filter then prunes the panel.
    await page.evaluate(async (id) => {
      const r = await fetch(`/api/tickets/${id}/archive`, { method: "POST" });
      if (!r.ok) throw new Error(`archive failed: ${r.status}`);
    }, ticketB.id);

    await overview.reload();
    await overview.panel(ticketB.id).expectAbsent();
    await overview.panel(seed.ticket.id).expectMounted();
  });
});
