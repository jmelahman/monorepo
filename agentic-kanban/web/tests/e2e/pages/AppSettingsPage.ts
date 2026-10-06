import { expect, type Locator, type Page } from "@playwright/test";

// The app-wide Settings modal, opened from the header.
export class AppSettingsPage {
  readonly dialog: Locator;
  readonly notifications: {
    enable: Locator;
    // The enable toggle plus one checkbox per notification kind.
    checkboxes: Locator;
    testButton: Locator;
  };

  constructor(readonly page: Page) {
    const dialog = page.getByRole("dialog");
    this.dialog = dialog;
    this.notifications = {
      enable: dialog.getByRole("checkbox", { name: "Send notifications to this device" }),
      checkboxes: dialog.getByRole("group").getByRole("checkbox"),
      testButton: dialog.getByRole("button", { name: "send test notification" }),
    };
  }

  async open() {
    await this.page.goto("/");
    // At phone width the header collapses its buttons into a menu.
    const menu = this.page.getByRole("button", { name: "Menu" });
    const direct = this.page.getByRole("button", { name: "App settings" });
    await expect(menu.or(direct).first()).toBeVisible();
    if (await direct.isVisible()) {
      await direct.click();
    } else {
      await menu.click();
      await this.page.getByRole("menuitem", { name: "App settings" }).click();
    }
  }

  async openTab(name: "general" | "appearance" | "notifications" | "shortcuts") {
    await this.page.getByRole("dialog").getByRole("button", { name, exact: true }).click();
  }

  // The app is served with a web manifest and the service worker script the
  // browser needs to offer "install" and to receive pushes.
  async expectInstallable() {
    await expect(this.page.locator('link[rel="manifest"]')).toHaveAttribute(
      "href",
      "/manifest.webmanifest",
    );
    const manifest = await this.page.request.get("/manifest.webmanifest");
    expect(manifest.ok()).toBe(true);
    const body = await manifest.json();
    expect(body.display).toBe("standalone");
    for (const icon of body.icons) {
      expect((await this.page.request.get(icon.src)).ok(), icon.src).toBe(true);
    }
    const worker = await this.page.request.get("/sw.js");
    expect(worker.ok()).toBe(true);
    expect(worker.headers()["content-type"]).toContain("javascript");
  }

  async expectServiceWorkerActive() {
    await expect
      .poll(() =>
        this.page.evaluate(async () => {
          const reg = await navigator.serviceWorker.getRegistration();
          return reg?.active?.scriptURL ?? null;
        }),
      )
      .toMatch(/\/sw\.js$/);
  }
}
