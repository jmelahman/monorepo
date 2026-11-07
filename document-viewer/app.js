import { getDocument, GlobalWorkerOptions, PasswordResponses } from './vendor/pdfjs-dist/pdf.mjs';

GlobalWorkerOptions.workerSrc = new URL('./vendor/pdfjs-dist/pdf.worker.min.mjs', import.meta.url).href;

const MAX_CANVAS_PIXELS = 24 * 1024 * 1024;
const MIN_ZOOM = 0.05;
const MAX_ZOOM = 4;
const ARROW_SCROLL = 80;
const ZOOM_LEVELS = [0.25, 0.5, 0.75, 1, 1.25, 1.5, 2, 3, 4];
const $ = (id) => document.getElementById(id);

const workspace = $('workspace');
const fileInput = $('file');
const pageInput = $('page');
const zoomInput = $('zoom');
const passwordDialog = $('password-dialog');

let pdf = null;
let loadingTask = null;
let loadVersion = 0;
let pageNumber = 1;
let pageViews = [];
let customScale = 1;
let customZoom = null;
let passwordCallback = null;
let resizeTimer = null;
let scrollFrame = null;

function setMessage(message) {
  $('message').textContent = message;
}

function pagePadding() {
  return window.innerWidth <= 640 ? 12 : 24;
}

function updateNavigation() {
  const count = pdf?.numPages ?? 0;
  pageInput.value = pageNumber;
  pageInput.max = count || 1;
  $('page-count').textContent = `/ ${count}`;
  $('previous').disabled = !pdf || pageNumber <= 1;
  $('next').disabled = !pdf || pageNumber >= count;
  const view = pageViews[pageNumber - 1];
  if (view) {
    const viewport = view.baseViewport;
    $('dimensions').textContent = `${Math.round(viewport.width * view.targetScale)} × ${Math.round(viewport.height * view.targetScale)} px`;
  } else {
    $('dimensions').textContent = '';
  }
}

function setDocumentControls(enabled) {
  for (const id of ['page', 'zoom', 'zoom-out', 'zoom-in', 'invert', 'theater', 'fullscreen']) $(id).disabled = !enabled;
  $('zoom-chip').hidden = !enabled;
  if (!enabled) {
    $('previous').disabled = true;
    $('next').disabled = true;
  }
}

function removeCustomZoom() {
  customZoom?.remove();
  customZoom = null;
}

function setZoom(level) {
  customScale = Math.min(MAX_ZOOM, Math.max(MIN_ZOOM, level));
  const preset = [...zoomInput.options].find((option) => option !== customZoom && Number(option.value) === customScale);
  if (preset) {
    removeCustomZoom();
    zoomInput.value = preset.value;
  } else {
    customZoom ??= zoomInput.appendChild(document.createElement('option'));
    customZoom.value = 'custom';
    customZoom.textContent = `${Math.round(customScale * 100)}%`;
    zoomInput.value = 'custom';
  }
}

function zoomScale(baseViewport) {
  const padding = pagePadding() * 2;
  if (zoomInput.value === 'fit-width') {
    return Math.max(0.05, (workspace.clientWidth - padding) / baseViewport.width);
  }
  if (zoomInput.value === 'fit-page') {
    return Math.max(0.05, Math.min(
      (workspace.clientWidth - padding) / baseViewport.width,
      (workspace.clientHeight - padding) / baseViewport.height,
    ));
  }
  return zoomInput.value === 'custom' ? customScale : Number(zoomInput.value);
}

function sizePageView(view) {
  view.targetScale = zoomScale(view.baseViewport);
  const width = `${Math.floor(view.baseViewport.width * view.targetScale)}px`;
  const height = `${Math.floor(view.baseViewport.height * view.targetScale)}px`;
  view.frame.style.width = width;
  view.frame.style.height = height;
  view.canvas.style.width = width;
  view.canvas.style.height = height;
}

