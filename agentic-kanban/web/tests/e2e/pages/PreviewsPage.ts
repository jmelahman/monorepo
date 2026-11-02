import { expect, type Page } from "@playwright/test";

// Page object for the Previews tab (`web/src/components/PreviewsDashboard.tsx`).

export class PreviewsPage {
  constructor(readonly page: Page) {}

  async goto() {
    await this.page.goto("/");
    await this.page
      .getByRole("navigation")
      .getByRole("button", { name: "preview", exact: true })
      .click();
  }

  async setFiltered(board: string, shown: boolean) {
    await this.page.getByRole("button", { name: /^filter: / }).click();
    await this.page.getByLabel(board, { exact: true }).setChecked(shown);
    await this.page.getByRole("button", { name: "done", exact: true }).click();
  }

  async expectFilter(shown: number, total: number) {
    await expect(
      this.page.getByRole("button", { name: `filter: ${shown} of ${total} shown` }),
    ).toBeVisible();
  }

  private deployRow(board: string) {
    return this.page.getByRole("listitem").filter({ hasText: board });
  }

  async expectDeploy(board: string) {
    await expect(this.deployRow(board).first()).toBeVisible();
  }

  async expectNoDeploy(board: string) {
    await expect(this.deployRow(board)).toHaveCount(0);
  }
}
