/*
 * Regenerates public/words/<lang>/{answers,allowed}.txt.
 *
 * Run with `node tools/gen-wordlists.ts` (Node strips the types natively), or
 * with a list of languages to do fewer: `node tools/gen-wordlists.ts de`.
 * Outputs are committed, so this runs rarely, but every source is pinned to a
 * commit SHA so re-running it produces byte-identical files rather than
 * silently drifting with upstream.
 *
 * We deliberately do NOT ship the New York Times' curated answer list. This
 * derives its own from open corpora:
 *
 *   allowed  every 5-letter word in a public-domain dictionary
 *   answers  the most frequent of those, so the secret word is always fair
 *
 * There are two pipelines under that, not one, and the split is not a tidiness
 * failure — it is what the corpora offer. English has `dwyl/english-words`, a
 * flat scrape of 370k inflected forms, and needs a hunspell dictionary only as
 * a *signal* (proper nouns are capitalized in it, so a lowercase hit means the
 * word is a common noun). No other language has a dwyl. What they have is the
 * hunspell dictionary itself, which is stems plus affix flags rather than a
 * word list, so for those three the dictionary has to be *expanded* into one
 * before anything else can happen. See `expanded()`.
 *
 * The English path is unchanged and must stay that way: its outputs are what
 * the golden vectors were recorded against, so a regeneration that moves them
 * is a bug in this file rather than a new baseline.
 *
 * Sources:
 *   dwyl/english-words         Unlicense                (en allowed)
 *   hermitdave/FrequencyWords  MIT                      (OpenSubtitles 2018 frequencies, all four)
 *   LDNOOBW/*bad-words*        CC-BY-4.0                (all four)
 *   wooorm/dictionaries        en MIT/BSD, es GPL-3.0, fr MPL-2.0, de GPL-2.0-or-3.0
 *
 * Every one of those is compatible with this game's own GPL-3.0, which is worth
 * saying out loud because two of the three new dictionaries are copyleft and
 * the lists derived from them ship in the repository.
 */

import { mkdir, writeFile } from "node:fs/promises"
import { dirname, join } from "node:path"
import { fileURLToPath } from "node:url"

const LANGS = ["en", "es", "fr", "de"] as const
type Lang = (typeof LANGS)[number]

const WORD_LENGTH = 5

const DICTIONARIES =
  "https://raw.githubusercontent.com/wooorm/dictionaries/8cfea406b505e4d7df52d5a19bce525df98c54ab/dictionaries"
const FREQUENCIES =
  "https://raw.githubusercontent.com/hermitdave/FrequencyWords/525f9b560de45753a5ea01069454e72e9aa541c6/content/2018"
const PROFANITIES =
  "https://raw.githubusercontent.com/LDNOOBW/List-of-Dirty-Naughty-Obscene-and-Otherwise-Bad-Words/5faf2ba42d7b1c0977169ec3611df25a3c08eb13"
const WORDS_ALPHA =
  "https://raw.githubusercontent.com/dwyl/english-words/20f5cc9b3f0ccc8ce45d814c532b7c2031bba31c/words_alpha.txt"

const dictionary = (lang: Lang, ext: "dic" | "aff") => `${DICTIONARIES}/${lang}/index.${ext}`
const frequency = (lang: Lang) => `${FREQUENCIES}/${lang}/${lang}_50k.txt`
const profanity = (lang: Lang) => `${PROFANITIES}/${lang}`

const OUT_DIR = join(dirname(fileURLToPath(import.meta.url)), "..", "public", "words")

async function fetchText(url: string): Promise<string> {
  const res = await fetch(url)
  if (!res.ok) throw new Error(`${res.status} ${res.statusText} fetching ${url}`)
  return res.text()
}

function lines(text: string): string[] {
  return text
    .split("\n")
    .map((l) => l.trim().toLowerCase())
    .filter((l) => l.length > 0)
}

/**
 * Every word in this game is 26 letters' worth of word, in all four languages.
 *
 * The board draws one glyph per tile and every letter-indexed system in the
 * engine — etchings, ranges, modifiers, MIN_LIVE_LETTERS, the keyboard — is an
 * index into A-Z, so an É on a tile is not a rendering question, it is a
 * twenty-seventh letter and a second engine. Folding is what buys all of that
 * for free, and it is the same fold the UI applies to what the player types, so
 * CAFÉ and CAFE are the same word on both sides of the guess.
 *
 * Two steps, and the order matters. First the ligatures and the eszett, which
 * are not accents and survive NFD untouched: each language's own orthography
 * says what to write when the character is unavailable, and that is the rule
 * used rather than a guess. ß→SS is the official substitution (GROSS, WEISS),
 * œ→OE and æ→AE are how French has always spelled them on a typewriter. Then
 * NFD and drop the combining marks, which is what turns é into e and ñ into n.
 *
 * Dropping ß words instead was the first plan, on the grounds that ß→ss changes
 * the length and so admits a four-letter word as a five-letter answer. It does,
 * and the objection is backwards: WEISS *is* the five-letter spelling of that
 * word, and a German player types those five keys. The cost of the other choice
 * is measured: 1,131 of the 75,888 German entries carry an ß, and expanding
 * them adds 13 guesses and 20 answers, among them GROSS, WEISS and HEISS. The
 * French ligature is worth less again — 4 answers — but they are COEUR and
 * SOEUR, which is not a rounding error in a French word game.
 */
