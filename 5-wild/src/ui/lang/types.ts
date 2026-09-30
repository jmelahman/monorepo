import type {
  BossTier,
  Color,
  ConsumableNote,
  Growth,
  GuessNote,
  ModId,
  PackId,
  Payout,
  Rarity,
  Refusal,
} from "../../engine"
import type { Skin } from "../skin"

/**
 * The shape every language has to fill.
 *
 * This file is the contract and `en.ts` is the reference filling of it. A new
 * language is a new module of this type and nothing else: if it compiles, it is
 * complete, which is the whole reason the shape is written down rather than
 * left as whatever `en` happens to export. A missing key is a type error at
 * build time instead of an `undefined` on someone's screen.
 *
 * Nothing is left in the engine and nothing is left in the screens: `ui` at the
 * bottom is the second half, and it is the larger one.
 */

export type Lang = "en" | "es" | "fr" | "de"

export const LANGS: readonly Lang[] = ["en", "es", "fr", "de"]

/** A card's two strings: what it is called and what it does. */
export type Card = { name: string; text: string }

/**
 * A card whose sentence is built from the balance table it describes.
 *
 * Anything a translator would otherwise have to re-type out of the content
 * files is a parameter instead, so a nerf never has to be chased through four
 * languages. Only the etchings and the endless ascension need it: they are the
 * two places where the number in the sentence is also a field somebody can
 * edit. Relics and bosses keep their numbers written into the prose, because
 * there the number lives in the effect's own code with no field to read it
 * from, and restating it in the sentence is what the file already does.
 */
export type CardOf<A extends unknown[]> = { name: string; text: (...args: A) => string }

/**
 * One entry per refusal, derived from the engine's own union.
 *
 * A code that carries nothing is a plain string, since there is nothing to build
 * the sentence out of; a code that carries operands is a function of exactly
 * those operands. Both halves come from `Refusal` by construction, so a rule
 * that starts refusing something new, or an existing one that starts carrying a
 * number, is a compile error in every language at once. That is the property
 * worth having here: refusals are read at the moment the player is already
 * confused, and a missing one would show as silence.
 */
export type Refusals = {
  [C in Refusal["code"]]: RefusalEntry<Omit<Extract<Refusal, { code: C }>, "code">>
}

type RefusalEntry<O> = keyof O extends never ? string : (operands: O) => string

/** A titled paragraph in the rules sheet: the term in bold, then the sentence. */
export type Rule = { term: string; text: string }

/** A titled paragraph whose sentence quotes a number the content files own. */
export type RuleOf<A extends unknown[]> = { term: string; text: (...args: A) => string }

/** A collapsible codex section: the heading, and the line under it. */
export type Section = { title: string; blurb: string }

/** A codex section whose blurb quotes a number the content files own. */
export type SectionOf<A extends unknown[]> = { title: string; blurb: (...args: A) => string }

