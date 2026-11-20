'use strict';

// Keeps the installed app working off the tailnet: the shell and the
// library listing are cached so the page opens, and albums saved for
// offline are served from the cache, with range support synthesized so
// seeking works without the network. Everything is network-first, so the
// online page is always the current one; the cache answers when the
// network doesn't.

const SHELL = 'music-shell-v1';
const AUDIO = 'music-audio-v1'; // the same name the page uses for saving

// The shell: the page and the shared design, all that renders before the
// first fetch.
const shell = [
  './',
  'index.html',
  'style.css',
  'app.js',
  'favicon.svg',
  'manifest.webmanifest',
  'design/design.css',
  'design/toolbar.css',
  'design/theme.js',
  'design/fonts/plex-cond-600.woff2',
  'design/fonts/plex-sans-var.woff2',
  'design/fonts/plex-mono-400.woff2',
  'design/fonts/plex-mono-600.woff2',
];

self.addEventListener('install', (event) => {
  event.waitUntil(
    caches
      .open(SHELL)
      .then((cache) => Promise.allSettled(shell.map((u) => cache.add(u))))
      .then(() => self.skipWaiting())
  );
});

self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches
      .keys()
      .then((names) =>
        Promise.all(names.filter((n) => n !== SHELL && n !== AUDIO).map((n) => caches.delete(n)))
      )
      .then(() => self.clients.claim())
  );
});

// offlineKey is the cache key the page and this worker share for a saved
// track: relative, and never a route the server answers.
const offlineKey = (path) => './saved-audio/' + encodeURIComponent(path);

self.addEventListener('fetch', (event) => {
  const req = event.request;
  if (req.method !== 'GET') return;
  const url = new URL(req.url);
  if (url.origin !== self.location.origin) return;

  // The app is served at the root or behind a path; name is its route
  // relative to whichever it is.
  const scope = new URL(self.registration.scope).pathname;
  const name = url.pathname.slice(scope.length - 1) || '/';

  // The Android client's APK: tens of megabytes, a new one every build,
  // and nothing the cache is for. Its version file rides along.
  if (name.startsWith('/api/apk') || name.startsWith('/api/app')) {
    return;
  }

  if (name.startsWith('/api/audio')) {
    event.respondWith(audio(req, url));
  } else if (name.startsWith('/api/library')) {
    event.respondWith(networkFirst(req, SHELL, './library.json'));
  } else if (req.mode === 'navigate') {
    event.respondWith(networkFirst(req, SHELL, './'));
  } else {
    event.respondWith(networkFirst(req, SHELL, req));
  }
});

// audio serves a saved track from the cache, online or not — saved files
// don't change — and falls back to the network; without either, the page
// gets a short 503 to explain itself with.
async function audio(req, url) {
  const path = url.searchParams.get('path') || '';
  const cache = await caches.open(AUDIO);
  const saved = await cache.match(offlineKey(path));
  if (saved) {
    return range(req, saved);
  }
  try {
    return await fetch(req);
  } catch {
    return new Response('not saved for offline', { status: 503 });
  }
}

// range answers one request for a saved track, synthesizing the partial
// responses the audio element asks for when it seeks.
async function range(req, saved) {
  const body = await saved.arrayBuffer();
  const type = saved.headers.get('Content-Type') || 'audio/mpeg';
  const m = /^bytes=(\d+)-(\d*)$/.exec(req.headers.get('Range') || '');
  if (!m || body.byteLength === 0) {
    return new Response(body, {
      status: 200,
      headers: { 'Content-Type': type, 'Accept-Ranges': 'bytes' },
    });
  }
  const start = Math.min(Number(m[1]), body.byteLength - 1);
  const end = m[2] === '' ? body.byteLength - 1 : Math.min(Number(m[2]), body.byteLength - 1);
  const slice = body.slice(start, end + 1);
  return new Response(slice, {
    status: 206,
    headers: {
      'Content-Type': type,
      'Content-Range': `bytes ${start}-${end}/${body.byteLength}`,
      'Accept-Ranges': 'bytes',
    },
  });
}

// networkFirst answers from the network, keeping a copy in cache, and
// falls back to the copy when the network is unreachable. key is where
// the copy lives in the cache: a fixed name for the page and the library,
// the request itself for the rest.
async function networkFirst(req, cacheName, key) {
  const cache = await caches.open(cacheName);
  try {
    const fresh = await fetch(req);
    if (fresh.ok && key) {
      cache.put(key, fresh.clone());
    }
    return fresh;
  } catch {
    const hit = await cache.match(key);
    if (hit) return hit;
    return new Response('offline and not saved', { status: 503 });
  }
}
