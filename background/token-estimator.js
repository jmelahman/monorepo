// Heuristic token estimator — the pluggable seam. To swap in a real BPE
// tokenizer later, replace this module with one exporting the same signature.
//
// Identifier runs approximate BPE sub-word splitting (~4 chars per token);
// each symbol/punctuation char counts as one token (code is dense in these);
// the result is blended with a chars/3.5 estimate as a sanity floor.

export function estimateTokens(text) {
  if (!text) return 0;
  let structural = 0;
  let runLength = 0;
  for (let i = 0; i < text.length; i++) {
    const c = text.charCodeAt(i);
    const isWord =
      (c >= 48 && c <= 57) || // 0-9
      (c >= 65 && c <= 90) || // A-Z
      (c >= 97 && c <= 122) || // a-z
      c === 95; // _
    if (isWord) {
      runLength++;
      continue;
    }
    if (runLength) {
      structural += Math.ceil(runLength / 4);
      runLength = 0;
    }
    const isSpace = c === 9 || c === 10 || c === 11 || c === 12 || c === 13 || c === 32;
    if (!isSpace) structural++;
  }
  if (runLength) structural += Math.ceil(runLength / 4);
  const charEstimate = text.length / 3.5;
  return Math.round((structural + charEstimate) / 2);
}
