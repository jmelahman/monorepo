'use strict';

const DRAFT_KEY = 'text-editor:draft';
const VIEW_KEY = 'text-editor:view';
const DRAFT_DELAY = 400;
const INDENT = '    '; // wide enough to nest under both "- " and "10. "
const FILE_TYPES = [{ description: 'Markdown', accept: { 'text/markdown': ['.md', '.markdown', '.txt'] } }];

const WELCOME = [
  '# Markdown Editor',
  '',
  'Write on the left, read on the right. Nothing leaves your browser.',
  '',
  '## Formatting',
  '',
  '**Bold**, *italic*, ~~strikethrough~~, `code`, and [links](https://commonmark.org/help/).',
  '',
  '- Lists',
  '    - with nesting',
  '- [x] Task lists',
  '- [ ] with checkboxes',
  '',
  '> Block quotes',
  '',
  '```js',
  'const answer = 42;',
  '```',
  '',
  '| Tables | Too |',
  '| ------ | --: |',
  '| left   | right |',
  '',
].join('\n');

const $ = (id) => document.getElementById(id);

const workspace = $('workspace');
const editor = $('editor');
const preview = $('preview'); // scrolls; `doc` inside it holds the rendered page
const doc = $('doc');
const headingInput = $('heading');
const fileInput = $('file');
const discardDialog = $('discard-dialog');
const viewButtons = [...document.querySelectorAll('[data-view]')];

let fileName = 'untitled.md';
let fileHandle = null; // File System Access handle, where the browser has one
let savedText = ''; // contents as last opened or saved; null when unknown
let renderQueued = false;
let draftTimer = 0;
let scrollLock = null; // pane being scrolled by the other one
let tabOut = false; // Esc was pressed: let the next Tab leave the editor

// ---------------------------------------------------------------- markdown

