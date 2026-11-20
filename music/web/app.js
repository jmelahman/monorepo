'use strict';

// The player: the owned library as albums and artists, streams found by
// search or mixed from what is owned, and a bar that plays either. The
// views hang off the hash, so the back button and a reload both work.

const $ = (id) => document.getElementById(id);
const view = $('view');
const player = $('player');
const bar = $('player-bar');
const playpause = $('playpause');
const seek = $('seek');
const volume = $('volume');
const muteBtn = $('mute');
const toast = $('toast');

let library = []; // the artists, from api/library
let libraryState = 'loading'; // loading, ready or failed
let queue = []; // { key, name, sub, url, art, album }: owned tracks and streams alike
let playingIndex = -1; // where in the queue the player is
let mix = null; // the supermix last built, kept for the home page
let lastSearch = { q: '', tracks: null }; // the search last run, so back doesn't run it again
let apkReady = false; // whether the Android client can be offered

// The same cache names the service worker keeps (web/sw.js).
const AUDIO = 'music-audio-v1';
const offlineKey = (path) => './saved-audio/' + encodeURIComponent(path);

// The shell and saved albums come from the service worker; ask the browser
// to keep them, since eviction would take the offline library with it.
if ('serviceWorker' in navigator) {
  navigator.serviceWorker.register('sw.js');
  if (navigator.storage && navigator.storage.persist) {
    navigator.storage.persist().catch(() => {});
  }
}

fetch('api/health')
  .then((r) => r.json())
  .then((h) => setStatus(h.ok, h.ok ? `Server up · ${h.music}` : 'Server down'))
  .catch(() => setStatus(false, 'Server unreachable'));

function setStatus(up, text) {
  const s = $('status');
  s.classList.toggle('up', up);
  s.classList.toggle('down', !up);
  s.title = text;
  s.setAttribute('aria-label', text);
}

loadLibrary();

// The native client's APK, offered only where it can install: on the
// phone, when a build is actually mounted behind the endpoint.
if (/Android/.test(navigator.userAgent)) {
  fetch('api/apk', { method: 'HEAD' })
    .then((r) => {
      apkReady = r.ok;
      if (apkReady && route().kind === 'home') render();
    })
    .catch(() => {});
}

// ---------------------------------------------------------------- nodes

// el builds an element; attrs are set as properties, except aria-* and
// role, which only exist as attributes.
const el = (tag, attrs, ...children) => {
  const node = document.createElement(tag);
  for (const [k, v] of Object.entries(attrs || {})) {
    if (k === 'dataset') Object.assign(node.dataset, v);
    else if (k === 'style') node.style.cssText = v;
    else if (k === 'role' || k.startsWith('aria-')) node.setAttribute(k, v);
    else node[k] = v;
  }
  node.append(...children.flat().filter((c) => c != null && c !== false));
  return node;
};

// show puts a view on the page, skipping the false and null that
// conditional parts leave.
function show(...nodes) {
  view.replaceChildren(...nodes.flat().filter((n) => n != null && n !== false));
}

const svgNS = 'http://www.w3.org/2000/svg';

function icon(name) {
  const svg = document.createElementNS(svgNS, 'svg');
  svg.setAttribute('aria-hidden', 'true');
  const use = document.createElementNS(svgNS, 'use');
  use.setAttribute('href', `#i-${name}`);
  svg.append(use);
  return svg;
}

function setIcon(btn, name) {
  btn.querySelector('use').setAttribute('href', `#i-${name}`);
}

// hue tints a placeholder by its name, so each album's metal is its own
// and the same on every visit.
function hue(s) {
  let h = 0;
  for (const c of s) h = (h * 31 + c.codePointAt(0)) % 360;
  return h;
}

function initial(s) {
  const m = s.match(/[\p{L}\p{N}]/u);
  return m ? m[0].toUpperCase() : '♪';
}

// art is a cover over a placeholder of tinted metal with the name's
// initial; the placeholder stays when the image is missing.
function art(name, src, opts = {}) {
  const box = el('div', {
    className: `art${opts.round ? ' round' : ''}`,
    style: `--hue: ${hue(name)}`,
    dataset: { initial: initial(name) },
  });
  if (src) {
    const img = el('img', { alt: '', loading: opts.eager ? 'eager' : 'lazy', decoding: 'async' });
    img.onload = () => img.classList.add('loaded');
    img.onerror = () => img.remove();
    img.src = src;
    box.append(img);
  }
  return box;
}

