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
let order = []; // the queue's indices in the order they play: shuffled, or not
let mix = null; // the supermix last built, kept for the home page
let mixSource = ''; // account (YouTube Music's own) or radio (seeded from the library)
let libMix = null; // the library mix: null until asked for, then 'loading', a list or 'failed'
let playlists = null; // from api/playlists, without their tracks
const playlistTracks = new Map(); // id → the playlist with its tracks, as last read
let excluded = new Set(); // what is kept out of the mixes: paths, and yt:<id> for streams
let lastSearch = { q: '', tracks: null }; // the search last run, so back doesn't run it again
let apkReady = false; // whether the Android client can be offered
let mixSize = remember('music-mix-size', 50); // tracks in the library mix
let supermixReady = false; // whether the server has YouTube Music set up for the supermix

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
loadPlaylists();
fetch('api/excluded')
  .then((r) => (r.ok ? r.json() : []))
  .then((keys) => {
    excluded = new Set(keys);
    markExcluded();
  })
  .catch(() => {});

// The supermix is offered only once the server has YouTube Music set up.
fetch('api/mix')
  .then((r) => (r.ok ? r.json() : {}))
  .then((m) => {
    supermixReady = !!m.supermix;
    if (supermixReady && route().kind === 'home') render();
  })
  .catch(() => {});

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

// libraryStats is the line iTunes kept under its library: how many songs,
// how long they play and how much room they take.
function libraryStats() {
  const tracks = library.flatMap((a) => a.albums.flatMap((al) => al.tracks));
  const secs = tracks.reduce((n, t) => n + (t.duration || 0), 0);
  const bytes = tracks.reduce((n, t) => n + t.size, 0);
  const time =
    secs >= 86400
      ? `${(secs / 86400).toFixed(1)} days`
      : secs >= 3600
        ? `${(secs / 3600).toFixed(1)} hours`
        : plural(Math.round(secs / 60), 'minute');
  const size = bytes >= 2 ** 30 ? `${(bytes / 2 ** 30).toFixed(2)} GB` : `${(bytes / 2 ** 20).toFixed(1)} MB`;
  return `${plural(tracks.length, 'song').replace(/^\d+/, (n) => Number(n).toLocaleString())}, ${time}, ${size}`;
}

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

