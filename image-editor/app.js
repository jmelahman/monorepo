'use strict';

const MAX_DIM = 8192;
const MAX_HISTORY = 50;
const MAX_HISTORY_BYTES = 256 * 1024 * 1024;
const LINE_HEIGHT = 1.2;
const FILL_TOLERANCE = 24;

const $ = (id) => document.getElementById(id);

const workspace = $('workspace');
const stage = $('stage');
const canvas = $('canvas');
const overlay = $('overlay');
const ctx = canvas.getContext('2d');
const octx = overlay.getContext('2d');

const colorInput = $('color');
const sizeInput = $('size');
const filledInput = $('filled');
const fontInput = $('font');
const fontSizeInput = $('font-size');
const boldInput = $('bold');
const zoomInput = $('zoom');
const fileInput = $('file');
const toolButtons = [...document.querySelectorAll('[data-tool]')];
const toolOptions = [...document.querySelectorAll('[data-tools]')];

let tool = 'brush';
let prevTool = 'brush';
let zoom = 1;
let drag = null; // in-progress stroke or shape
let text = null; // in-progress text box: { x, y, el }
let swallowClick = false;
let dirty = false;
const undoStack = [];
const redoStack = [];

// ---------------------------------------------------------------- helpers

const strokeSize = () => Number(sizeInput.value);
const fontSize = () => Math.min(400, Math.max(6, Number(fontSizeInput.value) || 32));
const fontSpec = (px) => `${boldInput.checked ? 'bold ' : ''}${px}px ${fontInput.value}`;

function setMessage(msg) {
  $('message').textContent = msg;
}

function pointerPos(e) {
  const r = canvas.getBoundingClientRect();
  return {
    x: ((e.clientX - r.left) * canvas.width) / r.width,
    y: ((e.clientY - r.top) * canvas.height) / r.height,
  };
}

function clearOverlay() {
  octx.clearRect(0, 0, overlay.width, overlay.height);
}

function setCanvasSize(w, h) {
  canvas.width = overlay.width = w;
  canvas.height = overlay.height = h;
  $('dims').textContent = `${w} × ${h} px`;
  applyZoom();
}

function applyZoom() {
  if (zoomInput.value === 'fit') {
    zoom = Math.min(
      1,
      (workspace.clientWidth - 32) / canvas.width,
      (workspace.clientHeight - 32) / canvas.height,
    );
  } else {
    zoom = Number(zoomInput.value);
  }
  zoom = Math.max(zoom, 0.01);
  stage.style.width = `${canvas.width * zoom}px`;
  stage.style.height = `${canvas.height * zoom}px`;
  canvas.style.imageRendering = zoom > 1 ? 'pixelated' : 'auto';
  if (text) layoutText();
}

// ---------------------------------------------------------------- history

function updateHistoryButtons() {
  $('undo').disabled = undoStack.length === 0;
  $('redo').disabled = redoStack.length === 0;
}

function snapshot() {
  return ctx.getImageData(0, 0, canvas.width, canvas.height);
}

function restore(image) {
  if (image.width !== canvas.width || image.height !== canvas.height) {
    setCanvasSize(image.width, image.height);
  }
  ctx.putImageData(image, 0, 0);
}

// Call before every change to the canvas.
function pushUndo() {
  const image = snapshot();
  const limit = Math.max(5, Math.min(MAX_HISTORY, Math.floor(MAX_HISTORY_BYTES / image.data.length)));
  undoStack.push(image);
  while (undoStack.length > limit) undoStack.shift();
  redoStack.length = 0;
  dirty = true;
  updateHistoryButtons();
}

function undo() {
  if (text) return cancelText();
  if (!undoStack.length) return;
  redoStack.push(snapshot());
  restore(undoStack.pop());
  updateHistoryButtons();
}

function redo() {
  commitText();
  if (!redoStack.length) return;
  undoStack.push(snapshot());
  restore(redoStack.pop());
  updateHistoryButtons();
}

