import { expect, type Page, test } from "@playwright/test";

// Tests share one in-memory backend and run serially. They don't assume a
// fresh database (unique titles, .last()), so --repeat-each and a reused
// server both work.
const uid = () => Math.random().toString(36).slice(2, 8);
test.beforeAll(async ({ request }) => {
  // Pin the check-in to "morning" so the tests don't depend on the clock.
  const res = await request.patch("/api/settings", { data: { checkin_times: "morning" } });
  expect(res.ok()).toBeTruthy();
});

async function ensureCheckin(page: Page) {
  const snap = await (await page.request.get("/api/today")).json();
  if (!snap.morning) {
    await page.request.post("/api/checkins", { data: { kind: "morning", mood: 5, energy: 5 } });
  }
}

test("manual check-in, plan a step on the board, complete it with ratings", async ({ page }) => {
  const run = uid();
  await page.goto("/");
  // On a reused server the check-in may already exist; edit it instead.
  const heading = page.getByRole("heading", { name: "Good morning. How are you arriving?" });
  const edit = page.getByRole("button", { name: "Edit", exact: true });
  await expect(heading.or(edit)).toBeVisible();
  if (await edit.isVisible()) await edit.click();
  await expect(heading).toBeVisible();
  await page.getByLabel("Mood", { exact: true }).fill("4");
  await page.getByLabel("Energy", { exact: true }).fill("3");
  await page.getByLabel("Anxiety", { exact: true }).fill("6");
  await page.getByRole("button", { name: /^(Check in|Update check-in)$/ }).click();
  await expect(page.getByText("Morning check-in done")).toBeVisible();
  await expect(page.getByText("Mood 4 · Energy 3 · Anxiety 6")).toBeVisible();
  await expect(page.getByText(/Low energy/)).toBeVisible();

  // Add a step to This week, then drag it into Today.
  const title = `Water the plants ${run}`;
  await page.getByRole("link", { name: "Board" }).click();
  const week = page.getByRole("region", { name: "This week" });
  const today = page.getByRole("region", { name: "Today" });
  await week.getByRole("button", { name: "+ Add" }).click();
  await week.getByLabel("New step in week").fill(title);
  await week.getByRole("button", { name: "Add", exact: true }).click();
  const card = week.getByText(title);
  await expect(card).toBeVisible();

  const from = await card.boundingBox();
  const to = await today.boundingBox();
  if (!from || !to) throw new Error("no layout");
  await page.mouse.move(from.x + 10, from.y + from.height / 2);
  await page.mouse.down();
  await page.mouse.move(from.x + 30, from.y + 20, { steps: 5 });
  await page.mouse.move(to.x + to.width / 2, to.y + 80, { steps: 15 });
  await page.mouse.up();
  await expect(today.getByText(title)).toBeVisible();

  // It shows on Today; completing asks for mastery and pleasure.
  await page.getByRole("link", { name: "Today" }).click();
  await page.getByRole("button", { name: `Complete ${title}` }).click();
  const dialog = page.getByRole("dialog", { name: "Nice. That counts." });
  await dialog.getByLabel("Mastery (sense of accomplishment)").fill("6");
  await dialog.getByLabel("Pleasure (enjoyment)").fill("7");
  await dialog.getByRole("button", { name: "Save" }).click();
  await expect(page.getByText("Done today")).toBeVisible();
  await expect(page.getByRole("listitem").filter({ hasText: title })).toContainText("M 6 · P 7");
});

test("thought record walkthrough", async ({ page }) => {
  const run = uid();
  await page.goto("/thoughts");
  await page.getByRole("button", { name: "New thought record" }).click();
  await page.getByLabel("Situation").fill("Boss emailed asking to talk tomorrow");
  await page.getByRole("button", { name: "Next" }).click();
  await page.getByRole("button", { name: "+ anxious" }).click();
  await page.getByLabel("anxious intensity").fill("80");
  await page.getByRole("button", { name: "Next" }).click();
  await page.getByLabel("Automatic thought").fill(`I'm going to be fired (${run})`);
  await page.getByLabel(/Catastrophizing/).check();
  await page.getByRole("button", { name: "Next" }).click();
  await page.getByLabel("Evidence against it").fill("My last review was good");
  await page.getByRole("button", { name: "Next" }).click();
  await page
    .getByLabel("Balanced thought")
    .fill("It could be about anything, probably the project");
  await page.getByLabel("anxious intensity").fill("40");
  await page.getByRole("button", { name: "Save" }).click();

  const item = page.getByRole("button", { name: new RegExp(`fired \\(${run}\\)`) });
  await expect(item).toBeVisible();
  await item.click();
  await expect(page.getByText("anxious 80 → 40")).toBeVisible();
  await expect(page.getByText("Catastrophizing")).toBeVisible();
});

test("curator chat shows an action chip that can be undone", async ({ page }) => {
  await ensureCheckin(page);
  await page.goto("/");
  await page.getByLabel("Message").fill("Work is a lot today");
  await page.getByRole("button", { name: "Send" }).click();

  const chip = page.getByTestId("action-chip").filter({ hasText: "Short walk" }).last();
  await expect(chip).toBeVisible();
  await expect(page.getByText("Done, a short walk is on Today. Go gently.").last()).toBeVisible();
  await expect(page.getByRole("button", { name: "Complete Short walk" })).toBeVisible();

  await chip.getByRole("button", { name: /^Undo/ }).click();
  await expect(chip.getByText("undone")).toBeVisible();
  await expect(page.getByRole("button", { name: "Complete Short walk" })).toHaveCount(0);
});

test("retro drafts with AI and saves", async ({ page }) => {
  await page.goto("/retro");
  await page.getByRole("button", { name: "Draft with AI" }).click();
  await expect(page.getByLabel("What helped")).toHaveValue("You showed up for check-ins.");
  await expect(page.getByLabel("One thing to try next week")).toHaveValue(
    "One short walk after lunch.",
  );
  await page.getByRole("button", { name: "Save retro" }).click();
  await expect(page.getByText("Saved")).toBeVisible();
});

test("need help now shows crisis resources", async ({ page }) => {
  await page.goto("/");
  await page.getByRole("button", { name: "Need help now?" }).click();
  await expect(page.getByRole("dialog").getByText(/988/)).toBeVisible();
});