// route reads the hash: #/, #/library[/artists|/playlists], #/artist/A,
// #/album/A/B, #/playlist/ID or #/search/Q.
function route() {
  const parts = location.hash.replace(/^#\/?/, '').split('/').map(decodeURIComponent);
  switch (parts[0]) {
    case 'library':
      return { kind: 'library', tab: ['artists', 'playlists'].includes(parts[1]) ? parts[1] : 'albums' };
    case 'playlist':
      return { kind: 'playlist', id: parts[1] || '' };
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
    const inLibrary = ['artist', 'album', 'playlist'].includes(r.kind);
    const on = a.dataset.tab === r.kind || (a.dataset.tab === 'library' && inLibrary);
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
    case 'playlist':
      renderPlaylist(r.id);
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
  markExcluded();
  backdrop(backdropFor(r));
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
    el('div', { className: 'mixes' }, supermixReady ? supermixHero() : connectHero(), libraryMixHero()),
    Array.isArray(libMix) && libMix.length > 0 && shelf('Library mix', null, ownedList(libItems(libMix))),
    supermixReady && Array.isArray(mix) && shelf('Supermix', null, streamList(mix)),
    libraryPending(),
    recent.length > 0 && shelf('Jump back in', null, albumRow(recent)),
    all.length > 0 && shelf('From your library', '#/library', albumRow(shuffled(all).slice(0, 18)))
  );
}

// mixArt leads a mix's card with four of the covers it will play, or one
// when it has fewer, or the mix's mark before it has any. Each is a
// [name, cover] pair, the name drawing the tile when the cover won't.
function mixArt(covers) {
  const seen = new Set();
  const picked = covers.filter(([name, src]) => src && !seen.has(src) && seen.add(src)).slice(0, 4);
  const shown = picked.length === 4 ? picked : picked.slice(0, 1);
  return el(
    'div',
    { className: `mix-art${shown.length === 1 ? ' one' : ''}`, 'aria-hidden': 'true' },
    shown.length ? shown.map(([name, src]) => art(name, src, { eager: true })) : icon('mix')
  );
}

// The supermix streams YouTube Music's own for the account, or a radio
// seeded from the library when the account doesn't answer.
function supermixHero() {
  return el(
    'section',
    { className: `hero${mix === 'loading' ? ' spinning' : ''}`, 'aria-labelledby': 'mix-title' },
    mixArt(Array.isArray(mix) ? mix.map((t) => [t.title, t.thumb]) : []),
    el(
      'div',
      {},
      el('p', {
        className: 'kicker',
        textContent: mixSource === 'radio' ? 'Radio from your library' : 'From YouTube Music',
      }),
      el('h2', { id: 'mix-title', textContent: 'Supermix' }),
      el(
        'div',
        { className: 'btn-row' },
        el(
          'button',
          { className: 'btn chrome', type: 'button', disabled: mix === 'loading', onclick: () => buildMix(false) },
          icon('play'),
          mix === 'loading' ? 'Mixing…' : 'Play'
        ),
        Array.isArray(mix) &&
          el(
            'button',
            { className: 'btn ghost', type: 'button', onclick: () => buildMix(true) },
            icon('mix'),
            'Remix'
          )
      )
    ),
    el(
      'button',
      {
        className: 'icon-btn card-more',
        type: 'button',
        title: 'The YouTube Music account',
        'aria-label': 'The YouTube Music account',
        onclick: openYTAccount,
      },
      icon('more')
    )
  );
}

// Without YouTube Music set up, the supermix's place offers to set it up.
function connectHero() {
  return el(
    'section',
    { className: 'hero connect', 'aria-labelledby': 'connect-title' },
    mixArt([]),
    el(
      'div',
      {},
      el('p', { className: 'kicker', textContent: 'YouTube Music' }),
      el('h2', { id: 'connect-title', textContent: 'Supermix' }),
      el('p', { textContent: "Connect your account and the Supermix it makes for you plays here." }),
      el('div', { className: 'btn-row' }, button('btn chrome', openYTAccount, 'Connect YouTube Music'))
    )
  );
}

// openYTAccount is the panel that sets up the account's supermix: a
// cookies.txt from a signed-in browser and the Supermix's share link.
// The server keeps both; the cookies never come back to the page.
async function openYTAccount() {
  const status = el('p', { className: 'yt-status', role: 'status' });
  const failed = el('p', { className: 'yt-failed', hidden: true });
  const file = el('input', { type: 'file', accept: '.txt,text/plain', className: 'visually-hidden' });
  const upload = el('label', { className: 'btn chrome' }, file, 'Upload cookies.txt');
  const removeCookies = button('btn ghost', () => send('DELETE', 'api/yt/account/cookies'), 'Remove');
  const link = el('input', {
    type: 'url',
    placeholder: 'https://music.youtube.com/playlist?list=…',
    'aria-label': 'The Supermix share link',
  });
  const check = button('btn ghost', runCheck, 'Test');
  let account = null;

  const show_ = (a) => {
    account = a;
    const when = a.cookiesAt ? ` on ${new Date(a.cookiesAt).toLocaleDateString()}` : '';
    status.textContent = a.ready
      ? `Connected: cookies ${a.cookies === 'saved' ? `uploaded${when}` : 'from the server’s .env'}, Supermix ${a.supermix}.`
      : !a.cookies && !a.supermix
        ? 'Not connected yet.'
        : !a.cookies
          ? 'The Supermix link is set; the cookies are still needed.'
          : 'The cookies are set; the Supermix link is still needed.';
    failed.hidden = !a.failed;
    failed.textContent = a.failed ? `YouTube refused the account last time: ${a.failed}` : '';
    removeCookies.hidden = a.cookies !== 'saved';
    if (document.activeElement !== link) link.value = a.supermixFrom === 'saved' ? a.supermix : '';
    link.placeholder = a.supermixFrom === 'env' ? `${a.supermix} (from .env)` : 'https://music.youtube.com/playlist?list=…';
    check.disabled = !a.ready;
    if (supermixReady !== a.ready) {
      supermixReady = a.ready;
      if (route().kind === 'home') render();
    }
  };

  async function send(method, url, body, headers) {
    const r = await fetch(url, { method, body, headers }).catch(() => null);
    if (!r || !r.ok) {
      notice(r ? (await r.text()).trim() : 'The server could not be reached');
      return false;
    }
    mix = null; // the account changed; the next Play asks YouTube again
    show_(await r.json());
    return true;
  }

  async function runCheck() {
    check.disabled = true;
    check.textContent = 'Testing…';
    const r = await fetch('api/yt/account/check', { method: 'POST' }).catch(() => null);
    const out = r && r.ok ? await r.json() : { error: 'the server could not be reached' };
    check.textContent = 'Test';
    check.disabled = !account?.ready;
    failed.hidden = out.ok;
    failed.textContent = out.ok ? '' : `YouTube refused the account: ${out.error}`;
    if (out.ok) notice(`Signed in: the Supermix has ${plural(out.tracks, 'track')} ready`);
  }

  file.onchange = async () => {
    const f = file.files[0];
    if (!f) return;
    if (await send('PUT', 'api/yt/account/cookies', await f.text(), { 'Content-Type': 'text/plain' })) {
      notice('Cookies saved');
    }
    file.value = '';
  };

  const form = el(
    'form',
    { method: 'dialog', className: 'editor yt-account' },
    el('h2', { textContent: 'YouTube Music' }),
    status,
    failed,
    el('h3', { textContent: '1. Cookies' }),
    el(
      'p',
      { className: 'yt-help' },
      'In a browser signed in to ',
      el('a', { href: 'https://music.youtube.com', target: '_blank', rel: 'noopener', textContent: 'music.youtube.com' }),
      ', export its cookies as a cookies.txt with an extension such as ',
      el('a', {
        href: 'https://github.com/kairi003/Get-cookies.txt-LOCALLY',
        target: '_blank',
        rel: 'noopener',
        textContent: 'Get cookies.txt LOCALLY',
      }),
      ', then choose the file here. Exported from a private window you then close, they last longer; when YouTube stops taking them, export again.'
    ),
    el('div', { className: 'btn-row' }, upload, removeCookies),
    el('h3', { textContent: '2. The Supermix' }),
    el(
      'p',
      { className: 'yt-help' },
      'Open your Supermix on YouTube Music, then ⋮ → Share → Copy link, and paste it here.'
    ),
    el('div', { className: 'yt-link' }, link, el('button', { className: 'btn ghost', type: 'submit' }, 'Save')),
    el('div', { className: 'btn-row' }, check, button('btn ghost', () => dialog.close(), 'Done'))
  );
  const dialog = el('dialog', { className: 'sheet' }, form);
  form.onsubmit = async (e) => {
    e.preventDefault();
    if (
      await send('PUT', 'api/yt/account/supermix', JSON.stringify({ supermix: link.value }), {
        'Content-Type': 'application/json',
      })
    ) {
      link.blur();
      notice(link.value ? 'Supermix saved' : 'Supermix cleared');
    }
  };
  dialog.addEventListener('close', () => dialog.remove());
  document.body.append(dialog);
  dialog.showModal();

  const r = await fetch('api/yt/account').catch(() => null);
  if (r && r.ok) show_(await r.json());
  else status.textContent = 'The server could not be reached.';
}

// The library mix plays what is owned, a new one each day; the phone
// keeps it saved.
function libraryMixHero() {
  if (libMix === null) loadLibraryMix();
  const ready = Array.isArray(libMix) && libMix.length > 0;
  return el(
    'section',
    { className: 'hero', 'aria-labelledby': 'libmix-title' },
    mixArt(ready ? libMix.map((t) => [t.album, coverUrl(t.artist, t.album)]) : []),
    el(
      'div',
      {},
      el('p', { className: 'kicker', textContent: 'Made from your library' }),
      el('h2', { id: 'libmix-title', textContent: 'Library mix' }),
      libMix === 'failed' && el('p', { textContent: 'The mix could not be read.' }),
      el(
        'div',
        { className: 'btn-row' },
        el(
          'button',
          { className: 'btn chrome', type: 'button', disabled: !ready, onclick: () => playQueue(libItems(libMix), 0) },
          icon('play'),
          'Play'
        ),
        sizePicker(mixSize, (n) => {
          mixSize = n;
          keep('music-mix-size', n);
          libMix = null;
          render();
        })
      )
    )
  );
}

// sizePicker chooses how many tracks a mix holds, from what the server
// makes.
function sizePicker(current, onchange) {
  return el(
    'select',
    { className: 'mix-size', 'aria-label': 'Tracks in the mix', onchange: (e) => onchange(Number(e.target.value)) },
    [50, 100, 200, 500].map((n) => el('option', { value: n, selected: n === current, textContent: `${n} tracks` }))
  );
}

async function loadLibraryMix() {
  libMix = 'loading';
  try {
    const r = await fetch(`api/mix/library?size=${mixSize}`);
    if (!r.ok) throw new Error(r.status);
    libMix = await r.json();
  } catch {
    libMix = 'failed';
  }
  if (route().kind === 'home') render();
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

// buildMix plays the supermix, reading it first if it isn't here yet;
// fresh asks the server for a new one.
async function buildMix(fresh) {
  if (Array.isArray(mix) && !fresh) {
    if (mix.length) playStreams(mix, 0);
    return;
  }
  mix = 'loading';
  render();
  try {
    const r = await fetch(`api/yt/supermix${fresh ? '?fresh=1' : ''}`);
    if (!r.ok) throw new Error((await r.text()).trim() || r.status);
    mixSource = r.headers.get('X-Mix-Source') || '';
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
    ['albums', 'artists', 'playlists'].map((t) =>
      el('a', {
        className: 'chip',
        href: t === 'albums' ? '#/library' : `#/library/${t}`,
        textContent: t[0].toUpperCase() + t.slice(1),
        'aria-current': t === tab ? 'page' : 'false',
      })
    )
  );
  const pending = libraryPending();
  let body;
  if (tab === 'playlists') {
    body = playlistGrid();
  } else if (pending) {
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
  const stats = tab !== 'playlists' && !pending && el('p', { className: 'library-stats', textContent: libraryStats() });
  show(el('h2', { className: 'view-title', textContent: 'Library' }), chips, body, stats);
}

// albumTile links to an album; hovering its cover shows a button that
// plays it straight away, as YouTube Music's do.
function albumTile(artist, album) {
  return el(
    'div',
    { className: 'tile-box' },
    el(
      'a',
      { className: 'tile', href: href('album', artist.name, album.name) },
      art(album.name, coverUrl(artist.name, album.name)),
      el('span', { className: 'tile-title', textContent: album.name }),
      el('span', { className: 'tile-sub', textContent: artist.name })
    ),
    el(
      'div',
      { className: 'art-play' },
      el(
        'button',
        {
          type: 'button',
          'aria-label': `Play ${album.name}`,
          onclick: () => playQueue(ownedItems(artist, album), 0),
        },
        icon('play')
      )
    )
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
  return libItems(album.tracks.map((t) => ({ ...t, artist: artist.name, album: album.name })));
}

// libItems are tracks from anywhere in the library, as playlists and the
// library mix list them, as the queue holds them.
function libItems(tracks) {
  return tracks.map((t) => ({
    key: t.path,
    path: t.path,
    name: trackName(t.name),
    sub: `${t.artist} · ${t.album}`,
    size: t.size,
    url: `api/audio?path=${encodeURIComponent(t.path)}`,
    art: coverUrl(t.artist, t.album),
    album: { artist: t.artist, album: t.album },
  }));
}

// ownedList lists owned tracks with a menu each: numbered on an album,
// with their covers elsewhere.
function ownedList(items, opts = {}) {
  if (!items.length) return el('p', { className: 'empty', textContent: opts.empty || 'Nothing here yet.' });
  return el(
    'ol',
    { className: 'tracks' },
    items.map((t, i) =>
      rowOf(
        t,
        () => playQueue(items, i),
        opts.numbered ? el('span', { className: 'num', textContent: String(i + 1) }) : art(t.album.album, t.art),
        el(
          'span',
          {},
          el('span', { className: 'name', textContent: t.name }),
          !opts.numbered && el('span', { className: 'sub', textContent: t.sub })
        ),
        el(
          'span',
          { className: 'end' },
          el('span', { className: 'no-mix', title: 'Excluded from mixing', textContent: 'No mix' }),
          opts.numbered && t.size && el('span', { className: 'size', textContent: mb(t.size) }),
          menuButton(t, opts.playlist)
        ),
        opts.numbered ? '' : 'stream'
      )
    )
  );
}

// rowOf is one track's row. It can't be a button, as it holds the menu
// and buy buttons, so it takes a button's keys.
function rowOf(item, play, lead, body, end, extra) {
  return el(
    'li',
    {},
    el(
      'div',
      {
        className: `track ${extra || ''}`.trim(),
        role: 'button',
        tabIndex: 0,
        dataset: { key: item.key },
        onclick: play,
        onkeydown: (e) => {
          if (e.target === e.currentTarget && (e.key === 'Enter' || e.key === ' ')) {
            e.preventDefault();
            play();
          }
        },
      },
      lead,
      body,
      end
    )
  );
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
    ownedList(items, { numbered: true })
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

// streamList lists streams with their stills, a buy button and a menu
// each.
function streamList(tracks) {
  if (!tracks.length) return el('p', { className: 'empty', textContent: 'Nothing found.' });
  const items = streamItems(tracks);
  return el(
    'ol',
    { className: 'tracks' },
    tracks.map((t, i) =>
      rowOf(
        items[i],
        () => playQueue(items, i),
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
          el('span', { className: 'no-mix', title: 'Excluded from mixing', textContent: 'No mix' }),
          el('span', { textContent: t.duration ? fmtTime(t.duration) : '' }),
          el('button', {
            className: 'buy',
            type: 'button',
            title: 'Buy this track into the owned library',
            dataset: { buy: buyKey(t.artist, t.title) },
            onclick: (e) => buyTrack(e, t),
            textContent: 'Buy',
          }),
          menuButton(items[i])
        ),
        'stream'
      )
    )
  );
}

// ------------------------------------------------------------ playlists

// Playlists live on the server, so the phone and the page share them. A
// smart one's tracks are whatever meets its rules when it is read.
function loadPlaylists() {
  return fetch('api/playlists')
    .then((r) => (r.ok ? r.json() : Promise.reject(new Error(r.status))))
    .then((list) => {
      playlists = list;
    })
    .catch(() => {
      playlists = playlists || 'failed';
    })
    .finally(() => {
      const r = route();
      if (r.kind === 'library' && r.tab === 'playlists') render();
    });
}

function playlistGrid() {
  const head = el(
    'div',
    { className: 'btn-row' },
    button('btn chrome', () => editPlaylist(null, false), icon('plus'), 'New playlist'),
    button('btn', () => editPlaylist(null, true), icon('smart'), 'New smart playlist')
  );
  const empty = (text) => [head, el('p', { className: 'empty', textContent: text })];
  if (playlists === null) return empty('Reading the playlists…');
  if (playlists === 'failed') return empty('The playlists could not be read.');
  if (!playlists.length) return empty('No playlists yet. Make one, or let a smart one fill itself.');
  return [
    head,
    el(
      'ul',
      { className: 'grid' },
      playlists.map((p) =>
        el(
          'li',
          {},
          el(
            'a',
            { className: 'tile', href: href('playlist', p.id) },
            playlistArt(p),
            el('span', { className: 'tile-title', textContent: p.name }),
            el('span', {
              className: 'tile-sub',
              textContent: `${p.smart ? 'Smart · ' : ''}${plural(p.count, 'track')}`,
            })
          )
        )
      )
    ),
  ];
}

function playlistArt(p, opts = {}) {
  const box = art(p.name, '', opts);
  box.classList.add('playlist-art');
  box.append(icon(p.smart ? 'smart' : 'list'));
  return box;
}

function renderPlaylist(id) {
  const cached = playlistTracks.get(id);
  if (cached) showPlaylist(cached);
  else show(el('p', { className: 'empty', textContent: 'Reading the playlist…' }));
  fetch(`api/playlists/${encodeURIComponent(id)}`)
    .then((r) => {
      if (r.status === 404) return null;
      if (!r.ok) throw new Error(r.status);
      return r.json();
    })
    .then((p) => {
      const r = route();
      if (r.kind !== 'playlist' || r.id !== id) return;
      if (!p) {
        show(el('p', { className: 'empty', textContent: 'No such playlist.' }));
        return;
      }
      playlistTracks.set(id, p);
      showPlaylist(p);
      markTrack();
      markExcluded();
    })
    .catch(() => {
      if (!cached) show(el('p', { className: 'empty', textContent: 'The playlist could not be read.' }));
    });
}

function showPlaylist(p) {
  const items = libItems(p.tracks || []);
  show(
    el(
      'header',
      { className: 'album-head' },
      playlistArt(p, { eager: true }),
      el(
        'div',
        {},
        el('p', { className: 'kicker', textContent: p.smart ? 'Smart playlist' : 'Playlist' }),
        el('h2', { textContent: p.name }),
        el('p', {
          className: 'meta',
          textContent: plural(items.length, 'track') + (p.smart ? ` · ${describeRules(p.smart)}` : ''),
        }),
        el(
          'div',
          { className: 'btn-row' },
          items.length > 0 && button('btn chrome', () => playQueue(items, 0), icon('play'), 'Play'),
          items.length > 0 && button('btn', () => playQueue(shuffled(items), 0), icon('shuffle'), 'Shuffle'),
          button('btn ghost', () => editPlaylist(p, !!p.smart), 'Edit'),
          button('btn ghost', () => deletePlaylist(p), 'Delete')
        )
      )
    ),
    ownedList(items, {
      playlist: p.smart ? null : p,
      empty: p.smart ? 'Nothing in the library meets these rules yet.' : 'Add tracks from their menus (⋯).',
    })
  );
}

async function deletePlaylist(p) {
  if (!confirm(`Delete the playlist “${p.name}”?`)) return;
  const r = await fetch(`api/playlists/${encodeURIComponent(p.id)}`, { method: 'DELETE' }).catch(() => null);
  if (!r || !r.ok) {
    notice('The playlist could not be deleted');
    return;
  }
  playlistTracks.delete(p.id);
  await loadPlaylists();
  location.hash = '#/library/playlists';
}

// The smart rules, by field: the comparisons each takes and what its
// value is.
const TEXT_OPS = { contains: 'contains', is: 'is', not: 'does not contain', starts: 'starts with' };
const RULE_FIELDS = {
  artist: { label: 'Artist', ops: TEXT_OPS },
  album: { label: 'Album', ops: TEXT_OPS },
  title: { label: 'Title', ops: TEXT_OPS },
  plays: { label: 'Plays', ops: { '>': 'more than', '<': 'fewer than', '=': 'exactly' }, number: true },
  added: { label: 'Added', ops: { within: 'in the last … days', before: 'more than … days ago' }, number: true },
  played: {
    label: 'Last played',
    ops: { within: 'in the last … days', before: 'not in the last … days' },
    number: true,
  },
};
const SORTS = {
  artist: 'Artist',
  added: 'Recently added',
  plays: 'Most played',
  played: 'Recently played',
  random: 'Shuffled daily',
};

function describeRules(sm) {
  const parts = (sm.rules || []).map((r) => {
    const f = RULE_FIELDS[r.field];
    if (!f) return '';
    const op = f.ops[r.op] || r.op;
    return `${f.label.toLowerCase()} ${op.includes('…') ? op.replace('…', r.value) : `${op} ${r.value}`}`;
  });
  const joined = parts.length ? parts.join(sm.match === 'any' ? ' or ' : ' and ') : 'everything';
  return `${joined}${sm.limit ? `, up to ${sm.limit}` : ''}`;
}

// editPlaylist opens the editor: a name, and for a smart playlist its
// rules, order and size. p is null for a new one.
function editPlaylist(p, isSmart) {
  const blank = () => ({ field: 'artist', op: 'contains', value: '' });
  const sm = p?.smart || { match: 'all', rules: [blank()], sort: 'artist', limit: 0 };
  const name = el('input', { type: 'text', value: p?.name || '', placeholder: 'Name', required: true });
  name.setAttribute('aria-label', 'Name');
  const rules = el('div', { className: 'rules' });
  const match = select({ all: 'all', any: 'any' }, sm.match || 'all', 'Match');
  const sort = select(SORTS, sm.sort || 'artist', 'Order');
  const limit = el('input', { type: 'number', min: 0, max: 5000, value: sm.limit || 0, title: '0 for no limit' });

  const addRule = (r) => {
    const labels = Object.fromEntries(Object.entries(RULE_FIELDS).map(([k, f]) => [k, f.label]));
    const field = select(labels, r.field, 'Field');
    const op = el('select', { 'aria-label': 'Comparison' });
    const value = el('input', { type: 'text', value: r.value ?? '', 'aria-label': 'Value' });
    const fill = (keep) => {
      const f = RULE_FIELDS[field.value];
      op.replaceChildren(...Object.entries(f.ops).map(([k, label]) => el('option', { value: k, textContent: label })));
      if (keep && f.ops[keep]) op.value = keep;
      value.type = f.number ? 'number' : 'text';
      value.min = f.number ? '0' : '';
    };
    fill(r.op);
    field.onchange = () => fill(null);
    const row = el(
      'div',
      { className: 'rule' },
      field,
      op,
      value,
      el(
        'button',
        { className: 'icon-btn', type: 'button', 'aria-label': 'Remove the rule', onclick: () => row.remove() },
        icon('close')
      )
    );
    row.read = () => ({ field: field.value, op: op.value, value: value.value.trim() });
    rules.append(row);
  };
  if (isSmart) for (const r of sm.rules || []) addRule(r);

  const form = el(
    'form',
    { method: 'dialog', className: 'editor' },
    el('h2', { textContent: p ? `Edit ${p.name}` : isSmart ? 'New smart playlist' : 'New playlist' }),
    name,
    isSmart && [
      el('p', { className: 'rule-match' }, 'Tracks that meet ', match, ' of these:'),
      rules,
      button('btn ghost', () => addRule(blank()), icon('plus'), 'Add a rule'),
      el('div', { className: 'rule-match' }, el('label', {}, 'Order ', sort), el('label', {}, 'Limit ', limit)),
    ],
    el(
      'div',
      { className: 'btn-row' },
      el('button', { className: 'btn chrome', type: 'submit', value: 'save' }, 'Save'),
      button('btn ghost', () => dialog.close(), 'Cancel')
    )
  );
  const dialog = el('dialog', { className: 'sheet' }, form);
  form.onsubmit = async (e) => {
    e.preventDefault();
    const body = { name: name.value.trim() };
    if (isSmart) {
      body.smart = {
        match: match.value,
        rules: [...rules.children].map((row) => row.read()),
        sort: sort.value,
        limit: Number(limit.value) || 0,
      };
    } else {
      body.paths = p?.paths || [];
    }
    const r = await fetch(p ? `api/playlists/${encodeURIComponent(p.id)}` : 'api/playlists', {
      method: p ? 'PUT' : 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    }).catch(() => null);
    if (!r || !r.ok) {
      notice(`The playlist could not be saved${r ? `: ${(await r.text()).trim()}` : ''}`);
      return;
    }
    const saved = await r.json();
    playlistTracks.set(saved.id, saved);
    dialog.close();
    await loadPlaylists();
    if (location.hash === href('playlist', saved.id)) render();
    else location.hash = href('playlist', saved.id);
  };
  dialog.addEventListener('close', () => dialog.remove());
  document.body.append(dialog);
  dialog.showModal();
}

function select(options, value, label) {
  const options_ = Object.entries(options).map(([k, v]) => el('option', { value: k, textContent: v }));
  const s = el('select', { 'aria-label': label }, options_);
  s.value = value;
  return s;
}

function button(className, onclick, ...children) {
  return el('button', { className, type: 'button', onclick }, ...children);
}

// addToPlaylist appends tracks to a plain playlist, or to a new one.
async function addToPlaylist(p, paths) {
  if (!p) {
    const name = prompt('Name the new playlist');
    if (!name || !name.trim()) return;
    const r = await fetch('api/playlists', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name: name.trim(), paths }),
    }).catch(() => null);
    if (!r || !r.ok) return notice('The playlist could not be made');
    notice(`Added to ${name.trim()}`);
  } else {
    const r = await fetch(`api/playlists/${encodeURIComponent(p.id)}/add`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ paths }),
    }).catch(() => null);
    if (!r || !r.ok) return notice('The track could not be added');
    playlistTracks.delete(p.id);
    notice(`Added to ${p.name}`);
  }
  loadPlaylists();
}