const LIGATURES: [RegExp, string][] = [
  [/ß/g, "ss"],
  [/œ/g, "oe"],
  [/æ/g, "ae"],
]

function fold(word: string): string {
  let out = word.toLowerCase()
  for (const [from, to] of LIGATURES) out = out.replace(from, to)
  return out.normalize("NFD").replace(/[̀-ͯ]/g, "")
}

const isTarget = (w: string) => w.length === WORD_LENGTH && /^[a-z]+$/.test(w)

/** What a language's pipeline hands back, whichever pipeline it was. */
type Lists = { allowed: string[]; answers: string[]; notes: string[] }

// ---------------------------------------------------------------------------
// What the block list misses
// ---------------------------------------------------------------------------

/*
 * Both pipelines already filter answers through LDNOOBW, and both are right to.
 * The two lists below exist because reading the output showed that filter is
 * thinner than it looks, in two different ways, and only one of them is a
 * question of taste.
 *
 * The taste half is HAND_PROFANE. LDNOOBW/en is 403 entries and carries WHORE,
 * PUSSY and BITCH but not PRICK, HUSSY, BIMBO or RANDY, so those were answers —
 * words the game prints on the board as the thing the player was looking for.
 * The per-language lists are thinner again (de is 66 entries, es 68) and miss
 * their own language's obvious ones: FICKT and HODEN were German answers, POLLA
 * and JODER Spanish ones, PISSE both. Taking the union of all four lists was the
 * first idea and it is wrong: it blocks NEGRO, which is how Spanish says
 * *black*, BITTE, which is how German says *please*, and KRAUT, which is how
 * German says *herb*. A word is only obscene in the language it is being read
 * in, so this is keyed by language and each entry is judged there. KRAUT is
 * blocked in English and kept in German for exactly that reason.
 *
 * The other half is SLURS, and it is not taste. `allowed` is deliberately
 * permissive — see the note on the English guess list about how much worse it is
 * to reject a word a player typed than to accept a rude one — but that argument
 * is about vulgarity, and it does not reach a word whose only meaning is an
 * ethnic slur. Nobody probes the board with KIKES. So these come out of the
 * guess list too, which is the one place this file overrides that rule, and the
 * bar for entry is correspondingly narrow: no ordinary reading in any of the
 * four languages. That bar is doing real work rather than decorating the
 * comment. CHINK is a slur and stays guessable, because a chink in the armour is
 * a chink in the armour. DYKES, FAGOT, GIMPS and MONGO stay for the same reason
 * — an embankment, a bundle of sticks, a braid trim, a currency. NEGRO and NIGER
 * stay, being Spanish and a country. And German SPICK stays where German SPAST
 * goes, which is the pair worth keeping in mind, because the two look alike and
 * the dictionary tells them apart: it carries `spicken/DIXYW`, so SPICK is a
 * generated imperative and a real word, and it carries SPASTIK, SPASTIKER and
 * SPASTISCH but never SPAST, which reaches the list only from the subtitles at
 * rank #38,170. NIGGA is the same shape at #12,359. A German form that the
 * German dictionary cannot derive is not a German word.
 *
 * What this costs: 13 of English's 15,949 guesses and 2 of German's 6,765, with
 * Spanish and French already clean. Nothing is lost from the answer lists, which
 * are a fixed count taken off the top of a frequency ranking — a blocked answer
 * is backfilled by the next word down, so the only effect is that the lists are
 * that many ranks deeper.
 */
const HAND_PROFANE: Record<Lang, string[]> = {
  en: ["bimbo", "booby", "chink", "hussy", "kraut", "prick", "queer", "randy", "sperm", "willy"],
  es: ["folla", "joder", "orgia", "polla", "porno", "zorra"],
  fr: ["bimbo", "orgie", "penis", "pisse", "porno"],
  // KRAUT and BITTE are not on this list and must not be added to it.
  de: ["dildo", "ficke", "fickt", "hoden", "orgie", "pisse"],
}

const SLURS: Record<Lang, string[]> = {
  en: [
    "coons",
    "dagos",
    "darky",
    "faggy",
    "gippo",
    "gooks",
    "gyppo",
    "honky",
    "kikes",
    "nigre",
    "spick",
    "spics",
    "squaw",
  ],
  es: [],
  fr: [],
  de: ["nigga", "spast"],
}

// ---------------------------------------------------------------------------
// English
// ---------------------------------------------------------------------------

