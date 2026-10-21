// Speaks lines with Kokoro, off the page's thread. The model comes from the
// Hugging Face Hub on first use and the browser keeps it; the ONNX runtime
// that runs it is vendored beside this file.
//
// Messages in:  {type: 'load', device?, threads?} | {type: 'speak', id, text, voice, speed}
// Messages out: {type: 'progress', loaded, total, file} | {type: 'ready', device, threads, voices}
//               | {type: 'audio', id, samples, rate} | {type: 'error', id?, error}

import { KokoroTTS, env } from './vendor/kokoro/kokoro.mjs';

const MODEL = 'onnx-community/Kokoro-82M-v1.0-ONNX';
// Kokoro reads about 500 phonemes at a time; longer text is cut short, so
// longer lines are spoken a few sentences at a time, and longer sentences a
// few clauses or words at a time.
const MAX_CHARS = 350;
const SPLITS = [/(?<=[.!?…]["'”’)]*)\s+/, /(?<=[,;:—–])\s+/, /\s+/];

env.backends.onnx.wasm.wasmPaths = new URL('./vendor/kokoro/', import.meta.url).href;

let tts;
let loading;

/** Whether there's a real GPU; a software one is far slower than WASM. */
async function webgpu() {
  try {
    const adapter = await navigator.gpu?.requestAdapter();
    if (!adapter || adapter.info?.isFallbackAdapter) return false;
    const { vendor = '', architecture = '', description = '' } = adapter.info ?? {};
    return !/swiftshader|llvmpipe|lavapipe/i.test(`${vendor} ${architecture} ${description}`);
  } catch {
    return false;
  }
}

async function load(options) {
  // Kokoro needs full precision on WebGPU; on the CPU, 8-bit is about as
  // good and several times faster.
  const device = options.device ?? ((await webgpu()) ? 'webgpu' : 'wasm');
  // Threads need a cross-origin isolated page (the server's COOP and COEP
  // headers).
  env.backends.onnx.wasm.numThreads = self.crossOriginIsolated ? options.threads || 4 : 1;
  const files = new Map();
  tts = await KokoroTTS.from_pretrained(MODEL, {
    device,
    dtype: device === 'webgpu' ? 'fp32' : 'q8',
    progress_callback: (p) => {
      if (p.status !== 'progress' || !p.total) return;
      files.set(p.file, [p.loaded, p.total]);
      let loaded = 0;
      let total = 0;
      for (const [l, t] of files.values()) {
        loaded += l;
        total += t;
      }
      postMessage({ type: 'progress', loaded, total, file: p.file });
    },
  });
  const voices = Object.entries(tts.voices)
    .filter(([id]) => id.startsWith('a') || id.startsWith('b'))
    .map(([id, v]) => ({ id, name: v.name, gender: v.gender, grade: v.overallGrade, language: v.language }));
  const threads = device === 'wasm' ? env.backends.onnx.wasm.numThreads || 1 : 0;
  postMessage({ type: 'ready', device, threads, voices });
}

/**
 * Splits text into chunks of at most MAX_CHARS, between sentences where
 * that will do, else between clauses, else between words.
 */
function chunks(text, level = 0) {
  text = text.trim();
  if (text.length <= MAX_CHARS) return text ? [text] : [];
  if (level === SPLITS.length) return text.match(new RegExp(`[^]{1,${MAX_CHARS}}`, 'g'));
  const out = [];
  let cur = '';
  for (const part of text.split(SPLITS[level])) {
    if (cur && cur.length + 1 + part.length > MAX_CHARS) {
      out.push(cur);
      cur = '';
    }
    cur = cur ? `${cur} ${part}` : part;
  }
  if (cur) out.push(cur);
  return out.flatMap((c) => chunks(c, level + 1));
}

async function speak({ id, text, voice, speed }) {
  const parts = [];
  let rate = 24000;
  for (const chunk of chunks(text)) {
    const audio = await tts.generate(chunk, { voice, speed });
    parts.push(audio.audio);
    rate = audio.sampling_rate;
  }
  const samples = new Float32Array(parts.reduce((n, p) => n + p.length, 0));
  let at = 0;
  for (const p of parts) {
    samples.set(p, at);
    at += p.length;
  }
  postMessage({ type: 'audio', id, samples, rate }, [samples.buffer]);
}

// One message at a time: the model isn't safe to run concurrently.
let queue = Promise.resolve();

onmessage = ({ data }) => {
  queue = queue.then(async () => {
    try {
      if (data.type === 'load') {
        loading ??= load(data);
        await loading;
      } else if (data.type === 'speak') {
        await (loading ??= load({}));
        await speak(data);
      }
    } catch (err) {
      if (data.type === 'load') loading = undefined;
      postMessage({ type: 'error', id: data.id, error: String(err?.message ?? err) });
    }
  });
};
