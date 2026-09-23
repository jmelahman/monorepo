import { spawnSync } from "node:child_process";
import { DOCKER_AVAILABLE } from "./fixtures/docker";

// Pre-pull the default session image once before any spec runs. The first
// `POST /api/sessions/{id}/start` would otherwise trigger a multi-minute
// pull inside the test timeout. Default tag matches
// `internal/docker/builtin.go:17`.
const IMAGE = process.env.KANBAN_E2E_IMAGE ?? "lahmanja/kanban-devcontainer:latest";

export default async function globalSetup() {
  if (!DOCKER_AVAILABLE) {
    process.stdout.write(
      "[e2e] docker daemon not reachable — session-based specs will skip\n",
    );
    return;
  }

  const inspect = spawnSync("docker", ["image", "inspect", IMAGE], {
    stdio: "ignore",
  });
  if (inspect.status === 0) return;

  process.stdout.write(`[e2e] pulling ${IMAGE} (one-time)…\n`);
  const pull = spawnSync("docker", ["pull", IMAGE], { stdio: "inherit" });
  if (pull.status !== 0) {
    throw new Error(
      `failed to pull ${IMAGE}. Ensure Docker is available and the image is reachable, ` +
        `or set KANBAN_E2E_IMAGE to a locally available tag.`,
    );
  }
}