// Each view serializes its own renders. A zoom change cancels the current
// render and marks it stale, but never lets two tasks draw to one canvas.
function renderPageView(view) {
  view.renderNeeded = true;
  view.renderTask?.cancel();
  if (view.rendering) return view.rendering;
  const documentRef = pdf;
  if (!documentRef) return Promise.resolve();

  view.rendering = (async () => {
    try {
      while (view.renderNeeded && documentRef === pdf) {
        view.renderNeeded = false;
        const page = await documentRef.getPage(view.number);
        if (view.renderNeeded || documentRef !== pdf) continue;

        view.baseViewport = page.getViewport({ scale: 1 });
        sizePageView(view);
        const scale = view.targetScale;
        const viewport = page.getViewport({ scale });
        const pixelRatio = window.devicePixelRatio || 1;
        const outputScale = Math.min(pixelRatio, Math.sqrt(MAX_CANVAS_PIXELS / (viewport.width * viewport.height)));
        if (view.renderedScale === scale && view.renderedOutputScale === outputScale) continue;

        view.frame.classList.add('loading');
        view.canvas.width = Math.max(1, Math.floor(viewport.width * outputScale));
        view.canvas.height = Math.max(1, Math.floor(viewport.height * outputScale));
        const task = page.render({
          canvasContext: view.canvas.getContext('2d', { alpha: false }),
          viewport,
          transform: outputScale === 1 ? null : [outputScale, 0, 0, outputScale, 0, 0],
          background: '#ffffff',
        });
        view.renderTask = task;
        try {
          await task.promise;
          if (documentRef === pdf) {
            view.renderedScale = scale;
            view.renderedOutputScale = outputScale;
          }
        } catch (error) {
          if (error?.name !== 'RenderingCancelledException' && documentRef === pdf) {
            console.error(error);
            setMessage(`Could not render page ${view.number}.`);
          }
        } finally {
          if (view.renderTask === task) view.renderTask = null;
          view.frame.classList.remove('loading');
        }
      }
    } catch (error) {
      if (documentRef === pdf) {
        console.error(error);
        setMessage(`Could not render page ${view.number}.`);
      }
    } finally {
      view.rendering = null;
      if (view === pageViews[pageNumber - 1]) updateNavigation();
    }
  })();
  return view.rendering;
}

function updatePageSizes() {
  for (const view of pageViews) {
    view.renderedScale = null;
    sizePageView(view);
    if (view.near || view.renderTask) renderPageView(view);
  }
  updateNavigation();
}

async function zoomTo(level, anchor = null) {
  if (!pdf) return;
  const view = pageViews[pageNumber - 1];
  const oldRect = view.canvas.getBoundingClientRect();
  const point = anchor ? {
    x: (anchor.clientX - oldRect.left) / view.targetScale,
    y: (anchor.clientY - oldRect.top) / view.targetScale,
  } : null;
  setZoom(level);
  updatePageSizes();
  await renderPageView(view);
  if (point) {
    const rect = view.canvas.getBoundingClientRect();
    workspace.scrollLeft += rect.left + point.x * view.targetScale - anchor.clientX;
    workspace.scrollTop += rect.top + point.y * view.targetScale - anchor.clientY;
  }
}

function stepZoom(direction) {
  const currentScale = pageViews[pageNumber - 1]?.targetScale ?? 1;
  const levels = direction > 0 ? ZOOM_LEVELS : ZOOM_LEVELS.toReversed();
  const level = levels.find((value) => direction * (value - currentScale) > 0.001);
  zoomTo(level ?? (direction > 0 ? MAX_ZOOM : MIN_ZOOM));
}

function updateCurrentPage() {
  scrollFrame = null;
  if (!pdf) return;
  const bounds = workspace.getBoundingClientRect();
  let mostVisible = 0;
  let current = pageNumber;
  for (const view of pageViews) {
    const rect = view.frame.getBoundingClientRect();
    const visible = Math.max(0, Math.min(rect.bottom, bounds.bottom) - Math.max(rect.top, bounds.top));
    if (visible > mostVisible) {
      mostVisible = visible;
      current = view.number;
    }
  }
  if (current !== pageNumber) {
    pageNumber = current;
    updateNavigation();
  }
}

function goToPage(value) {
  if (!pdf) return;
  pageNumber = Math.min(pdf.numPages, Math.max(1, Math.round(Number(value) || pageNumber)));
  updateNavigation();
  const view = pageViews[pageNumber - 1];
  const bounds = workspace.getBoundingClientRect();
  const rect = view.frame.getBoundingClientRect();
  workspace.scrollTo({ top: workspace.scrollTop + rect.top - bounds.top - pagePadding() });
  renderPageView(view);
}

