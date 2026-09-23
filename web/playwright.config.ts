import { defineConfig, devices } from "@playwright/test";
import { readFileSync, existsSync, mkdirSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = dirname(fileURLToPath(import.meta.url));

// E2E suite for terminal session loading. Spins up the real Go backend
// against an in-memory SQLite DB and the Vite dev server, then drives
// the UI through Chromium. Container session ops are real — first run
// pulls `lahmanja/kanban-devcontainer:latest` (see global-setup.ts).

const BACKEND_PORT = 7474;
const FRONTEND_PORT = 5174;

// Container-aware mount root and host-path translation.
//
// When this suite is run from inside the kanban devcontainer, the test
// backend tries to spawn session containers via the host docker daemon.
// `dockerd` resolves bind sources against the *host* filesystem, so any
// `/tmp/kanban-e2e-…` path created inside this container is invisible to
// it (CLAUDE.md "Spawning session containers from this devcontainer").
//
// To make the suite work both inside the devcontainer and on a bare host,
// we:
//   1. Detect whether the workspace dir is bind-mounted from elsewhere by
//      scanning `/proc/self/mountinfo`.
//   2. If so, plant test mount paths under `<workspace>/.tmp/kanban-e2e/`
//      (a host-visible subtree) and pass `KANBAN_HOST_WORKSPACE` /
//      `KANBAN_HOST_HOME` to the backend so it rewrites those prefixes
//      when handing paths to `dockerd`.
//   3. Otherwise, fall back to the OS tmp dir like a normal test runner.
const workspaceRoot = resolve(__dirname, "..");
const hostMapping = detectHostMapping(workspaceRoot);
const mountRoot = hostMapping ? resolve(workspaceRoot, ".tmp", "kanban-e2e") : "";
if (mountRoot) mkdirSync(mountRoot, { recursive: true });

const backendEnv: Record<string, string> = {
  // Strip ssh-agent forwarding: `internal/docker/builtin.go:34` binds
  // `$SSH_AUTH_SOCK` into every session container, and inside the
  // devcontainer the socket path (`/tmp/ssh-agent.sock`) isn't
  // host-visible, which fails the bind. Tests don't need agent
  // forwarding.
  SSH_AUTH_SOCK: "",
};
if (hostMapping) {
  backendEnv.KANBAN_HOST_WORKSPACE = hostMapping.hostWorkspace;
  if (hostMapping.hostHome) backendEnv.KANBAN_HOST_HOME = hostMapping.hostHome;
}

export default defineConfig({
  testDir: "./tests/e2e",
  fullyParallel: false,
  workers: 1,
  reporter: process.env.CI ? "list" : [["list"], ["html", { open: "never" }]],
  globalSetup: "./tests/e2e/global-setup.ts",
  timeout: 90_000,
  expect: { timeout: 15_000 },
  use: {
    baseURL: `http://localhost:${FRONTEND_PORT}`,
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
    video: "retain-on-failure",
    extraHTTPHeaders: {},
  },
  // The seed fixture reads this to pick a directory the host dockerd can
  // stat — leaving it unset is fine on a bare host (falls back to tmpdir).
  ...(mountRoot && { metadata: { mountRoot } }),
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
  webServer: [
    {
      command: "go run . serve --in-memory --claude-config=false",
      cwd: "..",
      env: backendEnv,
      url: `http://localhost:${BACKEND_PORT}/api/boards`,
      reuseExistingServer: !process.env.CI,
      stdout: "pipe",
      stderr: "pipe",
      timeout: 120_000,
    },
    {
      command: `npm run dev -- --port ${FRONTEND_PORT} --strictPort`,
      cwd: ".",
      env: { KANBAN_BACKEND: `localhost:${BACKEND_PORT}` },
      url: `http://localhost:${FRONTEND_PORT}/`,
      reuseExistingServer: !process.env.CI,
      stdout: "pipe",
      stderr: "pipe",
      timeout: 60_000,
    },
  ],
});

type HostMapping = { hostWorkspace: string; hostHome: string | null };

// Parse `/proc/self/mountinfo` to find the bind-mount source of a given
// in-container path. Returns the host-side path of `/workspace` (and
// `$HOME` if it can be inferred) so the backend can rewrite those
// prefixes for dockerd. Returns null on platforms without mountinfo or
// when workspaceRoot isn't a bind mount (e.g. running on the host).
function detectHostMapping(workspaceRoot: string): HostMapping | null {
  if (!existsSync("/proc/self/mountinfo")) return null;
  let raw: string;
  try {
    raw = readFileSync("/proc/self/mountinfo", "utf8");
  } catch {
    return null;
  }
  // mountinfo columns (space-separated, optional fields ended by `-`):
  //   id parent major:minor root mount_point options... - fs source ...
  // We want entries where `mount_point` equals the workspace dir and the
  // backing source path is non-empty (a real host directory).
  let workspaceSrc: string | null = null;
  let homeSrc: string | null = null;
  for (const line of raw.split("\n")) {
    if (!line) continue;
    const parts = line.split(" ");
    const mountPoint = parts[4];
    const dashIdx = parts.indexOf("-");
    if (dashIdx < 0 || dashIdx + 2 >= parts.length) continue;
    const root = parts[3];
    if (mountPoint === workspaceRoot && root && root.startsWith("/")) {
      // The host source is `root` (relative to the device root) — for bind
      // mounts on the same fs this is the absolute host path.
      workspaceSrc = root;
    }
    if (mountPoint === "/root/.zshrc.host" && root && root.startsWith("/")) {
      // Heuristic: the user's host shell rc is bound in — derive the
      // host home from it.
      homeSrc = root.replace(/\/\.zshrc$/, "");
    }
  }
  if (!workspaceSrc) return null;
  return { hostWorkspace: workspaceSrc, hostHome: homeSrc };
}
