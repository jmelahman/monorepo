import { test as base } from "@playwright/test";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { api, type Board, type Ticket } from "./api";

// playwright.config.ts populates this in `metadata.mountRoot` when the
// suite is running inside a container that needs host-visible bind
// sources. Outside that case we fall back to tmpdir().
function mountRootFromConfig(testInfo: { config: { metadata?: unknown } }): string {
  const meta = testInfo.config.metadata as
    | { mountRoot?: string }
    | undefined;
  return meta?.mountRoot ?? tmpdir();
}

export type Seed = {
  board: Board;
  ticket: Ticket;
  mountPath: string;
  addTicket: (title: string) => Promise<Ticket>;
};

// Per-test isolation: a fresh tmp dir (used as `mount_path` so the board
// has no git repo and the session lands in a directory we own + can clean),
// a unique board name, and one ticket. The teardown stops any sessions
// spawned during the test (deleting the board cascades through
// `sessions.Destroy` in `internal/api/handlers.go:204`).
export const test = base.extend<{ seed: Seed }>({
  seed: async ({}, use, testInfo) => {
    const mountPath = await mkdtemp(join(mountRootFromConfig(testInfo), "kanban-e2e-"));
    const safeName = testInfo.title.replace(/[^a-zA-Z0-9]+/g, "-").slice(0, 32);
    const board = await api.createBoard({
      name: `e2e-${safeName}-${Date.now()}-${testInfo.workerIndex}`,
      mount_path: mountPath,
    });
    const ticket = await api.createTicket(board.id, { title: "session ticket" });
    const addTicket = (title: string) => api.createTicket(board.id, { title });

    await use({ board, ticket, mountPath, addTicket });

    try {
      await api.deleteBoard(board.id);
    } catch (err) {
      console.warn(`teardown: deleteBoard(${board.id}) failed:`, err);
    }
    await rm(mountPath, { recursive: true, force: true }).catch(() => {});
  },
});

export const expect = test.expect;
