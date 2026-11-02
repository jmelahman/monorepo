import { mkdtemp, rm, writeFile } from "node:fs/promises";
import { join } from "node:path";
import { test } from "@playwright/test";
import { api, type Board } from "./fixtures/api";
import { createMonorepo, git } from "./fixtures/repo";
import { mountRootFromConfig } from "./fixtures/seed";
import { PreviewsPage } from "./pages/PreviewsPage";

const MANIFEST = `[frontend]
path  = "."
build = [["sh", "-c", "mkdir -p dist && echo hi > dist/index.html"]]
dist  = "dist"
`;

test.describe("Previews", () => {
  let root: string;
  let boards: Board[];

  test.beforeEach(async ({}, testInfo) => {
    root = await mkdtemp(join(mountRootFromConfig(testInfo), "kanban-e2e-previews-"));
    const repoPath = await createMonorepo(root, "app");
    await writeFile(join(repoPath, "preview.toml"), MANIFEST);
    git(repoPath, "add", "-A");
    git(repoPath, "-c", "user.name=e2e", "-c", "user.email=e2e@example.com", "commit", "-m", "Add");

    const stamp = `${Date.now()}-${testInfo.workerIndex}`;
    boards = [];
    for (const name of ["one", "two", "three"]) {
      const board = await api.createBoard({
        name: `e2e-previews-${name}-${stamp}`,
        repo_path: repoPath,
      });
      boards.push(board);
      await api.deployBoard(board.id);
    }
  });

  test.afterEach(async () => {
    for (const board of boards) await api.deleteBoard(board.id).catch(() => {});
    await rm(root, { recursive: true, force: true }).catch(() => {});
  });

  test("the filter hides boards and remembers it", async ({ page }) => {
    const [one, two, three] = boards as [Board, Board, Board];
    const previews = new PreviewsPage(page);
    await previews.goto();
    await previews.expectDeploy(one.name);
    await previews.expectDeploy(two.name);
    await previews.expectDeploy(three.name);

    await previews.setFiltered(two.name, false);
    await previews.expectNoDeploy(two.name);
    await previews.expectDeploy(one.name);
    await previews.expectDeploy(three.name);

    await page.reload();
    await previews.expectNoDeploy(two.name);
    await previews.expectDeploy(one.name);

    await previews.setFiltered(two.name, true);
    await previews.expectDeploy(two.name);
  });
});