function makePageViews(firstPage) {
  const firstViewport = firstPage.getViewport({ scale: 1 });
  const inverted = $('invert').getAttribute('aria-pressed') === 'true';
  pageViews = Array.from({ length: pdf.numPages }, (_, index) => {
    const number = index + 1;
    const frame = document.createElement('div');
    frame.className = 'page-frame';
    frame.dataset.page = number;
    const canvas = document.createElement('canvas');
    canvas.className = 'page-canvas';
    canvas.classList.toggle('inverted', inverted);
    canvas.setAttribute('role', 'img');
    canvas.setAttribute('aria-label', `Page ${number} of ${pdf.numPages}`);
    // CSS sizes the placeholder; the renderer sets the real pixel dimensions.
    canvas.width = canvas.height = 1;
    frame.append(canvas);
    const view = {
      number,
      frame,
      canvas,
      baseViewport: firstViewport,
      targetScale: 1,
      renderedScale: null,
      renderedOutputScale: null,
      renderNeeded: false,
      rendering: null,
      renderTask: null,
      near: false,
    };
    sizePageView(view);
    renderObserver.observe(frame);
    return view;
  });
  $('pages').replaceChildren(...pageViews.map((view) => view.frame));
}

function clearDocument() {
  pdf = null;
  pageNumber = 1;
  for (const view of pageViews) {
    view.renderNeeded = false;
    view.renderTask?.cancel();
    renderObserver.unobserve(view.frame);
  }
  pageViews = [];
  $('pages').replaceChildren();
  $('document').hidden = true;
  $('welcome').hidden = false;
  $('name').textContent = 'No document';
  $('dimensions').textContent = '';
  document.title = 'PDF Viewer';
  setDocumentControls(false);
  updateNavigation();
}

async function openPdf(data, name) {
  const version = ++loadVersion;
  passwordCallback = null;
  if (passwordDialog.open) passwordDialog.close('cancel');

  const previousLoad = loadingTask;
  loadingTask = null;
  clearDocument();
  setMessage(`Opening ${name}…`);
  $('name').textContent = name;
  previousLoad?.destroy().catch(() => {});

  const task = getDocument({
    data,
    // Keep the vendored app self-contained and avoid PDF expression eval.
    isEvalSupported: false,
    useWasm: false,
  });
  loadingTask = task;
  task.onPassword = (callback, reason) => {
    if (version !== loadVersion) {
      callback(new Error('Document loading was cancelled.'));
      return;
    }
    passwordCallback = callback;
    $('password-help').textContent = reason === PasswordResponses.INCORRECT_PASSWORD
      ? 'That password was incorrect. Try again.'
      : 'This document needs a password.';
    $('password-input').value = '';
    if (!passwordDialog.open) passwordDialog.showModal();
    $('password-input').focus();
  };

  try {
    const nextPdf = await task.promise;
    if (version !== loadVersion) {
      await nextPdf.destroy();
      return;
    }
    pdf = nextPdf;
    pageNumber = 1;
    const firstPage = await pdf.getPage(1);
    if (version !== loadVersion) return;
    makePageViews(firstPage);
    $('welcome').hidden = true;
    $('document').hidden = false;
    $('name').textContent = name;
    document.title = `${name} — PDF Viewer`;
    setDocumentControls(true);
    workspace.scrollTo({ top: 0, left: 0 });
    // Sizes depend on the now-visible workspace, so compute them once more.
    updatePageSizes();
    await renderPageView(pageViews[0]);
    if (version === loadVersion) setMessage('');
  } catch (error) {
    if (version !== loadVersion) return;
    console.error(error);
    clearDocument();
    if (error?.name === 'InvalidPDFException') {
      setMessage(`${name} is not a valid PDF.`);
    } else if (error?.name === 'PasswordException' || error?.message === 'Password prompt cancelled.') {
      setMessage('The PDF was not unlocked.');
    } else {
      setMessage(`Could not open ${name}.`);
    }
  }
}

function pdfFrom(files) {
  return [...(files ?? [])].find((file) => file.type === 'application/pdf' || file.name?.toLowerCase().endsWith('.pdf'));
}

async function openFile(file) {
  if (!file) return;
  try {
    const data = new Uint8Array(await file.arrayBuffer());
    await openPdf(data, file.name || 'document.pdf');
  } catch (error) {
    console.error(error);
    setMessage(`Could not read ${file.name || 'the PDF'}.`);
  }
}

