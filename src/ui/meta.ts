import { clampAscension, MAX_ASCENSION } from "../engine"

/**
 * Its own key, so a run save that goes bad cannot take the record with it.
 *
 * v2 because the ascension ladder was renumbered under it. `cleared` is a level
 * and a level is a name. A stored 6 used to mean "won under Finish It" and now
 * means "won under Lean Years", which is four rungs easier. Nothing in the file
 * says which build wrote it, so the only honest reading of the old one is none:
 * the bump resets every record rather than silently promoting players past four
 * rules they have never played. Everything else in here is a counter and would
 * have survived, which is the trade: a v2 key costs the tallies too.
 *
 * It stayed v2 through the next renumbering, and the difference is the whole
 * rule for when this key moves. Replacing `Five` with `Dead Weight` changed what
 * 9 and 10 name, exactly as before, but it changed them *downward*. Old 9 and
 * old 10 were strictly harder than the levels now wearing those numbers, so a
 * record written by that build understates what its player can do and cannot
 * promote anyone past anything. The test is not "did a level change meaning" but
 * "could the old reading credit a player with more than they earned". When the
 * ladder gets easier under a stored `cleared`, the honest move is to keep it and
 * keep their tallies with it.
 */
const META_KEY = "5wild:meta:v2"

/**
 * What outlives a run.
 *
 * A run is a closed world: it starts at stage 1 with four gold and ends when it
 * ends, and `RunState` is complete on its own. This is the other thing: the
 * player, across all of them. It lives in the UI layer for the same reason the
 * save does: `src/engine` must not know a browser exists. An ascension is an
 * *input* to `startRun`, and this is what remembers which one has been earned
 * and which one was last asked for.
 */
