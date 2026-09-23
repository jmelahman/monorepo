import { test } from "node:test";
import assert from "node:assert/strict";
import { estimateTokens } from "../background/token-estimator.js";

test("empty and falsy input is zero tokens", () => {
  assert.equal(estimateTokens(""), 0);
  assert.equal(estimateTokens(null), 0);
  assert.equal(estimateTokens(undefined), 0);
});

test("non-empty text yields at least one token", () => {
  assert.ok(estimateTokens("x") >= 1);
  assert.ok(estimateTokens("{") >= 1);
});

test("monotonic: more text never yields fewer tokens", () => {
  const line = "const result = compute(alpha, beta) + gamma;\n";
  let prev = 0;
  for (let n = 1; n <= 50; n += 7) {
    const tokens = estimateTokens(line.repeat(n));
    assert.ok(tokens >= prev, `repeat(${n}) gave ${tokens} < ${prev}`);
    prev = tokens;
  }
});

test("deterministic", () => {
  const text = "function foo(barBaz) { return barBaz * 2; }";
  assert.equal(estimateTokens(text), estimateTokens(text));
});

test("code-like text lands in a plausible tokens-per-char band", () => {
  const code = 'if (userAccountManager.isActive()) {\n  return httpResponse.status(200).json({ ok: true });\n}\n';
  const tokens = estimateTokens(code);
  // Real BPE tokenizers put dense code around 3-4 chars/token; the heuristic
  // should stay in a generous band around that, not be wildly off.
  assert.ok(tokens > code.length / 8, `too few: ${tokens}`);
  assert.ok(tokens < code.length / 1.5, `too many: ${tokens}`);
});

test("long identifiers count as multiple sub-word tokens", () => {
  const short = estimateTokens("ab");
  const long = estimateTokens("abcdefghijklmnopqrstuvwxyz_abcdefghijklm");
  assert.ok(long > short * 3);
});
