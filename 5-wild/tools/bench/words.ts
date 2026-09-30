/**
 * The shipped word lists, read off disk.
 *
 * Lifted out of `test/telemetry-report.test.ts`, which needed the same thing
 * for the same reason: a replay is only a replay against the bytes the run was
 * dealt from, and those are `public/words`, not a fixture. The benchmark host
 * and that test now read them through one function, so they cannot drift into
 * disagreeing about what a list is (a trailing newline, a blank line).
 */

import { readFileSync } from "node:fs"
import { dirname, join, resolve } from "node:path"
import { fileURLToPath } from "node:url"
import type { WordSource } from "../../src/engine"

const WORDS = resolve(dirname(fileURLToPath(import.meta.url)), "..", "..", "public", "words")

const lists = new Map<string, WordSource>()

export function wordsFor(lang: string): WordSource {
  let words = lists.get(lang)
  if (!words) {
    const read = (name: string) =>
      readFileSync(join(WORDS, lang, `${name}.txt`), "utf8")
        .split("\n")
        .filter(Boolean)
    words = { answers: read("answers"), allowed: new Set(read("allowed")) }
    lists.set(lang, words)
  }
  return words
}
