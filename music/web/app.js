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
let mix = null; // the supermix last built, kept for its page
let mixSource = ''; // account (YouTube Music's own) or radio (seeded from the library)
let libMix = null; // the library mix: null until asked for, then 'loading', a list or 'failed'
let libRemix = 0; // which of the day's library mixes: 0 is the day's own, others Remix's
let playlists = null; // from api/playlists, without their tracks
const playlistTracks = new Map(); // id → the playlist with its tracks, as last read
let excluded = new Set(); // what is kept out of the mixes: paths, and yt:<id> for streams
let lastSearch = { q: '', tracks: null }; // the search last run, so back doesn't run it again
let apkReady = false; // whether the Android client can be offered
let mixSize = remember('music-mix-size', 50); // tracks in the library mix
let supermixReady = false; // whether the server has YouTube Music set up for the supermix
let movesSeq = remember('music-moves-seq', 0); // the last of the server's renames applied here

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

applyMoves().finally(loadLibrary);
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
    if (supermixReady && onMixes()) render();
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

// coverSetAt marks the albums whose cover was changed here, so their
// art is asked for afresh rather than taken from the browser's cache.
const coverSetAt = new Map();

const coverUrl = (artist, album) => {
  const v = coverSetAt.get(`${artist}\0${album}`);
  return `api/cover?artist=${encodeURIComponent(artist)}&album=${encodeURIComponent(album)}${v ? `&v=${v}` : ''}`;
};

// An artist's picture is the cover of their first album with one; the
// grid can't know which ahead, so it uses the first.
const artistCover = (a) => (a.albums.length ? coverUrl(a.name, a.albums[0].name) : '');

// The leading track number a file name carries, shown as the row's own.
const trackNumber = /^\d+[\s._-]*/;
const trackName = (name) => name.replace(trackNumber, '') || name;

// ownKey names a track by its artist and title as letters and digits
// alone, so a stream's "TENDER — go steady" finds the library's
// "Tender/…/05 - Go Steady".
const fold = (s) => s.replaceAll('&', 'and').toLowerCase().replace(/[^\p{L}\p{N}]/gu, '');
const ownKey = (artist, title) => `${fold(artist)}\u0000${fold(title)}`;
let ownedKeys = new Set(); // every library track's ownKey

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