// ---------------------------------------------------------------- tools

function setTool(name) {
  commitText();
  if (tool !== 'eyedropper') prevTool = tool;
  tool = name;
  canvas.dataset.tool = name;
  for (const b of toolButtons) {
    b.setAttribute('role', 'radio');
    b.setAttribute('aria-checked', String(b.dataset.tool === name));
  }
  for (const el of toolOptions) {
    el.hidden = !el.dataset.tools.split(' ').includes(name);
  }
  clearOverlay();
  setMessage(name === 'text' ? 'Click to place text. Ctrl+Enter to apply, Esc to cancel.' : '');
}

function strokeSegment(a, b) {
  ctx.save();
  ctx.globalCompositeOperation = tool === 'eraser' ? 'destination-out' : 'source-over';
  ctx.strokeStyle = colorInput.value;
  ctx.lineWidth = strokeSize();
  ctx.lineCap = ctx.lineJoin = 'round';
  ctx.beginPath();
  ctx.moveTo(a.x, a.y);
  // A zero-length segment draws nothing in some browsers, so nudge it.
  ctx.lineTo(b.x + (a.x === b.x && a.y === b.y ? 0.01 : 0), b.y);
  ctx.stroke();
  ctx.restore();
}

function drawShape(c, a, b, constrain) {
  let dx = b.x - a.x;
  let dy = b.y - a.y;
  c.save();
  c.strokeStyle = c.fillStyle = colorInput.value;
  c.lineWidth = strokeSize();
  c.lineCap = 'round';
  c.lineJoin = tool === 'rect' ? 'miter' : 'round';
  c.beginPath();
  if (tool === 'line') {
    if (constrain) {
      const step = Math.PI / 4;
      const angle = Math.round(Math.atan2(dy, dx) / step) * step;
      const len = Math.hypot(dx, dy);
      dx = Math.cos(angle) * len;
      dy = Math.sin(angle) * len;
    }
    c.moveTo(a.x, a.y);
    c.lineTo(a.x + dx, a.y + dy);
    c.stroke();
  } else {
    if (constrain) {
      const side = Math.max(Math.abs(dx), Math.abs(dy));
      dx = (dx < 0 ? -1 : 1) * side;
      dy = (dy < 0 ? -1 : 1) * side;
    }
    if (tool === 'rect') {
      c.rect(a.x, a.y, dx, dy);
    } else {
      c.ellipse(a.x + dx / 2, a.y + dy / 2, Math.abs(dx) / 2, Math.abs(dy) / 2, 0, 0, Math.PI * 2);
    }
    if (filledInput.checked) c.fill();
    else c.stroke();
  }
  c.restore();
}

// Outline of the brush under the cursor.
function drawBrushRing(p) {
  const r = Math.max(strokeSize() / 2, 0.5);
  octx.save();
  octx.lineWidth = 1 / zoom;
  for (const [style, grow] of [['#000', 0], ['#fff', 1 / zoom]]) {
    octx.strokeStyle = style;
    octx.beginPath();
    octx.arc(p.x, p.y, r + grow, 0, Math.PI * 2);
    octx.stroke();
  }
  octx.restore();
}

