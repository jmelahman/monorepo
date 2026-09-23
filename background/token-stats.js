import { parseDiff } from "./diff-parser.js";
import { estimateTokens } from "./token-estimator.js";

// computeTokenStats(diffText) -> {
//   total: { added, removed },
//   perFile: [{ path, oldPath, added, removed, status, binary }]
// }
export function computeTokenStats(diffText, { estimate = estimateTokens } = {}) {
  const files = parseDiff(diffText);
  let added = 0;
  let removed = 0;
  const perFile = [];
  for (const f of files) {
    const path = f.newPath ?? f.oldPath;
    if (f.isBinary) {
      perFile.push({ path, oldPath: f.oldPath, added: 0, removed: 0, status: f.status, binary: true });
      continue;
    }
    const a = estimate(f.addedText);
    const r = estimate(f.removedText);
    added += a;
    removed += r;
    perFile.push({ path, oldPath: f.oldPath, added: a, removed: r, status: f.status, binary: false });
  }
  return { total: { added, removed }, perFile };
}
