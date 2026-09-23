// Two-tier cache for computed token stats: in-memory Map (fast path within one
// event-page lifetime) + storage.session (survives event-page suspension, is
// cleared on browser exit). Stores { etag, stats, fetchedAt } — never the raw
// diff text, so private-repo content stays out of extension storage.

export class DiffStatsCache {
  constructor() {
    this.mem = new Map();
  }

  key(owner, repo, prNumber) {
    return `${owner}/${repo}#${prNumber}`;
  }

  get(owner, repo, prNumber) {
    return this.mem.get(this.key(owner, repo, prNumber)) ?? null;
  }

  async hydrate(owner, repo, prNumber) {
    const key = this.key(owner, repo, prNumber);
    try {
      const stored = await browser.storage.session.get(key);
      const entry = stored?.[key] ?? null;
      if (entry) this.mem.set(key, entry);
      return entry;
    } catch {
      return null;
    }
  }

  async set(owner, repo, prNumber, entry) {
    const key = this.key(owner, repo, prNumber);
    this.mem.set(key, entry);
    try {
      await browser.storage.session.set({ [key]: entry });
    } catch {
      // storage.session unavailable — in-memory tier still works
    }
  }
}
