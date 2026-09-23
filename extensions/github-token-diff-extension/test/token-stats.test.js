import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { computeTokenStats } from "../background/token-stats.js";

const fixture = (name) => readFileSync(new URL(`./fixtures/${name}`, import.meta.url), "utf8");

const ALL_FIXTURES = [
  "simple-modify.diff",
  "rename-only.diff",
  "rename-with-changes.diff",
  "binary-simple.diff",
  "binary-git-patch.diff",
  "mode-change-only.diff",
  "new-file.diff",
  "deleted-file.diff",
];

test("invariant: totals equal the sum of per-file counts", () => {
  const stats = computeTokenStats(ALL_FIXTURES.map(fixture).join(""));
  const sumAdded = stats.perFile.reduce((s, f) => s + f.added, 0);
  const sumRemoved = stats.perFile.reduce((s, f) => s + f.removed, 0);
  assert.equal(stats.total.added, sumAdded);
  assert.equal(stats.total.removed, sumRemoved);
  assert.equal(stats.perFile.length, ALL_FIXTURES.length);
});

test("binary files contribute zero and are flagged", () => {
  const stats = computeTokenStats(fixture("binary-simple.diff"));
  assert.equal(stats.perFile.length, 1);
  assert.equal(stats.perFile[0].binary, true);
  assert.equal(stats.perFile[0].added, 0);
  assert.equal(stats.perFile[0].removed, 0);
  assert.deepEqual(stats.total, { added: 0, removed: 0 });
});

test("added-only file has zero removed tokens", () => {
  const stats = computeTokenStats(fixture("new-file.diff"));
  assert.equal(stats.total.removed, 0);
  assert.ok(stats.total.added > 0);
  assert.equal(stats.perFile[0].path, "docs/notes.md");
});

test("pure rename and mode change contribute zero tokens", () => {
  const stats = computeTokenStats(fixture("rename-only.diff") + fixture("mode-change-only.diff"));
  assert.deepEqual(stats.total, { added: 0, removed: 0 });
});

test("estimator is pluggable", () => {
  const stats = computeTokenStats(fixture("simple-modify.diff"), {
    estimate: (text) => (text ? text.split("\n").length : 0),
  });
  assert.deepEqual(stats.total, { added: 2, removed: 1 });
});
