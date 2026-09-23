/**
 * Seeded randomness.
 *
 * JavaScript has no seeded PRNG in its standard library, and everything random
 * in this game, the answer and the shop and the boss order, has to replay
 * identically from a saved run and from a golden test vector. So we carry our
 * own, and the engine never touches Math.random.
 */

export type Rng = () => number

/** sfc32: 128-bit state, 32-bit ops only, no BigInt. Fast and good enough. */
export function sfc32(a: number, b: number, c: number, d: number): Rng {
  let [x, y, z, w] = [a >>> 0, b >>> 0, c >>> 0, d >>> 0]
  return () => {
    const t = (((x + y) | 0) + w) | 0
    w = (w + 1) | 0
    x = y ^ (y >>> 9)
    y = (z + (z << 3)) | 0
    z = (z << 21) | (z >>> 11)
    z = (z + t) | 0
    return (t >>> 0) / 4294967296
  }
}

/** xmur3: mixes a string into a stream of 32-bit words to seed sfc32. */
function xmur3(str: string): () => number {
  let h = 1779033703 ^ str.length
  for (let i = 0; i < str.length; i++) {
    h = Math.imul(h ^ str.charCodeAt(i), 3432918353)
    h = (h << 13) | (h >>> 19)
  }
  return () => {
    h = Math.imul(h ^ (h >>> 16), 2246822507)
    h = Math.imul(h ^ (h >>> 13), 3266489909)
    h ^= h >>> 16
    return h >>> 0
  }
}

/**
 * One root seed per run; every consumer derives an independent sub-stream from
 * a coordinate. `derive(seed, "word", 3, 1)` always yields the same stream, so
 * rerolling the shop cannot shift which word stage 3 gets: the streams do not
 * share a cursor. That independence is what makes save/resume and the golden
 * vectors work without storing any PRNG state.
 *
 * Which makes the string coordinates *content*, not names. They are hashed, so
 * `"joker"` and `"relic"` are different streams and every seed in existence
 * would deal itself a different run if one became the other, a balance change
 * wearing a rename's clothes. Three literals still spell the old vocabulary for
 * exactly that reason (`"joker"` in `scoring.ts`, `"blind_start"` and
 * `"blind_end"` in `reduce.ts`), and they are frozen. Rename them only as a
 * deliberate reshuffle: bump `CONTENT_VERSION` and re-record the vectors.
 */
export function derive(root: number, ...coord: ReadonlyArray<string | number>): Rng {
  const next = xmur3(`${root}:${coord.join(":")}`)
  return sfc32(next(), next(), next(), next())
}

export function randomInt(rng: Rng, max: number): number {
  return Math.floor(rng() * max)
}

export function pick<T>(rng: Rng, items: readonly T[]): T {
  const item = items[randomInt(rng, items.length)]
  if (item === undefined) throw new Error("pick from empty list")
  return item
}

export function shuffled<T>(rng: Rng, items: readonly T[]): T[] {
  const out = [...items]
  for (let i = out.length - 1; i > 0; i--) {
    const j = randomInt(rng, i + 1)
    const a = out[i]
    const b = out[j]
    if (a === undefined || b === undefined) continue
    out[i] = b
    out[j] = a
  }
  return out
}