// route reads the hash: #/, #/library[/albums|/playlists], #/artist/A,
// #/album/A/B, #/playlist/ID, #/mix/supermix|library, #/buys or #/search/Q.
function route() {
  const parts = location.hash.replace(/^#\/?/, '').split('/').map(decodeURIComponent);
  switch (parts[0]) {
    case 'library':
      return { kind: 'library', tab: ['albums', 'playlists'].includes(parts[1]) ? parts[1] : 'artists' };
    case 'playlist':
      return { kind: 'playlist', id: parts[1] || '' };
    case 'artist':
      return { kind: 'artist', artist: parts[1] || '' };
    case 'album':
      return { kind: 'album', artist: parts[1] || '', album: parts.slice(2).join('/') };
    case 'mix':
      return { kind: 'mix', which: parts[1] === 'library' ? 'library' : 'supermix' };
    case 'search':
      return { kind: 'search', q: parts.slice(1).join('/') };
    case 'buys':
      return { kind: 'buys' };
    case 'store':
      return { kind: 'store', artist: parts[1] || '', album: parts.slice(2).join('/') };
    default:
      return { kind: 'home' };
  }
}

const href = (...parts) => '#/' + parts.map(encodeURIComponent).join('/');

// onMixes is whether the page shows a mix: home's cards or a mix's page.
const onMixes = () => ['home', 'mix'].includes(route().kind);

function render() {
  const r = route();
  if (r.kind === 'buys') $('buys-link').setAttribute('aria-current', 'page');
  else $('buys-link').removeAttribute('aria-current');
  for (const a of document.querySelectorAll('.tabs a')) {
    const inLibrary = ['artist', 'album', 'playlist'].includes(r.kind);
    const on =
      a.dataset.tab === r.kind ||
      (a.dataset.tab === 'library' && inLibrary) ||
      (a.dataset.tab === 'home' && r.kind === 'mix');
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
    case 'mix':
      if (r.which === 'library') renderLibraryMix();
      else renderSupermix();
      break;
    case 'search':
      $('search-q').value = r.q;
      renderSearch(r.q);
      break;
    case 'buys':
      renderBuys();
      break;
    case 'store':
      renderStoreAlbum(r.artist, r.album);
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
      ownedKeys = new Set(
        artists.flatMap((a) => a.albums.flatMap((al) => al.tracks.map((t) => ownKey(a.name, trackName(t.name)))))
      );
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
  if (supermixReady && mix === null) loadSupermix();
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
    libraryPending(),
    recent.length > 0 && shelf('Jump back in', null, albumRow(recent)),
    all.length > 0 && shelf('From your library', '#/library/albums', albumRow(shuffled(all).slice(0, 18)))
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
      el('h2', { id: 'mix-title' }, el('a', { href: href('mix', 'supermix'), textContent: 'Supermix' })),
      el(
        'div',
        { className: 'btn-row' },
        el(
          'button',
          { className: 'btn chrome', type: 'button', disabled: mix === 'loading', onclick: () => openMix('supermix') },
          icon('play'),
          mix === 'loading' ? 'Mixing…' : 'Play'
        ),
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
      if (onMixes()) render();
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
      el('h2', { id: 'libmix-title' }, el('a', { href: href('mix', 'library'), textContent: 'Library mix' })),
      libMix === 'failed' && el('p', { textContent: 'The mix could not be read.' }),
      el(
        'div',
        { className: 'btn-row' },
        el(
          'button',
          { className: 'btn chrome', type: 'button', disabled: !ready, onclick: () => openMix('library') },
          icon('play'),
          'Play'
        ),
        el(
          'button',
          {
            className: 'btn ghost',
            type: 'button',
            disabled: libMix === 'loading',
            onclick: () => {
              location.hash = href('mix', 'library');
              remix();
            },
          },
          icon('mix'),
          'Remix'
        )
      )
    )
  );
}

function mixSizePicker() {
  return sizePicker(mixSize, (n) => {
    mixSize = n;
    keep('music-mix-size', n);
    libMix = null;
    render();
  });
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
    const r = await fetch(`api/mix/library?size=${mixSize}${libRemix ? `&remix=${libRemix}` : ''}`);
    if (!r.ok) throw new Error(r.status);
    libMix = await r.json();
  } catch {
    libMix = 'failed';
  }
  if (onMixes()) render();
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

// openMix plays a mix from the top and opens its page, as YouTube Music
// does; the supermix is read first if it isn't here yet.
async function openMix(which) {
  location.hash = href('mix', which);
  if (which === 'library') {
    if (Array.isArray(libMix) && libMix.length) playQueue(libItems(libMix), 0);
    return;
  }
  if (!Array.isArray(mix)) await loadSupermix();
  if (Array.isArray(mix) && mix.length) playStreams(mix, 0);
}

// loadSupermix reads the supermix.
async function loadSupermix() {
  mix = 'loading';
  render();
  try {
    const r = await fetch('api/yt/supermix');
    if (!r.ok) throw new Error((await r.text()).trim() || r.status);
    mixSource = r.headers.get('X-Mix-Source') || '';
    mix = await r.json();
  } catch (err) {
    mix = 'failed'; // Play asks again; rendering doesn't
    notice(`The mix could not be built: ${err.message}`);
  }
  if (onMixes()) render();
}

// remix draws another library mix for the day and plays it. The supermix
// has none: YouTube Music makes it, and asking again brings the same.
async function remix() {
  libRemix = 1 + Math.floor(Math.random() * 1e9);
  const loaded = loadLibraryMix();
  if (onMixes()) render(); // "Mixing…" while it loads
  await loaded;
  if (Array.isArray(libMix) && libMix.length) playQueue(libItems(libMix), 0);
}

// The mixes' pages: the mix's covers, its buttons, then its tracks.

function mixHead(covers, kicker, title, meta, ...buttons) {
  return el(
    'header',
    { className: 'album-head' },
    mixArt(covers),
    el(
      'div',
      {},
      el('p', { className: 'kicker', textContent: kicker }),
      el('h2', { textContent: title }),
      meta && el('p', { className: 'meta', textContent: meta }),
      el('div', { className: 'btn-row' }, ...buttons)
    )
  );
}

function renderSupermix() {
  if (!supermixReady) {
    show(connectHero());
    return;
  }
  if (mix === null) loadSupermix();
  const ready = Array.isArray(mix) && mix.length > 0;
  show(
    mixHead(
      ready ? mix.map((t) => [t.title, t.thumb]) : [],
      mixSource === 'radio' ? 'Radio from your library' : 'From YouTube Music',
      'Supermix',
      ready && plural(mix.length, 'track'),
      ready && button('btn chrome', () => playStreams(mix, 0), icon('play'), 'Play'),
      ready && button('btn', () => playStreams(shuffled(mix), 0), icon('shuffle'), 'Shuffle'),
      el(
        'button',
        {
          className: 'icon-btn',
          type: 'button',
          title: 'The YouTube Music account',
          'aria-label': 'The YouTube Music account',
          onclick: openYTAccount,
        },
        icon('more')
      )
    ),
    mix === 'loading'
      ? el('p', { className: 'empty', textContent: 'Mixing…' })
      : mix === 'failed'
        ? el('p', { className: 'empty' }, "The mix didn't load. ", button('btn', loadSupermix, 'Try again'))
        : Array.isArray(mix) && streamList(mix)
  );
}

function renderLibraryMix() {
  if (libMix === null) loadLibraryMix();
  const ready = Array.isArray(libMix) && libMix.length > 0;
  const items = ready ? libItems(libMix) : [];
  show(
    mixHead(
      ready ? libMix.map((t) => [t.album, coverUrl(t.artist, t.album)]) : [],
      'Made from your library',
      'Library mix',
      ready && `${plural(items.length, 'track')} · ${libRemix ? 'remixed' : 'a new one each day'}`,
      ready && button('btn chrome', () => playQueue(items, 0), icon('play'), 'Play'),
      ready && button('btn', () => playQueue(shuffled(items), 0), icon('shuffle'), 'Shuffle'),
      el(
        'button',
        { className: 'btn ghost', type: 'button', disabled: libMix === 'loading', onclick: remix },
        icon('mix'),
        'Remix'
      ),
      mixSizePicker()
    ),
    libMix === 'failed'
      ? el('p', { className: 'empty', textContent: 'The mix could not be read.' })
      : ready
        ? ownedList(items)
        : el('p', { className: 'empty', textContent: 'Mixing…' })
  );
}

// -------------------------------------------------------------- library

function renderLibrary(tab) {
  const chips = el(
    'div',
    { className: 'chips', role: 'group', 'aria-label': 'Show' },
    ['artists', 'albums', 'playlists'].map((t) =>
      el('a', {
        className: 'chip',
        href: t === 'artists' ? '#/library' : `#/library/${t}`,
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
  const play = el(
    'button',
    {
      type: 'button',
      onclick: () => (playingHere(box) ? togglePlay() : playQueue(ownedItems(artist, album), 0)),
    },
    icon('play')
  );
  const box = el(
    'div',
    { className: 'tile-box' },
    el(
      'a',
      { className: 'tile', href: href('album', artist.name, album.name) },
      art(album.name, coverUrl(artist.name, album.name)),
      el('span', { className: 'tile-title', textContent: album.name }),
      el('span', { className: 'tile-sub', textContent: artist.name })
    ),
    el('div', { className: 'art-play' }, play)
  );
  box.paths = new Set(album.tracks.map((t) => t.path));
  box.label = album.name;
  syncTile(box);
  return box;
}

// A tile's button pauses while its album plays, and resumes it rather
// than starting over while it is paused.
function playingHere(box) {
  return box.paths.has(queue[playingIndex]?.key);
}

function syncTile(box) {
  const here = playingHere(box);
  const pause = here && !player.paused;
  const btn = box.querySelector('.art-play button');
  setIcon(btn, pause ? 'pause' : 'play');
  btn.setAttribute('aria-label', `${pause ? 'Pause' : 'Play'} ${box.label}`);
  box.classList.toggle('playing', here);
}

function syncTiles() {
  for (const box of document.querySelectorAll('.tile-box')) if (box.paths) syncTile(box);
}

function renderArtist(name) {
  const a = findArtist(name);
  if (!a) {
    show(libraryPending() || el('p', { className: 'empty', textContent: `No artist “${name}”.` }));
    return;
  }
  const tracks = a.albums.flatMap((al) => ownedItems(a, al));
  // Everything by the artist, as one album to the offline cache.
  const all = { name: a.name, tracks: a.albums.flatMap((al) => al.tracks) };
  const saveBtn = el('button', { className: 'btn', type: 'button', onclick: () => toggleSave(all, saveBtn) });
  refreshSaveButton(all, saveBtn);
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
          ),
          saveBtn,
          button('btn ghost', () => editArtist(a.name), 'Edit')
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
  // The row playing pauses and resumes rather than starting over.
  const go = (row) => (row.classList.contains('current') ? togglePlay() : play());
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
        onclick: (e) => go(e.currentTarget),
        onkeydown: (e) => {
          if (e.target === e.currentTarget && (e.key === 'Enter' || e.key === ' ')) {
            e.preventDefault();
            go(e.currentTarget);
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
          saveBtn,
          album.name !== 'Singles' && button('btn ghost', () => editAlbum(a.name, album.name), 'Edit')
        )
      )
    ),
    ownedList(items, { numbered: true })
  );
}

// openCoverEditor sets an album's cover: one of the covers MusicBrainz
// has for albums like it, a picture uploaded or pasted, or one from a
// link. The server keeps it as the folder's cover.jpg.
function openCoverEditor(artist, album) {
  const status = el('p', { className: 'yt-status', role: 'status' });
  const choices = el('div', { className: 'cover-choices' });
  const query = el('input', { type: 'search', value: album, 'aria-label': 'Album to look for' });
  const file = el('input', { type: 'file', accept: 'image/jpeg,image/png,image/gif', className: 'visually-hidden' });
  const upload = el('label', { className: 'btn' }, file, 'Upload a picture');
  const link = el('input', { type: 'url', placeholder: 'https://…/cover.jpg', 'aria-label': 'A link to a picture' });
  let busy = false;

  async function set(body, headers, what) {
    if (busy) return;
    busy = true;
    status.textContent = `Saving ${what}…`;
    const target = `api/cover?artist=${encodeURIComponent(artist)}&album=${encodeURIComponent(album)}`;
    const r = await fetch(target, { method: 'PUT', body, headers }).catch(() => null);
    busy = false;
    if (!r || !r.ok) {
      status.textContent = r ? (await r.text()).trim() : 'The server could not be reached.';
      return;
    }
    coverSetAt.set(`${artist}\0${album}`, Date.now());
    dialog.close();
    notice('Cover saved');
    render();
  }
  const json = (o) => [JSON.stringify(o), { 'Content-Type': 'application/json' }];

  async function search() {
    const q = query.value.trim();
    if (!q) return;
    status.textContent = 'Looking on MusicBrainz…';
    choices.replaceChildren();
    const r = await fetch(`api/cover/search?artist=${encodeURIComponent(artist)}&album=${encodeURIComponent(q)}`).catch(
      () => null
    );
    if (!r || !r.ok) {
      status.textContent = r ? (await r.text()).trim() : 'The server could not be reached.';
      return;
    }
    const found = await r.json();
    let shown = found.length;
    const none = () => {
      status.textContent = 'No covers found. Try other words, or upload one.';
    };
    if (!shown) none();
    else status.textContent = 'Pick a cover:';
    choices.replaceChildren(
      ...found.map((c) => {
        const img = el('img', {
          alt: '',
          loading: 'lazy',
          src: `https://coverartarchive.org/release-group/${encodeURIComponent(c.id)}/front-250`,
        });
        const about = [c.artist, c.type, c.year].filter(Boolean).join(' · ');
        const pick = button(
          'cover-choice',
          () => set(...json({ releaseGroup: c.id }), `the cover of ${c.title}`),
          img,
          el('span', { className: 'name', textContent: c.title }),
          el('span', { className: 'sub', textContent: about })
        );
        pick.title = `${c.title} · ${about}`;
        // Only the albums the archive has a cover for are offered.
        img.onerror = () => {
          pick.remove();
          if (--shown === 0) none();
        };
        return pick;
      })
    );
  }

  file.onchange = () => {
    const f = file.files[0];
    if (f) set(f, { 'Content-Type': f.type || 'application/octet-stream' }, 'the picture');
    file.value = '';
  };

  const form = el(
    'form',
    { method: 'dialog', className: 'editor yt-account cover-editor' },
    el('h2', { textContent: 'Album cover' }),
    el('p', { className: 'yt-help', textContent: `${album} · ${artist}` }),
    el('div', { className: 'yt-link' }, query, button('btn ghost', search, 'Search')),
    status,
    choices,
    el('h3', { textContent: 'Or your own' }),
    el('p', { className: 'yt-help', textContent: 'Upload a JPEG, PNG or GIF, paste one here, or give a link to one.' }),
    el('div', { className: 'yt-link' }, link, button('btn ghost', () => link.value && set(...json({ url: link.value }), 'the linked picture'), 'Use link')),
    el('div', { className: 'btn-row' }, upload, button('btn ghost', () => dialog.close(), 'Cancel'))
  );
  const dialog = el('dialog', { className: 'sheet' }, form);
  form.onsubmit = (e) => {
    e.preventDefault();
    if (document.activeElement === link) link.value && set(...json({ url: link.value }), 'the linked picture');
    else search();
  };
  // A picture pasted anywhere in the panel is taken as the cover.
  dialog.addEventListener('paste', (e) => {
    const f = [...e.clipboardData.files].find((x) => x.type.startsWith('image/'));
    if (!f) return;
    e.preventDefault();
    set(f, { 'Content-Type': f.type }, 'the pasted picture');
  });
  dialog.addEventListener('close', () => dialog.remove());
  document.body.append(dialog);
  dialog.showModal();
  search();
}

// ------------------------------------------------------------- editing

// The library's names are its folders' and files' names, so an edit is a
// rename on the server. Each is logged, and applyMoves replays the log on
// what this browser keeps by path: saved audio, the queue, recent albums.
async function applyMoves() {
  const r = await fetch(`api/moves?since=${movesSeq}`).catch(() => null);
  if (!r || !r.ok) return;
  const { seq, moves } = await r.json();
  const cache = moves.length && 'caches' in window ? await caches.open(AUDIO).catch(() => null) : null;
  let recent = remember('music-recent', []);
  for (const m of moves) {
    if (m.album) {
      const [from, to] = [splitAlbum(m.from), splitAlbum(m.to)];
      recent = recent.map((x) => (x.artist === from.artist && x.album === from.album ? to : x));
      continue;
    }
    if (cache) {
      const hit = await cache.match(offlineKey(m.from));
      if (hit) {
        await cache.put(offlineKey(m.to), hit);
        await cache.delete(offlineKey(m.from));
      }
    }
    for (const item of queue) {
      if (item.path !== m.from) continue;
      const { artist, album } = splitTrack(m.to);
      Object.assign(item, libItems([{ path: m.to, name: fileStem(m.to), size: item.size, artist, album }])[0]);
    }
  }
  if (moves.length) {
    saveSession();
    keep('music-recent', recent.filter((x, i) => recent.findIndex((y) => y.artist === x.artist && y.album === x.album) === i));
    if (excluded.size) {
      const keys = await fetch('api/excluded').then((r) => (r.ok ? r.json() : null)).catch(() => null);
      if (keys) excluded = new Set(keys);
    }
  }
  movesSeq = seq;
  keep('music-moves-seq', seq);
}

// An artist's name can't hold a slash, so the first one ends it.
const splitAlbum = (s) => ({ artist: s.slice(0, s.indexOf('/')), album: s.slice(s.indexOf('/') + 1) });
const fileStem = (path) => path.slice(path.lastIndexOf('/') + 1).replace(/\.[^.]*$/, '');
function splitTrack(path) {
  const parts = path.split('/');
  return { artist: parts[0], album: parts.length === 3 ? parts[1] : 'Singles' };
}

// openEditor asks for new names and makes the edit. A plain rename goes
// straight through; a merge into what is already there is shown first,
// and two different tracks of one name stop it.
function openEditor({ title, about, fields, request, done, more = [] }) {
  const status = el('p', { className: 'yt-status', role: 'status' });
  const save = el('button', { className: 'btn chrome', type: 'submit', textContent: 'Save' });
  let shown = null; // the request whose plan is on show, waiting to be confirmed

  const inputs = fields.map((f) => {
    const input = el('input', { type: 'text', value: f.value, required: true, spellcheck: false });
    if (f.options) {
      // Filled as it is wanted, since a track's albums follow its artist.
      const list = el('datalist', { id: `edit-${f.name}` });
      input.setAttribute('list', list.id);
      input.addEventListener('focus', () => list.replaceChildren(...f.options().map((o) => el('option', { value: o }))));
      f.list = list;
    }
    input.addEventListener('input', () => {
      shown = null;
      save.textContent = 'Save';
      status.textContent = '';
    });
    f.input = input;
    return el('label', { className: 'edit-field' }, el('span', { textContent: f.label }), input, f.list);
  });
  const values = () => Object.fromEntries(fields.map((f) => [f.name, f.input.value.trim()]));

  async function post(body, dry) {
    const r = await fetch(`api/edit${dry ? '?dry=1' : ''}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    }).catch(() => null);
    if (!r) throw new Error('The server could not be reached.');
    if (!r.ok) throw new Error((await r.text()).trim());
    return r.json();
  }

  async function submit() {
    const v = values();
    const body = request(v);
    if (!body) {
      dialog.close();
      return;
    }
    const key = JSON.stringify(body);
    save.disabled = true;
    try {
      if (shown !== key) {
        const plan = await post(body, true);
        if (plan.conflicts?.length) {
          status.textContent = `Can't: ${plan.conflicts.join(', ')} is already there, a different track of the same name. Rename one first.`;
          return;
        }
        if (plan.merge) {
          const dups = plan.duplicates?.length || 0;
          status.textContent =
            `${plan.to} is already there: ${plural(plan.tracks, 'track')} will join it.` +
            (dups ? ` ${plural(dups, 'track')} it already has will be set aside in the library's .trash folder.` : '');
          shown = key;
          save.textContent = 'Merge';
          return;
        }
      }
      status.textContent = 'Saving…';
      await post(body, false);
      dialog.close();
      await applyMoves();
      playlistTracks.clear();
      libMix = null;
      notice('Renamed');
      await loadLibrary();
      done(v);
    } catch (e) {
      status.textContent = e.message;
    } finally {
      save.disabled = false;
    }
  }

  const form = el(
    'form',
    { className: 'editor yt-account' },
    el('h2', { textContent: title }),
    el('p', { className: 'yt-help', textContent: about }),
    inputs,
    status,
    el(
      'div',
      { className: 'btn-row' },
      save,
      button('btn ghost', () => dialog.close(), 'Cancel'),
      // Other edits, each in its own panel in place of this one.
      more.map(([label, open]) =>
        button(
          'btn ghost',
          () => {
            dialog.close();
            open();
          },
          label
        )
      )
    )
  );
  form.onsubmit = (e) => {
    e.preventDefault();
    submit();
  };
  const dialog = el('dialog', { className: 'sheet' }, form);
  dialog.addEventListener('close', () => dialog.remove());
  document.body.append(dialog);
  dialog.showModal();
  fields[0].input.select();
}

