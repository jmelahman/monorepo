'use strict';

const DRAFT_KEY = 'text-editor:draft';
const MODE_KEY = 'text-editor:mode';
const THEME_KEY = 'text-editor:theme'; // also read by theme.js
const FOLDER_KEY = 'text-editor:folder';
const FILES_URL = '/files/'; // a folder of the server's, where it offers one
const DEVICE = Symbol('device'); // chosen over the server's files
const SETTLE_DELAY = 300;
const UNTITLED = 'untitled.md';
const RECENT_MAX = 8;
const REVISION_MAX = 50;
const REVISION_DELAY = 3 * 60 * 1000;
const FILE_TYPES = [{ description: 'Markdown', accept: { 'text/markdown': ['.md', '.markdown', '.txt'] } }];

const WELCOME = [
  '# Markdown Editor',
  '',
  'Write here as you would in a word processor. Nothing leaves your browser.',
  '',
  '## Formatting',
  '',
  '**Bold**, *italic*, ~~strikethrough~~, `code`, and [links](https://commonmark.org/help/).',
  '',
  '* Lists',
  '    * with nesting',
  '* [x] Task lists',
  '* [ ] with checkboxes',
  '',
  '> Block quotes',
  '',
  '```js',
  'const answer = 42;',
  '```',
  '',
  '| Tables | Too |',
  '| ------ | --: |',
  '| left | right |',
].join('\n');

const $ = (id) => document.getElementById(id);

const workspace = $('workspace');
const headingInput = $('heading');
const fileInput = $('file');
const discardDialog = $('discard-dialog');
const linkDialog = $('link-dialog');
const richDialog = $('rich-dialog');
const nameDialog = $('name-dialog');
const filesDialog = $('files-dialog');
const bubble = $('bubble');
const recentMenu = $('recent-menu');
const revisionsMenu = $('revisions-menu');
const menus = [...document.querySelectorAll('.menu')];
const root = document.documentElement;
const formatButtons = [...document.querySelectorAll('[data-format]')];
const coarsePointer = matchMedia('(pointer: coarse)');
const modeButtons = [...document.querySelectorAll('[data-mode]')];
const darkScheme = matchMedia('(prefers-color-scheme: dark)');

let editor = null; // toastui.Editor; rebuilt for every document
let mode = 'wysiwyg'; // the mode asked for, which a document can overrule
let fileName = UNTITLED;
let fileHandle = null; // File System Access handle, where the browser has one
let filePath = null; // where the file is among the server's, when it is one of them
let served = false; // whether the server has files to open and save in place
let savedText = ''; // Markdown as last opened or saved; null when unknown
let settleTimer = 0;
// The files last opened or saved, newest first: each a name with the handle
// to read it again, or with its text where the browser has no handles.
let recent = [];
let revisions = [];
let keptText = ''; // Markdown as it was before any change, or as last kept as a revision
let revisionTimer = 0;
let revisionQueue = Promise.resolve(); // revisions are stored one at a time
let heldSelection = null; // where the caret was when the editor lost the focus
let selecting = false; // a pointer is down in the document
let bubbleFrame = 0;
let htmlSources = new Map(); // an HTML block as rich mode holds it -> as it was written

// ---------------------------------------------------------------- sanitizer

const words = (s) => new Set(s.split(' '));
const SAFE_TAGS = words('a abbr address article aside b blockquote br caption center cite code col colgroup dd del details dfn div dl dt em figcaption figure footer h1 h2 h3 h4 h5 h6 header hr i img input ins kbd li main mark nav ol p pre q s samp section small span strong sub summary sup table tbody td tfoot th thead tr u ul var');
// Removed along with their contents; any other unknown tag keeps its children.
const DROP_TAGS = words('applet audio base button canvas embed frame frameset iframe link meta noscript object option script select style template textarea title video');
const SAFE_ATTRS = words('align alt colspan height open rowspan start title width');
// Drawings only: nothing that scripts, styles, embeds HTML, or refers elsewhere.
const SVG_TAGS = words('a circle desc ellipse g line path polygon polyline rect svg text title tspan');
const SVG_ATTRS = words('aria-hidden aria-label cx cy d dominant-baseline dx dy fill fill-opacity fill-rule font-family font-size font-style font-weight height letter-spacing opacity paint-order points preserveAspectRatio r role rx ry stroke stroke-dasharray stroke-dashoffset stroke-linecap stroke-linejoin stroke-opacity stroke-width text-anchor transform viewBox width x x1 x2 y y1 y2');

function safeUrl(url, schemes) {
  // Browsers ignore whitespace and control characters inside the scheme.
  const scheme = /^([a-z][a-z0-9+.-]*):/i.exec(url.replace(/[\0-\x20]/g, ''))?.[1].toLowerCase();
  return !scheme || schemes.includes(scheme);
}

