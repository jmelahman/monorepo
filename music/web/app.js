'use strict';

// All the skeleton does so far: show the server is up and where the owned
// library lives. Mixes, streaming and purchases hang off the api/ as they
// land.

const status = document.getElementById('status');

fetch('api/health')
  .then((r) => r.json())
  .then((h) => {
    status.textContent = h.ok ? `up · ${h.music}` : 'down';
  })
  .catch(() => {
    status.textContent = 'down';
  });