const coverUrl = (artist, album) =>
  `api/cover?artist=${encodeURIComponent(artist)}&album=${encodeURIComponent(album)}`;

// An artist's picture is the cover of their first album with one; the
// grid can't know which ahead, so it uses the first.
const artistCover = (a) => (a.albums.length ? coverUrl(a.name, a.albums[0].name) : '');

// The leading track number a file name carries, shown as the row's own.
const trackNumber = /^\d+[\s._-]*/;
const trackName = (name) => name.replace(trackNumber, '') || name;

function mb(bytes) {
  return `${(bytes / 1e6).toFixed(1)} MB`;
}

function fmtTime(seconds) {
  if (!Number.isFinite(seconds) || seconds < 0) return '0:00';
  seconds = Math.floor(seconds);
  const h = Math.floor(seconds / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  const s = String(seconds % 60).padStart(2, '0');
  return h ? `${h}:${String(m).padStart(2, '0')}:${s}` : `${m}:${s}`;
}

const plural = (n, one, many = `${one}s`) => `${n} ${n === 1 ? one : many}`;

function shuffled(xs) {
  const out = xs.slice();
  for (let i = out.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1));
    [out[i], out[j]] = [out[j], out[i]];
  }
  return out;
}

// Per-viewer conveniences, kept where the browser allows.
function remember(key, fallback) {
  try {
    const v = localStorage.getItem(key);
    return v == null ? fallback : JSON.parse(v);
  } catch {
    return fallback;
  }
}

function keep(key, value) {
  try {
    localStorage.setItem(key, JSON.stringify(value));
  } catch {
    // Private windows and full disks just forget.
  }
}

// --------------------------------------------------------------- routes

