/**
 * A run's seed as something a person can read out, type back and send.
 *
 * The engine's seed is 31 random bits, which as a number is up to ten digits
 * that mean nothing and are easy to transpose. This spells the same bits as
 * seven characters of Crockford's base 32: five bits a character, so seven hold
 * thirty-five where six would hold thirty, one short, and every code therefore
 * starts with 0 or 1. The alphabet leaves out I, L, O and U, so the letters a
 * hand confuses with digits are read back as those digits rather than refused.
 * It is a spelling and not a hash: every code is exactly one seed, which is
 * what lets the pause sheet show the run in hand and a friend's link deal
 * that run and not a lookalike.
 *
 * A seed alone is not a run. The same seed dealt from the French list draws a
 * different answer from the English one, and the ascension bends every target,
 * so the link carries both beside the code. The words are the part people will
 * forget: "same seed, different word" would be the first bug report.
 */

import { clampAscension } from "../engine"
import type { Lang } from "./lang"
import { LANGS } from "./lang"

/** The public site. A link has to open somewhere, and the APK's origin is not a place. */
export const SITE_URL = "https://5-wild.com"

const ALPHABET = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
const LENGTH = 7
/** One past the largest seed `rootSeed` can draw. */
const SEEDS = 2 ** 31

export function seedCode(seed: number): string {
  let rest = seed
  let code = ""
  for (let i = 0; i < LENGTH; i++) {
    code = ALPHABET[rest % 32] + code
    rest = Math.floor(rest / 32)
  }
  return code
}

/**
 * The seed a typed code names, or null. Forgiving about the ways a person
 * copies seven characters (case, spaces, a dash to split them, the letter O
 * for a zero) and strict about everything else: a code that is too short
 * is a typo, not a request for a run the player never saw.
 */
export function parseSeed(text: string): number | null {
  const clean = text.toUpperCase().replace(/[\s-]/g, "").replace(/O/g, "0").replace(/[IL]/g, "1")
  if (clean.length !== LENGTH) return null
  let seed = 0
  for (const ch of clean) {
    const digit = ALPHABET.indexOf(ch)
    if (digit < 0) return null
    seed = seed * 32 + digit
  }
  return seed < SEEDS ? seed : null
}

/** A run someone else dealt, as far as dealing it again needs. */
export type SeededOffer = {
  seed: number
  ascension: number
  /** The list the run was dealt from, which is not the interface language. */
  words: Lang
}

/**
 * The link the pause sheet copies. Defaults are left off, so the common case
 * (English, no ascension) is a link short enough to read aloud.
 */
export function seedLink(seed: number, ascension: number, words: Lang): string {
  const params = new URLSearchParams({ seed: seedCode(seed) })
  if (ascension > 0) params.set("a", String(ascension))
  if (words !== "en") params.set("w", words)
  return `${SITE_URL}/?${params}`
}

/**
 * The run a `?seed=` link asks for, or null if it asks for none. A bad
 * ascension is clamped rather than refused, since the code is the part that
 * matters; an unknown or absent `w` is the list the link's own default says,
 * English, because that is what `seedLink` leaves off.
 */
export function readSeedParam(search: string): SeededOffer | null {
  const params = new URLSearchParams(search)
  const code = params.get("seed")
  const seed = code === null ? null : parseSeed(code)
  if (seed === null) return null
  const words = params.get("w")
  return {
    seed,
    ascension: clampAscension(Number(params.get("a") ?? 0)),
    words: LANGS.includes(words as Lang) ? (words as Lang) : "en",
  }
}

/**
 * The run a link pasted into the seed field asks for, or null if the text is
 * not one. The copy button puts a link on the clipboard, not a code, so pasting
 * what was copied is the likeliest thing anyone types there, and a link read
 * for its code alone would drop the level and list it carries.
 */
export function readPastedLink(text: string): SeededOffer | null {
  const query = text.trim().indexOf("?")
  return query < 0 ? null : readSeedParam(text.trim().slice(query))
}
