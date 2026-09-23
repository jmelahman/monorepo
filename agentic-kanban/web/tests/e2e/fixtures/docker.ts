import { spawnSync } from "node:child_process";

// Module-level probe: is a docker daemon reachable? `docker info` succeeds
// iff the CLI can talk to a daemon (socket present and responsive). Cached
// across the file so each spec doesn't reprobe.
export const DOCKER_AVAILABLE = (() => {
  try {
    const r = spawnSync("docker", ["info"], { stdio: "ignore", timeout: 5_000 });
    return r.status === 0;
  } catch {
    return false;
  }
})();