export type MetaState = {
  /** Runs started. Every "Play" is one, whether or not it went anywhere. */
  runs: number
  /** Runs that reached the win. */
  wins: number
  /** The deepest stage any run has stood on, including the endless ones. */
  bestStage: number
  /**
   * The highest ascension a run has been won at, or -1 when none has.
   *
   * -1 rather than 0 because ascension 0 is the ordinary game, and "won it once"
   * has to be tellable from "never has", and that difference is the whole unlock.
   */
  cleared: number
  /**
   * The level last chosen on the title screen, which is where the next run
   * starts. Remembered rather than defaulted to the hardest unlocked one: a
   * player who has climbed to 4 and wants an easy evening should not have to
   * step down four times every launch to get one.
   */
  ascension: number
  /** Guesses submitted, ever. The denominator the word tally is read against. */
  guesses: number
  /**
   * Rounds whose answer was found, counted by which guess found it, so
   * `solves[4]` is "cracked it on the fourth". Index 0 is always zero and index
   * 1 is the lucky ones; the array is indexed by guess number rather than by
   * `n - 1` so that a glance at the stored JSON reads straight.
   */
  solves: number[]
  /** Rounds that ended with the answer still hidden, won on score or lost. */
  missed: number
  /**
   * Rounds solved back to back, right now.
   *
   * Counted across runs rather than inside one, which is the whole reason it is
   * interesting: a run ends every few minutes and takes its own numbers with it,
   * so a streak scoped to a run would top out around fifteen and reset on
   * something the player did not choose. This one is only ever broken by a round
   * where the word did not come, which is the thing it is asking about.
   *
   * A round solved and then lost on score keeps the streak alive. The question
   * is whether the word was found, and it was.
   */
  streak: number
  /** The longest that run of solves has ever been. */
  bestStreak: number
  /**
   * Every distinct answer ever cracked, sorted, in the language it was cracked
   * in.
   *
   * One of the two growing fields, and they earn it the same way: each list is
   * bounded by a *content* list rather than by play, so a player who cracked
   * every word in the game has four arrays totalling 8,300 entries and no way to
   * make any of them longer. That is the test a growing field has to pass here,
   * and it is why the frequency tally below is capped instead — its ceiling
   * would be however many distinct words a career happens to contain, which is
   * not a ceiling.
   *
   * These are also the collections this game has. "Which words have I beaten" is
   * a question worth a screen and cannot be answered by a counter; `playedBy` is
   * the same question about the other, larger pool.
   *
   * Keyed by language because the screen reads it as a fraction, and the
   * denominator is one language's pool. Flat, it counted a Spanish answer
   * against the English list it could never have come from, which is a number
   * that can pass 100%. The keys collide as well as the totals: ACTOR is an
   * answer in English and in Spanish, and a set that held it once would credit
   * one crack for two.
   *
   * A `Record` rather than a `Record<Lang, …>`, for the same reason the tables
   * below are: this is parsed from a store a previous build wrote and a future
   * one might, and a language dropped from the game should not take a player's
   * words with it on the way past.
   */
  crackedBy: Record<string, string[]>
  /**
   * Every distinct word ever submitted and accepted, sorted, in the language it
   * was played in.
   *
   * `crackedBy`'s larger sibling, and a different question: that one is the
   * answers found, this one is every legal word spent finding them. It cannot be
   * derived from the other, because the interesting part of it — the thousands
   * of words that were never anybody's answer — is exactly what the other throws
   * away. English allows 15,936 words against 2,300 answers, so this is the
   * bigger collection by seven times and the one that moves on a round that went
   * badly.
   *
   * Bounded the same way and keyed the same way, for the same reasons: the
   * screen reads it as a fraction of one language's `allowed` list, and a word
   * legal in two languages is two different words.
   *
   * Only guesses the engine accepted reach this, which is the definition worth
   * having. `Profile.guessed` is called from `tally` in `app.ts`, which reads a
   * guess off the run *after* the reducer took it, so a refusal — not a word, a
   * repeat under ascension 9, a boss's rule — never lands here. "Words played"
   * means words the game agreed were words.
   */
  playedBy: Record<string, string[]>
  /**
   * How often the most-played words have been played. Capped; see `tally`.
   */
  words: Record<string, number>
  /**
   * How much of a `words` entry was inherited rather than played, for the
   * entries that inherited anything.
   *
   * The other half of the algorithm in `tally`, and it is what makes that table
   * readable rather than merely correct. A word arriving at a full table takes
   * the count of the entry it evicted, so its stored figure is an upper bound
   * and the true one is `words[w] - wordError[w]`. Without this field the two
   * are indistinguishable, and a word typed once reads as however deep the
   * table had got: over 50 simulated profiles at 1,000 guesses with no habitual
   * opener, the reported favorite averaged 42 against a true count of 2, and all
   * 24 entries were overcounted. That is most players of this game, because
   * ascension 9 forbids repeating a word inside a run, so nearly every guess is
   * a newcomer.
   *
   * Absent for an entry that never displaced anything, which includes every
   * entry of every record written before this field existed. Absent means zero
   * means the count is exact, so an old record reads today exactly as it read
   * yesterday and no key had to move.
   */
  wordError: Record<string, number>
  /** How often each relic has been taken. Bounded by the catalog. */
  relics: Record<string, number>
}

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

/**
 * How many words the tally keeps counts for.
 *
 * Small on purpose. The alternative, a count for every word ever typed, tops
 * out near sixteen thousand entries, grows with play rather than with content,
 * and is rewritten on every guess. This is the one field on the hot path, so it
 * is the one field that is not allowed to grow.
 */
const WORD_SLOTS = 24

/** The most guesses a round could ever take, as a ceiling on `solves`. */
const GUESS_CAP = 12

/**
 * The hardest level actually *earned*: one past the hardest ever won.
 *
 * Zero for a player who has never won. It is no longer a gate, since every rung
 * of the ladder can be dialed in from the first launch, but it is still the line
 * between the climb as it was designed and a leap past it, which is what the
 * title screen warns about.
 */
export const unlocked = (meta: MetaState): number => Math.min(MAX_ASCENSION, meta.cleared + 1)

/** Whether a level is above what the record has earned, and so wears a lock. */
export const isLocked = (meta: MetaState, level: number): boolean => level > unlocked(meta)

/**
 * The level a new run would start at.
 *
 * Clamped to the ladder but not to what has been won: a player who wants to open
 * with ascension 6 is allowed to, and gets told on the way that the rungs below
 * it are there for a reason. Clamping happens on the way out rather than on the
 * way in, so a record carrying a level this build no longer offers, from a
 * shorter ladder, reads as the nearest legal one instead of refusing to start a run.
 */
export const chosenAscension = (meta: MetaState): number => clampAscension(meta.ascension)

/**
 * How many distinct answers have been cracked in one language.
 *
 * One language and never the sum, because the only number to read it against is
 * that language's answer pool, and the only pool whose size the app knows is the
 * one it currently has loaded.
 */