/**
 * Guesses younger than the corpus that carries the guesses.
 *
 * words_alpha is a scrape of an older dictionary, and it shows: 198 five-letter
 * words in the ENABLE lexicon are missing from it, and ENABLE itself closed in
 * 2001, so anything the language picked up after that is missing from both.
 * EMOJI, LATTE, REHAB, DETOX and PESTO were not legal guesses at all, which is
 * the one failure the permissive-guesses rule below exists to prevent.
 *
 * Scoped to everyday modern vocabulary a player might actually type. The 198
 * are mostly Scrabble esoterica (KHAPH, KHETH, LWEIS) and none of that is here.
 *
 * Singulars only, which is a rule about this list and not about guesses. It
 * held nine plurals of four-letter words (BLOGS, VAPES, MEMES, CROCS) and they
 * are gone. `allowed` is full of plurals and should be, but those arrive from
 * the dictionary with their singulars beside them, so isPlural can see the
 * shape and keep it out of answers. A hand-added plural has no singular in the
 * lemma list by definition, since a word missing from the dictionary is why it
 * is here, so isPlural is blind to it and it is eligible to be the secret word.
 * CARBS proved that rather than threatened it: it ranked in on the first
 * regeneration and needed a HAND_BLOCKED entry to get back out. Better to spend
 * the entry on a fifth letter that is doing something.
 */
const HAND_ALLOWED = [
  "bling",
  "chemo",
  "decaf",
  "detox",
  "dorky",
  "dweeb",
  "emoji",
  "expat",
  "futon",
  "geeky",
  "glitz",
  "gulag",
  "inbox",
  "indie",
  "latte",
  "nuked",
  "penne",
  "pesto",
  "ramen",
  "rehab",
  "satay",
  "sicko",
  "synth",
  "thunk",
  "tranq",
  "wacko",
  "wimpy",
  "wussy",
]

/**
 * What the corpora between them still get wrong, one word at a time.
 *
 * This was two entries and a note saying a hand list is a maintenance burden
 * every entry has to earn itself. It is fifty-five, and the note still holds;
 * what changed is that somebody read all 2300 answers instead of spot-checking
 * them. VINOD is a given name that beat the lemma check by ending in D, so VINO
 * answered for it. It surfaced when the plural filter pulled words in from
 * deeper down the frequency ranks, which is the general risk of digging: the
 * further down you go, the worse the corpus gets. The rest arrived in three
 * groups.
 *
 * SQUAW used to head this list, described as a slur the block list does not
 * carry, and that description is now a whole mechanism: see SLURS above, which
 * it moved to. This list is for taste and for correctness. Obscenity has its own
 * two lists because it also has to reach the other three languages, and because
 * a slur has to come out of the guess list as well, which nothing here does.
 *
 * The first is the one that is simply a defect. `stems()` strips -ER and -LY
 * before the lemma lookup, so a name passes whenever its own first three or
 * four letters are a word: ASHER on ash, BOYER on boy, DOVER on dove, EWING on
 * ewe, MILLY on mill, POLLY on pol, TOLLY on toll. 151 of the 2300 got in by a
 * suffix strip and nothing else, and 13 of those are in no dictionary at all.
 * Tightening the strip is not the repair, because it is the same rule that
 * admits COMER, FEWER, NOBLY and about a hundred other words that are real.
 * So the leak is patched by name, here, where the names are visible.
 *
 * The second and third are taste, and worth saying so plainly rather than
 * dressing up as correctness. SENOR, CREME and VOILA are all in Merriam-Webster
 * and all Scrabble-legal; what disqualifies them is that their English spelling
 * carries a diacritic the board cannot draw, so the answer is a word spelled
 * wrong. The rest of that group is borrowings English only reaches for when it
 * is quoting another language. And GONNA sits at corpus rank #8, which is not a
 * fact about English but a fact about movie subtitles being transcribed speech.
 */
const HAND_BLOCKED = [
  "vinod",
  // Names, trademarks and non-words the suffix strip let through.
  "asher",
  "boyer",
  "ching",
  "dover",
  "elmer",
  "ewing",
  "goths",
  "jared",
  "jello",
  "lovey",
  "mayer",
  "milly",
  "multi",
  "polly",
  "rager",
  "tilly",
  "tolly",
  "versa",
  // Borrowings, and the three whose real spelling needs an accent.
  "adios",
  "aloha",
  "amigo",
  "amour",
  "begum",
  "bijou",
  "creme",
  "fakir",
  "ganja",
  "hajji",
  "kanji",
  "laird",
  "largo",
  "padre",
  "pasha",
  "sadhu",
  "sahib",
  "senor",
  "swami",
  "tutti",
  "voila",
  // Eye-dialect, and inflections that left English with the King James Bible.
  "aught",
  "begat",
  "canst",
  "cuppa",
  "didst",
  "dunno",
  "durst",
  "gimme",
  "gonna",
  "gotta",
  "shalt",
  "thine",
  "wanna",
  // What blocking the above let in, since fifty-two seats freed is fifty-two
  // words pulled up from where the corpus is thinner. BOLLY fails the same test
  // as the names: in words_alpha, in no dictionary.
  "bolly",
]

