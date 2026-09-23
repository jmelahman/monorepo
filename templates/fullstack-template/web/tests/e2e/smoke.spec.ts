import { expect, test } from "@playwright/test";

test("loads, adds, and deletes an item", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByRole("heading", { name: "Fullstack Template" })).toBeVisible();

  const title = `item-${Date.now()}`;
  await page.getByPlaceholder("Add an item…").fill(title);
  await page.getByRole("button", { name: "Add" }).click();
  await expect(page.getByText(title)).toBeVisible();

  await page.getByRole("button", { name: `Delete ${title}` }).click();
  await expect(page.getByText(title)).not.toBeVisible();
});