async function openUrl(address) {
  setMessage(`Downloading ${address}…`);
  try {
    const response = await fetch(address);
    if (!response.ok) {
      setMessage(`Could not open the URL: the server answered ${response.status}.`);
      return;
    }
    const data = new Uint8Array(await response.arrayBuffer());
    let name = decodeURIComponent(new URL(address).pathname.split('/').pop()) || 'document.pdf';
    if (!name.toLowerCase().endsWith('.pdf')) name += '.pdf';
    await openPdf(data, name);
  } catch {
    setMessage('Could not download the PDF. Check the address, CORS, and browser blocking settings.');
  }
}

function applyInverted(inverted) {
  for (const view of pageViews) view.canvas.classList.toggle('inverted', inverted);
  $('invert').setAttribute('aria-pressed', String(inverted));
  try {
    localStorage.setItem('viewer:invert', String(inverted));
  } catch {
    // The preference is optional when storage is disabled.
  }
}

// Theater mode hides the toolbar and status so only the document remains.
function setTheater(enabled) {
  document.body.classList.toggle('chrome-hidden', enabled);
  $('theater').setAttribute('aria-pressed', String(enabled));
  $('exit-theater').hidden = !enabled;
  if (enabled) $('exit-theater').focus();
}

// Full screen is the browser's own, so it also hides the browser chrome.
function toggleFullscreen() {
  if (document.fullscreenElement) document.exitFullscreen().catch(() => {});
  else document.documentElement.requestFullscreen?.().catch(() => setMessage('Full screen is not available.'));
}

const renderObserver = new IntersectionObserver((entries) => {
  for (const entry of entries) {
    const view = pageViews.find((candidate) => candidate.frame === entry.target);
    if (!view) continue;
    view.near = entry.isIntersecting;
    if (view.near) renderPageView(view);
  }
}, { root: workspace, rootMargin: '100% 0px' });

for (const menu of document.querySelectorAll('.menu')) {
  const button = $(menu.getAttribute('aria-labelledby'));
  menu.addEventListener('beforetoggle', (event) => {
    const open = event.newState === 'open';
    button.setAttribute('aria-expanded', String(open));
    if (!open) return;
    const { left, bottom } = button.getBoundingClientRect();
    menu.style.left = `${left}px`;
    menu.style.top = `${bottom + 4}px`;
  });
  menu.addEventListener('toggle', (event) => {
    if (event.newState === 'open') menu.querySelector('button').focus();
  });
  menu.addEventListener('click', (event) => {
    if (event.target.closest('button')) menu.hidePopover();
  });
  menu.addEventListener('keydown', (event) => {
    const step = { ArrowDown: 1, ArrowUp: -1 }[event.key];
    if (!step) return;
    const items = [...menu.querySelectorAll(':scope > button:not([hidden])')];
    items[(items.indexOf(document.activeElement) + step + items.length) % items.length].focus();
    event.preventDefault();
  });
}

for (const id of ['open', 'welcome-open']) $(id).addEventListener('click', () => fileInput.click());
fileInput.addEventListener('change', () => {
  openFile(fileInput.files[0]);
  fileInput.value = '';
});

$('previous').addEventListener('click', () => goToPage(pageNumber - 1));
$('next').addEventListener('click', () => goToPage(pageNumber + 1));
pageInput.addEventListener('change', () => goToPage(pageInput.value));
pageInput.addEventListener('keydown', (event) => {
  if (event.key === 'Enter') pageInput.blur();
});
zoomInput.addEventListener('change', () => {
  if (zoomInput.value !== 'custom') removeCustomZoom();
  updatePageSizes();
});
$('zoom-out').addEventListener('click', () => stepZoom(-1));
$('zoom-in').addEventListener('click', () => stepZoom(1));
$('invert').addEventListener('click', () => applyInverted($('invert').getAttribute('aria-pressed') !== 'true'));
$('theater').addEventListener('click', () => setTheater(true));
$('fullscreen').addEventListener('click', toggleFullscreen);
$('exit-theater').addEventListener('click', () => setTheater(false));
document.addEventListener('fullscreenchange', () => {
  $('fullscreen').setAttribute('aria-pressed', String(Boolean(document.fullscreenElement)));
});

const urlDialog = $('url-dialog');
$('open-url').addEventListener('click', () => {
  urlDialog.returnValue = '';
  urlDialog.showModal();
  $('url-input').select();
});
$('url-cancel').addEventListener('click', () => urlDialog.close('cancel'));
urlDialog.addEventListener('close', () => {
  if (urlDialog.returnValue === 'open') openUrl($('url-input').value.trim());
});