/**
 * Answers the ranking cannot reach, for the two reasons it cannot reach them.
 *
 * The first group is the false rejections the lemma note below already admits
 * to: the small SCOWL list does not carry these lemmas, so the words never
 * reach the ranking to be ranked. The note calls it about twenty words and
 * accepts the loss. That was the right call when the loss was invisible; it
 * reads differently once you notice a Wordle-alike whose secret word is never
 * ALIVE and never TEETH. Every entry here ranked above #5223, which is where
 * the 2300th answer sits, so each would have made the list on merit.
 *
 * The second group is younger than both corpora. EMOJI is in neither
 * words_alpha nor ENABLE, and OpenSubtitles-2018 ranks it #39350 on 249 hits,
 * so no amount of loosening a filter reaches it: a word the corpus has barely
 * heard of cannot be ranked into a list the corpus orders. Hand-adding is the
 * only mechanism there is, and these are the words worth spending it on.
 */
const HAND_ADDED = [
  // Real words the lemma check ate. ALIVE is #113, TEETH #355, FORTH #698.
  "ahold",
  "alive",
  "caddy",
  "cyber",
  "depot",
  "donut",
  "forth",
  "inlet",
  "input",
  "pinky",
  "repay",
  "rerun",
  "reset",
  "resin",
  "snuck",
  "teeth",
  "untie",
  "unzip",
  "woken",
  // Too new for a 2017 scrape of an older dictionary, let alone a 2001 lexicon.
  "bling",
  "chemo",
  "decaf",
  "detox",
  "emoji",
  "futon",
  "geeky",
  "indie",
  "inbox",
  "latte",
  "pesto",
  "ramen",
  "rehab",
  "synth",
]

const EN_ANSWER_COUNT = 2300

