'use strict';

// The one file nginx accepts a PUT for.
const DATA_URL = 'data/apps.json';
const SVG = 'http://www.w3.org/2000/svg';

const ICONS = {
  link: 'M10 14a4 4 0 005.7 0l3-3A4 4 0 0013 5.3l-1 1M14 10a4 4 0 00-5.7 0l-3 3a4 4 0 005.7 5.7l1-1',
  book: 'M5 4h13v16H7a2 2 0 01-2-2zM5 18a2 2 0 012-2h11',
  chart: 'M4 20V4M4 20h16M8 16v-4M12 16V8M16 16v-6',
  chat: 'M4 5h16v11H9l-5 4z',
  code: 'M9 7l-5 5 5 5M15 7l5 5-5 5',
  download: 'M12 4v11M7 11l5 5 5-5M5 20h14',
  draw: 'M4 20l1-4L16 5l3 3L8 19zM14 7l3 3',
  folder: 'M3 6h6l2 2h10v11H3z',
  globe: 'M12 3a9 9 0 100 18 9 9 0 000-18zM3 12h18M12 3c3 3 3 15 0 18M12 3c-3 3-3 15 0 18',
  headphones: 'M4 15v-3a8 8 0 0116 0v3M4 14h3v6H4zM17 14h3v6h-3z',
  home: 'M4 11l8-7 8 7v9h-5v-6H9v6H4z',
  image: 'M4 5h16v14H4zM4 16l5-5 4 4 3-3 4 4M9 9h.01',
  kanban: 'M4 4h16v16H4zM9.5 4v16M14.5 4v16M6 8h1.5M11.5 8H13M16.5 8H18M6 11.5h1.5',
  music: 'M9 18V5l10-2v13M9 18a2.5 2.5 0 11-5 0 2.5 2.5 0 015 0zM19 16a2.5 2.5 0 11-5 0 2.5 2.5 0 015 0z',
  server: 'M4 4h16v6H4zM4 14h16v6H4zM7.5 7h.01M7.5 17h.01',
  settings: 'M4 7h10M18 7h2M4 17h2M10 17h10M16 5v4M8 15v4',
  shield: 'M12 3l7 3v5c0 5-3 8-7 10-4-2-7-5-7-10V6zM9 12l2 2 4-4',
  terminal: 'M4 5h16v14H4zM8 10l3 2-3 2M13 15h3',
  video: 'M3 5h18v12H3zM10 8.5l5 2.5-5 2.5zM8 21h8',
  write: 'M6 3h9l4 4v14H6zM14 3v5h5M9 12h7M9 16h5',
};

const $ = (id) => document.getElementById(id);

const groupsEl = $('groups');
const appDialog = $('app-dialog');
const urlInput = $('app-url');

let groups = []; // [{ name, apps: [{ name, description, url, icon }] }]
let saved = '[]'; // `groups` as last loaded or saved, to cancel back to
let editing = false;
let dragged = null; // { group, index } of the card being dragged
let target = null; // { group, index } the app dialog will write to

// ---------------------------------------------------------------- data

// Where an app's address leads, or null when it is not one we will link to.
// The file can be written by anyone who can reach the page, so only web
// addresses get through.
function resolve(url) {
  // A bare port is another service on this host.
  if (/^:\d{1,5}$/.test(url)) return `https://${location.hostname}${url}/`;
  if (/^\/(?!\/)/.test(url)) return url;
  try {
    const parsed = new URL(url);
    return ['http:', 'https:'].includes(parsed.protocol) ? parsed.href : null;
  } catch {
    return null;
  }
}

const text = (value, max) => (typeof value === 'string' ? value.trim().slice(0, max) : '');

// Reduce whatever the file held to the shape the page works with.
function clean(data) {
  const list = Array.isArray(data?.groups) ? data.groups : [];
  return list.map((group) => ({
    name: text(group?.name, 60),
    apps: (Array.isArray(group?.apps) ? group.apps : [])
      .map((app) => ({
        name: text(app?.name, 60),
        description: text(app?.description, 120),
        url: text(app?.url, 500),
        icon: Object.hasOwn(ICONS, app?.icon) ? app.icon : 'link',
      }))
      .filter((app) => app.name && resolve(app.url)),
  }));
}