function floodFill(p) {
  const w = canvas.width;
  const h = canvas.height;
  const x0 = Math.floor(p.x);
  const y0 = Math.floor(p.y);
  if (x0 < 0 || y0 < 0 || x0 >= w || y0 >= h) return;

  const image = snapshot();
  const d = image.data;
  const start = (y0 * w + x0) * 4;
  const tr = d[start];
  const tg = d[start + 1];
  const tb = d[start + 2];
  const ta = d[start + 3];
  const hex = colorInput.value;
  const fr = parseInt(hex.slice(1, 3), 16);
  const fg = parseInt(hex.slice(3, 5), 16);
  const fb = parseInt(hex.slice(5, 7), 16);

  const seen = new Uint8Array(w * h);
  const matches = (px) => {
    if (seen[px]) return false;
    const i = px * 4;
    return (
      Math.abs(d[i] - tr) <= FILL_TOLERANCE &&
      Math.abs(d[i + 1] - tg) <= FILL_TOLERANCE &&
      Math.abs(d[i + 2] - tb) <= FILL_TOLERANCE &&
      Math.abs(d[i + 3] - ta) <= FILL_TOLERANCE
    );
  };

  // Scanline fill: paint a whole row span, then queue the spans above and below.
  const stack = [y0 * w + x0];
  while (stack.length) {
    const px = stack.pop();
    if (seen[px]) continue;
    const row = px - (px % w);
    let left = px;
    let right = px;
    while (left > row && matches(left - 1)) left--;
    while (right < row + w - 1 && matches(right + 1)) right++;
    let aboveOpen = false;
    let belowOpen = false;
    for (let q = left; q <= right; q++) {
      seen[q] = 1;
      const i = q * 4;
      d[i] = fr;
      d[i + 1] = fg;
      d[i + 2] = fb;
      d[i + 3] = 255;
      const above = row > 0 && matches(q - w);
      if (above && !aboveOpen) stack.push(q - w);
      aboveOpen = above;
      const below = row + w < w * h && matches(q + w);
      if (below && !belowOpen) stack.push(q + w);
      belowOpen = below;
    }
  }

  pushUndo();
  ctx.putImageData(image, 0, 0);
}

function pickColor(p) {
  const x = Math.floor(p.x);
  const y = Math.floor(p.y);
  if (x < 0 || y < 0 || x >= canvas.width || y >= canvas.height) return;
  const [r, g, b, a] = ctx.getImageData(x, y, 1, 1).data;
  if (a > 0) {
    colorInput.value = '#' + [r, g, b].map((v) => v.toString(16).padStart(2, '0')).join('');
  }
  setTool(prevTool);
}

// ---------------------------------------------------------------- text

function startText(p) {
  const el = document.createElement('textarea');
  el.className = 'text-input';
  el.rows = 1;
  el.wrap = 'off';
  el.spellcheck = false;
  // Put the first line's middle at the click point, like a text cursor.
  text = { x: p.x, y: p.y - (fontSize() * LINE_HEIGHT) / 2, el };
  el.addEventListener('input', layoutText);
  el.addEventListener('keydown', (e) => {
    if (e.key === 'Escape') cancelText();
    else if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) commitText();
  });
  stage.append(el);
  layoutText();
  el.focus();
}

function layoutText() {
  const { el, x, y } = text;
  const px = fontSize() * zoom;
  const s = el.style;
  s.left = `${x * zoom}px`;
  s.top = `${y * zoom}px`;
  s.font = fontSpec(px);
  s.lineHeight = LINE_HEIGHT;
  s.color = colorInput.value;
  s.caretColor = colorInput.value;
  // Shrink first so scroll sizes report the content size.
  s.width = s.height = '0';
  s.width = `${el.scrollWidth + px / 2}px`;
  s.height = `${el.scrollHeight}px`;
}

function cancelText() {
  if (!text) return;
  text.el.remove();
  text = null;
}

function commitText() {
  if (!text) return;
  const { el, x, y } = text;
  cancelText();
  if (!el.value.trim()) return;

  pushUndo();
  const px = fontSize();
  const lineHeight = px * LINE_HEIGHT;
  ctx.save();
  ctx.font = fontSpec(px);
  ctx.fillStyle = colorInput.value;
  ctx.textBaseline = 'alphabetic';
  // Match where CSS puts the baseline inside a line box.
  const m = ctx.measureText('Mg');
  const ascent = m.fontBoundingBoxAscent ?? px * 0.8;
  const descent = m.fontBoundingBoxDescent ?? px * 0.2;
  const baseline = y + (lineHeight - (ascent + descent)) / 2 + ascent;
  el.value.split('\n').forEach((line, i) => {
    ctx.fillText(line, x, baseline + i * lineHeight);
  });
  ctx.restore();
}