export const crackedIn = (meta: MetaState, lang: string): number =>
  (meta.crackedBy[lang] ?? []).length

/** How many distinct words have been played in one language. Same rule, larger pool. */
export const playedIn = (meta: MetaState, lang: string): number =>
  (meta.playedBy[lang] ?? []).length

/** Rounds whose word was found, on whichever guess found it. */
export const wordsFound = (meta: MetaState): number =>
  meta.solves.reduce((sum, count) => sum + count, 0)

/** Rounds that reached an ending, which is what the breakdown is a breakdown of. */
export const roundsPlayed = (meta: MetaState): number => meta.missed + wordsFound(meta)

/**
 * Which guess a solve lands on on average, or null before one has.
 *
 * The number the distribution has always implied and never said. A player reads
 * five bars and comes away with an impression; this is the impression as a
 * figure, and it is the one that moves when they get better at the game. The
 * bars can look much the same while the weight shifts a column to the left.
 *
 * Averaged over solves and not over rounds, so a round where the word never came
 * is not a sixth guess dragging the mean up. Never found is its own row, and
 * folding it in here would make one number answer two questions badly.
 */
export function meanSolve(meta: MetaState): number | null {
  const found = wordsFound(meta)
  if (found === 0) return null
  return meta.solves.reduce((sum, count, at) => sum + at * count, 0) / found
}

/**
 * The word played most, and how often, or null before anything has been.
 *
 * Both the ranking and the figure are `count - error`, the part of the entry
 * this player is known to have played, rather than the stored count. See
 * `tally`: the stored count is an upper bound, and ranking on it hands the
 * screen to whichever one-off word most recently inherited a big number. The
 * corrected figure is a lower bound, so the sentence it produces is one the
 * record can defend: a word it says was played nine times was played at least
 * nine times, and a word played habitually since before the table filled reads
 * exactly.
 *
 * Note which number eviction uses, because they must differ: `tally` ranks on
 * the raw count and this ranks on the corrected one. The first is about what a
 * word might yet turn out to be, the second about what it has been shown to be,
 * and a table that confused the two would either forget favorites or invent
 * them.
 */
export function favoriteWord(meta: MetaState): { word: string; count: number } | null {
  let best: { word: string; count: number } | null = null
  for (const [word, stored] of Object.entries(meta.words)) {
    const count = stored - (meta.wordError[word] ?? 0)
    // Ties break alphabetically rather than by insertion order, so the answer
    // does not depend on the order `Object.entries` happens to hand things back.
    if (!best || count > best.count || (count === best.count && word < best.word)) {
      best = { word, count }
    }
  }
  return best
}

/** Relics by how often they have been taken, most-taken first. */
export const favoriteRelics = (meta: MetaState): { id: string; count: number }[] =>
  Object.entries(meta.relics)
    .map(([id, count]) => ({ id, count }))
    .sort((a, b) => b.count - a.count || a.id.localeCompare(b.id))

/**
 * Bump a word's count, in a table that is not allowed to outgrow `WORD_SLOTS`.
 *
 * This is Space-Saving: a newcomer arriving at a full table evicts the weakest
 * entry and *inherits its count* rather than starting from one. Which sounds
 * like cheating, and is exactly what makes the table trustworthy: a word can
 * only be pushed out by something at least as common, so anything played more
 * than one part in `WORD_SLOTS` of the time is guaranteed to still be here. The
 * question the screen asks is "what do I play most", and this answers it
 * correctly, where a naive top-24 would not: that one would freeze on the first
 * 24 words ever typed, since a newcomer's count of 1 never beats an incumbent's.
 *
 * What the inheritance buys in retention it costs in the figure: the stored
 * count is an upper bound, and for a newcomer it is almost entirely borrowed. So
 * the amount borrowed is written down beside it, in `wordError`, and the true
 * count lives in `[count - error, count]`. The bracket is tight where it
 * matters — an entry that has been in the table since before it filled has no
 * error at all and reads exactly — which is why the screen can show
 * `count - error` and simply be right about a habit while refusing to invent a
 * number for a word played once.
 *
 * Two rules keep the guarantee intact. Eviction still ranks on the raw count,
 * never on the corrected one, because the raw count is what bounds how often the
 * word *could* have been played and dropping the wrong entry is what loses a
 * genuine favorite. And the error is fixed at insertion and never touched again,
 * so every guess after it narrows the bracket by one rather than widening it.
 */