// `strict` also drops the class and data-* attributes the editor works with.
function safeAttr(el, name, value, strict) {
  const tag = el.localName;
  if (name === 'href') return tag === 'a' && safeUrl(value, ['http', 'https', 'mailto', 'tel']);
  if (el instanceof SVGElement) {
    if (name === 'class') return !strict;
    // A paint can be a url(), which fetches.
    return SVG_ATTRS.has(name) && !/url\s*\(/i.test(value);
  }
  if (name === 'src') {
    return tag === 'img' && safeUrl(value, ['http', 'https', 'blob', 'data'])
      && (!/^\s*data:/i.test(value) || /^\s*data:image\/(?:png|jpeg|gif|webp|avif)[;,]/i.test(value));
  }
  if (name === 'class') return !strict || (tag === 'code' && /^lang(?:uage)?-\S+$/.test(value));
  if (name.startsWith('data-')) return !strict;
  if (name === 'type' || name === 'checked' || name === 'disabled') return tag === 'input';
  return SAFE_ATTRS.has(name);
}

function scrub(root, strict) {
  for (const node of [...root.childNodes]) {
    if (node instanceof Text) continue;
    // Forms go first: their named controls shadow the properties read below.
    const svg = node instanceof SVGElement;
    if (svg ? !SVG_TAGS.has(node.localName)
      : !(node instanceof HTMLElement) || node instanceof HTMLFormElement || DROP_TAGS.has(node.localName)) {
      root.removeChild(node);
      continue;
    }
    scrub(node, strict);
    const tag = node.localName;
    if (!svg && !SAFE_TAGS.has(tag)) {
      node.replaceWith(...node.childNodes);
      continue;
    }
    for (const { name, value } of [...node.attributes]) {
      if (!safeAttr(node, name, value, strict)) node.removeAttribute(name);
    }
    if (tag === 'a' && node.hasAttribute('href') && !node.getAttribute('href').startsWith('#')) {
      node.setAttribute('target', '_blank');
      node.setAttribute('rel', 'noopener noreferrer');
    }
    if (!svg && tag === 'input') {
      if (node.type === 'checkbox') node.disabled = true;
      else root.removeChild(node);
    }
  }
}

// Reduce untrusted HTML to allowlisted markup. The template keeps it inert
// (no scripts, no image loads) while it is cleaned.
function sanitize(html, strict = false) {
  const template = document.createElement('template');
  template.innerHTML = html;
  if (strict) {
    // The editor draws task boxes with CSS; a plain page needs real ones.
    for (const item of template.content.querySelectorAll('li[data-task]')) {
      const box = document.createElement('input');
      box.type = 'checkbox';
      box.checked = item.hasAttribute('data-task-checked');
      box.toggleAttribute('checked', box.checked);
      (item.firstElementChild?.localName === 'p' ? item.firstElementChild : item).prepend(box, ' ');
    }
    for (const code of template.content.querySelectorAll('pre > code[data-language]')) {
      code.className = `language-${code.dataset.language.replace(/\s/g, '')}`;
    }
  }
  scrub(template.content, strict);
  return template.innerHTML;
}

// ---------------------------------------------------------------- editor

// The vendored editor does the editing in both modes. Its own toolbar and
// mode switch are hidden in favor of the ones in index.html.
function createEditor(text) {
  buildEditor(text);
  if (mode === 'wysiwyg' && !losesHtml()) enterRich();
  showMode();
  updateHeading();
}

// Raw HTML that rich mode keeps: a block opened by one of these, shown as
// one uneditable block, and text inside one of the inline tags. Of any other
// tag it keeps only the text, apart from the few it has formatting for.
const HTML_BLOCKS = words('address article aside center details div figure footer header main nav section svg');
const HTML_INLINES = words('abbr cite dfn ins kbd mark q samp small sub sup u var');
const NATIVE_INLINES = words('b code del em i s strong');
// It rebuilds an inline tag from the attributes it knows, so only a tag
// that comes back the same is kept.
const INLINE_ATTRS = { a: ' href="[^"]*"', img: ' src="[^"]*"', abbr: '(?: title="[^"]*")?' };

// The tag as the sanitizer leaves it. The editor puts what the renderers
// below return into a live element before sanitizing it, where an image's
// error handler would already have run.
function cleanElement(tagName, attrs, html = '') {
  const template = document.createElement('template');
  template.innerHTML = `<${tagName}>${html}</${tagName}>`;
  const el = template.content.firstElementChild;
  for (const [name, value] of Object.entries(attrs ?? {})) {
    try {
      el.setAttribute(name, value);
    } catch {
      // Not a name an attribute can have.
    }
  }
  scrub(template.content, false);
  return el;
}

const attributesOf = (el) => Object.fromEntries([...el.attributes].map((a) => [a.name, a.value]));

const htmlBlock = (tagName) => (node) => {
  const el = cleanElement(tagName, node.attrs, node.childrenHTML);
  return [
    { type: 'openTag', tagName, outerNewLine: true, attributes: attributesOf(el) },
    { type: 'html', content: el.innerHTML },
    { type: 'closeTag', tagName, outerNewLine: true },
  ];
};

const htmlInline = (tagName) => (node, { entering }) => (entering
  ? { type: 'openTag', tagName, attributes: attributesOf(cleanElement(tagName, node.attrs)) }
  : { type: 'closeTag', tagName });

// Left to the editor, an SVG is rebuilt as an HTML element of that name,
// which draws nothing.
function svgView(node) {
  const dom = cleanElement('svg', node.attrs.htmlAttrs, node.attrs.childrenHTML);
  dom.classList.add('html-block');
  return { dom };
}

const renderers = (tags, renderer) => Object.fromEntries([...tags].map((tag) => [tag, renderer(tag)]));
const HTML_RENDERERS = {
  htmlBlock: renderers(HTML_BLOCKS, htmlBlock),
  htmlInline: renderers(HTML_INLINES, htmlInline),
  // The end of a line within a paragraph is a space, where the editor makes it a break.
  softbreak: () => ({ type: 'html', content: '\n' }),
};

// The editor rebuilds a block's opening tag and trims around its contents.
// A block cannot be edited in rich mode, so write back what was read.
function htmlToMarkdown({ node }, { origin }) {
  const source = node.attrs.htmlBlock && htmlSources.get(`${node.type.name}\n${node.attrs.childrenHTML}`);
  return source ? { text: source } : origin();
}

// Pasted pages are made of these tags; take the text, not frozen blocks.
function flattenPasted(html) {
  // Unless it was copied from here.
  if (html.includes('data-pm-slice')) return html;
  const template = document.createElement('template');
  template.innerHTML = html;
  const selector = [...HTML_BLOCKS].join(',');
  for (const el of [...template.content.querySelectorAll(selector)].reverse()) {
    if (el.localName === 'svg') {
      el.remove();
    } else if (el.querySelector('p,ul,ol,dl,table,pre,blockquote,hr,h1,h2,h3,h4,h5,h6')) {
      el.replaceWith(...el.childNodes);
    } else {
      const p = document.createElement('p');
      p.append(...el.childNodes);
      el.replaceWith(p);
    }
  }
  return template.innerHTML;
}

// Always in Markdown mode, where any document can be shown and the parsed
// tree is there to check before going on to rich mode.
// The editor takes every line for a paragraph of its own in rich mode, and a
// blank line for an empty one. Its conversions are replaced here with what
// Markdown means: lines run on into one paragraph, and a blank line ends it.
// A break forced within a paragraph has no place in rich mode, and starts a
// new paragraph there rather than being dropped.
function plainParagraphs({ toWwConvertors, toMdConvertors }) {
  toWwConvertors.paragraph = (state, node, { entering }, attrs) => {
    if (entering) state.openNode(state.schema.nodes.paragraph, attrs);
    else state.closeNode();
  };
  toWwConvertors.linebreak = toWwConvertors.softbreak;
  toWwConvertors.softbreak = (state) => state.addText(' ');
  toMdConvertors.nodeTypeConvertors.paragraph = (state, { node }) => {
    state.convertInline(node);
    if (!state.stopNewline && node.childCount) state.closeBlock(node);
  };
}

function buildEditor(text) {
  editor?.destroy();
  editor = null; // events fire while the next one is still being built
  heldSelection = null;
  editor = new toastui.Editor({
    el: $('editor'),
    height: '100%',
    initialValue: text,
    initialEditType: 'markdown',
    previewStyle: 'vertical',
    theme: isDark() ? 'dark' : 'light',
    hideModeSwitch: true,
    toolbarItems: [],
    autofocus: false,
    usageStatistics: false, // otherwise it reports the hostname to its authors
    // Replaces the bundled sanitizer, which is too old to trust.
    customHTMLSanitizer: (html) => sanitize(html),
    customHTMLRenderer: HTML_RENDERERS,
    customMarkdownRenderer: { html: htmlToMarkdown },
    frontMatter: true,
    events: { change: onChange, caretChange: updateHeading },
  });
  plainParagraphs(editor.convertor);
  editor.eventEmitter.listen('changeToolbarState', ({ toolbarState }) => {
    for (const b of formatButtons) {
      const state = toolbarState[TOOLBAR_STATES[b.dataset.format] ?? b.dataset.format];
      b.setAttribute('aria-pressed', String(!!state?.active));
    }
  });
  const { view } = editor.wwEditor;
  const { transformPastedHTML } = view.props;
  view.setProps({
    transformPastedHTML: (html, ...rest) => transformPastedHTML(flattenPasted(html), ...rest),
    nodeViews: { ...view.props.nodeViews, svg: svgView },
  });
}

// The blocks of raw HTML in the document, comments aside, each with what
// the editor will make of its contents: the block with every tag of its own
// name taken out, so one nested in another of the same name is not kept.
function htmlBlocks() {
  const blocks = [];
  const walker = editor.toastMark.getRootNode().walker();
  for (let event = walker.next(); event; event = walker.next()) {
    const { type, literal } = event.node;
    if (!event.entering || type !== 'htmlBlock' || /^\s*<!--/.test(literal)) continue;
    const tag = /^\s*<([a-z][a-z0-9]*)/i.exec(literal)?.[1].toLowerCase();
    const tags = new RegExp(`<\\s*${tag}[^>]*>|</${tag}\\s*>`, 'gi');
    const kept = HTML_BLOCKS.has(tag) && literal.match(tags).length === 2;
    blocks.push({ tag, literal, kept, children: literal.replace(tags, '').trim() });
  }
  // Nor can two that differ only in their opening tags be told apart later.
  for (const block of blocks) {
    const twin = blocks.find((other) => other.tag === block.tag && other.children === block.children);
    if (twin.literal !== block.literal) block.kept = twin.kept = false;
  }
  return blocks;
}

function inTable(node) {
  for (let parent = node.parent; parent; parent = parent.parent) {
    if (parent.type === 'tableCell') return true;
  }
  return false;
}

// Whether rich mode would write the document back without some of its HTML.
function losesHtml() {
  if (htmlBlocks().some((block) => !block.kept)) return true;
  const walker = editor.toastMark.getRootNode().walker();
  for (let event = walker.next(); event; event = walker.next()) {
    if (!event.entering || event.node.type !== 'htmlInline') continue;
    const { literal } = event.node;
    const tag = /^<\/?([a-z][a-z0-9]*)/i.exec(literal)?.[1].toLowerCase();
    // A line break is kept in a table, and elsewhere becomes a plain newline.
    if (tag === 'br' && inTable(event.node)) continue;
    if (!HTML_INLINES.has(tag) && !NATIVE_INLINES.has(tag) && !(tag in INLINE_ATTRS)) return true;
    if (!new RegExp(`^</?${tag}${literal[1] === '/' ? '' : INLINE_ATTRS[tag] ?? ''}>$`, 'i').test(literal)) return true;
  }
  return false;
}

// False when the document is one the editor cannot convert, such as an SVG
// with links in it, which makes it throw part way through.
function enterRich() {
  const text = editor.getMarkdown();
  htmlSources = new Map();
  for (const { tag, literal, children } of htmlBlocks()) {
    htmlSources.set(`${tag}\n${children}`, literal);
  }
  try {
    editor.changeMode('wysiwyg', true);
    return true;
  } catch {
    buildEditor(text);
    return false;
  }
}

function showMode() {
  const shown = editor.isWysiwygMode() ? 'wysiwyg' : 'markdown';
  for (const b of modeButtons) b.setAttribute('aria-checked', String(b.dataset.mode === shown));
  // Firefox marks only so many misspellings on a page, and the editor out of
  // sight can use them all up: in Markdown, the source of an SVG is full of them.
  editor.wwEditor.view.dom.spellcheck = shown === 'wysiwyg';
  editor.mdEditor.view.dom.spellcheck = shown === 'markdown';
}

// Why a document that was to open in rich mode did not.
function modeNote() {
  return mode === 'wysiwyg' && !editor.isWysiwygMode()
    ? 'Opened as Markdown, as Rich mode cannot keep everything in this document.'
    : '';
}

// Names the editor reports toolbar state under, where they differ from ours.
const TOOLBAR_STATES = { bold: 'strong', italic: 'emph' };

// Show the heading level at the caret in the block style menu.
function updateHeading() {
  if (!editor) return;
  let level = 0;
  if (editor.isWysiwygMode()) {
    const { $from } = editor.getCurrentModeEditor().view.state.selection;
    for (let depth = $from.depth; depth > 0 && !level; depth--) {
      const node = $from.node(depth);
      if (node.type.name === 'heading') level = node.attrs.level;
    }
  } else {
    const line = editor.getMarkdown().split('\n')[editor.getSelection()[0][0] - 1] ?? '';
    level = /^ {0,3}(#{1,6})(?:[ \t]|$)/.exec(line)?.[1].length ?? 0;
  }
  headingInput.value = level;
}

// Firefox moves the caret to the start of an editor that is handed the focus
// back, so the selection is noted on the way out and put back here.
function focusEditor() {
  editor.focus();
  if (heldSelection) editor.setSelection(...heldSelection);
  heldSelection = null;
}

function exec(command, payload) {
  focusEditor();
  editor.exec(command, payload);
  editor.focus();
}

// Resolves to true when some of the HTML can be given up for rich mode.
function confirmRich() {
  richDialog.returnValue = '';
  richDialog.showModal();
  return new Promise((resolve) => {
    richDialog.addEventListener('close', () => resolve(richDialog.returnValue === 'switch'), { once: true });
  });
}

async function setMode(name) {
  if (name === 'wysiwyg' && !editor.isWysiwygMode()) {
    if (losesHtml() && !(await confirmRich())) return;
    if (!enterRich()) return setMessage('Rich mode cannot show this document.');
  } else if (name === 'markdown' && editor.isWysiwygMode()) {
    editor.changeMode('markdown', true);
  }
  showMode();
  setMessage('');
  mode = name;
  try {
    localStorage.setItem(MODE_KEY, name);
  } catch {
    // Storage can be disabled; the mode just won't be remembered.
  }
}

function insertLink() {
  $('link-text').value = editor.getSelectedText();
  $('link-url').value = '';
  linkDialog.returnValue = '';
  linkDialog.showModal();
  ($('link-text').value ? $('link-url') : $('link-text')).focus();
}

// Not a submit button, so that Enter in either field inserts the link.
$('link-cancel').addEventListener('click', () => linkDialog.close('cancel'));

linkDialog.addEventListener('close', () => {
  if (linkDialog.returnValue === 'insert') {
    const linkUrl = $('link-url').value.trim();
    exec('addLink', { linkUrl, linkText: $('link-text').value || linkUrl });
  } else {
    focusEditor();
  }
});

function insertTable() {
  const from = editor.isWysiwygMode() ? editor.getSelection()[0] : null;
  exec('addTable', { rowCount: 3, columnCount: 3 });
  if (from === null) return;
  // The editor leaves the caret in the last cell; start in the first.
  const { doc } = editor.getCurrentModeEditor().view.state;
  let first = -1;
  doc.nodesBetween(from, doc.content.size, (node, pos) => {
    if (first < 0 && pos >= from && node.type.name === 'tableHeadCell') first = pos + 2;
    return first < 0;
  });
  if (first >= 0) editor.setSelection(first, first);
}

const FORMATS = { link: insertLink, table: insertTable };

// ---------------------------------------------------------------- bubble

const BUBBLE_GAP = 8;

// The box around the selected text, when the formatting bar belongs over it:
// the selection is in the editor, and so is the focus, or it is in the bar.
function selectedBox() {
  const dom = editor?.getCurrentModeEditor().view.dom;
  const selection = getSelection();
  const active = document.activeElement;
  if (selecting || !dom || !selection.rangeCount || selection.isCollapsed) return null;
  if (!dom.contains(selection.anchorNode) || !(dom.contains(active) || bubble.contains(active))) return null;
  const box = selection.getRangeAt(0).getBoundingClientRect();
  const frame = workspace.getBoundingClientRect();
  if (!box.width && !box.height) return null;
  return box.bottom < frame.top || box.top > frame.bottom ? null : box;
}

// Above the selection, or below it where there is no room, and on touch
// screens, where the system puts its own menu above.
function placeBubble() {
  bubbleFrame = 0;
  const box = selectedBox();
  bubble.hidden = !box;
  if (!box) return;
  const frame = workspace.getBoundingClientRect();
  const { width, height } = bubble.getBoundingClientRect();
  const above = box.top - height - BUBBLE_GAP;
  const below = Math.min(box.bottom + BUBBLE_GAP, frame.bottom - height - BUBBLE_GAP);
  const left = box.left + (box.width - width) / 2;
  bubble.style.top = `${above < frame.top || coarsePointer.matches ? below : above}px`;
  bubble.style.left = `${Math.max(BUBBLE_GAP, Math.min(left, innerWidth - width - BUBBLE_GAP))}px`;
}

function scheduleBubble() {
  bubbleFrame ||= requestAnimationFrame(placeBubble);
}

// ---------------------------------------------------------------- theme

// The chosen theme, which theme.js put on the root, or else the system's.
function isDark() {
  return root.dataset.theme ? root.dataset.theme === 'dark' : darkScheme.matches;
}

function applyTheme() {
  const dark = isDark();
  root.classList.toggle('dark', dark);
  $('editor').firstElementChild?.classList.toggle('toastui-editor-dark', dark);
  const label = `Switch to the ${dark ? 'light' : 'dark'} theme`;
  $('theme').title = label;
  $('theme').ariaLabel = label;
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

function updateStatus(text = editor.getMarkdown()) {
  const modified = text !== savedText;
  $('name').textContent = `${filePath ?? fileName}${modified ? ' •' : ''}`;
  document.title = `${modified ? '• ' : ''}${fileName} · Markdown Editor`;
  $('count').textContent = `${text.match(/\S+/g)?.length ?? 0} words · ${text.length} chars`;
}

// ---------------------------------------------------------------- files

function saveDraft(text = editor.getMarkdown()) {
  try {
    localStorage.setItem(DRAFT_KEY, JSON.stringify({ name: fileName, path: filePath, text, saved: text === savedText }));
  } catch {
    // Storage can be full or disabled; the draft is only a convenience.
  }
}

function loadDraft() {
  try {
    const draft = JSON.parse(localStorage.getItem(DRAFT_KEY));
    return typeof draft?.text === 'string' ? draft : null;
  } catch {
    return null;
  }
}

// Serializing the document is too slow to do on every keystroke.
function settle() {
  clearTimeout(settleTimer);
  settleTimer = 0;
  const text = editor.getMarkdown();
  updateStatus(text);
  saveDraft(text);
  trackRevision(text);
}

function onChange() {
  settleTimer ||= setTimeout(settle, SETTLE_DELAY);
}

// Replace the whole document. The editor is rebuilt so that its undo history
// can never bring one file's text back under another file's name.
function setDocument(text, name, handle = null, path = null) {
  keepRevision();
  createEditor(text);
  fileName = name;
  fileHandle = handle;
  filePath = path;
  // As the editor writes it back, which is not always how it was typed.
  savedText = editor.getMarkdown();
  keptText = savedText;
  settle();
  setMessage(modeNote());
}

// Resolves to true when the current document can be thrown away.
function confirmDiscard() {
  const text = editor.getMarkdown();
  if (text === savedText || !text.trim()) return Promise.resolve(true);
  $('discard-name').textContent = fileName;
  discardDialog.returnValue = '';
  discardDialog.showModal();
  return new Promise((resolve) => {
    discardDialog.addEventListener('close', () => resolve(discardDialog.returnValue === 'discard'), { once: true });
  });
}

const isText = (file) => !file.type || file.type.startsWith('text/') || /\.(?:md|markdown|txt)$/i.test(file.name);

async function loadFile(file, handle = null) {
  if (!isText(file)) return setMessage(`${file.name} is not a text file.`);
  try {
    const text = await file.text();
    setDocument(text, file.name, handle);
    remember(file.name, handle, text);
  } catch {
    setMessage(`Could not open ${file.name || 'file'}.`);
  }
}

async function openFile() {
  if (!(await confirmDiscard())) return;
  if (served) {
    const path = await browse(false);
    if (path !== DEVICE) return path ? openServed(path) : focusEditor();
  }
  if (!window.showOpenFilePicker) return fileInput.click();
  try {
    const [handle] = await window.showOpenFilePicker({ types: FILE_TYPES });
    await loadFile(await handle.getFile(), handle);
  } catch (err) {
    if (err.name !== 'AbortError') setMessage('Could not open the file.');
  }
}

async function openRecent(entry) {
  const { name, handle } = entry;
  if (!(await confirmDiscard())) return;
  if (entry.path) return openServed(entry.path);
  if (!handle) return setDocument(entry.text, name);
  try {
    // Access to a file ends with the visit, and is asked for again here.
    if (await handle.requestPermission({ mode: 'read' }) !== 'granted') return setMessage(`Not allowed to open ${name}.`);
    await loadFile(await handle.getFile(), handle);
  } catch (err) {
    if (err.name !== 'NotFoundError') return setMessage(`Could not open ${name}.`);
    setMessage(`${name} is no longer there.`);
    storeRecent(recent.filter((r) => r !== entry));
  }
}

async function newFile() {
  if (await confirmDiscard()) setDocument('', UNTITLED);
  editor.focus();
}

function download(text, name, type) {
  const url = URL.createObjectURL(new Blob([text], { type }));
  const a = document.createElement('a');
  a.href = url;
  a.download = name;
  a.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}

// Stands in for the browser's save dialog where it has none to offer.
// Resolves to the name to save under, or null.
function askName() {
  const input = $('save-name');
  input.value = fileName;
  nameDialog.returnValue = '';
  nameDialog.showModal();
  input.setSelectionRange(0, fileName.replace(/\.\w+$/, '').length);
  return new Promise((resolve) => {
    nameDialog.addEventListener('close', () => {
      const name = input.value.trim();
      resolve(nameDialog.returnValue === 'save' && name ? name : null);
    }, { once: true });
  });
}

// Not a submit button, so that Enter in the field saves.
$('name-cancel').addEventListener('click', () => nameDialog.close('cancel'));

// Writes back to the opened file where the server or the browser allows it,
// and falls back to a download elsewhere. `saveAs` always asks where to, as
// does a document that has not been named yet.
async function save(saveAs = false) {
  const text = editor.getMarkdown();
  const contents = text.endsWith('\n') ? text : `${text}\n`;
  let path = saveAs ? null : filePath;
  try {
    if (served && (saveAs || (!filePath && !fileHandle))) {
      path = await browse(true);
      if (!path) return focusEditor();
      if (path === DEVICE) [path, saveAs] = [null, true];
    }
    if (path) {
      await writeServed(path, contents);
      fileHandle = null;
      fileName = baseName(path);
    } else {
      if ((saveAs || !fileHandle) && window.showSaveFilePicker) {
        fileHandle = await window.showSaveFilePicker({ suggestedName: fileName, types: FILE_TYPES });
      } else if (saveAs || (!fileHandle && fileName === UNTITLED)) {
        const name = await askName();
        if (!name) return focusEditor();
        fileName = /\.\w+$/.test(name) ? name : `${name}.md`;
      }
      if (fileHandle) {
        const writable = await fileHandle.createWritable();
        await writable.write(contents);
        await writable.close();
        fileName = fileHandle.name;
      } else {
        download(contents, fileName, 'text/markdown');
      }
    }
    filePath = path;
    remember(fileName, fileHandle, contents, filePath);
    savedText = text;
    settle();
    keepRevision(text);
    flashMessage(`Saved ${filePath ?? fileName}.`);
  } catch (err) {
    if (err.name !== 'AbortError') setMessage(`Could not save ${path ?? fileName}.`);
  }
}

// ---------------------------------------------------------------- server

// The server can offer a folder of its own under FILES_URL: a listing in JSON
// for each folder in it, and Markdown files that can be read and written back.
// Files are then opened and saved there, which works in every browser.

const isMarkdown = (name) => /\.(?:md|markdown|txt)$/i.test(name);
const baseName = (path) => path.slice(path.lastIndexOf('/') + 1);
const fileUrl = (path) => FILES_URL + path.split('/').map(encodeURIComponent).join('/');

// The folders and Markdown files in a folder, which is '' or ends in a slash.
async function listFolder(folder) {
  const response = await fetch(fileUrl(folder), { cache: 'no-store' });
  if (!response.ok) throw new Error(response.statusText);
  return (await response.json())
    .filter(({ name, type }) => !name.startsWith('.') && (type === 'directory' || isMarkdown(name)))
    .sort((a, b) => (a.type === b.type ? a.name.localeCompare(b.name) : a.type === 'directory' ? -1 : 1));
}

async function readServed(path) {
  const response = await fetch(fileUrl(path), { cache: 'no-store' });
  if (!response.ok) throw new Error(response.statusText);
  return response.text();
}

async function writeServed(path, text) {
  const response = await fetch(fileUrl(path), { method: 'PUT', body: text });
  if (!response.ok) throw new Error(response.statusText);
}

async function openServed(path) {
  try {
    const text = await readServed(path);
    setDocument(text, baseName(path), null, path);
    remember(fileName, null, text, path);
  } catch {
    setMessage(`Could not open ${path}.`);
  }
  editor.focus();
}

// Shows a folder of the server's in the dialog. One that cannot be listed is
// left alone, or gives way to the top one where it was only remembered.
async function showFolder(folder, saving, remembered = false) {
  let entries;
  try {
    entries = await listFolder(folder);
  } catch {
    if (remembered && folder) showFolder('', saving);
    return;
  }
  filesDialog.dataset.folder = folder;
  $('files-path').textContent = `/${folder}`;
  const up = folder && { name: '..', type: 'directory', to: folder.replace(/[^/]+\/$/, '') };
  const items = [...(up ? [up] : []), ...entries].map((entry) => {
    const item = document.createElement('button');
    item.type = 'button';
    const isFolder = entry.type === 'directory';
    item.textContent = isFolder ? `${entry.name}/` : entry.name;
    item.addEventListener('click', () => {
      if (isFolder) showFolder(entry.to ?? `${folder}${entry.name}/`, saving);
      else if (saving) $('files-name').value = entry.name;
      else filesDialog.close(`${folder}${entry.name}`);
    });
    return item;
  });
  $('files-list').replaceChildren(...items);
  if (!saving) items[0]?.focus();
}

// Resolves to the path chosen among the server's files, to DEVICE when the
// browser's own way of opening or saving was asked for instead, or to null.
function browse(saving) {
  const input = $('files-name');
  let folder = filePath?.slice(0, filePath.lastIndexOf('/') + 1);
  try {
    folder ??= localStorage.getItem(FOLDER_KEY) ?? '';
  } catch {
    folder ??= '';
  }
  $('files-title').textContent = saving ? 'Save as' : 'Open';
  $('files-name-label').hidden = !saving;
  $('files-save').hidden = !saving;
  input.required = saving;
  input.value = fileName;
  $('files-list').replaceChildren();
  filesDialog.returnValue = '';
  filesDialog.showModal();
  if (saving) input.setSelectionRange(0, fileName.replace(/\.\w+$/, '').length);
  showFolder(folder, saving, true);
  return new Promise((resolve) => {
    filesDialog.addEventListener('close', () => {
      const { returnValue } = filesDialog;
      const name = input.value.trim().replaceAll('/', '');
      folder = filesDialog.dataset.folder ?? '';
      if (returnValue === 'device') return resolve(DEVICE);
      if (!returnValue || (saving && !name)) return resolve(null);
      try {
        localStorage.setItem(FOLDER_KEY, folder);
      } catch {
        // Storage is disabled; the folder just won't be remembered.
      }
      // The server only takes Markdown.
      resolve(saving ? `${folder}${isMarkdown(name) ? name : `${name}.md`}` : returnValue);
    }, { once: true });
  });
}

$('files-cancel').addEventListener('click', () => filesDialog.close(''));
$('files-device').addEventListener('click', () => filesDialog.close('device'));

// A file of the server's is shown as it is there now, whatever the draft
// held. Changes that had not been saved are kept as a revision instead.
async function initServed(draft) {
  try {
    await listFolder('');
    served = true;
  } catch {
    return; // the server has no files to offer
  }
  if (typeof draft?.path !== 'string') return;
  try {
    const text = await readServed(draft.path);
    if (!draft.saved) addRevision(draft.text);
    setDocument(text, baseName(draft.path), null, draft.path);
    if (!draft.saved && draft.text.trim() !== text.trim()) {
      setMessage('Reloaded from disk. The changes that were not saved are under Revisions in the File menu.');
    }
  } catch {
    // Gone, perhaps. The draft stays, to be saved back there.
    filePath = draft.path;
    updateStatus();
  }
}

// ---------------------------------------------------------------- recent

// IndexedDB, as it is the only storage able to hold a file handle, and one
// with the room for revisions.
function stateStore(mode) {
  return new Promise((resolve, reject) => {
    const open = indexedDB.open('text-editor', 1);
    open.onupgradeneeded = () => open.result.createObjectStore('state');
    open.onerror = () => reject(open.error);
    open.onsuccess = () => resolve(open.result.transaction('state', mode).objectStore('state'));
  });
}

const settled = (request) => new Promise((resolve, reject) => {
  request.onsuccess = () => resolve(request.result);
  request.onerror = () => reject(request.error);
});

// Fills a menu within the File menu, which is left out while it would be empty.
function listMenu(menu, entries, label, open, clear) {
  const last = document.createElement('button');
  last.textContent = 'Clear list';
  last.addEventListener('click', clear);
  const items = entries.map((entry) => {
    const item = document.createElement('button');
    item.textContent = label(entry);
    item.addEventListener('click', () => open(entry));
    return item;
  });
  for (const item of [...items, last]) {
    item.type = 'button';
    item.role = 'menuitem';
  }
  $(menu.getAttribute('aria-labelledby')).hidden = !entries.length;
  menu.replaceChildren(...items, document.createElement('hr'), last);
}

function showRecent() {
  listMenu(recentMenu, recent, (entry) => entry.path ?? entry.name, openRecent, () => storeRecent([]));
}

async function loadRecent() {
  try {
    recent = (await settled((await stateStore('readonly')).get('recent'))) ?? [];
  } catch {
    // Storage can be disabled; there is just no list then.
  }
  showRecent();
}

async function storeRecent(entries) {
  recent = entries;
  showRecent();
  try {
    await settled((await stateStore('readwrite')).put(recent, 'recent'));
  } catch {
    // The list just won't be remembered.
  }
}

// A file is the same as one in the list when its path or its handle says so,
// or, with only a copy to go on, when it has the same name.
async function remember(name, handle, text, path = null) {
  try {
    const same = await Promise.all(recent.map((r) => {
      if (path || r.path) return r.path === path;
      return handle && r.handle ? r.handle.isSameEntry(handle) : !handle && !r.handle && r.name === name;
    }));
    const entry = path ? { name, path } : handle ? { name, handle } : { name, text };
    await storeRecent([entry, ...recent.filter((_, i) => !same[i])].slice(0, RECENT_MAX));
  } catch {
    // A handle that cannot be compared is not worth keeping.
  }
}

// ---------------------------------------------------------------- revisions

function revisionLabel({ name, time }) {
  const date = new Date(time);
  const day = date.toDateString() === new Date().toDateString() ? {} : { day: 'numeric', month: 'short' };
  return `${date.toLocaleString(undefined, { ...day, hour: '2-digit', minute: '2-digit' })} · ${name}`;
}

function showRevisions() {
  listMenu(revisionsMenu, revisions, revisionLabel, restoreRevision, () => changeRevisions(() => []));
}

async function loadRevisions() {
  try {
    revisions = (await settled((await stateStore('readonly')).get('revisions'))) ?? [];
  } catch {
    // Storage can be disabled; there are just no revisions then.
  }
  showRevisions();
}

// Read and written in one go, as another tab may be keeping revisions too.
function changeRevisions(change) {
  revisionQueue = revisionQueue.then(async () => {
    try {
      const store = await stateStore('readwrite');
      revisions = change((await settled(store.get('revisions'))) ?? []);
      await settled(store.put(revisions, 'revisions'));
    } catch {
      // The revision just won't be kept.
    }
    showRevisions();
  });
}

function addRevision(text) {
  if (!text.trim()) return;
  const entry = { name: fileName, text, time: Date.now() };
  changeRevisions((list) => (list[0]?.text === text ? list : [entry, ...list].slice(0, REVISION_MAX)));
}

// Keeps the document as it is now, after the one it was changed from.
function keepRevision(text = editor.getMarkdown()) {
  clearTimeout(revisionTimer);
  revisionTimer = 0;
  if (text === keptText) return;
  addRevision(keptText);
  addRevision(text);
  keptText = text;
}

// A document that goes on changing is kept every few minutes. What it was
// before the first change is kept at once, in case the page is gone by then.
function trackRevision(text) {
  if (text === keptText || revisionTimer) return;
  addRevision(keptText);
  revisionTimer = setTimeout(keepRevision, REVISION_DELAY);
}

// The document being replaced becomes a revision itself, so nothing is lost.
function restoreRevision({ name, text }) {
  const [handle, path, saved] = name === fileName ? [fileHandle, filePath, savedText] : [null, null, null];
  setDocument(text, name, handle, path);
  savedText = saved;
  settle();
  editor.focus();
}

// ---------------------------------------------------------------- export

const escapeHtml = (s) => s.replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' })[c]);

const EXPORT_STYLE = `
  :root { color-scheme: light dark; }
  body { max-width: 46rem; margin: 0 auto; padding: 2rem 1.25rem; font: 1rem/1.65 system-ui, sans-serif; overflow-wrap: break-word; }
  h1, h2, h3, h4, h5, h6 { margin: 1.6em 0 0.6em; line-height: 1.25; }
  h1, h2 { padding-bottom: 0.3em; border-bottom: 1px solid #8884; }
  code, kbd, samp { padding: 0.1em 0.3em; border: 1px solid #8884; border-radius: 3px; font: 0.875em ui-monospace, monospace; }
  pre { padding: 0.875rem 1rem; overflow: auto; border: 1px solid #8884; border-radius: 3px; line-height: 1.5; }
  pre code { padding: 0; border: 0; }
  blockquote { margin-left: 0; padding-left: 1rem; border-left: 3px solid #8884; opacity: 0.8; }
  hr { border: 0; border-top: 1px solid #8884; margin: 2em 0; }
  img { max-width: 100%; height: auto; }
  table { display: block; max-width: 100%; overflow-x: auto; border-collapse: collapse; }
  th, td { padding: 0.375rem 0.75rem; border: 1px solid #8884; }
  th p, td p, li p { margin: 0; }
  li:has(> input, > p:first-child > input) { list-style: none; }
  li input { margin: 0 0.5em 0 -1.4em; }
`;

function exportHtml() {
  const body = sanitize(editor.getHTML(), true);
  const base = fileName.replace(/\.(?:md|markdown|txt)$/i, '');
  const title = /<h1[^>]*>(.*?)<\/h1>/s.exec(body)?.[1].replace(/<[^>]+>/g, '').trim() || escapeHtml(base);
  const page = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>${title}</title>
<style>${EXPORT_STYLE}</style>
</head>
<body>
${body}
</body>
</html>
`;
  download(page, `${base}.html`, 'text/html');
}

// Rich text for pasting into mail or a document, with the source as fallback.
async function copy() {
  try {
    await navigator.clipboard.write([
      new ClipboardItem({
        'text/html': new Blob([sanitize(editor.getHTML(), true)], { type: 'text/html' }),
        'text/plain': new Blob([editor.getMarkdown()], { type: 'text/plain' }),
      }),
    ]);
    flashMessage('Copied as rich text.');
  } catch {
    setMessage('Copying is not supported in this browser.');
  }
}

fileInput.addEventListener('change', () => {
  if (fileInput.files[0]) loadFile(fileInput.files[0]);
  fileInput.value = '';
});

// A dropped document is opened. Images and dragged text are left to the
// editor, which places them at the drop point.
const droppedDocument = (e) => {
  const item = [...(e.dataTransfer?.items ?? [])].find((i) => i.kind === 'file');
  return item && !item.type.startsWith('image/') ? item : null;
};

workspace.addEventListener('dragover', (e) => {
  if (!droppedDocument(e)) return;
  e.preventDefault();
  e.stopPropagation();
  workspace.classList.add('dragover');
}, true);
workspace.addEventListener('dragleave', () => workspace.classList.remove('dragover'), true);
workspace.addEventListener('drop', async (e) => {
  const item = droppedDocument(e);
  if (!item) return;
  e.preventDefault();
  e.stopPropagation();
  workspace.classList.remove('dragover');
  const file = item.getAsFile();
  // The handle is only on offer until the event handler first yields.
  const pending = item.getAsFileSystemHandle?.().catch(() => null);
  if (!file || !(await confirmDiscard())) return;
  const handle = await pending;
  loadFile(file, handle?.kind === 'file' ? handle : null);
}, true);

// ---------------------------------------------------------------- controls

$('new').addEventListener('click', newFile);
$('open').addEventListener('click', openFile);
$('save').addEventListener('click', () => save());
$('save-as').addEventListener('click', () => save(true));
$('export').addEventListener('click', exportHtml);
$('copy').addEventListener('click', copy);

for (const b of formatButtons) {
  b.addEventListener('click', () => (FORMATS[b.dataset.format] ?? exec)(b.dataset.format));
}
headingInput.addEventListener('change', () => exec('heading', { level: Number(headingInput.value) }));

for (const b of modeButtons) {
  b.addEventListener('click', async () => {
    await setMode(b.dataset.mode);
    updateHeading();
    heldSelection = null;
    editor.focus();
  });
}

// A menu sits in the top layer, so it is placed by hand before it is shown:
// under its button, or beside it for a menu within a menu that has the room.
for (const menu of menus) {
  const button = $(menu.getAttribute('aria-labelledby'));
  const parent = button.closest('.menu');
  menu.addEventListener('beforetoggle', (e) => {
    const open = e.newState === 'open';
    button.setAttribute('aria-expanded', String(open));
    if (!open) return;
    const { left, right, top, bottom } = button.getBoundingClientRect();
    const beside = parent && right + parent.offsetWidth < innerWidth;
    menu.style.left = `${beside ? right + 4 : left}px`;
    menu.style.top = `${beside ? top - 5 : bottom + 4}px`;
  });
  menu.addEventListener('toggle', (e) => {
    if (e.newState === 'open') menu.querySelector('button').focus();
  });
  menu.addEventListener('click', (e) => {
    const item = e.target.closest('button');
    if (item && !item.popoverTargetElement) menu.hidePopover();
  });
  menu.addEventListener('keydown', (e) => {
    const active = document.activeElement;
    const step = { ArrowDown: 1, ArrowUp: -1 }[e.key];
    if (step) {
      const items = [...menu.querySelectorAll(':scope > button:not([hidden])')];
      items[(items.indexOf(active) + step + items.length) % items.length].focus();
    } else if (e.key === 'ArrowRight' && active.popoverTargetElement) {
      active.popoverTargetElement.showPopover();
    } else if (e.key === 'ArrowLeft' && parent) {
      menu.hidePopover();
      button.focus();
    } else {
      return;
    }
    e.preventDefault();
    e.stopPropagation();
  });
}
// Another tab may have opened files or kept revisions since.
$('file-menu').addEventListener('beforetoggle', (e) => {
  if (e.newState !== 'open') return;
  loadRecent();
  loadRevisions();
});

// Keeps the selection in the document when a formatting button is pressed.
for (const bar of [bubble, $('insert')]) {
  bar.addEventListener('mousedown', (e) => {
    if (e.target.closest('button')) e.preventDefault();
  });
}
workspace.addEventListener('pointerdown', () => {
  heldSelection = null;
  selecting = true;
  scheduleBubble();
});
workspace.addEventListener('keydown', () => {
  heldSelection = null;
}, true);
workspace.addEventListener('focusout', () => {
  if (editor) heldSelection = editor.getSelection();
}, true);
for (const type of ['pointerup', 'pointercancel']) {
  document.addEventListener(type, () => {
    selecting = false;
    scheduleBubble();
  });
}
for (const type of ['selectionchange', 'focusin', 'focusout']) document.addEventListener(type, scheduleBubble);
workspace.addEventListener('scroll', scheduleBubble, true);
window.addEventListener('resize', scheduleBubble);

$('theme').addEventListener('click', () => {
  const theme = isDark() ? 'light' : 'dark';
  root.dataset.theme = theme;
  try {
    localStorage.setItem(THEME_KEY, theme);
  } catch {
    // Storage can be disabled; the theme just won't be remembered.
  }
  applyTheme();
});
darkScheme.addEventListener('change', applyTheme);

// Capturing, as the editor would otherwise take Ctrl+S for strikethrough.
document.addEventListener('keydown', (e) => {
  if (e.key === 'Escape') bubble.hidden = true;
  if (discardDialog.open || linkDialog.open || richDialog.open || nameDialog.open || filesDialog.open || e.altKey || !(e.ctrlKey || e.metaKey)) return;
  const key = e.key.toLowerCase();
  const action = { s: () => save(e.shiftKey), o: openFile, k: insertLink }[key];
  if (!action || (key !== 's' && e.shiftKey)) return;
  e.preventDefault();
  e.stopPropagation();
  action();
}, true);

// Links are inert while editing, but can still be opened in a new tab.
for (const type of ['click', 'auxclick']) {
  workspace.addEventListener(type, (e) => {
    const href = e.target.closest?.('a')?.getAttribute('href');
    if (href && !safeUrl(href, ['http', 'https', 'mailto', 'tel'])) e.preventDefault();
  }, true);
}

// Changes settle on a delay, so flush them before the page goes away.
document.addEventListener('visibilitychange', () => {
  if (document.hidden && settleTimer) settle();
});
window.addEventListener('pagehide', () => {
  if (settleTimer) settle();
});

// ---------------------------------------------------------------- init

{
  const draft = loadDraft();
  let stored = null;
  try {
    stored = localStorage.getItem(MODE_KEY);
  } catch {
    // Storage is disabled.
  }
  if (modeButtons.some((b) => b.dataset.mode === stored)) mode = stored;
  applyTheme();
  loadRecent();
  loadRevisions();
  createEditor(draft ? draft.text : WELCOME);
  keptText = editor.getMarkdown();
  setMessage(modeNote());
  if (draft?.name) fileName = String(draft.name);
  savedText = !draft || draft.saved ? editor.getMarkdown() : null;
  updateStatus();
  initServed(draft);
}
