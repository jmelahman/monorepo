import { test } from "node:test";
import assert from "node:assert/strict";

// pr-locator.js is a classic content script (IIFE attaching to globalThis);
// importing it as a module runs the IIFE.
await import("../content/pr-locator.js");
const { parseGithubPrLocation } = globalThis.GHTD;

test("conversation page", () => {
  assert.deepEqual(parseGithubPrLocation("/owner/repo/pull/123"), {
    owner: "owner",
    repo: "repo",
    prNumber: "123",
    view: "conversation",
  });
});

test("classic files view", () => {
  assert.equal(parseGithubPrLocation("/owner/repo/pull/123/files").view, "files");
});

test("new React files view uses /changes", () => {
  assert.equal(parseGithubPrLocation("/owner/repo/pull/12996/changes").view, "files");
});

test("commits and checks sub-views", () => {
  assert.equal(parseGithubPrLocation("/o/r/pull/1/commits").view, "commits");
  assert.equal(parseGithubPrLocation("/o/r/pull/1/checks").view, "checks");
});

test("nested sub-paths still resolve", () => {
  assert.equal(parseGithubPrLocation("/o/r/pull/1/files/abc123").view, "files");
  assert.equal(parseGithubPrLocation("/o/r/pull/1/commits/abc123").view, "commits");
});

test("non-PR pages return null", () => {
  assert.equal(parseGithubPrLocation("/owner/repo"), null);
  assert.equal(parseGithubPrLocation("/owner/repo/pulls"), null);
  assert.equal(parseGithubPrLocation("/owner/repo/issues/5"), null);
  assert.equal(parseGithubPrLocation("/owner/repo/pull/abc"), null);
});