const artistNames = () => library.map((a) => a.name);

function editAlbum(artist, album) {
  openEditor({
    title: 'Edit album',
    about: 'Renames the album’s folder. Under another artist it moves there; onto an album that is already there it joins it.',
    fields: [
      { name: 'album', label: 'Album', value: album },
      { name: 'artist', label: 'Artist', value: artist, options: artistNames },
    ],
    request: (v) =>
      v.album === album && v.artist === artist
        ? null
        : { artist, album, to: v.album, toArtist: v.artist === artist ? '' : v.artist },
    done: (v) => (location.hash = href('album', v.artist, v.album)),
    more: [['Change cover', () => openCoverEditor(artist, album)]],
  });
}

function editArtist(artist) {
  openEditor({
    title: 'Edit artist',
    about: 'Renames the artist’s folder. Onto an artist that is already there, the albums join theirs.',
    fields: [{ name: 'artist', label: 'Artist', value: artist, options: artistNames }],
    request: (v) => (v.artist === artist ? null : { artist, to: v.artist }),
    done: (v) => (location.hash = href('artist', v.artist)),
  });
}

function editTrack(item) {
  const { artist, album } = splitTrack(item.path);
  const fields = [
    { name: 'name', label: 'File name', value: fileStem(item.path) },
    {
      name: 'album',
      label: 'Album',
      value: album,
      options: () => ['Singles', ...(findArtist(fields[2].input.value.trim())?.albums.map((a) => a.name) || [])],
    },
    { name: 'artist', label: 'Artist', value: artist, options: artistNames },
  ];
  openEditor({
    title: 'Edit track',
    about: 'Renames the track’s file; a number at the front orders the album. “Singles” keeps it loose under the artist.',
    fields,
    request: (v) => {
      const moved = v.artist !== artist || v.album !== album;
      if (!moved && v.name === fileStem(item.path)) return null;
      return {
        track: item.path,
        to: v.name,
        toArtist: v.artist === artist ? '' : v.artist,
        toAlbum: moved ? v.album || 'Singles' : '',
      };
    },
    done: () => render(),
  });
}

