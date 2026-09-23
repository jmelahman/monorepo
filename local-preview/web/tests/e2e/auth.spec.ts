import { expect, test } from "@playwright/test";

// The E2E backend boots without --sso-* flags, so SSO is disabled. In that
// mode the API reports the viewer as anonymous and the dashboard renders with
// no login wall — the backward-compatible default that keeps every other spec
// (and existing deployments) working unchanged.

test("reports anonymous when SSO is disabled", async ({ request }) => {
  const res = await request.get("/api/auth/me");
  expect(res.status()).toBe(200);
  expect(await res.json()).toMatchObject({ anonymous: true });
});

test("renders the dashboard without a sign-in wall", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByRole("heading", { name: "Previews" })).toBeVisible();
  await expect(page.getByRole("link", { name: /Sign in with GitHub/ })).toHaveCount(0);
});