// ---------------------------------------------------------------- pointer input

canvas.addEventListener('pointerdown', (e) => {
  if (e.button !== 0 || drag) return;
  swallowClick = false;
  if (text) {
    // Clicking away applies the text; don't also start a new box.
    commitText();
    swallowClick = true;
    return;
  }
  const p = pointerPos(e);
  if (tool === 'brush' || tool === 'eraser') {
    pushUndo();
    drag = { id: e.pointerId, last: p };
    strokeSegment(p, p);
  } else if (tool === 'line' || tool === 'rect' || tool === 'ellipse') {
    drag = { id: e.pointerId, start: p };
  } else if (tool === 'fill') {
    floodFill(p);
    return;
  } else if (tool === 'eyedropper') {
    pickColor(p);
    return;
  } else {
    return;
  }
  canvas.setPointerCapture(e.pointerId);
});

canvas.addEventListener('pointermove', (e) => {
  const p = pointerPos(e);
  $('pos').textContent = `${Math.floor(p.x)}, ${Math.floor(p.y)}`;
  if (drag && e.pointerId !== drag.id) return;

  const brushing = tool === 'brush' || tool === 'eraser';
  if (drag?.last) {
    const events = e.getCoalescedEvents?.() ?? [];
    for (const ev of events.length ? events : [e]) {
      const q = pointerPos(ev);
      strokeSegment(drag.last, q);
      drag.last = q;
    }
  }
  if (drag?.start || brushing) clearOverlay();
  if (drag?.start) drawShape(octx, drag.start, p, e.shiftKey);
  if (brushing && e.pointerType !== 'touch') drawBrushRing(p);
});

function endDrag(e) {
  if (!drag || e.pointerId !== drag.id) return;
  if (drag.start && e.type === 'pointerup') {
    pushUndo();
    drawShape(ctx, drag.start, pointerPos(e), e.shiftKey);
  }
  drag = null;
  clearOverlay();
}

canvas.addEventListener('pointerup', endDrag);
canvas.addEventListener('pointercancel', endDrag);

canvas.addEventListener('pointerleave', () => {
  $('pos').textContent = '';
  if (!drag) clearOverlay();
});

// Text starts on click (not pointerdown) so focusing the box sticks and
// mobile browsers open the keyboard.
canvas.addEventListener('click', (e) => {
  if (tool !== 'text' || swallowClick) return;
  startText(pointerPos(e));
});

// ---------------------------------------------------------------- files

function newCanvas(w, h, background) {
  commitText();
  pushUndo();
  setCanvasSize(w, h);
  if (background) {
    ctx.fillStyle = background;
    ctx.fillRect(0, 0, w, h);
  }
}

async function loadImage(file) {
  const url = URL.createObjectURL(file);
  try {
    const img = new Image();
    img.src = url;
    await img.decode();
    const scale = Math.min(1, MAX_DIM / img.naturalWidth, MAX_DIM / img.naturalHeight);
    const w = Math.max(1, Math.round(img.naturalWidth * scale));
    const h = Math.max(1, Math.round(img.naturalHeight * scale));
    newCanvas(w, h, '');
    ctx.drawImage(img, 0, 0, w, h);
    setMessage(scale < 1 ? `Image scaled down to fit ${MAX_DIM}px.` : '');
  } catch {
    setMessage(`Could not open ${file.name || 'image'}.`);
  } finally {
    URL.revokeObjectURL(url);
  }
}

const toBlob = () => new Promise((resolve) => canvas.toBlob(resolve, 'image/png'));

async function save() {
  commitText();
  const url = URL.createObjectURL(await toBlob());
  const a = document.createElement('a');
  a.href = url;
  a.download = 'image.png';
  a.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
  dirty = false;
}

