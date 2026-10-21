// Speech engines, and putting an episode's lines together into one track.
//
// Every engine's speak(text, voice, speed) resolves to {samples, rate}:
// mono Float32Array samples at RATE.

export const RATE = 24000;

// Pauses between lines, in seconds: a host going on, the other host
// answering, and a new segment.
const GAP_SAME = 0.2;
const GAP_TURN = 0.32;
const GAP_SEGMENT = 0.9;

/**
 * Kokoro in this browser. On the GPU that's one worker; on the CPU it's
 * several, each speaking its own line on a few threads, as a model this
 * small gains little from more threads than that.
 */
export class BrowserEngine {
  constructor(onProgress) {
    this.onProgress = onProgress;
    this.workers = [];
    this.idle = [];
    this.queue = [];
    this.next = 0;
    this.ready = null;
    const cores = navigator.hardwareConcurrency || 4;
    this.size = Math.max(1, Math.min(4, Math.floor(cores / 4)));
    this.threads = Math.max(1, Math.min(4, Math.floor(cores / this.size)));
  }

  /** Starts the first worker; on the CPU, the rest follow once it's ready. */
  load() {
    if (this.ready) return this.ready;
    this.ready = this.start({ threads: this.threads }, this.onProgress).then((ready) => {
      if (ready.device === 'webgpu') this.where = 'on the GPU';
      else {
        // The model is in the browser's cache by now, so these are quick.
        for (let i = 1; i < this.size; i++) this.start({ device: 'wasm', threads: this.threads }).catch(() => {});
        this.where = `on the CPU, ${this.size} at a time`;
      }
      return ready;
    });
    this.ready.catch(() => {
      this.ready = null;
    });
    return this.ready;
  }

  start(options, onProgress) {
    const worker = new Worker(new URL('./tts-worker.js', import.meta.url), { type: 'module' });
    const ready = new Promise((resolve, reject) => {
      worker.onmessage = ({ data }) => {
        if (data.type === 'progress') onProgress?.(data);
        else if (data.type === 'ready') {
          this.workers.push(worker);
          this.free(worker);
          resolve(data);
        } else if (data.type === 'audio' || (data.type === 'error' && data.id !== undefined)) {
          const job = worker.job;
          worker.job = null;
          if (data.type === 'audio') job?.resolve({ samples: data.samples, rate: data.rate });
          else job?.reject(new Error(data.error));
          this.free(worker);
        } else if (data.type === 'error') {
          worker.terminate();
          reject(new Error(data.error));
        }
      };
      worker.onerror = (e) => {
        worker.terminate();
        const err = new Error(e.message || 'the speech worker stopped');
        reject(err);
        // A worker can also fail after it's ready: its line fails, and if
        // no workers are left, so do the waiting lines, and the next line
        // starts the engine again.
        worker.job?.reject(err);
        worker.job = null;
        this.workers = this.workers.filter((w) => w !== worker);
        this.idle = this.idle.filter((w) => w !== worker);
        if (!this.workers.length && this.ready) {
          this.ready = null;
          this.where = '';
          for (const job of this.queue.splice(0)) job.reject(err);
        }
      };
    });
    worker.postMessage({ type: 'load', ...options });
    return ready;
  }

  free(worker) {
    const job = this.queue.shift();
    if (!job) {
      this.idle.push(worker);
      return;
    }
    worker.job = job;
    worker.postMessage({ type: 'speak', id: this.next++, ...job.line });
  }

  async speak(text, voice, speed) {
    await this.load();
    const { samples, rate } = await new Promise((resolve, reject) => {
      this.queue.push({ line: { text, voice, speed }, resolve, reject });
      if (this.idle.length) this.free(this.idle.shift());
    });
    return { samples: rate === RATE ? samples : resample(samples, rate, RATE), rate: RATE };
  }

  /** Drops the lines waiting for a worker and stops waiting on the rest. */
  cancel() {
    const stopped = new DOMException('stopped', 'AbortError');
    for (const job of this.queue.splice(0)) job.reject(stopped);
    // A worker can't be interrupted; it finishes its line and is free.
    for (const worker of this.workers) {
      worker.job?.reject(stopped);
      worker.job = null;
    }
  }
}

/** Speech on the notebook server, through its /api/tts. */
export class ServerEngine {
  constructor() {
    this.abort = new AbortController();
  }

