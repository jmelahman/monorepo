import { expect, type Locator, type Page } from "@playwright/test";

// Page objects for the Releases tab (`web/src/components/ReleasesDashboard.tsx`).

export class ReleasesPage {
  readonly confirm: ReleaseConfirm;

  constructor(readonly page: Page) {
    this.confirm = new ReleaseConfirm(page);
  }

  // Refreshes because a board created after the server's last refresh has no
  // state yet.
  async goto() {
    await this.page.goto("/");
    await this.page
      .getByRole("navigation")
      .getByRole("button", { name: "release", exact: true })
      .click();
    const refresh = this.page.getByRole("button", { name: "refresh", exact: true });
    await refresh.click();
    await expect(refresh).toBeEnabled();
    await expect(this.page.getByText("Unexpected end of JSON input")).toBeHidden();
  }

  async viewAll() {
    await this.page.getByLabel("view all").check();
  }

  async stopViewingAll() {
    await this.page.getByLabel("view all").uncheck();
  }

  project(repoPath: string) {
    return new ReleaseProject(this.page, repoPath);
  }

  reminder(note: string) {
    return new ReleaseReminderCard(this.page, note);
  }

  async expectLogContains(text: string) {
    await expect(this.page.getByTestId("release-log")).toContainText(text);
  }

  async closeLog() {
    await this.page.getByRole("button", { name: "close", exact: true }).click();
    await expect(this.page.getByTestId("release-log")).toBeHidden();
  }
}

class ReleaseProject {
  readonly root: Locator;

  constructor(
    readonly page: Page,
    readonly repoPath: string,
  ) {
    this.root = page
      .locator("section")
      .filter({ has: page.getByRole("heading", { name: repoPath }) });
  }

  private row(prefix: string) {
    return this.root.getByRole("row").filter({ hasText: prefix });
  }

  async setFiltered(prefix: string, shown: boolean) {
    await this.page.getByRole("button", { name: /^filter/ }).click();
    const modal = this.page.getByRole("dialog");
    await modal.getByRole("group", { name: this.repoPath }).getByLabel(prefix).setChecked(shown);
    await modal.getByRole("button", { name: "done" }).click();
    await expect(modal).toBeHidden();
  }

  async expectSubtree(prefix: string) {
    await expect(this.row(prefix)).toBeVisible();
  }

  async expectSuggested(prefix: string, suggestion: string) {
    await expect(this.row(prefix)).toContainText(suggestion);
  }

  async expectDeployment(prefix: string, status: string) {
    await expect(this.row(prefix).getByRole("button", { name: status, exact: true })).toBeVisible();
  }

  async openLog(prefix: string, status: string) {
    await this.row(prefix).getByRole("button", { name: status, exact: true }).click();
  }

  // The log is below the table, not above what's being released.
  async expectLogBelowTable() {
    const log = this.page.getByTestId("release-log");
    await expect(async () => {
      const table = await this.root.getByRole("table").boundingBox();
      const logBox = await log.boundingBox();
      expect(logBox?.y).toBeGreaterThan((table?.y ?? 0) + (table?.height ?? 0) - 1);
    }).toPass();
  }

  async expectNothingShown() {
    await expect(this.root.getByText("No subtree here is selected.")).toBeVisible();
    await expect(this.root.getByRole("table")).toBeHidden();
  }

  async openRelease(prefix: string) {
    await this.row(prefix).getByRole("button", { name: "release", exact: true }).click();
  }

  async remind(prefix: string, bump: string, note: string) {
    await this.openRelease(prefix);
    await this.page.getByRole("button", { name: "remind me later" }).click();
    const modal = this.page.getByRole("dialog");
    await modal.getByLabel("Bump").selectOption(bump);
    await modal.getByRole("textbox").fill(note);
    await modal.getByRole("button", { name: "save" }).click();
    await expect(modal).toBeHidden();
  }
}

class ReleaseReminderCard {
  readonly root: Locator;

  constructor(page: Page, note: string) {
    this.root = page.getByRole("listitem").filter({ hasText: note });
  }

  async expectBump(bump: string) {
    await expect(this.root).toContainText(bump);
  }

  async expectReleasedSince(version: string) {
    await expect(this.root).toContainText(`${version} was released`);
  }

  async openRelease() {
    await this.root.getByRole("button", { name: "release", exact: true }).click();
  }

  async expectGone() {
    await expect(this.root).toBeHidden();
  }
}

class ReleaseConfirm {
  readonly root: Locator;
  readonly releaseButton: Locator;

  constructor(page: Page) {
    this.root = page.getByRole("dialog");
    this.releaseButton = this.root.getByRole("button", { name: "release", exact: true });
  }

  async expectBump(bump: string) {
    await expect(this.root.getByLabel("Bump")).toHaveValue(bump);
  }

  async expectVersion(version: string) {
    await expect(this.root).toContainText(version);
  }

  async expectBlocked(reason: string) {
    await expect(this.root).toContainText(reason);
    await expect(this.releaseButton).toBeDisabled();
  }

  async pushToUpstream() {
    await this.root.getByLabel("to the upstream now").check();
  }

  async release() {
    await this.releaseButton.click();
  }
}
