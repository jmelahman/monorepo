import { expect, type Locator, type Page } from "@playwright/test";

// Page objects for the Overview view (`web/src/components/Overview/`).
// Each surface — sidebar resizer, board tree node, tree ticket, session
// panel — gets its own class. Specs invoke high-level actions rather than
// building locators inline (see tests/e2e/README.md).

const STORAGE_KEYS = [
  "overview.panels",
  "overview.tree.collapsed",
  "overview.sidebar.width",
  "overview.sidebar.collapsed",
];

export class OverviewPage {
  readonly sidebar: Sidebar;
  readonly canvas: Locator;
  readonly emptyState: Locator;
  readonly newTicketModal: NewTicketModal;

  constructor(readonly page: Page) {
    this.sidebar = new Sidebar(page);
    this.canvas = page.locator('[data-overview-canvas="true"]');
    this.emptyState = page.getByText("Click a ticket on the left to open a session panel.");
    this.newTicketModal = new NewTicketModal(page);
  }

  // Force the Overview view on and wipe every overview localStorage key so
  // each test starts from a known layout. The seed fixture creates a fresh
  // board per test, so cross-test leakage is the only failure mode we have
  // to prevent. The init script gates on a sessionStorage flag so it runs
  // exactly once per page session — otherwise `page.reload()` would wipe
  // the layout state the test under test just persisted to localStorage.
  async goto() {
    await this.page.addInitScript((keys) => {
      if (sessionStorage.getItem("__overview_test_init") === "1") return;
      sessionStorage.setItem("__overview_test_init", "1");
      localStorage.setItem("app.view", "overview");
      for (const k of keys) localStorage.removeItem(k);
    }, STORAGE_KEYS);
    await this.page.goto("/");
    await this.waitForReady();
  }

  async reload() {
    await this.page.reload();
    await this.waitForReady();
  }

  // Ready signal that covers every Overview layout:
  //   - Desktop expanded → "Boards" heading inside <aside>.
  //   - Desktop collapsed → no aside, but the canvas is mounted.
  //   - Mobile → BoardTree at root (heading present, no canvas).
  // `Locator.or()` resolves as soon as any of the three is visible.
  private async waitForReady() {
    await expect(
      this.page
        .getByRole("heading", { name: "Boards" })
        .or(this.canvas)
        .or(this.sidebar.showButton)
        .first(),
    ).toBeVisible();
  }

  async setViewport(w: number, h: number) {
    await this.page.setViewportSize({ width: w, height: h });
  }

  boardNode(boardId: number): BoardTreeNode {
    return new BoardTreeNode(this.page, boardId);
  }

  treeTicket(ticketId: number): TreeTicket {
    return new TreeTicket(this.page, ticketId);
  }

  panel(ticketId: number): SessionPanel {
    return new SessionPanel(this.page, ticketId);
  }

  panels(): Locator {
    return this.page.locator("[data-panel]");
  }

  async expectPanelCount(n: number) {
    await expect(this.panels()).toHaveCount(n);
  }

  async canvasBox(): Promise<{ x: number; y: number; width: number; height: number }> {
    const box = await this.canvas.boundingBox();
    if (!box) throw new Error("canvas not visible");
    return box;
  }

  // The mobile drilldown renders a SessionView at the root in place of the
  // tree. Surfaced as a top-level Locator so the spec can assert on it
  // without reaching back into the desktop hierarchy.
  readonly mobileSession = {
    terminal: this.page.locator('[data-terminal="true"]'),
    backButton: this.page.getByRole("button", { name: "✕" }),
  };
}

export class Sidebar {
  readonly aside: Locator;
  readonly resizer: Locator;
  readonly hideButton: Locator;
  readonly showButton: Locator;

  constructor(private readonly page: Page) {
    this.aside = page.locator("aside").filter({ has: page.getByRole("heading", { name: "Boards" }) });
    this.resizer = page.getByRole("separator", { name: "Resize sidebar" });
    this.hideButton = page.getByRole("button", { name: "Hide boards sidebar" });
    this.showButton = page.getByRole("button", { name: "Show boards sidebar" });
  }

  async expectVisible() {
    await expect(this.aside).toBeVisible();
  }

  async expectCollapsed() {
    await expect(this.aside).toBeHidden();
    await expect(this.showButton).toBeVisible();
  }

  async collapse() {
    await this.hideButton.click();
  }

  async expand() {
    await this.showButton.click();
  }

