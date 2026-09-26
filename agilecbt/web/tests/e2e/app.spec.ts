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

test("standup is a coach chat, plan a step on the board, complete it", async ({ page }) => {
  const run = uid();
  await page.goto("/");
  // On a reused server the check-in may already exist; edit its readings.
  const fresh = !(await (await page.request.get("/api/today")).json()).morning;
  if (fresh) await expect(page.getByText("Morning. How are you doing?")).toBeVisible();
  else await page.getByRole("button", { name: /^(Mood|How are you feeling)/ }).click();
  await page.getByLabel("Mood", { exact: true }).fill("4");
  await page.getByLabel("Energy", { exact: true }).fill("3");
  await page.getByLabel("Anxiety", { exact: true }).fill("6");
  if (fresh) {
    // The first answer starts the check-in; the coach leads from there.
    await page.getByLabel("Message").fill(`Pretty tired ${run}`);
    await page.getByRole("button", { name: "Send" }).click();
    await expect(page.getByText(`Pretty tired ${run}`)).toBeVisible();
    const chip = page.getByTestId("action-chip").filter({ hasText: "Short walk" }).last();
    await expect(chip).toBeVisible();
    await expect(page.getByText("Done, a short walk is on Today. Go gently.")).toBeVisible();
    await chip.getByRole("button", { name: /^Undo/ }).click();
    await expect(chip.getByText("undone")).toBeVisible();
    // After a reload the greeting is part of the saved conversation.
    await page.reload();
    await expect(page.getByText("Morning. How are you doing?")).toBeVisible();
    await expect(page.getByText(`Pretty tired ${run}`)).toBeVisible();
  }
  await expect(page.getByText("Mood 4 · Energy 3 · Anxiety 6")).toBeVisible();

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
  await expect(page.getByRole("listitem").filter({ hasText: title })).toContainText("M 6 · P 7");
});

test("thought record walkthrough", async ({ page }) => {
  const run = uid();
  await page.goto("/thoughts");
  await page.getByRole("button", { name: "New record" }).click();
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

test("coach chat shows an action chip that can be undone", async ({ page }) => {
  await ensureCheckin(page);
  await page.goto("/");
  const walks = page.getByRole("button", { name: "Complete Short walk" });
  await expect(page.getByLabel("Message")).toBeVisible();
  const before = await walks.count();
  await page.getByLabel("Message").fill("Work is a lot today");
  await page.getByRole("button", { name: "Send" }).click();

  const chip = page.getByTestId("action-chip").filter({ hasText: "Short walk" }).last();
  await expect(chip).toBeVisible();
  await expect(page.getByText("Done, a short walk is on Today. Go gently.").last()).toBeVisible();
  await expect(walks).toHaveCount(before + 1);

  await chip.getByRole("button", { name: /^Undo/ }).click();
  await expect(chip.getByText("undone")).toBeVisible();
  await expect(walks).toHaveCount(before);
});

test("roadmap is a coach conversation that persists", async ({ page }) => {
  const run = uid();
  await page.goto("/roadmap");
  // The coach opens, with suggestions above the composer.
  await expect(
    page.getByText(/^(What matters to you\?|What would you like to change)/),
  ).toBeVisible();
  await expect(
    page
      .getByRole("button", { name: "Break a goal into small steps" })
      .or(page.getByRole("button", { name: "Help me figure out what matters to me" })),
  ).toBeVisible();
  await page.getByLabel("Message").fill(`I want to feel healthier ${run}`);
  await page.getByRole("button", { name: "Send" }).click();
  await expect(
    page.getByTestId("action-chip").filter({ hasText: "Short walk" }).last(),
  ).toBeVisible();
  await expect(page.getByText("Done, a short walk is on Today. Go gently.").last()).toBeVisible();

  // Today's roadmap conversation comes back after a reload, and it's not a
  // standup, so Today's check-in state is untouched.
  await page.reload();
  await expect(page.getByText(`I want to feel healthier ${run}`)).toBeVisible();
  // The outline sits beside the chat on wide screens.
  await expect(
    page.getByRole("complementary").getByRole("heading", { name: "Your roadmap" }),
  ).toBeVisible();

  // Undo from here too, which also leaves Today as the other tests expect.
  const chip = page.getByTestId("action-chip").filter({ hasText: "Short walk" }).last();
  await chip.getByRole("button", { name: /^Undo/ }).click();
  await expect(chip.getByText("undone")).toBeVisible();
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

test("settings is a tabbed dialog", async ({ page }) => {
  await page.goto("/board");
  await page.getByRole("button", { name: "Settings" }).click();
  const dialog = page.getByRole("dialog", { name: "Settings" });
  await expect(dialog.getByRole("tab", { name: "Coach" })).toHaveAttribute("aria-selected", "true");
  await expect(dialog.getByText("What your coach remembers")).toBeVisible();
  await dialog.getByRole("tab", { name: "Coach" }).press("ArrowRight");
  await expect(dialog.getByRole("tab", { name: "AI model" })).toHaveAttribute(
    "aria-selected",
    "true",
  );
  await expect(page).toHaveURL(/\/board\?settings=model$/);
  await page.reload();
  await expect(dialog.getByRole("combobox", { name: "Model", exact: true })).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(dialog).toBeHidden();
  await expect(page).toHaveURL(/\/board$/);
  // The old page URL still lands on the dialog.
  await page.goto("/settings");
  await expect(dialog.getByText("What your coach remembers")).toBeVisible();
});

test("crisis lines are in Settings, read-only", async ({ page }) => {
  await page.goto("/?settings=support");
  const dialog = page.getByRole("dialog", { name: "Settings" });
  await expect(dialog.getByText(/call or text 988/)).toBeVisible();
  await expect(dialog.getByRole("link", { name: "https://findahelpline.com" })).toHaveAttribute(
    "href",
    "https://findahelpline.com",
  );
  await expect(dialog.getByRole("textbox")).toHaveCount(0);
});

test("the coach's model is set in Settings", async ({ page }) => {
  await page.goto("/?settings=model");
  const dialog = page.getByRole("dialog");
  await expect(dialog.getByText("Ready", { exact: true })).toBeVisible();
  const model = dialog.getByRole("combobox", { name: "Model", exact: true });
  await expect(model).toHaveAttribute("placeholder", "fake");

  // A model the server doesn't have shows why the coach is unavailable.
  await model.fill("missing");
  await dialog.getByRole("button", { name: "Save" }).click();
  await expect(dialog.getByText("Unavailable", { exact: true })).toBeVisible();
  await expect(dialog.getByText(/ollama pull missing/)).toBeVisible();

  await dialog.getByRole("button", { name: "Reset to config" }).click();
  await expect(dialog.getByText("Ready", { exact: true })).toBeVisible();
  await expect(model).toHaveValue("");
});
