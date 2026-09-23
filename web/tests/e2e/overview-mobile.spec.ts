import { test, expect } from "./fixtures/seed";
import { OverviewPage } from "./pages/OverviewPage";

// MobileOverview replaces the desktop split-pane with a single-pane
// drilldown: tree → tap ticket → SessionView. Triggered by
// `(max-width: 639px)` in Overview.tsx.

test.describe("Overview / Mobile", () => {
  test("at narrow viewport, only the tree is rendered (no resizer, no canvas)", async ({ page, seed }) => {
    const overview = new OverviewPage(page);
    await overview.setViewport(375, 812);
    await overview.goto();

    // The tree still mounts (we can see the seeded board's node).
    await overview.boardNode(seed.board.id).expectVisible();
    await overview.treeTicket(seed.ticket.id).expectVisible();

    // The resizer/canvas only exist in DesktopOverview.
    await expect(overview.sidebar.resizer).toHaveCount(0);
    await expect(overview.canvas).toHaveCount(0);
  });

  test("tapping a ticket drills into the SessionView; back returns to the tree", async ({ page, seed }) => {
    const overview = new OverviewPage(page);
    await overview.setViewport(375, 812);
    await overview.goto();

    await overview.treeTicket(seed.ticket.id).click();

    // SessionView mounts. The terminal slot may not yet render (session
    // isn't started by default for mobile drill-in), but the SessionView
    // header with the ✕ close button is visible.
    await expect(overview.mobileSession.backButton).toBeVisible();
    // The tree node should be unmounted while the session is open.
    await expect(overview.boardNode(seed.board.id).root).toHaveCount(0);

    await overview.mobileSession.backButton.click();
    await overview.boardNode(seed.board.id).expectVisible();
  });
});
