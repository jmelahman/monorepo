import { defineConfig, devices } from "@playwright/test";

// E2E suite. Spins up the real Go backend against an in-memory SQLite DB and
// the Vite dev server, then drives the UI through Chromium.

// Non-default ports so a running dev server is never reused with the wrong
// curator config.
const BACKEND_PORT = 8095;
const FAKE_OLLAMA_PORT = 11499;
const FRONTEND_PORT = 5177; // 5175 is the Docs task

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
      command: "node tests/e2e/fake-ollama.mjs",
      cwd: ".",
      env: { FAKE_OLLAMA_PORT: String(FAKE_OLLAMA_PORT) },
      url: `http://127.0.0.1:${FAKE_OLLAMA_PORT}/api/tags`,
      reuseExistingServer: false,
      stdout: "pipe",
    },
    {
      command: `go run . serve --in-memory --addr 127.0.0.1:${BACKEND_PORT}`,
      cwd: "..",
      // The real Ollama backend, pointed at the scripted fake.
      env: {
        APP_LLM: "ollama",
        APP_MODEL: "fake",
        OLLAMA_HOST: `http://127.0.0.1:${FAKE_OLLAMA_PORT}`,
        APP_SECRET: "",
      },
      url: `http://127.0.0.1:${BACKEND_PORT}/api/health`,
      reuseExistingServer: !process.env.CI,
      stdout: "pipe",
      stderr: "pipe",
      timeout: 120_000,
    },
    {
      command: `node_modules/.bin/vite --port ${FRONTEND_PORT} --strictPort`,
      cwd: ".",
      env: { APP_BACKEND: `127.0.0.1:${BACKEND_PORT}` },
      url: `http://localhost:${FRONTEND_PORT}/`,
      reuseExistingServer: !process.env.CI,
      stdout: "pipe",
      stderr: "pipe",
      timeout: 60_000,
    },
  ],
});
