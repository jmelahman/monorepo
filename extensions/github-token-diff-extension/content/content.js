"use strict";

// Orchestrator: a debounced, idempotent run() reacts to every navigation/render
// signal. Injection is a no-op when badges for the current PR already exist,
// so over-triggering (Turbo events + MutationObserver + interval) is cheap.

(() => {
  const GHTD = globalThis.GHTD;
  if (!GHTD) return;

  let currentKey = null;
  let stats = null;
  let lastError = null;
  let pending = null;

  async function run() {
    const loc = GHTD.parseGithubPrLocation(location.pathname);
    if (!loc) {
      if (currentKey) {
        currentKey = null;
        stats = null;
        lastError = null;
        GHTD.removeBadges();
      }
      return;
    }
    const key = `${loc.owner}/${loc.repo}#${loc.prNumber}`;
    if (key !== currentKey) {
      currentKey = key;
      stats = null;
      lastError = null;
      GHTD.removeBadges();
    }

    if (!stats && !lastError) {
      if (!pending) {
        pending = browser.runtime
          .sendMessage({ type: "GET_TOKEN_DIFF", owner: loc.owner, repo: loc.repo, prNumber: loc.prNumber })
          .then((res) => {
            if (currentKey !== key) return;
            if (res && res.ok) stats = res.stats;
            else lastError = (res && res.error) || "NO_RESPONSE";
          })
          .catch(() => {
            if (currentKey === key) lastError = "MESSAGING";
          })
          .finally(() => {
            pending = null;
            scheduleRun();
          });
      }
      return;
    }

    if (stats) {
      GHTD.injectHeaderBadge(key, stats.total);
      if (loc.view === "files") GHTD.injectFileBadges(key, stats.perFile);
    } else if (lastError === "TOO_LARGE") {
      GHTD.injectHeaderBadge(key, null, "Diff too large to estimate tokens");
    }
    // Other errors (private repo without session, network): stay silent.
  }

  let timer = null;
  function scheduleRun() {
    clearTimeout(timer);
    timer = setTimeout(run, 200);
  }

  // Layered navigation/async-render detection — all funnel into the same
  // debounced run(): Turbo soft navigations, history, React Suspense mounts.
  document.addEventListener("turbo:load", scheduleRun);
  document.addEventListener("turbo:render", scheduleRun);
  window.addEventListener("popstate", scheduleRun);
  new MutationObserver(scheduleRun).observe(document.body, { childList: true, subtree: true });
  // Belt-and-suspenders: recovers if GitHub renames its events; run() is idempotent.
  setInterval(run, 1500);
  scheduleRun();
})();
