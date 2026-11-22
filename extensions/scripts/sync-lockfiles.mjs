#!/usr/bin/env bun
// Keeps standalone bun.lock files in sync for any extension that is mirrored as
// a git-orchard subtree (listed in ../.config/git-orchard/subtrees).
//
// Usage:
//   bun run sync-lockfiles          # regenerate standalone subtree lockfiles
//   bun run sync-lockfiles --check  # exit non-zero if any lockfile is stale
import { copyFileSync, existsSync, mkdtempSync, readFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { execFileSync } from "node:child_process";

const checkOnly = process.argv.includes("--check");
const extRoot = new URL("..", import.meta.url).pathname;
const subtreesPath = join(extRoot, "..", ".config", "git-orchard", "subtrees");

if (!existsSync(subtreesPath)) {
  process.exit(0);
}

const subtreesText = readFileSync(subtreesPath, "utf8");
const extSubtrees = [...subtreesText.matchAll(/\[subtree "extensions\/([^"]+)"\]/g)].map(
  (m) => m[1],
);

let stale = false;

for (const name of extSubtrees) {
  const pkgDir = join(extRoot, name);
  const pkgJson = join(pkgDir, "package.json");
  if (!existsSync(pkgJson)) continue;

  const tmp = mkdtempSync(join(tmpdir(), `ext-lock-${name}-`));
  try {
    copyFileSync(pkgJson, join(tmp, "package.json"));
    try {
      execFileSync("bun", ["install", "--lockfile-only", "--offline", "--silent"], {
        cwd: tmp,
        stdio: "pipe",
      });
    } catch {
      execFileSync("bun", ["install", "--lockfile-only", "--silent"], {
        cwd: tmp,
        stdio: "inherit",
      });
    }

    const generated = readFileSync(join(tmp, "bun.lock"), "utf8");
    const targetPath = join(pkgDir, "bun.lock");
    const current = existsSync(targetPath) ? readFileSync(targetPath, "utf8") : "";

    if (generated !== current) {
      if (checkOnly) {
        console.error(
          `${name}/bun.lock is out of date; run \`bun run sync-lockfiles\` in extensions/`,
        );
        stale = true;
      } else {
        copyFileSync(join(tmp, "bun.lock"), targetPath);
        console.log(`Updated ${name}/bun.lock`);
      }
    }
  } finally {
    rmSync(tmp, { recursive: true, force: true });
  }
}

if (stale) process.exit(1);