// route reads the hash: #/, #/library[/artists], #/artist/A,
// #/album/A/B or #/search/Q.
function route() {
  const parts = location.hash.replace(/^#\/?/, '').split('/').map(decodeURIComponent);
  switch (parts[0]) {
    case 'library':
      return { kind: 'library', tab: parts[1] === 'artists' ? 'artists' : 'albums' };
    case 'artist':
      return { kind: 'artist', artist: parts[1] || '' };
    case 'album':
      return { kind: 'album', artist: parts[1] || '', album: parts.slice(2).join('/') };
    case 'search':
      return { kind: 'search', q: parts.slice(1).join('/') };
    default:
      return { kind: 'home' };
  }
}

const href = (...parts) => '#/' + parts.map(encodeURIComponent).join('/');

function render() {
  const r = route();
  for (const a of document.querySelectorAll('.tabs a')) {
    const on = a.dataset.tab === r.kind || (a.dataset.tab === 'library' && (r.kind === 'artist' || r.kind === 'album'));
    if (on) a.setAttribute('aria-current', 'page');
    else a.removeAttribute('aria-current');
  }
  if (r.kind !== 'search') $('search-q').value = '';
  switch (r.kind) {
    case 'library':
      renderLibrary(r.tab);
      break;
    case 'artist':
      renderArtist(r.artist);
      break;
    case 'album':
      renderAlbum(r.artist, r.album);
      break;
    case 'search':
      $('search-q').value = r.q;
      renderSearch(r.q);
      break;
    default:
      renderHome();
  }
  markTrack();
  markBuys();
}

window.addEventListener('hashchange', () => {
  render();
  window.scrollTo(0, 0);
  view.focus({ preventScroll: true });
});

$('search').addEventListener('submit', (e) => {
  e.preventDefault();
  const q = $('search-q').value.trim();
  if (q) location.hash = href('search', q);
});

function loadLibrary() {
  return fetch('api/library')
    .then((r) => {
      if (!r.ok) throw new Error(r.status);
      return r.json();
    })
    .then((artists) => {
      library = artists;
      libraryState = 'ready';
    })
    .catch(() => {
      libraryState = 'failed';
    })
    .finally(() => {
      if (route().kind !== 'search') render();
    });
}

// libraryPending stands in for a view while the library loads or after
// it couldn't.
function libraryPending() {
  if (libraryState === 'ready') return null;
  return el('p', {
    className: 'empty',
    textContent: libraryState === 'loading' ? 'Reading the library…' : 'The library could not be read.',
  });
}

const findArtist = (name) => library.find((a) => a.name === name);

// ----------------------------------------------------------------- home

function renderHome() {
  const recent = remember('music-recent', [])
    .map((r) => {
      const a = findArtist(r.artist);
      const al = a && a.albums.find((x) => x.name === r.album);
      return al && { artist: a, album: al };
    })
    .filter(Boolean);
  const all = library.flatMap((a) => a.albums.map((al) => ({ artist: a, album: al })));

  show(
    apkReady &&
      el(
        'section',
        { className: 'shelf' },
        el('a', { className: 'btn chrome', href: 'api/apk', download: '' }, icon('save'), 'Get the Android app')
      ),
    el(
      'section',
      { className: `hero${mix === 'loading' ? ' spinning' : ''}`, 'aria-labelledby': 'mix-title' },
      el('div', { className: 'hero-disc', 'aria-hidden': 'true' }),
      el(
        'div',
        {},
        el('p', { className: 'kicker', textContent: 'Made from your library' }),
        el('h2', { id: 'mix-title', textContent: 'Supermix' }),
        el('p', {
          textContent: 'Endless streams seeded by what you already own. Hear something you like, buy it in.',
        }),
        el(
          'div',
          { className: 'btn-row' },
          el(
            'button',
            { className: 'btn chrome', type: 'button', disabled: mix === 'loading', onclick: buildMix },
            icon('mix'),
            mix === 'loading' ? 'Mixing…' : Array.isArray(mix) ? 'Mix again' : 'Start a mix'
          ),
          Array.isArray(mix) &&
            mix.length > 0 &&
            el(
              'button',
              { className: 'btn ghost', type: 'button', onclick: () => playStreams(mix, 0) },
              icon('play'),
              'Play'
            )
        )
      )
    ),
    Array.isArray(mix) && shelf('Your mix', null, streamList(mix)),
    libraryPending(),
    recent.length > 0 && shelf('Jump back in', null, albumRow(recent)),
    all.length > 0 && shelf('From your library', '#/library', albumRow(shuffled(all).slice(0, 18)))
  );
}

function shelf(title, more, body) {
  return el(
    'section',
    { className: 'shelf' },
    el(
      'div',
      { className: 'shelf-head' },
      el('h2', { textContent: title }),
      more && el('a', { className: 'more', href: more, textContent: 'See all' })
    ),
    body
  );
}

function albumRow(items) {
  return el('ul', { className: 'grid row-scroll' }, items.map((x) => el('li', {}, albumTile(x.artist, x.album))));
}

async function buildMix() {
  mix = 'loading';
  render();
  try {
    const r = await fetch('api/yt/supermix');
    if (!r.ok) throw new Error((await r.text()).trim() || r.status);
    mix = await r.json();
  } catch (err) {
    mix = null;
    notice(`The mix could not be built: ${err.message}`);
  }
  if (route().kind === 'home') render();
  if (Array.isArray(mix) && mix.length) playStreams(mix, 0);
}

// -------------------------------------------------------------- library

function renderLibrary(tab) {
  const chips = el(
    'div',
    { className: 'chips', role: 'group', 'aria-label': 'Show' },
    ['albums', 'artists'].map((t) =>
      el('a', {
        className: 'chip',
        href: t === 'albums' ? '#/library' : '#/library/artists',
        textContent: t === 'albums' ? 'Albums' : 'Artists',
        'aria-current': t === tab ? 'page' : 'false',
      })
    )
  );
  const pending = libraryPending();
  let body;
  if (pending) {
    body = pending;
  } else if (tab === 'artists') {
    body = el(
      'ul',
      { className: 'grid' },
      library.map((a) =>
        el(
          'li',
          {},
          el(
            'a',
            { className: 'tile artist', href: href('artist', a.name) },
            art(a.name, artistCover(a), { round: true }),
            el('span', { className: 'tile-title', textContent: a.name }),
            el('span', { className: 'tile-sub', textContent: plural(a.albums.length, 'album') })
          )
        )
      )
    );
  } else {
    body = el(
      'ul',
      { className: 'grid' },
      library.flatMap((a) => a.albums.map((al) => el('li', {}, albumTile(a, al))))
    );
  }
  show(el('h2', { className: 'view-title', textContent: 'Library' }), chips, body);
}

function albumTile(artist, album) {
  return el(
    'a',
    { className: 'tile', href: href('album', artist.name, album.name) },
    art(album.name, coverUrl(artist.name, album.name)),
    el('span', { className: 'tile-title', textContent: album.name }),
    el('span', { className: 'tile-sub', textContent: artist.name })
  );
}

function renderArtist(name) {
  const a = findArtist(name);
  if (!a) {
    show(libraryPending() || el('p', { className: 'empty', textContent: `No artist “${name}”.` }));
    return;
  }
  const tracks = a.albums.flatMap((al) => ownedItems(a, al));
  show(
    el(
      'header',
      { className: 'album-head' },
      art(a.name, artistCover(a), { round: true, eager: true }),
      el(
        'div',
        {},
        el('p', { className: 'kicker', textContent: 'Artist' }),
        el('h2', { textContent: a.name }),
        el('p', {
          className: 'meta',
          textContent: `${plural(a.albums.length, 'album')} · ${plural(tracks.length, 'track')}`,
        }),
        el(
          'div',
          { className: 'btn-row' },
          el(
            'button',
            { className: 'btn chrome', type: 'button', onclick: () => playQueue(tracks, 0) },
            icon('play'),
            'Play'
          ),
          el(
            'button',
            { className: 'btn', type: 'button', onclick: () => playQueue(shuffled(tracks), 0) },
            icon('shuffle'),
            'Shuffle'
          )
        )
      )
    ),
    shelf('Albums', null, el('ul', { className: 'grid' }, a.albums.map((al) => el('li', {}, albumTile(a, al)))))
  );
}

// ownedItems are an album's tracks as the queue holds them.
function ownedItems(artist, album) {
  return album.tracks.map((t) => ({
    key: t.path,
    name: trackName(t.name),
    sub: `${artist.name} · ${album.name}`,
    url: `api/audio?path=${encodeURIComponent(t.path)}`,
    art: coverUrl(artist.name, album.name),
    album: { artist: artist.name, album: album.name },
  }));
}

function renderAlbum(artistName, albumName) {
  const a = findArtist(artistName);
  const album = a && a.albums.find((x) => x.name === albumName);
  if (!album) {
    show(
      libraryPending() || el('p', { className: 'empty', textContent: `No album “${albumName}”.` })
    );
    return;
  }
  const items = ownedItems(a, album);
  const total = album.tracks.reduce((n, t) => n + t.size, 0);
  const saveBtn = el('button', { className: 'btn', type: 'button', onclick: () => toggleSave(album, saveBtn) });
  refreshSaveButton(album, saveBtn);

  show(
    el(
      'header',
      { className: 'album-head' },
      art(album.name, coverUrl(a.name, album.name), { eager: true }),
      el(
        'div',
        {},
        el('p', { className: 'kicker', textContent: album.name === 'Singles' ? 'Singles' : 'Album' }),
        el('h2', { textContent: album.name }),
        el(
          'p',
          { className: 'meta' },
          el('a', { href: href('artist', a.name), textContent: a.name }),
          ` · ${plural(album.tracks.length, 'track')} · ${mb(total)}`
        ),
        el(
          'div',
          { className: 'btn-row' },
          el(
            'button',
            { className: 'btn chrome', type: 'button', onclick: () => playQueue(items, 0) },
            icon('play'),
            'Play'
          ),
          el(
            'button',
            { className: 'btn', type: 'button', onclick: () => playQueue(shuffled(items), 0) },
            icon('shuffle'),
            'Shuffle'
          ),
          saveBtn
        )
      )
    ),
    el(
      'ol',
      { className: 'tracks' },
      album.tracks.map((t, i) =>
        el(
          'li',
          {},
          el(
            'button',
            {
              className: 'track',
              type: 'button',
              dataset: { key: items[i].key },
              onclick: () => playQueue(items, i),
            },
            el('span', { className: 'num', textContent: String(i + 1) }),
            el('span', {}, el('span', { className: 'name', textContent: items[i].name })),
            el('span', { className: 'end' }, el('span', { className: 'size', textContent: mb(t.size) }))
          )
        )
      )
    )
  );
}

// --------------------------------------------------------------- search

function renderSearch(q) {
  if (lastSearch.q === q && lastSearch.tracks) {
    show(searchHead(q), streamList(lastSearch.tracks));
    return;
  }
  show(searchHead(q), el('p', { className: 'empty', textContent: 'Searching…' }));
  fetch(`api/yt/search?q=${encodeURIComponent(q)}`)
    .then((r) => {
      if (!r.ok) throw new Error(r.status);
      return r.json();
    })
    .then((tracks) => {
      lastSearch = { q, tracks };
      if (route().kind === 'search' && route().q === q) render();
    })
    .catch(() => {
      if (route().kind === 'search' && route().q === q) {
        show(searchHead(q), el('p', { className: 'empty', textContent: 'The search could not be read.' }));
      }
    });
}

function searchHead(q) {
  return el(
    'div',
    { className: 'shelf-head' },
    el('div', {}, el('p', { className: 'kicker', textContent: 'Streams for' }), el('h2', { textContent: q }))
  );
}

const streamItems = (tracks) =>
  tracks.map((t) => ({
    key: `yt:${t.videoId}`,
    name: t.title,
    sub: t.artist,
    url: `api/yt/stream?v=${encodeURIComponent(t.videoId)}`,
    art: t.thumb,
  }));

function playStreams(tracks, i) {
  playQueue(streamItems(tracks), i);
}

// streamList lists streams with their stills and a buy button each. The
// row can't be a button, as it holds one, so it takes a button's keys.
function streamList(tracks) {
  if (!tracks.length) return el('p', { className: 'empty', textContent: 'Nothing found.' });
  const items = streamItems(tracks);
  return el(
    'ol',
    { className: 'tracks' },
    tracks.map((t, i) => {
      const play = () => playQueue(items, i);
      return el(
        'li',
        {},
        el(
          'div',
          {
            className: 'track stream',
            role: 'button',
            tabIndex: 0,
            dataset: { key: items[i].key },
            onclick: play,
            onkeydown: (e) => {
              if (e.target === e.currentTarget && (e.key === 'Enter' || e.key === ' ')) {
                e.preventDefault();
                play();
              }
            },
          },
          art(t.title, t.thumb),
          el(
            'span',
            {},
            el('span', { className: 'name', textContent: t.title }),
            el('span', { className: 'sub', textContent: t.artist })
          ),
          el(
            'span',
            { className: 'end' },
            el('span', { textContent: t.duration ? fmtTime(t.duration) : '' }),
            el('button', {
              className: 'buy',
              type: 'button',
              title: 'Buy this track into the owned library',
              dataset: { buy: buyKey(t.artist, t.title) },
              onclick: (e) => buyTrack(e, t),
              textContent: 'Buy',
            })
          )
        )
      );
    })
  );
}

// ------------------------------------------------------- saved offline

// The button mirrors the album's saved state: how many of its tracks the
// cache holds.
async function refreshSaveButton(album, btn) {
  if (!('caches' in window)) {
    btn.hidden = true;
    return;
  }
  const have = await savedCount(album);
  const all = have === album.tracks.length;
  btn.replaceChildren(
    icon(all ? 'saved' : 'save'),
    all ? 'Saved' : have ? `Save offline (${have}/${album.tracks.length})` : 'Save offline'
  );
  btn.title = all ? 'Saved offline — click to remove' : 'Keep this album for when there is no connection';
}

function savedCount(album) {
  return caches.open(AUDIO).then((cache) =>
    Promise.all(album.tracks.map((t) => cache.match(offlineKey(t.path)))).then((hits) => hits.filter(Boolean).length)
  );
}

// toggleSave pins an album into the cache, track by track, or clears it
// once everything is there. Saved tracks play from the cache, so this is
// also a "stop streaming this album" switch.
async function toggleSave(album, btn) {
  const cache = await caches.open(AUDIO);
  const have = await savedCount(album);
  if (have === album.tracks.length) {
    await Promise.all(album.tracks.map((t) => cache.delete(offlineKey(t.path))));
    await refreshSaveButton(album, btn);
    notice(`Removed ${album.name} from this device`);
    return;
  }
  btn.disabled = true;
  let saved = have;
  for (const t of album.tracks) {
    if (await cache.match(offlineKey(t.path))) continue;
    try {
      const resp = await fetch(`api/audio?path=${encodeURIComponent(t.path)}`);
      if (!resp.ok) throw new Error(resp.status);
      await cache.put(offlineKey(t.path), resp);
    } catch {
      notice(`Could not save ${trackName(t.name)} — the device may be out of space`);
      break;
    }
    saved++;
    btn.replaceChildren(icon('save'), `Saving… ${saved}/${album.tracks.length}`);
  }
  btn.disabled = false;
  await refreshSaveButton(album, btn);
  if (saved === album.tracks.length) notice(`${album.name} is saved for offline`);
}

// ----------------------------------------------------------------- play

// playQueue starts the queue at an index; the queue holds owned tracks
// and streams alike, each with the key its row is marked by.
function playQueue(items, index) {
  if (!items.length) return;
  queue = items;
  playingIndex = index;
  const t = items[index];
  player.src = t.url;
  player.play().catch(() => {});
  if (t.album) rememberAlbum(t.album);

  bar.hidden = false;
  document.body.classList.add('playing-bar');
  $('np-title').textContent = t.name;
  $('np-sub').textContent = t.sub;
  $('np-art').replaceWith(Object.assign(art(t.name, t.art, { eager: true }), { id: 'np-art' }));
  updateTime();
  markTrack();

  if ('mediaSession' in navigator) {
    // The lock screen's controls, on an installed app.
    navigator.mediaSession.metadata = new MediaMetadata({
      title: t.name,
      artist: t.album ? t.album.artist : t.sub,
      album: t.album ? t.album.album : '',
      artwork: t.art ? [{ src: new URL(t.art, location.href).href }] : [],
    });
  }
}

if ('mediaSession' in navigator) {
  navigator.mediaSession.setActionHandler('previoustrack', () => step(-1));
  navigator.mediaSession.setActionHandler('nexttrack', () => step(1));
}

// rememberAlbum keeps the albums last played, newest first, for "Jump
// back in".
function rememberAlbum(album) {
  const recent = remember('music-recent', []).filter((r) => r.artist !== album.artist || r.album !== album.album);
  keep('music-recent', [album, ...recent].slice(0, 12));
}

function step(delta) {
  // Back from a few seconds in restarts the track, as players do.
  if (delta < 0 && player.currentTime > 3) {
    player.currentTime = 0;
    return;
  }
  const next = playingIndex + delta;
  if (next >= 0 && next < queue.length) playQueue(queue, next);
}

player.addEventListener('ended', () => step(1));

// A track that won't load offline is a track that was never saved.
player.addEventListener('error', () => {
  notice(navigator.onLine ? 'This track could not be played' : 'Offline, and this album is not saved');
});

$('prev').addEventListener('click', () => step(-1));
$('next').addEventListener('click', () => step(1));

function togglePlay() {
  if (!player.src) return;
  if (player.paused) player.play().catch(() => {});
  else player.pause();
}

playpause.addEventListener('click', togglePlay);

function syncPlaying() {
  const paused = player.paused;
  setIcon(playpause, paused ? 'play' : 'pause');
  playpause.setAttribute('aria-label', paused ? 'Play' : 'Pause');
  document.body.classList.toggle('paused', paused);
}

player.addEventListener('play', syncPlaying);
player.addEventListener('pause', syncPlaying);

// Space plays and pauses, unless something focused wants it.
document.addEventListener('keydown', (e) => {
  if (e.key !== ' ' || e.defaultPrevented || e.ctrlKey || e.metaKey || e.altKey) return;
  if (e.target.closest('input, textarea, button, a, [role="button"], select')) return;
  e.preventDefault();
  togglePlay();
});

// ----------------------------------------------------------- the rails

let seeking = false;

function setPct(range, pct) {
  range.style.setProperty('--pct', `${pct}%`);
}

function updateTime() {
  const d = player.duration;
  const t = player.currentTime;
  $('time').textContent = `${fmtTime(t)} / ${fmtTime(d)}`;
  if (seeking) return;
  const pct = Number.isFinite(d) && d > 0 ? (t / d) * 100 : 0;
  seek.value = String(Math.round(pct * 10));
  setPct(seek, pct);
}

player.addEventListener('timeupdate', updateTime);
player.addEventListener('durationchange', updateTime);

seek.addEventListener('input', () => {
  seeking = true;
  setPct(seek, seek.value / 10);
});

seek.addEventListener('change', () => {
  seeking = false;
  if (Number.isFinite(player.duration)) player.currentTime = (seek.value / 1000) * player.duration;
});

function syncVolume() {
  const level = player.muted ? 0 : player.volume;
  volume.value = String(Math.round(level * 100));
  setPct(volume, level * 100);
  setIcon(muteBtn, level === 0 ? 'mute' : 'volume');
  muteBtn.setAttribute('aria-label', player.muted ? 'Unmute' : 'Mute');
}

volume.addEventListener('input', () => {
  player.volume = volume.value / 100;
  player.muted = player.volume === 0;
  keep('music-volume', player.volume);
});

muteBtn.addEventListener('click', () => {
  player.muted = !player.muted;
  if (!player.muted && player.volume === 0) player.volume = 0.5;
});

player.addEventListener('volumechange', syncVolume);
player.volume = Math.min(1, Math.max(0, Number(remember('music-volume', 1)) || 0));
syncVolume();

// Highlights the row of what is playing, in whichever list holds it.
function markTrack() {
  const current = queue[playingIndex]?.key;
  for (const row of document.querySelectorAll('.track[data-key]')) {
    const on = row.dataset.key === current;
    row.classList.toggle('current', on);
    if (on) row.setAttribute('aria-current', 'true');
    else row.removeAttribute('aria-current');
  }
}

// ---------------------------------------------------------------- toast

let toastTimer = null;

function notice(text) {
  toast.textContent = text;
  toast.hidden = false;
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => {
    toast.hidden = true;
  }, 6000);
}