function tally(meta: MetaState, word: string): Pick<MetaState, "words" | "wordError"> {
  const words = { ...meta.words }
  const seen = words[word]
  if (seen !== undefined) {
    words[word] = seen + 1
    return { words, wordError: meta.wordError }
  }
  const keys = Object.keys(words)
  if (keys.length < WORD_SLOTS) {
    words[word] = 1
    return { words, wordError: meta.wordError }
  }
  let weakest = keys[0] as string
  for (const key of keys) if ((words[key] ?? 0) < (words[weakest] ?? 0)) weakest = key
  const floor = words[weakest] ?? 0
  delete words[weakest]
  words[word] = floor + 1
  // The evicted word's own error goes with it, and the newcomer's is the whole
  // of what it just took. A word that was evicted once and comes back is a
  // newcomer again, which is the honest reading: whatever it had played before
  // is gone, and nothing here can tell it from a word never seen.
  const wordError = { ...meta.wordError, [word]: floor }
  delete wordError[weakest]
  return { words, wordError }
}

/**
 * The record, kept in memory and written through on every change.
 *
 * Reads happen on the title screen every render; writes happen a handful of
 * times a run. Holding the state here rather than re-reading the store keeps the
 * cheap thing cheap, and means a store that starts refusing writes mid-session
 * still shows the player a truthful number until they close the tab.
 */
export class Profile {
  private state: MetaState = loadMeta()

  get stats(): MetaState {
    return this.state
  }

  /** Called when a run actually begins, not when a `RunState` is constructed. */
  started(): void {
    this.write({ runs: this.state.runs + 1 })
  }

  /** The high-water mark. Idempotent, so it can be called on every save. */
  reached(stage: number): void {
    if (stage > this.state.bestStage) this.write({ bestStage: stage })
  }

  /** The level picked for the next run. Written straight through: it is a choice. */
  chose(ascension: number): void {
    if (ascension !== this.state.ascension) this.write({ ascension })
  }

  /**
   * One submitted guess, in the language it was played in. The hot path: every
   * keystroke run ends here.
   *
   * The language is the same one `solved` takes and is taken for the same
   * reason: the word came off one list and belongs to it, and the interface may
   * have been switched to another since the run was dealt.
   *
   * Three things at once, and the collection is the only one that can decline to
   * write. `guesses` and the frequency tally move on every guess; `playedBy`
   * moves the first time a word is seen and never again, which is most of what
   * keeps the biggest field in the record off the write path once a career is
   * under way.
   */
  guessed(word: string, lang: string): void {
    const played = insert(this.state.playedBy[lang] ?? [], word)
    this.write({
      guesses: this.state.guesses + 1,
      ...tally(this.state, word),
      ...(played ? { playedBy: { ...this.state.playedBy, [lang]: played } } : {}),
    })
  }

  /**
   * A round's answer found, on the `guesses`th try.
   *
   * The word goes in the collection whether or not the round was then won on
   * score, since cracking it is the thing being remembered, and a player who found
   * the word and still fell short of the target found the word.
   */
  solved(answer: string, guesses: number, lang: string): void {
    const at = Math.min(Math.max(1, Math.round(guesses)), GUESS_CAP)
    const solves = [...this.state.solves]
    while (solves.length <= at) solves.push(0)
    solves[at] = (solves[at] ?? 0) + 1
    // The language the word was *dealt* in, not the one the interface is wearing:
    // those disagree for as long as a run outlives a settings tap, and the answer
    // came from one list and belongs to it.
    const found = insert(this.state.crackedBy[lang] ?? [], answer)
    const streak = this.state.streak + 1
    this.write({
      solves,
      streak,
      bestStreak: Math.max(this.state.bestStreak, streak),
      ...(found ? { crackedBy: { ...this.state.crackedBy, [lang]: found } } : {}),
    })
  }

  /** A round that ended with the answer never found. The one thing that breaks a streak. */
  missed(): void {
    this.write({ missed: this.state.missed + 1, streak: 0 })
  }

  /** A relic joining the tray, however it got there. */
  took(id: string): void {
    this.write({ relics: { ...this.state.relics, [id]: (this.state.relics[id] ?? 0) + 1 } })
  }

