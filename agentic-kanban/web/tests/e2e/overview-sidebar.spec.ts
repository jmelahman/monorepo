import { test, expect } from "./fixtures/seed";
import { OverviewPage } from "./pages/OverviewPage";

// Sidebar chrome: collapse/expand, drag resize, keyboard resize, and the
// localStorage round-trip behind both. Constants mirror Overview.tsx —
// keep in sync if those values change.
const DEFAULT_WIDTH = 280;
const MIN_WIDTH = 200;
const MAX_WIDTH = 600;

test.describe("Overview / Sidebar", () => {
  test("opens at the default width with an accessible resizer", async ({ page }) => {
    const overview = new OverviewPage(page);
    await overview.goto();
    await overview.sidebar.expectVisible();
    await overview.sidebar.expectAriaWidth(DEFAULT_WIDTH);
    await expect(overview.sidebar.resizer).toHaveAttribute("aria-valuemin", String(MIN_WIDTH));
    await expect(overview.sidebar.resizer).toHaveAttribute("aria-valuemax", String(MAX_WIDTH));
  });

  test("collapse hides the aside, exposes the show button, and persists", async ({ page }) => {
    const overview = new OverviewPage(page);
    await overview.goto();

    await overview.sidebar.collapse();
    await overview.sidebar.expectCollapsed();

    await overview.reload();
    await overview.sidebar.expectCollapsed();

    await overview.sidebar.expand();
    await overview.sidebar.expectVisible();
    await overview.sidebar.expectAriaWidth(DEFAULT_WIDTH);
  });

  test("drag separator updates the width and persists across reload", async ({ page }) => {
    const overview = new OverviewPage(page);
    await overview.goto();

    // Drag the separator to clientX=360. Because the aside lives at the
    // left edge, clientX of the resizer ≈ sidebar width.
    await overview.sidebar.dragResizeTo(360);
    await overview.sidebar.expectAriaWidth(360);

    await overview.reload();
    await overview.sidebar.expectAriaWidth(360);
  });

  test("drag clamps below MIN_WIDTH and above MAX_WIDTH", async ({ page }) => {
    const overview = new OverviewPage(page);
    await overview.goto();

    await overview.sidebar.dragResizeTo(50); // below MIN
    await overview.sidebar.expectAriaWidth(MIN_WIDTH);

    await overview.sidebar.dragResizeTo(2000); // above MAX
    await overview.sidebar.expectAriaWidth(MAX_WIDTH);
  });

  test("keyboard arrows step the width; Home and End jump to the bounds", async ({ page }) => {
    const overview = new OverviewPage(page);
    await overview.goto();
    await overview.sidebar.focus();

    await overview.sidebar.pressKey("ArrowRight");
    await overview.sidebar.expectAriaWidth(DEFAULT_WIDTH + 16);

    await overview.sidebar.pressKey("ArrowRight", { shift: true });
    await overview.sidebar.expectAriaWidth(DEFAULT_WIDTH + 16 + 64);

    await overview.sidebar.pressKey("ArrowLeft");
    await overview.sidebar.expectAriaWidth(DEFAULT_WIDTH + 16 + 64 - 16);

    await overview.sidebar.pressKey("Home");
    await overview.sidebar.expectAriaWidth(MIN_WIDTH);

    await overview.sidebar.pressKey("End");
    await overview.sidebar.expectAriaWidth(MAX_WIDTH);

    // Steps beyond MAX clamp without throwing.
    await overview.sidebar.pressKey("ArrowRight", { shift: true });
    await overview.sidebar.expectAriaWidth(MAX_WIDTH);
  });
});
