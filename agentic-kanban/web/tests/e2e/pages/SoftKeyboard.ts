import { expect, type Locator, type Page } from "@playwright/test";

// Stand-in for the mobile soft keyboard. Headless Chromium has no on-screen
// keyboard, so we swap `window.visualViewport` for a fake whose height the
// spec controls — the same signal a real keyboard produces on iOS, where the
// layout viewport keeps its size and only the visual viewport shrinks.
export class SoftKeyboard {
  constructor(private readonly page: Page) {}

  // Must run before the app loads: the app reads `visualViewport` on mount.
  async install() {
    await this.page.addInitScript(() => {
      const vv = new EventTarget() as EventTarget & {
        height: number;
        offsetTop: number;
        scale: number;
      };
      vv.height = window.innerHeight;
      vv.offsetTop = 0;
      vv.scale = 1;
      Object.defineProperty(window, "visualViewport", { value: vv, configurable: true });
    });
  }

  async open(keyboardHeight: number) {
    await this.page.evaluate((h) => {
      const vv = window.visualViewport as unknown as EventTarget & { height: number };
      vv.height = window.innerHeight - h;
      vv.dispatchEvent(new Event("resize"));
    }, keyboardHeight);
  }

  async close() {
    await this.open(0);
  }

  // Pinch zoom (or iOS zooming into a focused input): the visual viewport
  // reports fewer CSS pixels, but nothing is covering the page.
  async zoom(scale: number) {
    await this.page.evaluate((s) => {
      const vv = window.visualViewport as unknown as EventTarget & {
        height: number;
        scale: number;
      };
      vv.scale = s;
      vv.height = window.innerHeight / s;
      vv.dispatchEvent(new Event("resize"));
    }, scale);
  }

  // The surface's bottom edge sits at or above the top of the keyboard.
  async expectClears(surface: Locator, keyboardHeight: number) {
    const visible = (this.page.viewportSize()?.height ?? 0) - keyboardHeight;
    await expect
      .poll(async () => {
        const box = await surface.boundingBox();
        return box ? box.y + box.height : Number.POSITIVE_INFINITY;
      })
      .toBeLessThanOrEqual(visible);
  }

  // The surface reaches the bottom of the visible area again.
  async expectFills(surface: Locator) {
    const full = this.page.viewportSize()?.height ?? 0;
    await expect
      .poll(async () => {
        const box = await surface.boundingBox();
        return box ? box.y + box.height : 0;
      })
      .toBe(full);
  }
}