// --------------------------------------------------------------- search

function renderSearch(q) {
  if (buysReady) searchStore(q);
  if (lastSearch.q === q && lastSearch.tracks) {
    show(searchHead(q), storeShelf(q), streamsHead(q), streamList(lastSearch.tracks));
    return;
  }
  show(searchHead(q), storeShelf(q), el('p', { className: 'empty', textContent: 'Searching…' }));
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
        show(searchHead(q), storeShelf(q), el('p', { className: 'empty', textContent: 'The search could not be read.' }));
      }
    });
}

// streamsHead names the streams, once the store's albums sit above them.
const streamsHead = (q) =>
  storeShelf(q) && el('div', { className: 'shelf-head' }, el('h2', { textContent: 'Streams' }));

function searchHead(q) {
  return el(
    'div',
    { className: 'shelf-head' },
    el('div', {}, el('p', { className: 'kicker', textContent: buysReady ? 'Search' : 'Streams for' }), el('h2', { textContent: q }))
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
            dataset: { buy: buyKey(t.artist, t.title), own: ownKey(t.artist, t.title) },
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
      items.length > 0 ? mixArt(items.map((i) => [i.album.album, i.art])) : playlistArt(p, { eager: true }),
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
  backdrop(backdropFor(route()));
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
    owned && item.path && entry('Edit track…', () => editTrack(item)),
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
    if (onMixes()) render();
  }
  if (out && Array.isArray(mix) && key.startsWith('yt:')) {
    mix = mix.filter((t) => `yt:${t.videoId}` !== key);
    if (onMixes()) render();
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
  btn.title = all ? 'Saved offline — click to remove' : `Keep ${album.name} for when there is no connection`;
}

function savedCount(album) {
  return caches.open(AUDIO).then((cache) =>
    Promise.all(album.tracks.map((t) => cache.match(offlineKey(t.path)))).then((hits) => hits.filter(Boolean).length)
  );
}

// toggleSave pins an album, or all of an artist's, into the cache, track by track, or clears it
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
// and streams alike, each with the key its row is marked by. A restored
// session loads the track paused, at where it was.
function playQueue(items, index, { restore = false, at = 0 } = {}) {
  if (!items.length) return;
  if (items !== queue) {
    queue = items;
    arrange(index);
  }
  playingIndex = index;
  counted = false;
  const t = items[index];
  // The page preloads nothing, but a restored track wants its length and
  // its place in it before anyone presses play.
  player.preload = restore ? 'metadata' : 'none';
  player.src = t.url;
  if (restore) {
    // Only if it is still this track by the time it loads.
    const src = player.src;
    if (at > 0) player.addEventListener('loadedmetadata', () => player.src === src && (player.currentTime = at), { once: true });
  } else {
    playNow();
    if (t.album) rememberAlbum(t.album);
  }

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
  // A restored session is already saved, and where in the track it was
  // isn't known to the player until it loads.
  if (!restore) saveSession();
  publish();
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
    playNow();
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

function setShuffle(on) {
  shuffle = on;
  keep('music-shuffle', shuffle);
  arrange(playingIndex);
  syncModes();
  renderUpNext();
  saveSession();
  publish();
}

function setRepeat(mode) {
  if (!['off', 'all', 'one'].includes(mode)) return;
  repeat = mode;
  keep('music-repeat', repeat);
  syncModes();
  renderUpNext();
  publish();
}

$('shuffle').addEventListener('click', () => setShuffle(!shuffle));
$('repeat').addEventListener('click', () => setRepeat({ off: 'all', all: 'one', one: 'off' }[repeat]));

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
    ...(more > 0 ? [el('p', { className: 'empty', textContent: `And ${more.toLocaleString()} more.` })] : [])
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
// album's or artist's in view, a mix's or playlist's first, or else what
// is playing.
function backdropFor(r) {
  if (r.kind === 'album') return coverUrl(r.artist, r.album);
  if (r.kind === 'mix') {
    const first = r.which === 'library' ? Array.isArray(libMix) && libMix[0] : Array.isArray(mix) && mix[0];
    if (first) return r.which === 'library' ? coverUrl(first.artist, first.album) : first.thumb;
  }
  if (r.kind === 'artist') {
    const a = findArtist(r.artist);
    if (a) return artistCover(a);
  }
  if (r.kind === 'playlist') {
    const first = playlistTracks.get(r.id)?.tracks?.[0];
    if (first) return coverUrl(first.artist, first.album);
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
  if (player.paused) playNow();
  else player.pause();
}

// playNow plays, and owns up when the browser won't let it: a page no one
// has clicked yet may not start sound, which a remote or a reload meets.
let blocked = false;
function playNow() {
  player.play().catch((err) => {
    if (err.name !== 'NotAllowedError') return;
    blocked = true;
    notice('The browser wants a click here before it plays');
    publish();
  });
}

playpause.addEventListener('click', togglePlay);

function syncPlaying() {
  const paused = player.paused;
  setIcon(playpause, paused ? 'play' : 'pause');
  playpause.setAttribute('aria-label', paused ? 'Play' : 'Pause');
  document.body.classList.toggle('paused', paused);
  syncTiles();
}

player.addEventListener('play', () => {
  blocked = false;
  syncPlaying();
  publish();
});
player.addEventListener('pause', () => {
  syncPlaying();
  saveSession();
  publish();
});
player.addEventListener('seeked', publish);
player.addEventListener('durationchange', publish);

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
  syncTiles();
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

// --------------------------------------------------------------- store

// The store's catalogue: a search lists its albums, and an album's page
// lists its tracks with their prices, those the library has already
// left unpicked. Nothing is bought until the picked tracks are confirmed.

let storeSearch = { q: '', albums: null }; // null unread, 'loading', 'failed', or the list
const storeAlbums = new Map(); // `artist\0album` → 'loading', 'failed', or the album's page
const storePicks = new Map(); // `artist\0album` → the track numbers picked

async function searchStore(q) {
  if (storeSearch.q === q && storeSearch.albums !== null) return;
  storeSearch = { q, albums: 'loading' };
  let albums = 'failed';
  try {
    const r = await fetch(`api/store/search?q=${encodeURIComponent(q)}`);
    if (r.ok) albums = await r.json();
  } catch {}
  if (storeSearch.q !== q) return;
  storeSearch.albums = albums;
  if (route().kind === 'search' && route().q === q) render();
}

// inLibrary is the library's album of the same name by the same artist.
function inLibrary(artist, album) {
  const a = library.find((x) => fold(x.name) === fold(artist));
  return a && a.albums.find((x) => fold(x.name) === fold(album));
}

// storeShelf is the store's albums for a search, when the server has a
// store to buy from: all of an artist's when the search names one, then
// a dozen of the rest.
function storeShelf(q) {
  if (!buysReady || storeSearch.q !== q) return null;
  const albums = storeSearch.albums;
  const section = (title, body) =>
    el('section', { className: 'shelf' }, el('div', { className: 'shelf-head' }, el('h2', { textContent: title })), body);
  const note = (text) => section('Albums in the store', el('p', { className: 'empty', textContent: text }));
  if (albums === 'loading' || albums === null) return note('Looking in the store…');
  if (albums === 'failed') return note('The store could not be searched.');
  if (!albums.length) return note('The store has no albums like that.');

  const theirs = albums.filter((a) => fold(a.artist) === fold(q));
  const rest = albums.filter((a) => fold(a.artist) !== fold(q)).slice(0, 12);
  return [
    theirs.length > 0 && section(`Albums by ${theirs[0].artist} in the store`, storeGrid(theirs)),
    rest.length > 0 && section(theirs.length ? 'More in the store' : 'Albums in the store', storeGrid(rest)),
  ];
}

function storeGrid(albums) {
  return el(
    'ul',
    { className: 'grid' },
    albums.map((a) => {
      const owned = inLibrary(a.artist, a.album);
      return el(
        'li',
        {},
        el(
          'a',
          { className: 'tile', href: href('store', a.artist, a.album) },
          art(a.album, owned ? coverUrl(a.artist, owned.name) : ''),
          el('span', { className: 'tile-title', textContent: a.album }),
          el('span', {
            className: 'tile-sub',
            textContent: [a.artist, a.year, owned && 'In library'].filter(Boolean).join(' · '),
          })
        )
      );
    })
  );
}

const dollars = (n) => `$${n.toFixed(2)}`;
const priceOf = (t) => parseFloat(String(t.price || '').replace(/[^\d.]/g, '')) || 0;

async function loadStoreAlbum(artist, album) {
  const key = `${artist}\0${album}`;
  storeAlbums.set(key, 'loading');
  let page = 'failed';
  try {
    const r = await fetch(`api/store/album?artist=${encodeURIComponent(artist)}&album=${encodeURIComponent(album)}`);
    if (r.ok) page = await r.json();
  } catch {}
  storeAlbums.set(key, page);
  const at = route();
  if (at.kind === 'store' && at.artist === artist && at.album === album) render();
}

// storeTrackState is where a store track stands for this library: in it,
// on its way, or to be bought, free when the account bought it before.
function storeTrackState(artist, t) {
  if (ownedKeys.has(ownKey(artist, t.title))) return { label: 'In library', pickable: false };
  const b = buys.find((x) => buyKey(x.artist, x.title) === buyKey(artist, t.title));
  if (b && b.status === 'done') return { label: 'Bought', pickable: false };
  if (b && (b.status === 'queued' || b.status === 'buying'))
    return { label: b.status === 'buying' ? 'Buying…' : 'Queued', pickable: false };
  return { label: t.download ? 'Free' : t.price || '', pickable: true, free: !!t.download };
}

function renderStoreAlbum(artist, album) {
  const key = `${artist}\0${album}`;
  const page = storeAlbums.get(key);
  if (page === undefined) loadStoreAlbum(artist, album);
  if (page === undefined || page === 'loading') {
    show(el('p', { className: 'empty', textContent: 'Reading the album in the store…' }));
    return;
  }
  if (page === 'failed' || !page.tracks?.length) {
    show(
      el('p', { className: 'empty' }, 'The store’s page for this album could not be read. ', button('btn', () => loadStoreAlbum(artist, album), 'Try again'))
    );
    return;
  }
  const states = page.tracks.map((t) => storeTrackState(artist, t));
  if (!storePicks.has(key)) storePicks.set(key, new Set(page.tracks.filter((_, i) => states[i].pickable).map((t) => t.number)));
  const picks = storePicks.get(key);
  const picked = page.tracks.filter((t, i) => states[i].pickable && picks.has(t.number));
  const cost = picked.reduce((n, t) => n + (t.download ? 0 : priceOf(t)), 0);
  const pickable = page.tracks.filter((_, i) => states[i].pickable);
  const owned = inLibrary(artist, album);

  const pick = (t, on) => {
    if (on) picks.add(t.number);
    else picks.delete(t.number);
    render();
  };
  const all = pickable.length > 0 && pickable.every((t) => picks.has(t.number));
  show(
    el(
      'header',
      { className: 'album-head' },
      art(album, owned ? coverUrl(artist, owned.name) : '', { eager: true }),
      el(
        'div',
        {},
        el('p', { className: 'kicker', textContent: 'In the store' }),
        el('h2', { textContent: page.name || album }),
        el(
          'p',
          { className: 'meta' },
          findArtist(artist) ? el('a', { href: href('artist', findArtist(artist).name), textContent: artist }) : artist,
          ` · ${plural(page.tracks.length, 'track')}`,
          owned && ' · ',
          owned && el('a', { href: href('album', findArtist(artist).name, owned.name), textContent: 'In the library' })
        ),
        el(
          'div',
          { className: 'btn-row' },
          el('button', {
            className: 'btn chrome',
            type: 'button',
            disabled: !picked.length,
            onclick: () => buyStoreAlbum(artist, album, page, picked, cost),
            textContent: picked.length
              ? `Buy ${plural(picked.length, 'track')} · ${dollars(cost)}`
              : pickable.length
                ? 'Nothing picked'
                : 'Nothing left to buy',
          }),
          pickable.length > 0 &&
            button(
              'btn ghost',
              () => {
                for (const t of pickable) all ? picks.delete(t.number) : picks.add(t.number);
                render();
              },
              all ? 'Pick none' : 'Pick all'
            )
        )
      )
    ),
    el(
      'ol',
      { className: 'tracks store-tracks' },
      page.tracks.map((t, i) => {
        const s = states[i];
        const box = el('input', {
          type: 'checkbox',
          checked: s.pickable && picks.has(t.number),
          disabled: !s.pickable,
          'aria-label': `Pick ${t.title}`,
          onchange: (e) => pick(t, e.target.checked),
        });
        return el(
          'li',
          { className: `store-track${s.pickable ? '' : ' unpickable'}` },
          el('label', {}, box, el('span', { className: 'num', textContent: t.number }), el('span', { className: 'name', textContent: t.title })),
          el('span', { className: 'meta', textContent: t.duration || '' }),
          el('span', { className: `store-state${s.free ? ' free' : ''}`, textContent: s.label })
        );
      })
    )
  );
}

// buyStoreAlbum queues the picked tracks once the price is confirmed.
async function buyStoreAlbum(artist, album, page, picked, cost) {
  const free = picked.filter((t) => t.download).length;
  const note = free ? ` (${free} bought before, so free)` : '';
  if (!confirm(`Buy ${plural(picked.length, 'track')} of ${artist} — ${album} for ${dollars(cost)}${note}?`)) return;
  const ok = await buysRequest(
    'POST',
    'api/buy/album',
    {
      artist,
      album: page.name || album,
      page: page.url,
      tracks: picked.map((t) => ({ number: t.number, title: t.title })),
    },
    'The album could not be queued'
  );
  if (ok) notice(`Queued ${plural(picked.length, 'track')} of ${album}. They show under Purchases as they land.`);
  render();
}

// ---------------------------------------------------------------- buys

// The server queues buys and runs them one at a time; the page polls the
// queue while any are pending and marks each track's button with where
// its buy stands.
let buys = []; // from api/buys, newest first
let buysPoll = null;

const buyKey = (artist, title) => `${artist}\u0000${title}`.toLowerCase();

const BUY_LABELS = { queued: 'Queued', buying: 'Buying…', done: 'Owned', failed: 'Retry' };

// buyTrack shows what the store would sell for a streamed track, the
// likeliest version first, and queues the one picked: its album's page
// and number go with the buy, so the buy is the version shown.
function buyTrack(e, t) {
  e.stopPropagation();
  const btn = e.currentTarget;
  const status = el('p', { className: 'yt-status', role: 'status', textContent: 'Looking in the store…' });
  const list = el('div', { className: 'buy-matches', role: 'radiogroup', 'aria-label': 'Versions in the store' });
  const buyBtn = el('button', { className: 'btn chrome', type: 'submit', disabled: true, textContent: 'Buy' });
  const find = el('a', {
    className: 'btn ghost',
    href: href('search', `${t.artist} ${t.title}`),
    onclick: () => dialog.close(),
    textContent: 'Browse the store',
  });
  let picked = null;

  const pick = (m) => {
    picked = m;
    buyBtn.disabled = false;
    buyBtn.textContent = `Buy · ${m.free ? 'free' : m.price || 'price unknown'}`;
  };
  const row = (m, i) => {
    const radio = el('input', { type: 'radio', name: 'match', disabled: m.owned, onchange: () => pick(m) });
    if (i === 0 && !m.owned) {
      radio.checked = true;
      pick(m);
    }
    return el(
      'label',
      { className: `buy-match${m.owned ? ' owned' : ''}` },
      radio,
      el(
        'span',
        { className: 'buy-match-text' },
        el('span', { className: 'name', textContent: m.title }),
        el('span', {
          className: 'sub',
          textContent: [`${m.artist} — ${m.album}`, m.year, m.duration].filter(Boolean).join(' · '),
        })
      ),
      el('span', {
        className: `store-state${m.free ? ' free' : ''}`,
        textContent: m.owned ? 'In library' : m.free ? 'Free' : m.price || '',
      })
    );
  };

  const form = el(
    'form',
    { method: 'dialog', className: 'editor yt-account buy-preview' },
    el('h2', { textContent: 'Buy a track' }),
    el('p', { className: 'yt-help', textContent: t.artist ? `${t.artist} — ${t.title}` : t.title }),
    status,
    list,
    el('div', { className: 'btn-row' }, buyBtn, find, button('btn ghost', () => dialog.close(), 'Cancel'))
  );
  form.onsubmit = async (ev) => {
    ev.preventDefault();
    if (!picked) return;
    buyBtn.disabled = true;
    const ok = await buysRequest(
      'POST',
      'api/buy',
      { artist: t.artist, title: t.title, album: picked.album, page: picked.page, number: picked.number },
      'The buy could not be queued'
    );
    if (!ok) {
      buyBtn.disabled = false;
      return;
    }
    dialog.close();
    btn.disabled = true;
    btn.textContent = 'Queued';
  };
  const dialog = el('dialog', { className: 'sheet' }, form);
  dialog.addEventListener('close', () => dialog.remove());
  document.body.append(dialog);
  dialog.showModal();

  if (!t.artist) {
    status.textContent = 'This stream names no artist to look for. Browse the store for the album instead.';
    return;
  }
  fetch(`api/buy/preview?artist=${encodeURIComponent(t.artist)}&title=${encodeURIComponent(t.title)}`)
    .then(async (r) => {
      if (!r.ok) throw new Error((await r.text()).trim().slice(0, 160) || r.status);
      return r.json();
    })
    .then(({ matches }) => {
      if (!matches.length) {
        status.textContent = 'The store has nothing by that name. Browse it for the album instead.';
        return;
      }
      status.textContent =
        matches.length === 1 ? 'The store has one version.' : `The store has ${matches.length} versions; pick the one to buy.`;
      list.append(...matches.map(row));
    })
    .catch((err) => {
      status.textContent = `The store could not be asked: ${err.message}`;
    });
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
  buysReady = true;
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
  markBuysLink();
  if (route().kind === 'buys') renderBuys();
  if (route().kind === 'store') render();
  if (landed) loadLibrary();

  const pending = buys.some((b) => b.status === 'queued' || b.status === 'buying');
  if (pending && !buysPoll) {
    buysPoll = setInterval(refreshBuys, 3000);
  } else if (!pending && buysPoll) {
    clearInterval(buysPoll);
    buysPoll = null;
  }
}

// markBuys labels each listed track's button with its newest buy, or as
// owned when the library has it, however it got there.
function markBuys() {
  const newest = new Map();
  for (const b of buys) {
    const k = buyKey(b.artist, b.title);
    if (!newest.has(k)) newest.set(k, b);
  }
  for (const btn of view.querySelectorAll('.buy')) {
    if (ownedKeys.has(btn.dataset.own)) {
      btn.textContent = 'Owned';
      btn.disabled = true;
      btn.classList.add('owned');
      btn.title = 'In the library';
      continue;
    }
    const b = newest.get(btn.dataset.buy);
    if (!b) continue;
    // A queued buy with an error has failed once and waits to try again.
    btn.textContent = b.status === 'queued' && b.error ? 'Retrying' : BUY_LABELS[b.status] || 'Buy';
    btn.disabled = b.status !== 'failed';
    btn.classList.toggle('owned', b.status === 'done');
    if (b.status === 'failed') btn.title = `The last try failed: ${b.error}`;
    else if (b.status === 'queued' && b.error) btn.title = `Retrying after: ${b.error}`;
  }
}

// markBuysLink shows the toolbar's way to the purchases once the server
// takes buys, counting the ones that failed for good.
function markBuysLink() {
  const failed = buys.filter((b) => b.status === 'failed').length;
  $('buys-link').hidden = !buysReady;
  const badge = $('buys-badge');
  badge.hidden = failed === 0;
  badge.textContent = failed > 9 ? '9+' : String(failed);
  $('buys-link').title = failed ? `Purchases: ${plural(failed, 'failed buy')}` : 'Purchases';
}

// The purchases page: what needs a hand first, then what is under way,
// what landed, and the store links found by hand.

let buysReady = false; // whether the server has a store to buy from
let storeLinks = null; // from api/buys/links: null unread, 'loading', or the list

async function loadStoreLinks() {
  storeLinks = 'loading';
  try {
    const r = await fetch('api/buys/links');
    storeLinks = r.ok ? await r.json() : [];
  } catch {
    storeLinks = [];
  }
  if (route().kind === 'buys') renderBuys();
}

const linkFor = (b) =>
  Array.isArray(storeLinks) ? storeLinks.find((l) => buyKey(l.artist, l.title) === buyKey(b.artist, b.title)) : null;

function renderBuys() {
  if (storeLinks === null) loadStoreLinks();
  // A field being typed in survives the poll's redraws.
  const typing = document.activeElement?.closest?.('.buy-link');
  if (typing && view.contains(typing)) return;

  const failed = buys.filter((b) => b.status === 'failed');
  const pending = buys.filter((b) => b.status === 'queued' || b.status === 'buying');
  const done = buys.filter((b) => b.status === 'done');
  const links = Array.isArray(storeLinks) ? storeLinks : [];
  show(
    el('h2', { className: 'view-title', textContent: 'Purchases' }),
    !buysReady && el('p', { className: 'empty', textContent: 'The server has no store to buy from.' }),
    buysReady &&
      !buys.length &&
      !links.length &&
      el('p', { className: 'empty', textContent: 'Nothing bought yet. Buy a streamed track, or an album found in the store, and it shows here.' }),
    failed.length > 0 && buysShelf('Needs attention', failed.map(failedRow)),
    pending.length > 0 && buysShelf('In progress', pending.map(pendingRow)),
    done.length > 0 && buysShelf('Bought', done.map(doneRow)),
    links.length > 0 && buysShelf('Store links', links.map(storeLinkRow))
  );
}

function buysShelf(title, rows) {
  return el(
    'section',
    { className: 'shelf buys-shelf' },
    el('div', { className: 'shelf-head' }, el('h2', { textContent: title })),
    el('ul', { className: 'purchases' }, rows)
  );
}

// purchase is one row: the track and where it stands, its buttons, and
// below them anything else the row needs.
function purchase(b, detail, actions, below) {
  return el(
    'li',
    { className: `purchase ${b.status || ''}` },
    el(
      'div',
      { className: 'purchase-text' },
      el('span', { className: 'name', textContent: b.title }),
      el('span', { className: 'sub', textContent: b.album ? `${b.artist} · ${b.album}` : b.artist }),
      detail
    ),
    el('div', { className: 'purchase-actions' }, actions),
    below
  );
}

const clock = (t) => new Date(t).toLocaleTimeString([], { hour: 'numeric', minute: '2-digit' });

// failedRow is a buy that failed, for good or for now: the reason, and a
// field for the store's album page when the provider couldn't find it.
function failedRow(b) {
  const retrying = b.status === 'queued';
  const saved = linkFor(b);
  const field = el('input', {
    type: 'url',
    required: true,
    placeholder: "Paste the store's album page",
    'aria-label': `The store's album page for ${b.artist} — ${b.title}`,
    value: saved ? saved.link : '',
  });
  const form = el(
    'form',
    {
      className: 'buy-link',
      onsubmit: (e) => {
        e.preventDefault();
        buyFromLink(b, field.value);
      },
    },
    field,
    el('button', { className: 'btn', type: 'submit', textContent: 'Buy from link' })
  );
  return purchase(
    b,
    el(
      'span',
      { className: 'purchase-error' },
      retrying
        ? `Retrying at ${clock(b.next)} after: ${b.error}`
        : `${plural(b.attempts, 'try', 'tries')}${b.link ? ', from the saved link' : ''}: ${b.error}`
    ),
    [!retrying && button('btn', () => retryBuy(b), 'Retry'), button('btn', () => dismissBuy(b), 'Dismiss')].filter(Boolean),
    form
  );
}

function pendingRow(b) {
  if (b.status === 'queued' && b.error) return failedRow(b);
  return purchase(b, el('span', { className: 'meta', textContent: b.status === 'buying' ? 'Buying…' : 'Queued' }), []);
}

function doneRow(b) {
  const r = b.result;
  return purchase(
    b,
    r &&
      el(
        'span',
        { className: 'meta' },
        r.alreadyOwned ? 'Already in the library: ' : 'Into ',
        el('a', { href: href('album', r.artist, r.album), textContent: `${r.artist} — ${r.album}` })
      ),
    [button('btn', () => dismissBuy(b), 'Dismiss')]
  );
}

function storeLinkRow(l) {
  return purchase(
    l,
    el('a', { className: 'meta store-link', href: l.link, target: '_blank', rel: 'noreferrer', textContent: l.link }),
    [button('btn', () => forgetStoreLink(l), 'Forget')]
  );
}

// retryBuy queues a failed buy again: it spends the store's balance, so
// it asks first, as Buy does.
async function retryBuy(b) {
  if (!confirm(`Buy ${b.artist} — ${b.title} into the library?`)) return;
  await buysRequest('POST', 'api/buy', { artist: b.artist, title: b.title }, 'The buy could not be queued');
}

async function buyFromLink(b, link) {
  if (!confirm(`Buy ${b.artist} — ${b.title} from that album page?`)) return;
  const ok = await buysRequest(
    'PUT',
    'api/buys/links',
    { artist: b.artist, title: b.title, link: link.trim() },
    'The link could not be saved'
  );
  if (ok) {
    document.activeElement?.blur();
    loadStoreLinks();
  }
}

async function dismissBuy(b) {
  await buysRequest('DELETE', `api/buys/${encodeURIComponent(b.id)}`, null, 'The buy could not be dismissed');
}

async function forgetStoreLink(l) {
  const ok = await buysRequest(
    'PUT',
    'api/buys/links',
    { artist: l.artist, title: l.title, link: '' },
    'The link could not be forgotten'
  );
  if (ok) loadStoreLinks();
}

async function buysRequest(method, url, body, failure) {
  try {
    const r = await fetch(url, {
      method,
      headers: body ? { 'Content-Type': 'application/json' } : {},
      body: body ? JSON.stringify(body) : undefined,
    });
    if (!r.ok) throw new Error((await r.text()).trim().slice(0, 160) || r.status);
  } catch (err) {
    notice(`${failure}: ${err.message}`);
    return false;
  }
  await refreshBuys();
  return true;
}

// ---------------------------------------------------- session, remote

// The session is the queue and where in it the player is, kept so a
// reload picks up where it left off, paused. Where in the track is kept
// apart, as it changes all the time and the queue can be long.
const SESSION = 'music-session';
const SESSION_AT = 'music-session-at';

function saveSession() {
  if (playingIndex < 0) return;
  keep(SESSION, { queue, order, index: playingIndex });
  saveAt();
}

const saveAt = () => keep(SESSION_AT, player.currentTime || 0);

function restoreSession() {
  const s = remember(SESSION, null);
  if (!s || !Array.isArray(s.queue) || !s.queue[s.index]) return;
  queue = s.queue;
  order =
    Array.isArray(s.order) && s.order.length === queue.length ? s.order : queue.map((_, i) => i);
  playQueue(queue, s.index, { restore: true, at: Number(remember(SESSION_AT, 0)) || 0 });
}

// Where it is in a track, every few seconds and on the way out.
let savedAt = 0;
player.addEventListener('timeupdate', () => {
  if (playingIndex < 0 || Date.now() - savedAt < 5000) return;
  savedAt = Date.now();
  saveAt();
});
window.addEventListener('pagehide', () => playingIndex >= 0 && player.readyState > 0 && saveAt());

// The page is a player the phone can drive, through the server: it keeps
// an event stream open for commands and puts its state at every change.
// The id is the tab's, so it lasts a reload but not a second tab.
const playerId = (() => {
  try {
    let id = sessionStorage.getItem('music-player-id');
    if (!id) {
      id = crypto.randomUUID();
      sessionStorage.setItem('music-player-id', id);
    }
    return id;
  } catch {
    return crypto.randomUUID();
  }
})();

function playerName() {
  const ua = navigator.userAgent;
  const browser = /Edg\//.test(ua) ? 'Edge' : /Firefox\//.test(ua) ? 'Firefox' : /Chrome\//.test(ua) ? 'Chrome' : /Safari\//.test(ua) ? 'Safari' : 'A browser';
  const os = /Android/.test(ua) ? 'Android' : /iPhone|iPad/.test(ua) ? 'iOS' : /Mac OS/.test(ua) ? 'macOS' : /Windows/.test(ua) ? 'Windows' : /Linux/.test(ua) ? 'Linux' : '';
  return os ? `${browser} on ${os}` : browser;
}

const absolute = (u) => (u ? new URL(u, location.href).href : '');

// remoteState is what the phone shows: the track, the transport, and
// what plays next, in the order it will.
function remoteState() {
  const t = queue[playingIndex];
  if (!t) return null;
  const at = order.indexOf(playingIndex);
  let next = order.slice(at + 1);
  if (repeat === 'all') next = next.concat(order.slice(0, at));
  const d = player.duration;
  return {
    key: t.key,
    title: t.name,
    sub: t.sub,
    art: absolute(t.art),
    playing: !player.paused,
    position: player.currentTime || 0,
    duration: Number.isFinite(d) ? d : 0,
    shuffle,
    repeat,
    hasNext: next.length > 0,
    blocked,
    upNext: next.slice(0, upNextShown).map((i) => ({ i, name: queue[i].name, sub: queue[i].sub, art: absolute(queue[i].art) })),
  };
}

let publishTimer = null;
function publish() {
  clearTimeout(publishTimer);
  publishTimer = setTimeout(() => {
    fetch(`api/remote/players/${encodeURIComponent(playerId)}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(remoteState()),
    }).catch(() => {});
  }, 150);
}

// obey runs a command from the phone.
function obey(c) {
  switch (c.do) {
    case 'toggle':
      togglePlay();
      break;
    case 'play':
      if (player.src) playNow();
      break;
    case 'pause':
      player.pause();
      break;
    case 'next':
      step(1);
      break;
    case 'prev':
      step(-1);
      break;
    case 'seek':
      if (Number.isFinite(c.at)) player.currentTime = c.at;
      break;
    case 'jump':
      if (queue[c.i]) playQueue(queue, c.i);
      break;
    case 'shuffle':
      setShuffle(!!c.on);
      break;
    case 'repeat':
      setRepeat(c.mode);
      break;
    case 'owned':
      if (Array.isArray(c.tracks) && c.tracks[c.index]) playQueue(libItems(c.tracks), c.index);
      break;
    case 'streams':
      if (Array.isArray(c.tracks) && c.tracks[c.index]) playQueue(streamItems(c.tracks), c.index);
      break;
  }
}

if ('EventSource' in window) {
  const events = new EventSource(
    `api/remote/events?id=${encodeURIComponent(playerId)}&name=${encodeURIComponent(playerName())}`
  );
  // Each (re)connect lists the page afresh, so it says where it is.
  events.addEventListener('open', publish);
  events.addEventListener('command', (e) => {
    try {
      obey(JSON.parse(e.data));
    } catch {
      // A command this page doesn't know.
    }
  });
}

restoreSession();
render();
refreshBuys();
