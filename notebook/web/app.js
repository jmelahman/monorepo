// The notebook's page: sources, and two-host audio episodes made from
// them. Notebooks live in this browser (db.js); Claude Code runs behind
// the notebook server's /api, and voices come from audio.js.

import * as db from './db.js';
import { BrowserEngine, ServerEngine, RATE, assemble, gap, trim, wav, toInt16, fromInt16 } from './audio.js';

const THEME_KEY = 'notebook:theme'; // also read by theme.js
const CURRENT_KEY = 'notebook:current';
const MODEL_KEY = 'notebook:model';
const SETTINGS_KEY = 'notebook:settings';

// Kokoro's English voices, which Kokoro speech servers share. The browser
// engine names and grades them once it has loaded.
const VOICES = [
  'af_heart', 'af_bella', 'af_nicole', 'af_aoede', 'af_kore', 'af_sarah', 'af_nova', 'af_sky', 'af_alloy',
  'af_jessica', 'af_river', 'am_michael', 'am_fenrir', 'am_puck', 'am_echo', 'am_eric', 'am_liam', 'am_onyx',
  'am_adam', 'bf_emma', 'bf_isabella', 'bf_alice', 'bf_lily', 'bm_george', 'bm_fable', 'bm_lewis', 'bm_daniel',
];

const $ = (id) => document.getElementById(id);
const root = document.documentElement;
const darkScheme = matchMedia('(prefers-color-scheme: dark)');

const state = {
  notebook: null,
  sources: [],
  episodes: [],
  episode: null,
  audio: null, // {blob, starts, url, lines, episode} for the episode on the player
  status: null,
};

// Every episode is one object for the life of the page, however often its
// notebook is opened, so a script being written and the one on screen are
// the same. A deleted episode is marked so that nothing writes it back.
const known = new Map();
const deletedNotebooks = new Set();

function remember(ep) {
  if (!known.has(ep.id)) known.set(ep.id, ep);
  return known.get(ep.id);
}

// Space kept for spoken lines; the least recently played go past it.
const CLIP_ROOM = 1e9;

function el(tag, props = {}, ...children) {
  const node = Object.assign(document.createElement(tag), props);
  node.append(...children.filter((c) => c != null));
  return node;
}

function load(key, fallback) {
  try {
    return JSON.parse(localStorage.getItem(key)) ?? fallback;
  } catch {
    return fallback;
  }
}

function save(key, value) {
  try {
    localStorage.setItem(key, JSON.stringify(value));
  } catch {
    // Storage is disabled; it just won't be remembered.
  }
}

let toastTimer;
function toast(message, info = false) {
  const t = $('toast');
  t.textContent = message;
  t.classList.toggle('info', info);
  t.hidden = false;
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => (t.hidden = true), info ? 4000 : 8000);
}

function words(text) {
  return text.split(/\s+/).filter(Boolean).length;
}

function debounce(fn, ms = 400) {
  let timer;
  return (...args) => {
    clearTimeout(timer);
    timer = setTimeout(() => fn(...args), ms);
  };
}

async function sha256(s) {
  const hash = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(s));
  return [...new Uint8Array(hash)].map((b) => b.toString(16).padStart(2, '0')).join('');
}

function aborted(err) {
  return err?.name === 'AbortError';
}

// ---------------------------------------------------------------- server

/** POSTs JSON and calls on(event, data) for each server-sent event. */
async function stream(path, body, on, signal) {
  const res = await fetch(path, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
    signal,
  });
  if (!res.ok) throw new Error((await res.json().catch(() => ({}))).error || `${res.status} ${res.statusText}`);
  const reader = res.body.pipeThrough(new TextDecoderStream()).getReader();
  let buf = '';
  try {
    for (;;) {
      const { value, done } = await reader.read();
      if (done) break;
      buf += value;
      let i;
      while ((i = buf.indexOf('\n\n')) >= 0) {
        const chunk = buf.slice(0, i);
        buf = buf.slice(i + 2);
        let event = 'message';
        let data = '';
        for (const line of chunk.split('\n')) {
          if (line.startsWith('event: ')) event = line.slice(7);
          else if (line.startsWith('data: ')) data += line.slice(6);
        }
        const v = JSON.parse(data);
        if (event === 'error') throw new Error(v.error);
        on(event, v);
      }
    }
  } finally {
    // Hangs up if reading stopped early, so the server stops Claude.
    reader.cancel().catch(() => {});
  }
}

