import { test, expect } from "./fixtures/seed";
import { AppSettingsPage } from "./pages/AppSettingsPage";
import { OverviewPage } from "./pages/OverviewPage";

// Installable app + Web Push. Actually subscribing needs a real push service
// (headless Chromium has none), so these cover what the browser needs to
// offer install and what the user sees on the way to enabling notifications.

test.describe("Mobile / install and notifications", () => {
  test("serves a manifest, its icons and an active service worker", async ({ page }) => {
    const settings = new AppSettingsPage(page);
    await settings.open();
    await settings.expectInstallable();
    await settings.expectServiceWorkerActive();
  });

  test("settings offer per-device notifications, off until enabled", async ({ page }) => {
    const settings = new AppSettingsPage(page);
    await settings.open();
    await settings.openTab("notifications");

    await expect(settings.notifications.enable).not.toBeChecked();
    // The toggle plus three kinds: awaiting permission, finished,
    // failed/stopped. Nothing to test until this device is subscribed.
    await expect(settings.notifications.checkboxes).toHaveCount(4);
    await expect(settings.notifications.testButton).toBeDisabled();
  });

  test("a notification's link opens that ticket's session", async ({ page, seed }) => {
    const overview = new OverviewPage(page);
    await overview.setViewport(375, 812);
    await overview.gotoTicketLink(seed.board.id, seed.ticket.id);

    await expect(overview.mobileSession.root).toBeVisible();
    await expect(overview.mobileSession.root).toContainText(seed.ticket.title);
    // The parameters are consumed, so a reload doesn't reopen the ticket.
    await expect(page).toHaveURL(/\/$/);
  });
});