async function english(): Promise<Lists> {
  const [alphaRaw, freqRaw, profanityRaw, hunspellRaw] = await Promise.all([
    fetchText(WORDS_ALPHA),
    fetchText(frequency("en")),
    fetchText(profanity("en")),
    fetchText(dictionary("en", "dic")),
  ])

  // Every legal guess. Kept permissive on purpose: rejecting a word a player
  // typed as an information probe is far more annoying than allowing a rude one.
  // SLURS is the one exception to that, and the note on it says why.
  const slurs = new Set(SLURS.en)
  const allowed = [...new Set([...lines(alphaRaw).filter(isTarget), ...HAND_ALLOWED])]
    .filter((word) => !slurs.has(word))
    .sort()

  // The frequency corpus is movie subtitles, so it is thick with proper nouns
  // (ALICE, DIEGO), foreign dialogue (NICHT, MERCI, DANKE) and apostrophe-stripped
  // contractions (DOESN, WEREN). words_alpha is lowercased and admits all of them.
  // So a candidate must additionally be a lowercase entry in an en_US hunspell
  // dictionary, which stores proper nouns capitalized and no other language at all.
  //
  // That dictionary lists only lemmas, so we try a few obvious suffix strips
  // before giving up, or ZONES and TRIED would fail on a technicality.
  // It is SCOWL's *small* list, so the rule is imperfect in one direction: real
  // words get rejected too, along with British spellings, which is arguably
  // correct for a US-spelling answer set. Erring this way is still right, because
  // letting ALICE through is a round the player cannot reason their way to.
  //
  // This note used to add that we accept the loss, since a word rejected here is
  // still a legal guess and only never the secret one. That held while the loss
  // was theoretical. It named ALIVE, TEETH, DEPOT and INPUT as the examples, and
  // a word game whose answer is never TEETH is not paying a technicality, it is
  // missing a word. The count is not twenty either: 228 of the rejections are in
  // ENABLE. Most are junk the ranking would never have reached anyway, so the
  // repair is not to loosen this rule but to name the survivors: see HAND_ADDED.
  const lemmas = new Set<string>()
  for (const line of hunspellRaw.split("\n")) {
    const entry = line.split("\t")[0]?.split("/")[0]?.trim()
    if (entry && /^[a-z]+$/.test(entry)) lemmas.add(entry)
  }

  function stems(w: string): string[] {
    const out = [w]
    const drop = (n: number) => w.slice(0, -n)
    if (w.endsWith("s")) {
      out.push(drop(1))
      if (w.endsWith("es")) out.push(drop(2), `${drop(2)}e`)
      if (w.endsWith("ies")) out.push(`${drop(3)}y`)
    }
    if (w.endsWith("d")) {
      out.push(drop(1))
      if (w.endsWith("ed")) out.push(drop(2), `${drop(2)}e`)
      if (w.endsWith("ied")) out.push(`${drop(3)}y`)
    }
    if (w.endsWith("ing") || w.endsWith("est")) out.push(drop(3), `${drop(3)}e`)
    if (w.endsWith("er") || w.endsWith("ly")) out.push(drop(2), `${drop(2)}e`)
    return out
  }

  const isEnglish = (w: string) => stems(w).some((s) => lemmas.has(s))

  /*
   * A word that is just a shorter word with S on the end. See the note on
   * `isPlural` in the shared pipeline below for why this is barred at all; the
   * three shapes here are what "add S" means specifically in English.
   *
   *   ZONE  + s   -> ZONES     four-letter word, the one the note is about
   *   BOX   + es  -> BOXES     the sibilant plural
   *   TRY -> TRIES             the y-plural
   *
   * A double S ending is never one of these: the plural of a word ending in S is
   * spelled ES, so GRASS, BLESS and CROSS are answers and always were.
   *
   * Judged against the hunspell lemmas rather than against `allowed`, which holds
   * only five-letter words and could not answer the question at all. Being the
   * small SCOWL list, it errs toward letting a plural through rather than toward
   * eating a real word, which is the right direction: a missed plural is one
   * awkward round, an eaten word is a word nobody ever gets to be dealt.
   */
  function isPlural(w: string): boolean {
    if (!w.endsWith("s") || w.endsWith("ss")) return false
    if (lemmas.has(w.slice(0, -1))) return true
    if (w.endsWith("es") && lemmas.has(w.slice(0, -2))) return true
    return w.endsWith("ies") && lemmas.has(`${w.slice(0, -3)}y`)
  }

  // Answers are the intersection of "allowed", "common" and "English", ranked by
  // corpus frequency. Unlike guesses, these ARE filtered for slurs, since nobody wants
  // one as the secret word. The block list holds lemmas, so it gets the same stem
  // treatment; matching it literally lets RAPES and CUNTS straight through.
  // Substring matching would catch those too, but it also eats GRAPE and SPOON.
  const blocked = new Set([...lines(profanityRaw), ...HAND_BLOCKED, ...HAND_PROFANE.en, ...slurs])
  const isBlocked = (w: string) => stems(w).some((s) => blocked.has(s))
  const allowedSet = new Set(allowed)

  // A hand-added answer nobody can guess is worse than no hand-add at all, and
  // the two lists are written far enough apart to make that an easy thing to do.
  const unguessable = HAND_ADDED.filter((word) => !allowedSet.has(word))
  if (unguessable.length > 0) {
    throw new Error(`hand-added answers that are not legal guesses: ${unguessable.join(", ")}`)
  }

  // The ranking still does the ranking; it just has fewer seats to fill.
  const RANKED_COUNT = EN_ANSWER_COUNT - HAND_ADDED.length
  const handAdded = new Set(HAND_ADDED)

  const ranked: string[] = []
  let plurals = 0
  for (const line of lines(freqRaw)) {
    const word = line.split(/\s+/)[0]
    if (!word || !isTarget(word)) continue
    if (!allowedSet.has(word) || isBlocked(word) || !isEnglish(word)) continue
    if (handAdded.has(word)) continue
    // Counted rather than silently skipped: this rule rejects common words on
    // purpose, and the number is how anyone re-running this can tell it is still
    // rejecting roughly what it was written to reject.
    if (isPlural(word)) {
      plurals++
      continue
    }
    ranked.push(word)
    if (ranked.length >= RANKED_COUNT) break
  }

  if (ranked.length < RANKED_COUNT) {
    throw new Error(`only found ${ranked.length} ranked answers, wanted ${RANKED_COUNT}`)
  }

  return {
    allowed,
    // Sorted, not frequency-ordered: the engine selects by seed, and alphabetical
    // order keeps regeneration diffs readable.
    answers: [...ranked, ...HAND_ADDED].sort(),
    notes: [
      `hand lists ${HAND_ALLOWED.length} allowed, ${HAND_ADDED.length} answers`,
      `skipped ${plurals} plurals`,
    ],
  }
}

// ---------------------------------------------------------------------------
// Spanish, French and German
// ---------------------------------------------------------------------------

/**
 * A hunspell affix file, reduced to the two things this needs from it.
 *
 * The real format carries about forty directives; a spell checker has to obey
 * most of them and a word-list generator has to obey two. `SFX`/`PFX` blocks are
 * the rules, and `FLAG` is how a `.dic` line's flags are cut into individual
 * flag names — one character each by default, two for `FLAG long`, comma-
 * separated numbers for `FLAG num`. All three appear across our four
 * dictionaries (es is UTF-8 single, fr is long, de is default), so all three
 * are handled rather than the one we happened to look at first.
 *
 * A block header is `SFX flag cross count`, and the lines under it are
 * `SFX flag strip add condition`. `strip` is what comes off the stem, `add` is
 * what goes on, `0` means neither, `condition` is a character-class pattern the
 * stem's end (or start, for a prefix) must match. `add` can carry its own
 * continuation flags after a slash; those are dropped, which is the one
 * deliberate incompleteness here and it costs a handful of doubly-derived forms
 * that are almost never five letters long.
 */
