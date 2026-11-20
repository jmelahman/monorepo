'use strict';

// The library: artists, albums and singles served from the owned folder,
// played straight from it. Mixes and store purchases hang off the same
// api/ as they land.

const $ = (id) => document.getElementById(id);
const status = $('status');
const list = $('list');
const crumbs = $('crumbs');
const crumbArtist = $('crumb-artist');
const crumbAlbum = $('crumb-album');
const nowPlaying = $('now-playing');
const playpause = $('playpause');
const npText = $('np-text');
const player = $('player');

let library = []; // the artists, from api/library
let queue = []; // { key, name, sub, url }: owned tracks and streams alike
let playingIndex = -1; // where in the queue the player is
let listedAlbum = null; // the album whose tracks are on screen

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
  .then((h) => {
    status.textContent = h.ok ? `up · ${h.music}` : 'down';
  })
  .catch(() => {
    status.textContent = 'down';
  });

fetch('api/library')
  .then((r) => r.json())
  .then((artists) => {
    library = artists;
    renderArtists();
  })
  .catch(() => {
    list.append(el('li', { className: 'empty', textContent: 'The library could not be read.' }));
  });

// The native client's APK, offered only where it can install: on the
// phone, when a build is actually mounted behind the endpoint.
if (/Android/.test(navigator.userAgent)) {
  fetch('api/apk', { method: 'HEAD' })
    .then((r) => {
      if (r.ok) $('download-app').hidden = false;
    })
    .catch(() => {});
}

// ---------------------------------------------------------------- nodes

const el = (tag, attrs, ...children) => {
  const node = document.createElement(tag);
  for (const [k, v] of Object.entries(attrs || {})) node[k] = v;
  node.append(...children.flat());
  return node;
};

// ---------------------------------------------------------------- lists

function renderArtists() {
  crumbs.hidden = true;
  saveBtn.hidden = true;
  list.replaceChildren(
    ...library.map((a) =>
      el(
        'li',
        {},
        el('button', {
          className: 'row',
          type: 'button',
          textContent: a.name,
          onclick: () => renderAlbums(a),
        })
      )
    )
  );
}

function renderAlbums(artist) {
  crumbs.hidden = false;
  saveBtn.hidden = true;
  crumbArtist.textContent = artist.name;
  crumbAlbum.replaceChildren();
  list.replaceChildren(
    ...artist.albums.map((al) =>
      el(
        'li',
        {},
        el('button', {
          className: 'row',
          type: 'button',
          textContent: al.name,
          onclick: () => renderTracks(artist, al),
        })
      )
    )
  );
}

function renderTracks(artist, album) {
  crumbAlbum.textContent = album.name;
  listedAlbum = album;
  refreshSaveButton(album);
  const items = album.tracks.map((t) => ({
    key: t.path,
    name: t.name,
    sub: `${artist.name} · ${album.name}`,
    url: `api/audio?path=${encodeURIComponent(t.path)}`,
  }));
  list.replaceChildren(
    ...album.tracks.map((t, i) =>
      el(
        'li',
        {},
        el(
          'button',
          {
            className: 'row track',
            type: 'button',
            dataset: { key: items[i].key },
            onclick: () => playQueue(items, i),
          },
          el('span', { className: 'name', textContent: t.name }),
          el('span', { className: 'size', textContent: mb(t.size) })
        )
      )
    )
  );
  markTrack();
}

$('crumb-root').addEventListener('click', renderArtists);
crumbArtist.addEventListener('click', () => {
  const artist = library.find((a) => a.name === crumbArtist.textContent);
  if (artist) renderAlbums(artist);
});

// ------------------------------------------------------ saved offline

const saveBtn = $('save-offline');

saveBtn.addEventListener('click', () => {
  if (listedAlbum) toggleSave(listedAlbum);
});

// The button mirrors the album's saved state: how many of its tracks the
// cache holds.
async function refreshSaveButton(album) {
  const have = await savedCount(album);
  saveBtn.hidden = false;
  if (have === 0) {
    saveBtn.textContent = 'Save offline';
  } else if (have === album.tracks.length) {
    saveBtn.textContent = 'Saved offline — remove';
  } else {
    saveBtn.textContent = `Save offline (${have} of ${album.tracks.length})`;
  }
}

function savedCount(album) {
  return caches.open(AUDIO).then((cache) =>
    Promise.all(album.tracks.map((t) => cache.match(offlineKey(t.path)))).then((hits) => hits.filter(Boolean).length)
  );
}

