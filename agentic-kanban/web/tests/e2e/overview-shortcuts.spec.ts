import { test, expect } from "./fixtures/seed";
import { OverviewPage } from "./pages/OverviewPage";

// Keyboard shortcuts that only fire on the Overview route:
//   F                       — toggle fullscreen on focused panel
//   N                       — open NewTicketModal
//   Ctrl+Alt+Arrow          — focus neighbor panel (PanelCanvas's own
//                             window listener; pre-empts board.column.* on
//                             the board view)
//   Ctrl+Shift+Alt+Arrow    — re-tile focused panel toward direction

test.describe("Overview / Shortcuts", () => {
  test.beforeEach(async ({ page }) => {
    await page.setViewportSize({ width: 1280, height: 800 });
  });

  test("F maximizes the focused panel", async ({ page, seed }) => {
    const overview = new OverviewPage(page);
    await overview.goto();
    await overview.treeTicket(seed.ticket.id).click();
    const panel = overview.panel(seed.ticket.id);
    await panel.expectMounted();

    // Click the drag handle so DOM focus lands on a non-editable element.
    // session.fullscreen's binding has no Ctrl/Meta, so the dispatcher
    // skips it if focus is inside an xterm textarea.
    await panel.dragHandle.click();

    await page.keyboard.press("F");
    await panel.expectTile("max");

    await page.keyboard.press("F");
    await panel.expectTile("float");
  });

  test("Ctrl+Alt+ArrowRight moves focus to the panel cascaded to the right", async ({ page, seed }) => {
    const ticketB = await seed.addTicket("ticket B");
    const overview = new OverviewPage(page);
    await overview.goto();

    await overview.treeTicket(seed.ticket.id).click();
    // Wait for panel A to mount before opening B — otherwise the cascade
    // step in PanelCanvas may misplace B (and the neighbor-search relies
    // on A being to the left of B).
    await overview.panel(seed.ticket.id).expectMounted();
    await overview.treeTicket(ticketB.id).click();
    await overview.panel(ticketB.id).expectMounted();

    // ticketB was opened last → cascade puts it to the right and below
    // ticketA, and it starts as the focused panel.
    await overview.panel(ticketB.id).expectFocused();

    // Move focus to the panel "to the left" of B — which is A.
    await overview.panel(ticketB.id).focus();
    await page.keyboard.press("Control+Alt+ArrowLeft");
    await overview.panel(seed.ticket.id).expectFocused();

    // And back to the right.
    await page.keyboard.press("Control+Alt+ArrowRight");
    await overview.panel(ticketB.id).expectFocused();
  });

  test("Ctrl+Shift+Alt+ArrowLeft tiles the focused panel to the left half", async ({ page, seed }) => {
    const overview = new OverviewPage(page);
    await overview.goto();
    await overview.treeTicket(seed.ticket.id).click();
    const panel = overview.panel(seed.ticket.id);
    await panel.focus();
    await panel.expectTile("float");

    await page.keyboard.press("Control+Shift+Alt+ArrowLeft");
    await panel.expectTile("left");

    // Pressing the same direction again toggles back to floating.
    await page.keyboard.press("Control+Shift+Alt+ArrowLeft");
    await panel.expectTile("float");
  });

  test("N opens the New Ticket modal pre-targeting the focused board", async ({ page, seed }) => {
    const overview = new OverviewPage(page);
    await overview.goto();
    await overview.treeTicket(seed.ticket.id).click();
    const panel = overview.panel(seed.ticket.id);
    await panel.dragHandle.click();

    await page.keyboard.press("N");
    await overview.newTicketModal.expectOpen();

    // Board select defaults to the focused panel's board (the only one
    // the seed creates).
    await expect(overview.newTicketModal.boardSelect).toHaveValue(String(seed.board.id));

    await overview.newTicketModal.cancel.click();
    await overview.newTicketModal.expectClosed();
  });
});
