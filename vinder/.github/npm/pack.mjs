// Shared by Go subtrees published to npm: .config/git-orchard/shared/npm in
// the monorepo (https://github.com/jmelahman/monorepo). Edit it there; `git
// orchard sync` copies it here.
//
// Lays out npm packages for the binaries `goreleaser build` left in dist/,
// the way esbuild ships: one package per platform holding its binary, and a
// main package that depends on all of them optionally, so the package
// manager installs only the one that matches (by its os and cpu fields),
// and runs it from a small JS shim. Nothing runs at install time, so it
// works where install scripts are off: bun, pnpm 10, `--ignore-scripts`.
//
//   node .github/npm/pack.mjs
//
// writes dist/npm/platforms/<name>-<os>-<cpu>/ and dist/npm/main/, to be
// published in that order. The package name is GoReleaser's project name.
// Fields in .github/npm/package.json (description, keywords, homepage, ...)
// are merged into the main package's package.json.

import { chmodSync, copyFileSync, existsSync, mkdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { join } from "node:path";

const dist = "dist";
const out = join(dist, "npm");

// GoReleaser's names for platforms, as npm (and process.platform/arch)
// spell them.
const npmOS = { darwin: "darwin", freebsd: "freebsd", linux: "linux", windows: "win32" };
const npmCPU = { "386": "ia32", amd64: "x64", arm: "arm", arm64: "arm64" };

const readJSON = (file) => JSON.parse(readFileSync(file, "utf8"));
const writeJSON = (file, value) => writeFileSync(file, JSON.stringify(value, null, 2) + "\n");

const { project_name: name, version } = readJSON(join(dist, "metadata.json"));
if (!/^\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$/.test(version)) {
  throw new Error(`${version} is not a semver version; is the tag v1.2.3?`);
}
const binaries = readJSON(join(dist, "artifacts.json")).filter((a) => a.type === "Binary");
const names = new Set(binaries.map((a) => a.extra.Binary));
if (names.size !== 1) {
  throw new Error(`expected one binary, found ${[...names].join(", ") || "none"}; run goreleaser build first`);
}
const [bin] = names;

// Provenance requires repository.url to match the repository publishing.
const repository = process.env.GITHUB_REPOSITORY
  ? { type: "git", url: `git+https://github.com/${process.env.GITHUB_REPOSITORY}.git` }
  : undefined;
const overlay = existsSync(".github/npm/package.json") ? readJSON(".github/npm/package.json") : {};
const license = overlay.license;
const extras = ["LICENSE", "README.md"].filter((f) => existsSync(f));

rmSync(out, { recursive: true, force: true });

const platforms = {};
for (const a of binaries) {
  const os = npmOS[a.goos];
  const cpu = npmCPU[a.goarch];
  if (!os || !cpu) {
    console.warn(`skipping ${a.goos}/${a.goarch}: no npm equivalent`);
    continue;
  }
  const pkg = `${name}-${os}-${cpu}`;
  if (platforms[`${os}-${cpu}`]) {
    throw new Error(`two binaries for ${os}-${cpu}`);
  }
  platforms[`${os}-${cpu}`] = pkg;

  const dir = join(out, "platforms", pkg);
  mkdirSync(join(dir, "bin"), { recursive: true });
  const exe = join(dir, "bin", bin + (os === "win32" ? ".exe" : ""));
  copyFileSync(a.path, exe);
  chmodSync(exe, 0o755);
  for (const f of extras.filter((f) => f === "LICENSE")) copyFileSync(f, join(dir, f));
  writeJSON(join(dir, "package.json"), {
    name: pkg,
    version,
    description: `The ${os}-${cpu} binary for ${name}.`,
    ...(license && { license }),
    ...(repository && { repository }),
    os: [os],
    cpu: [cpu],
    // Yarn's Plug'n'Play would otherwise leave the binary in a zip, where
    // it can't be run.
    preferUnplugged: true,
  });
}

const main = join(out, "main");
mkdirSync(join(main, "bin"), { recursive: true });
for (const f of extras) copyFileSync(f, join(main, f));
const shim = join(main, "bin", `${bin}.js`);
writeFileSync(
  shim,
  `#!/usr/bin/env node
"use strict";
// Runs ${bin} from the package for this platform, which the package manager
// installed as an optional dependency.
const { spawnSync } = require("node:child_process");

const platforms = ${JSON.stringify(platforms, null, 2)};
const target = process.platform + "-" + process.arch;
const pkg = platforms[target];
if (!pkg) {
  console.error("${bin}: no binary for " + target);
  process.exit(1);
}
let exe;
try {
  exe = require.resolve(pkg + "/bin/${bin}" + (process.platform === "win32" ? ".exe" : ""));
} catch {
  console.error("${bin}: " + pkg + " is not installed; it is an optional dependency of ${name}, so install without --omit=optional or --no-optional.");
  process.exit(1);
}
const res = spawnSync(exe, process.argv.slice(2), { stdio: "inherit" });
if (res.error) {
  console.error("${bin}: " + res.error.message);
  process.exit(1);
}
if (res.signal) {
  process.kill(process.pid, res.signal);
}
process.exit(res.status ?? 1);
`,
);
chmodSync(shim, 0o755);
writeJSON(join(main, "package.json"), {
  name,
  version,
  ...overlay,
  ...(repository && { repository }),
  bin: { [bin]: `bin/${bin}.js` },
  optionalDependencies: Object.fromEntries(Object.values(platforms).sort().map((p) => [p, version])),
});

console.log(`packed ${name}@${version} for ${Object.keys(platforms).sort().join(", ")}`);
