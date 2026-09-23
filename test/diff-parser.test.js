import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { parseDiff } from "../background/diff-parser.js";

const fixture = (name) => readFileSync(new URL(`./fixtures/${name}`, import.meta.url), "utf8");

test("simple modify", () => {
  const files = parseDiff(fixture("simple-modify.diff"));
  assert.equal(files.length, 1);
  const f = files[0];
  assert.equal(f.status, "modified");
  assert.equal(f.isBinary, false);
  assert.equal(f.oldPath, "src/app.js");
  assert.equal(f.newPath, "src/app.js");
  assert.equal(f.addedText, "const b = 3;\nconst c = 4;");
  assert.equal(f.removedText, "const b = 2;");
});

test("pure rename has no token content", () => {
  const files = parseDiff(fixture("rename-only.diff"));
  assert.equal(files.length, 1);
  const f = files[0];
  assert.equal(f.status, "renamed");
  assert.equal(f.oldPath, "old/name.txt");
  assert.equal(f.newPath, "new/name.txt");
  assert.equal(f.addedText, "");
  assert.equal(f.removedText, "");
});

test("rename with edits keeps hunk content", () => {
  const files = parseDiff(fixture("rename-with-changes.diff"));
  assert.equal(files.length, 1);
  const f = files[0];
  assert.equal(f.status, "renamed");
  assert.equal(f.oldPath, "lib/util.py");
  assert.equal(f.newPath, "lib/utils.py");
  assert.equal(f.addedText, "def helper(x):");
  assert.equal(f.removedText, "def helper():");
});

test("'Binary files ... differ' is flagged binary with no content", () => {
  const files = parseDiff(fixture("binary-simple.diff"));
  assert.equal(files.length, 1);
  const f = files[0];
  assert.equal(f.isBinary, true);
  assert.equal(f.status, "binary");
  assert.equal(f.addedText, "");
  assert.equal(f.removedText, "");
});

test("GIT binary patch payload is skipped", () => {
  const files = parseDiff(fixture("binary-git-patch.diff"));
  assert.equal(files.length, 1);
  const f = files[0];
  assert.equal(f.isBinary, true);
  assert.equal(f.status, "added");
  assert.equal(f.addedText, "");
  assert.equal(f.removedText, "");
});

test("mode-change-only file has no token content", () => {
  const files = parseDiff(fixture("mode-change-only.diff"));
  assert.equal(files.length, 1);
  const f = files[0];
  assert.equal(f.status, "mode-change");
  assert.equal(f.addedText, "");
  assert.equal(f.removedText, "");
});

test("new file: oldPath null, all lines added", () => {
  const files = parseDiff(fixture("new-file.diff"));
  assert.equal(files.length, 1);
  const f = files[0];
  assert.equal(f.status, "added");
  assert.equal(f.oldPath, null);
  assert.equal(f.newPath, "docs/notes.md");
  assert.equal(f.addedText, "# Notes\nhello world");
  assert.equal(f.removedText, "");
});

test("deleted file: newPath null, all lines removed", () => {
  const files = parseDiff(fixture("deleted-file.diff"));
  assert.equal(files.length, 1);
  const f = files[0];
  assert.equal(f.status, "removed");
  assert.equal(f.oldPath, "tmp/scratch.txt");
  assert.equal(f.newPath, null);
  assert.equal(f.addedText, "");
  assert.equal(f.removedText, "line one\nline two");
});

test("multi-file diff: all fixtures concatenated parse independently", () => {
  const names = [
    "simple-modify.diff",
    "rename-only.diff",
    "rename-with-changes.diff",
    "binary-simple.diff",
    "binary-git-patch.diff",
    "mode-change-only.diff",
    "new-file.diff",
    "deleted-file.diff",
  ];
  const files = parseDiff(names.map(fixture).join(""));
  assert.equal(files.length, names.length);
  assert.deepEqual(
    files.map((f) => f.status),
    ["modified", "renamed", "renamed", "binary", "added", "mode-change", "added", "removed"]
  );
});

test("'\\ No newline at end of file' marker is ignored", () => {
  const diff = [
    "diff --git a/x.txt b/x.txt",
    "index 1234567..89abcde 100644",
    "--- a/x.txt",
    "+++ b/x.txt",
    "@@ -1 +1 @@",
    "-old line",
    "\\ No newline at end of file",
    "+new line",
    "\\ No newline at end of file",
    "",
  ].join("\n");
  const files = parseDiff(diff);
  assert.equal(files.length, 1);
  assert.equal(files[0].addedText, "new line");
  assert.equal(files[0].removedText, "old line");
});

test("removed line starting with '--' is not mistaken for a file header", () => {
  const diff = [
    "diff --git a/y.txt b/y.txt",
    "index 1234567..89abcde 100644",
    "--- a/y.txt",
    "+++ b/y.txt",
    "@@ -1,2 +1,1 @@",
    "--- this looks like a header",
    " context",
    "",
  ].join("\n");
  const files = parseDiff(diff);
  assert.equal(files.length, 1);
  assert.equal(files[0].removedText, "-- this looks like a header");
});

test("empty input parses to no files", () => {
  assert.deepEqual(parseDiff(""), []);
});