  async width(): Promise<number> {
    const box = await this.aside.boundingBox();
    if (!box) throw new Error("sidebar aside not visible");
    return box.width;
  }

  // Auto-retry until the aria-valuenow stops changing and matches. The
  // resizer's aria-valuenow tracks state directly, so it's a more reliable
  // signal than the layout-derived bounding box width.
  async expectAriaWidth(n: number) {
    await expect(this.resizer).toHaveAttribute("aria-valuenow", String(n));
  }

  async expectAriaWidthInRange(min: number, max: number) {
    await expect
      .poll(async () => Number(await this.resizer.getAttribute("aria-valuenow")))
      .toBeGreaterThanOrEqual(min);
    const v = Number(await this.resizer.getAttribute("aria-valuenow"));
    if (v > max) throw new Error(`aria-valuenow ${v} > ${max}`);
  }

  // Drag the separator from its current screen position to absolute screen X.
  async dragResizeTo(targetClientX: number) {
    const box = await this.resizer.boundingBox();
    if (!box) throw new Error("resizer not visible");
    const fromX = box.x + box.width / 2;
    const fromY = box.y + box.height / 2;
    await this.page.mouse.move(fromX, fromY);
    await this.page.mouse.down();
    // Move in two steps to ensure mousemove events fire en route.
    await this.page.mouse.move((fromX + targetClientX) / 2, fromY, { steps: 5 });
    await this.page.mouse.move(targetClientX, fromY, { steps: 5 });
    await this.page.mouse.up();
  }

  async focus() {
    await this.resizer.focus();
  }

  async pressKey(key: string, opts: { shift?: boolean } = {}) {
    await this.resizer.focus();
    await this.page.keyboard.press(opts.shift ? `Shift+${key}` : key);
  }
}

export class BoardTreeNode {
  readonly root: Locator;
  readonly toggleButton: Locator;

  constructor(page: Page, boardId: number) {
    this.root = page.locator(`[data-board-node="${boardId}"]`);
    // The toggle button carries `aria-expanded`; the drag handle sibling
    // does not. Filtering on the attribute keeps this stable if a third
    // button (e.g. a board-level menu) is ever added.
    this.toggleButton = this.root.locator("button[aria-expanded]");
  }

  async expectVisible() {
    await expect(this.root).toBeVisible();
  }

  async toggle() {
    await this.toggleButton.click();
  }

  async expectCollapsed() {
    // The chevron caret loses `rotate-90` when the node collapses; columns
    // are also removed from the DOM. We assert the latter because it's
    // observable from the user's point of view.
    await expect(this.root.locator("h3")).toHaveCount(0);
  }

  async expectExpanded() {
    await expect(this.root.locator("h3").first()).toBeVisible();
  }

  column(columnId: number, columnName: string): TreeColumn {
    return new TreeColumn(this.root, columnId, columnName);
  }

  async expectTotalTickets(n: number) {
    await expect(this.toggleButton.locator("span").last()).toHaveText(String(n));
  }
}

export class TreeColumn {
  readonly root: Locator;
  readonly addButton: Locator;
  readonly titleInput: Locator;
  readonly submitButton: Locator;

  constructor(boardRoot: Locator, columnId: number, columnName: string) {
    this.root = boardRoot.locator(`[data-tree-column="${columnId}"]`);
    this.addButton = this.root.getByRole("button", {
      name: new RegExp(`Add ticket to ${escapeRe(columnName)}`, "i"),
    });
    this.titleInput = this.root.locator('input[placeholder="ticket title"]');
    this.submitButton = this.root.getByRole("button", { name: "add", exact: true });
  }

  async startAdd() {
    await this.addButton.click();
    await expect(this.titleInput).toBeVisible();
  }

  async submit(title: string) {
    await this.titleInput.fill(title);
    await this.submitButton.click();
    await expect(this.titleInput).toBeHidden();
  }
}

export class TreeTicket {
  readonly root: Locator;
  readonly button: Locator;

  constructor(private readonly page: Page, readonly ticketId: number) {
    this.root = page.locator(`[data-tree-ticket="${ticketId}"]`);
    this.button = this.root.locator("button").first();
  }

  async click() {
    await this.button.click();
  }

  async expectVisible() {
    await expect(this.root).toBeVisible();
  }

  async expectActive() {
    await expect(this.button).toHaveAttribute("aria-current", "true");
  }

  async expectInactive() {
    await expect(this.button).not.toHaveAttribute("aria-current", /.*/);
  }