async function removeFromPlaylist(p, path) {
  const body = { name: p.name, paths: (p.paths || []).filter((x) => x !== path) };
  const r = await fetch(`api/playlists/${encodeURIComponent(p.id)}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  }).catch(() => null);
  if (!r || !r.ok) return notice('The track could not be removed');
  playlistTracks.set(p.id, await r.json());
  loadPlaylists();
  render();
}

// ---------------------------------------------------------------- menus

// menuButton opens a track's menu: add to a playlist, keep it out of the
// mixes, and take it out of the playlist it is listed in.
function menuButton(item, inPlaylist) {
  return el(
    'button',
    {
      className: 'icon-btn more',
      type: 'button',
      'aria-label': `More for ${item.name}`,
      'aria-haspopup': 'menu',
      onclick: (e) => {
        e.stopPropagation();
        openMenu(e.currentTarget, item, inPlaylist);
      },
    },
    icon('more')
  );
}

let openedMenu = null;

function closeMenu() {
  if (openedMenu) openedMenu.remove();
  openedMenu = null;
}

document.addEventListener('click', (e) => {
  if (openedMenu && !openedMenu.contains(e.target)) closeMenu();
});
document.addEventListener('keydown', (e) => {
  if (e.key === 'Escape') closeMenu();
});
window.addEventListener('hashchange', closeMenu);

function openMenu(anchor, item, inPlaylist) {
  closeMenu();
  const owned = !item.key.startsWith('yt:');
  const out = excluded.has(item.key);
  const entry = (label, fn) =>
    el('button', {
      type: 'button',
      role: 'menuitem',
      textContent: label,
      onclick: (e) => {
        e.stopPropagation();
        closeMenu();
        fn();
      },
    });
  const lists = Array.isArray(playlists) ? playlists.filter((p) => !p.smart) : [];
  const menu = el(
    'div',
    { className: 'menu', role: 'menu' },
    owned && el('p', { className: 'menu-head', textContent: 'Add to playlist' }),
    owned && lists.map((p) => entry(p.name, () => addToPlaylist(p, [item.path]))),
    owned && entry('New playlist…', () => addToPlaylist(null, [item.path])),
    owned && el('hr'),
    inPlaylist && entry(`Remove from ${inPlaylist.name}`, () => removeFromPlaylist(inPlaylist, item.path)),
    entry(out ? 'Include in mixing' : 'Exclude from mixing', () => setExcluded(item.key, !out)),
    item.album && entry('Go to album', () => (location.hash = href('album', item.album.artist, item.album.album)))
  );
  document.body.append(menu);
  const r = anchor.getBoundingClientRect();
  const top = Math.min(r.bottom + 4, window.innerHeight - menu.offsetHeight - 8);
  menu.style.top = `${Math.max(8, top)}px`;
  menu.style.left = `${Math.max(8, Math.min(r.right - menu.offsetWidth, window.innerWidth - menu.offsetWidth - 8))}px`;
  openedMenu = menu;
  menu.querySelector('button')?.focus();
}

// setExcluded keeps a track out of the mixes, or lets it back in. The
// phone stops keeping a library-mix track once it is out.
async function setExcluded(key, out) {
  const r = await fetch('api/excluded', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ key, excluded: out }),
  }).catch(() => null);
  if (!r || !r.ok) return notice('That could not be saved');
  if (out) excluded.add(key);
  else excluded.delete(key);
  markExcluded();
  if (out && Array.isArray(libMix) && libMix.some((t) => t.path === key)) {
    libMix = libMix.filter((t) => t.path !== key);
    if (route().kind === 'home') render();
  }
  if (out && Array.isArray(mix) && key.startsWith('yt:')) {
    mix = mix.filter((t) => `yt:${t.videoId}` !== key);
    if (route().kind === 'home') render();
  }
  notice(out ? 'Excluded from mixing' : 'Included in mixing again');
}

function markExcluded() {
  for (const row of view.querySelectorAll('.track[data-key]')) {
    row.classList.toggle('excluded', excluded.has(row.dataset.key));
  }
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
  if (items !== queue) {
    queue = items;
    arrange(index);
  }
  playingIndex = index;
  counted = false;
  const t = items[index];
  player.src = t.url;
  player.play().catch(() => {});
  if (t.album) rememberAlbum(t.album);

  bar.hidden = false;
  document.body.classList.add('playing-bar');
  $('np-title').textContent = t.name;
  $('np-sub').textContent = t.sub;
  $('np-art').replaceWith(Object.assign(art(t.name, t.art, { eager: true }), { id: 'np-art' }));
  backdrop(backdropFor(route()));
  updateTime();
  markTrack();
  renderUpNext();

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
  let next = order.indexOf(playingIndex) + delta;
  if (repeat === 'all') next = (next + queue.length) % queue.length;
  if (next >= 0 && next < queue.length) playQueue(queue, order[next]);
}

player.addEventListener('ended', () => {
  if (repeat === 'one') {
    player.currentTime = 0;
    player.play().catch(() => {});
  } else step(1);
});

// ------------------------------------------------------ shuffle, repeat

let shuffle = remember('music-shuffle', false);
let repeat = remember('music-repeat', 'off'); // 'off', 'all' or 'one'

// arrange sets the order the queue plays in: as listed, or the track
// playing first and the rest shuffled.
function arrange(start) {
  order = queue.map((_, i) => i);
  if (!shuffle) return;
  const rest = order.filter((i) => i !== start);
  for (let i = rest.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1));
    [rest[i], rest[j]] = [rest[j], rest[i]];
  }
  order = start >= 0 ? [start, ...rest] : rest;
}

function syncModes() {
  const sh = $('shuffle');
  sh.setAttribute('aria-pressed', String(shuffle));
  sh.title = shuffle ? 'Shuffle is on' : 'Shuffle';
  const re = $('repeat');
  re.setAttribute('aria-pressed', String(repeat !== 'off'));
  re.title = { off: 'Repeat', all: 'Repeating the queue', one: 'Repeating this track' }[repeat];
  setIcon(re, repeat === 'one' ? 'repeat-one' : 'repeat');
}

$('shuffle').addEventListener('click', () => {
  shuffle = !shuffle;
  keep('music-shuffle', shuffle);
  arrange(playingIndex);
  syncModes();
  renderUpNext();
});

$('repeat').addEventListener('click', () => {
  repeat = { off: 'all', all: 'one', one: 'off' }[repeat];
  keep('music-repeat', repeat);
  syncModes();
  renderUpNext();
});

syncModes();

// -------------------------------------------------------------- up next

const upNext = $('up-next');
const upNextShown = 100; // rows listed; a long shuffle says how many more

// toggleUpNext shows or hides what plays after this track.
function toggleUpNext(open = upNext.hidden) {
  upNext.hidden = !open;
  $('queue').setAttribute('aria-expanded', String(open));
  renderUpNext();
}

// renderUpNext lists the track playing and then the queue in the order
// it will play: shuffled or not, and wrapping round when repeating it.
function renderUpNext() {
  if (upNext.hidden) return;
  const at = order.indexOf(playingIndex);
  let next = order.slice(at + 1);
  if (repeat === 'all') next = next.concat(order.slice(0, at));
  const row = (i) =>
    rowOf(
      queue[i],
      () => playQueue(queue, i),
      art(queue[i].name, queue[i].art),
      el(
        'span',
        {},
        el('span', { className: 'name', textContent: queue[i].name }),
        el('span', { className: 'sub', textContent: queue[i].sub })
      )
    );
  const more = next.length - upNextShown;
  upNext.replaceChildren(
    el('p', { className: 'kicker', textContent: 'Now playing' }),
    el('ol', { className: 'tracks' }, queue[playingIndex] && row(playingIndex)),
    el('p', { className: 'kicker', textContent: repeat === 'one' ? 'Up next, after this repeats' : 'Up next' }),
    next.length
      ? el('ol', { className: 'tracks' }, next.slice(0, upNextShown).map(row))
      : el('p', { className: 'empty', textContent: 'Nothing after this.' }),
    more > 0 && el('p', { className: 'empty', textContent: `And ${more.toLocaleString()} more.` })
  );
  markTrack();
}

$('queue').addEventListener('click', (e) => {
  e.stopPropagation();
  toggleUpNext();
});
// A click elsewhere closes it; the path, since playing a row redraws it.
document.addEventListener('click', (e) => {
  const path = e.composedPath();
  if (!upNext.hidden && !path.includes(upNext) && !path.includes(bar)) toggleUpNext(false);
});
document.addEventListener('keydown', (e) => {
  if (e.key === 'Escape' && !upNext.hidden) toggleUpNext(false);
});
window.addEventListener('hashchange', () => toggleUpNext(false));

// ------------------------------------------------------------ backdrop

// backdropFor is the cover whose colour washes the top of the page: the
// album's or artist's in view, or else what is playing.
function backdropFor(r) {
  if (r.kind === 'album') return coverUrl(r.artist, r.album);
  if (r.kind === 'artist') {
    const a = findArtist(r.artist);
    if (a) return artistCover(a);
  }
  const t = queue[playingIndex];
  return t ? t.art : '';
}

const glows = new Map(); // cover → its colour, or '' when it has none to give
let glowFor = null;

// backdrop sets the wash to a cover's colour: its pixels averaged, the
// more colourful ones counting most, then made vivid enough to show.
function backdrop(src) {
  glowFor = src;
  const set = (c) => {
    if (glowFor !== src) return;
    if (c) document.documentElement.style.setProperty('--glow', c);
    else document.documentElement.style.removeProperty('--glow');
  };
  if (!src) return set('');
  if (glows.has(src)) return set(glows.get(src));
  const img = new Image();
  img.crossOrigin = 'anonymous';
  img.onload = () => {
    let c = '';
    try {
      const canvas = Object.assign(document.createElement('canvas'), { width: 24, height: 24 });
      const g = canvas.getContext('2d', { willReadFrequently: true });
      g.drawImage(img, 0, 0, 24, 24);
      const px = g.getImageData(0, 0, 24, 24).data;
      let r = 0, gr = 0, b = 0, w = 0;
      for (let i = 0; i < px.length; i += 4) {
        const max = Math.max(px[i], px[i + 1], px[i + 2]);
        const min = Math.min(px[i], px[i + 1], px[i + 2]);
        const weight = 0.05 + (max - min) / 255;
        r += px[i] * weight;
        gr += px[i + 1] * weight;
        b += px[i + 2] * weight;
        w += weight;
      }
      c = vivid(r / w, gr / w, b / w);
    } catch {
      // A cover from elsewhere that won't be read; the default stays.
    }
    glows.set(src, c);
    set(c);
  };
  img.onerror = () => {
    glows.set(src, '');
    set('');
  };
  img.src = src;
}

// vivid lifts a colour's saturation and sets its lightness into a band
// that shows against the page without glaring.
function vivid(r, g, b) {
  r /= 255;
  g /= 255;
  b /= 255;
  const max = Math.max(r, g, b);
  const min = Math.min(r, g, b);
  const l = (max + min) / 2;
  const d = max - min;
  let h = 0;
  if (d) {
    if (max === r) h = ((g - b) / d) % 6;
    else if (max === g) h = (b - r) / d + 2;
    else h = (r - g) / d + 4;
  }
  const s = d ? d / (1 - Math.abs(2 * l - 1)) : 0;
  return `hsl(${Math.round(((h * 60) + 360) % 360)} ${Math.round(Math.min(1, s * 1.7) * 100)}% ${Math.round(Math.min(0.55, Math.max(0.4, l)) * 100)}%)`;
}

// An owned track counts as played once half of it, or half a minute, has
// played; smart playlists and the library mix go by the counts.
let counted = false;
player.addEventListener('timeupdate', () => {
  const t = queue[playingIndex];
  if (counted || !t || !t.path) return;
  const d = player.duration;
  if (player.currentTime < Math.min(30, Number.isFinite(d) ? d / 2 : 30)) return;
  counted = true;
  fetch('api/played', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ path: t.path }),
  }).catch(() => {});
});

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
