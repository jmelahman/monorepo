import { fetchDiff } from "./diff-fetcher.js";
import { computeTokenStats } from "./token-stats.js";
import { DiffStatsCache } from "./cache.js";

const cache = new DiffStatsCache();
const inFlight = new Map();

async function getTokenDiff({ owner, repo, prNumber }) {
  const entry = cache.get(owner, repo, prNumber) ?? (await cache.hydrate(owner, repo, prNumber));
  const result = await fetchDiff(owner, repo, prNumber, { etag: entry?.etag });
  if (result.notModified && entry) return { ok: true, stats: entry.stats, fromCache: true };
  if (result.error) return { ok: false, error: result.error };
  const stats = computeTokenStats(result.text);
  await cache.set(owner, repo, prNumber, { etag: result.etag, stats, fetchedAt: Date.now() });
  return { ok: true, stats };
}

browser.runtime.onMessage.addListener((msg) => {
  if (msg?.type !== "GET_TOKEN_DIFF") return;
  const key = `${msg.owner}/${msg.repo}#${msg.prNumber}`;
  let pending = inFlight.get(key);
  if (!pending) {
    pending = getTokenDiff(msg).finally(() => inFlight.delete(key));
    inFlight.set(key, pending);
  }
  return pending;
});
