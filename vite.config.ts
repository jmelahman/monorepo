import { execSync } from "node:child_process"
import { readFileSync } from "node:fs"

import { defineConfig } from "vitest/config"

const { version } = JSON.parse(readFileSync(new URL("./package.json", import.meta.url), "utf8"))

/**
 * The commit this bundle was built from.
 *
 * Pages deploys on every push to master, but the version bump is its own commit
 * that lands *after* the change it names, so between the two, package.json is
 * still reporting the previous release while the site is already serving the
 * new code. The version alone therefore cannot identify what is live. The hash
 * can, and it is the difference between "roughly v0.1.9" and "exactly this".
 *
 * Actions sets GITHUB_SHA and its checkout is shallow but complete enough for
 * rev-parse; the git call is the local fallback. Neither is load-bearing, so a
 * build somewhere without either still succeeds, just anonymously.
 */
function commit(): string {
  const fromCi = process.env.GITHUB_SHA
  if (fromCi) return fromCi.slice(0, 7)
  try {
    return execSync("git rev-parse --short=7 HEAD", { stdio: ["ignore", "pipe", "ignore"] })
      .toString()
      .trim()
  } catch {
    return ""
  }
}

export default defineConfig({
  define: {
    __BUILD_VERSION__: JSON.stringify(version),
    __BUILD_COMMIT__: JSON.stringify(commit()),
  },
  // Relative asset paths: Capacitor serves the bundle from a non-root scheme,
  // so absolute "/assets/..." URLs 404 inside the APK.
  base: "./",
  build: {
    target: "es2022",
    outDir: "dist",
  },
  server: {
    // host:true exposes the dev server on the LAN so a physical phone can
    // load it via capacitor.config server.url for live reload.
    host: true,
    port: 5173,
  },
  test: {
    // The engine is pure and DOM-free, so node is the right default. UI tests
    // opt into jsdom per-file with an @vitest-environment docblock.
    environment: "node",
    include: ["test/**/*.test.ts", "src/**/*.test.ts"],
  },
})
