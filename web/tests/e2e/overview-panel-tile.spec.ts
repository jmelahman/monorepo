import { test, expect } from "./fixtures/seed";
import { OverviewPage } from "./pages/OverviewPage";

// Tile zones, snap zones, maximize toggle. Zone thresholds mirror
// tile.ts (EDGE = 24, CORNER = 80). The drag must end with the cursor
// inside the zone for tileZoneFromPoint to fire.

test.describe("Overview / Tiling", () => {
  test.beforeEach(async ({ page }) => {
    // Use a known viewport so the canvas dimensions are reliable. Default
    // Playwright Desktop Chrome is 1280x720 but specifying it pins the
    // canvas math even if a project changes the device.
    await page.setViewportSize({ width: 1280, height: 800 });
  });

  test("dropping near the top edge maximizes the panel", async ({ page, seed }) => {
    const overview = new OverviewPage(page);
    await overview.goto();
    await overview.treeTicket(seed.ticket.id).click();

    const panel = overview.panel(seed.ticket.id);
    await panel.expectMounted();

    const canvas = await overview.canvasBox();
    // 10px below top — inside py<24 → "max".
    await panel.dragTo(canvas.width / 2, 10);
    await panel.expectTile("max");

    const tiled = await panel.box();
    // Maxed panel covers the full canvas (within a pixel or two of header).
    expect(tiled.width).toBeGreaterThan(canvas.width * 0.95);
    expect(tiled.height).toBeGreaterThan(canvas.height * 0.95);
  });

  test("dropping into the left edge tiles to the left half", async ({ page, seed }) => {
    const overview = new OverviewPage(page);
    await overview.goto();
    await overview.treeTicket(seed.ticket.id).click();

    const panel = overview.panel(seed.ticket.id);
    const canvas = await overview.canvasBox();
    // Left edge but well below the top corner zone (y > 80) — px<24 → "left".
    await panel.dragTo(10, canvas.height / 2);
    await panel.expectTile("left");

    const tiled = await panel.box();
    expect(tiled.x).toBeLessThanOrEqual(canvas.x + 2);
    expect(tiled.width).toBeGreaterThan(canvas.width * 0.45);
    expect(tiled.width).toBeLessThan(canvas.width * 0.55);
  });

  test("dropping into the right edge tiles to the right half", async ({ page, seed }) => {
    const overview = new OverviewPage(page);
    await overview.goto();
    await overview.treeTicket(seed.ticket.id).click();

    const panel = overview.panel(seed.ticket.id);
    const canvas = await overview.canvasBox();
    await panel.dragTo(canvas.width - 5, canvas.height / 2);
    await panel.expectTile("right");
  });

  test("dropping into a corner tiles to the matching quadrant", async ({ page, seed }) => {
    const overview = new OverviewPage(page);
    await overview.goto();
    await overview.treeTicket(seed.ticket.id).click();

    const panel = overview.panel(seed.ticket.id);
    const canvas = await overview.canvasBox();
    // px<80 && py>canvasH-80 → "bl".
    await panel.dragTo(40, canvas.height - 40);
    await panel.expectTile("bl");

    const tiled = await panel.box();
    // Bottom-left quadrant: left half × bottom half.
    expect(tiled.width).toBeLessThan(canvas.width * 0.55);
    expect(tiled.height).toBeLessThan(canvas.height * 0.55);
  });

  test("dragging a tiled panel back out un-tiles it and restores the float rect", async ({ page, seed }) => {
    const overview = new OverviewPage(page);
    await overview.goto();
    await overview.treeTicket(seed.ticket.id).click();
    const panel = overview.panel(seed.ticket.id);

    const beforeFloat = await panel.box();

    // First tile to the left.
    const canvas = await overview.canvasBox();
    await panel.dragTo(10, canvas.height / 2);
    await panel.expectTile("left");

    // Then drag back into the middle of the canvas (no tile zone).
    await panel.dragTo(canvas.width / 2, canvas.height / 2);
    await panel.expectTile("float");

    const afterFloat = await panel.box();
    // Floating dimensions should match the pre-tile rect, modulo cascade
    // offset (the drop point becomes the new top-left, clamped).
    expect(Math.abs(afterFloat.width - beforeFloat.width)).toBeLessThan(2);
    expect(Math.abs(afterFloat.height - beforeFloat.height)).toBeLessThan(2);
  });

  test("double-clicking the drag handle toggles maximize and restores on the second click", async ({ page, seed }) => {
    const overview = new OverviewPage(page);
    await overview.goto();
    await overview.treeTicket(seed.ticket.id).click();
    const panel = overview.panel(seed.ticket.id);

    await panel.doubleClickHandle();
    await panel.expectTile("max");

    await panel.doubleClickHandle();
    await panel.expectTile("float");
  });

  test("the in-header Maximize button toggles to Restore and back", async ({ page, seed }) => {
    const overview = new OverviewPage(page);
    await overview.goto();
    await overview.treeTicket(seed.ticket.id).click();
    const panel = overview.panel(seed.ticket.id);

    await panel.maximizeButton.click();
    await panel.expectTile("max");
    await expect(panel.restoreButton).toBeVisible();

    await panel.restoreButton.click();
    await panel.expectTile("float");
    await expect(panel.maximizeButton).toBeVisible();
  });

  test("tile state persists across reload", async ({ page, seed }) => {
    const overview = new OverviewPage(page);
    await overview.goto();
    await overview.treeTicket(seed.ticket.id).click();
    const panel = overview.panel(seed.ticket.id);

    const canvas = await overview.canvasBox();
    await panel.dragTo(canvas.width - 5, canvas.height / 2);
    await panel.expectTile("right");

    await overview.reload();
    await overview.panel(seed.ticket.id).expectTile("right");
  });
});
