import { defineConfig, devices } from "@playwright/test";

// E2E suite. Spins up the real Go backend against an in-memory SQLite DB and
// the Vite dev server, then drives the UI through Chromium.

const BACKEND_PORT = 8080;
const FRONTEND_PORT = 5174;

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
      command: "go run . serve --in-memory",
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
      env: { APP_BACKEND: `localhost:${BACKEND_PORT}` },
      url: `http://localhost:${FRONTEND_PORT}/`,
      reuseExistingServer: !process.env.CI,
      stdout: "pipe",
      stderr: "pipe",
      timeout: 60_000,
    },
  ],
});