async function copy() {
  commitText();
  try {
    await navigator.clipboard.write([new ClipboardItem({ 'image/png': toBlob() })]);
    setMessage('Copied to clipboard.');
  } catch {
    setMessage('Copying images is not supported in this browser.');
  }
}

const imageFrom = (files) => [...(files ?? [])].find((f) => f.type.startsWith('image/'));

fileInput.addEventListener('change', () => {
  if (fileInput.files[0]) loadImage(fileInput.files[0]);
  fileInput.value = '';
});

document.addEventListener('paste', (e) => {
  const file = imageFrom(e.clipboardData?.files);
  if (!file) return;
  e.preventDefault();
  loadImage(file);
});

workspace.addEventListener('dragover', (e) => {
  e.preventDefault();
  workspace.classList.add('dragover');
});
workspace.addEventListener('dragleave', () => workspace.classList.remove('dragover'));
workspace.addEventListener('drop', (e) => {
  e.preventDefault();
  workspace.classList.remove('dragover');
  const file = imageFrom(e.dataTransfer?.files);
  if (file) loadImage(file);
});

// ---------------------------------------------------------------- controls

for (const b of toolButtons) {
  b.addEventListener('click', () => setTool(b.dataset.tool));
}

$('undo').addEventListener('click', undo);
$('redo').addEventListener('click', redo);
$('open').addEventListener('click', () => fileInput.click());
$('save').addEventListener('click', save);
$('copy').addEventListener('click', copy);

const newDialog = $('new-dialog');
$('new').addEventListener('click', () => {
  $('new-width').value = canvas.width;
  $('new-height').value = canvas.height;
  newDialog.returnValue = '';
  newDialog.showModal();
});
newDialog.addEventListener('close', () => {
  if (newDialog.returnValue !== 'create') return;
  newCanvas(Number($('new-width').value), Number($('new-height').value), $('new-bg').value);
});

sizeInput.addEventListener('input', () => {
  $('size-out').textContent = sizeInput.value;
});
for (const input of [colorInput, fontInput, fontSizeInput, boldInput]) {
  input.addEventListener('input', () => {
    if (text) layoutText();
  });
}
zoomInput.addEventListener('change', applyZoom);
window.addEventListener('resize', applyZoom);

const TOOL_KEYS = { b: 'brush', e: 'eraser', l: 'line', r: 'rect', o: 'ellipse', t: 'text', g: 'fill', i: 'eyedropper' };

document.addEventListener('keydown', (e) => {
  if (newDialog.open) return;
  const key = e.key.toLowerCase();
  const typing = e.target.matches?.('textarea, select, input:not([type="range"], [type="checkbox"], [type="color"])');

  if (e.ctrlKey || e.metaKey) {
    if (key === 's') {
      e.preventDefault();
      save();
    } else if (key === 'o') {
      e.preventDefault();
      fileInput.click();
    } else if (!typing && key === 'z') {
      e.preventDefault();
      if (e.shiftKey) redo();
      else undo();
    } else if (!typing && key === 'y') {
      e.preventDefault();
      redo();
    }
    return;
  }
  if (typing || e.altKey) return;

  if (TOOL_KEYS[key]) {
    setTool(TOOL_KEYS[key]);
    // Otherwise Space/Enter would re-trigger whichever button was clicked last.
    document.activeElement?.blur?.();
  } else if (key === '[' || key === ']') {
    const step = strokeSize() >= 20 ? 5 : 1;
    sizeInput.value = strokeSize() + (key === ']' ? step : -step);
    sizeInput.dispatchEvent(new Event('input'));
  }
});

window.addEventListener('beforeunload', (e) => {
  if (dirty) e.preventDefault();
});

// ---------------------------------------------------------------- init

setCanvasSize(1280, 720);
ctx.fillStyle = '#ffffff';
ctx.fillRect(0, 0, canvas.width, canvas.height);
setTool('brush');
