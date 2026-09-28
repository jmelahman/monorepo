/**
 * The run save and its log, enciphered at rest.
 *
 * What this is for, exactly: `round.answer` sits in the save, and so does the
 * seed that deals it, so a player who opened devtools' storage panel read the
 * word they were about to guess. This stops that and nothing more. The key is
 * in the bundle, because a game that plays offline has nowhere else to keep it,
 * so anyone who reads the source can unseal a save; and the answer is in memory
 * the whole round anyway, since `computeFeedback` needs it. Real secrecy would
 * mean a server dealing the words and scoring the guesses, which is the offline
 * APK and the pure reducer both given up for a single-player game.
 *
 * Given that, the cipher is chosen for being synchronous rather than strong.
 * WebCrypto's AES is async only, and `loadSave` runs before the app exists in
 * `main.ts` while `save` runs on every dispatch; making both async would reorder
 * startup and race the saves against each other for a secret that still ships
 * in the bundle. A keystream XOR does the same job in the same breath.
 *
 * A fresh nonce per write means the same run never seals to the same text
 * twice, so there is no pattern to learn by saving the same answer over.
 *
 * Nothing here touches `RunState`: the engine still hands back plain JSON, and
 * sealing happens at the storage edge, beside the language and the log.
 */

const PREFIX = "s1."
const KEY = 0x5f3759df

/** mulberry32, keyed by the nonce. Four bytes of keystream per step. */
function keystream(nonce: number): () => number {
  let a = (KEY ^ nonce) >>> 0
  return () => {
    a = (a + 0x6d2b79f5) >>> 0
    let t = a
    t = Math.imul(t ^ (t >>> 15), t | 1)
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61)
    return (t ^ (t >>> 14)) >>> 0
  }
}

function xor(bytes: Uint8Array, nonce: number): Uint8Array {
  const next = keystream(nonce)
  const out = new Uint8Array(bytes.length)
  let word = 0
  for (let i = 0; i < bytes.length; i++) {
    if (i % 4 === 0) word = next()
    out[i] = (bytes[i] ?? 0) ^ ((word >>> ((i % 4) * 8)) & 0xff)
  }
  return out
}

// A loop rather than `String.fromCharCode(...bytes)`: a long run's log is tens
// of kilobytes, and spreading that many arguments overflows the stack.
function toBase64(bytes: Uint8Array): string {
  let binary = ""
  for (const byte of bytes) binary += String.fromCharCode(byte)
  return btoa(binary)
}

function fromBase64(text: string): Uint8Array {
  const binary = atob(text)
  const bytes = new Uint8Array(binary.length)
  for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i)
  return bytes
}

export function seal(plain: string): string {
  const [nonce = 0] = crypto.getRandomValues(new Uint32Array(1))
  const body = xor(new TextEncoder().encode(plain), nonce)
  const out = new Uint8Array(4 + body.length)
  new DataView(out.buffer).setUint32(0, nonce)
  out.set(body, 4)
  return PREFIX + toBase64(out)
}

/**
 * The inverse of `seal`, and of nothing sealed at all: a value that is not ours
 * comes back as it went in. That is how a save from before this module still
 * resumes, since the old builds wrote bare JSON and bare JSON never starts with
 * the prefix. It needs no key bump for the same reason: the fields mean what
 * they meant, only the envelope changed.
 *
 * Throws on a sealed value that will not decode, which every caller already
 * answers by treating the save as missing.
 */
export function unseal(stored: string): string {
  if (!stored.startsWith(PREFIX)) return stored
  const bytes = fromBase64(stored.slice(PREFIX.length))
  if (bytes.length < 4) throw new Error("sealed value too short")
  const nonce = new DataView(bytes.buffer).getUint32(0)
  return new TextDecoder("utf-8", { fatal: true }).decode(xor(bytes.subarray(4), nonce))
}