type AffixEntry = { strip: string; add: string; condition: string }
type AffixRule = { cross: boolean; entries: AffixEntry[] }
type Affixes = { flags: (raw: string | undefined) => string[]; sfx: Affix; pfx: Affix }
type Affix = Map<string, AffixRule>

function parseAffixes(text: string): Affixes {
  const kind = /^FLAG\s+(\S+)/m.exec(text)?.[1] ?? "char"
  const sfx: Affix = new Map()
  const pfx: Affix = new Map()
  for (const raw of text.split("\n")) {
    const line = raw.trim()
    if (!line.startsWith("SFX ") && !line.startsWith("PFX ")) continue
    const parts = line.split(/\s+/)
    const table = parts[0] === "SFX" ? sfx : pfx
    const flag = parts[1]
    if (!flag) continue
    // A header declares the flag and says whether it may combine with the other
    // kind; everything after it is a rule under that flag.
    if (parts[2] === "Y" || parts[2] === "N") {
      table.set(flag, { cross: parts[2] === "Y", entries: [] })
      continue
    }
    const rule = table.get(flag)
    if (!rule || !parts[3]) continue
    rule.entries.push({
      strip: parts[2] === "0" ? "" : (parts[2] ?? ""),
      add: parts[3] === "0" ? "" : (parts[3].split("/")[0] ?? ""),
      condition: parts[4] ?? ".",
    })
  }
  const flags = (raw: string | undefined): string[] => {
    if (!raw) return []
    if (kind === "long") return raw.match(/../g) ?? []
    if (kind === "num") return raw.split(",")
    return [...raw]
  }
  return { flags, sfx, pfx }
}

const matchesEnd = (stem: string, condition: string) =>
  condition === "." || new RegExp(`(?:${condition})$`).test(stem)
const matchesStart = (stem: string, condition: string) =>
  condition === "." || new RegExp(`^(?:${condition})`).test(stem)

/**
 * What expanding a dictionary yields, which is three sets and not one.
 *
 * `all` is every folded five-letter form, from every entry, and it is the guess
 * list. `common` is the same but only from entries the dictionary spelled in
 * lowercase, which is the proper-noun filter the English path gets from its own
 * hunspell file — capitalized entries are names and a name is never the answer.
 * `stems` is every folded form of five letters *or fewer*, common ones only,
 * and it exists for `isPlural`: to know that VIDAS is a plural you have to know
 * that VIDA is a word, and VIDA is four letters.
 *
 * That last set is why the plural rule works at all here. The first version
 * asked the `.dic` directly, the way the English one asks its hunspell file, and
 * caught almost nothing: Spanish stores VIDA nowhere, because the entry is VIDO
 * — no, it is not stored at all, the feminine is an affix of another entry.
 * Against bare `.dic` entries the rule rejected 459 Spanish words and left 5.9%
 * of the answer list ending in S; against expanded forms it rejects 556 and
 * leaves 2.1%, which is the English list's own 2.0% to within a rounding error.
 */
type Expansion = { all: Set<string>; common: Set<string>; stems: Set<string> }

function expand(dic: string, aff: Affixes): Expansion {
  const all = new Set<string>()
  const common = new Set<string>()
  const stems = new Set<string>()
  // The first line of a .dic is the entry count, and some ship a licence block
  // indented with tabs. Neither is a word.
  for (const raw of dic.split("\n").slice(1)) {
    if (raw.startsWith("\t") || raw.startsWith(" ")) continue
    const line = raw.trim().split("\t")[0]
    if (!line) continue
    const slash = line.indexOf("/")
    const word = (slash === -1 ? line : line.slice(0, slash)).trim()
    // Hyphens, digits and apostrophes are all over these files (`1er`,
    // `accroche-cœur`), and none of them can reach a board of 26 letters.
    if (!word || /[^\p{L}]/u.test(word)) continue
    const isCommon = word === word.toLowerCase()
    const keep = (form: string) => {
      const folded = fold(form)
      if (!/^[a-z]+$/.test(folded)) return
      if (folded.length === WORD_LENGTH) {
        all.add(folded)
        if (isCommon) common.add(folded)
      }
      if (isCommon && folded.length <= WORD_LENGTH) stems.add(folded)
    }
    keep(word)

    const suffixes: AffixEntry[] = []
    const prefixes: AffixEntry[] = []
    const crossing = { sfx: [] as AffixEntry[], pfx: [] as AffixEntry[] }
    for (const flag of aff.flags(slash === -1 ? undefined : line.slice(slash + 1))) {
      const suffix = aff.sfx.get(flag)
      if (suffix) {
        for (const entry of suffix.entries) {
          if (!word.endsWith(entry.strip) || !matchesEnd(word, entry.condition)) continue
          suffixes.push(entry)
          if (suffix.cross) crossing.sfx.push(entry)
        }
      }
      const prefix = aff.pfx.get(flag)
      if (prefix) {
        for (const entry of prefix.entries) {
          if (!word.startsWith(entry.strip) || !matchesStart(word, entry.condition)) continue
          prefixes.push(entry)
          if (prefix.cross) crossing.pfx.push(entry)
        }
      }
    }
    const suffixed = (stem: string, entry: AffixEntry) =>
      stem.slice(0, stem.length - entry.strip.length) + entry.add
    const prefixed = (stem: string, entry: AffixEntry) => entry.add + stem.slice(entry.strip.length)

    for (const entry of suffixes) keep(suffixed(word, entry))
    for (const entry of prefixes) keep(prefixed(word, entry))
    // Both ends at once, which hunspell allows only when both rules said Y.
    for (const prefix of crossing.pfx) {
      const base = prefixed(word, prefix)
      for (const suffix of crossing.sfx) {
        if (!base.endsWith(suffix.strip)) continue
        keep(suffixed(base, suffix))
      }
    }
  }
  return { all, common, stems }
}

