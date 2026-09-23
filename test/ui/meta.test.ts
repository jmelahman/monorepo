import { afterEach, beforeEach, describe, expect, it } from "vitest"
import { MAX_ASCENSION } from "../../src/engine"
import type { MetaState } from "../../src/ui/meta"
import {
  chosenAscension,
  crackedIn,
  favoriteRelics,
  favoriteWord,
  isLocked,
  loadMeta,
  meanSolve,
  Profile,
  playedIn,
  roundsPlayed,
  unlocked,
  wordsFound,
} from "../../src/ui/meta"

const KEY = "5wild:meta:v2"

/**
 * A store, standing in for the browser's.
 *
 * The tests run in node, where there is no `localStorage` to lean on, and the
 * point of this module is what it does with one, including what it does with
 * one that has stopped cooperating, which no real store will do on demand.
 */
class FakeStorage {
  readonly items = new Map<string, string>()
  /** Set to make every write throw, the way a full or blocked store does. */
  sealed = false

  getItem(key: string): string | null {
    return this.items.get(key) ?? null
  }

  setItem(key: string, value: string): void {
    if (this.sealed) throw new Error("QuotaExceededError")
    this.items.set(key, value)
  }
}

let store: FakeStorage

const stored = (): unknown => JSON.parse(store.items.get(KEY) ?? "null")

const FRESH: MetaState = {
  runs: 0,
  wins: 0,
  bestStage: 0,
  cleared: -1,
  ascension: 0,
  guesses: 0,
  solves: [],
  missed: 0,
  streak: 0,
  bestStreak: 0,
  crackedBy: {},
  playedBy: {},
  words: {},
  wordError: {},
  relics: {},
}

beforeEach(() => {
  store = new FakeStorage()
  Object.defineProperty(globalThis, "localStorage", { value: store, configurable: true })
})

afterEach(() => {
  Reflect.deleteProperty(globalThis, "localStorage")
})

describe("reading the record", () => {
  it("starts everyone at nothing, with no win to their name", () => {
    expect(loadMeta()).toEqual(FRESH)
  })

  it("reads back what was written", () => {
    const record: MetaState = {
      runs: 12,
      wins: 2,
      bestStage: 14,
      cleared: 1,
      ascension: 2,
      guesses: 431,
      solves: [0, 1, 4, 19, 22, 9],
      missed: 7,
      streak: 3,
      bestStreak: 11,
      crackedBy: { en: ["crane", "slate"], es: ["actor"] },
      playedBy: { en: ["crane", "slate", "torch"], es: ["actor"] },
      words: { crane: 88, slate: 12 },
      wordError: { slate: 11 },
      relics: { snowball: 3 },
    }
    store.items.set(KEY, JSON.stringify(record))
    expect(loadMeta()).toEqual(record)
  })

  it("keeps the fields it can read when one of them is garbage", () => {
    // The opposite of `loadSave`, which throws a bad run away whole. Independent
    // counters: a broken one is no reason to forget the rest.
    store.items.set(KEY, JSON.stringify({ runs: 9, wins: "lots", bestStage: 6.5, cleared: null }))
    expect(loadMeta()).toEqual({ ...FRESH, runs: 9 })
  })

  it("survives anything at all under its key", () => {
    for (const raw of ["", "not json", "null", "7", '"a string"', "[]"]) {
      store.items.set(KEY, raw)
      expect(loadMeta(), raw).toEqual(FRESH)
    }
  })

  it("reads a record written before a field existed as zero on it", () => {
    // Which is what every record in the wild looks like to the build that added
    // the ascension to it: a player mid-career, at the bottom of a new ladder.
    store.items.set(KEY, JSON.stringify({ runs: 3, wins: 1, bestStage: 8, cleared: 0 }))
    expect(loadMeta()).toEqual({ ...FRESH, runs: 3, wins: 1, bestStage: 8, cleared: 0 })
  })
})

