// Pure unified-diff parser (no WebExtension APIs — Node-testable).
//
// FileDiff: {
//   oldPath: string|null, newPath: string|null,
//   status: 'added'|'removed'|'modified'|'renamed'|'binary'|'mode-change',
//   isBinary: boolean,
//   addedText: string,   // '\n'-joined content of '+' lines (marker stripped)
//   removedText: string, // '\n'-joined content of '-' lines (marker stripped)
// }

const HEADER_RE = /^diff --git a\/(.*) b\/(.*)$/;
const HEADER_QUOTED_RE = /^diff --git "a\/(.*)" "b\/(.*)"$/;

export function parseDiff(diffText) {
  const files = [];
  const lines = diffText.split("\n");
  let current = null;
  let inHunk = false;
  let inBinaryPayload = false;

  const flush = () => {
    if (current) files.push(finalize(current));
  };

  for (const line of lines) {
    const header = line.match(HEADER_QUOTED_RE) || line.match(HEADER_RE);
    if (header) {
      flush();
      current = {
        oldPath: header[1],
        newPath: header[2],
        status: "modified",
        isBinary: false,
        renamed: false,
        modeChange: false,
        hasHunks: false,
        addedLines: [],
        removedLines: [],
      };
      inHunk = false;
      inBinaryPayload = false;
      continue;
    }
    if (!current) continue;

    if (inBinaryPayload) {
      // Each "GIT binary patch" section (forward + reverse) ends with a blank line.
      if (line === "") inBinaryPayload = false;
      continue;
    }

    if (inHunk) {
      if (line.startsWith("@@")) continue;
      if (line.startsWith("+")) {
        current.addedLines.push(line.slice(1));
        continue;
      }
      if (line.startsWith("-")) {
        current.removedLines.push(line.slice(1));
        continue;
      }
      if (line.startsWith(" ") || line === "") continue; // context
      if (line.startsWith("\\")) continue; // "\ No newline at end of file"
      inHunk = false; // fall through to header-mode handling
    }

    if (line.startsWith("@@")) {
      inHunk = true;
      current.hasHunks = true;
      continue;
    }
    if (line.startsWith("rename from ")) {
      current.renamed = true;
      current.oldPath = line.slice("rename from ".length);
      continue;
    }
    if (line.startsWith("rename to ")) {
      current.renamed = true;
      current.newPath = line.slice("rename to ".length);
      continue;
    }
    if (line.startsWith("new file mode")) {
      current.status = "added";
      continue;
    }
    if (line.startsWith("deleted file mode")) {
      current.status = "removed";
      continue;
    }
    if (line.startsWith("old mode ") || line.startsWith("new mode ")) {
      current.modeChange = true;
      continue;
    }
    if (line.startsWith("Binary files ") && line.endsWith(" differ")) {
      current.isBinary = true;
      continue;
    }
    if (line === "GIT binary patch") {
      current.isBinary = true;
      inBinaryPayload = true;
      continue;
    }
    if (current.isBinary && (line.startsWith("literal ") || line.startsWith("delta "))) {
      inBinaryPayload = true;
      continue;
    }
    if (line.startsWith("--- ")) {
      const p = line.slice(4);
      if (p === "/dev/null") current.status = "added";
      else if (p.startsWith("a/")) current.oldPath = p.slice(2);
      continue;
    }
    if (line.startsWith("+++ ")) {
      const p = line.slice(4);
      if (p === "/dev/null") current.status = "removed";
      else if (p.startsWith("b/")) current.newPath = p.slice(2);
      continue;
    }
    // index lines, similarity index, extended headers: ignore
  }
  flush();
  return files;
}

function finalize(f) {
  let status = f.status;
  if (f.renamed) {
    status = "renamed";
  } else if (f.isBinary && status === "modified") {
    status = "binary";
  } else if (!f.hasHunks && !f.isBinary && status === "modified" && f.modeChange) {
    status = "mode-change";
  }
  return {
    oldPath: status === "added" ? null : f.oldPath,
    newPath: status === "removed" ? null : f.newPath,
    status,
    isBinary: f.isBinary,
    addedText: f.addedLines.join("\n"),
    removedText: f.removedLines.join("\n"),
  };
}
