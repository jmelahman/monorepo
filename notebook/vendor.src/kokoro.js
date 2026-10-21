// What the page uses of kokoro-js, and transformers.js's env, through which
// the page points it at the vendored ONNX runtime rather than a CDN.
export { KokoroTTS } from 'kokoro-js';
export { env } from '@huggingface/transformers';