const serialize = () => `${JSON.stringify({ groups }, null, 2)}\n`;

async function load() {
  try {
    const res = await fetch(DATA_URL, { cache: 'no-store' });
    if (!res.ok) throw new Error(res.statusText);
    groups = clean(await res.json());
    saved = JSON.stringify(groups);
  } catch {
    setMessage('Could not load the apps.');
  }
  render();
}

async function save() {
  if (JSON.stringify(groups) === saved) return true;
  try {
    const res = await fetch(DATA_URL, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: serialize(),
    });
    if (!res.ok) throw new Error(res.statusText);
    saved = JSON.stringify(groups);
    flashMessage('Saved.');
    return true;
  } catch {
    setMessage('Could not save. The changes are still here.');
    return false;
  }
}

// ---------------------------------------------------------------- status

function setMessage(msg) {
  $('message').textContent = msg;
}

// A message that clears itself.
function flashMessage(msg) {
  setMessage(msg);
  setTimeout(() => {
    if ($('message').textContent === msg) setMessage('');
  }, 4000);
}

// ---------------------------------------------------------------- render

function el(tag, props = {}, ...children) {
  const node = Object.assign(document.createElement(tag), props);
  node.append(...children);
  return node;
}

function icon(name) {
  const svg = document.createElementNS(SVG, 'svg');
  svg.setAttribute('viewBox', '0 0 24 24');
  svg.setAttribute('aria-hidden', 'true');
  const path = document.createElementNS(SVG, 'path');
  path.setAttribute('d', ICONS[name]);
  svg.append(path);
  return svg;
}

function button(label, title, onclick, disabled = false) {
  return el('button', { type: 'button', textContent: label, title, ariaLabel: title, onclick, disabled });
}

function card(app, g, i) {
  const link = el('a', { href: resolve(app.url) }, icon(app.icon), el('strong', { textContent: app.name }));
  if (app.description) link.append(el('span', { textContent: app.description }));
  const item = el('li', {}, link);
  if (!editing) return item;

  // While editing, a card opens its own settings in place of its address.
  link.title = `Edit ${app.name}`;
  link.addEventListener('click', (e) => {
    e.preventDefault();
    editApp(g, i);
  });
  const last = groups[g].apps.length - 1;
  item.append(el('div', { className: 'controls' },
    button('‹', `Move ${app.name} earlier`, () => move({ group: g, index: i }, { group: g, index: i - 1 }), i === 0),
    button('›', `Move ${app.name} later`, () => move({ group: g, index: i }, { group: g, index: i + 2 }), i === last),
    button('×', `Remove ${app.name}`, () => change(() => groups[g].apps.splice(i, 1))),
  ));
  item.draggable = true;
  item.addEventListener('dragstart', (e) => {
    dragged = { group: g, index: i };
    e.dataTransfer.effectAllowed = 'move';
    e.dataTransfer.setData('text/plain', app.name);
  });
  item.addEventListener('dragend', () => {
    dragged = null;
  });
  return item;
}

function section(group, g) {
  const list = el('ul', {}, ...group.apps.map((app, i) => card(app, g, i)));
  if (!editing) return el('section', {}, el('h2', { textContent: group.name }), list);

  const name = el('input', { type: 'text', value: group.name, maxLength: 60, placeholder: 'Group name', ariaLabel: 'Group name' });
  name.addEventListener('input', () => {
    group.name = name.value.trim();
  });
  const head = el('div', { className: 'group-head' },
    name,
    button('↑', 'Move group up', () => change(() => groups.splice(g - 1, 0, ...groups.splice(g, 1))), g === 0),
    button('↓', 'Move group down', () => change(() => groups.splice(g + 1, 0, ...groups.splice(g, 1))), g === groups.length - 1),
    button('Add app', 'Add an app to this group', () => editApp(g, -1)),
    button('Delete', group.apps.length ? 'Only an empty group can be deleted' : 'Delete this group',
      () => change(() => groups.splice(g, 1)), group.apps.length > 0),
  );
  if (!group.apps.length) list.append(el('li', { className: 'empty', textContent: 'Drop an app here, or add one.' }));

  // A card dropped on another takes its place; dropped elsewhere in the
  // list, it goes to the end.
  list.addEventListener('dragover', (e) => {
    if (dragged) e.preventDefault();
  });
  list.addEventListener('drop', (e) => {
    if (!dragged) return;
    e.preventDefault();
    const over = e.target.closest('li[draggable]');
    const index = over ? [...list.children].indexOf(over) : group.apps.length;
    move(dragged, { group: g, index });
  });
  return el('section', {}, head, list);
}