/**
 * How each language spells "and one more of them".
 *
 * Each pair is a suffix and the stem it implies: drop the suffix, append the
 * replacement, and if what is left is a word then the word in hand is that word
 * plus a plural marker. English's own rule is spelled out separately above,
 * against a different lemma source, because its output is frozen.
 *
 * Spanish is the clean case, -S after a vowel and -ES after a consonant.
 * French adds -X, which is the plural of -AU and -EU (TUYAUX, CHEVEUX), and
 * -AUX for the -AL nouns (CHEVAL, CHEVAUX).
 *
 * German was going to be skipped. The plan said its plurals are too irregular
 * for a suffix rule — they are, there are five endings and half of them umlaut
 * the stem — and that skipping was the honest move. Then the number came in:
 * without any rule, 7.0% of the German answer list ended in a bare S against
 * English's 2.0%, and reading them showed why. They are not plurals. They are
 * genitives (TAGES, IHRES), neuter adjective endings (NEUES, GUTES, JEDES) and
 * the loanword plural (FOTOS, AUTOS) — all of them a stem with a free letter on
 * the end, which is the thing the rule exists to stop, whatever grammar calls
 * it. The -S/-ES pair catches 117 of them and takes the rate to 3.0%.
 *
 * What is left in German is the other four plural endings, and leaving them is
 * the deliberate half of this. HUNDE, AUGEN and JAHRE are all answers. A German
 * plural can end -E, -EN, -ER, -S or nothing at all, so knowing the answer is
 * plural tells a player nothing about the last tile; the English objection is
 * specifically that -S is a *free letter in a known slot*, and German has no
 * such letter to give away.
 *
 * All three rules eat real words on the way, the same way the English one does,
 * and it is worth naming the casualties rather than only the counts: ANTES,
 * MENOS and JAMÁS in Spanish, APRÈS, COURS and HÉROS in French, ETWAS and LINKS
 * in German. Each is a word in its own right that happens to be some other word
 * plus an S. Each is still a legal guess and only never the secret one, which is
 * the trade the English list has always made — and every one of them ends in S,
 * which is the shape the rule exists to make rare.
 */
const PLURALS: Record<Exclude<Lang, "en">, [suffix: string, stem: string][]> = {
  es: [
    ["s", ""],
    ["es", ""],
  ],
  fr: [
    ["s", ""],
    ["x", ""],
    ["aux", "al"],
  ],
  de: [
    ["s", ""],
    ["es", ""],
  ],
}

/**
 * How many answers each language gets, and why it is not 2300.
 *
 * English draws its 2300 from the top 5,223 ranks of a 50k frequency list,
 * because English is analytic and thick with short words: 15,949 of them are
 * five letters long. The other three are not. Spanish and French inflect, German
 * compounds, and after expansion each yields between 4,300 and 8,800 five-letter
 * forms against English's 15,949. Once the proper-noun, profanity and plural
 * filters have run, the eligible answers are:
 *
 *   es 2,323    fr 2,192    de 2,231
 *
 * Taking 2,300 of those is not a ranking, it is the whole barrel including what
 * is at the bottom of it — the 2,300th Spanish answer sits at corpus rank
 * #44,517 and is MOJAS, HUCHA, FLUIA. 2,000 leaves each language between 192 and
 * 323 words of headroom, which is enough for the frequency ranking to still be
 * doing something, and puts the last answer around rank #33,000 (TACHO, OBESO,
 * ZARPO) rather than #45,000. Reading the list at 1,500 it is better again and
 * at 1,000 better still; 2,000 is where the pool stops feeling like a different
 * game from the English one, whose 2,300 nobody has complained about.
 *
 * These are honestly deeper ranks than English's, and that is a fact about the
 * corpora rather than a compromise made here: matching English's *rank depth*
 * instead of its count would give Spanish 1,041 answers and French 939.
 */