  /** Banked at the offer, not at the ending: playing on cannot lose the win. */
  won(ascension: number): void {
    this.write({
      wins: this.state.wins + 1,
      cleared: Math.max(this.state.cleared, ascension),
    })
  }

  private write(change: Partial<MetaState>): void {
    this.state = { ...this.state, ...change }
    try {
      localStorage.setItem(META_KEY, JSON.stringify(this.state))
    } catch {
      // A full or blocked store costs the player their record, not their run.
    }
  }
}

/**
 * Read the record, field by field rather than all or nothing.
 *
 * `loadSave` throws a malformed run away whole, because half a run is not a run
 * anybody can play. This is the opposite kind of object: a handful of
 * independent counters, where one field arriving as garbage is no reason to
 * forget the rest. So each is taken if it is sane and defaulted if it is not,
 * and a build that adds a field later, as the ascension one was, reads older
 * records as zero on it rather than as absent.
 */
export function loadMeta(): MetaState {
  try {
    const raw = localStorage.getItem(META_KEY)
    if (!raw) return { ...FRESH }
    const parsed: unknown = JSON.parse(raw)
    if (typeof parsed !== "object" || parsed === null) return { ...FRESH }
    const meta = parsed as Partial<Record<keyof MetaState, unknown>>
    // Two fields changed spelling when antes became stages and jokers relics.
    // Nothing changed *meaning*, so this is the one case the key does not move
    // for: a bump would be honest about the schema and would throw away every
    // counter in the file to say it, including the `cleared` that gates the
    // ascension ladder. Reading the old name where the new one is absent costs
    // two lines and keeps the record whole; the next write puts it back under
    // the new spelling, and the dead key is ignored from then on.
    // `cracked` joined them when the words gained languages. A flat list is a
    // list from before there was more than one, which was necessarily English —
    // the same reading `5wild:run:lang` gives its own absence — so it is not
    // discarded, it is filed under `en`.
    const legacy = parsed as Partial<Record<"bestAnte" | "jokers" | "cracked", unknown>>
    // Read before the object it belongs to is built, because the error terms are
    // only meaningful against the counts that survived the trim: an entry cut
    // for being small takes its correction with it.
    const words = table(meta.words, WORD_SLOTS)
    return {
      runs: count(meta.runs),
      wins: count(meta.wins),
      bestStage: count(meta.bestStage ?? legacy.bestAnte),
      cleared: count(meta.cleared, -1),
      ascension: count(meta.ascension),
      guesses: count(meta.guesses),
      solves: counts(meta.solves).slice(0, GUESS_CAP + 1),
      missed: count(meta.missed),
      streak: count(meta.streak),
      // Read independently rather than as `max(stored, streak)`: a hand-edited
      // record claiming a streak of 400 should say so on one line, not quietly
      // promote itself to a best as well.
      bestStreak: count(meta.bestStreak),
      crackedBy: collections(meta.crackedBy, collection(legacy.cracked)),
      // No legacy arm and none possible: the field is new, and a record without
      // it is a record whose owner played every one of those words before
      // anybody was counting. Absent is an empty collection rather than a
      // reconstruction, since the only list that could seed it is `crackedBy`,
      // and "the answers you found" is a different question wearing this one's
      // clothes — it would credit a player with 273 words played out of 15,936
      // and be wrong by every legal word they ever spent getting there.
      playedBy: collections(meta.playedBy),
      words,
      wordError: errors(meta.wordError, words),
      relics: table(meta.relics ?? legacy.jokers),
    }
  } catch {
    return { ...FRESH }
  }
}

/** A whole number that is not negative, or the default. Nothing here counts down. */
const count = (value: unknown, fallback = 0): number =>
  typeof value === "number" && Number.isInteger(value) && value >= 0 ? value : fallback

/** A row of counters. A bad cell reads as zero rather than voiding the row. */
const counts = (value: unknown): number[] =>
  Array.isArray(value) ? value.map((cell) => count(cell)) : []

/**
 * A map of counters, trimmed to `slots` by taking the largest.
 *
 * The trim is what stops a record hand-edited or written by a build with a
 * bigger cap from reintroducing the unbounded map this was built to avoid.
 * Nothing is trimmed when `slots` is left off, which is right for the relic
 * map: it is bounded by the catalog, and a relic retired from the game should
 * still be able to say it was somebody's favorite.
 */