async function post(path, body) {
  const res = await fetch(path, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
  const v = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(v.error || `${res.status} ${res.statusText}`);
  return v;
}

function model() {
  return $('model').value;
}

async function checkStatus() {
  const s = $('status');
  try {
    const res = await fetch('api/status');
    if (!res.ok) throw new Error(res.statusText);
    state.status = await res.json();
  } catch {
    state.status = null;
    s.className = 'status bad';
    s.textContent = 'No notebook server';
    s.title = 'Finding sources and writing scripts need Claude Code: run `notebook serve` and open the page it serves.';
    return;
  }
  const { claude, publish, tts, models } = state.status;
  s.className = `status ${claude.ok ? 'ok' : 'bad'}`;
  s.textContent = claude.detail;
  s.title = [`Claude: ${claude.detail}`, `Publishing: ${publish.detail}`, `Voices: ${tts.detail}`].join('\n');

  const chosen = load(MODEL_KEY, '');
  $('model').replaceChildren(el('option', { value: '', textContent: 'Default' }), ...models.map((m) => el('option', { value: m, textContent: m[0].toUpperCase() + m.slice(1) })));
  $('model').value = models.includes(chosen) ? chosen : '';

  const server = $('engine').querySelector('[value=server]');
  server.disabled = !tts.ok;
  server.title = tts.detail;
  if (!tts.ok && $('engine').value === 'server') $('engine').value = 'browser';
  $('publish').title = publish.ok ? `Publish to ${publish.detail}` : publish.detail;
  renderPlayer();
}

// ---------------------------------------------------------------- theme

function isDark() {
  return root.dataset.theme ? root.dataset.theme === 'dark' : darkScheme.matches;
}

function applyTheme() {
  const dark = isDark();
  root.classList.toggle('dark', dark);
  const label = `Switch to the ${dark ? 'light' : 'dark'} theme`;
  $('theme').title = label;
  $('theme').ariaLabel = label;
}

// ---------------------------------------------------------------- notebooks

async function notebooks() {
  return (await db.all('notebooks')).sort((a, b) => b.updated - a.updated);
}

async function createNotebook(title = 'Untitled notebook') {
  const now = Date.now();
  const nb = { id: db.uid(), title, created: now, updated: now };
  await db.put('notebooks', nb);
  return nb;
}

async function touch(id = state.notebook.id) {
  const nb = state.notebook.id === id ? state.notebook : await db.get('notebooks', id);
  if (!nb || deletedNotebooks.has(id)) return;
  nb.updated = Date.now();
  await db.put('notebooks', nb);
}

async function openNotebook(id) {
  const all = await notebooks();
  const nb = all.find((n) => n.id === id) ?? all[0] ?? (await createNotebook());
  state.notebook = nb;
  save(CURRENT_KEY, nb.id);
  state.sources = (await db.byNotebook('sources', nb.id)).sort((a, b) => a.added - b.added);
  state.episodes = (await db.byNotebook('episodes', nb.id)).map(remember).sort((a, b) => b.created - a.created);
  // What Claude finds is for the notebook it was asked from.
  discovering?.abort();
  $('candidates').hidden = true;
  $('discover-log').hidden = true;
  setEpisode(state.episodes[0] ?? null);
  renderNotebook();
  renderSources();
}

async function renderNotebook() {
  $('notebook-title').textContent = state.notebook.title;
  document.title = `${state.notebook.title} · Notebook`;
  const all = await notebooks();
  $('notebook-list').replaceChildren(
    ...all.map((nb) => {
      const b = el('button', { type: 'button', role: 'menuitem', textContent: nb.title });
      b.setAttribute('aria-current', String(nb.id === state.notebook.id));
      b.addEventListener('click', () => {
        $('notebook-menu').hidePopover();
        openNotebook(nb.id);
      });
      return b;
    }),
  );
}

// ---------------------------------------------------------------- sources

function enabled() {
  return state.sources.filter((s) => s.enabled);
}

/** The sources as the API takes them, numbered by their order. */
function apiSources(list = enabled()) {
  return list.map(({ id, title, url, text }) => ({ id, title, url, text }));
}

/** Adds a source to a notebook: the one that was open when it was asked for. */
async function addSource(notebook, { title, url = '', siteName = '', text }) {
  if (deletedNotebooks.has(notebook)) return null;
  const src = { id: db.uid(), notebook, title: title || url || 'Untitled', url, siteName, text, enabled: true, added: Date.now() };
  await db.put('sources', src);
  if (deletedNotebooks.has(notebook)) {
    await db.del('sources', src.id);
    return null;
  }
  await touch(notebook);
  if (state.notebook.id === notebook) {
    state.sources.push(src);
    renderSources();
  }
  return src;
}

function renderSources() {
  const on = enabled();
  $('source-count').textContent = state.sources.length ? `${on.length}/${state.sources.length}` : '';
  $('sources-empty').hidden = state.sources.length > 0;
  $('toggle-all').hidden = state.sources.length < 2;
  $('source-list').replaceChildren(
    ...state.sources.map((src) => {
      const box = el('input', { type: 'checkbox', checked: src.enabled, ariaLabel: `Use ${src.title}` });
      box.addEventListener('change', async () => {
        src.enabled = box.checked;
        await db.put('sources', src);
        renderSources();
      });
      const title = el('button', { type: 'button', className: 'title', textContent: src.title, title: src.title });
      title.addEventListener('click', () => view(src));
      const remove = el('button', { type: 'button', className: 'small icon', ariaLabel: `Remove ${src.title}`, title: 'Remove' });
      remove.innerHTML = '<svg viewBox="0 0 24 24"><path d="M6 6l12 12M18 6L6 18"/></svg>';
      remove.addEventListener('click', async () => {
        if (!confirm(`Remove “${src.title}”?`)) return;
        await db.del('sources', src.id);
        state.sources = state.sources.filter((s) => s !== src);
        renderSources();
      });
      let host = src.siteName;
      if (!host && src.url) {
        try {
          host = new URL(src.url).hostname.replace(/^www\./, '');
        } catch {}
      }
      const meta = el('span', { className: 'meta', textContent: [host, `${words(src.text).toLocaleString()} words`].filter(Boolean).join(' · ') });
      return el('li', { className: src.enabled ? '' : 'off' }, box, title, remove, meta);
    }),
  );
}

function view(src) {
  $('viewer-title').textContent = src.title;
  $('viewer-url').hidden = !src.url;
  $('viewer-url').href = src.url || '';
  $('viewer-url').textContent = src.url || '';
  $('viewer-text').textContent = src.text;
  if (!$('viewer').open) $('viewer').showModal();
  $('viewer-text').scrollTop = 0;
}

async function fetchSource(url) {
  const src = await post('api/fetch', { url });
  if (!src.text?.trim()) throw new Error('no readable text on the page');
  return src;
}

let discovering;

async function discover(e) {
  e.preventDefault();
  const topic = $('topic').value.trim();
  if (!topic) return;
  discovering = new AbortController();
  const log = $('discover-log');
  log.replaceChildren(el('li', { textContent: 'Asking Claude…' }));
  log.hidden = false;
  $('candidates').hidden = true;
  $('discover-go').hidden = true;
  $('discover-stop').hidden = false;
  const have = new Set(state.sources.map((s) => s.url).filter(Boolean));
  const notebook = state.notebook.id;
  try {
    await stream(
      'api/discover',
      { topic, count: Number($('discover-count').value) || 6, exclude: [...have], model: model() },
      (event, v) => {
        if (event === 'progress') {
          const text = v.query ? `Searching: ${v.query}` : v.url ? `Reading: ${v.url}` : v.tool;
          log.append(el('li', { textContent: text, title: text }));
        } else if (event === 'result' && state.notebook.id === notebook) {
          showCandidates((v.sources ?? []).filter((c) => !have.has(c.url)));
        }
      },
      discovering.signal,
    );
    log.hidden = true;
  } catch (err) {
    if (!aborted(err)) toast(`Finding sources failed: ${err.message}`);
    log.hidden = true;
  } finally {
    $('discover-go').hidden = false;
    $('discover-stop').hidden = true;
  }
}

function showCandidates(candidates) {
  if (!candidates.length) {
    toast('Claude found nothing new on that.', true);
    return;
  }
  $('candidate-list').replaceChildren(
    ...candidates.map((c) => {
      const box = el('input', { type: 'checkbox', checked: true });
      const link = el('a', { href: c.url, target: '_blank', rel: 'noopener noreferrer', textContent: c.title || c.url, className: 'title' });
      const li = el('li', {}, el('label', {}, box, link, el('span'), el('span', { className: 'why', textContent: c.why })));
      li.candidate = c;
      return li;
    }),
  );
  $('candidates').hidden = false;
}

async function addCandidates() {
  const items = [...$('candidate-list').children].filter((li) => li.querySelector('input').checked);
  const button = $('add-candidates');
  button.disabled = true;
  const notebook = state.notebook.id;
  let added = 0;
  // A few pages at a time.
  const queue = [...items];
  const work = async () => {
    for (let li; (li = queue.shift()); ) {
      li.querySelector('.fail')?.remove();
      try {
        const src = await fetchSource(li.candidate.url);
        await addSource(notebook, { ...src, title: src.title || li.candidate.title });
        li.remove();
        added++;
      } catch (err) {
        li.querySelector('label').append(el('span', { className: 'fail', textContent: err.message }));
      }
    }
  };
  await Promise.all([work(), work(), work(), work()]);
  button.disabled = false;
  if (!$('candidate-list').children.length) $('candidates').hidden = true;
  if (added) toast(`Added ${added} source${added === 1 ? '' : 's'}.`, true);
}

async function addFiles(files) {
  const notebook = state.notebook.id;
  for (const file of files) {
    const text = await file.text();
    const heading = /^#\s+(.+)$/m.exec(text)?.[1];
    await addSource(notebook, { title: heading?.trim() || file.name.replace(/\.[^.]+$/, ''), text });
  }
}

// ---------------------------------------------------------------- studio

const settings = {
  fields: ['minutes', 'style', 'focus', 'host-a', 'host-b', 'voice-a', 'voice-b', 'engine', 'speed'],
  restore() {
    const saved = load(SETTINGS_KEY, {});
    for (const f of this.fields) if (saved[f] != null) $(f).value = saved[f];
  },
  save() {
    save(SETTINGS_KEY, Object.fromEntries(this.fields.map((f) => [f, $(f).value])));
  },
};

function fillVoices(voices = VOICES.map((id) => ({ id }))) {
  for (const [select, fallback] of [[$('voice-a'), 'af_heart'], [$('voice-b'), 'am_michael']]) {
    const was = select.value || load(SETTINGS_KEY, {})[select.id] || fallback;
    select.replaceChildren(
      ...voices.map((v) => {
        const label = v.name ? `${v.name} (${v.id.slice(0, 2)}${v.grade ? `, ${v.grade}` : ''})` : v.id;
        return el('option', { value: v.id, textContent: label });
      }),
    );
    select.value = voices.some((v) => v.id === was) ? was : fallback;
  }
}

function hostName(speaker) {
  return (speaker === 'A' ? $('host-a').value : $('host-b').value).trim() || (speaker === 'A' ? 'Alex' : 'Sam');
}

// Edits are saved a moment after typing stops, each episode on its own.
const pendingSaves = new Map();

function putEpisode(ep) {
  if (!ep.deleted) db.put('episodes', ep);
}

function saveEpisode(ep) {
  clearTimeout(pendingSaves.get(ep));
  pendingSaves.set(
    ep,
    setTimeout(() => {
      pendingSaves.delete(ep);
      putEpisode(ep);
    }, 400),
  );
}

function flushSaves() {
  for (const [ep, timer] of pendingSaves) {
    clearTimeout(timer);
    putEpisode(ep);
  }
  pendingSaves.clear();
}

function forgetEpisode(ep) {
  clearTimeout(pendingSaves.get(ep));
  pendingSaves.delete(ep);
  ep.deleted = true;
}

function setEpisode(ep) {
  // The audio being made is for the episode on screen.
  if (synthesizing && ep !== state.episode) stopSynthesis();
  state.episode = ep;
  clearAudio();
  renderEpisodes();
  renderScript();
}

function renderEpisodes() {
  const list = $('episode-list');
  list.hidden = state.episodes.length < 2;
  list.replaceChildren(
    ...state.episodes.map((ep) => el('option', { value: ep.id, textContent: `${ep.title || 'Untitled'} · ${new Date(ep.created).toLocaleDateString()}` })),
  );
  if (state.episode) list.value = state.episode.id;
  $('delete-episode').hidden = !state.episode;
  $('episode').hidden = !state.episode;
  $('studio-empty').hidden = Boolean(state.episode);
  if (state.episode) {
    $('episode-title').value = state.episode.title;
    $('episode-description').value = state.episode.description;
  }
}

function scriptChanged() {
  saveEpisode(state.episode);
  if (state.audio) {
    state.audio.starts = null;
    $('synth-status').textContent = 'The script changed since this audio was made.';
    $('synth-status').hidden = false;
  }
}

function lineEl(ep, line) {
  const speaker = line.speaker === 'B' ? 'b' : 'a';
  const who = el('button', { type: 'button', className: 'who', textContent: hostName(line.speaker), title: 'Switch speaker' });
  who.addEventListener('click', () => {
    line.speaker = line.speaker === 'A' ? 'B' : 'A';
    scriptChanged();
    renderScript();
  });
  const text = el('textarea', { className: 'text', value: line.text, rows: 1, ariaLabel: `${hostName(line.speaker)} says` });
  text.addEventListener('input', () => {
    line.text = text.value;
    scriptChanged();
  });
  const tool = (label, path, fn) => {
    const b = el('button', { type: 'button', className: 'small icon', title: label, ariaLabel: label });
    b.innerHTML = `<svg viewBox="0 0 24 24"><path d="${path}"/></svg>`;
    b.addEventListener('click', fn);
    return b;
  };
  const tools = el(
    'div',
    { className: 'tools' },
    tool('Play this line', 'M8 5v14l11-7z', () => playLine(line)),
    tool('Add a line after', 'M12 5v14M5 12h14', () => {
      const i = ep.lines.indexOf(line);
      const next = { speaker: line.speaker === 'A' ? 'B' : 'A', text: '', segment: line.segment };
      ep.lines.splice(i + 1, 0, next);
      scriptChanged();
      renderScript();
      $('script').querySelectorAll('.line')[i + 1]?.querySelector('textarea').focus();
    }),
    tool('Delete this line', 'M6 6l12 12M18 6L6 18', () => {
      ep.lines.splice(ep.lines.indexOf(line), 1);
      scriptChanged();
      renderScript();
    }),
  );
  return el('li', { className: `line ${speaker}` }, who, text, tools);
}

function renderScript() {
  const ep = state.episode;
  if (!ep) {
    $('script').replaceChildren();
    return;
  }
  const items = [];
  let segment = -1;
  for (const line of ep.lines) {
    if (line.segment !== segment) {
      segment = line.segment;
      const heading = ep.segments[segment]?.heading;
      if (heading) items.push(el('li', { className: 'segment', textContent: heading }));
    }
    items.push(lineEl(ep, line));
  }
  $('script').replaceChildren(...items);
  renderPlayer();
}

let writing;

async function writeScript() {
  const sources = enabled();
  if (!sources.length) {
    toast('Check at least one source to make an episode from.');
    return;
  }
  writing = new AbortController();
  const notebook = state.notebook;
  const progress = $('script-progress');
  progress.textContent = 'Planning the episode…';
  progress.hidden = false;
  $('write-script').hidden = true;
  $('script-stop').hidden = false;
  let ep = null;
  try {
    await stream(
      'api/script',
      {
        notebook: notebook.title,
        sources: apiSources(sources),
        minutes: Number($('minutes').value) || 15,
        style: $('style').value,
        focus: $('focus').value.trim(),
        hosts: [{ name: hostName('A') }, { name: hostName('B') }],
        model: model(),
      },
      (event, v) => {
        if (event === 'outline') {
          if (deletedNotebooks.has(notebook.id)) {
            writing.abort();
            return;
          }
          ep = remember({
            id: db.uid(),
            notebook: notebook.id,
            title: v.title,
            description: v.description,
            segments: v.segments,
            lines: [],
            created: Date.now(),
          });
          putEpisode(ep);
          if (state.notebook.id === notebook.id) {
            state.episodes.unshift(ep);
            setEpisode(ep);
          }
        } else if (event === 'progress' && v.step === 'segment') {
          progress.textContent = `Writing part ${v.segment + 1} of ${v.of}: ${ep?.segments[v.segment]?.heading ?? ''}`;
        } else if (event === 'lines' && ep) {
          // Deleted while it was being written: stop writing it.
          if (ep.deleted) {
            writing.abort();
            return;
          }
          ep.lines.push(...v.lines.map((l) => ({ speaker: l.speaker === 'B' ? 'B' : 'A', text: l.text, segment: v.segment })));
          putEpisode(ep);
          if (state.episode === ep) renderScript();
        }
      },
      writing.signal,
    );
    progress.hidden = true;
    if (ep) toast(`Wrote “${ep.title}”: about ${Math.round(ep.lines.reduce((n, l) => n + words(l.text), 0) / 150)} minutes.`, true);
  } catch (err) {
    progress.hidden = true;
    if (!aborted(err)) toast(`Writing the script failed: ${err.message}`);
    else if (ep && !ep.deleted) toast('Stopped; the parts written so far are kept.', true);
  } finally {
    writing = null;
    $('write-script').hidden = false;
    $('script-stop').hidden = true;
  }
}

// ---------------------------------------------------------------- speech

const engines = {
  browser: new BrowserEngine(({ loaded, total }) => {
    const mb = (n) => (n / 1e6).toFixed(0);
    synthMessage(`Loading the voice model: ${mb(loaded)} of ${mb(total)} MB (once; the browser keeps it)`);
  }),
  server: new ServerEngine(),
};

function synthMessage(text) {
  $('synth-status').textContent = text;
  $('synth-status').hidden = !text;
}

/** One line's speech, from the cache or made now: {samples, cached}. */
async function speech(line) {
  const engine = $('engine').value;
  const voice = line.speaker === 'B' ? $('voice-b').value : $('voice-a').value;
  const speed = Number($('speed').value) || 1;
  const key = await sha256(JSON.stringify([engine, voice, speed, line.text.trim()]));
  const cached = await db.getClip(key);
  if (cached) return { samples: fromInt16(cached.samples), cached: true };
  if (engine === 'browser') {
    const ready = await engines.browser.load();
    if (!engines.browser.named) {
      engines.browser.named = true;
      fillVoices(ready.voices);
    }
  }
  const { samples } = await engines[engine].speak(line.text.trim(), voice, speed);
  await db.putClip({ key, samples: toInt16(samples) });
  return { samples, cached: false };
}

let synthesizing = false;
let stopSynth = false;
// The lines made so far, in order, for listening along.
let synthClips = [];

async function synthesize() {
  const ep = state.episode;
  const lines = ep.lines.filter((l) => l.text.trim());
  if (!lines.length) return;
  synthesizing = true;
  stopSynth = false;
  $('synthesize').hidden = true;
  $('synth-stop').hidden = false;
  $('synth-listen').hidden = false;
  synthClips = [];
  const bar = $('synth-progress');
  bar.hidden = false;
  bar.value = 0;
  // Lines are spoken a few at a time, as many as there are voices running
  // plus a few queued, and put together in order.
  const AHEAD = 8;
  const clips = synthClips;
  const jobs = [];
  let done = 0;
  // The estimate goes by the characters spoken since the first spoken line
  // came back, so saved lines and loading the model don't skew it.
  let since = 0;
  let spokenChars = 0;
  const report = () => {
    let message = `Spoken ${done} of ${lines.length} lines${where()}`;
    if (spokenChars) {
      const left = lines.slice(done).reduce((n, l) => n + l.text.length, 0);
      const ms = ((performance.now() - since) / spokenChars) * left;
      message += `, at most about ${Math.ceil(ms / 60000)} min left`;
    }
    synthMessage(message);
  };
  const launch = (i) => {
    jobs[i] = speech(lines[i]).then((clip) => {
      done++;
      if (!clip.cached) {
        if (since) spokenChars += lines[i].text.length;
        else since = performance.now();
      }
      bar.value = done / lines.length;
      report();
      return clip;
    });
    // Handled when its turn comes; this keeps an early failure from being
    // reported as unhandled.
    jobs[i].catch(() => {});
  };
  try {
    report();
    for (let i = 0; i < Math.min(AHEAD, lines.length); i++) launch(i);
    for (const [i, line] of lines.entries()) {
      if (stopSynth) throw new DOMException('stopped', 'AbortError');
      const { samples } = await jobs[i];
      if (i + AHEAD < lines.length) launch(i + AHEAD);
      clips.push({ samples, speaker: line.speaker, segment: line.segment });
      live?.add(clips.at(-1));
    }
    const { track, starts } = assemble(clips);
    clearAudio();
    const blob = wav(track);
    state.audio = { blob, url: URL.createObjectURL(blob), starts, lines, episode: ep };
    $('audio').src = state.audio.url;
    synthMessage(`${formatTime(track.length / RATE)} of audio.`);
  } catch (err) {
    synthMessage(aborted(err) ? 'Stopped. The lines spoken so far are kept for next time.' : '');
    if (!aborted(err)) toast(`Making the audio failed: ${err.message}`);
  } finally {
    synthesizing = false;
    db.pruneClips(CLIP_ROOM).catch(() => {});
    bar.hidden = true;
    $('synthesize').hidden = false;
    $('synth-stop').hidden = true;
    $('synth-listen').hidden = true;
    renderPlayer();
  }
}

/** Where the voices run, once the browser engine knows. */
function where() {
  if ($('engine').value === 'server') return state.status?.tts.detail.startsWith('on this computer') ? ' on this computer' : ' on the speech server';
  return engines.browser.where ? ` ${engines.browser.where}` : '';
}

/**
 * Plays an episode's lines while they're still being made: each is queued
 * after the last, with the same pauses the finished track has.
 */
class Live {
  constructor() {
    this.ctx = new AudioContext({ sampleRate: RATE });
    this.at = 0;
    this.prev = null;
    this.sources = new Set();
  }

  add(clip) {
    const samples = trim(clip.samples);
    const buffer = this.ctx.createBuffer(1, samples.length, RATE);
    buffer.copyToChannel(samples, 0);
    const src = this.ctx.createBufferSource();
    src.buffer = buffer;
    src.connect(this.ctx.destination);
    // If making the audio fell behind, carry on from now.
    const at = Math.max(this.at + gap(this.prev, clip), this.ctx.currentTime + 0.05);
    src.start(at);
    src.onended = () => this.sources.delete(src);
    this.sources.add(src);
    this.at = at + buffer.duration;
    this.prev = clip;
  }

  stop() {
    for (const src of this.sources) src.stop();
    this.ctx.close();
  }
}

let live = null;

function stopLive() {
  live?.stop();
  live = null;
  $('synth-listen').textContent = 'Listen now';
}

function formatTime(s) {
  const t = Math.round(s);
  return `${Math.floor(t / 60)}:${String(t % 60).padStart(2, '0')}`;
}

function stopSynthesis() {
  stopSynth = true;
  engines.browser.cancel();
  engines.server.cancel();
  stopLive();
}

function clearAudio() {
  if (state.audio) URL.revokeObjectURL(state.audio.url);
  state.audio = null;
  $('audio').removeAttribute('src');
  synthMessage('');
  renderPlayer();
}

function renderPlayer() {
  const has = Boolean(state.audio);
  $('audio').hidden = !has;
  $('save-audio').disabled = !has;
  $('publish').disabled = !has || !state.status?.publish.ok;
}

let lineAudio;

async function playLine(line) {
  if (!line.text.trim()) return;
  try {
    const { samples } = await speech(line);
    synthMessage('');
    lineAudio?.pause();
    if (lineAudio) URL.revokeObjectURL(lineAudio.src);
    lineAudio = new Audio(URL.createObjectURL(wav(samples)));
    lineAudio.play();
  } catch (err) {
    toast(`Speaking the line failed: ${err.message}`);
  }
}

/** Marks the line being heard. */
function follow() {
  const a = state.audio;
  const items = [...$('script').querySelectorAll('.line')];
  items.forEach((li) => li.classList.remove('playing'));
  if (!a?.starts || a.episode !== state.episode || $('audio').paused) return;
  const t = $('audio').currentTime;
  const i = a.starts.findLastIndex((s) => s <= t);
  if (i < 0) return;
  const li = items[state.episode.lines.indexOf(a.lines[i])];
  li?.classList.add('playing');
}

function fileName(ep) {
  return (ep.title || 'episode').replace(/[\\/:*?"<>|]+/g, ' ').trim();
}

/** Publishes the audio on the player, as the episode it was made from. */
async function publish() {
  const { episode: ep, blob } = state.audio;
  const nb = state.notebook.id === ep.notebook ? state.notebook : await db.get('notebooks', ep.notebook);
  if (!nb || ep.deleted) {
    toast('That episode was deleted.');
    return;
  }
  const form = new FormData();
  form.set('podcast', nb.title);
  form.set('title', ep.title);
  form.set('description', ep.description);
  form.set('audio', blob, `${fileName(ep)}.wav`);
  $('publish').disabled = true;
  synthMessage('Publishing…');
  try {
    const res = await fetch('api/publish', { method: 'POST', body: form });
    const v = await res.json().catch(() => ({}));
    if (!res.ok) throw new Error(v.error || res.statusText);
    synthMessage(`Published to ${v.path}`);
  } catch (err) {
    synthMessage('');
    toast(`Publishing failed: ${err.message}`);
  } finally {
    renderPlayer();
  }
}

// ---------------------------------------------------------------- wiring

$('theme').addEventListener('click', () => {
  root.dataset.theme = isDark() ? 'light' : 'dark';
  try {
    localStorage.setItem(THEME_KEY, root.dataset.theme);
  } catch {}
  applyTheme();
});
darkScheme.addEventListener('change', applyTheme);

$('model').addEventListener('change', () => save(MODEL_KEY, model()));

$('new-notebook').addEventListener('click', async () => {
  $('notebook-menu').hidePopover();
  const title = prompt('Name the new notebook', 'Untitled notebook');
  if (title === null) return;
  const nb = await createNotebook(title.trim() || 'Untitled notebook');
  await openNotebook(nb.id);
});

$('rename-notebook').addEventListener('click', async () => {
  $('notebook-menu').hidePopover();
  const title = prompt('Rename the notebook', state.notebook.title);
  if (!title?.trim()) return;
  state.notebook.title = title.trim();
  await touch();
  renderNotebook();
});

$('delete-notebook').addEventListener('click', async () => {
  $('notebook-menu').hidePopover();
  if (!confirm(`Delete “${state.notebook.title}” with its sources and episodes?`)) return;
  const id = state.notebook.id;
  deletedNotebooks.add(id);
  for (const ep of known.values()) if (ep.notebook === id) forgetEpisode(ep);
  await db.deleteNotebook(id);
  await openNotebook(null);
});

$('notebook-title').addEventListener('dblclick', () => $('rename-notebook').click());

// Sources
$('discover').addEventListener('submit', discover);
$('topic').addEventListener('keydown', (e) => {
  if (e.key === 'Enter' && !e.shiftKey) {
    e.preventDefault();
    $('discover').requestSubmit();
  }
});
$('discover-stop').addEventListener('click', () => discovering?.abort());
$('add-candidates').addEventListener('click', addCandidates);
$('dismiss-candidates').addEventListener('click', () => ($('candidates').hidden = true));

$('add-url').addEventListener('submit', async (e) => {
  e.preventDefault();
  const url = $('url').value.trim();
  const button = $('add-url').querySelector('button');
  button.disabled = true;
  const notebook = state.notebook.id;
  try {
    await addSource(notebook, await fetchSource(url));
    $('url').value = '';
  } catch (err) {
    toast(`Couldn't add ${url}: ${err.message}`);
  } finally {
    button.disabled = false;
  }
});

let pasteNotebook;
$('paste-source').addEventListener('click', () => {
  $('paste-form').reset();
  // Escape keeps the last returnValue, which may be a past Add.
  $('paste').returnValue = '';
  pasteNotebook = state.notebook.id;
  $('paste').showModal();
});
$('paste').addEventListener('close', () => {
  if ($('paste').returnValue !== 'add') return;
  addSource(pasteNotebook, { title: $('paste-title').value.trim(), text: $('paste-text').value });
});

$('file-source').addEventListener('click', () => $('file').click());
$('file').addEventListener('change', async () => {
  await addFiles([...$('file').files]);
  $('file').value = '';
});
// Files can also be dropped on the sources pane.
$('sources').addEventListener('dragover', (e) => {
  if (e.dataTransfer.types.includes('Files')) e.preventDefault();
});
$('sources').addEventListener('drop', (e) => {
  if (!e.dataTransfer.files.length) return;
  e.preventDefault();
  addFiles([...e.dataTransfer.files].filter((f) => /\.(md|markdown|txt)$/i.test(f.name) || f.type.startsWith('text/')));
});

$('toggle-all').addEventListener('click', async () => {
  const on = !state.sources.every((s) => s.enabled);
  for (const s of state.sources) {
    s.enabled = on;
    await db.put('sources', s);
  }
  renderSources();
});

// Studio
fillVoices();
settings.restore();
for (const f of settings.fields) $(f).addEventListener('change', () => settings.save());
for (const f of ['host-a', 'host-b']) $(f).addEventListener('input', debounce(() => renderScript(), 300));

$('write-script').addEventListener('click', writeScript);
$('script-stop').addEventListener('click', () => writing?.abort());
$('episode-list').addEventListener('change', () => setEpisode(state.episodes.find((ep) => ep.id === $('episode-list').value)));
$('delete-episode').addEventListener('click', async () => {
  const ep = state.episode;
  if (!confirm(`Delete “${ep.title}”?`)) return;
  forgetEpisode(ep);
  state.episodes = state.episodes.filter((e) => e !== ep);
  setEpisode(state.episodes[0] ?? null);
  await db.del('episodes', ep.id);
});
$('episode-title').addEventListener('input', () => {
  state.episode.title = $('episode-title').value;
  saveEpisode(state.episode);
});
$('episode-description').addEventListener('input', () => {
  state.episode.description = $('episode-description').value;
  saveEpisode(state.episode);
});

$('synthesize').addEventListener('click', synthesize);
$('synth-stop').addEventListener('click', stopSynthesis);
for (const speaker of ['A', 'B']) {
  const h = speaker.toLowerCase();
  $(`hear-${h}`).addEventListener('click', () => {
    const name = $(`host-${h}`).value.trim() || `Host ${speaker}`;
    playLine({ speaker, text: `Hi, I'm ${name}. Welcome to the show. Today we're digging into something I've wanted to talk about for a while.` });
  });
}
$('synth-listen').addEventListener('click', () => {
  if (live) {
    stopLive();
    return;
  }
  $('audio').pause();
  live = new Live();
  for (const clip of synthClips) live.add(clip);
  $('synth-listen').textContent = 'Stop listening';
});
// Playing the finished track ends listening along.
$('audio').addEventListener('play', () => live && stopLive());
$('save-audio').addEventListener('click', () => {
  const a = el('a', { href: state.audio.url, download: `${fileName(state.audio.episode)}.wav` });
  a.click();
});
$('publish').addEventListener('click', publish);
for (const ev of ['timeupdate', 'pause', 'play', 'seeked']) $('audio').addEventListener(ev, follow);

window.addEventListener('beforeunload', (e) => {
  flushSaves();
  if (synthesizing || writing) e.preventDefault();
});
// The page may be closed without beforeunload once it's in the background.
document.addEventListener('visibilitychange', () => document.hidden && flushSaves());
window.addEventListener('pagehide', flushSaves);

applyTheme();
await openNotebook(load(CURRENT_KEY, null));
checkStatus();
db.pruneClips(CLIP_ROOM).catch(() => {});

// For the tests.
window.notebook = { state, openNotebook, addSource, stream };