$('password-cancel').addEventListener('click', () => passwordDialog.close('cancel'));
passwordDialog.addEventListener('close', () => {
  const callback = passwordCallback;
  passwordCallback = null;
  if (callback) {
    callback(passwordDialog.returnValue === 'open' ? $('password-input').value : new Error('Password prompt cancelled.'));
  }
});

document.addEventListener('paste', (event) => {
  const file = pdfFrom(event.clipboardData?.files);
  if (!file) return;
  event.preventDefault();
  openFile(file);
});

workspace.addEventListener('dragover', (event) => {
  // Some browsers keep the file list private until the drop event.
  if (!event.dataTransfer?.types.includes('Files')) return;
  event.preventDefault();
  workspace.classList.add('dragover');
});
workspace.addEventListener('dragleave', (event) => {
  if (!workspace.contains(event.relatedTarget)) workspace.classList.remove('dragover');
});
workspace.addEventListener('drop', (event) => {
  const file = pdfFrom(event.dataTransfer?.files);
  if (!file) return;
  event.preventDefault();
  workspace.classList.remove('dragover');
  openFile(file);
});

workspace.addEventListener('scroll', () => {
  if (scrollFrame) return;
  scrollFrame = requestAnimationFrame(updateCurrentPage);
}, { passive: true });

document.addEventListener('keydown', (event) => {
  if (document.querySelector('dialog[open]')) return;
  const key = event.key.toLowerCase();
  if ((event.ctrlKey || event.metaKey) && key === 'o') {
    event.preventDefault();
    fileInput.click();
    return;
  }
  if (event.ctrlKey || event.metaKey || event.altKey || event.target.closest?.('.menu')) return;
  if (event.target.matches?.('input, select, textarea')) return;
  if (key === 'escape' && document.body.classList.contains('chrome-hidden')) {
    setTheater(false);
    return;
  }
  if (!pdf) return;

  if (key === 'i') {
    applyInverted($('invert').getAttribute('aria-pressed') !== 'true');
  } else if (key === 't') {
    setTheater(!document.body.classList.contains('chrome-hidden'));
  } else if (key === 'f') {
    toggleFullscreen();
  } else if (key === 'pageup' || key === 'arrowleft') {
    event.preventDefault();
    goToPage(pageNumber - 1);
  } else if (key === 'pagedown' || key === ' ' || key === 'arrowright') {
    event.preventDefault();
    goToPage(pageNumber + 1);
  } else if (key === 'arrowup' || key === 'arrowdown') {
    event.preventDefault();
    workspace.scrollBy({ top: key === 'arrowup' ? -ARROW_SCROLL : ARROW_SCROLL });
  } else if (key === 'home') {
    event.preventDefault();
    goToPage(1);
  } else if (key === 'end') {
    event.preventDefault();
    goToPage(pdf.numPages);
  } else if (key === '-' || key === '_') {
    event.preventDefault();
    stepZoom(-1);
  } else if (key === '+' || key === '=') {
    event.preventDefault();
    stepZoom(1);
  } else if (key === '0') {
    removeCustomZoom();
    zoomInput.value = 'fit-page';
    updatePageSizes();
  }
});

// Ctrl+wheel and trackpad pinch zoom the page rather than the browser UI.
document.addEventListener('wheel', (event) => {
  if ((!event.ctrlKey && !event.metaKey) || !pdf || document.querySelector('dialog[open]')) return;
  event.preventDefault();
  const unit = event.deltaMode === WheelEvent.DOM_DELTA_PIXEL ? 1 : 16;
  const delta = Math.max(-50, Math.min(50, event.deltaY * unit));
  const currentScale = pageViews[pageNumber - 1]?.targetScale ?? 1;
  zoomTo(currentScale * Math.exp(-delta * 0.005), { clientX: event.clientX, clientY: event.clientY });
}, { passive: false });

new ResizeObserver(() => {
  if (!pdf || !zoomInput.value.startsWith('fit-')) return;
  clearTimeout(resizeTimer);
  resizeTimer = setTimeout(updatePageSizes, 100);
}).observe(workspace);

window.addEventListener('beforeunload', () => {
  loadingTask?.destroy();
});

try {
  applyInverted(localStorage.getItem('viewer:invert') === 'true');
} catch {
  applyInverted(false);
}
clearDocument();
