import { access, rm } from "node:fs/promises";
import { join } from "node:path";
import { test, expect } from "@playwright/test";
import { api, waitForSession, isAttachable, type Session } from "./fixtures/api";
import { DOCKER_AVAILABLE } from "./fixtures/docker";
import { containerExec, createMonorepo } from "./fixtures/repo";
import { mountRootFromConfig } from "./fixtures/seed";

test.skip(!DOCKER_AVAILABLE, "requires a reachable docker daemon");

const PROJECT_DIR = "services/api";

// A board scoped to a monorepo subdirectory must still get a branch-isolated
// worktree of the *whole* repo at the workspace mount, with only the agent's
// working directory moved down into the subproject. This is the regression
// guard for the whole project_dir feature: before it existed the only way to
// express "work in services/api" was a descendant mount_path, which bound the
// main checkout and left /workspace outside any git repository.
//
// Driven through the API rather than the UI: the assertions are about what
// dockerd was told at container-create time, which no UI surface shows.
test("a project_dir board runs the agent inside the subproject", async ({}, testInfo) => {
  const repoPath = await createMonorepo(mountRootFromConfig(testInfo), PROJECT_DIR);
  const board = await api.createBoard({
    name: `e2e-monorepo-${Date.now()}-${testInfo.workerIndex}`,
    repo_path: repoPath,
    project_dir: PROJECT_DIR,
  });
  expect(board.project_dir).toBe(PROJECT_DIR);
  expect(board.mount_path).toBeFalsy();

  let session: Session | undefined;
  try {
    const ticket = await api.createTicket(board.id, { title: "monorepo ticket" });
    const ensured = await api.ensureSession(ticket.id);

    // writeClaudeSettings runs during ensure and must land in the agent's cwd,
    // not at the worktree root — the status hooks are wired up from there.
    await expect(
      access(join(ensured.worktree_path, PROJECT_DIR, ".claude", "settings.local.json")),
    ).resolves.toBeUndefined();

    await api.startSession(ensured.id);
    session = await waitForSession(board.id, ensured.id, isAttachable, {
      timeoutMs: 120_000,
    });

    const container = session.container_name;
    expect(container).toBeTruthy();
    if (!container) return;

    // The mount target is unchanged; only the working directory descends.
    expect(session.workspace_folder).toBe(`/workspace/${PROJECT_DIR}`);
    expect(containerExec(container, "pwd")).toBe(`/workspace/${PROJECT_DIR}`);

    // The whole repo is checked out, so git resolves from the subproject and
    // reports it as a prefix of the workspace root.
    expect(containerExec(container, "git", "rev-parse", "--show-toplevel")).toBe(
      "/workspace",
    );
    expect(containerExec(container, "git", "rev-parse", "--show-prefix")).toBe(
      `${PROJECT_DIR}/`,
    );

    // And it is the ticket's worktree, not the main checkout — the bug that
    // a descendant mount_path silently introduced.
    expect(containerExec(container, "git", "rev-parse", "--abbrev-ref", "HEAD")).toBe(
      session.branch_name,
    );

    // Files above the subproject are still present and editable.
    expect(containerExec(container, "cat", "/workspace/go.work")).toContain(
      `use ./${PROJECT_DIR}`,
    );
  } finally {
    if (session) {
      await api.stopSession(session.id).catch(() => {});
    }
    await api.deleteBoard(board.id).catch((err) => {
      console.warn(`teardown: deleteBoard(${board.id}) failed:`, err);
    });
    await rm(repoPath, { recursive: true, force: true }).catch(() => {});
  }
});
