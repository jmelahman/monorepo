'use strict';

// Shared by the no-build frontends: .config/git-orchard/shared/design in the
// monorepo (https://github.com/jmelahman/monorepo). Edit it there; `git orchard
// sync` copies it here.
//
// Loaded in the head, so that a chosen theme is there for the first paint.
// The choice is kept under one key, so every page on an origin shares it, and
// without one the page follows the system. The button that switches it is
// added to the end of the page's .toolbar, and styled by toolbar.css. Pages
// with more to restyle than CSS can reach listen for `themechange`.
(() => {
  const KEY = 'theme';
  const root = document.documentElement;
  const system = matchMedia('(prefers-color-scheme: dark)');
  let button = null;

  function stored() {
    try {
      const theme = localStorage.getItem(KEY);
      return theme === 'light' || theme === 'dark' ? theme : null;
    } catch {
      return null; // Storage is disabled.
    }
  }

  function apply(theme) {
    if (theme) root.dataset.theme = theme;
    else delete root.dataset.theme;
    const dark = theme ? theme === 'dark' : system.matches;
    root.classList.toggle('dark', dark);
    if (button) {
      const label = `Switch to the ${dark ? 'light' : 'dark'} theme`;
      button.title = label;
      button.ariaLabel = label;
    }
    document.dispatchEvent(new CustomEvent('themechange', { detail: { dark } }));
  }

  apply(stored());
  system.addEventListener('change', () => apply(root.dataset.theme));
  // Chosen on another page, or in another tab.
  addEventListener('storage', (e) => {
    if (e.key === KEY || e.key === null) apply(stored());
  });

  document.addEventListener('DOMContentLoaded', () => {
    const toolbar = document.querySelector('.toolbar');
    if (!toolbar) return;
    button = document.createElement('button');
    button.type = 'button';
    button.id = 'theme';
    button.className = 'icon';
    // The button shows the theme it switches to.
    button.innerHTML =
      '<svg viewBox="0 0 24 24" class="sun" aria-hidden="true"><path d="M12 8a4 4 0 100 8 4 4 0 000-8zM12 3v2M12 19v2M3 12h2M19 12h2M5.6 5.6L7 7M17 17l1.4 1.4M5.6 18.4L7 17M17 7l1.4-1.4"/></svg>' +
      '<svg viewBox="0 0 24 24" class="moon" aria-hidden="true"><path d="M20 14.5A8 8 0 019.5 4a8 8 0 1010.5 10.5z"/></svg>';
    button.addEventListener('click', () => {
      const theme = root.classList.contains('dark') ? 'light' : 'dark';
      try {
        localStorage.setItem(KEY, theme);
      } catch {
        // Storage is disabled; the theme just won't be remembered.
      }
      apply(theme);
    });
    toolbar.append(button);
    apply(root.dataset.theme);
  });
})();
