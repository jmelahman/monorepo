import { expect, type Locator, type Page } from "@playwright/test";

// Page objects for the surfaces our UI-driving specs touch. WS-contract
// specs that only use `page.goto("/")` for a same-origin WebSocket
// don't drive any UI and intentionally skip this layer.

export class BoardPage {
  readonly sessionPane: SessionPane;

  constructor(readonly page: Page) {
    this.sessionPane = new SessionPane(page);
  }

  async goto(boardId: number) {
    await this.page.addInitScript((id) => {
      localStorage.setItem("kanban.activeBoardId", String(id));
    }, boardId);
    await this.page.goto("/");
  }

  async reload() {
    await this.page.reload();
  }

  ticketCard(title: string): TicketCard {
    return new TicketCard(this.page, title);
  }
}

export class TicketCard {
  readonly locator: Locator;

  constructor(page: Page, title: string) {
    this.locator = page
      .locator('[data-ticket-card="true"]')
      .filter({ hasText: title });
  }

  async click() {
    await this.locator.click();
  }

  async expectVisible() {
    await expect(this.locator).toBeVisible();
  }

  async expectStatus(status: string) {
    await expect(this.locator).toContainText(status);
  }
}

export class SessionPane {
  readonly terminal: Locator;

  constructor(private readonly page: Page) {
    this.terminal = page.locator('[data-terminal="true"]');
  }

  async openTerminalTab() {
    await this.page.getByRole("button", { name: "terminal" }).click();
  }

  async stop() {
    await this.page.getByRole("button", { name: "stop", exact: true }).click();
  }

  async expectTerminalMounted() {
    await expect(this.terminal.first()).toBeVisible();
  }

  async expectTerminalAbsent() {
    await expect(this.terminal).toHaveCount(0);
  }
}