export type Strings = {
  /** BCP 47, for `document.documentElement.lang` and `Intl`. */
  tag: string
  /** What this language calls itself, for the picker. Never translated. */
  endonym: string

  relic: Record<string, Card>
  boss: Record<string, Card>
  modifier: Record<ModId, Card>
  consumable: Record<string, Card>
  pack: Record<PackId, Card>
  category: Record<string, Card>
  etching: Record<string, CardOf<[chips: number]>>
  /**
   * Keyed by authored level. `steeper` is the rung the endless ladder
   * synthesizes past level 10, which is why it is a function: its two numbers
   * are computed from how far past the end the run has gone.
   */
  ascension: Record<number, Card> & {
    steeper: CardOf<[percent: number, total: string]>
    /**
     * Added to the first endless rung's text, the one place the ladder takes
     * something besides a notch of target: the first-guess clear.
     */
    endsFirstGuess: string
  }
  round: readonly [string, string, string]

  /** Why an action was turned down, said in one short line. */
  refusal: Refusals

  /**
   * The prose the *event stream* used to carry preformatted, now that it carries
   * operands instead. Not the screens' own copy, which is a separate move; these
   * three are here because stripping the tables is what took them off the events,
   * so leaving them out would mean shipping a step that does not run.
   *
   * Each is a whole sentence built in one place. The temptation is to expose the
   * pieces — a word for "chips", a word for "level" — and paste them together at
   * the call site, which works in English and then puts the adjective on the
   * wrong side of the noun somewhere else.
   */
  event: {
    /** What a scaling relic wears and announces: "+120 chips". */
    growth: (growth: Growth) => string
    /**
     * What a card said as it fired: "+20", "×3 mult". Read in two places at
     * once, the floater over the board and the line in a tile's tip, and it has
     * to be short enough for the first, which is what keeps the chip case a bare
     * number. The tile it floats over is the noun.
     */
    payout: (paid: Payout) => string
    /** The floater over a leveled category: "Cluster Lv 3". */
    categoryLevel: (name: string, level: number) => string
    /** The toast when a bought modifier lands: "Steel E". `letter` is lowercase. */
    modPlaced: (name: string, letter: string) => string
    /** The toast after a one-shot card. `letter` arrives lowercase. */
    note: (note: ConsumableNote) => string
    /** What a boss says about a row instead of coloring it. */
    guessNote: (note: GuessNote) => string
  }

  /**
   * Everything the screens say for themselves.
   *
   * Grouped by screen rather than alphabetically, because a translator works a
   * screen at a time with the thing in front of them, and because a sentence's
   * neighbours are most of what tells you how long it is allowed to be. Two
   * shapes were deliberately not extracted mechanically:
   *
   * A line whose *shape* changes with the state is two entries, not one with an
   * optional fragment. The quit sheet is the case: it says "stage 4 of 8" during
   * a run and "stage 12" past the end, and a single string with a hole in it
   * would force every language to build the sentence the way English happens to.
   *
   * A panel assembled with `lines.join("\n")` is one entry per line, and the
   * join stays in code. The tips are the case: which lines appear is a question
   * about the run, not about the language, and a catalog that owned the whole
   * panel would have to be told the answer.
   *
   * What stayed in code is punctuation with no words in it: the `·` between
   * clauses, the `›` on a link, the `×` in the readout. A separator is not a
   * sentence, and a table of them would be a table nobody ever edits.
   *
   * A figure arrives as a `number` where the sentence around it has to agree
   * with it (a plural, a small count) and as an already-formatted `string`
   * where it does not. The second kind is anything `formatNumber` has been
   * through: a score, a target, a chip total. Those are six digits often
   * enough that the abbreviating is the point, and the catalog would only be
   * handed the number to hand it straight back.
   */
  ui: {
    /**
     * The abbreviation ladder every large number on screen is written with, from
     * thousands upward. See `formatNumber`, which owns the arithmetic; this is
     * only what each rung is called.
     *
     * It is prose and not notation, which is easy to miss because it looks like
     * notation. The long scale is the reason: `10^9` is a billion in English and
     * a milliard in French and German, so the third rung is `B`, `Md` and `Mrd`
     * and the *fourth* rung of one language means the third rung of another. A
     * translator filling this in is translating the words, not transliterating
     * the letters.
     *
     * Read positionally, so a language that runs out early would misprint every
     * rung after it rather than one. Only the first three or four are reachable
     * in a run that ends; endless has no last stage and so no last rung, which is
     * why the tail is written out past anything anyone will see.
     */
    units: readonly string[]

    /**
     * The shell's own screen, drawn when there is no game to draw.
     *
     * It survives the failure it describes because the catalog is bundled and
     * the word lists are fetched: whatever went wrong with the network, the
     * sentence about it was already on the page. The cause is appended raw and
     * untranslated, because it is a status code or a browser's own message and
     * the person who can act on it is reading a bug report.
     */
    error: {
      words: (cause: string) => string
    }

    /**
     * The spectator's banner, over a run someone else is playing: a model on
     * the benchmark host, or an episode it recorded. See `src/ui/spectate.ts`.
     *
     * The label and the refused command arrive from the host as they were
     * typed, and the refusal's reason is the host's English: it is the sentence
     * the model was shown, and translating it would be showing the spectator a
     * different sentence from the one being answered.
     */
    watch: {
      /** Before the host has said which run it is dealing. */
      waiting: string
      /** The host is unreachable; the browser retries by itself. */
      lost: string
      seed: (seed: number, ascension: number) => string
      move: (n: number) => string
      refused: (command: string, reason: string) => string
      over: string
      /** The host is on another content version, so its moves will not fit this game. */
      version: (host: number, here: number) => string
      /** A move this browser's engine refused, which the host accepted. */
      diverged: (n: number) => string
    }

    /** Said the same way on more than one screen, so it is written once. */
    common: {
      close: string
      back: string
      play: string
      howToPlay: string
      codex: string
      /**
       * The title speaker's `aria-label`, one per level. Each is the setting's
       * name and then its value ("Sound: off"), so it reads as what is playing
       * now and never as what a tap will do.
       */
      sound: { off: string; sound: string; music: string; musicOnly: string }
      /** The level, named as a level: "Ascension 3". */
      ascension: (level: number) => string
      /**
       * A share of something. Its own entry because the sign is not `%`
       * everywhere and does not sit tight against the number everywhere.
       */
      percent: (share: number) => string
      /** For a figure no run has earned yet. An em dash, not a zero. */
      none: string
      /** While a word list is on its way. See `loadingView`. */
      loading: string
    }

    /** The round screen: HUD, tray, keyboard, and the lines under the board. */
    board: {
      /** On the ☰, which has no text of its own. */
      menu: string
      /**
       * The letter-values switch, in each of its three states. The label names
       * what the *next* tap does, so the three entries are a cycle and have to be
       * read as one.
       */
      decor: Record<"all" | "minimal" | "none", { label: string; tip: string }>
      stage: (stage: number, total: number) => string
      /** Past the last authored stage there is no denominator to print. */
      stageEndless: (stage: number) => string
      /** Two characters in the HUD corner: "A3". */
      ascensionTag: (level: number) => string
      /** Under the score. The number arrives already formatted. */
      target: (target: string) => string
      /** A tray card's panel, and the same again for a screen reader. */
      relicTip: (text: string, detail: string) => string
      relicLabel: (name: string, text: string) => string
      relicLabelGrown: (name: string, text: string, detail: string) => string
      /** The two wide keys. Short enough to fit one, whatever else changes. */
      enter: string
      del: string
      solveFactor: (factor: number) => string
      solveFloor: (score: string) => string
      solveFloorClears: (score: string) => string
      /** Shared with the shapes sheet, which asks the same question of a shape. */
      shapeLevel: (level: number) => string
      /** Why the mult reads `?` while a word is being typed. */
      multUnknown: string
      /** The floater when a letter breaks. Not a tile's, so it names the letter. */
      letterBroken: (letter: string) => string
      /** The floater when a relic leaves the tray on its own. Gets the card's name. */
      relicGone: (name: string) => string
    }

    /**
     * The two hover panels, line by line. Which lines a panel gets is decided by
     * the run; each entry here is one of them, whole.
     */
    tip: {
      /** A played tile's headline: what this letter put in. */
      tileChips: (letter: string, chips: number) => string
      /** A key's headline: what this letter is worth right now. */
      keyChips: (letter: string, chips: number) => string
      broken: (letter: string) => string
      base: (chips: number) => string
      etched: (chips: number) => string
      fromRange: (chips: number, range: string, level: number) => string
      /** The boss, named only where it actually moved this letter. */
      boss: (name: string, text: string) => string
      /** The color that scored, and what color is worth. */
      color: (color: Color, mult: number) => string
      /** A modifier that fired, wearing the label it gave at the time. */
      mod: (name: string, badge: string) => string
      /** A modifier the run is holding, with nothing to report. */
      modIdle: (name: string, text: string) => string
      /** Under The Vandal: bought, placed, and not firing this round. */
      modSilenced: (name: string, text: string) => string
      /** Asked and had nothing to say: Lucky rolled and lost. */
      modQuiet: (name: string, text: string) => string
      relic: (name: string, badge: string) => string
      /** The foot of a tile's panel, in its two shapes. Numbers arrive formatted. */
      share: (chips: string, total: string) => string
      shareWithMult: (chips: string, total: string, mult: string, multTotal: string) => string
    }

    /** The card between the shop and the board. */
    intro: {
      stage: (stage: number, total: number) => string
      stageEndless: (stage: number) => string
      scoreAtLeast: string
      /** The word under the guess count on the card: "6 / guesses". */
      guesses: (guesses: number) => string
      /** The word under the payout on the card: "$4 / reward". */
      reward: string
      /** What the stage track is called to a screen reader. */
      track: string
      /** The three rounds of a stage as the track names them, token beside each. */
      trackRound: readonly [string, string, string]
      /** A round of this stage already won. */
      cleared: string
      /** The round this card is announcing. */
      current: string
      /** The round after it. */
      upNext: string
      /** The endless half of the ladder, said as the number it is. */
      targets: (factor: string) => string
      /** The two buttons are the whole question, asked once before the first board. */
      coachYes: string
      coachNo: string
      /** When there is nothing to ask, the button is just the button. */
      play: string
    }

    reward: {
      cleared: string
      /** Shared with the end screen, which reveals the same word for a worse reason. */
      answerWas: (word: string) => string
      score: (score: string, target: string) => string
      unusedGuesses: string
      interest: string
      /** The line for gold a relic paid on top of the round. */
      relics: string
      /** Why a round under its target is being called cleared. Gets the card's name. */
      savedBy: (name: string) => string
      /** Why a round under its target is being called cleared, when the word came first try. */
      firstGuess: string
      total: string
      collect: string
    }

    /**
     * The shelf, and every kind of thing sold on it. `tag` is the ticket at the
     * top right of a card and `tip` is the sentence behind it, so the two are
     * written together: the tag is only useful if the tip can finish it.
     */
    shop: {
      title: string
      sold: string
      sell: (amount: string) => string
      /** The label on a held card in the shop, where a tap throws it away for nothing. */
      drop: string
      /** The verb alone: the button sets the price after it, in gold. */
      reroll: string
      nextRound: string
      /** Hover tip on a shelf card the player cannot afford: the shortfall, as money. */
      short: (amount: string) => string
      shapesLabel: string
      shapesLevel: (name: string, level: number) => string
      shapesNone: string

      tagPack: string
      tipPack: (picks: number) => string
      tagRelic: string
      tipRelic: string
      tagConsumable: string
      /**
       * The line a consumable card grows when the tray has no seat for it. A
       * line rather than a second tag: the tag is a drawing now, and a drawing
       * cannot say "full".
       */
      slotsFull: string
      tipConsumable: string
      tagLetter: string
      tipMod: string
      tagRange: string
      tipRange: string
      tagShape: string
      tipShape: string
      tagEtching: string
      tipEtching: string

      /** The shop's unaimed modifier: a decision, not a letter. */
      modAnyTitle: (name: string) => string
      modAnyText: (text: string) => string
      modAnyTextOnly: (text: string, letters: string) => string
      /** A pack's aimed one, letter already chosen. */
      modTitle: (name: string, letter: string) => string
      modText: (letter: string, text: string) => string
      /** What tapping it would throw away. Named with its pip, as the key wears it. */
      swap: (name: string, pip: string) => string

      /** A level on a card's ladder, "Lv 3": said twice, from and to. */
      level: (level: number) => string
      /** Spelled out letter by letter, and shared with the codex's own list. */
      rangeText: (letters: string, chips: number) => string
      levelText: (name: string, chips: number, mult: number) => string
      /** Titles for a save that names something this build no longer sells. */
      fallbackRange: string
      fallbackLevel: string
      fallbackEtching: string
    }

    pack: {
      choose: (left: number) => string
      choosePicks: (picks: number, left: number) => string
      taken: string
      skip: string
      skipSome: string
    }

    /** The letter picker, and the question it asks before it destroys anything. */
    place: {
      choose: (text: string) => string
      onlyOn: (name: string, letters: string) => string
      oneEach: string
      /**
       * The swap warning, in two halves with the outgoing card's name in bold
       * between them. Split there because the name is what the block is colored
       * for, and every one of these languages puts the object of "is carrying"
       * last, so the lead can end where the bold begins.
       */
      carrying: (letter: string) => string
      loses: (name: string) => string
      replace: (name: string) => string
      keep: (name: string) => string
    }

    end: {
      won: string
      lost: string
      /** The score that fell short, and by how much. */
      short: (score: string, target: string, by: string) => string
      /** A run that had already won and went on anyway. */
      wonAndWent: (stages: number) => string
      reached: (stage: number, round: string) => string
      endlessNote: (stages: number) => string
      endless: string
      mainMenu: string
      newRun: string
      /** What the level meant, at the end rather than only at the start. */
      firstEarned: string
      earned: (level: number, next: number) => string
      topOfLadder: (level: number) => string
    }

    title: {
      /** The game's own name. A proper noun, and not really a translation. */
      name: string
      tagline: string
      /** The one-line record, which is also the button that opens the long one. */
      runs: (count: number) => string
      wins: (count: number) => string
      bestStage: (stage: number) => string
    }

    stats: {
      title: string
      runs: string
      /**
       * The one figure label that agrees with its own number, because a single
       * win is the rare one and "1 WINS" would spoil it. The others are counts
       * of ordinary things and read as headings.
       */
      wins: (count: number) => string
      bestStage: string
      guesses: string
      solved: string
      meanSolve: string
      /**
       * The collection, and its denominator when the word list has landed.
       *
       * One sentence in three keys, because the count is bold and the rest of it
       * is not. All three take the count, including the two with no number in
       * them: English "cracked" is the same word after a 1 and after a 273, and
       * `descifrada` and `trouvé` are not. Written without it, the tail could
       * only ever be the plural, which is wrong exactly once per language and on
       * the screen a player sees first.
       */
      cracked: (count: number) => string
      crackedBare: (count: number) => string
      crackedOf: (count: number, pool: string) => string
      /**
       * The other collection, in the same three pieces and for the same reason:
       * every distinct word this player has legally typed, against everything
       * the language will accept.
       *
       * Split from `cracked` rather than sharing it, even where a language
       * spells the count identically, because the tail is a different
       * participle agreeing with a different verb, and one of the four
       * languages disagreeing is enough to make the pair two keys. That is the
       * same reasoning that gave `crackedBare` its ignored count.
       */
      played: (count: number) => string
      playedBare: (count: number) => string
      playedOf: (count: number, pool: string) => string
      mostPlayed: string
      times: (count: number) => string
      breakdown: string
      solvedIn: (guesses: number) => string
      neverFound: string
      favoriteRelics: string
      taken: (count: number) => string
      noStreak: string
      streakBest: (now: string) => string
      streakWithNow: (best: string, now: string) => string
      streak: (best: string) => string
    }

    /** The difficulty dial, and the sheet behind its lock. */
    ladder: {
      carrot: string
      /**
       * Rung zero's two lines, standing where every other rung has its rule's
       * name and its sentence, so the card is one shape at every level.
       */
      base: string
      baseText: string
      lower: string
      raise: string
      locked: string
      skipTo: (level: number) => string
      /** Said once, under the rule, rather than at every rung. */
      andBelow: string
      askTitle: (level: number) => string
      /** The rung's name, in bold, ahead of what it does. */
      ruleLabel: (name: string) => string
      askAndBelow: string
      intended: string
      skipAnyway: (level: number) => string
    }

    /**
     * The rules sheet. Read once, at the start, so it is the longest prose in
     * the game and the only place a paragraph is allowed.
     */
    help: {
      title: string
      lead: string
      scored: string
      chipsMult: Rule
      letters: Rule
      colors: Rule
      solving: Rule
      farming: string
      solveLine: Rule
      runHeading: string
      target: RuleOf<[stages: number, rounds: number, authored: number]>
      endless: RuleOf<[stages: number]>
      bosses: Rule
      ascensions: RuleOf<[authored: number]>
      money: RuleOf<[payouts: string, perGuess: string, per: string, cap: string]>
      relics: RuleOf<[slots: number]>
      packs: Rule
      mods: Rule
      codexNote: string
      /**
       * The tutorial's second door, at the top of the sheet for a player who
       * has neither played it nor turned it down. `tutorialStart` is the button.
       */
      tutorial: string
      tutorialStart: string
      openCodex: string
      gotIt: string
    }

    shapes: {
      title: string
      /** The word on the board, and which of the five it scores as. */
      scoresAs: (word: string, shape: string) => string
      anyWord: string
      note: string
      scoring: string
      alsoMatches: string
      payNow: (chips: number, mult: number) => string
      payPerLevel: (chips: number, mult: number) => string
    }

    codex: {
      title: string
      lead: string
      relics: SectionOf<[slots: number]>
      bosses: Section
      ascensions: Section
      shapes: Section
      mods: Section
      upgrades: Section
      consumables: SectionOf<[slots: number]>
      packs: Section
      /** The band headings inside a section. */
      rarity: Record<Rarity, string>
      tier: Record<BossTier, string>
      tierBand: (tier: string, first: number, last: number) => string
      shapePer: (chips: number, mult: number) => string
      modName: (name: string, pip: string) => string
      modText: (text: string) => string
      modTextOnly: (text: string, letters: string) => string
      packText: (text: string) => string
      packTextPicks: (text: string, picks: number) => string
    }

    pause: {
      title: string
      /**
       * The settings rows, each the name of a setting and never its state: the
       * value sits on the right of the row, drawn as a switch or written out
       * (the track's title, `×2`, a flag and an endonym), and none of it is
       * the translator's. See `setting` in `views.ts`.
       */
      sound: string
      music: string
      track: string
      speed: string
      /** The look, on every layout: one row for what used to be a theme and a skin. */
      skin: string
      /** Each look's name, the row's value: a place or a style, so the translator's. */
      skins: Record<Skin, string>
      /**
       * Also the noun in front of the title screen's language pill in its
       * accessible name, which is what tells a screen reader that "Español" is
       * the setting rather than the destination.
       */
      language: string
      /**
       * Said under the button only while it is true: a run already dealt is
       * played out in the words it was dealt from, whatever the interface has
       * moved to since. Short, because it is a reassurance and not an
       * instruction — nothing is being asked of the player.
       */
      wordsNextRun: string
      quit: string
      resume: string
    }

    /**
     * The sheet behind the title screen's ⓘ: the things about the game rather
     * than about a run, which is why it is not the pause sheet.
     */
    about: {
      /** The sheet's heading, and the ⓘ button's accessible name, since it has no other. */
      title: string
      /**
       * The switch, and unlike music it does not state itself: the track at the
       * row's end says on or off, and the label says what is being shared.
       */
      sharing: string
      /** Under the switch on the about sheet: what is sent, and what is not. */
      sharingNote: string
      /**
       * A sentence after the note, pointing at the privacy page's account of
       * exactly what a run carries; only `link` is the link. Three pieces
       * rather than one template, because where the name falls in the sentence
       * is the language's business, and either end may be empty.
       */
      privacy: { before: string; link: string; after: string }
      /** Said the moment sharing is turned on, wherever it was turned on. */
      thanks: string
      credits: string
      /** Opens the repository in the browser. */
      source: string
      /** Opens a new bug report on GitHub, partly filled in. */
      report: string
    }

    /**
     * Names and titles are not here: they are the same in every language, so the
     * view holds them and these are only the words around them.
     */
    credits: {
      title: string
      madeBy: string
      music: string
      /**
       * The sentence around the artist's name, which the view draws as a link
       * to their site. Two pieces for the same reason as `about.privacy`: where
       * the name falls is the language's business.
       */
      musicText: (piece: string) => { before: string; after: string }
      sounds: string
      /** `people` arrives already joined, by `Intl.ListFormat` in the language in force. */
      soundsText: (people: string) => string
      licence: string
      licenceText: string
      /** The display face the desktop table sets its numbers and headings in. */
      font: string
      /** Names the face and its licence; the designer's name is the view's, as the artists' are. */
      fontText: string
      back: string
    }

    quit: {
      title: string
      /** Two whole sentences, because past the last stage there is no "of". */
      body: (stage: number, total: number, round: string) => string
      bodyEndless: (stage: number, round: string) => string
      confirm: string
      cancel: string
    }

    /**
     * Seeded runs: the sheet behind the title screen's version stamp, which a
     * `?seed=` link also opens, and the one line on the pause sheet and the end
     * screen that names the run in hand. The code itself is never the
     * translator's: it is seven characters of base 32, and it is the same code
     * in every language.
     */
    seed: {
      title: string
      /** The version stamp's accessible name, since its face is only a version. */
      open: string
      /** One sentence on what a seed is, above the field. */
      blurb: string
      /** The field's label. */
      field: string
      play: string
      /** Under Play while a run is open, which this would replace. */
      abandons: string
      /** A toast for a code that is not one, which says what one looks like. */
      badCode: string
      /** Only when a link's list differs from the interface: the run is dealt from it. */
      words: (language: string) => string
      /** The pause sheet's and end screen's line, which copies the link. */
      line: (code: string) => string
      /** The line's accessible name, since its face does not say it copies. */
      copy: string
      copied: string
    }

    /**
     * The first round, taught while it is played. Each beat quotes a live figure
     * off the board beside it, which is the whole reason these are functions and
     * the reason they cannot be shortened much: the sentence has to say what the
     * number it is pointing at means.
     */
    coach: {
      chips: string
      rare: (chips: string) => string
      mult: string
      banked: (chips: string, mult: string, score: string, target: string) => string
      /**
       * The meter, part by part: `score` of `target` is the solid bar, `now` is
       * what solving on this guess multiplies by, and `floor` is `score` times
       * `now`, the faint bar's length.
       */
      solve: (score: string, target: string, now: string, floor: string) => string
      /**
       * The shape chip, quoting the word it is naming and the shape that word
       * scores as, both as the chip shows them.
       */
      shape: (word: string, shape: string) => string
      /**
       * The menu button: the settings behind it and the two references. The
       * buttons it names should be named as the pause sheet labels them.
       */
      menu: string
      /** The board's decoration switch, named for the rounds that will need it. */
      decor: string
      /** The first shop, before a relic is held: what each kind of card is. */
      shelf: string
      /** Once one is: where it went, and `slots` is how many fit. */
      relics: (slots: string) => string
    }
  }
}
