import type { Color, Growth } from "../../engine"
import { formatNumber as num, pluralizer } from "../format"
import type { Strings } from "./types"

/** Every sentence below that has to agree with a count asks this, not a `=== 1`. */
const plural = pluralizer("en")

/**
 * Lifted out of the object below because one refusal has to read it: telling a
 * player their card cannot go on J is useless without saying which card, and
 * the name is this file's to spell.
 *
 * A modifier's text reads after the letter it sits on: "K scores ×2 mult". A
 * language that cannot put the subject first has to reword these into something
 * that can stand alone, which is why the sentence is here rather than assembled
 * from a fragment at the call site.
 */
const MODIFIER: Strings["modifier"] = {
  chip: { name: "Chip", text: "scores +20 chips" },
  mult: { name: "Mult", text: "scores +8 mult" },
  gold: { name: "Gold", text: "pays $2 every time you play it" },
  wild: { name: "Wild", text: "scores +24 mult on a gray, +14 on a yellow, +4 on a green" },
  lucky: { name: "Lucky", text: "has a 1 in 4 chance of scoring +20 mult" },
  echo: { name: "Echo", text: "scores +60 chips when the word repeats it" },
  anchor: { name: "Anchor", text: "scores +125 chips when it lands green" },
  steel: { name: "Steel", text: "scores ×2 mult" },
  glass: { name: "Glass", text: "scores ×3 mult, and can break when it lands gray" },
}

/**
 * The two currencies a relic can grow in, spelled. English happens to spell them
 * exactly as the union does, which is why this looked like a pointless map for a
 * moment; it is not, and the moment a second language fills it that stops being
 * a coincidence.
 */
const UNIT: Record<Growth["unit"], string> = { chips: "chips", mult: "mult" }

/**
 * The three colors, said in words.
 *
 * They reach a screen in exactly one place, the tile tip's mult line, and
 * everywhere else the color *is* the paint. The engine spells them as its own
 * union, which the tip used to print raw, and that was the last English word in
 * the game arriving from `src/engine`.
 */
const COLOR: Record<Color, string> = { green: "green", yellow: "yellow", gray: "gray" }

/**
 * English, and the reference filling of `Strings`.
 *
 * Every one of these sentences used to live beside the code that implements it,
 * which is the arrangement the content files argue for at length and which is
 * right about everything except this: a `text` field next to an `onTile` is the
 * best place for a description and the worst place for a translation. The
 * numbers stayed behind. What came here is only the prose.
 *
 * So the standing rule when a card's balance moves: the number in the sentence
 * below is a restatement, not a source, and the two have to be moved together.
 * Where the number had a field to read it from, the sentence reads it instead
 * and the problem does not arise; see the etchings.
 */