function table(value: unknown, slots?: number): Record<string, number> {
  if (typeof value !== "object" || value === null || Array.isArray(value)) return {}
  const kept = Object.entries(value as Record<string, unknown>)
    .filter(([, cell]) => count(cell) > 0)
    .sort((a, b) => count(b[1]) - count(a[1]))
    .slice(0, slots ?? Infinity)
  return Object.fromEntries(kept.map(([key, cell]) => [key, count(cell)]))
}

/**
 * A sorted collection with one more word in it, or null if it was already there.
 *
 * Null rather than the unchanged array, so the caller writes nothing at all for
 * a word it has seen before. That is the common case by a wide margin once a
 * career is under way, and it is what keeps the two largest fields in the record
 * off the write path: a guess that adds nothing costs a binary search.
 *
 * Placed rather than appended-and-sorted, which is what `crackedBy` did while it
 * was the only collection and the only one small enough not to care. Inserting
 * into the full English `allowed` list: 0.478ms to copy and re-sort, 0.041ms to
 * copy and splice, for the same four lines. Both are affordable; the sort is
 * eleven times the price of the search and gets worse exactly as a player
 * plays more, which is the wrong direction for the one field that grows.
 */
function insert(list: readonly string[], word: string): string[] | null {
  let lo = 0
  let hi = list.length
  while (lo < hi) {
    const mid = (lo + hi) >> 1
    // Non-null: `mid` is strictly below `hi`, which starts at the length and
    // only ever shrinks. Asserted rather than defaulted because a default would
    // silently place the word rather than fail on a list that cannot exist.
    const held = list[mid] as string
    if (held === word) return null
    if (held < word) lo = mid + 1
    else hi = mid
  }
  const next = [...list]
  next.splice(lo, 0, word)
  return next
}

/**
 * The word table's error terms, read against the counts they correct.
 *
 * Kept honest here rather than at the point of display, so nothing downstream
 * has to defend itself: an error is dropped unless its word is still in the
 * table, and it is capped at one below that word's count. Both cases are
 * unreachable from `tally` and both are one hand-edit away, and the failure they
 * would cause is a screen claiming a word was played zero or minus four times,
 * which reads as a bug in the counting rather than as a record somebody edited.
 *
 * The cap rather than a rejection, because an error is a *correction* and the
 * conservative reading of a broken one is the smallest true thing the entry can
 * still say: played at least once, which is why it is in the table at all.
 */
function errors(value: unknown, words: Record<string, number>): Record<string, number> {
  const raw = table(value)
  const kept = Object.entries(raw).flatMap(([word, error]) => {
    const stored = words[word]
    return stored === undefined ? [] : [[word, Math.min(error, stored - 1)] as const]
  })
  return Object.fromEntries(kept.filter(([, error]) => error > 0))
}

/**
 * The collections, one per language, with an optional fallback for the flat one
 * that came before them.
 *
 * Optional because `playedBy` reads through here too and has nothing behind it
 * to salvage: it is the same shape and wants the same sanity, and only
 * `crackedBy` has a previous spelling to answer for.
 *
 * The fallback lands under `en` only when there is no `crackedBy` at all, not
 * per-language: once this build has written the field once, an old `cracked`
 * still sitting in the blob is a value this build already read and migrated, and
 * merging it again on every launch would resurrect words a player has no way to
 * remove and keep resurrecting them forever.
 */
function collections(value: unknown, fallback: string[] = []): Record<string, string[]> {
  if (typeof value !== "object" || value === null || Array.isArray(value)) {
    return fallback.length > 0 ? { en: fallback } : {}
  }
  const out: Record<string, string[]> = {}
  for (const [lang, words] of Object.entries(value as Record<string, unknown>)) {
    const found = collection(words)
    if (found.length > 0) out[lang] = found
  }
  return out
}

/**
 * The distinct words of one collection. Deduped and sorted on the way in,
 * because the screen counts them and `insert` assumes what it reads back is a
 * sorted set — a list out of order would not merely place a word oddly, it would
 * fail to find one already there and grow a duplicate on every guess.
 */
const collection = (value: unknown): string[] =>
  Array.isArray(value)
    ? [...new Set(value.filter((word): word is string => typeof word === "string"))].sort()
    : []