// ---------------------------------------------------------------- buys

// The server queues buys and runs them one at a time; the page polls the
// queue while any are pending and marks each track's button with where
// its buy stands.
let buys = []; // from api/buys, newest first
let buysPoll = null;

const buyKey = (artist, title) => `${artist}\u0000${title}`.toLowerCase();

const BUY_LABELS = { queued: 'Queued', buying: 'Buying…', done: 'Owned', failed: 'Retry' };

// buyTrack hands a streamed track to the server's queue.
async function buyTrack(e, t) {
  e.stopPropagation();
  const btn = e.currentTarget;
  // A buy spends the store account's balance; say so before it does.
  if (!confirm(`Buy ${t.artist} — ${t.title} into the library?`)) return;
  btn.disabled = true;
  btn.textContent = 'Queued';
  try {
    const r = await fetch('api/buy', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ artist: t.artist, title: t.title }),
    });
    if (!r.ok) throw new Error((await r.text()).trim().slice(0, 120) || r.status);
    await refreshBuys();
  } catch (err) {
    notice(`The buy could not be queued: ${err.message}`);
    btn.disabled = false;
    btn.textContent = 'Buy';
  }
}

// refreshBuys reads the queue, says what finished since the last read,
// and keeps polling while anything is pending.
async function refreshBuys() {
  let next;
  try {
    const r = await fetch('api/buys');
    if (!r.ok) return;
    next = await r.json();
  } catch {
    return;
  }
  const before = new Map(buys.map((b) => [b.id, b.status]));
  let landed = false;
  for (const b of next) {
    const was = before.get(b.id);
    if (was === undefined || was === b.status) continue;
    if (b.status === 'done') {
      landed = true;
      const r = b.result;
      notice(
        !r
          ? `Bought: ${b.artist} — ${b.title}`
          : r.alreadyOwned
            ? `Already in the library — ${r.artist} · ${r.title}`
            : `Bought: ${r.artist} — ${r.title} (${r.album})`
      );
    } else if (b.status === 'failed') {
      notice(`The buy did not go through: ${b.artist} — ${b.title}: ${b.error}`);
    }
  }
  buys = next;
  markBuys();
  if (landed) loadLibrary();

  const pending = buys.some((b) => b.status === 'queued' || b.status === 'buying');
  if (pending && !buysPoll) {
    buysPoll = setInterval(refreshBuys, 3000);
  } else if (!pending && buysPoll) {
    clearInterval(buysPoll);
    buysPoll = null;
  }
}

// markBuys labels each listed track's button with its newest buy.
function markBuys() {
  const newest = new Map();
  for (const b of buys) {
    const k = buyKey(b.artist, b.title);
    if (!newest.has(k)) newest.set(k, b);
  }
  for (const btn of view.querySelectorAll('.buy')) {
    const b = newest.get(btn.dataset.buy);
    if (!b) continue;
    btn.textContent = BUY_LABELS[b.status] || 'Buy';
    btn.disabled = b.status !== 'failed';
    btn.classList.toggle('owned', b.status === 'done');
    if (b.status === 'failed') btn.title = `The last try failed: ${b.error}`;
    else if (b.status === 'queued' && b.error) btn.title = `Retrying after: ${b.error}`;
  }
}

render();
refreshBuys();
