'use strict';

const MAX_DIM = 8192;
const MAX_HISTORY = 50;
const MAX_HISTORY_BYTES = 256 * 1024 * 1024;
const LINE_HEIGHT = 1.2;
const FILL_TOLERANCE = 24;

const $ = (id) => document.getElementById(id);

const workspace = $('workspace');
const stage = $('stage');
const layerStack = $('layers');
const layerList = $('layer-list');
const overlay = $('overlay'); // topmost canvas: tool previews and pointer input
const octx = overlay.getContext('2d');

const colorInput = $('color'); // primary color, used by every tool
const color2Input = $('color2');
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
let layers = []; // bottom to top; text layers also have { text, box }
let active = null; // selected layer
let layerSeq = 0;
let drag = null; // in-progress stroke, shape or text move
let text = null; // in-progress text box: { x, y, box, el, layer }
let swallowClick = false;
let dirty = false;
const undoStack = [];
const redoStack = [];

// ---------------------------------------------------------------- helpers

const strokeSize = () => Number(sizeInput.value);
const fontSize = () => Math.min(400, Math.max(6, Number(fontSizeInput.value) || 32));
const textStyle = () => ({
  family: fontInput.value,
  size: fontSize(),
  bold: boldInput.checked,
  color: colorInput.value,
});
const fontSpec = (t, px = t.size) => `${t.bold ? 'bold ' : ''}${px}px ${t.family}`;

function setMessage(msg) {
  $('message').textContent = msg;
}

// A message that clears itself, for explaining why a click did nothing.
function flashMessage(msg) {
  setMessage(msg);
  setTimeout(() => {
    if ($('message').textContent === msg) setMessage('');
  }, 4000);
}

function pointerPos(e) {
  const r = overlay.getBoundingClientRect();
  return {
    x: ((e.clientX - r.left) * overlay.width) / r.width,
    y: ((e.clientY - r.top) * overlay.height) / r.height,
  };
}

function clearOverlay() {
  octx.clearRect(0, 0, overlay.width, overlay.height);
}

// Layers are sized when created, so callers must rebuild them afterwards.
function setCanvasSize(w, h) {
  overlay.width = w;
  overlay.height = h;
  $('dims').textContent = `${w} × ${h} px`;
  applyZoom();
}

function applyZoom() {
  if (zoomInput.value === 'fit') {
    zoom = Math.min(
      1,
      (workspace.clientWidth - 32) / overlay.width,
      (workspace.clientHeight - 32) / overlay.height,
    );
  } else {
    zoom = Number(zoomInput.value);
  }
  zoom = Math.max(zoom, 0.01);
  stage.style.width = `${overlay.width * zoom}px`;
  stage.style.height = `${overlay.height * zoom}px`;
  layerStack.style.imageRendering = zoom > 1 ? 'pixelated' : 'auto';
  if (text) layoutText();
}

// ---------------------------------------------------------------- history

function updateHistoryButtons() {
  $('undo').disabled = undoStack.length === 0;
  $('redo').disabled = redoStack.length === 0;
}

// Unchanged layers share their pixels between snapshots through `saved`.
function snapshot() {
  return {
    w: overlay.width,
    h: overlay.height,
    active: active.id,
    layers: layers.map((l) => {
      if (!l.text) l.saved ??= l.ctx.getImageData(0, 0, overlay.width, overlay.height);
      return { id: l.id, name: l.name, visible: l.visible, text: l.text, image: l.saved };
    }),
  };
}

function restore(state) {
  const resized = state.w !== overlay.width || state.h !== overlay.height;
  if (resized) setCanvasSize(state.w, state.h);
  const old = new Map(layers.map((l) => [l.id, l]));
  layers = state.layers.map(({ image, ...props }) => {
    const prev = old.get(props.id);
    // A layer still holding this very snapshot has not been drawn on since.
    if (!resized && image && prev?.saved === image) return Object.assign(prev, props);
    const layer = makeLayer({ ...props, saved: image });
    if (image) layer.ctx.putImageData(image, 0, 0);
    else renderText(layer);
    return layer;
  });
  active = layers.find((l) => l.id === state.active);
  renderLayers();
}

