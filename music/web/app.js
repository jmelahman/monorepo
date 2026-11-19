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
