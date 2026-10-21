'use strict';

// Loaded in the head, so that a chosen theme is there for the first paint.
// Without a choice, the page follows the system.
try {
  const theme = localStorage.getItem('notebook:theme');
  if (theme === 'light' || theme === 'dark') document.documentElement.dataset.theme = theme;
} catch {
  // Storage is disabled.
}