describe("keeping the record", () => {
  it("counts runs as they are started", () => {
    const profile = new Profile()
    profile.started()
    profile.started()
    expect(profile.stats.runs).toBe(2)
    expect(stored()).toMatchObject({ runs: 2 })
  })

  it("keeps the deepest stage and ignores the shallower ones", () => {
    const profile = new Profile()
    profile.reached(1)
    profile.reached(7)
    profile.reached(3)
    expect(profile.stats.bestStage).toBe(7)
  })

  it("does not touch the store when the mark has not moved", () => {
    const profile = new Profile()
    profile.reached(5)
    store.items.delete(KEY)
    // Called on every save, which is every action of every run. A write per
    // keystroke would be the whole cost of the feature.
    profile.reached(4)
    profile.reached(5)
    expect(store.items.has(KEY)).toBe(false)
  })

  it("records the win and the level it was won at", () => {
    const profile = new Profile()
    profile.won(0)
    expect(profile.stats).toMatchObject({ wins: 1, cleared: 0 })
    // A second win at an easier level is still a win, but it does not un-clear
    // the harder one: `cleared` is a high-water mark, not a last-seen.
    profile.won(3)
    profile.won(1)
    expect(profile.stats).toMatchObject({ wins: 3, cleared: 3 })
  })

  it("picks up where the last session left off", () => {
    const first = new Profile()
    first.started()
    first.reached(9)
    first.won(0)
    expect(new Profile().stats).toEqual(first.stats)
  })

  it("remembers the level that was chosen, and writes nothing when it has not moved", () => {
    const profile = new Profile()
    profile.chose(3)
    expect(profile.stats.ascension).toBe(3)
    store.items.delete(KEY)
    profile.chose(3)
    expect(store.items.has(KEY)).toBe(false)
  })

  it("keeps counting for the session when the store stops taking writes", () => {
    const profile = new Profile()
    profile.started()
    store.sealed = true
    profile.started()
    // The number on screen stays true even though nothing will remember it.
    expect(profile.stats.runs).toBe(2)
    expect(stored()).toMatchObject({ runs: 1 })
  })
})

describe("what the ladder offers", () => {
  const meta = (change: Partial<MetaState>): MetaState => ({ ...FRESH, ...change })

  it("has earned nothing until the game has been won once", () => {
    // The dial is still on the title screen and every rung is reachable, but
    // nothing above the ordinary game has been climbed to yet.
    expect(unlocked(FRESH)).toBe(0)
    expect(chosenAscension(FRESH)).toBe(0)
  })

  it("earns exactly one rung above the hardest ever won", () => {
    expect(unlocked(meta({ cleared: 0 }))).toBe(1)
    expect(unlocked(meta({ cleared: 3 }))).toBe(4)
  })

  it("stops at the top of the ladder", () => {
    expect(unlocked(meta({ cleared: MAX_ASCENSION }))).toBe(MAX_ASCENSION)
  })

  it("starts a run where the dial was left", () => {
    expect(chosenAscension(meta({ cleared: 4, ascension: 2 }))).toBe(2)
  })

  it("lets a run start above anything ever won", () => {
    // The warning under the dial is the whole of the gate. A player who dialled
    // past their record gets the run they asked for, not the one they earned.
    expect(chosenAscension(meta({ cleared: 0, ascension: 5 }))).toBe(5)
    expect(chosenAscension(meta({ cleared: -1, ascension: 3 }))).toBe(3)
  })

  it("hands back a level the ladder does not have", () => {
    // A record can outlive the ladder it was written against. Reading it as the
    // nearest legal level is what stops that record from being an unstartable run.
    expect(chosenAscension(meta({ cleared: 999, ascension: 999 }))).toBe(MAX_ASCENSION)
    expect(chosenAscension(meta({ ascension: -4 }))).toBe(0)
  })

  it("locks every level above the hardest earned, and nothing at or below it", () => {
    const fresh = meta({ cleared: -1 })
    expect(isLocked(fresh, 0)).toBe(false)
    expect(isLocked(fresh, 1)).toBe(true)

    const won = meta({ cleared: 2 })
    expect(isLocked(won, 3)).toBe(false)
    expect(isLocked(won, 4)).toBe(true)
  })
})