// toggleSave pins an album into the cache, track by track, or clears it
// once everything is there. Saved tracks play from the cache, so this is
// also a "stop streaming this album" switch.
async function toggleSave(album) {
  const cache = await caches.open(AUDIO);
  const have = await savedCount(album);
  if (have === album.tracks.length) {
    await Promise.all(album.tracks.map((t) => cache.delete(offlineKey(t.path))));
    await refreshSaveButton(album);
    return;
  }
  saveBtn.disabled = true;
  nowPlaying.hidden = false;
  for (let i = 0; i < album.tracks.length; i++) {
    const t = album.tracks[i];
    if (await cache.match(offlineKey(t.path))) continue;
    try {
      const resp = await fetch(`api/audio?path=${encodeURIComponent(t.path)}`);
      if (!resp.ok) throw new Error(resp.status);
      await cache.put(offlineKey(t.path), resp);
    } catch {
      npText.textContent = `Could not save ${t.name} — the phone may be out of space`;
      break;
    }
    npText.textContent = `Saving ${album.name}… ${i + 1} of ${album.tracks.length}`;
  }
  saveBtn.disabled = false;
  await refreshSaveButton(album);
}

// ---------------------------------------------------------------- play

// playQueue starts the queue at an index; the queue holds owned tracks
// and streams alike, each with the key its row is marked by.
function playQueue(items, index) {
  queue = items;
  playingIndex = index;
  const t = items[index];
  player.src = t.url;
  player.play();
  nowPlaying.hidden = false;
  npText.textContent = `${t.name} — ${t.sub}`;
  playpause.textContent = 'Pause';
  markTrack();
  if ('mediaSession' in navigator) {
    // The lock screen's controls, on an installed app.
    navigator.mediaSession.metadata = new MediaMetadata({
      title: t.name,
      artist: t.sub,
    });
    navigator.mediaSession.setActionHandler('previoustrack', () => step(-1));
    navigator.mediaSession.setActionHandler('nexttrack', () => step(1));
  }
}

function step(delta) {
  const next = playingIndex + delta;
  if (next >= 0 && next < queue.length) {
    playQueue(queue, next);
  }
}

player.addEventListener('ended', () => step(1));

// A track that won't load offline is a track that was never saved.
player.addEventListener('error', () => {
  if (!navigator.onLine) {
    nowPlaying.hidden = false;
    npText.textContent = 'Offline, and this album is not saved';
  }
});

playpause.addEventListener('click', () => {
  if (player.paused) {
    player.play();
    playpause.textContent = 'Pause';
  } else {
    player.pause();
    playpause.textContent = 'Play';
  }
});

// Highlights the row of what is playing, in whichever list holds it.
function markTrack() {
  const current = queue[playingIndex]?.key;
  for (const row of document.querySelectorAll('.row[data-key]')) {
    row.classList.toggle('current', row.dataset.key === current);
  }
}

function mb(bytes) {
  return `${(bytes / 1e6).toFixed(1)} MB`;
}

function fmtDuration(seconds) {
  if (!seconds) return '';
  const m = Math.floor(seconds / 60);
  const s = String(seconds % 60).padStart(2, '0');
  return `${m}:${s}`;
}

// ------------------------------------------------------------ discover

const discoverForm = $('discover-form');
const discoverInput = $('discover-q');
const discoverList = $('discover-list');
let discoverItems = [];

discoverForm.addEventListener('submit', (e) => {
  e.preventDefault();
  const q = discoverInput.value.trim();
  if (!q) return;
  discoverList.hidden = false;
  discoverList.replaceChildren(el('li', { className: 'empty', textContent: 'Searching…' }));
  fetch(`api/yt/search?q=${encodeURIComponent(q)}`)
    .then((r) => r.json())
    .then(renderStreams)
    .catch(() => {
      discoverList.replaceChildren(el('li', { className: 'empty', textContent: 'The search could not be read.' }));
    });
});

$('supermix').addEventListener('click', () => {
  discoverList.hidden = false;
  discoverList.replaceChildren(
    el('li', { className: 'empty', textContent: 'Building a mix from the owned library…' })
  );
  fetch('api/yt/supermix')
    .then((r) => r.json())
    .then(renderStreams)
    .catch(() => {
      discoverList.replaceChildren(el('li', { className: 'empty', textContent: 'The mix could not be built.' }));
    });
});

function renderStreams(tracks) {
  discoverItems = tracks.map((t) => ({
    key: `yt:${t.videoId}`,
    name: t.title,
    sub: t.artist,
    url: `api/yt/stream?v=${encodeURIComponent(t.videoId)}`,
  }));
  discoverList.replaceChildren(
    ...tracks.map((t, i) =>
      el(
        'li',
        {},
        el(
          'button',
          {
            className: 'row stream',
            type: 'button',
            dataset: { key: discoverItems[i].key },
            onclick: () => playQueue(discoverItems, i),
          },
          el('span', { className: 'name', textContent: t.title }),
          el('span', { className: 'size', textContent: fmtDuration(t.duration) })
        )
      )
    )
  );
  markTrack();
}
