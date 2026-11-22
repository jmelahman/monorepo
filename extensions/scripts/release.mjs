#!/usr/bin/env bun
// Bumps package.json (and manifest.json, if present) for the current extension,
// commits the version bump, and tags extensions/<name>/v<version> via `tag`.
//
// Usage (from inside an extension directory):
//   bun run release [--major|--minor|--patch] [--suffix <name>] [--dry-run]
import { existsSync, readFileSync, writeFileSync } from "node:fs";
import { basename, join } from "node:path";
import { execFileSync } from "node:child_process";

const args = process.argv.slice(2);
const dryRun = args.includes("--dry-run");
const tagArgs = args.filter((a) => a !== "--dry-run");

const cwd = process.cwd();
const name = basename(cwd);
const prefix = `extensions/${name}`;
const tagPrefix = `${prefix}/v`;

const pkgPath = join(cwd, "package.json");
const manifestPath = join(cwd, "manifest.json");

if (!existsSync(pkgPath)) {
  console.error(`release: expected package.json in ${cwd}`);
  process.exit(1);
}

const nextTag = execFileSync("tag", ["--prefix", prefix, "--print-only", ...tagArgs], {
  encoding: "utf8",
}).trim();

if (!nextTag.startsWith(tagPrefix)) {
  console.error(`release: unexpected tag output: ${nextTag}`);
  process.exit(1);
}

const nextVersion = nextTag.slice(tagPrefix.length);

function updateJsonVersion(filePath) {
  const data = JSON.parse(readFileSync(filePath, "utf8"));
  data.version = nextVersion;
  writeFileSync(filePath, `${JSON.stringify(data, null, 2)}\n`);
}

const files = [pkgPath];
if (existsSync(manifestPath)) files.push(manifestPath);

if (dryRun) {
  console.log(
    `dry-run: would set version to ${nextVersion} in ${files.map((f) => basename(f)).join(", ")}`,
  );
  console.log(`dry-run: would commit "chore(${name}): release v${nextVersion}" and tag ${nextTag}`);
  process.exit(0);
}

for (const f of files) updateJsonVersion(f);

execFileSync("git", ["add", "--", ...files], { stdio: "inherit" });
execFileSync("git", ["commit", "-m", `chore(${name}): release v${nextVersion}`, "--", ...files], {
  stdio: "inherit",
});
execFileSync("git", ["tag", "-a", nextTag, "-m", `${name} v${nextVersion}`], { stdio: "inherit" });

console.log(`Released ${nextTag}. Push with: git push && git push origin ${nextTag}`);