describe("what the runs added up to", () => {
  it("counts every guess and remembers which words they were", () => {
    const profile = new Profile()
    for (const word of ["crane", "slate", "crane"]) profile.guessed(word, "en")
    expect(profile.stats.guesses).toBe(3)
    expect(profile.stats.words).toEqual({ crane: 2, slate: 1 })
    expect(favoriteWord(profile.stats)).toEqual({ word: "crane", count: 2 })
  })

  it("has no favorite before anything has been played", () => {
    expect(favoriteWord(loadMeta())).toBeNull()
  })

  it("never lets the word table outgrow its cap", () => {
    const profile = new Profile()
    // Far more distinct words than there are slots, each played once. A table
    // that grew with play would now hold two hundred entries and a save that
    // gets slower every session.
    for (let n = 0; n < 200; n++) profile.guessed(`w${n.toString().padStart(4, "0")}`, "en")
    expect(Object.keys(profile.stats.words).length).toBeLessThanOrEqual(24)
    expect(profile.stats.guesses).toBe(200)
  })

  it("does not credit a word with the count it inherited", () => {
    // What the table used to report, and the reason `wordError` exists: fill it
    // with one-offs until the eviction floor has climbed, and the next word in
    // takes that floor as its own. Every word here was played exactly once, so
    // the only honest answer to "how often" is one, whatever the table stores.
    const profile = new Profile()
    for (let n = 0; n < 200; n++) profile.guessed(`w${n.toString().padStart(4, "0")}`, "en")
    const stored = profile.stats.words
    expect(Math.max(...Object.values(stored))).toBeGreaterThan(1)
    for (const [word, count] of Object.entries(stored)) {
      expect(count - (profile.stats.wordError[word] ?? 0), word).toBe(1)
    }
    expect(favoriteWord(profile.stats)?.count).toBe(1)
  })

  it("counts a habit begun before the table filled exactly", () => {
    // The bracket is tight where it has to be: a word that never displaced
    // anything carries no correction at all and reads as itself.
    const profile = new Profile()
    for (let n = 0; n < 30; n++) profile.guessed("crane", "en")
    expect(profile.stats.wordError.crane).toBeUndefined()
    expect(favoriteWord(profile.stats)).toEqual({ word: "crane", count: 30 })
  })

  it("counts a habit begun after the table filled exactly too", () => {
    // The harder half: this one arrived at a full table and carries the floor it
    // displaced, and subtracting that lands back on the truth rather than merely
    // near it. Every `Profile` in this file reads the same fake store, so these
    // two are separate tests rather than one: sharing it would hand the second
    // profile the first one's cranes.
    const profile = new Profile()
    for (let n = 0; n < 60; n++) profile.guessed(`w${n.toString().padStart(4, "0")}`, "en")
    for (let n = 0; n < 40; n++) profile.guessed("crane", "en")
    expect(favoriteWord(profile.stats)).toEqual({ word: "crane", count: 40 })
  })

  it("finds the real favorite even when it started late", () => {
    // The failure a naive top-N has: fill the table with one-offs first, so a
    // newcomer's count of 1 can never beat an incumbent's, and the table freezes
    // on the first words ever typed. Space-Saving lets the newcomer in.
    const profile = new Profile()
    for (let n = 0; n < 60; n++) profile.guessed(`w${n.toString().padStart(4, "0")}`, "en")
    for (let n = 0; n < 40; n++) profile.guessed("crane", "en")
    expect(favoriteWord(profile.stats)?.word).toBe("crane")
  })

  it("breaks a tie the same way twice", () => {
    const profile = new Profile()
    profile.guessed("slate", "en")
    profile.guessed("crane", "en")
    // Alphabetical, not whichever key `Object.entries` happens to hand back
    // first, since the screen should not change its mind between renders.
    expect(favoriteWord(profile.stats)?.word).toBe("crane")
  })

  it("collects every valid guess, not only the ones that solved a round", () => {
    const profile = new Profile()
    profile.guessed("crane", "en")
    profile.guessed("slate", "en")
    profile.guessed("crane", "en")
    profile.solved("mound", 3, "en")
    // Sorted and deduplicated, exactly as `crackedBy` is, and holding the two
    // openers that never were the answer. The answer itself only lands here if
    // it was typed, which `solved` does not do on its own.
    expect(profile.stats.playedBy.en).toEqual(["crane", "slate"])
    expect(playedIn(profile.stats, "en")).toBe(2)
  })

  it("keeps a played collection per language", () => {
    const profile = new Profile()
    profile.guessed("crane", "en")
    // ACTOR is a word in both, and the two are counted against different
    // allowed lists, so the same reason `crackedBy` is keyed applies here.
    profile.guessed("actor", "en")
    profile.guessed("actor", "es")
    expect(playedIn(profile.stats, "en")).toBe(2)
    expect(playedIn(profile.stats, "es")).toBe(1)
    expect(playedIn(profile.stats, "fr")).toBe(0)
  })

  it("counts a solve under the guess that found it", () => {
    const profile = new Profile()
    profile.solved("crane", 4, "en")
    profile.solved("slate", 4, "en")
    profile.solved("mound", 2, "en")
    expect(profile.stats.solves).toEqual([0, 0, 1, 0, 2])
    expect(roundsPlayed(profile.stats)).toBe(3)
  })

  it("collects each answer once, however often it comes up", () => {
    const profile = new Profile()
    profile.solved("crane", 3, "en")
    profile.solved("crane", 5, "en")
    profile.solved("adobe", 4, "en")
    // Sorted, so the array is a set with an order and the screen can show it.
    expect(profile.stats.crackedBy.en).toEqual(["adobe", "crane"])
  })

  it("keeps one collection per language, counted against its own pool", () => {
    const profile = new Profile()
    profile.solved("crane", 3, "en")
    profile.solved("adobe", 4, "en")
    profile.solved("gorra", 3, "es")
    // Two collections, not one of five. The count is read as a fraction of one
    // language's answer list, and a Spanish word in the English numerator is a
    // percentage that can pass 100.
    expect(crackedIn(profile.stats, "en")).toBe(2)
    expect(crackedIn(profile.stats, "es")).toBe(1)
    // A language nobody has played is nothing found, not a missing field the
    // screen has to guard against.
    expect(crackedIn(profile.stats, "de")).toBe(0)
  })

  it("credits a word cracked in two languages to each of them", () => {
    const profile = new Profile()
    // ACTOR is an answer in English and in Spanish, and they are two different
    // words that happen to be spelled alike: cracking one has told the player
    // nothing about the other. Flat, the set would hold it once and the second
    // crack would count for nothing.
    profile.solved("actor", 3, "en")
    profile.solved("actor", 5, "es")
    expect(crackedIn(profile.stats, "en")).toBe(1)
    expect(crackedIn(profile.stats, "es")).toBe(1)
    // Still two rounds solved, which is the counter that was never per-language.
    expect(wordsFound(profile.stats)).toBe(2)
  })

  it("counts a round that ran out of guesses", () => {
    const profile = new Profile()
    profile.solved("crane", 3, "en")
    profile.missed()
    expect(roundsPlayed(profile.stats)).toBe(2)
    expect(profile.stats.crackedBy.en).toEqual(["crane"])
  })

  it("counts solves back to back and keeps the longest run of them", () => {
    const profile = new Profile()
    profile.solved("crane", 3, "en")
    profile.solved("slate", 4, "en")
    profile.solved("mound", 2, "en")
    expect(profile.stats.streak).toBe(3)
    expect(profile.stats.bestStreak).toBe(3)

    // The one counter in the record that goes down, and the one thing that
    // sends it down.
    profile.missed()
    expect(profile.stats.streak).toBe(0)
    expect(profile.stats.bestStreak).toBe(3)

    profile.solved("adobe", 5, "en")
    expect(profile.stats.streak).toBe(1)
    expect(profile.stats.bestStreak).toBe(3)
  })

  it("carries a streak across runs, because a run ending is not a word missed", () => {
    const profile = new Profile()
    profile.solved("crane", 3, "en")
    // A win, a fresh run, and the streak is still standing: it is a count of
    // words found, and nothing here is a word that was not found.
    profile.won(2)
    profile.started()
    profile.solved("slate", 3, "en")
    expect(profile.stats.streak).toBe(2)
  })

  it("averages over the solves and not over the rounds", () => {
    const profile = new Profile()
    profile.solved("crane", 2, "en")
    profile.solved("slate", 4, "en")
    expect(meanSolve(profile.stats)).toBe(3)

    // A round where the word never came has its own row on the chart. Counting
    // it here as a seventh guess would make one number answer two questions
    // badly, and would move the mean on a round that has no guess to average.
    profile.missed()
    expect(meanSolve(profile.stats)).toBe(3)
    expect(wordsFound(profile.stats)).toBe(2)
    expect(roundsPlayed(profile.stats)).toBe(3)
  })

  it("has no average at all before a word has been found", () => {
    const profile = new Profile()
    profile.missed()
    // Null rather than zero, so the screen can say "—" instead of claiming this
    // player solves on the zeroth guess.
    expect(meanSolve(profile.stats)).toBe(null)
  })

  it("keeps a wild guess count inside the row it has", () => {
    const profile = new Profile()
    profile.solved("crane", 0, "en")
    profile.solved("slate", 99, "en")
    // Nothing is solved on guess zero, and no round runs to ninety-nine. Both
    // are clamped rather than trusted, because this array is indexed with them.
    expect(profile.stats.solves.length).toBeLessThanOrEqual(13)
    expect(profile.stats.solves[1]).toBe(1)
    expect(profile.stats.solves[12]).toBe(1)
  })

  it("ranks the relics by how often they were taken", () => {
    const profile = new Profile()
    for (const id of ["snowball", "banker", "snowball", "banker", "snowball"]) profile.took(id)
    expect(favoriteRelics(profile.stats)).toEqual([
      { id: "snowball", count: 3 },
      { id: "banker", count: 2 },
    ])
  })
})