  async expectStatus(status: string) {
    await expect(this.button).toHaveAttribute("data-session-status", status);
  }

  async dragTo(target: TreeTicket | TreeColumn) {
    // dnd-kit PointerSensor activation distance = 5px. Move twice past
    // threshold so the activation fires before the target hover.
    const fromBox = await this.button.boundingBox();
    const toBox = await (target instanceof TreeTicket ? target.button : target.root).boundingBox();
    if (!fromBox || !toBox) throw new Error("drag endpoints not visible");
    const fromX = fromBox.x + fromBox.width / 2;
    const fromY = fromBox.y + fromBox.height / 2;
    const toX = toBox.x + toBox.width / 2;
    const toY = toBox.y + toBox.height / 2;
    await this.page.mouse.move(fromX, fromY);
    await this.page.mouse.down();
    await this.page.mouse.move(fromX + 8, fromY + 8, { steps: 3 });
    await this.page.mouse.move(toX, toY, { steps: 10 });
    await this.page.mouse.up();
  }
}

export class SessionPanel {
  readonly root: Locator;
  readonly dragHandle: Locator;
  readonly maximizeButton: Locator;
  readonly restoreButton: Locator;

  constructor(private readonly page: Page, readonly ticketId: number) {
    this.root = page.locator(`[data-panel="${ticketId}"]`);
    // The drag handle is the bar at the top of the panel; aria-label is
    // unique within the panel.
    this.dragHandle = this.root.getByRole("button", { name: "Move panel" });
    this.maximizeButton = this.root.getByRole("button", { name: "Maximize" });
    this.restoreButton = this.root.getByRole("button", { name: "Restore" });
  }

  async expectMounted() {
    await expect(this.root).toBeVisible();
  }

  async expectAbsent() {
    await expect(this.root).toHaveCount(0);
  }

  async expectTile(slot: string) {
    await expect(this.root).toHaveAttribute("data-tile", slot);
  }

  async expectFocused() {
    await expect(this.root).toHaveAttribute("data-focused", "true");
  }

  // react-rnd places transform on the outer Rnd wrapper; we read it back
  // through the bounding box of the inner panel root for assertions.
  async box(): Promise<{ x: number; y: number; width: number; height: number }> {
    const box = await this.root.boundingBox();
    if (!box) throw new Error(`panel ${this.ticketId} not visible`);
    return box;
  }

  async dragTo(canvasX: number, canvasY: number) {
    const handle = await this.dragHandle.boundingBox();
    const canvasBox = await this.page.locator('[data-overview-canvas="true"]').boundingBox();
    if (!handle || !canvasBox) throw new Error("drag endpoints not visible");
    const fromX = handle.x + handle.width / 2;
    const fromY = handle.y + handle.height / 2;
    const toX = canvasBox.x + canvasX;
    const toY = canvasBox.y + canvasY;
    await this.page.mouse.move(fromX, fromY);
    await this.page.mouse.down();
    // react-draggable doesn't gate on distance, but going through an
    // intermediate point makes the path inspectable in trace viewer.
    await this.page.mouse.move((fromX + toX) / 2, (fromY + toY) / 2, { steps: 8 });
    await this.page.mouse.move(toX, toY, { steps: 8 });
    await this.page.mouse.up();
  }

  async doubleClickHandle() {
    await this.dragHandle.dblclick();
  }

  async close() {
    // SessionView's close button is a "✕" icon-only button with no
    // aria-label — accessible name comes from textContent.
    await this.root.getByRole("button", { name: "✕" }).click();
  }

  // Bring focus to this panel without dragging — clicking the drag handle
  // selects the panel but doesn't move it.
  async focus() {
    await this.dragHandle.click();
  }
}

export class NewTicketModal {
  readonly root: Locator;
  readonly boardSelect: Locator;
  readonly titleInput: Locator;
  readonly submit: Locator;
  readonly cancel: Locator;

  constructor(page: Page) {
    this.root = page.getByRole("dialog");
    this.boardSelect = this.root.getByRole("combobox", { name: "Board" });
    this.titleInput = this.root.getByRole("textbox").first();
    this.submit = this.root.getByRole("button", { name: /^create/i });
    this.cancel = this.root.getByRole("button", { name: /^cancel$/i });
  }

  async expectOpen() {
    await expect(this.root).toBeVisible();
  }

  async expectClosed() {
    await expect(this.root).toBeHidden();
  }
}

function escapeRe(s: string): string {
  return s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}