// Call before every change. `changed` is the layer whose pixels are about to
// be drawn on, if any.
function pushUndo(changed) {
  const bytes = overlay.width * overlay.height * 4;
  const limit = Math.max(5, Math.min(MAX_HISTORY, Math.floor(MAX_HISTORY_BYTES / bytes)));
  undoStack.push(snapshot());
  if (changed) changed.saved = null;
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

// ---------------------------------------------------------------- layers

function makeLayer({ id = ++layerSeq, name = `Layer ${id}`, visible = true, text = null, saved = null } = {}) {
  const el = document.createElement('canvas');
  el.width = overlay.width;
  el.height = overlay.height;
  return { id, name, visible, text, saved, canvas: el, ctx: el.getContext('2d') };
}

// Adds a layer above the active one and selects it.
function insertLayer(layer) {
  layers.splice(layers.indexOf(active) + 1, 0, layer);
  active = layer;
  renderLayers();
}

function removeLayer(layer) {
  const i = layers.indexOf(layer);
  layers.splice(i, 1);
  if (active === layer) active = layers[Math.max(0, i - 1)];
  renderLayers();
}

function resetLayers(w, h) {
  setCanvasSize(w, h);
  layers = [];
  active = null;
  insertLayer(makeLayer({ name: 'Background' }));
}

function moveLayer(step) {
  commitText();
  const i = layers.indexOf(active);
  const j = i + step;
  if (j < 0 || j >= layers.length) return;
  pushUndo();
  [layers[i], layers[j]] = [layers[j], layers[i]];
  renderLayers();
}

// Merging a hidden layer would either reveal it or hide the other one.
function canMerge() {
  const lower = layers[layers.indexOf(active) - 1];
  return Boolean(lower?.visible && active.visible);
}

function mergeDown() {
  commitText();
  if (!canMerge()) return;
  const upper = active;
  const lower = layers[layers.indexOf(active) - 1];
  pushUndo(lower);
  // A text layer's canvas already holds its pixels, so dropping `text` rasterizes it.
  lower.text = null;
  lower.ctx.drawImage(upper.canvas, 0, 0);
  active = lower;
  removeLayer(upper);
}

// Start drawing on the active layer. Text stays editable, so painting over a
// text layer goes on a new layer above it.
function beginPaint() {
  const onText = active.text;
  pushUndo(onText ? null : active);
  if (onText) insertLayer(makeLayer());
  return active.ctx;
}

// Flatten the visible layers into one canvas.
function composite(x = 0, y = 0, w = overlay.width, h = overlay.height) {
  const out = document.createElement('canvas');
  out.width = w;
  out.height = h;
  const c = out.getContext('2d');
  for (const l of layers) {
    if (l.visible) c.drawImage(l.canvas, -x, -y);
  }
  return out;
}

function renderLayers() {
  layerStack.replaceChildren(...layers.map((l) => l.canvas));
  for (const l of layers) l.canvas.hidden = !l.visible || l === text?.layer;

  layerList.replaceChildren(
    ...layers.toReversed().map((l) => {
      const label = l.text ? `T  ${l.text.value.trim().split('\n')[0]}` : l.name;
      const eye = document.createElement('input');
      eye.type = 'checkbox';
      eye.checked = l.visible;
      eye.title = 'Show layer';
      eye.setAttribute('aria-label', `Show ${label}`);
      eye.addEventListener('change', () => {
        commitText();
        pushUndo();
        l.visible = eye.checked;
        renderLayers();
      });
      const name = document.createElement('button');
      name.type = 'button';
      name.textContent = label;
      name.setAttribute('aria-pressed', String(l === active));
      name.addEventListener('click', () => {
        commitText();
        active = l;
        renderLayers();
      });
      const li = document.createElement('li');
      li.append(eye, name);
      return li;
    }),
  );

  const i = layers.indexOf(active);
  $('layer-delete').disabled = layers.length < 2;
  $('layer-up').disabled = i === layers.length - 1;
  $('layer-down').disabled = i === 0;
  $('layer-merge').disabled = !canMerge();
}

// ---------------------------------------------------------------- tools

function setTool(name) {
  commitText();
  if (tool !== 'eyedropper') prevTool = tool;
  tool = name;
  overlay.dataset.tool = name;
  overlay.style.cursor = '';
  for (const b of toolButtons) {
    b.setAttribute('role', 'radio');
    b.setAttribute('aria-checked', String(b.dataset.tool === name));
  }
  for (const el of toolOptions) {
    el.hidden = !el.dataset.tools.split(' ').includes(name);
  }
  clearOverlay();
  setMessage(
    name === 'text' ? 'Click to place or edit text, drag text to move it. Ctrl+Enter to apply, Esc to cancel.' : '',
  );
}

function swapColors() {
  [colorInput.value, color2Input.value] = [color2Input.value, colorInput.value];
  if (text) layoutText();
}

function strokeSegment(a, b) {
  const ctx = active.ctx;
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
  const w = overlay.width;
  const h = overlay.height;
  const x0 = Math.floor(p.x);
  const y0 = Math.floor(p.y);
  if (x0 < 0 || y0 < 0 || x0 >= w || y0 >= h) return;

  // The region is found in the picture as it looks, then painted on the active layer.
  const src = composite().getContext('2d').getImageData(0, 0, w, h).data;
  const ctx = beginPaint();
  const image = ctx.getImageData(0, 0, w, h);
  const d = image.data;
  const start = (y0 * w + x0) * 4;
  const tr = src[start];
  const tg = src[start + 1];
  const tb = src[start + 2];
  const ta = src[start + 3];
  const hex = colorInput.value;
  const fr = parseInt(hex.slice(1, 3), 16);
  const fg = parseInt(hex.slice(3, 5), 16);
  const fb = parseInt(hex.slice(5, 7), 16);

  const seen = new Uint8Array(w * h);
  const matches = (px) => {
    if (seen[px]) return false;
    const i = px * 4;
    return (
      Math.abs(src[i] - tr) <= FILL_TOLERANCE &&
      Math.abs(src[i + 1] - tg) <= FILL_TOLERANCE &&
      Math.abs(src[i + 2] - tb) <= FILL_TOLERANCE &&
      Math.abs(src[i + 3] - ta) <= FILL_TOLERANCE
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

  ctx.putImageData(image, 0, 0);
}

function pickColor(p) {
  const x = Math.floor(p.x);
  const y = Math.floor(p.y);
  if (x < 0 || y < 0 || x >= overlay.width || y >= overlay.height) return;
  const [r, g, b, a] = composite(x, y, 1, 1).getContext('2d').getImageData(0, 0, 1, 1).data;
  if (a > 0) {
    colorInput.value = '#' + [r, g, b].map((v) => v.toString(16).padStart(2, '0')).join('');
  }
  setTool(prevTool);
}

// ---------------------------------------------------------------- text

function openText(x, y, layer) {
  const box = document.createElement('div');
  box.className = 'text-box';
  const handle = document.createElement('div');
  handle.className = 'text-handle';
  handle.title = 'Drag to move';
  const el = document.createElement('textarea');
  el.className = 'text-input';
  el.rows = 1;
  el.wrap = 'off';
  el.spellcheck = false;
  el.value = layer?.text.value ?? '';
  text = { x, y, box, el, layer };
  el.addEventListener('input', layoutText);
  el.addEventListener('keydown', (e) => {
    if (e.key === 'Escape') cancelText();
    else if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) commitText();
  });
  handle.addEventListener('pointerdown', (e) => {
    const from = pointerPos(e);
    const origin = { x: text.x, y: text.y };
    handle.setPointerCapture(e.pointerId);
    handle.onpointermove = (ev) => {
      const p = pointerPos(ev);
      text.x = origin.x + p.x - from.x;
      text.y = origin.y + p.y - from.y;
      layoutText();
    };
    handle.onpointerup = handle.onpointercancel = () => {
      handle.onpointermove = null;
      el.focus();
    };
  });
  box.append(handle, el);
  stage.append(box);
  renderLayers();
  layoutText();
  el.focus();
}

function startText(p) {
  // Put the first line's middle at the click point, like a text cursor.
  openText(p.x, p.y - (fontSize() * LINE_HEIGHT) / 2);
}

// Reopen a text layer for editing, with the toolbar showing its style.
function editText(layer) {
  const t = layer.text;
  const toolbar = textStyle();
  fontInput.value = t.family;
  fontSizeInput.value = t.size;
  boldInput.checked = t.bold;
  colorInput.value = t.color;
  active = layer;
  openText(t.x, t.y, layer);
  text.toolbar = toolbar;
}

function layoutText() {
  const { box, el, x, y } = text;
  const style = textStyle();
  const px = style.size * zoom;
  box.style.left = `${x * zoom}px`;
  box.style.top = `${y * zoom}px`;
  const s = el.style;
  s.font = fontSpec(style, px);
  s.lineHeight = LINE_HEIGHT;
  s.color = style.color;
  s.caretColor = style.color;
  // Shrink first so scroll sizes report the content size.
  s.width = s.height = '0';
  s.width = `${el.scrollWidth + px / 2}px`;
  s.height = `${el.scrollHeight}px`;
}

function cancelText() {
  if (!text) return;
  text.box.remove();
  // Editing borrowed the toolbar for that text's style; give it back.
  if (text.toolbar) {
    fontInput.value = text.toolbar.family;
    fontSizeInput.value = text.toolbar.size;
    boldInput.checked = text.toolbar.bold;
    colorInput.value = text.toolbar.color;
  }
  text = null;
  renderLayers();
}

function commitText() {
  if (!text) return;
  const { el, x, y, layer } = text;
  const props = { x, y, value: el.value, ...textStyle() };
  cancelText();
  if (!el.value.trim()) {
    if (!layer) return;
    // Emptying an existing text box deletes it, or blanks the last layer.
    pushUndo();
    if (layers.length > 1) {
      removeLayer(layer);
    } else {
      layer.text = null;
      layer.ctx.clearRect(0, 0, overlay.width, overlay.height);
      renderLayers();
    }
    return;
  }
  if (layer && JSON.stringify(props) === JSON.stringify(layer.text)) return;

  pushUndo();
  if (layer) layer.text = props;
  else insertLayer(makeLayer({ text: props }));
  renderText(layer ?? active);
  renderLayers();
}

// Draw a text layer from its `text`, which is replaced rather than mutated so
// history snapshots can share it.
function renderText(layer) {
  const { x, y, value, size, color } = layer.text;
  const ctx = layer.ctx;
  const lineHeight = size * LINE_HEIGHT;
  const lines = value.split('\n');
  ctx.clearRect(0, 0, overlay.width, overlay.height);
  ctx.save();
  ctx.font = fontSpec(layer.text);
  ctx.fillStyle = color;
  ctx.textBaseline = 'alphabetic';
  // Match where CSS puts the baseline inside a line box.
  const m = ctx.measureText('Mg');
  const ascent = m.fontBoundingBoxAscent ?? size * 0.8;
  const descent = m.fontBoundingBoxDescent ?? size * 0.2;
  const baseline = y + (lineHeight - (ascent + descent)) / 2 + ascent;
  lines.forEach((line, i) => {
    ctx.fillText(line, x, baseline + i * lineHeight);
  });
  const w = Math.max(...lines.map((line) => ctx.measureText(line).width));
  ctx.restore();
  layer.box = { x, y, w, h: lines.length * lineHeight };
}

// Topmost visible text layer under a point.
function textLayerAt(p) {
  const pad = 4 / zoom;
  return layers.findLast(
    ({ text: t, visible, box: b }) =>
      t && visible && p.x >= b.x - pad && p.x <= b.x + b.w + pad && p.y >= b.y - pad && p.y <= b.y + b.h + pad,
  );
}

function drawTextOutline({ box: b }) {
  const pad = 2 / zoom;
  octx.save();
  octx.lineWidth = 1 / zoom;
  octx.setLineDash([4 / zoom, 4 / zoom]);
  octx.strokeStyle = '#2563eb';
  octx.strokeRect(b.x - pad, b.y - pad, b.w + pad * 2, b.h + pad * 2);
  octx.restore();
}

// ---------------------------------------------------------------- pointer input

overlay.addEventListener('pointerdown', (e) => {
  if (e.button !== 0 || drag) return;
  swallowClick = false;
  if (text) {
    // Clicking away applies the text; don't also start a new box.
    commitText();
    swallowClick = true;
    return;
  }
  const p = pointerPos(e);
  if (tool === 'text') {
    const layer = textLayerAt(p);
    if (!layer) return;
    drag = { id: e.pointerId, layer, from: p, origin: layer.text };
  } else if (tool !== 'eyedropper' && !active.visible && !active.text) {
    flashMessage('The selected layer is hidden.');
    return;
  } else if (tool === 'eraser' && active.text) {
    flashMessage('Text can\u2019t be erased. Merge it down first, or clear the text to delete it.');
    return;
  } else if (tool === 'brush' || tool === 'eraser') {
    beginPaint();
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
  overlay.setPointerCapture(e.pointerId);
});

overlay.addEventListener('pointermove', (e) => {
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
  if (drag?.layer) {
    const dx = p.x - drag.from.x;
    const dy = p.y - drag.from.y;
    // A press that barely moves is a click, which edits the text instead.
    if (!drag.moved && Math.hypot(dx, dy) * zoom >= 4) {
      pushUndo();
      drag.moved = true;
      active = drag.layer;
      renderLayers();
    }
    if (drag.moved) {
      drag.layer.text = { ...drag.origin, x: drag.origin.x + dx, y: drag.origin.y + dy };
      renderText(drag.layer);
    }
  }
  if (tool === 'text' && !text) {
    const hit = drag?.layer ?? textLayerAt(p);
    clearOverlay();
    if (hit) drawTextOutline(hit);
    overlay.style.cursor = hit ? 'move' : '';
  }
  if (drag?.start || brushing) clearOverlay();
  if (drag?.start) drawShape(octx, drag.start, p, e.shiftKey);
  if (brushing && e.pointerType !== 'touch') drawBrushRing(p);
});

function endDrag(e) {
  if (!drag || e.pointerId !== drag.id) return;
  if (drag.start && e.type === 'pointerup') {
    drawShape(beginPaint(), drag.start, pointerPos(e), e.shiftKey);
  }
  if (drag.moved) swallowClick = true;
  drag = null;
  clearOverlay();
}

overlay.addEventListener('pointerup', endDrag);
overlay.addEventListener('pointercancel', endDrag);

overlay.addEventListener('pointerleave', () => {
  $('pos').textContent = '';
  if (!drag) clearOverlay();
});

// Text starts on click (not pointerdown) so focusing the box sticks and
// mobile browsers open the keyboard.
overlay.addEventListener('click', (e) => {
  if (tool !== 'text' || swallowClick) return;
  const p = pointerPos(e);
  const layer = textLayerAt(p);
  if (layer) editText(layer);
  else startText(p);
});

// ---------------------------------------------------------------- files

function newCanvas(w, h, background) {
  commitText();
  pushUndo();
  resetLayers(w, h);
  if (background) {
    active.ctx.fillStyle = background;
    active.ctx.fillRect(0, 0, w, h);
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
    active.ctx.drawImage(img, 0, 0, w, h);
    setMessage(scale < 1 ? `Image scaled down to fit ${MAX_DIM}px.` : '');
  } catch {
    setMessage(`Could not open ${file.name || 'image'}.`);
  } finally {
    URL.revokeObjectURL(url);
  }
}

const toBlob = () => new Promise((resolve) => composite().toBlob(resolve, 'image/png'));

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
$('swap').addEventListener('click', swapColors);

$('layer-add').addEventListener('click', () => {
  commitText();
  pushUndo();
  insertLayer(makeLayer());
});
$('layer-delete').addEventListener('click', () => {
  commitText();
  if (layers.length < 2) return;
  pushUndo();
  removeLayer(active);
});
$('layer-up').addEventListener('click', () => moveLayer(1));
$('layer-down').addEventListener('click', () => moveLayer(-1));
$('layer-merge').addEventListener('click', mergeDown);

const newDialog = $('new-dialog');
$('new').addEventListener('click', () => {
  $('new-width').value = overlay.width;
  $('new-height').value = overlay.height;
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
  } else if (key === 'x') {
    swapColors();
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

resetLayers(1280, 720);
active.ctx.fillStyle = '#ffffff';
active.ctx.fillRect(0, 0, overlay.width, overlay.height);
setTool('brush');
