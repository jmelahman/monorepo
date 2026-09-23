import { readFileSync } from "node:fs"
import { dirname, join, resolve } from "node:path"
import { fileURLToPath } from "node:url"
import type { WordSource } from "../../src/engine"

const WORDS = resolve(dirname(fileURLToPath(import.meta.url)), "..", "..", "public", "words")

const load = (lang: string, name: string) =>
  readFileSync(join(WORDS, lang, `${name}.txt`), "utf8")
    .split("\n")
    .filter(Boolean)

/**
 * The real shipped lists, for tests that want to exercise the game as played.
 *
 * English, and hard-coded to it rather than parameterised. The golden vectors
 * are replayed against these bytes and a vector is a recording of a run in one
 * language; pointing this at a setting would make every recorded run depend on
 * whichever list a machine happened to be configured for.
 */
export const realWords: WordSource = {
  answers: load("en", "answers"),
  allowed: new Set(load("en", "allowed")),
}
