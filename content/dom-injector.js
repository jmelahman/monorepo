"use strict";

// Badge placement only — token numbers never come from the DOM, so if GitHub
// redesigns its markup the worst case is a worse-placed badge, never a wrong
// number. All DOM writes are try/catch-guarded appends of our own nodes.

(() => {
  const GHTD = (globalThis.GHTD = globalThis.GHTD || {});

  const format = (n) => n.toLocaleString();

  const SVG_NS = "http://www.w3.org/2000/svg";

  // Coin/token glyph, drawn inline (CSP forbids external assets).
  function coinIcon() {
    const svg = document.createElementNS(SVG_NS, "svg");
    svg.setAttribute("viewBox", "0 0 16 16");
    svg.setAttribute("aria-hidden", "true");
    svg.classList.add("ghtd-icon");
    const outer = document.createElementNS(SVG_NS, "circle");
    outer.setAttribute("cx", "8");
    outer.setAttribute("cy", "8");
    outer.setAttribute("r", "6.25");
    const inner = document.createElementNS(SVG_NS, "circle");
    inner.setAttribute("cx", "8");
    inner.setAttribute("cy", "8");
    inner.setAttribute("r", "2.75");
    for (const c of [outer, inner]) {
      c.setAttribute("fill", "none");
      c.setAttribute("stroke", "currentColor");
      c.setAttribute("stroke-width", "1.5");
      svg.appendChild(c);
    }
    return svg;
  }

  function tokenSpans(added, removed) {
    const addedEl = document.createElement("span");
    addedEl.className = "ghtd-added";
    addedEl.textContent = `+${format(added)}`;
    const removedEl = document.createElement("span");
    removedEl.className = "ghtd-removed";
    removedEl.textContent = `−${format(removed)}`;
    return [addedEl, removedEl];
  }

  function buildBadge(kind, key, total, note) {
    const badge = document.createElement("span");
    badge.className = kind === "file" ? "ghtd-badge ghtd-file-badge" : "ghtd-badge ghtd-header-badge";
    badge.dataset.ghtdBadge = kind;
    badge.dataset.ghtdKey = key;
    if (total) {
      badge.append(...tokenSpans(total.added, total.removed));
      badge.title = `Estimated token diff: ${format(total.added)} added, ${format(total.removed)} removed (heuristic)`;
    } else {
      badge.append("n/a");
      badge.title = note || "Token estimate unavailable";
    }
    badge.appendChild(coinIcon());
    return badge;
  }

  // The header badge belongs just before GitHub's own "+X −Y" line diffstat.
  // React header: DiffSquares carry data-testid="... diffstat"; two parents up
  // is the widget holding the numbers + squares. Classic header: span#diffstat.
  function findHeaderAnchor() {
    const square = document.querySelector(
      '[data-testid="addition diffstat"], [data-testid="deletion diffstat"], [data-testid="neutral diffstat"]'
    );
    const widget = square?.parentElement?.parentElement;
    if (widget) return { anchor: widget, place: "prepend" };
    const classic = document.querySelector("#diffstat") || document.querySelector(".tabnav-extra .diffstat");
    if (classic) return { anchor: classic, place: "before" };
    const nav = document.querySelector('nav[aria-label="Pull request navigation tabs"], .tabnav-tabs');
    if (nav) return { anchor: nav, place: "append" };
    const title = document.querySelector('.js-issue-title, h1[data-component="PH_Title"]');
    if (title) return { anchor: title, place: "after" };
    return null;
  }

  GHTD.injectHeaderBadge = function injectHeaderBadge(key, total, note) {
    try {
      const existing = document.querySelector('[data-ghtd-badge="header"]');
      if (existing) {
        if (existing.dataset.ghtdKey === key) return;
        existing.remove();
      }
      const badge = buildBadge("header", key, total, note);
      const found = findHeaderAnchor();
      if (!found) {
        // Last resort: feature never silently disappears.
        badge.classList.add("ghtd-floating");
        document.body.appendChild(badge);
        return;
      }
      if (found.place === "prepend") found.anchor.prepend(badge);
      else if (found.place === "before") found.anchor.insertAdjacentElement("beforebegin", badge);
      else if (found.place === "append") found.anchor.appendChild(badge);
      else found.anchor.insertAdjacentElement("afterend", badge);
    } catch {
      // never break GitHub's page
    }
  };

  GHTD.injectFileBadges = function injectFileBadges(key, perFile) {
    try {
      const byPath = new Map();
      for (const f of perFile) {
        if (f.path) byPath.set(f.path, f);
        if (f.oldPath && !byPath.has(f.oldPath)) byPath.set(f.oldPath, f);
      }

      // Classic Files-changed markup (/files, logged-out).
      const headers = document.querySelectorAll(".file-header[data-path]");
      for (const header of headers) {
        placeFileBadge(header, header.getAttribute("data-path"), key, byPath, header.querySelector(".diffstat"));
      }
      if (headers.length) return;
      const wrappers = document.querySelectorAll("[data-tagsearch-path]");
      for (const wrapper of wrappers) {
        const target = wrapper.querySelector(".file-header, .file-info, h3") || wrapper.firstElementChild;
        if (target) placeFileBadge(target, wrapper.getAttribute("data-tagsearch-path"), key, byPath, null);
      }
      if (wrappers.length) return;

      // New React Files-changed experience (/changes): hashed class names, so
      // match file headers by their path text instead of markup structure.
      for (const el of document.querySelectorAll("h3, [data-file-path], a[title]")) {
        if (el.closest('[role="tree"], nav')) continue; // skip the file tree sidebar
        const path =
          el.getAttribute("data-file-path") ||
          byPath.has(el.getAttribute("title")) && el.getAttribute("title") ||
          el.textContent.trim();
        if (byPath.has(path)) placeFileBadge(el, path, key, byPath, null);
      }
    } catch {
      // fail soft — header badge still works
    }
  };

  function placeFileBadge(container, path, key, byPath, afterEl) {
    const stats = byPath.get(path);
    if (!stats || stats.binary) return;

    // Placement varies per view (see below), so scope the dedup check to the
    // file's whole header row rather than assuming a fixed sibling position —
    // otherwise re-renders would stack duplicate badges.
    const row = container.closest('[class*="DiffFileHeader-module__diff-file-header"], .file-header');
    const dedupScope = row || container.parentElement || container;
    const existing = dedupScope.querySelector('[data-ghtd-badge="file"]');
    if (existing) {
      if (existing.dataset.ghtdKey === key) return;
      existing.remove();
    }

    const badge = buildBadge("file", key, { added: stats.added, removed: stats.removed });

    if (afterEl) {
      // Classic Files-changed markup: inline, right after the line diffstat.
      afterEl.insertAdjacentElement("afterend", badge);
      return;
    }

    // React "optimized for large PRs" view: our path match lands on an element
    // in the left file-path section, but GitHub's per-file line diffstat lives
    // in a separate right-aligned action row. Drop the token badge just before
    // that diffstat so it sits left of the line counter (mirrors the header
    // badge) instead of stranded at the right edge of the left section.
    const diffstat = row && row.querySelector('[class*="hideOnNarrowContainer"]');
    if (diffstat) {
      badge.classList.add("ghtd-file-badge-inline");
      diffstat.insertAdjacentElement("beforebegin", badge);
      return;
    }

    // Fallback: block-level sibling that hugs the right edge of its container.
    badge.classList.add("ghtd-file-badge-block");
    container.insertAdjacentElement("afterend", badge);
  }

  GHTD.removeBadges = function removeBadges() {
    try {
      for (const el of document.querySelectorAll("[data-ghtd-badge]")) el.remove();
    } catch {
      // ignore
    }
  };
})();