const RE = {
  fence: /^( {0,3})(`{3,}|~{3,})[ \t]*([^`]*?)[ \t]*$/,
  heading: /^ {0,3}(#{1,6})(?:[ \t]+|$)(.*)$/,
  setext: /^ {0,3}(=+|-+)[ \t]*$/,
  hr: /^ {0,3}([-*_])(?:[ \t]*\1){2,}[ \t]*$/,
  quote: /^ {0,3}> ?/,
  item: /^( {0,3})([-+*]|\d{1,9}[.)])( +|$)/,
  task: /^\[([ xX])\][ \t]+/,
  tableRule: /^ {0,3}\|?[ \t]*:?-+:?[ \t]*(?:\|[ \t]*:?-+:?[ \t]*)*\|?[ \t]*$/,
  htmlBlock: /^ {0,3}<(?:\/?(?:address|article|aside|blockquote|center|dd|details|div|dl|dt|figcaption|figure|footer|h[1-6]|header|hr|li|main|nav|ol|p|pre|section|summary|table|tbody|td|tfoot|th|thead|tr|ul)(?:[\s/>]|$)|!--)/i,
  ref: /^ {0,3}\[([^\]^][^\]]*)\]:[ \t]*<?([^\s>]+)>?(?:[ \t]+(?:"([^"]*)"|'([^']*)'|\(([^)]*)\)))?[ \t]*$/,

  code: /(?<![`\\])(`+)(?!`)([^]+?)(?<!`)\1(?!`)/g,
  autolink: /<([a-z][a-z0-9+.-]{1,31}:[^\s<>]*)>/gi,
  email: /<([\w.+-]+@[\w-]+(?:\.[\w-]+)+)>/g,
  tag: /<\/?[a-zA-Z][\w-]*(?:\s[^<>]*)?\/?>|<!--[^]*?-->/g,
  escape: /\\([!-/:-@[-`{-~])/g,
  link: /(!?)\[((?:[^[\]]|\[[^[\]]*\])*)\]\([ \t\n]*((?:[^\s()]|\([^\s()]*\))*)(?:[ \t\n]+(?:"([^"]*)"|'([^']*)'))?[ \t\n]*\)/g,
  refLink: /(!?)\[((?:[^[\]]|\[[^[\]]*\])+)\](?:\[([^[\]]*)\])?/g,
  url: /(?<![\w/])https?:\/\/[^\s<\uE000\uE001]+/g,
  held: /\uE000(\d+)\uE001/g,
};

const escapeHtml = (s) => s.replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' })[c]);
// Like escapeHtml, but leaves quotes and existing entities alone.
const escapeText = (s) => s.replace(/&(?!#?\w+;)/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
const refKey = (label) => label.trim().replace(/\s+/g, ' ').toLowerCase();
const codeBlock = (lines, lang = '') =>
  `<pre><code${lang ? ` class="language-${escapeHtml(lang)}"` : ''}>${escapeHtml(lines.map((l) => `${l}\n`).join(''))}</code></pre>\n`;

// Markdown source to HTML. Raw HTML passes straight through, so the result
// must go through sanitize() before it is shown.
function renderMarkdown(src) {
  const lines = src
    .replace(/\r\n?/g, '\n')
    .replace(/[\0\uE000\uE001]/g, '') // the private-use pair marks held spans in inline()
    .split('\n')
    .map((line) => line.replace(/^[ \t]+/, (ws) => ws.replace(/\t/g, '    ')));

  // Link reference definitions can come after their use, so collect them first.
  const refs = {};
  let fence = null;
  lines.forEach((line, i) => {
    const f = RE.fence.exec(line);
    if (fence) {
      if (f && !f[3] && f[2][0] === fence[0] && f[2].length >= fence.length) fence = null;
    } else if (f) {
      fence = f[2];
    } else {
      const m = RE.ref.exec(line);
      if (!m) return;
      refs[refKey(m[1])] ??= { url: escapeText(m[2]), title: escapeText(m[3] ?? m[4] ?? m[5] ?? '') };
      lines[i] = '';
    }
  });
  return blocks(lines, refs);
}

// Can this line cut a paragraph short without a blank line before it?
function interruptsParagraph(line) {
  if (RE.fence.test(line) || RE.heading.test(line) || RE.hr.test(line) || RE.quote.test(line) || RE.htmlBlock.test(line)) {
    return true;
  }
  const m = RE.item.exec(line);
  return !!m && /^(?:[-+*]|1[.)])$/.test(m[2]) && line.slice(m[0].length).trim() !== '';
}

function tableCells(row) {
  return row
    .trim()
    .replace(/^\|/, '')
    .replace(/(?<!\\)\|$/, '')
    .split(/(?<!\\)\|/)
    .map((cell) => cell.trim().replace(/\\\|/g, '|'));
}

// `tight` is for the inside of a tight list item, whose paragraphs go unwrapped.
function blocks(lines, refs, tight = false) {
  let out = '';
  let i = 0;
  while (i < lines.length) {
    const line = lines[i];
    let m;

    if (!line.trim()) {
      i++;
    } else if (/^ {4}/.test(line)) {
      const body = [];
      for (; i < lines.length && (/^ {4}/.test(lines[i]) || !lines[i].trim()); i++) body.push(lines[i].slice(4));
      while (!body.at(-1).trim()) body.pop();
      out += codeBlock(body);
    } else if ((m = RE.fence.exec(line))) {
      const [, indent, mark, info] = m;
      const close = new RegExp(`^ {0,3}${mark[0]}{${mark.length},}[ \\t]*$`);
      const strip = new RegExp(`^ {0,${indent.length}}`);
      const body = [];
      for (i++; i < lines.length && !close.test(lines[i]); i++) body.push(lines[i].replace(strip, ''));
      i++;
      out += codeBlock(body, info.split(/\s+/)[0]);
    } else if ((m = RE.heading.exec(line))) {
      const text = m[2].replace(/(?:^|[ \t]+)#+[ \t]*$/, '').trim();
      out += `<h${m[1].length}>${inline(text, refs)}</h${m[1].length}>\n`;
      i++;
    } else if (RE.hr.test(line)) {
      out += '<hr>\n';
      i++;
    } else if (RE.quote.test(line)) {
      const body = [];
      for (; i < lines.length; i++) {
        if (RE.quote.test(lines[i])) body.push(lines[i].replace(RE.quote, ''));
        // A paragraph inside the quote may run on without repeating the marker.
        else if (lines[i].trim() && body.at(-1).trim() && !interruptsParagraph(lines[i])) body.push(lines[i]);
        else break;
      }
      out += `<blockquote>\n${blocks(body, refs)}</blockquote>\n`;
    } else if (RE.item.test(line)) {
      let html;
      [html, i] = list(lines, i, refs);
      out += html;
    } else if (RE.htmlBlock.test(line)) {
      const start = i;
      if (/^ {0,3}<!--/.test(line)) {
        while (i < lines.length && !lines[i].includes('-->')) i++;
        i++;
      } else {
        while (i < lines.length && lines[i].trim()) i++;
      }
      out += `${lines.slice(start, i).join('\n')}\n`;
    } else if (line.includes('|') && lines[i + 1]?.includes('|') && RE.tableRule.test(lines[i + 1])
      && tableCells(line).length === tableCells(lines[i + 1]).length) {
      const head = tableCells(line);
      const align = tableCells(lines[i + 1]).map((rule) => {
        const side = (rule.startsWith(':') ? 1 : 0) + (rule.endsWith(':') ? 2 : 0);
        return ['', ' align="left"', ' align="right"', ' align="center"'][side];
      });
      const row = (cells, tag) =>
        `<tr>${head.map((_, c) => `<${tag}${align[c]}>${inline(cells[c] ?? '', refs)}</${tag}>`).join('')}</tr>\n`;
      out += `<table>\n<thead>\n${row(head, 'th')}</thead>\n<tbody>\n`;
      for (i += 2; i < lines.length && lines[i].trim() && !interruptsParagraph(lines[i]); i++) {
        out += row(tableCells(lines[i]), 'td');
      }
      out += '</tbody>\n</table>\n';
    } else {
      const para = [line];
      let level = 0;
      for (i++; i < lines.length && lines[i].trim(); i++) {
        if ((m = RE.setext.exec(lines[i]))) {
          level = m[1][0] === '=' ? 1 : 2;
          i++;
          break;
        }
        if (interruptsParagraph(lines[i])) break;
        para.push(lines[i]);
      }
      const text = inline(para.map((l) => l.trimStart()).join('\n').trimEnd(), refs);
      if (level) out += `<h${level}>${text}</h${level}>\n`;
      else out += tight ? `${text}\n` : `<p>${text}</p>\n`;
    }
  }
  return out;
}

// Parse the list starting at lines[i]. Returns its HTML and the index of the
// first line after it.
function list(lines, i, refs) {
  const first = RE.item.exec(lines[i])[2];
  const ordered = first.length > 1;
  const items = [];
  let loose = false;
  let gap = false; // blank lines ended the previous item

  while (i < lines.length) {
    const m = RE.item.exec(lines[i]);
    // A different bullet or delimiter starts a new list.
    if (!m || m[2].at(-1) !== first.at(-1) || m[2].length > 1 !== ordered || RE.hr.test(lines[i])) break;
    if (gap) loose = true;

    // Continuation lines belong to the item when indented to its content column.
    const rest = lines[i].slice(m[0].length);
    const width = m[1].length + m[2].length + (rest.trim() && m[3].length <= 4 ? m[3].length : 1);
    const body = [lines[i].slice(width)];
    for (i++; i < lines.length; i++) {
      const l = lines[i];
      if (!l.trim()) body.push('');
      else if (l.search(/\S/) >= width) body.push(l.slice(width));
      else if (body.at(-1).trim() && !interruptsParagraph(l) && !RE.item.test(l)) body.push(l);
      else break;
    }
    gap = false;
    while (body.length > 1 && !body.at(-1).trim()) {
      body.pop();
      gap = true;
    }

    // Blank lines between an item's own blocks also make the list loose.
    let fenced = false;
    body.forEach((l, n) => {
      if (RE.fence.test(l)) fenced = !fenced;
      else if (n && !fenced && !l.trim()) loose = true;
    });

    const task = RE.task.exec(body[0]);
    if (task) body[0] = body[0].slice(task[0].length);
    items.push({ body, task });
  }

  const tag = ordered ? 'ol' : 'ul';
  const start = ordered && parseInt(first, 10) !== 1 ? ` start="${parseInt(first, 10)}"` : '';
  const html = items.map(({ body, task }) => {
    let inner = blocks(body, refs, !loose).trim();
    if (task) {
      const box = `<input type="checkbox" disabled${task[1] === ' ' ? '' : ' checked'}> `;
      inner = inner.startsWith('<p>') ? `<p>${box}${inner.slice(3)}` : box + inner;
    }
    return `<li>${inner}</li>\n`;
  });
  return [`<${tag}${start}>\n${html.join('')}</${tag}>\n`, i];
}

// Inline markup for one block of text. Finished spans are swapped for
// placeholders ("held") so that later passes cannot reach inside them.
function inline(src, refs) {
  const held = [];
  const hold = (html) => `\uE000${held.push(html) - 1}\uE001`;
  // For attribute values: held markup is dropped, held escaped characters stay.
  const attr = (s) => s.replace(RE.held, (all, n) => (held[n].includes('<') ? '' : all)).replace(/"/g, '&quot;');

  const emphasis = (s) => s
    .replace(/(\*\*\*|(?<!\w)___)(?=\S)([^]+?)(?<=\S)\1/g, '<strong><em>$2</em></strong>')
    .replace(/\*\*(?=\S)([^]+?)(?<=\S)\*\*(?!\*)/g, '<strong>$1</strong>')
    .replace(/(?<!\w)__(?=\S)([^]+?)(?<=\S)__(?!\w)/g, '<strong>$1</strong>')
    .replace(/(?<!\*)\*(?=[^\s*])([^*]+?)(?<=\S)\*(?!\*)/g, '<em>$1</em>')
    .replace(/(?<!\w)_(?=[^\s_])([^]+?)(?<=\S)_(?!\w)/g, '<em>$1</em>')
    .replace(/~~(?=\S)([^]+?)(?<=\S)~~/g, '<del>$1</del>');

  const media = (bang, text, url, title) => {
    const tip = title ? ` title="${attr(title)}"` : '';
    if (bang) return hold(`<img src="${attr(url)}" alt="${attr(text)}"${tip}>`);
    return hold(`<a href="${attr(url)}"${tip}>${emphasis(links(text))}</a>`);
  };

  const links = (s) => s
    .replace(RE.link, (_, bang, text, url, t1, t2) => media(bang, text, url, t1 ?? t2))
    .replace(RE.refLink, (all, bang, text, label) => {
      const ref = refs[refKey(label || text)];
      return ref ? media(bang, text, ref.url, ref.title) : all;
    });

  let s = src
    .replace(RE.code, (_, ticks, code) => {
      code = code.replace(/\n/g, ' ');
      if (/^ .* $/.test(code) && code.trim()) code = code.slice(1, -1);
      return hold(`<code>${escapeHtml(code)}</code>`);
    })
    .replace(RE.autolink, (_, url) => hold(`<a href="${escapeHtml(url)}">${escapeHtml(url)}</a>`))
    .replace(RE.email, (_, address) => hold(`<a href="mailto:${escapeHtml(address)}">${escapeHtml(address)}</a>`))
    .replace(RE.tag, (tag) => hold(tag))
    .replace(RE.escape, (_, c) => hold(escapeHtml(c)));

  s = links(escapeText(s)).replace(RE.url, (all) => {
    let url = all.replace(/[.,:;!?'"*_~]+$/, '');
    if (url.endsWith(')') && url.split('(').length < url.split(')').length) url = url.slice(0, -1);
    return hold(`<a href="${attr(url)}">${url}</a>`) + all.slice(url.length);
  });
  s = emphasis(s).replace(/(?: {2,}|\\)\n/g, '<br>\n');

  while (s.includes('\uE000')) s = s.replace(RE.held, (_, n) => held[n]);
  return s;
}

// ---------------------------------------------------------------- sanitizer

const words = (s) => new Set(s.split(' '));
const SAFE_TAGS = words('a abbr b blockquote br caption center code col colgroup dd del details div dl dt em figcaption figure h1 h2 h3 h4 h5 h6 hr i img input ins kbd li mark ol p pre q s samp small span strong sub summary sup table tbody td tfoot th thead tr u ul var');
// Removed along with their contents; any other unknown tag keeps its children.
const DROP_TAGS = words('applet audio base button canvas embed frame frameset iframe link meta noscript object option script select style template textarea title video');
const SAFE_ATTRS = words('align alt colspan height open rowspan start title width');

function safeUrl(url, schemes) {
  // Browsers ignore whitespace and control characters inside the scheme.
  const scheme = /^([a-z][a-z0-9+.-]*):/i.exec(url.replace(/[\0-\x20]/g, ''))?.[1].toLowerCase();
  return !scheme || schemes.includes(scheme);
}

function safeAttr(el, name, value) {
  const tag = el.localName;
  if (name === 'href') return tag === 'a' && safeUrl(value, ['http', 'https', 'mailto', 'tel']);
  if (name === 'src') {
    return tag === 'img' && safeUrl(value, ['http', 'https', 'blob', 'data'])
      && (!/^\s*data:/i.test(value) || /^\s*data:image\/(?:png|jpeg|gif|webp|avif)[;,]/i.test(value));
  }
  if (name === 'class') return tag === 'code' && /^language-\S+$/.test(value);
  if (name === 'type' || name === 'checked' || name === 'disabled') return tag === 'input';
  return SAFE_ATTRS.has(name);
}

function scrub(root) {
  for (const node of [...root.childNodes]) {
    if (node instanceof Text) continue;
    // Forms go first: their named controls shadow the properties read below.
    if (!(node instanceof HTMLElement) || node instanceof HTMLFormElement || DROP_TAGS.has(node.localName)) {
      root.removeChild(node);
      continue;
    }
    scrub(node);
    const tag = node.localName;
    if (!SAFE_TAGS.has(tag)) {
      node.replaceWith(...node.childNodes);
      continue;
    }
    for (const { name, value } of [...node.attributes]) {
      if (!safeAttr(node, name, value)) node.removeAttribute(name);
    }
    if (tag === 'a' && node.hasAttribute('href') && !node.getAttribute('href').startsWith('#')) {
      node.setAttribute('target', '_blank');
      node.setAttribute('rel', 'noopener noreferrer');
    }
    if (tag === 'input') {
      if (node.type === 'checkbox') node.disabled = true;
      else root.removeChild(node);
    }
  }
}

// Parse untrusted HTML into a fragment holding only allowlisted markup. The
// template keeps it inert (no scripts, no image loads) until it is clean.
function sanitize(html) {
  const template = document.createElement('template');
  template.innerHTML = html;
  scrub(template.content);
  return template.content;
}

// ---------------------------------------------------------------- preview

function render() {
  renderQueued = false;
  doc.replaceChildren(sanitize(renderMarkdown(editor.value)));
  syncScroll(editor, preview);
}

function queueRender() {
  if (renderQueued) return;
  renderQueued = true;
  requestAnimationFrame(() => {
    if (workspace.dataset.view === 'edit') renderQueued = false;
    else render();
  });
}

// Keep the two panes at the same relative position.
function syncScroll(from, to) {
  if (scrollLock === from) return;
  const range = from.scrollHeight - from.clientHeight;
  const target = range > 0 ? (from.scrollTop / range) * (to.scrollHeight - to.clientHeight) : 0;
  if (Math.abs(to.scrollTop - target) < 1) return;
  // Ignore the scroll event this causes, or the panes would chase each other.
  scrollLock = to;
  to.scrollTop = target;
  requestAnimationFrame(() => {
    scrollLock = null;
  });
}

function setView(name) {
  workspace.dataset.view = name;
  for (const b of viewButtons) b.setAttribute('aria-checked', String(b.dataset.view === name));
  try {
    localStorage.setItem(VIEW_KEY, name);
  } catch {
    // Storage can be disabled; the view just won't be remembered.
  }
  render();
}

// GitHub-style heading anchor, so "#like-this" links work inside the preview.
const slug = (s) => s.trim().toLowerCase().replace(/[^\p{L}\p{N}\s_-]/gu, '').replace(/\s/g, '-');

doc.addEventListener('click', (e) => {
  const link = e.target.closest?.('a[href^="#"]');
  if (!link) return;
  e.preventDefault();
  let target = link.getAttribute('href').slice(1);
  try {
    target = decodeURIComponent(target);
  } catch {
    // Not percent-encoded after all.
  }
  [...doc.querySelectorAll('h1, h2, h3, h4, h5, h6')]
    .find((h) => slug(h.textContent) === target.toLowerCase())
    ?.scrollIntoView();
});

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

function updateStatus() {
  const text = editor.value;
  const modified = text !== savedText;
  $('name').textContent = `${fileName}${modified ? ' •' : ''}`;
  document.title = `${modified ? '• ' : ''}${fileName} · Markdown Editor`;
  $('count').textContent = `${text.match(/\S+/g)?.length ?? 0} words · ${text.length} chars`;
}

function updateCaret() {
  const before = editor.value.slice(0, editor.selectionStart).split('\n');
  $('pos').textContent = `Ln ${before.length}, Col ${before.at(-1).length + 1}`;
  const from = editor.selectionStart - before.at(-1).length;
  headingInput.value = /^ {0,3}(#{1,6})(?:[ \t]|$)/.exec(editor.value.slice(from, from + 10))?.[1].length ?? 0;
}

// ---------------------------------------------------------------- editing

// Replace [start, end) with `text`, then select [selStart, selEnd).
function replaceRange(start, end, text, selStart = start + text.length, selEnd = selStart) {
  editor.focus();
  editor.setSelectionRange(start, end);
  if (start !== end || text) {
    // execCommand is deprecated, but nothing else keeps the edit on the
    // textarea's own undo stack.
    if (!document.execCommand(text ? 'insertText' : 'delete', false, text)) {
      editor.setRangeText(text, start, end);
      editor.dispatchEvent(new Event('input', { bubbles: true }));
    }
  }
  editor.setSelectionRange(selStart, selEnd);
}

// Toggle an inline marker such as ** around the selection.
function wrap(mark, placeholder) {
  const { selectionStart: s, selectionEnd: e, value: v } = editor;
  const n = mark.length;
  let before = 0;
  let after = 0;
  while (v[s - 1 - before] === mark[0]) before++;
  while (v[e + after] === mark[0]) after++;
  const run = Math.min(before, after);
  // A run of two asterisks is bold alone; three is bold and italic.
  const on = mark === '*' ? run % 2 === 1 : mark === '**' ? run >= 2 : run >= n;
  if (on) {
    replaceRange(s - n, e + n, v.slice(s, e), s - n, e - n);
  } else {
    const inner = v.slice(s, e) || placeholder;
    replaceRange(s, e, mark + inner + mark, s + n, s + n + inner.length);
  }
}

// Rewrite the lines touched by the selection. `fn` maps them to new lines.
function editLines(fn) {
  const { selectionStart: s, selectionEnd: e, value: v } = editor;
  const from = v.lastIndexOf('\n', s - 1) + 1;
  // A selection ending at the start of a line does not include that line.
  let to = v.indexOf('\n', e > s && v[e - 1] === '\n' ? e - 1 : e);
  if (to < 0) to = v.length;
  const old = v.slice(from, to);
  const text = fn(old.split('\n')).join('\n');
  if (text === old) return editor.focus();
  if (s === e) replaceRange(from, to, text, Math.max(from, s + text.length - old.length));
  else replaceRange(from, to, text, from, from + text.length);
}

// Toggle a line prefix. `fn(line, on, n)` rewrites the nth line, where `on`
// says every line already has the prefix. Blank lines inside a selection are
// left alone.
function toggleLines(test, fn) {
  editLines((lines) => {
    const empty = lines.every((l) => !l.trim());
    const on = !empty && lines.every((l) => !l.trim() || test.test(l));
    let n = 0;
    return lines.map((l) => (empty || l.trim() ? fn(l, on, n++) : l));
  });
}

const LIST_MARK = /^(\s*)(?:[-*+] \[[ xX]\] |[-*+] |\d+[.)] )?/;
const toggleList = (test, mark) =>
  toggleLines(test, (l, on, n) => l.replace(LIST_MARK, (_, indent) => indent + (on ? '' : mark(n))));

function setHeading(level) {
  const mark = level ? `${'#'.repeat(level)} ` : '';
  editLines((lines) => lines.map((l) => (l.trim() || lines.length === 1 ? mark + l.replace(/^ {0,3}#{1,6}(?:[ \t]+|$)/, '') : l)));
}

function insertLink() {
  const { selectionStart: s, selectionEnd: e, value: v } = editor;
  const sel = v.slice(s, e);
  if (/^(?:https?:\/\/|mailto:)\S+$/.test(sel)) replaceRange(s, e, `[text](${sel})`, s + 1, s + 5);
  else replaceRange(s, e, `[${sel || 'text'}](url)`, e + (sel ? 3 : 7), e + (sel ? 6 : 10));
}

function insertCode() {
  const { selectionStart: s, selectionEnd: e, value: v } = editor;
  const sel = v.slice(s, e);
  if (!sel.includes('\n')) return wrap('`', 'code');
  // Fences sit on lines of their own.
  const head = s > 0 && v[s - 1] !== '\n' ? '\n' : '';
  const tail = e < v.length && v[e] !== '\n' ? '\n' : '';
  const body = sel.endsWith('\n') ? sel : `${sel}\n`;
  const start = s + head.length + 4;
  replaceRange(s, e, `${head}\`\`\`\n${body}\`\`\`${tail}`, start, start + body.length - 1);
}

const FORMATS = {
  bold: () => wrap('**', 'bold'),
  italic: () => wrap('*', 'italic'),
  strike: () => wrap('~~', 'struck'),
  code: insertCode,
  link: insertLink,
  quote: () => toggleLines(/^ {0,3}>/, (l, on) => (on ? l.replace(RE.quote, '') : `> ${l}`)),
  ul: () => toggleList(/^\s*[-*+] (?!\[[ xX]\] )/, () => '- '),
  ol: () => toggleList(/^\s*\d+[.)] /, (n) => `${n + 1}. `),
  task: () => toggleList(/^\s*[-*+] \[[ xX]\] /, () => '- [ ] '),
};

// Enter inside a list or quote carries the marker to the new line, or clears
// it when the item is still empty. Returns false to leave Enter alone.
function continueLine() {
  const { selectionStart: s, selectionEnd: e, value: v } = editor;
  if (s !== e) return false;
  const from = v.lastIndexOf('\n', s - 1) + 1;
  const m = /^(\s*(?:> ?)*)(?:([-*+]|(\d+)([.)])) (\[[ xX]\] )?)?/.exec(v.slice(from, s));
  if (!m[0]) return false;
  if (!v.slice(from + m[0].length, s).trim()) {
    // Nothing typed yet: a bare indent is left alone, a bare marker is removed.
    if (!m[0].trim() || /[^\n]*\S/y.test(v.slice(s))) return false;
    replaceRange(from, s, '');
    return true;
  }
  const marker = m[3] ? `${Number(m[3]) + 1}${m[4]} ` : m[2] ? `${m[2]} ` : '';
  replaceRange(s, s, `\n${m[1]}${marker}${m[5] ? '[ ] ' : ''}`);
  return true;
}

function indentLines(out) {
  editLines((lines) => lines.map((l) => (out ? l.replace(/^(?: {1,4}|\t)/, '') : l.trim() ? INDENT + l : l)));
}

editor.addEventListener('keydown', (e) => {
  if (e.isComposing || e.ctrlKey || e.metaKey || e.altKey || e.key === 'Shift') return;
  if (e.key === 'Escape') {
    tabOut = true;
    flashMessage('Press Tab to leave the editor.');
    return;
  }
  const leaving = tabOut;
  tabOut = false;

  if (e.key === 'Enter' && !e.shiftKey) {
    if (continueLine()) e.preventDefault();
  } else if (e.key === 'Tab' && !leaving) {
    e.preventDefault();
    const { selectionStart: s, selectionEnd: end, value: v } = editor;
    const line = v.slice(v.lastIndexOf('\n', s - 1) + 1, s);
    // Tab indents whole lines, except mid-line where it just inserts space.
    if (e.shiftKey || s !== end || RE.item.test(line)) indentLines(e.shiftKey);
    else replaceRange(s, s, INDENT);
  }
});

// Pasting a URL over selected text turns the text into a link.
editor.addEventListener('paste', (e) => {
  const { selectionStart: s, selectionEnd: end, value: v } = editor;
  const url = e.clipboardData?.getData('text/plain').trim() ?? '';
  const sel = v.slice(s, end);
  if (!sel.trim() || sel.includes('\n') || !/^https?:\/\/\S+$/.test(url) || /^https?:\/\//.test(sel)) return;
  e.preventDefault();
  replaceRange(s, end, `[${sel}](${url})`);
});

// ---------------------------------------------------------------- files

function saveDraft() {
  clearTimeout(draftTimer);
  draftTimer = 0;
  try {
    const text = editor.value;
    localStorage.setItem(DRAFT_KEY, JSON.stringify({ name: fileName, text, saved: text === savedText }));
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

function onChange() {
  updateStatus();
  updateCaret();
  queueRender();
  draftTimer ||= setTimeout(saveDraft, DRAFT_DELAY);
}

// Replace the whole document. This also clears the undo history, so that
// undo can never bring one file's text back under another file's name.
function setDocument(text, name, handle = null) {
  editor.value = text;
  editor.setSelectionRange(0, 0);
  editor.scrollTop = 0;
  fileName = name;
  fileHandle = handle;
  savedText = editor.value; // as normalized by the textarea
  onChange();
  saveDraft();
}

// Resolves to true when the current document can be thrown away.
function confirmDiscard() {
  if (editor.value === savedText || !editor.value.trim()) return Promise.resolve(true);
  $('discard-name').textContent = fileName;
  discardDialog.returnValue = '';
  discardDialog.showModal();
  return new Promise((resolve) => {
    discardDialog.addEventListener('close', () => resolve(discardDialog.returnValue === 'discard'), { once: true });
  });
}

async function loadFile(file, handle = null) {
  if (file.type && !file.type.startsWith('text/') && !/\.(?:md|markdown|txt)$/i.test(file.name)) {
    return setMessage(`${file.name} is not a text file.`);
  }
  try {
    setDocument(await file.text(), file.name, handle);
    setMessage('');
  } catch {
    setMessage(`Could not open ${file.name || 'file'}.`);
  }
}

async function openFile() {
  if (!(await confirmDiscard())) return;
  if (!window.showOpenFilePicker) return fileInput.click();
  try {
    const [handle] = await window.showOpenFilePicker({ types: FILE_TYPES });
    await loadFile(await handle.getFile(), handle);
  } catch (err) {
    if (err.name !== 'AbortError') setMessage('Could not open the file.');
  }
}

async function newFile() {
  if (await confirmDiscard()) setDocument('', 'untitled.md');
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

// Writes back to the opened file where the browser allows it, and falls back
// to a download elsewhere. `saveAs` always asks for a new location.
async function save(saveAs = false) {
  const text = editor.value;
  try {
    if ((saveAs || !fileHandle) && window.showSaveFilePicker) {
      fileHandle = await window.showSaveFilePicker({ suggestedName: fileName, types: FILE_TYPES });
    }
    if (fileHandle) {
      const writable = await fileHandle.createWritable();
      await writable.write(text);
      await writable.close();
      fileName = fileHandle.name;
    } else {
      download(text, fileName, 'text/markdown');
    }
    savedText = text;
    updateStatus();
    saveDraft();
    flashMessage(`Saved ${fileName}.`);
  } catch (err) {
    if (err.name !== 'AbortError') setMessage(`Could not save ${fileName}.`);
  }
}

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
  li:has(> input, > p:first-child > input) { list-style: none; }
  li input { margin: 0 0.5em 0 -1.4em; }
`;

function exportHtml() {
  render();
  const base = fileName.replace(/\.(?:md|markdown|txt)$/i, '');
  const title = doc.querySelector('h1')?.textContent.trim() || base;
  const page = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>${escapeHtml(title)}</title>
<style>${EXPORT_STYLE}</style>
</head>
<body>
${doc.innerHTML}</body>
</html>
`;
  download(page, `${base}.html`, 'text/html');
}

// Rich text for pasting into mail or a document, with the source as fallback.
async function copy() {
  render();
  try {
    await navigator.clipboard.write([
      new ClipboardItem({
        'text/html': new Blob([doc.innerHTML], { type: 'text/html' }),
        'text/plain': new Blob([editor.value], { type: 'text/plain' }),
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

// Only file drags are taken over; dragging text within the editor still works.
const hasFiles = (e) => e.dataTransfer?.types.includes('Files');

workspace.addEventListener('dragover', (e) => {
  if (!hasFiles(e)) return;
  e.preventDefault();
  workspace.classList.add('dragover');
});
workspace.addEventListener('dragleave', () => workspace.classList.remove('dragover'));
workspace.addEventListener('drop', async (e) => {
  if (!hasFiles(e)) return;
  e.preventDefault();
  workspace.classList.remove('dragover');
  const file = e.dataTransfer.files[0];
  // The handle is only on offer until the event handler first yields.
  const pending = e.dataTransfer.items[0]?.getAsFileSystemHandle?.().catch(() => null);
  if (!file || !(await confirmDiscard())) return;
  const handle = await pending;
  loadFile(file, handle?.kind === 'file' ? handle : null);
});

// ---------------------------------------------------------------- controls

$('new').addEventListener('click', newFile);
$('open').addEventListener('click', openFile);
$('save').addEventListener('click', () => save());
$('export').addEventListener('click', exportHtml);
$('copy').addEventListener('click', copy);

for (const name of ['undo', 'redo']) {
  $(name).addEventListener('click', () => {
    editor.focus();
    document.execCommand(name);
  });
}

for (const b of document.querySelectorAll('[data-format]')) {
  b.addEventListener('click', () => FORMATS[b.dataset.format]());
}
headingInput.addEventListener('change', () => setHeading(Number(headingInput.value)));

for (const b of viewButtons) {
  b.addEventListener('click', () => setView(b.dataset.view));
}

editor.addEventListener('input', onChange);
editor.addEventListener('blur', () => {
  tabOut = false;
});
document.addEventListener('selectionchange', () => {
  if (document.activeElement === editor) updateCaret();
});

editor.addEventListener('scroll', () => syncScroll(editor, preview));
preview.addEventListener('scroll', () => syncScroll(preview, editor));

const FORMAT_KEYS = { b: 'bold', i: 'italic', k: 'link' };

document.addEventListener('keydown', (e) => {
  if (discardDialog.open || e.altKey || !(e.ctrlKey || e.metaKey)) return;
  const key = e.key.toLowerCase();
  if (key === 's') {
    e.preventDefault();
    save(e.shiftKey);
  } else if (key === 'o') {
    e.preventDefault();
    openFile();
  } else if (FORMAT_KEYS[key] && !e.shiftKey && e.target === editor) {
    e.preventDefault();
    FORMATS[FORMAT_KEYS[key]]();
  }
});

// The draft is written on a delay, so flush it before the page goes away.
document.addEventListener('visibilitychange', () => {
  if (document.hidden && draftTimer) saveDraft();
});
window.addEventListener('pagehide', () => {
  if (draftTimer) saveDraft();
});

// ---------------------------------------------------------------- init

{
  const draft = loadDraft();
  let view = null;
  try {
    view = localStorage.getItem(VIEW_KEY);
  } catch {
    // Storage is disabled.
  }
  if (!viewButtons.some((b) => b.dataset.view === view)) {
    view = matchMedia('(max-width: 640px)').matches ? 'edit' : 'split';
  }
  editor.value = draft ? draft.text : WELCOME;
  editor.setSelectionRange(0, 0);
  if (draft?.name) fileName = String(draft.name);
  savedText = !draft || draft.saved ? editor.value : null;
  updateStatus();
  updateCaret();
  setView(view);
}
