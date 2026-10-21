// The notebooks, kept in this browser's IndexedDB.
//
// notebooks  {id, title, created, updated}
// sources    {id, notebook, title, url, siteName, text, enabled, added}
// episodes   {id, notebook, title, description, segments: [{heading}], lines: [{speaker, text, segment}], created}
// clips      {key, samples: Int16Array}: speech for one line, keyed by what made it
// clipinfo   {key, size, used}: each clip's size and when it was last used, so
//            the least recently used go once the clips outgrow their room

const NAME = 'notebook';
const VERSION = 2;

let opened;

function open() {
  opened ??= new Promise((resolve, reject) => {
    const req = indexedDB.open(NAME, VERSION);
    req.onupgradeneeded = (e) => {
      const db = req.result;
      if (e.oldVersion < 1) {
        db.createObjectStore('notebooks', { keyPath: 'id' });
        db.createObjectStore('sources', { keyPath: 'id' }).createIndex('notebook', 'notebook');
        db.createObjectStore('episodes', { keyPath: 'id' }).createIndex('notebook', 'notebook');
        db.createObjectStore('clips', { keyPath: 'key' });
      }
      if (e.oldVersion < 2) {
        // Clips from before have no clipinfo; they're only a cache.
        req.transaction.objectStore('clips').clear();
        db.createObjectStore('clipinfo', { keyPath: 'key' });
      }
    };
    req.onsuccess = () => resolve(req.result);
    req.onerror = () => reject(req.error);
  });
  return opened;
}

function done(req) {
  return new Promise((resolve, reject) => {
    req.onsuccess = () => resolve(req.result);
    req.onerror = () => reject(req.error);
  });
}

async function store(name, mode = 'readonly') {
  return (await open()).transaction(name, mode).objectStore(name);
}

export async function get(name, key) {
  return done((await store(name)).get(key));
}

export async function put(name, value) {
  return done((await store(name, 'readwrite')).put(value));
}

export async function del(name, key) {
  return done((await store(name, 'readwrite')).delete(key));
}

export async function all(name) {
  return done((await store(name)).getAll());
}

export async function byNotebook(name, notebook) {
  return done((await store(name)).index('notebook').getAll(notebook));
}

/** A cached clip, or undefined; using it keeps it from being pruned. */
export async function getClip(key) {
  const clip = await get('clips', key);
  if (clip) put('clipinfo', { key, size: clip.samples.byteLength, used: Date.now() }).catch(() => {});
  return clip;
}

export async function putClip(clip) {
  const tx = (await open()).transaction(['clips', 'clipinfo'], 'readwrite');
  tx.objectStore('clips').put(clip);
  tx.objectStore('clipinfo').put({ key: clip.key, size: clip.samples.byteLength, used: Date.now() });
  return committed(tx);
}

/** Deletes the least recently used clips until they fit in limit bytes. */
export async function pruneClips(limit) {
  const info = (await all('clipinfo')).sort((a, b) => a.used - b.used);
  let total = info.reduce((n, c) => n + c.size, 0);
  if (total <= limit) return;
  const tx = (await open()).transaction(['clips', 'clipinfo'], 'readwrite');
  for (const c of info) {
    if (total <= limit) break;
    tx.objectStore('clips').delete(c.key);
    tx.objectStore('clipinfo').delete(c.key);
    total -= c.size;
  }
  return committed(tx);
}

function committed(tx) {
  return new Promise((resolve, reject) => {
    tx.oncomplete = () => resolve();
    tx.onerror = () => reject(tx.error);
    tx.onabort = () => reject(tx.error);
  });
}

/** Deletes a notebook and everything in it. Clips are shared by key and left. */
export async function deleteNotebook(id) {
  const db = await open();
  const tx = db.transaction(['notebooks', 'sources', 'episodes'], 'readwrite');
  tx.objectStore('notebooks').delete(id);
  for (const name of ['sources', 'episodes']) {
    const s = tx.objectStore(name);
    for (const key of await done(s.index('notebook').getAllKeys(id))) s.delete(key);
  }
  return committed(tx);
}

export function uid() {
  return crypto.randomUUID();
}
