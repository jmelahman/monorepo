import { mkdtemp, rm, writeFile } from "node:fs/promises";
import { join } from "node:path";
import { expect, test } from "@playwright/test";
import { api, type Board } from "./fixtures/api";
import { createMonorepo, git } from "./fixtures/repo";
import { mountRootFromConfig } from "./fixtures/seed";
import { ReleasesPage } from "./pages/ReleasesPage";

const PREFIX = "tools/foo";

// A real monorepo with an origin and a mirrored subtree, on bare repos, so the
// release goes through the real orchard code and neither remote is on GitHub.
// Nothing in these specs needs a session or docker.
type World = { root: string; repoPath: string; origin: string; upstream: string; board: Board };

function commit(cwd: string, message: string) {
  git(cwd, "add", "-A");
  git(cwd, "-c", "user.name=e2e", "-c", "user.email=e2e@example.com", "commit", "-m", message);
}

async function change(world: World, name: string) {
  await writeFile(join(world.repoPath, PREFIX, name), "package main\n");
  commit(world.repoPath, `Change ${name}`);
  git(world.repoPath, "push", "origin", "main");
}

function mirror(world: World) {
  const split = git(world.repoPath, "subtree", "split", `--prefix=${PREFIX}`);
  git(world.repoPath, "push", world.upstream, `${split}:refs/heads/master`);
}

async function newWorld(parent: string, name: string): Promise<World> {
  const root = await mkdtemp(join(parent, "kanban-e2e-releases-"));
  const repoPath = await createMonorepo(root, "app");
  const origin = join(root, "origin.git");
  const upstream = join(root, "foo.git");
  git(root, "init", "--bare", "--initial-branch=main", origin);
  git(root, "init", "--bare", "--initial-branch=master", upstream);

  const work = join(root, "foo");
  git(root, "init", "--initial-branch=master", work);
  await writeFile(join(work, "README"), "foo\n");
  commit(work, "Start foo");
  git(work, "push", upstream, "master");

  git(
    repoPath,
    "-c",
    "user.name=e2e",
    "-c",
    "user.email=e2e@example.com",
    "subtree",
    "add",
    `--prefix=${PREFIX}`,
    upstream,
    "master",
  );
  await writeFile(
    join(repoPath, ".gitsubtrees"),
    `[subtree "${PREFIX}"]\n\tremote = ${upstream}\n\tbranch = master\n`,
  );
  commit(repoPath, "List tools/foo");
  git(repoPath, "remote", "add", "origin", origin);
  git(repoPath, "push", "origin", "main");

  const board = await api.createBoard({
    name,
    repo_path: repoPath,
    git_author_name: "e2e",
    git_author_email: "e2e@example.com",
  });
  const world = { root, repoPath, origin, upstream, board };
  await change(world, "main.go");
  mirror(world);
  return world;
}

test.describe("Releases", () => {
  let world: World;

  test.beforeEach(async ({}, testInfo) => {
    const name = `e2e-releases-${Date.now()}-${testInfo.workerIndex}`;
    world = await newWorld(mountRootFromConfig(testInfo), name);
  });

  test.afterEach(async () => {
    await api.deleteBoard(world.board.id).catch(() => {});
    await rm(world.root, { recursive: true, force: true }).catch(() => {});
  });

  test("a reminder goes from a note to a release with its log", async ({ page }) => {
    const releases = new ReleasesPage(page);
    await releases.goto();
    const project = releases.project(world.repoPath);
    await project.expectSubtree(PREFIX);
    const note = `the new thing ${world.board.id}`;

    await project.remind(PREFIX, "minor", note);
    const reminder = releases.reminder(note);
    await reminder.expectBump("minor");

    await reminder.openRelease();
    await releases.confirm.expectVersion("v0.1.0");
    await releases.confirm.release();

    await releases.expectLogContains("v0.1.0");
    await releases.expectRun("v0.1.0", "released");
    await reminder.expectGone();
    expect(git(world.repoPath, "ls-remote", "--tags", world.origin)).toContain(`${PREFIX}/v0.1.0`);
  });

  test("commits the mirror hasn't published block the release", async ({ page }) => {
    await change(world, "unmirrored.go");
    const releases = new ReleasesPage(page);
    await releases.goto();
    const project = releases.project(world.repoPath);
    await project.expectSubtree(PREFIX);

    await project.openRelease(PREFIX);
    await releases.confirm.expectBlocked("wait for the mirror");
  });
});
