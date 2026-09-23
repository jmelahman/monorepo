import { mkdtempSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { defineConfig, devices } from "@playwright/test";

// E2E suite. Spins up the real Go backend against an in-memory SQLite DB and
// the Vite dev server, then drives the UI through Chromium.

// Overridable so the suite can run beside a dev server already on :8080
// (whose DB would otherwise leak state into the tests).
const BACKEND_PORT = Number(process.env.E2E_BACKEND_PORT ?? 8080);
const FRONTEND_PORT = Number(process.env.E2E_FRONTEND_PORT ?? 5174);

// Builds, artifacts, and state need real disk even with an in-memory DB.
const DATA_DIR = mkdtempSync(join(tmpdir(), "preview-e2e-"));

export default defineConfig({
  testDir: "./tests/e2e",
  fullyParallel: false,
  workers: 1,
  reporter: process.env.CI ? "list" : [["list"], ["html", { open: "never" }]],
  timeout: 60_000,
  expect: { timeout: 10_000 },
  use: {
    baseURL: `http://localhost:${FRONTEND_PORT}`,
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
  },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
  webServer: [
    {
      command: `go run . serve --in-memory --data-dir ${DATA_DIR} --addr :${BACKEND_PORT}`,
      cwd: "..",
      url: `http://localhost:${BACKEND_PORT}/api/health`,
      reuseExistingServer: !process.env.CI,
      stdout: "pipe",
      stderr: "pipe",
      timeout: 120_000,
    },
    {
      command: `npm run dev -- --port ${FRONTEND_PORT} --strictPort`,
      cwd: ".",
      env: { PREVIEW_BACKEND: `localhost:${BACKEND_PORT}` },
      url: `http://localhost:${FRONTEND_PORT}/`,
      reuseExistingServer: !process.env.CI,
      stdout: "pipe",
      stderr: "pipe",
      timeout: 60_000,
    },
  ],
});
