import { test } from "./fixtures/seed";
import { AppSettingsPage } from "./pages/AppSettingsPage";
import { BoardPage } from "./pages/BoardPage";
import { OverviewPage } from "./pages/OverviewPage";
import { SoftKeyboard } from "./pages/SoftKeyboard";

// The session UI must shrink to the visual viewport so the soft keyboard
// never covers the bottom of the terminal (where the agent's prompt is).
// See REGRESSIONS.md: "Soft keyboard covers the terminal".

const KEYBOARD = 340;

test.describe("Mobile / soft keyboard", () => {
  test("overview drill-in stays above the keyboard", async ({ page, seed }) => {
    const keyboard = new SoftKeyboard(page);
    await keyboard.install();
    const overview = new OverviewPage(page);
    await overview.setViewport(375, 812);
    await overview.goto();
    await overview.treeTicket(seed.ticket.id).click();

    await keyboard.expectFills(overview.mobileSession.root);
    await keyboard.open(KEYBOARD);
    await keyboard.expectClears(overview.mobileSession.root, KEYBOARD);
    await keyboard.close();
    await keyboard.expectFills(overview.mobileSession.root);
  });

  test("board session overlay stays above the keyboard", async ({ page, seed }) => {
    const keyboard = new SoftKeyboard(page);
    await keyboard.install();
    const board = new BoardPage(page);
    await board.setViewport(375, 812);
    await board.goto(seed.board.id);
    await board.ticketCard(seed.ticket.title).click();

    await keyboard.expectFills(board.sessionPane.root);
    await keyboard.open(KEYBOARD);
    await keyboard.expectClears(board.sessionPane.root, KEYBOARD);
    await keyboard.close();
    await keyboard.expectFills(board.sessionPane.root);
  });

  test("zooming in doesn't shrink the session view", async ({ page, seed }) => {
    const keyboard = new SoftKeyboard(page);
    await keyboard.install();
    const overview = new OverviewPage(page);
    await overview.setViewport(375, 812);
    await overview.goto();
    await overview.treeTicket(seed.ticket.id).click();

    await keyboard.expectFills(overview.mobileSession.root);
    await keyboard.zoom(1.5);
    // Zoom reports a shorter visual viewport with nothing covering the page;
    // shrinking to it would leave a dead band the user can't scroll to.
    await keyboard.expectFills(overview.mobileSession.root);
  });

  test("dialogs stay above the keyboard", async ({ page }) => {
    const keyboard = new SoftKeyboard(page);
    await keyboard.install();
    const settings = new AppSettingsPage(page);
    await page.setViewportSize({ width: 375, height: 812 });
    await settings.open();

    await keyboard.open(KEYBOARD);
    await keyboard.expectClears(settings.dialog, KEYBOARD);
  });
});