function render() {
  for (const id of ['add-group', 'cancel', 'done']) $(id).hidden = !editing;
  $('edit').hidden = editing;
  document.body.classList.toggle('editing', editing);
  const shown = editing ? groups : groups.filter((group) => group.apps.length);
  groupsEl.replaceChildren(...shown.map((group) => section(group, groups.indexOf(group))));
  if (!shown.length) groupsEl.append(el('p', { className: 'empty', textContent: 'No apps yet. Choose Edit to add one.' }));
}

// ---------------------------------------------------------------- editing

function change(mutate) {
  mutate();
  render();
}

// Take the app at `from` and put it before whatever is at `to`.
function move(from, to) {
  change(() => {
    const [app] = groups[from.group].apps.splice(from.index, 1);
    // Taking it out shifts everything after it in the same group.
    const index = from.group === to.group && from.index < to.index ? to.index - 1 : to.index;
    groups[to.group].apps.splice(index, 0, app);
  });
}

// `index` is -1 for an app that does not exist yet.
function editApp(group, index) {
  const app = groups[group].apps[index] ?? { name: '', description: '', url: '', icon: 'link' };
  target = { group, index };
  $('app-title').textContent = index < 0 ? 'Add app' : 'Edit app';
  $('app-name').value = app.name;
  $('app-description').value = app.description;
  urlInput.value = app.url;
  $('app-icon').value = app.icon;
  urlInput.setCustomValidity('');
  appDialog.returnValue = '';
  appDialog.showModal();
}

urlInput.addEventListener('input', () => {
  const ok = !urlInput.value.trim() || resolve(urlInput.value.trim());
  urlInput.setCustomValidity(ok ? '' : 'Use a path such as /app/, a port such as :8443, or an http(s) address.');
});

// Not a submit button, so that Enter in a field saves.
$('app-cancel').addEventListener('click', () => appDialog.close('cancel'));

appDialog.addEventListener('close', () => {
  if (appDialog.returnValue !== 'save') return;
  const app = {
    name: $('app-name').value.trim(),
    description: $('app-description').value.trim(),
    url: urlInput.value.trim(),
    icon: $('app-icon').value,
  };
  if (!app.name || !resolve(app.url)) return;
  const { apps } = groups[target.group];
  change(() => (target.index < 0 ? apps.push(app) : apps.splice(target.index, 1, app)));
});

$('edit').addEventListener('click', () => {
  editing = true;
  setMessage('');
  render();
});

$('add-group').addEventListener('click', () => {
  change(() => groups.push({ name: '', apps: [] }));
  groupsEl.querySelector('section:last-child input')?.focus();
});

$('cancel').addEventListener('click', () => {
  groups = JSON.parse(saved);
  editing = false;
  setMessage('');
  render();
});

$('done').addEventListener('click', async () => {
  // Stay in edit mode when the save fails, so nothing is lost.
  if (!(await save())) return;
  editing = false;
  render();
});

window.addEventListener('beforeunload', (e) => {
  if (editing && JSON.stringify(groups) !== saved) e.preventDefault();
});

// ---------------------------------------------------------------- init

$('host').textContent = location.hostname;
$('app-icon').append(...Object.keys(ICONS).map((name) => el('option', { value: name, textContent: name })));
load();
