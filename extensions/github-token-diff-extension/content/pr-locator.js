"use strict";

(() => {
  const GHTD = (globalThis.GHTD = globalThis.GHTD || {});

  // /{owner}/{repo}/pull/{n}[/(files|changes|commits|checks)...]
  // "changes" is the new React Files-changed experience; "files" the classic one.
  GHTD.parseGithubPrLocation = function parseGithubPrLocation(pathname) {
    const m = pathname.match(
      /^\/([^/]+)\/([^/]+)\/pull\/(\d+)(?:\/(files|changes|commits|checks))?(?:\/|$)/
    );
    if (!m) return null;
    const sub = m[4] || "conversation";
    const view = sub === "files" || sub === "changes" ? "files" : sub;
    return { owner: m[1], repo: m[2], prNumber: m[3], view };
  };
})();