export const en: Strings = {
  tag: "en",
  endonym: "English",

  relic: {
    green_thumb: { name: "Green Thumb", text: "+8 chips per green tile" },
    scavenger: { name: "Scavenger", text: "+$1 per yellow tile" },
    vowel_hoarder: { name: "Vowel Hoarder", text: "+4 mult per vowel" },
    slow_burn: {
      name: "Slow Burn",
      text: "+5 mult for each guess already made this round",
    },
    consonant_cluster: {
      name: "Consonant Cluster",
      text: "×1.5 mult if the word has 3+ consonants in a row",
    },
    cold_open: { name: "Cold Open", text: "+30 chips on the first guess of a round" },
    bloodhound: { name: "Bloodhound", text: "+6 chips per yellow tile" },
    head_start: { name: "Head Start", text: "+15 mult if the word begins with a vowel" },
    loaded_dice: { name: "Loaded Dice", text: "+0 to +20 mult, rolled fresh every guess" },
    anagrammer: { name: "Anagrammer", text: "×2 mult if no letter repeats" },
    keystone: { name: "Keystone", text: "×3 mult if the middle tile is green" },
    lexicographer: {
      name: "Lexicographer",
      text: "+3 chips for each different letter in your earlier guesses this round",
    },
    sunk_cost: { name: "Sunk Cost", text: "+10 mult per guess you would have left" },
    speedrunner: { name: "Speedrunner", text: "×3 mult when you solve in 3 guesses or fewer" },
    qs_bargain: { name: "Q's Bargain", text: "J, Q, X and Z score triple chips" },
    greedy_grammarian: { name: "Greedy Grammarian", text: "+15 chips per gray tile" },
    doppelganger: { name: "Doppelgänger", text: "Repeated letters score their chips twice" },
    hot_streak: {
      name: "Hot Streak",
      text: "Permanently gains +30 chips each round you clear in 3 guesses or fewer",
    },
    hoarder: {
      name: "The Hoarder",
      text: "Permanently gains +40 chips when you reach the shop with both card slots full",
    },
    masochist: { name: "Masochist", text: "+8 mult per gray tile" },
    chorus: { name: "The Chorus", text: "×3 mult if the word holds three or more vowels" },
    alphabetist: { name: "Alphabetist", text: "×2 mult if your letters are in alphabetical order" },
    vault: { name: "The Vault", text: "+25 chips for each guess already made this round" },
    mint: { name: "The Mint", text: "+3 mult per $5 you hold. You earn no interest." },
    scorched_earth: {
      name: "Scorched Earth",
      text: "+12 mult for each letter broken out of the alphabet",
    },
    snowball: { name: "Snowball", text: "Permanently gains +1 mult for each green tile you play" },
    long_game: { name: "The Long Game", text: "+1 to your solve multiplier" },
    pyromaniac: {
      name: "Pyromaniac",
      text: "+40 mult. Breaks a random letter out of the alphabet each round",
    },
  },

  boss: {
    silence: {
      name: "The Silence",
      text: "Misplaced letters score as absent, and read as absent. You are told only how many.",
    },
    fog: { name: "The Fog", text: "Yellow and gray look identical. They still score differently." },
    tyrant: {
      name: "The Tyrant",
      text: "Every guess must reuse the green letters you have found.",
    },
    miser: { name: "The Miser", text: "Letters you have already used score no chips." },
    clock: { name: "The Clock", text: "Four guesses only." },
    glutton: { name: "The Glutton", text: "Every guess must contain at least two vowels." },
    auditor: { name: "The Auditor", text: "Your solve multiplier is capped at ×2." },
    purist: { name: "The Purist", text: "No letter may appear twice in a guess." },
    drought: { name: "The Drought", text: "Vowels score no chips." },
    mirror: {
      name: "The Mirror",
      text: "Your feedback is shown back to front. It still scores as it fell.",
    },
    famine: { name: "The Famine", text: "Three guesses only." },
    rust: {
      name: "The Rust",
      text: "Letter upgrades score nothing. Letters are worth only what they started as.",
    },
    margin: { name: "The Margin", text: "The first and last letters score no chips." },
    vandal: { name: "The Vandal", text: "Letter modifiers do nothing." },
    plateau: {
      name: "The Plateau",
      text: "Multiplying effects do nothing. Mult may only be added.",
    },
  },

  modifier: MODIFIER,

  consumable: {
    oracle: { name: "The Oracle", text: "Reveal one letter of the answer, in place" },
    hermit: {
      name: "The Hermit",
      text: "Rule a letter out of the answer without spending a guess",
    },
    magician: {
      name: "The Magician",
      text: "Your next guess scores its first gray tile as a yellow. It is worth mult, not a clue.",
    },
    fool: { name: "The Fool", text: "Score your previous guess a second time" },
  },

  pack: {
    alphabet: { name: "Alphabet Pack", text: "Choose one of three letter modifiers" },
    relic: { name: "Relic Pack", text: "Choose one of three relics" },
    category: { name: "Category Pack", text: "Choose one of three word categories to level" },
  },

  category: {
    alphabetical: { name: "Alphabetical", text: "Its letters never go backwards" },
    vowel_heavy: { name: "Vowel Heavy", text: "Three or more vowels" },
    cluster: { name: "Cluster", text: "Three consonants in a row" },
    twinned: { name: "Twinned", text: "Some letter appears twice" },
    distinct: { name: "Distinct", text: "No letter repeats" },
  },

  /**
   * The one table whose numbers come back out of the content file. Etch
   * Consonants is also the only card in the game whose unit goes singular, at
   * +1 chip, which is the cheapest possible reminder that a count in a sentence
   * is a grammatical problem and not a formatting one.
   */
  etching: {
    etch_vowels: {
      name: "Etch Vowels",
      text: (chips) => `A E I O U are worth +${chips} chips`,
    },
    etch_staples: {
      name: "Etch Staples",
      text: (chips) => `L N S T R are worth +${chips} chips`,
    },
    etch_heavy: {
      name: "Etch Heavy",
      text: (chips) => `J Q X Z are worth +${chips} chips`,
    },
    etch_consonants: {
      name: "Etch Consonants",
      text: (chips) =>
        plural(chips, {
          one: `Every consonant is worth +${chips} chip`,
          other: `Every consonant is worth +${chips} chips`,
        }),
    },
  },

  ascension: {
    1: { name: "Hunted", text: "Every guess must use the letters you have found." },
    2: { name: "Once Only", text: "No word twice in the same round." },
    3: { name: "Steeper", text: "Every target is 15% higher." },
    4: { name: "Anchored", text: "Every guess must use the letters you have placed." },
    5: { name: "Tyranny", text: "Letters you have placed must stay where you placed them." },
    6: { name: "Crowded", text: "Four relic slots, not five." },
    7: { name: "Lean Years", text: "Every round pays $1 less." },
    8: { name: "Dead Weight", text: "A round you did not solve pays nothing." },
    9: { name: "No Echoes", text: "No word twice in the whole run." },
    10: {
      name: "Finish It",
      text: "Reaching the target is not enough. You have to solve the word.",
    },
    steeper: {
      name: "Steeper",
      text: (percent, total) => `Targets rise another ${percent}% (×${total} in all).`,
    },
  },

  round: ["Normal Round", "Elite Round", "Boss Round"],

  /**
   * Lowercase and clipped short, because every one of these appears in a toast
   * over the board while the player is mid-thought. They are the shortest text
   * in the game on purpose: a refusal that has to be read twice has failed.
   *
   * Nothing here is punctuated. The board says the rest.
   */
  refusal: {
    not_your_turn: "not your turn",
    not_a_letter: "not a letter",
    letter_broken: ({ letter }) => `${letter.toUpperCase()} is broken`,
    no_room: "no room",
    // "5 letters", which is a count and so a plural in most languages and an
    // agreement in some. English gets away with the bare number.
    wrong_length: ({ length }) => `${length} letters`,
    not_in_word_list: "not in word list",

    must_use: ({ letter }) => `must use ${letter.toUpperCase()}`,
    must_keep: ({ letter, position }) =>
      `must keep ${letter.toUpperCase()} in position ${position}`,
    needs_two_vowels: "needs at least two vowels",
    no_repeated_letters: "no repeated letters",
    already_guessed_round: "already guessed this round",
    already_used_run: "already used this run",

    only_during_round: "only during a round",
    no_such_card: "no such card",
    word_already_revealed: "the whole word is already revealed",
    nothing_to_reveal: "nothing to reveal",
    nothing_to_rule_out: "nothing left to rule out",
    already_prepared: "already prepared",
    no_guess_to_repeat: "no guess to repeat",

    nothing_to_collect: "nothing to collect",
    run_not_won: "the run is not won",

    not_in_shop: "not in the shop",
    sell_only_in_shop: "you can only sell in the shop",
    finish_pack_first: "finish the open pack first",
    place_mod_first: "place the modifier first",
    already_bought: "already bought",
    not_enough_gold: "not enough gold",
    no_such_relic: "no such relic",
    no_relic_slots: "no relic slots free",
    no_card_slots: "no card slots free",
    pack_empty: "nothing left to put in that pack",
    no_pack_open: "no pack is open",
    already_taken: "already taken",
    nothing_to_place: "nothing to place",
    no_letter_for_mod: "no letter left for that",
    // Reads back the card the player is holding, which is the only refusal that
    // has to name anything. Echo is the only modifier that can produce it.
    mod_not_allowed: ({ id, letter }) =>
      `${MODIFIER[id].name} cannot go on ${letter.toUpperCase()}`,

    // Below here is the diagnostic half: every one of these means a lookup that
    // cannot fail did, so they are bugs wearing a refusal's clothes. Kept short
    // and kept legible, and a translator may leave them exactly as they are.
    unknown_card: "unknown card",
    mod_needs_letter: "that one needs a letter first",
    nested_pack: "a pack cannot come out of a pack",
    unknown_letter: "unknown letter",
    unknown_etching: "unknown etching",
    unknown_category: "unknown category",
    unknown_range: "unknown range",
    unknown_modifier: "unknown modifier",
    unknown_pack: "unknown pack",
  },

  event: {
    growth: ({ amount, unit }) => `+${amount} ${UNIT[unit]}`,

    /**
     * Chips are the bare number and mult is the number plus the word, which
     * looks inconsistent written out like this and is not. Chips are the default
     * currency of the board: every tile already floats one, so `+20` over a tile
     * needs no more saying. Mult is the rarer half and the one worth naming.
     *
     * `blocked` is The Plateau, and it says the multiply it stopped rather than
     * saying nothing, because a card that lit up and moved no numbers reads as a
     * bug. `×1` is exactly what happened.
     */
    payout: (paid) => {
      switch (paid.kind) {
        case "chips":
          return `+${paid.amount}`
        case "mult":
          return `+${paid.amount} mult`
        case "times":
          return `×${paid.factor} mult`
        case "blocked":
          return "×1 blocked"
        case "gold":
          return `+$${paid.amount}`
      }
    },

    categoryLevel: (name, level) => `${name} Lv ${level}`,
    modPlaced: (name, letter) => `${name} ${letter.toUpperCase()}`,
    note: (note) => {
      switch (note.card) {
        // Both of these say a letter out loud, and both uppercase it here rather
        // than at the source: the engine works in lowercase throughout, and a
        // language whose alphabet does not case the way English does gets to
        // decide that for itself instead of receiving it already decided.
        case "oracle":
          return `${note.letter.toUpperCase()} is #${note.position}`
        case "hermit":
          return `no ${note.letter.toUpperCase()}`
        case "magician":
          return "next gray becomes yellow"
        case "fool":
          return `+${note.score}`
      }
    },

    // The Silence, and the one place in the game where a zero is said in words:
    // "0 misplaced" over a row of gray reads as a tile count that failed to
    // render, where "none misplaced" reads as the sentence it is. The wording is
    // load-bearing enough that the golden vectors hold it, since they were
    // recorded before this was a count and are compared against it still.
    guessNote: ({ count }) => (count === 0 ? "none misplaced" : `${count} misplaced`),
  },

  ui: {
    units: ["K", "M", "B", "T", "Qa", "Qi", "Sx", "Sp"],

    error: {
      words: (cause) => `Could not load the word lists: ${cause}`,
    },

    common: {
      close: "Close",
      back: "Back",
      play: "Play",
      howToPlay: "How to play",
      codex: "Codex",
      soundOn: "Sound on",
      soundOff: "Sound off",
      ascension: (level) => `Ascension ${level}`,
      percent: (share) => `${share}%`,
      none: "—",
      loading: "Loading words…",
    },

    board: {
      menu: "Menu",
      // Each label names the state the *next* tap lands on, and each tip says
      // what the board is now before it says what the tap will do. Read them as
      // a cycle: a wrong order here shows up as a wrong sentence.
      decor: {
        all: {
          label: "Mark only what you have changed",
          tip: "Every letter shows what it scores.\nTap to mark only what you have changed.",
        },
        minimal: {
          label: "Clear the board",
          tip: "Only modifiers and raised letters are marked.\nTap to clear the board.",
        },
        none: {
          label: "Show every letter's value",
          tip: "Nothing on the board says what a letter is worth.\nTap to show it all again.",
        },
      },
      stage: (stage, total) => `Stage ${stage}/${total}`,
      // "Stage 9/8" is nonsense, and so is any denominator once the run is past
      // the last authored stage. A won run counts up instead of counting down.
      stageEndless: (stage) => `Stage ${stage} ∞`,
      ascensionTag: (level) => `A${level}`,
      target: (target) => `of ${target}`,
      relicTip: (text, detail) => `${text} (${detail})`,
      // The name goes back in for a screen reader, which has no card in view for
      // it to be already on.
      relicLabel: (name, text) => `${name}: ${text}`,
      relicLabelGrown: (name, text, detail) => `${name}: ${text} (${detail})`,
      enter: "ENTER",
      del: "DEL",
      solveFactor: (factor) => `solve ×${factor}`,
      solveFloor: (score) => `→ ${score}`,
      solveFloorClears: (score) => `→ ${score}, clears`,
      shapeLevel: (level) => `Lv ${level}`,
      shapeBonus: (chips, mult) => `+${chips} +${mult} mult`,
      shapesMore: "shapes ›",
      multUnknown: "Color is the multiplier. Guessing is how you find it out.",
      letterBroken: (letter) => `${letter.toUpperCase()} broken`,
    },

    tip: {
      // Zero is its own sentence rather than a plural form. "0 chips" is
      // arithmetic and "no chips" is the answer to what the player asked, and
      // CLDR has nothing to say about the difference: English selects `other` at
      // zero, so a `zero` key here would simply never be read.
      tileChips: (letter, chips) =>
        chips === 0
          ? `${letter.toUpperCase()} · no chips`
          : plural(chips, {
              one: `${letter.toUpperCase()} · +${chips} chip`,
              other: `${letter.toUpperCase()} · +${chips} chips`,
            }),
      keyChips: (letter, chips) =>
        chips === 0
          ? `${letter.toUpperCase()} · no chips`
          : plural(chips, {
              one: `${letter.toUpperCase()} · ${chips} chip`,
              other: `${letter.toUpperCase()} · ${chips} chips`,
            }),
      broken: (letter) => `${letter.toUpperCase()} · broken, no longer typeable`,
      base: (chips) => `${chips} base`,
      etched: (chips) => `+${chips} etched`,
      fromRange: (chips, range, level) => `+${chips} from ${range} Lv ${level}`,
      boss: (name, text) => `${name}: ${text}`,
      color: (color, mult) =>
        mult === 0 ? `${COLOR[color]} · no mult` : `${COLOR[color]} · +${mult} mult`,
      mod: (name, badge) => `${name} · ${badge}`,
      modIdle: (name, text) => `${name} · ${text}`,
      modSilenced: (name, text) => `${name} · ${text} · silenced this round`,
      modQuiet: (name, text) => `${name} · ${text} · nothing this time`,
      relic: (name, badge) => `${name} · ${badge}`,
      // Mult starts the row at 1 and that 1 belongs to no letter, so a column
      // that added none says so rather than claiming a share of it. That is why
      // this is two entries: the second half is absent, not zero.
      share: (chips, total) => `${chips} of ${total} chips · no mult`,
      shareWithMult: (chips, total, mult, multTotal) =>
        `${chips} of ${total} chips · ${mult} of ${multTotal} mult`,
    },

    intro: {
      stage: (stage, total) => `Stage ${stage} of ${total}`,
      stageEndless: (stage) => `Stage ${stage} · endless`,
      scoreAtLeast: "Score at least",
      meta: (guesses, reward) => `${guesses} guesses · reward ${reward}`,
      targets: (factor) => `targets ×${factor}`,
      // Said before the buttons rather than written into them, because the
      // choice is only meaningful to someone who knows what they are turning
      // down, and "tips" alone does not say the board is about to explain itself.
      coachAsk: "First run. The board can talk you through the scoring as you play.",
      coachYes: "Play with tips",
      // Phrased as the whole tutorial rather than as this card: it is the last
      // time the question is asked, so it must not read as "not now".
      coachNo: "Skip the tips",
      play: "Play",
    },

    reward: {
      cleared: "Round cleared",
      answerWas: (word) => `The word was ${word.toUpperCase()}`,
      score: (score, target) => `${score} of ${target}`,
      unusedGuesses: "Unused guesses",
      interest: "Interest",
      total: "Total",
      collect: "Collect",
    },

    shop: {
      title: "Shop",
      sold: "sold",
      sell: (amount) => `sell ${amount}`,
      reroll: (cost) => `Reroll ${cost}`,
      nextRound: "Next round",
      owned: (relics, relicSlots, cards, cardSlots) =>
        `Relics ${relics}/${relicSlots} · Consumables ${cards}/${cardSlots}`,
      // The instruction only when there is something to obey it with: an empty
      // tray under "tap a relic to sell" reads as a control that is broken
      // rather than as one with nothing to act on yet.
      ownedSellable: (relics, relicSlots, cards, cardSlots) =>
        `Relics ${relics}/${relicSlots} · Consumables ${cards}/${cardSlots} · tap a relic to sell`,
      shapesLabel: "Word shapes",
      shapesLevel: (name, level) => `${name} Lv ${level}`,
      shapesNone: "all at level 1",

      tagPack: "Pack",
      tipPack: (picks) =>
        `It deals its cards face up, and you keep ${picks > 1 ? picks : "one"} of them free.`,
      tagRelic: "Relic",
      // What separates it from everything else on the shelf, in one clause: it is
      // never used up and never used at all. The slot count is a number the tray
      // under it is already showing, so it stays out of here.
      tipRelic: "You keep it for the whole run, and it works on its own every round.",
      // "Card" was what this line called itself, and a shelf where every item is
      // drawn as a card had no way to hear that as a kind. The word that says the
      // mechanic is the one the code has used all along: it is consumed.
      tagConsumable: "Consumable",
      tagConsumableFull: "Consumable · slots full",
      tipConsumable: "You use it once, whenever you like, and then it is gone.",
      tagLetter: "Letter",
      tipMod: "It sticks to one letter for the rest of the run.",
      tagRange: "Alphabet",
      tipRange: "It levels a slice of the alphabet, so every letter in it is worth more.",
      // "Shape" rather than "Category", because that is the word the board, the
      // shop's own levels line and the panel behind it all use for this.
      tagShape: "Word shape",
      tipShape: "It levels one shape of word, so every guess of that shape pays more.",
      tagEtching: "Etching",
      tipEtching: "It adds chips to a group of letters for good, and buying it again stacks.",

      // The qualifier sits after the name rather than reading as a verb ("Gold a
      // letter"), and it is what tells this card apart from the pack's aimed one
      // at a glance: "Gold E" is a letter, "Gold · any letter" is a decision.
      modAnyTitle: (name) => `${name} · any letter`,
      modAnyText: (text) => `Choose any letter. It ${text}`,
      modAnyTextOnly: (text, letters) => `Choose any letter. It ${text}, from ${letters}`,
      modTitle: (name, letter) => `${name} ${letter}`,
      modText: (letter, text) => `${letter} ${text}`,
      // Named with its pip, which is the mark the player has been reading off the
      // key all run. "Replaces Steel" asks them to remember what Steel was;
      // "Replaces Steel ×2" says what is coming off the letter.
      swap: (name, pip) => `Replaces ${name} ${pip}`,

      // Named as the level it buys rather than the one you hold, because the gold
      // buys the step and the step is what the price is for.
      rangeTitle: (name, level) => `${name} → Lv ${level}`,
      rangeText: (letters, chips) => `${letters} are worth +${chips} chips per level`,
      levelTitle: (name, level) => `${name} → Lv ${level}`,
      levelText: (name, chips, mult) =>
        `${name} words score +${chips} chips and +${mult} mult per level`,
      fallbackRange: "Range",
      fallbackLevel: "Level",
      fallbackEtching: "Etching",
    },

    pack: {
      choose: (left) => `Choose one of ${left}`,
      choosePicks: (picks, left) => `Choose ${picks} of ${left}`,
      taken: "taken",
      skip: "Skip",
      skipSome: "Take no more",
    },

    place: {
      choose: (text) => `Choose a letter. It ${text}.`,
      onlyOn: (name, letters) => `${name} only goes on ${letters}.`,
      oneEach: "A letter holds one modifier. Tapping one that already has a pip asks first.",
      carrying: (letter) => `${letter.toUpperCase()} is carrying`,
      // "Gone for the rest of the run" rather than "replaced", because replaced
      // is what the player is trying to do and says nothing about the cost.
      loses: (name) => `Putting ${name} here loses it for the rest of the run.`,
      // The two buttons name the two outcomes rather than saying yes and no, so
      // the sentence above does not have to be re-read to work out which is which.
      replace: (name) => `Replace ${name}`,
      keep: (name) => `Keep ${name}`,
    },

    end: {
      won: "Run complete",
      lost: "Run over",
      short: (score, target, by) => `${score} of ${target}, short by ${by}`,
      wonAndWent: (stages) => `Beat stage ${stages} and kept going`,
      reached: (stage, round) => `Reached stage ${stage}, ${round}`,
      endlessNote: (stages) =>
        `Stage ${stages} is where the game stops, not where the run has to. The win is ` +
        "yours either way. Playing on only asks how far this build really goes, and " +
        "the targets keep growing at the same rate the whole way.",
      endless: "Endless mode",
      mainMenu: "Main menu",
      newRun: "New run",
      firstEarned: "Ascension 1 is earned, and the run can be made harder",
      earned: (level, next) => `Ascension ${level} cleared, and ${next} is earned`,
      topOfLadder: (level) => `Ascension ${level} cleared. There is nothing above it.`,
    },

    title: {
      name: "5 WILD",
      tagline: "A word-guessing roguelike",
      runs: (count) => plural(count, { one: `${count} run`, other: `${count} runs` }),
      wins: (count) => plural(count, { one: `${count} win`, other: `${count} wins` }),
      bestStage: (stage) => `best stage ${stage}`,
    },

    stats: {
      title: "Record",
      runs: "runs",
      wins: (count) => plural(count, { one: "win", other: "wins" }),
      bestStage: "best stage",
      guesses: "guesses",
      solved: "solved",
      meanSolve: "avg guess",
      cracked: (count) =>
        plural(count, { one: `${num(count)} word`, other: `${num(count)} words` }),
      // English is the one language here whose participle does not agree, so the
      // count arrives and is ignored. It is still in the signature, because what
      // the three other catalogs need is a shape, and a shape English opts out
      // of is one every translator has to discover for themselves.
      crackedBare: (_count) => "cracked",
      // The one collection the game has, so it is worth a denominator: "273 of
      // 2,300" is a thing to finish, and "273 words cracked" is only a number.
      crackedOf: (_count, pool) => `cracked, of ${pool}`,
      played: (count) => plural(count, { one: `${num(count)} word`, other: `${num(count)} words` }),
      // The same shape as the three above it and deliberately not sharing them:
      // English happens to spell both halves identically, and three of the four
      // catalogs here have to agree a participle with the noun instead.
      playedBare: (_count) => "played",
      playedOf: (_count, pool) => `played, of ${pool}`,
      mostPlayed: "Most played:",
      times: (count) => plural(count, { one: `${num(count)} time`, other: `${num(count)} times` }),
      breakdown: "How the answers go",
      solvedIn: (guesses) => `Solved in ${guesses}`,
      neverFound: "Never found",
      favoriteRelics: "Favorite relics",
      taken: (count) => `taken ${num(count)}×`,
      noStreak: "No answer found yet.",
      streakBest: (now) => `${now} solved in a row, the longest yet, and still going.`,
      streakWithNow: (best, now) => `Longest streak: ${best} in a row, ${now} now.`,
      streak: (best) => `Longest streak: ${best} in a row.`,
    },

    ladder: {
      carrot: "Beat this to unlock ascension 1",
      lower: "Lower the ascension",
      raise: "Raise the ascension",
      locked: "Locked",
      skipTo: (level) => `Skip to ascension ${level}`,
      // Every level plays the rules below it as well, and saying so once here is
      // what stops the stepper reading as a menu of separate modes.
      andBelow: "Every rule below it, too.",
      noRule: "The game as it is written, with nothing extra asked of you.",
      askTitle: (level) => `Skip to ascension ${level}?`,
      ruleLabel: (name) => `${name}:`,
      askAndBelow: "Plus every rule below it.",
      // "Intended" rather than an argument about tuning: it is what the sentence
      // actually means, and the button beside it says "anyway", which is the
      // honest name for the thing being pressed.
      intended: "Winning at the level below is the intended way up.",
      skipAnyway: (level) => `Skip to ${level} anyway`,
    },

    help: {
      title: "How to play",
      lead:
        "Guess the word, the way you already know how: green is the right letter in " +
        "the right place, yellow is the right letter somewhere else.",
      scored: "The difference is that every guess is scored.",
      chipsMult: {
        term: "Chips × Mult",
        text: "Each guess is worth its chips multiplied by its mult.",
      },
      letters: {
        term: "Letters pay chips",
        text:
          "Rare letters pay more. The shop sells two ways to raise them: etchings, which " +
          "add to a kind of letter, and levels on a slice of the alphabet. Every letter " +
          "sits in exactly one slice, and the two stack.",
      },
      colors: {
        term: "Colors pay mult",
        text:
          "Green is worth +3 mult, yellow +1, gray nothing. A guess full of gray is " +
          "worth almost nothing, so a throwaway probe costs you real score.",
      },
      solving: {
        term: "Solving multiplies the round",
        text:
          "Land the word and your whole banked pile for the round, not just the guess " +
          "that solved it, is multiplied by 1 + the guesses you had left. Then the " +
          "round ends immediately, target met or not.",
      },
      farming:
        "That is the game: every guess you spend farming grows the pile, and " +
        "shrinks the multiplier waiting for it.",
      solveLine: {
        term: "So watch the solve line",
        text:
          "Under the board it shows the multiplier a solve would earn right now, and " +
          "what the pile is already worth at it. When it turns green, solving wins " +
          "the round.",
      },
      runHeading: "The run",
      target: {
        term: "Beat the target",
        text: (stages, rounds) =>
          `${stages} stages of ${rounds} rounds. Fall short of a round's target ` +
          "and the run is over. That is the only way to lose.",
      },
      endless: {
        term: "Then keep going, if you dare",
        text: (stages) =>
          `Clearing stage ${stages} wins the run, and you can bank it there or play on into ` +
          "stages nobody balanced. The targets keep growing at the same rate, and dying out " +
          "there does not take the win back.",
      },
      bosses: {
        term: "Bosses",
        text: "Every third round bends a rule. Read it before you play.",
      },
      ascensions: {
        term: "Ascensions",
        text: (authored) =>
          "The difficulty dial on the title screen, and the number worth comparing. The " +
          `first ${authored} levels each add one standing rule: to what you may ` +
          "guess, what a round pays, how high the targets are, how many relics you may keep, " +
          "how many guesses you get. Above that the ladder does not end: every further level raises " +
          "every target again. Winning at one earns the next, and climbing a rung at a time " +
          "is the intended way up, not a lock.",
      },
      money: {
        term: "Money",
        text: (payouts, perGuess, per, cap) =>
          `Rounds pay ${payouts}, plus ${perGuess} per ` +
          `unused guess, plus $1 interest per ${per} you are holding, up to ` +
          `${cap}. Sitting on cash is a strategy.`,
      },
      relics: {
        term: "Relics",
        text: (slots) =>
          `Up to ${slots}, and they fire left to right, so the order you buy them ` +
          "in matters. Tap one to read it.",
      },
      packs: {
        term: "Packs",
        text:
          "One slot sells a choice rather than a card. A pack lays three out and you " +
          "keep one, free. The rest of the shop waits until you have picked or " +
          "walked away, and walking away keeps nothing.",
      },
      mods: {
        term: "Letter mods",
        text:
          "The shop sells modifiers and you choose which letter each one sticks to for " +
          "the rest of the run. Every time you play that letter, it does this. A " +
          "×mult letter multiplies what the word has scored up to where it sits, so it " +
          "is worth more late in a word than early. Packs deal them cheaper, with the " +
          "letter already chosen for you.",
      },
      codexNote:
        "The codex has every relic, boss, word shape and modifier in the game, listed in full.",
      openCodex: "Open the codex",
      gotIt: "Got it",
    },

    shapes: {
      title: "Word shapes",
      scoresAs: (word, shape) => `${word.toUpperCase()} scores as ${shape}`,
      anyWord: "Every guess has a shape.",
      note:
        "A guess scores as the rarest shape it matches, which is the first of these it " +
        "matches. Leveling a shape raises every future guess of that shape. Level 1 " +
        "pays nothing, so a level is what makes a shape worth aiming at.",
      scoring: "scoring",
      alsoMatches: "also matches",
      payNow: (chips, mult) => `now +${chips} chips, +${mult} mult`,
      payPerLevel: (chips, mult) => `+${chips} chips, +${mult} mult per level`,
    },

    codex: {
      title: "Codex",
      lead: "Everything in the game, whether you have met it or not.",
      relics: {
        title: "Relics",
        blurb: (slots) =>
          `Up to ${slots} at once, firing left to right, so the order you buy them ` +
          "in is part of the build.",
      },
      bosses: {
        title: "Bosses",
        blurb:
          "Every third round. Each band is drawn without replacement, so a run never " +
          "meets the same boss twice.",
      },
      ascensions: {
        title: "Ascensions",
        blurb:
          "The run's own difficulty, chosen before it starts and fixed for the whole of it. " +
          "A run at a level plays every rule up to it, and winning at one unlocks the next. " +
          "These are the written ones; past the last of them each level simply raises every " +
          "target by another 8%, and there is no last level.",
      },
      shapes: {
        title: "Word shapes",
        blurb:
          "Every guess scores as exactly one shape: the rarest one it matches, which is " +
          "the first in this list. Leveling a shape raises every future guess of it.",
      },
      mods: {
        title: "Letter modifiers",
        blurb:
          "Stuck to a single letter for the rest of the run. One at a time per letter, " +
          "and the keyboard wears the mark. A ×mult letter multiplies what the word has " +
          "scored up to where it sits, so it is worth more late in a word than early. " +
          "The shop price buys the card and lets you pick the letter; a pack deals it " +
          "for the price beside it, letter already chosen.",
      },
      upgrades: {
        title: "Letter upgrades",
        blurb:
          "Two lines that both add chips to letters, and stack: etchings raise a kind of " +
          "letter, ranges raise a slice of the alphabet. Every letter sits in exactly one " +
          "slice.",
      },
      // One word for this line everywhere it is named, on the shelf's ticket, the
      // shop's tray count and this heading, because a player who bought a
      // Consumable and then reads a codex full of Cards has to work out that they
      // are the same line before they can look anything up.
      consumables: {
        title: "Consumables",
        blurb: (slots) => `Used once, whenever you like. You can hold ${slots}.`,
      },
      packs: {
        title: "Packs",
        blurb:
          "Sold in a slot of their own. A pack lays its cards out and you keep one, free. " +
          "The shop waits until you have chosen or walked away.",
      },
      rarity: {
        common: "common",
        uncommon: "uncommon",
        rare: "rare",
        legendary: "legendary",
      },
      tier: { early: "Early", mid: "Mid", late: "Late" },
      tierBand: (tier, first, last) => `${tier} · stages ${first}–${last}`,
      shapePer: (chips, mult) => `+${chips} / +${mult} per level`,
      modName: (name, pip) => `${name} ${pip}`,
      modText: (text) => `The letter ${text}.`,
      modTextOnly: (text, letters) => `The letter ${text}. Only on ${letters}.`,
      packText: (text) => `${text}.`,
      packTextPicks: (text, picks) => `${text}, and keep ${picks}.`,
    },

    pause: {
      title: "Paused",
      musicOn: "Music on",
      musicOff: "Music off",
      speed: (speed) => `Animation speed ×${speed}`,
      language: "Language",
      wordsNextRun: "Words change when you start a new run.",
      quit: "Quit run",
      resume: "Resume",
    },

    quit: {
      title: "Quit this run?",
      body: (stage, total, round) =>
        `You are on stage ${stage} of ${total}, ${round}. Quitting deletes it. ` +
        "There is no way back to this run.",
      bodyEndless: (stage, round) =>
        `You are on stage ${stage}, ${round}. Quitting deletes it. ` +
        "There is no way back to this run.",
      confirm: "Quit run",
      cancel: "Keep playing",
    },

    coach: {
      chips:
        "Every guess is scored. Type a word, and the line under the board counts what its letters are worth.",
      // The three letters named are the ends and the middle of the table in
      // `content/letters.ts`, so a language whose chip table is retuned has to
      // retune this sentence with it.
      rare: (chips) => `${chips} chips so far. Rare letters pay more: A is 1, K is 5, Z is 10.`,
      mult: "The ? is mult, and only the answer knows it: green +3, yellow +1, gray nothing. Press ENTER to find out.",
      banked: (chips, mult, score, target) =>
        `${chips} × ${mult} = ${score}, banked toward ${target}. Every guess adds to the same pile.`,
      solve: (now, next) =>
        `Solving multiplies the whole pile by ×${now}, not just the guess that lands it, ` +
        `and ends the round. One more guess and it is ×${next}.`,
    },
  },
}
