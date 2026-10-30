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
    await this.page.getByRole("button", { name: "release", exact: true }).click();
    const refresh = this.page.getByRole("button", { name: "refresh", exact: true });
    await refresh.click();
    await expect(refresh).toBeEnabled();
    await expect(this.page.getByText("Unexpected end of JSON input")).toBeHidden();
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

  async expectRun(version: string, status: string) {
    await expect(this.page.getByRole("listitem").filter({ hasText: version })).toContainText(
      status,
    );
  }
}

class ReleaseProject {
  readonly root: Locator;

  constructor(
    readonly page: Page,
    repoPath: string,
  ) {
    this.root = page.locator("section").filter({ hasText: repoPath });
  }

  private row(prefix: string) {
    return this.root.getByRole("row").filter({ hasText: prefix });
  }

  async expectSubtree(prefix: string) {
    await expect(this.row(prefix)).toBeVisible();
  }

  async openRelease(prefix: string) {
    await this.row(prefix).getByRole("button", { name: "release…" }).click();
  }

  async remind(prefix: string, bump: string, note: string) {
    await this.row(prefix).getByRole("button", { name: "remind", exact: true }).click();
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

  async openRelease() {
    await this.root.getByRole("button", { name: "release…" }).click();
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

  async expectVersion(version: string) {
    await expect(this.root).toContainText(version);
  }

  async expectBlocked(reason: string) {
    await expect(this.root).toContainText(reason);
    await expect(this.releaseButton).toBeDisabled();
  }

  async release() {
    await this.releaseButton.click();
  }
}
