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
let playing = null; // { artist, album, index } of what the player holds
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
            dataset: { index: i },
            onclick: () => play(artist, album, i),
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

function play(artist, album, index) {
  const t = album.tracks[index];
  playing = { artist, album, index };
  player.src = `api/audio?path=${encodeURIComponent(t.path)}`;
  player.play();
  nowPlaying.hidden = false;
  npText.textContent = `${t.name} — ${artist.name} · ${album.name}`;
  playpause.textContent = 'Pause';
  markTrack();
  if ('mediaSession' in navigator) {
    // The lock screen's controls, on an installed app.
    navigator.mediaSession.metadata = new MediaMetadata({
      title: t.name,
      artist: artist.name,
      album: album.name,
    });
    navigator.mediaSession.setActionHandler('previoustrack', () => step(-1));
    navigator.mediaSession.setActionHandler('nexttrack', () => step(1));
  }
}

function step(delta) {
  if (!playing) return;
  const next = playing.index + delta;
  if (next >= 0 && next < playing.album.tracks.length) {
    play(playing.artist, playing.album, next);
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

// Highlights the row of what is playing, in whichever album is listed.
function markTrack() {
  for (const row of list.querySelectorAll('.track')) {
    const here = playing && listedAlbum === playing.album && Number(row.dataset.index) === playing.index;
    row.classList.toggle('current', Boolean(here));
  }
}

function mb(bytes) {
  return `${(bytes / 1e6).toFixed(1)} MB`;
}
