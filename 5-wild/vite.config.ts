import { execFileSync } from "node:child_process"

import { defineConfig } from "vitest/config"

/**
 * The release this bundle belongs to: the newest `v*` tag at or behind HEAD,
 * without its `v`.
 *
 * It used to be package.json's `version`, which the release flow bumped in a
 * commit of its own before tagging, and which drifted the way a number kept in
 * two places does: v0.8.2 was tagged over a package.json still saying 0.8.1,
 * and the lockfile's copy had stopped at 0.7.1 two releases before that. The
 * tag is what the APK, the desktop installers and the GitHub release are all
 * named by already, so it is now the only place the number is written.
 *
 * A tag build takes it from GITHUB_REF_NAME, which needs no history. Anything
 * else asks git, which is why every workflow that builds dist/ checks out with
 * `fetch-depth: 0`: the default clone is one commit deep with no tags in it,
 * and describe would find nothing. `5-wild/v*` is the same tag in the monorepo
 * this repository is mirrored out of. A build with neither, from a tarball of
 * the source, says 0.0.0 rather than failing.
 */
function version(): string {
  const tag =
    process.env.GITHUB_REF_TYPE === "tag"
      ? process.env.GITHUB_REF_NAME
      : git("describe", "--tags", "--abbrev=0", "--match", "v*", "--match", "5-wild/v*")
  return tag?.replace(/^(.*\/)?v/, "") || "0.0.0"
}

/**
 * The commit this bundle was built from.
 *
 * Pages deploys on every push to master, but a release is only cut once per
 * phase of work, so between two tags the site serves new code under the last
 * tag's number. The version alone therefore cannot identify what is live. The
 * hash can, and it is the difference between "roughly v0.1.9" and "exactly
 * this".
 *
 * Actions sets GITHUB_SHA and its checkout is shallow but complete enough for
 * rev-parse; the git call is the local fallback. Neither is load-bearing, so a
 * build somewhere without either still succeeds, just anonymously.
 */
function commit(): string {
  const fromCi = process.env.GITHUB_SHA
  if (fromCi) return fromCi.slice(0, 7)
  return git("rev-parse", "--short=7", "HEAD") ?? ""
}

/** No shell, so `v*` reaches git as a pattern rather than as vite.config.ts. */
function git(...args: string[]): string | undefined {
  try {
    return execFileSync("git", args, { stdio: ["ignore", "pipe", "ignore"] })
      .toString()
      .trim()
  } catch {
    return undefined
  }
}

export default defineConfig({
  define: {
    __BUILD_VERSION__: JSON.stringify(version()),
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
    include: ["test/**/*.test.ts", "src/**/*.test.ts", "telemetry/test/**/*.test.ts"],
  },
})