describe("salvaging the longer record", () => {
  it("reads a record written before the statistics existed", () => {
    // Every record in the wild, to the build that adds them: a player mid-career
    // whose history starts today.
    store.items.set(KEY, JSON.stringify({ runs: 40, wins: 3, bestStage: 9, cleared: 2 }))
    expect(loadMeta()).toEqual({ ...FRESH, runs: 40, wins: 3, bestStage: 9, cleared: 2 })
  })

  it("drops the cells it cannot read and keeps the row", () => {
    store.items.set(
      KEY,
      JSON.stringify({
        solves: [0, "two", 3.5, 4],
        crackedBy: { en: ["crane", 7, "crane"], es: "no", de: [] },
        playedBy: { en: ["slate", null], fr: 3 },
        words: "no",
      }),
    )
    expect(loadMeta()).toMatchObject({
      solves: [0, 0, 0, 4],
      // A language whose list read as nothing is dropped rather than kept as an
      // empty array, so an absent language and an unplayed one are one case and
      // `crackedIn` does not have to tell them apart.
      crackedBy: { en: ["crane"] },
      playedBy: { en: ["slate"] },
      words: {},
    })
  })

  it("reads a word table from before the corrections as exact", () => {
    // Every record in the wild when this field arrives. Absent corrections are
    // the same thing as no corrections, so the figures it has been showing all
    // along are the figures it goes on showing: the field costs nobody their
    // favorite word and the key does not move for it.
    store.items.set(KEY, JSON.stringify({ guesses: 100, words: { crane: 88, slate: 12 } }))
    expect(loadMeta()).toMatchObject({ words: { crane: 88, slate: 12 }, wordError: {} })
    expect(favoriteWord(loadMeta())).toEqual({ word: "crane", count: 88 })
  })

  it("keeps a correction from outliving the count it corrects", () => {
    // Neither case can happen through `tally`, and both are one hand-edit away.
    // A correction for a word that is no longer in the table is dropped, and one
    // that would eat its own count is capped at "played at least once", which is
    // the smallest true thing an entry in this table can say.
    store.items.set(
      KEY,
      JSON.stringify({ words: { crane: 9 }, wordError: { crane: 400, slate: 3 } }),
    )
    expect(loadMeta().wordError).toEqual({ crane: 8 })
    expect(favoriteWord(loadMeta())).toEqual({ word: "crane", count: 1 })
  })

  it("reads a record from before the played collection as empty", () => {
    // Unlike the corrections above, there is no honest reconstruction here: the
    // word table holds only the words that survived eviction, so it would name a
    // few dozen of the thousands played. Nothing is the truthful start, and the
    // line hides itself at zero rather than opening a career at "24 words".
    store.items.set(KEY, JSON.stringify({ guesses: 4000, words: { crane: 88 } }))
    expect(loadMeta().playedBy).toEqual({})
    expect(playedIn(loadMeta(), "en")).toBe(0)
  })

  it("files a collection from before the languages under English", () => {
    // A flat `cracked` is a list from a build that had one word list, and that
    // list was English. Discarding it would cost a player every word they ever
    // found to say something about a schema; the record's other two renames
    // answered this the same way, and this key does not bump for any of them.
    store.items.set(KEY, JSON.stringify({ runs: 4, cracked: ["slate", "crane"] }))
    expect(loadMeta()).toMatchObject({ runs: 4, crackedBy: { en: ["crane", "slate"] } })
  })

  it("does not merge the old collection back in once the new one exists", () => {
    // The launch after the migration: this build has written `crackedBy`, and
    // the dead `cracked` beside it is a value it has already read. Merging it
    // again would resurrect words on every launch forever, and a player who
    // cracked ADOBE under the old build and has since played only Spanish would
    // find it back in the English column each time.
    store.items.set(KEY, JSON.stringify({ cracked: ["adobe"], crackedBy: { es: ["gorra"] } }))
    expect(loadMeta().crackedBy).toEqual({ es: ["gorra"] })
  })

  it("will not let a claimed streak promote itself to a record", () => {
    // Read field by field rather than as `max(streak, bestStreak)`. A record
    // hand-edited to say 400 says it on one line and gets nothing for it.
    store.items.set(KEY, JSON.stringify({ streak: 400, bestStreak: 2 }))
    expect(loadMeta()).toMatchObject({ streak: 400, bestStreak: 2 })
  })

  it("trims a word table that arrives too big for its slots", () => {
    // A hand-edited record, or one from a build with a bigger cap. Trimmed to
    // the largest, so the field cannot be talked back into growing forever.
    const bloated = Object.fromEntries(
      Array.from({ length: 90 }, (_, n) => [`w${n.toString().padStart(4, "0")}`, n + 1]),
    )
    store.items.set(KEY, JSON.stringify({ words: bloated, relics: bloated }))
    const meta = loadMeta()
    expect(Object.keys(meta.words).length).toBe(24)
    expect(favoriteWord(meta)).toEqual({ word: "w0089", count: 90 })
    // The relic map is bounded by the catalog, so it is left alone. A relic
    // no longer in the game should still be able to have been a favorite.
    expect(Object.keys(meta.relics).length).toBe(90)
  })

  it("carries the statistics across sessions", () => {
    const first = new Profile()
    first.guessed("crane", "en")
    first.solved("crane", 1, "en")
    first.took("banker")
    expect(new Profile().stats).toEqual(first.stats)
  })
})