  async speak(text, voice, speed) {
    const res = await fetch('api/tts', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ input: text, voice, speed }),
      signal: this.abort.signal,
    });
    if (!res.ok) {
      const err = await res.json().catch(() => ({}));
      throw new Error(err.error || `speech server: ${res.status}`);
    }
    const buf = await res.arrayBuffer();
    // Decoding at RATE resamples whatever the server sent.
    const ctx = new OfflineAudioContext(1, 1, RATE);
    const audio = await ctx.decodeAudioData(buf);
    return { samples: audio.getChannelData(0).slice(), rate: RATE };
  }

  /** Stops the lines being spoken, which frees the server for others. */
  cancel() {
    this.abort.abort(new DOMException('stopped', 'AbortError'));
    this.abort = new AbortController();
  }
}

/** Linear resampling: good enough for speech between nearby rates. */
export function resample(samples, from, to) {
  const out = new Float32Array(Math.round((samples.length * to) / from));
  const step = from / to;
  for (let i = 0; i < out.length; i++) {
    const x = i * step;
    const j = Math.floor(x);
    const f = x - j;
    out[i] = (samples[j] ?? 0) * (1 - f) + (samples[j + 1] ?? samples[j] ?? 0) * f;
  }
  return out;
}

/** Cuts the silence a model leaves around speech, keeping a little. */
export function trim(samples, rate = RATE, threshold = 0.004, keep = 0.03) {
  let start = 0;
  let end = samples.length;
  while (start < end && Math.abs(samples[start]) < threshold) start++;
  while (end > start && Math.abs(samples[end - 1]) < threshold) end--;
  const pad = Math.round(keep * rate);
  return samples.subarray(Math.max(0, start - pad), Math.min(samples.length, end + pad));
}

/** The pause before clip, in seconds, after prev (if any). */
export function gap(prev, clip) {
  if (!prev) return 0;
  if (clip.segment !== prev.segment) return GAP_SEGMENT;
  return clip.speaker === prev.speaker ? GAP_SAME : GAP_TURN;
}

/**
 * Joins clips into one track. Each clip is {samples, speaker, segment}; the
 * pause before it depends on whether the speaker or segment changed.
 * Returns the track and where each clip starts, in seconds.
 */
export function assemble(clips, rate = RATE) {
  const gaps = clips.map((c, i) => gap(clips[i - 1], c));
  const trimmed = clips.map((c) => trim(c.samples, rate));
  const length = trimmed.reduce((n, s, i) => n + s.length + Math.round(gaps[i] * rate), 0);
  const track = new Float32Array(length);
  const starts = [];
  let at = 0;
  trimmed.forEach((s, i) => {
    at += Math.round(gaps[i] * rate);
    starts.push(at / rate);
    track.set(s, at);
    at += s.length;
  });
  return { track, starts };
}

/** 16-bit PCM WAV. */
export function wav(samples, rate = RATE) {
  const buf = new ArrayBuffer(44 + samples.length * 2);
  const v = new DataView(buf);
  const str = (o, s) => [...s].forEach((c, i) => v.setUint8(o + i, c.charCodeAt(0)));
  str(0, 'RIFF');
  v.setUint32(4, 36 + samples.length * 2, true);
  str(8, 'WAVE');
  str(12, 'fmt ');
  v.setUint32(16, 16, true);
  v.setUint16(20, 1, true);
  v.setUint16(22, 1, true);
  v.setUint32(24, rate, true);
  v.setUint32(28, rate * 2, true);
  v.setUint16(32, 2, true);
  v.setUint16(34, 16, true);
  str(36, 'data');
  v.setUint32(40, samples.length * 2, true);
  for (let i = 0; i < samples.length; i++) {
    const s = Math.max(-1, Math.min(1, samples[i]));
    v.setInt16(44 + i * 2, s < 0 ? s * 0x8000 : s * 0x7fff, true);
  }
  return new Blob([buf], { type: 'audio/wav' });
}

/** Stores samples as 16-bit, half the space of floats. */
export function toInt16(samples) {
  const out = new Int16Array(samples.length);
  for (let i = 0; i < samples.length; i++) {
    const s = Math.max(-1, Math.min(1, samples[i]));
    out[i] = s < 0 ? s * 0x8000 : s * 0x7fff;
  }
  return out;
}

export function fromInt16(samples) {
  const out = new Float32Array(samples.length);
  for (let i = 0; i < samples.length; i++) out[i] = samples[i] / (samples[i] < 0 ? 0x8000 : 0x7fff);
  return out;
}
