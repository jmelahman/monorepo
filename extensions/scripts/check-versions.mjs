#!/usr/bin/env bun
import { readdirSync, existsSync, readFileSync } from "node:fs";
import { join } from "node:path";

const root = new URL("..", import.meta.url).pathname;
let failed = false;

for (const entry of readdirSync(root, { withFileTypes: true })) {
  if (!entry.isDirectory() || entry.name === "node_modules" || entry.name === "scripts") continue;
  const dir = join(root, entry.name);
  const pkgPath = join(dir, "package.json");
  const manifestPath = join(dir, "manifest.json");
  if (!existsSync(pkgPath) || !existsSync(manifestPath)) continue;

  const pkg = JSON.parse(readFileSync(pkgPath, "utf8"));
  const manifest = JSON.parse(readFileSync(manifestPath, "utf8"));
  if (pkg.version !== manifest.version) {
    console.error(
      `${entry.name}: version mismatch — package.json has ${pkg.version}, manifest.json has ${manifest.version}`,
    );
    failed = true;
  }
}

if (failed) process.exit(1);