const ANSWER_COUNTS: Record<Lang, number> = {
  en: EN_ANSWER_COUNT,
  es: 2000,
  fr: 2000,
  de: 2000,
}

async function expanded(lang: Exclude<Lang, "en">): Promise<Lists> {
  const [dicRaw, affRaw, freqRaw, profanityRaw] = await Promise.all([
    fetchText(dictionary(lang, "dic")),
    fetchText(dictionary(lang, "aff")),
    fetchText(frequency(lang)),
    fetchText(profanity(lang)),
  ])

  const { all, common, stems } = expand(dicRaw, parseAffixes(affRaw))

  // Ranked, deduplicated, and folded on the way in, so CANTÉ and CANTE are one
  // word here exactly as they are one word on the board. Order is the corpus's.
  const ranked: string[] = []
  const seen = new Set<string>()
  for (const line of lines(freqRaw)) {
    const word = fold(line.split(/\s+/)[0] ?? "")
    if (!isTarget(word) || seen.has(word)) continue
    seen.add(word)
    ranked.push(word)
  }

  // Every legal guess: the dictionary, plus anything the corpus saw often enough
  // to make the top 50,000 words of it. The second half is what the English list
  // gets from words_alpha, which carries FRANK and ALICE and every other name a
  // player might reasonably type. The dictionaries here are lemma-and-affix and
  // so much smaller than a scrape — 4,320 five-letter forms for German — and a
  // word game that refuses a real word is worse than one that accepts a name.
  // Minus the slurs, which is the same carve-out the English guess list takes
  // and is made here too rather than there, since `ranked` carries whatever the
  // subtitles said and the dictionaries are not the only way in.
  const slurs = new Set(SLURS[lang])
  const allowed = [...new Set([...all, ...ranked])].filter((word) => !slurs.has(word)).sort()

  // Answers are filtered against the slurs as well as the profanity, and not
  // merely by inheriting `allowed`: this pipeline ranks answers straight out of
  // the corpus and never asks whether they were guessable, so a word removed
  // above would otherwise come back as the secret one and trip the guessability
  // check at the bottom of this file.
  const blocked = new Set([...lines(profanityRaw).map(fold), ...HAND_PROFANE[lang], ...slurs])
  const suffixes = PLURALS[lang]
  const isPlural = (w: string) =>
    !w.endsWith("ss") &&
    suffixes.some(
      ([suffix, stem]) => w.endsWith(suffix) && stems.has(w.slice(0, -suffix.length) + stem),
    )

  const want = ANSWER_COUNTS[lang]
  const answers: string[] = []
  let plurals = 0
  let names = 0
  for (const word of ranked) {
    // A capitalized-only dictionary entry is a proper noun, and the corpus is
    // subtitles, so this is the filter doing most of the work: PETER, HARRY and
    // SARAH are all in the top 5,000 of every one of these lists.
    if (!common.has(word)) {
      names++
      continue
    }
    if (blocked.has(word)) continue
    if (isPlural(word)) {
      plurals++
      continue
    }
    answers.push(word)
    if (answers.length >= want) break
  }

  if (answers.length < want) {
    throw new Error(`${lang}: only found ${answers.length} answers, wanted ${want}`)
  }

  return {
    allowed,
    answers: answers.sort(),
    notes: [
      `dictionary ${all.size} forms, ${common.size} lowercase`,
      `skipped ${names} not in the dictionary, ${plurals} plurals`,
    ],
  }
}

// ---------------------------------------------------------------------------

const requested = process.argv.slice(2)
const targets: Lang[] = requested.length
  ? requested.map((arg) => {
      const lang = LANGS.find((entry) => entry === arg)
      if (!lang) throw new Error(`unknown language ${arg}; have ${LANGS.join(" ")}`)
      return lang
    })
  : [...LANGS]

for (const lang of targets) {
  const { allowed, answers, notes } = lang === "en" ? await english() : await expanded(lang)

  // Every answer has to be guessable, which is trivially true when both lists
  // come from the same expansion and was worth asserting anyway: it is the one
  // way to build a run nobody can finish.
  const guessable = new Set(allowed)
  const unguessable = answers.filter((word) => !guessable.has(word))
  if (unguessable.length > 0) {
    throw new Error(`${lang}: answers that are not legal guesses: ${unguessable.join(", ")}`)
  }

  const dir = join(OUT_DIR, lang)
  await mkdir(dir, { recursive: true })
  await writeFile(join(dir, "allowed.txt"), `${allowed.join("\n")}\n`)
  await writeFile(join(dir, "answers.txt"), `${answers.join("\n")}\n`)

  console.log(`${lang}  allowed ${allowed.length}  answers ${answers.length}`)
  for (const note of notes) console.log(`    ${note}`)
}
