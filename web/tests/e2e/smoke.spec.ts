import { expect, test } from "@playwright/test";

test("loads and reaches the backend", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByRole("heading", { name: "Previews" })).toBeVisible();
  await expect(page.getByText(/^server /)).toBeVisible();
});
