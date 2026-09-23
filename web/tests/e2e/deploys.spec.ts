import { execFileSync } from "node:child_process";
import { appendFileSync, cpSync, mkdtempSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { expect, type Page, test } from "@playwright/test";

// Drives the whole vertical slice against the real backend: register the Go
// fixture repo, deploy a commit, watch the dashboard, then open the real
// preview subdomain (Chromium resolves *.localhost natively).

const FIXTURE_SRC = resolve(
  dirname(fileURLToPath(import.meta.url)),
  "../../../internal/build/testdata/fixture-repo",
);

function git(dir: string, ...args: string[]): string {
  return execFileSync("git", args, {
    cwd: dir,
    env: {
      ...process.env,
      GIT_AUTHOR_NAME: "e2e",
      GIT_AUTHOR_EMAIL: "e2e@example.com",
      GIT_COMMITTER_NAME: "e2e",
      GIT_COMMITTER_EMAIL: "e2e@example.com",
    },
  })
    .toString()
    .trim();
}

let repoDir: string;
let sha: string;

test.beforeAll(() => {
  repoDir = mkdtempSync(join(tmpdir(), "preview-fixture-"));
  cpSync(FIXTURE_SRC, repoDir, { recursive: true });
  // A downloadable artifact alongside the two sides, with one file per
  // "platform", so the dashboard's download menu is exercised end-to-end.
  appendFileSync(
    join(repoDir, "preview.toml"),
    `
[artifacts.cli]
path  = "backend"
build = [["sh", "-c", "mkdir -p bin && echo linux-build > bin/fixture-cli-linux && echo darwin-build > bin/fixture-cli-darwin"]]
files = ["bin/fixture-cli-linux", "bin/fixture-cli-darwin"]
`,
  );
  git(repoDir, "init", "-q", "-b", "main");
  git(repoDir, "add", "-A");
  git(repoDir, "commit", "-qm", "initial");
  sha = git(repoDir, "rev-parse", "HEAD");
});

// Registration clones in the background; wait until the repo is deployable.
async function waitRepoReady(page: Page, name: string) {
  await expect
    .poll(
      async () => {
        const r = await page.request.get(`/api/repos/${name}`);
        const repo = await r.json();
        if (repo.status === "failed") throw new Error(`clone failed: ${repo.error}`);
        return repo.status;
      },
      { timeout: 60_000, intervals: [250] },
    )
    .toBe("ready");
}

test("register, deploy, and open a preview", async ({ page }) => {
  test.setTimeout(180_000);

  const repoRes = await page.request.post("/api/repos", {
    data: { name: "fixture", source: repoDir },
  });
  expect(repoRes.status()).toBe(202);
  expect((await repoRes.json()).status).toBe("cloning");
  await waitRepoReady(page, "fixture");

  const depRes = await page.request.post("/api/deploys", {
    data: { repo: "fixture", ref: sha },
  });
  expect(depRes.status()).toBe(202);
  const deploy = await depRes.json();

  // Artifacts build on after the deploy turns ready (they never gate the
  // preview), so wait for both the deploy and its artifact to finish.
  let latest = deploy;
  await expect
    .poll(
      async () => {
        const r = await page.request.get(`/api/deploys/${deploy.id}`);
        latest = await r.json();
        if (latest.status === "failed" || latest.artifacts?.[0]?.status === "failed") {
          const logs = await page.request.get(`/api/deploys/${deploy.id}/logs`);
          throw new Error(`deploy failed: ${latest.error}\n${await logs.text()}`);
        }
        return latest.status === "ready" ? latest.artifacts?.[0]?.status : latest.status;
      },
      { timeout: 120_000, intervals: [500] },
    )
    .toBe("ready");

  // Dashboard lists the deploy with a preview link and the commit's
  // metadata: the branch is derived even for this deploy-by-sha, and the
  // author comes from the fixture commit. Fresh builds auto-start their
  // processes, so the badge converges on the warm state.
  await page.goto("/");
  await expect(page.getByText(latest.short_sha)).toBeVisible();
  await expect(page.getByText("running", { exact: true })).toBeVisible();
  await expect(page.getByText("main", { exact: true })).toBeVisible();
  await expect(page.getByText("by e2e")).toBeVisible();
  await expect(page.getByRole("link", { name: /open/ })).toBeVisible();

  // The per-service content hashes moved off the row into the commit sha's
  // hover text.
  await expect(page.getByText(latest.short_sha)).toHaveAttribute(
    "title",
    new RegExp(`backend ${latest.be_hash.slice(0, 12)}`),
  );

  // The declared artifact is listed on the deploy (files sorted by name) and
  // downloadable both via the API and the dashboard's per-artifact download
  // menu, which lists one entry per file.
  expect(latest.artifacts).toEqual([
    {
      name: "cli",
      hash: expect.any(String),
      status: "ready",
      files: [
        {
          name: "fixture-cli-darwin",
          size: 13,
          url: expect.stringMatching(/fixture-cli-darwin$/),
        },
        { name: "fixture-cli-linux", size: 12, url: expect.stringMatching(/fixture-cli-linux$/) },
      ],
    },
  ]);
  const download = page.getByRole("button", { name: "cli" });
  await expect(download).toBeVisible();
  await download.click();
  const menuItem = page.getByRole("menuitem", { name: /fixture-cli-linux/ });
  await expect(menuItem).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(menuItem).toBeHidden();
  const linux = latest.artifacts[0].files.find(
    (f: { name: string }) => f.name === "fixture-cli-linux",
  );
  const dlRes = await page.request.get(linux.url);
  expect(dlRes.status()).toBe(200);
  expect(dlRes.headers()["content-disposition"]).toContain("attachment");
  expect(await dlRes.text()).toBe("linux-build\n");

  // The preview itself, at its real subdomain, served by Host routing.
  await page.goto(latest.preview_url);
  await expect(page.getByText("fixture frontend v1")).toBeVisible();

  // /api on the preview host reaches the fixture backend (cold start
  // included).
  const health = await page.evaluate(() => fetch("/api/health").then((r) => r.text()));
  expect(health).toBe("ok");
  const counter = await page.evaluate(() => fetch("/api/counter").then((r) => r.text()));
  expect(counter).toBe("1");

  // Back on the dashboard the deploy now shows as warm.
  await page.goto("/");
  await expect(page.getByText("running", { exact: true })).toBeVisible();

  // The observability panel: the run log tails the live process output and
  // the stats strip samples the warm process.
  await page.getByRole("button", { name: "logs" }).click();
  await expect(page.getByText(/fixture backend listening on/)).toBeVisible();
  await expect(page.getByText("start attempt 1")).toBeVisible();
  await expect(page.getByText(/[KMG]iB/).first()).toBeVisible({ timeout: 10_000 });

  // The pane's copy button lifts the tailed text into the clipboard and
  // confirms in place.
  await page.context().grantPermissions(["clipboard-read", "clipboard-write"]);
  await page.getByRole("button", { name: "Copy", exact: true }).click();
  await expect(page.getByRole("button", { name: "Copied", exact: true })).toBeVisible();
  expect(await page.evaluate(() => navigator.clipboard.readText())).toMatch(
    /fixture backend listening on/,
  );

  // Build logs are one tab over.
  await page.getByRole("button", { name: "build log" }).click();
  await expect(page.getByText("--- backend build ---")).toBeVisible();
});

test("evict and redeploy from the dashboard", async ({ page }) => {
  test.setTimeout(180_000);

  const waitReady = async (id: number) => {
    let latest: { status: string; error?: string } = { status: "" };
    await expect
      .poll(
        async () => {
          const r = await page.request.get(`/api/deploys/${id}`);
          latest = await r.json();
          if (latest.status === "failed") {
            const logs = await page.request.get(`/api/deploys/${id}/logs`);
            throw new Error(`deploy failed: ${latest.error}\n${await logs.text()}`);
          }
          return latest.status;
        },
        { timeout: 120_000, intervals: [500] },
      )
      .toBe("ready");
    return latest;
  };

  // Self-contained when run alone: registering again after the first test
  // conflicts (409) and re-deploying a ready commit is a no-op.
  const repoRes = await page.request.post("/api/repos", {
    data: { name: "fixture", source: repoDir },
  });
  expect([202, 409]).toContain(repoRes.status());
  await waitRepoReady(page, "fixture");
  const dep1 = await (
    await page.request.post("/api/deploys", { data: { repo: "fixture", ref: sha } })
  ).json();

  // A newer commit gives the sweep a deploy to keep. Only the frontend
  // changes, so the shared backend half survives eviction and the later
  // rebuild stays cheap.
  writeFileSync(join(repoDir, "web", "src", "index.html"), "fixture frontend v2");
  git(repoDir, "commit", "-qam", "v2");
  const dep2 = await (
    await page.request.post("/api/deploys", {
      data: { repo: "fixture", ref: git(repoDir, "rev-parse", "HEAD") },
    })
  ).json();
  await waitReady(dep1.id);
  await waitReady(dep2.id);

  // Tighten retention to one deploy per repo and sweep: the newest ready
  // deploy is protected, so the older one is evicted. Relax the policy again
  // right away so it can't surprise any later test.
  const putRes = await page.request.put("/api/retention", {
    data: { max_deploys_per_repo: 1, max_age_days: 0 },
  });
  expect(putRes.ok()).toBeTruthy();
  const gc = await (await page.request.post("/api/gc")).json();
  await page.request.put("/api/retention", { data: { max_deploys_per_repo: 0, max_age_days: 0 } });
  expect(gc.evicted.map((e: { id: number }) => e.id)).toContain(dep1.id);

  // The evicted row trades its preview link for a redeploy button.
  await page.goto("/");
  const row = page.locator("li").filter({ hasText: dep1.short_sha });
  await expect(row.getByText("evicted", { exact: true })).toBeVisible();
  await expect(row.getByRole("link", { name: /open/ })).toBeHidden();

  // Redeploying re-queues the build and the revived preview serves the
  // original commit's content again.
  await row.getByRole("button", { name: "redeploy" }).click();
  await expect(row.getByText("evicted", { exact: true })).toBeHidden();
  const revived = await waitReady(dep1.id);
  await page.goto((revived as { preview_url?: string }).preview_url ?? "");
  await expect(page.getByText("fixture frontend v1")).toBeVisible();
});

test("search and status filters narrow the deployments list", async ({ page }) => {
  test.setTimeout(180_000);

  const waitReady = async (id: number) => {
    await expect
      .poll(
        async () => {
          const r = await page.request.get(`/api/deploys/${id}`);
          const latest = await r.json();
          if (latest.status === "failed") throw new Error(`deploy failed: ${latest.error}`);
          return latest.status;
        },
        { timeout: 120_000, intervals: [500] },
      )
      .toBe("ready");
  };

  // Self-contained: ensure the fixture repo and two distinct deploys exist
  // (a fresh commit guarantees the second even when run alone).
  const repoRes = await page.request.post("/api/repos", {
    data: { name: "fixture", source: repoDir },
  });
  expect([202, 409]).toContain(repoRes.status());
  await waitRepoReady(page, "fixture");
  const dep1 = await (
    await page.request.post("/api/deploys", { data: { repo: "fixture", ref: sha } })
  ).json();
  writeFileSync(join(repoDir, "web", "src", "index.html"), "fixture frontend v3");
  git(repoDir, "commit", "-qam", "v3");
  const dep2 = await (
    await page.request.post("/api/deploys", {
      data: { repo: "fixture", ref: git(repoDir, "rev-parse", "HEAD") },
    })
  ).json();
  await waitReady(dep1.id);
  await waitReady(dep2.id);

  // The API narrows by free-text query and rejects unknown statuses.
  const byQ = await (await page.request.get(`/api/deploys?q=${dep2.short_sha}`)).json();
  expect(byQ.map((d: { id: number }) => d.id)).toEqual([dep2.id]);
  expect((await page.request.get("/api/deploys?status=bogus")).status()).toBe(400);

  // Searching for one commit's sha hides the other row.
  await page.goto("/");
  const rows = page.locator("main li");
  await expect(rows.filter({ hasText: dep1.short_sha })).toBeVisible();
  await expect(rows.filter({ hasText: dep2.short_sha })).toBeVisible();
  await page.getByRole("searchbox", { name: "Search deployments" }).fill(dep1.short_sha);
  await expect(rows.filter({ hasText: dep2.short_sha })).toBeHidden();
  await expect(rows.filter({ hasText: dep1.short_sha })).toBeVisible();
  await page.getByRole("searchbox", { name: "Search deployments" }).fill("");

  // No deploy in the suite ever ends up failed, so the status filter lands
  // on the no-match state, and clearing restores the list.
  await page.getByRole("combobox", { name: "Filter by status" }).selectOption("failed");
  await expect(page.getByText("No matching deployments")).toBeVisible();
  await page.getByRole("button", { name: "Clear filters" }).click();
  await expect(rows.filter({ hasText: dep1.short_sha })).toBeVisible();
  await expect(rows.filter({ hasText: dep2.short_sha })).toBeVisible();
});

test("register dialog tracks the background clone and closes itself", async ({ page }) => {
  await page.goto("/");
  await page.getByRole("button", { name: "register repo" }).click();
  await page.getByRole("textbox", { name: "Name" }).fill("dialogfixture");
  await page.getByRole("textbox", { name: "Source" }).fill(repoDir);
  await page.getByRole("button", { name: "Register", exact: true }).click();

  // The form is replaced by the cloning phase, which dismisses the dialog
  // on its own once the repo turns ready.
  await expect(page.getByRole("dialog")).toBeHidden({ timeout: 60_000 });

  await page.getByRole("button", { name: "Manage registered repositories" }).click();
  await expect(page.getByText("dialogfixture")).toBeVisible();
});

test("register dialog surfaces a failed clone and offers a retry", async ({ page }) => {
  await page.goto("/");
  await page.getByRole("button", { name: "register repo" }).click();
  await page.getByRole("textbox", { name: "Name" }).fill("badclone");
  await page.getByRole("textbox", { name: "Source" }).fill(join(tmpdir(), "no-such-repo"));
  await page.getByRole("button", { name: "Register", exact: true }).click();

  await expect(page.getByRole("button", { name: "Edit & retry" })).toBeVisible({
    timeout: 30_000,
  });
  await expect(page.getByText(/repository not found/)).toBeVisible();

  // Edit & retry frees the name and returns to the form with its values.
  await page.getByRole("button", { name: "Edit & retry" }).click();
  await expect(page.getByRole("textbox", { name: "Name" })).toHaveValue("badclone");
  await page.getByRole("textbox", { name: "Source" }).fill(repoDir);
  await page.getByRole("button", { name: "Register", exact: true }).click();
  await expect(page.getByRole("dialog")).toBeHidden({ timeout: 60_000 });
  await waitRepoReady(page, "badclone");
});

test("deploy dialog tracks the build through to a servable preview", async ({ page }) => {
  test.setTimeout(180_000);

  const repoRes = await page.request.post("/api/repos", {
    data: { name: "fixture", source: repoDir },
  });
  expect([202, 409]).toContain(repoRes.status());
  await waitRepoReady(page, "fixture");
  // A fresh commit: re-deploying a sha the suite already built conflicts.
  writeFileSync(join(repoDir, "web", "src", "index.html"), "fixture frontend v4");
  git(repoDir, "commit", "-qam", "v4");
  const fresh = git(repoDir, "rev-parse", "HEAD");

  await page.goto("/");
  await page.locator("main").getByRole("button", { name: "Deploy", exact: true }).click();
  const dialog = page.getByRole("dialog");
  await dialog.getByRole("combobox", { name: "Repository" }).selectOption("fixture");
  await dialog.getByRole("textbox", { name: "Ref" }).fill(fresh);
  await dialog.getByRole("button", { name: "Deploy", exact: true }).click();

  // The form is replaced by the progress view, which stays put — tailing the
  // build log — until the preview is actually servable.
  await expect(dialog.getByText(/Building content-addressed artifacts/)).toBeVisible();
  await expect(dialog.getByText(/--- backend build ---/)).toBeVisible({ timeout: 120_000 });
  await expect(dialog.getByText("Deploy finished.")).toBeVisible({ timeout: 120_000 });

  const open = dialog.getByRole("link", { name: /Open preview/ });
  await expect(open).toBeVisible();
  await page.goto((await open.getAttribute("href")) ?? "");
  await expect(page.getByText("fixture frontend v4")).toBeVisible();
});
