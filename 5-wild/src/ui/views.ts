import type { GuessRecord, Modifier, Range, Rarity, RunState, ShopItem } from "../engine"
import {
  ASCENSIONS,
  AUTHORED_ASCENSIONS,
  ascensionAt,
  BOSS_TIERS,
  BOSSES,
  baseChips,
  bossesIn,
  CATEGORIES,
  CATEGORY_BY_ID,
  CHIPS_PER_LEVEL,
  CONSUMABLE_SLOTS,
  CONSUMABLES,
  categoryOf,
  difficultyOf,
  draftChips,
  ETCHING_BY_ID,
  ETCHINGS,
  GOLD_PER_UNUSED_GUESS,
  getBoss,
  INTEREST_CAP,
  INTEREST_PER,
  keyboardColors,
  LETTER_CHIPS,
  levelBonus,
  levelOf,
  MAX_ASCENSION,
  MODIFIER_BY_ID,
  MODIFIERS,
  MULT_FOR_COLOR,
  modifierOf,
  PACK_BY_ID,
  PACKS,
  placeableLetters,
  RANGE_BY_ID,
  RANGES,
  RELIC_BY_ID,
  RELIC_SLOTS,
  RELICS,
  ROUND_PAYOUT,
  ROUNDS_PER_STAGE,
  rangeChips,
  rangeLevelOf,
  rangeOf,
  rerollCost,
  rulesFor,
  STAGES,
  sellValue,
  solveBonusFor,
  TIER_STAGES,
} from "../engine"
import type { CoachStep } from "./coach"
import { h } from "./dom"
import { money, formatNumber as num } from "./format"
import type { Lang, Rule, RuleOf, Section, SectionOf } from "./lang"
import {
  ascensionCard,
  bossCard,
  categoryCard,
  consumableCard,
  etchingCard,
  growthBadge,
  guessNote,
  keyRows,
  LANG_FLAGS,
  LANG_NAMES,
  modifierCard,
  packCard,
  payoutBadge,
  relicCard,
  roundName,
  ui,
} from "./lang"
import type { MetaState } from "./meta"
import {
  chosenAscension,
  crackedIn,
  favoriteRelics,
  favoriteWord,
  isLocked,
  meanSolve,
  playedIn,
  roundsPlayed,
  unlocked,
  wordsFound,
} from "./meta"
import type { Speed } from "./speed"

export type Handlers = {
  key: (letter: string) => void
  enter: () => void
  back: () => void
  useConsumable: (index: number) => void
  collect: () => void
  buy: (index: number) => void
  sell: (index: number) => void
  reroll: () => void
  nextRound: () => void
  continueRun: () => void
  pickPack: (index: number) => void
  skipPack: () => void
  /**
   * Where the modifier just bought is going.
   *
   * Two taps rather than one when the letter is already carrying something, and
   * the decision between them is made here rather than in the view: the picker's
   * keys and the physical keyboard both arrive through this, and they must not
   * be able to disagree about which letters ask before they trade.
   */
  placeMod: (letter: string) => void
  /** Back out of a replacement without placing anything. The modifier is still held. */
  cancelPlace: () => void
  newRun: () => void
  /** The difficulty the *next* run starts at. Nothing in flight can hear this. */
  setAscension: (level: number) => void
  play: () => void
  mute: () => void
  toggleMusic: () => void
  /** Step the board down a level of decoration, wrapping back to all of it. */
  cycleDecor: () => void
  /** Step the animations up a rung, wrapping back to the speed they are drawn at. */
  cycleSpeed: () => void
  /** Step the interface to the next language, wrapping back to English. */
  cycleLanguage: () => void
  openMenu: () => void
  openHelp: () => void
  /**
   * Start the first round without the coaching, now and for good. Offered once,
   * on that round's intro card, beside the button that starts it with them.
   */
  skipCoach: () => void
  openCodex: () => void
  /** The shape panel, from the board or from the shop. */
  openShapes: () => void
  /** The long record, from the title screen's one-line version of it. */
  openStats: () => void
  closeOverlay: () => void
  askQuit: () => void
  quit: () => void
  /** The lock on the ladder, opened far enough to read what is behind it. */
  askAscend: (level: number) => void
  /** Past the lock, having read it. */
  ascend: () => void
}

/**
 * Presentation state the engine has no opinion about.
 *
 * `coach` is the odd one out, since it is derived from the run rather than from a
 * setting, and it rides here anyway, because the question it answers is a
 * presentation one: whether this player has been shown the first round yet. The
 * engine cannot know that, the view must not decide it, and putting it here is
 * what keeps `roundView`'s signature the same three arguments every other
 * screen takes.
 */
export type Chrome = {
  muted: boolean
  musicOff: boolean
  decor: Decor
  speed: Speed
  /**
   * Which language is chosen, which the views need for the picker alone: every
   * *sentence* on the screen comes from the catalog in force, which is a module
   * value rather than an argument. See `current` in `./lang`.
   */
  lang: Lang
  /**
   * Whether a run in progress is still being played in the words of the language
   * the player has just switched away from. `App` owns this: it is the only
   * thing that knows which list the run in hand was dealt from.
   */
  wordsDeferred: boolean
  coach: CoachStep | null
  /**
   * Whether the intro card on screen should ask about the tutorial before it
   * starts. Computed by `App` rather than here, because half the answer is a
   * `localStorage` flag that outlives the run and no view may read one.
   */
  coachOffer: boolean
}

/**
 * How much of the scoring game the board draws on itself.
 *
 * Three states rather than two because the one switch was hiding six marks that
 * answer three different questions: what a letter is worth, which modifier it
 * carries, and what just happened while the guess scored. A player who
 * only wanted the third to stop had to give up the first two to get it.
 *
 * The middle state is a rule rather than a dimmer: mark what is *not* ordinary.
 * A letter still worth what it started as says nothing, a letter bought up says
 * so, a modifier says which one it is, and the board holds still while it
 * scores. That is close to the board this game shipped with, before values went
 * on every key, so it is a known-good screen rather than a new one invented for
 * the setting.
 */
export type Decor = "all" | "minimal" | "none"

/*
 * The keyboard rows used to be a constant here and are now `keyRows()` from
 * `./lang`: they are QWERTY, AZERTY or QWERTZ depending on the language, which
 * is a fact about the language exactly as its sentences are, so it comes from
 * the same place they do. Read from there rather than off `chrome` because
 * neither of the two functions that draw a keyboard takes one.
 */

/* -------------------------------------------------------------- shared bits */

/**
 * The menu is the only route to sound, the rules and quitting once a run is
 * under way, so it has to be on every in-run screen rather than parked on a
 * screen the player passes through once.
 */
function menuButton(on: Handlers): HTMLElement {
  return h(
    "button",
    {
      class: "menu-button",
      type: "button",
      "aria-label": ui().board.menu,
      onclick: () => on.openMenu(),
    },
    "☰",
  )
}

/**
 * The letter-values switch, on the board rather than in the menu.
 *
 * It used to live in the pause sheet beside sound and music, and it was the one
 * setting there whose effect could not be seen from the screen it was set on:
 * pressing it turned the pips off behind a sheet covering them, and the player
 * had to close the sheet to find out what they had done. Here the board changes
 * under the thumb, which is the entire feedback the switch needs.
 *
 * It sits beside the chips × mult readout rather than up in the HUD, because
 * that is where the thing it governs is being totted up. The numbers on the
 * keys feed that line, and the switch belongs at the bottom of the screen with
 * them and within reach of the thumb already on the keyboard.
 *
 * The face is the setting rather than a label for it: a letter carrying its
 * value, the same letter carrying only a mark, or the letter on its own. So a
 * button two characters wide says what it does without spending a word on it.
 * The digit is drawn the way an ordinary letter's value is drawn rather than a
 * bought-up one's, because that is what most of the keyboard looks like, and it
 * is what makes the pair read as a before and after rather than as an "A" and a
 * "1". The middle face is honest for the same reason: an ordinary letter shows
 * no value under `minimal`, and a dot is what a marked one would still show.
 *
 * A third state costs the switch its best property: you could not get a flip
 * wrong, and you can get a cycle wrong, because nothing about a button says
 * which way it goes round. Two things pay for it. The order only ever takes
 * decoration away, so a mis-tap is always one step further down the ramp rather
 * than somewhere unexpected; and every state is one tap from the next, so the
 * way back is the same gesture that got here. What it buys is that the three
 * questions the board answers stopped being one lump: a player who wants the
 * scoring to stop shouting mid-guess no longer has to also forget which letter
 * they made Gold.
 *
 * No `aria-pressed`: it is a two-valued attribute and this is three, and a
 * screen reader told "not pressed" on the middle state would be told something
 * false. The label carries the whole thing instead: what the board is now and
 * what the tap will do. That is what the tip says to everyone else.
 */
function decorToggle(on: Handlers, chrome: Chrome): HTMLElement {
  const face = ui().board.decor[chrome.decor]
  const mark = DECOR_MARK[chrome.decor]
  return h(
    "button",
    {
      class: `decor-toggle ${chrome.decor === "none" ? "bare" : ""}`,
      type: "button",
      // Pressing it rebuilds the screen it is standing on, so without a name to
      // be found again by it would drop focus on every press and a keyboard
      // player would have to tab back across the board to reach it a second
      // time. Cycling makes that worse than it sounds: three states is up to two
      // presses to land on the one you wanted. See `holdFocus`.
      "data-focus": "decor",
      "aria-label": face.label,
      "data-tip": face.tip,
      onclick: () => on.cycleDecor(),
    },
    h("span", {}, "A"),
    mark ? h("span", { class: mark.class }, mark.glyph) : null,
  )
}

/**
 * What the switch shows in each state. What it *says* is in the catalog, beside
 * every other sentence in the game; the two halves are indexed by the same key,
 * so a state added here without a face there is a compile error.
 *
 * The mark is described rather than built: a module-level `h()` would be one
 * node handed to every render, and since appending a node moves it, the table
 * would be quietly mutable shared state in a file whose whole contract is that a
 * view builds from scratch.
 */
const DECOR_MARK: Record<Decor, { class: string; glyph: string } | null> = {
  all: { class: "decor-toggle-value", glyph: "1" },
  minimal: { class: "decor-toggle-mark", glyph: "•" },
  none: null,
}

/**
 * The cycle, in one place because two things depend on the order: the switch
 * that steps through it and the tip that names where the next tap goes.
 *
 * It only ever takes decoration away until there is none, then restores all of
 * it. The other candidate, walking back up the way it came, reads better as a
 * dimmer but makes the same tap mean "less" or "more" depending on history,
 * which is exactly the thing a player cannot see on the face of a button.
 */
export const NEXT_DECOR: Record<Decor, Decor> = {
  all: "minimal",
  minimal: "none",
  none: "all",
}

function hud(state: RunState, on: Handlers): HTMLElement {
  const round = state.round
  const board = ui().board
  return h(
    "header",
    { class: "hud" },
    h(
      "div",
      { class: "hud-round" },
      h("div", { class: "round-name" }, roundName(state.roundIndex)),
      h(
        "div",
        { class: "stage" },
        state.won ? board.stageEndless(state.stage) : board.stage(state.stage, STAGES),
        // The terms the whole run is being played under, in the space of two
        // characters. It shares its color with the boss banner because it is
        // the same kind of fact, something bending what a guess may be, and
        // the rules it stands for are named in full on every intro card.
        state.ascension
          ? h("span", { class: "stage-asc" }, board.ascensionTag(state.ascension))
          : null,
      ),
    ),
    h(
      "div",
      { class: `hud-score ${round.score >= round.target ? "met" : ""}` },
      h("div", { class: "score" }, num(round.score)),
      h("div", { class: "target" }, board.target(num(round.target))),
      // The same fact as the two numbers above it, in the form a glance can take
      // in. The scoring animation drives it frame by frame, so it fills as the
      // total climbs rather than jumping to the answer.
      h(
        "div",
        { class: "meter" },
        h("div", { class: "meter-fill", style: `--fill:${meterFill(round.score, round.target)}` }),
      ),
    ),
    h("div", { class: "hud-gold" }, money(state.gold)),
    menuButton(on),
  )
}

/** Shared with the animation controller, so the bar and the number agree. */
export const meterFill = (score: number, target: number): number =>
  target > 0 ? Math.min(1, score / target) : 1

function relicRow(state: RunState): HTMLElement {
  // The tray draws as many seats as the run actually has, so ascension 6 reads
  // as four slots rather than as a fifth that silently refuses every purchase.
  const slots = Array.from({ length: difficultyOf(state).relicSlots }, (_, slot) => {
    const instance = state.relics[slot]
    if (!instance) return h("div", { class: "relic empty" })
    const relic = RELIC_BY_ID.get(instance.id)
    if (!relic) return h("div", { class: "relic empty" })
    // What a scaling relic has grown to. A card whose value moves and does not
    // say so is a card the player cannot plan around, so it goes on the face
    // rather than only in the tip.
    const growth = relic.growth?.(instance)
    const detail = growth ? growthBadge(growth) : null
    const card = relicCard(instance.id)
    // A card to read, not a control to press. It answered a tap with its own
    // text in a toast, which is a message across the board for a thumb that only
    // brushed the tray on its way to the keyboard, and which said what the tip
    // beside the card says anyway. The tip is the better half of the two,
    // since it arrives next to the thing it is about instead of over the round.
    //
    // So no handler, and a `tabindex` rather than a button: a button that does
    // nothing when pressed offers the keyboard a press worth making and then
    // takes it back. What is left is a stop on the tab order, and the tip shows
    // itself on focus. See `bindTips`.
    return h(
      "div",
      {
        class: `relic rarity-${relic.rarity}`,
        "data-slot": slot,
        // Read by the hover tip. It lives on the card rather than in a nested
        // element because the tray clips its own children. The panel that
        // shows this has to be built outside it, and so cannot inherit either.
        // The name is left out: it is already on the card the tip points at.
        "data-tip": detail ? ui().board.relicTip(card.text, detail) : card.text,
        "data-rarity": relic.rarity,
        tabindex: 0,
        // The tip is drawn rather than spoken, so what it holds is said again
        // here for a reader that will never see the panel. The name goes back in,
        // because a screen reader has no card in view for it to be already on.
        "aria-label": detail
          ? ui().board.relicLabelGrown(card.name, card.text, detail)
          : ui().board.relicLabel(card.name, card.text),
      },
      h("span", { class: "relic-name" }, card.name),
      detail ? h("span", { class: "relic-detail" }, detail) : null,
    )
  })
  return h("div", { class: "relics" }, ...slots)
}

function consumableRow(state: RunState, on: Handlers): HTMLElement | null {
  if (state.consumables.length === 0) return null
  const cards = state.consumables.map((instance, index) => {
    const card = consumableCard(instance.id)
    return h(
      "button",
      {
        class: "consumable",
        type: "button",
        onclick: () => on.useConsumable(index),
      },
      h("span", { class: "consumable-name" }, card.name),
      h("span", { class: "consumable-text" }, card.text),
    )
  })
  return h("div", { class: "consumables" }, ...cards)
}

/* --------------------------------------------------------------- the round */

function grid(state: RunState, coach: CoachStep | null): HTMLElement {
  const round = state.round
  const width = round.answer.length
  const active = round.guesses.length
  const modsOff = getBoss(round.bossId)?.noModifiers ?? false

  const rows = Array.from({ length: round.maxGuesses }, (_, row) => {
    const played = round.guesses[row]

    const tiles = Array.from({ length: width }, (_, column) => {
      if (played) {
        const tile = played.tiles[column]
        // Marked on played tiles only. The draft row is patched in place rather
        // than rebuilt (see `patchDraft`), and anything drawn there would have
        // to be reproduced by that patch to survive the next keystroke.
        return h(
          "div",
          {
            class: `tile ${tile?.shown ?? "gray"}`,
            "data-tile": column,
            // The dot says "a modifier fired here", so under The Vandal, where
            // none did, there is nothing for it to say. The key keeps its pip;
            // the row is a record of what happened.
            "data-mod": tile && !modsOff ? modifierOf(state, tile.letter)?.id : undefined,
            // And the arithmetic behind the dot, on the same terms the keys
            // offer it: press and hold, or hover. The mark is the reminder, the
            // panel is the explanation. See `tileTip`. Held back until the tile
            // has turned over; `App.tipHost` is where that happens.
            "data-tip": tileTip(state, played, column),
          },
          (tile?.letter ?? "").toUpperCase(),
        )
      }

      if (row === active && !round.done) {
        const typed = round.draft[column]
        if (typed) {
          // Only the tile at the end of the draft lands. The board is rebuilt on
          // every keystroke, so animating `.filled` would replay the whole word
          // each time a letter is added to it.
          const landed = column === round.draft.length - 1
          return h("div", { class: `tile filled ${landed ? "land" : ""}` }, typed.toUpperCase())
        }
        // The Oracle's reveals sit in place as ghosts, so the hint is spatial
        // rather than a line of text the player has to hold in their head.
        const revealed = round.revealed[column]
        if (revealed) return h("div", { class: "tile ghost" }, revealed.toUpperCase())
      }

      return h("div", { class: "tile" })
    })

    // The boss's line about this row, laid over it rather than beside it. There
    // is no "beside": the board is `min(100%, …)` of the wrap, so on a portrait
    // phone it is as often width-bound as height-bound and a gutter that exists
    // on one device is clipped on the next. Over the tiles' bottom edge always
    // has room, and under The Silence the row it covers is grays and greens.
    const note = played?.note
      ? [
          h(
            "span",
            {
              class: "row-note",
              "data-tip": state.round.bossId ? bossCard(state.round.bossId).text : undefined,
            },
            guessNote(played.note),
          ),
        ]
      : []

    return h("div", { class: "row", "data-row": row }, ...tiles, ...note)
  })

  // The board's shape varies (The Clock takes two rows away), so its
  // proportions are data, not a constant the stylesheet can hard-code.
  return h(
    "div",
    { class: "grid-wrap" },
    h("div", { class: "grid", style: `--rows:${round.maxGuesses};--cols:${width}` }, ...rows),
    // The coaching card, laid over the board's unplayed rows. See `coachSlot`
    // for why this is the one place on the round screen with room for it.
    coachSlot(coach),
  )
}

/**
 * Where the first round explains itself.
 *
 * Inside the board rather than anywhere else on the screen, because there is
 * nowhere else: measured at 390×844 the round screen runs board 115–567,
 * category 573–590, readout 596–630, solve hint 636–656, keyboard 662–836.
 * Between the board's bottom edge and the keyboard there are twenty pixels of
 * gap and three lines of numbers, and the card is about those numbers, so putting
 * it there would mean covering the thing it is pointing at. The strip above the
 * keyboard is already spoken for by the toast, which is where a refusal lands.
 *
 * The board is the opposite: at the moment each beat fires, the rows below the
 * one being typed are empty by definition, so the card is laid over blank
 * squares and nothing the player needs is hidden. It costs no layout either. It
 * is absolutely positioned inside `.grid-wrap`, so the board does not resize
 * when the card appears, which on this screen would be the board *moving*.
 *
 * The slot is always in the document, empty or not, for `fillCategory`'s
 * reason: the draft row is patched rather than re-rendered, the card's text
 * changes with the letters being typed, and a node that came and went would
 * leave the patch with nowhere to write.
 */
function coachSlot(coach: CoachStep | null): HTMLElement {
  const slot = h("div", { class: "coach-slot" })
  fillCoach(slot, coach)
  return slot
}

/**
 * Refill the coaching card. Called on every keystroke, like `fillCategory` and
 * `fillReadout`, because two of the five beats are about the word being typed,
 * and one of them quotes its running chip count. A card a keystroke behind the
 * number it is pointing at would be teaching the wrong lesson.
 */
export function fillCoach(slot: Element, coach: CoachStep | null): void {
  slot.replaceChildren()
  if (!coach) return
  // No button. The card is a sentence about the board, and the one decision it
  // ever asked for was moved to the intro card that precedes it; see `coachAsks`.
  slot.append(
    h(
      "div",
      { class: "coach", "data-step": coach.id },
      h("p", { class: "coach-text" }, coach.text),
    ),
  )
}

/**
 * Where a letter's chips came from, or null when it is still worth what it
 * started as and there is nothing to break down.
 *
 * Shared by the two tips rather than written twice, because they are answering
 * the same question about the same letter from opposite ends of the round. The
 * key asks before the guess, the played tile asks after it, and two copies of
 * this sum would eventually disagree about a purchase.
 */
function chipBreakdown(state: RunState, letter: string): string | null {
  const etch = state.letters[letter]?.etch ?? 0
  const fromRange = rangeChips(state, letter)
  if (etch === 0 && fromRange === 0) return null

  const tip = ui().tip
  const range = rangeOf(letter)
  const parts = [tip.base(LETTER_CHIPS[letter] ?? 0)]
  if (etch > 0) parts.push(tip.etched(etch))
  if (fromRange > 0 && range) {
    parts.push(tip.fromRange(fromRange, range.name, rangeLevelOf(state, range.id)))
  }
  return parts.join(" ")
}

/**
 * How one letter of a played row came to be worth what it was.
 *
 * The row's arithmetic is on screen for about a second. The tile floats what it
 * paid as it turns over, a modifier throws its label across the board, and then
 * the numbers are gone and the board is a record of colors. Everything the
 * player might want to check afterwards happens during the one animation they
 * cannot replay: which letter carried the guess, whether the Lucky landed,
 * whether the boss was eating the etchings. This is where those numbers go on
 * being readable, through the panel and the gesture the keys already use.
 *
 * It prices the color off `color`, not `shown`, and so tells a fogged gray that
 * it was really a yellow. That is a decision and not an oversight, and it was
 * made the other way first: the tip quoted `shown` exactly as `tileGain` does
 * while the tile turns, on the reasoning that a panel undoing The Fog sells the
 * boss's card for the price of a hover.
 *
 * Two things settled it the other way. The Fog is punishing enough played
 * straight. It is the one boss that attacks deduction itself, and a round of it
 * asks the player to guess blind while the score demands they guess well, so
 * the hover is a cost paid for the answer rather than a hole in the design: it
 * is one gesture per tile, after the row is spent, and it never comes to you.
 * And the round was already leaking. `readout` moves to `event.mult` as each
 * tile lands, which is the true running mult, so a gray that jumps the
 * multiplier by one has announced itself to anyone watching the number. What
 * this panel changes is who catches it: before, the attentive; now, whoever
 * asks. Making a boss's tell depend on staring at the right corner of the screen
 * during a one-second animation was never the difficulty that was wanted.
 *
 * The rest of the panel follows from the same rule, which is that the tip is the
 * row's own account of itself. The modifier's line is the label it gave at the
 * time. The relics that paid are each named, in the order they fired.
 *
 * Only the relics that fire per tile. The rest of the tray works on the finished
 * row and is named in the readout under the board, and listing a ×3 that priced
 * the whole guess under one of its five letters would be answering the question
 * wrongly rather than at length.
 */
export function tileTip(state: RunState, guess: GuessRecord, index: number): string | undefined {
  const tile = guess.tiles[index]
  const paid = guess.paid?.[index]
  // A row played before guesses started keeping their arithmetic: a save
  // carried across the change, and only until the round ends. It gets no tip at
  // all rather than a plausible reconstruction: `baseChips` would answer for
  // most rounds and lie under The Miser and The Rust, and a tip that is right
  // except under the bosses is worst exactly where it is most wanted.
  if (!tile || !paid) return undefined

  const tip = ui().tip
  const lines = [tip.tileChips(tile.letter, paid.base)]

  const breakdown = chipBreakdown(state, tile.letter)
  if (breakdown) lines.push(breakdown)

  // Named only when it actually moved this tile, which, unlike the key's tip,
  // this can ask about honestly: a played tile knows which column it landed in,
  // so The Margin is named on the first and last letters and stays quiet in the
  // middle three, where it did nothing.
  const boss = getBoss(state.round.bossId)
  if (boss && paid.base !== baseChips(state, tile.letter)) {
    const card = bossCard(boss.id)
    lines.push(tip.boss(card.name, card.text))
  }

  // The color that scored, which under The Fog and The Mirror is not the color
  // on the tile. Deliberate (see above), and it is also the only reading that
  // squares with the line at the bottom: the mult share is measured off what the
  // row actually gained, and a panel saying "gray · no mult" over "1 of 12 mult"
  // would be caught contradicting itself by the same player it was protecting.
  lines.push(tip.color(tile.color, MULT_FOR_COLOR[tile.color]))

  // The letter's modifier as it is now, which is as it was: the shop refuses to
  // be left with a modifier still in hand, so a round begins with every letter
  // settled and nothing inside one can move a modifier from a letter to another.
  const mod = modifierOf(state, tile.letter)
  if (mod) {
    const card = modifierCard(mod.id)
    lines.push(
      paid.mod
        ? tip.mod(card.name, payoutBadge(paid.mod))
        : boss?.noModifiers
          ? tip.modSilenced(card.name, card.text)
          : // The third silence, and the only one the player cannot work out from
            // anywhere else: the card fired and had nothing to say. Lucky rolled
            // and lost, Anchor wanted a green, Echo wanted the letter twice.
            tip.modQuiet(card.name, card.text),
    )
  }

  // In the order they fired, which is slot order, which is the order the tray
  // reads left to right, so a player checking a line against the card that
  // caused it is looking along the row the cards are already in.
  //
  // A relic that was asked and declined is not mentioned, unlike the letter's own
  // modifier above. The asymmetry is the count: one card is stuck to this letter
  // and its silence is about this letter, where five are asked about every tile
  // on the board and most of them want something it is not. "Green Thumb ·
  // nothing this time" under all thirty tiles of a lost round is noise wearing
  // the costume of an explanation.
  for (const fired of paid.relics ?? []) {
    if (RELIC_BY_ID.has(fired.id))
      lines.push(tip.relic(relicCard(fired.id).name, payoutBadge(fired.label)))
  }

  // What this letter put in, against what the row came to. The two totals are
  // still here as the right-hand halves, so the line that used to read
  // the same on all five tiles now reads differently on each, which is the only
  // thing a per-tile panel is for.
  //
  // Shares of the two numbers rather than a share of the score, though
  // `chips × mult` would divide exactly and read as points. The reason is what it
  // would say about a green: a 1-chip letter that tripled the row would be
  // credited with 7 of 189 and look like the least valuable thing on the board,
  // when the mult it added is most of why the row scored at all. The game is
  // played on both numbers and the tip prices both.
  lines.push(
    paid.mult === 0
      ? tip.share(num(paid.chips), num(guess.chips))
      : tip.shareWithMult(num(paid.chips), num(guess.chips), num(paid.mult), num(guess.mult)),
  )
  return lines.join("\n")
}

/**
 * Everything acting on one letter, in a sentence or four.
 *
 * The key already carries two marks: what the letter is worth and a pip for the
 * modifier. Marks are a reminder, not an explanation. A player who has
 * bought an etching, leveled a range and dropped a Lucky on the same letter is
 * looking at `4` and `?` and has no way to find out where either the four or the
 * question mark came from without leaving the round.
 *
 * The headline is the boss-adjusted figure rather than the raw one, because the
 * question is what this letter pays *now*: under The Rust every upgraded letter
 * on the board is quietly worth less than its pip claims, and this is where that
 * gets said. The breakdown below it is the honest arithmetic, and it is left off
 * entirely when there is nothing to break down.
 */
function letterTip(state: RunState, letter: string): string {
  const tip = ui().tip
  if (state.letters[letter]?.destroyed) return tip.broken(letter)

  const boss = getBoss(state.round.bossId)
  // `draftChips` prices a one-letter draft, which is to say the first column.
  // Fine for a boss that reads the letter, wrong for one that reads the column.
  // Under The Margin every key would announce "no chips", which is true of the
  // column it was asked about and false of the letter. So a positional boss gets
  // the letter's own value in the headline and says the rest in its own words.
  const now = boss?.positional ? baseChips(state, letter) : draftChips(state, letter)

  const lines = [tip.keyChips(letter, now)]

  const breakdown = chipBreakdown(state, letter)
  if (breakdown) lines.push(breakdown)

  // Only when the boss actually moved this letter. Naming a boss that is not
  // touching it would make every key look cursed. A positional one is always
  // named, since the headline above it deliberately stopped accounting for it.
  if (boss && (boss.positional || now !== baseChips(state, letter))) {
    const card = bossCard(boss.id)
    lines.push(tip.boss(card.name, card.text))
  }

  const mod = modifierOf(state, letter)
  // Under The Vandal the modifier is still bought, still placed and still worth
  // reading. It just will not fire this round, and the tip is the one place
  // that can say which of those two things is true.
  if (mod) {
    const card = modifierCard(mod.id)
    lines.push(
      boss?.noModifiers ? tip.modSilenced(card.name, card.text) : tip.modIdle(card.name, card.text),
    )
  }
  return lines.join("\n")
}

function keyboard(state: RunState, on: Handlers): HTMLElement {
  const colors = keyboardColors(state.round.guesses)
  const eliminated = new Set(state.round.eliminated)
  // The Vandal. The pip stays, since the modifier has not gone anywhere, and hiding
  // it would make the round look like it had eaten the purchase, but it grays
  // out, which is the same thing a broken key already does to it.
  const modsOff = getBoss(state.round.bossId)?.noModifiers ?? false

  const key = (letter: string) => {
    const destroyed = state.letters[letter]?.destroyed ?? false
    // Both upgrade lines in one number, because the key is answering "what is
    // this letter worth" and a player choosing a letter does not care which
    // purchase paid for it. Etchings and range levels crosscut, so most upgraded
    // keys are carrying some of each.
    const bought = (state.letters[letter]?.etch ?? 0) + rangeChips(state, letter)
    // And the whole figure, on every key rather than only the bought-up ones.
    //
    // The pip used to be a `+2` that appeared when an etching or a range level
    // moved a letter, which answers "what changed". The question a player is
    // actually holding while choosing a letter is "what is this one worth", and
    // the twenty-odd letters nobody has spent on answered that with silence.
    // The chip table is rarity-inverse, so Q is ten chips and E is one before
    // anybody touches either, which is the most useful comparison on the board
    // and was the one thing the board would not say. `bought` survives only to
    // report whether the figure was paid for, which the stylesheet draws as the
    // chips color against the off-white a letter still worth what it started as
    // gets. The ring that used to say it is gone, because the ring is how a
    // modifier says which modifier it is.
    //
    // Deliberately the raw value rather than the boss-adjusted one `letterTip`
    // leads with. A boss that bends chips does it per column (The Margin) or by
    // what has already been spent (The Miser), and one number sitting on a key
    // cannot say either without lying about the other; the tip has room for the
    // sentence and prints it.
    const value = baseChips(state, letter)
    const color = eliminated.has(letter) ? "gray" : colors.get(letter)
    // A modifier is bought once and paid off over the rest of the run, so the
    // key it lives on is the only place a player can be reminded it is there,
    // at the moment they are choosing whether to spend a letter on this guess.
    const mod = modifierOf(state, letter)
    return h(
      "button",
      {
        class: ["key", color ?? "", destroyed ? "broken" : "", bought > 0 ? "etched" : ""]
          .filter(Boolean)
          .join(" "),
        "data-mod": mod?.id,
        "data-tip": letterTip(state, letter),
        type: "button",
        disabled: destroyed,
        onclick: () => on.key(letter),
      },
      letter.toUpperCase(),
      h("span", { class: "value-pip" }, `${value}`),
      mod ? h("span", { class: `mod-pip${modsOff ? " silenced" : ""}` }, mod.pip) : null,
    )
  }

  return h(
    "div",
    { class: "keyboard" },
    ...keyRows().map((row, index) =>
      h(
        "div",
        { class: "key-row" },
        index === 2 &&
          h(
            "button",
            { class: "key wide", type: "button", onclick: () => on.enter() },
            ui().board.enter,
          ),
        ...[...row].map(key),
        index === 2 &&
          h(
            "button",
            { class: "key wide", type: "button", onclick: () => on.back() },
            ui().board.del,
          ),
      ),
    ),
  )
}

/**
 * What solving on this guess is worth, right now.
 *
 * The solve bonus multiplies everything banked this round, so the decision the
 * whole game turns on, cash out or farm another guess, is arithmetic the
 * player would otherwise have to do in their head, against a multiplier that
 * shrinks every time they guess. Showing it is the difference between a
 * gamble and a choice.
 *
 * The figure is a floor, not a prediction: it is what the pile is already worth
 * multiplied, before the solving guess adds its own chips. Solving can only beat
 * it, never miss it, which is what makes it safe to act on.
 *
 * The factor comes from the engine rather than from `maxGuesses` arithmetic here,
 * so The Long Game and The Auditor move this line as well as the score.
 *
 * Returns the box whether or not it has anything to say, for the reason
 * `categorySlot` does, and this one was worse. The line goes quiet the instant
 * the round is `done`, and `submit` keeps the board on screen through the whole
 * reveal after that (`render("round")`), so the 20px it was holding came out of
 * the grid on the guess that ended the round. Measured at 390×640, where the
 * board is bound by height: the grid went 275px to 301px and every tile 41.7px
 * to 46.1px, mid-flip, on the one guess the player is watching hardest. The
 * board is centered, so on a taller screen it slid instead of growing; neither is
 * something the round should do as it ends.
 */
function solveHint(state: RunState): HTMLElement {
  const round = state.round
  const factor = solveBonusFor(state, round.maxGuesses - round.guesses.length - 1)
  if (round.done || round.guesses.length >= round.maxGuesses || factor < 1) {
    return h("div", { class: "solve-hint" })
  }

  const board = ui().board
  const floor = Math.round(round.score * factor)
  const clears = floor >= round.target
  return h(
    "div",
    { class: `solve-hint ${clears ? "clears" : ""}` },
    h("span", { class: "solve-factor" }, board.solveFactor(factor)),
    round.score > 0 &&
      h(
        "span",
        { class: "solve-floor" },
        clears ? board.solveFloorClears(num(floor)) : board.solveFloor(num(floor)),
      ),
  )
}

/**
 * What shape of word is on the board, and what that shape is currently worth.
 *
 * Balatro names the hand you have selected before you play it, and that label is
 * how a player learns the hand list without being taught it. This is the same
 * job: a category at level one pays nothing, so without this line there would be
 * no way to discover the system exists until after buying into it.
 *
 * Reads the draft once it is a full word, and otherwise the last guess, the
 * same rule the readout beside it follows, so the two never disagree about which
 * word they are describing.
 */
export function wordInPlay(state: RunState): string {
  const round = state.round
  const last = round.guesses[round.guesses.length - 1]
  const word = round.draft.length === round.answer.length ? round.draft : (last?.word ?? "")
  return word.length === round.answer.length ? word : ""
}

/**
 * Where the shape of the word in play is named.
 *
 * The slot is always in the document, empty or not, because the draft row is
 * patched in place rather than re-rendered (see `patchDraft`), and this line
 * has to keep up with it. A line that appeared and vanished with the word would
 * mean the patch had to know where to reinsert it; an empty div costs nothing
 * and gives it somewhere to write.
 */
function categorySlot(state: RunState, on: Handlers): HTMLElement {
  const slot = h("div", { class: "category-slot" })
  fillCategory(slot, state, on)
  return slot
}

/**
 * Refill the slot from the word now in play. Called on every keystroke, so it
 * does the least it can: nothing at all until the draft is a whole word, which
 * is the first moment there is a shape to name.
 */
export function fillCategory(slot: Element, state: RunState, on: Handlers): void {
  slot.replaceChildren()
  const word = wordInPlay(state)
  if (!word) return

  const board = ui().board
  const category = categoryOf(word)
  const bonus = levelBonus(state, category)
  // A button rather than a div, because this line is the only place the shape
  // system announces itself during a round, and a player who wants to know what
  // the other four shapes are has nowhere else to press.
  slot.append(
    h(
      "button",
      { class: "category", type: "button", onclick: () => on.openShapes() },
      h("span", { class: "category-name" }, categoryCard(category.id).name),
      h("span", { class: "category-level" }, board.shapeLevel(bonus.level)),
      bonus.chips > 0 &&
        h("span", { class: "category-bonus" }, board.shapeBonus(bonus.chips, bonus.mult)),
      h("span", { class: "category-more" }, board.shapesMore),
    ),
  )
}

/**
 * The chips × mult line, which answers a different question while a word is
 * being typed than it does once one has been played.
 *
 * Between guesses it holds the last guess, so the number the player just earned
 * is still on screen while they think. From the first letter typed it switches
 * to the word being built, and that is the whole point of it: the choice of
 * which word to spend a guess on is partly a scoring choice, and a board that
 * only reveals the chips *after* the guess is spent makes that choice round.
 *
 * Only the chips half moves. Mult comes entirely from color, and color is the
 * thing the player is trying to find out, so there is genuinely nothing
 * truthful to put there, hence the placeholder rather than a 1 or a stale
 * figure, either of which would read as a promise the board cannot keep. Saying
 * "unknown" out loud also teaches the rule the readout is built on: chips are
 * the letters you chose, mult is what the answer thinks of them.
 */
function readoutSlot(state: RunState): HTMLElement {
  const el = h("div", { class: "readout" })
  fillReadout(el, state)
  return el
}

/**
 * Refill the readout from the word in play. Called on every keystroke, like
 * `fillCategory`, and split out for the same reason: the draft row is patched
 * rather than re-rendered, and a readout that only caught up on the next full
 * render would lag the letters it is counting.
 */
export function fillReadout(el: Element, state: RunState): void {
  const round = state.round
  const drafting = !round.done && round.draft.length > 0
  const last = round.guesses[round.guesses.length - 1]
  el.classList.toggle("drafting", drafting)
  el.replaceChildren(
    h(
      "span",
      { class: "chips" },
      num(drafting ? draftChips(state, round.draft) : (last?.chips ?? 0)),
    ),
    h("span", { class: "times" }, "×"),
    drafting
      ? h(
          "span",
          {
            class: "mult pending",
            "data-tip": ui().board.multUnknown,
          },
          "?",
        )
      : h("span", { class: "mult" }, num(last?.mult ?? 1)),
  )
}

export function roundView(state: RunState, on: Handlers, chrome: Chrome): HTMLElement {
  const boss = getBoss(state.round.bossId)
  return h(
    "div",
    { class: "screen round-screen" },
    hud(state, on),
    boss &&
      h(
        "div",
        { class: "boss" },
        h("strong", {}, bossCard(boss.id).name),
        h("span", {}, ` ${bossCard(boss.id).text}`),
      ),
    relicRow(state),
    consumableRow(state, on),
    grid(state, chrome.coach),
    categorySlot(state, on),
    // The toggle is the readout's sibling and not its child on purpose:
    // `fillReadout` calls `replaceChildren` on every keystroke, so a button
    // inside that element would be rebuilt, and lose a press mid-tap, five
    // times a word.
    h("div", { class: "readout-row" }, readoutSlot(state), decorToggle(on, chrome)),
    solveHint(state),
    h("div", { class: "relic-tip" }),
    h("div", { class: "toast" }),
    keyboard(state, on),
  )
}

/* --------------------------------------------------------- the round intro */

/**
 * The beat between the shop and the board. It exists for pacing. The player
 * arrives at a round having decided what to buy, and this is where they read
 * what they are walking into before the keyboard demands anything of them.
 */
export function introView(state: RunState, on: Handlers, chrome: Chrome): HTMLElement {
  const copy = ui().intro
  const boss = getBoss(state.round.bossId)
  const name = roundName(state.roundIndex)

  // Three rounds, three tokens. The shape carries the warning before the name is
  // read, which matters most for the one that changes the rules.
  const token = boss ? "boss" : state.roundIndex === 0 ? "normal" : "elite"

  // The one question the tutorial asks, on the one screen it can be asked from:
  // before the board, on the round the cards would run on. Everywhere else is
  // either too early to mean anything (the title screen, where "tips" is about a
  // game nobody has seen) or too late to be a choice (the board, where the first
  // card has already been read by the time its button is found).
  const asking = chrome.coachOffer

  return h(
    "div",
    // Tapping anywhere plays, which is the right shortcut for a card that says
    // one thing and is dismissed. It is withdrawn while the card is asking
    // something: a stray tap must not answer a question on the player's behalf,
    // least of all by picking the option they were reaching past.
    {
      class: `screen center intro ${asking ? "asking" : ""}`,
      ...(asking ? {} : { onclick: () => on.play() }),
    },
    h(
      "div",
      { class: "intro-stage" },
      state.won ? copy.stageEndless(state.stage) : copy.stage(state.stage, STAGES),
    ),
    h(
      "div",
      { class: `intro-card ${boss ? "boss-card" : ""}` },
      h("div", { class: `round-token ${token}` }),
      h("div", { class: "intro-name" }, boss ? bossCard(boss.id).name : name),
      boss && h("div", { class: "intro-rule" }, bossCard(boss.id).text),
      h("div", { class: "intro-label" }, copy.scoreAtLeast),
      h("div", { class: "intro-target" }, num(state.round.target)),
      h(
        "div",
        { class: "intro-meta" },
        // The payout the run will actually be handed, not the one the table
        // lists: ascension 7 takes a dollar off it, and a card that promises $3
        // before a round and pays $2 after it is the worst kind of wrong.
        copy.meta(
          state.round.maxGuesses,
          money(Math.max(0, (ROUND_PAYOUT[state.roundIndex] ?? 0) - difficultyOf(state).payoutCut)),
        ),
      ),
      standing(state),
    ),
    asking && h("p", { class: "intro-ask" }, copy.coachAsk),
    h(
      "button",
      { class: "primary", type: "button", onclick: () => on.play() },
      asking ? copy.coachYes : copy.play,
    ),
    // Second and quieter. Why it is phrased as the whole tutorial rather than as
    // this card is written down beside the string.
    asking &&
      h(
        "button",
        { class: "secondary", type: "button", onclick: () => on.skipCoach() },
        copy.coachNo,
      ),
    muteButton(on, chrome),
  )
}

/**
 * The run's own rules, named on the card that announces the round.
 *
 * Named rather than spelled out: at the top of the ladder that would be six
 * sentences competing with the target for a card meant to be read in a second,
 * and every one of them was read in full on the title screen before the run
 * started. The names are the reminder; the codex has the wording; and the toast
 * that refuses a guess says exactly what was broken, which is the moment the
 * detail is actually wanted.
 */
function standing(state: RunState): HTMLElement | null {
  const rules = rulesFor(state)
  if (rules.length === 0) return null
  const { targets } = difficultyOf(state)
  return h(
    "div",
    { class: "intro-asc" },
    h("strong", {}, ui().common.ascension(state.ascension ?? 0)),
    ` · ${rules.map((rule) => ascensionCard(rule).name).join(" · ")}`,
    // The endless half of the ladder, said as the number it is. `rulesFor`
    // deliberately does not return those rungs, and this is why: a run at
    // ascension 30 would otherwise put twenty copies of "Steeper" on this card,
    // when the one thing the player needs off it is how much the targets moved.
    targets > 1 ? ` · ${ui().intro.targets(targets.toFixed(2))}` : null,
  )
}

function muteButton(on: Handlers, chrome: Chrome): HTMLElement {
  return h(
    "button",
    {
      class: "mute",
      type: "button",
      // The card behind this is itself a tap target; without stopping here the
      // toggle would also start the round.
      onclick: (event: Event) => {
        event.stopPropagation()
        on.mute()
      },
    },
    chrome.muted ? ui().common.soundOff : ui().common.soundOn,
  )
}

/* -------------------------------------------------------------- the reward */

export function rewardView(state: RunState, on: Handlers): HTMLElement {
  const copy = ui().reward
  const reward = state.reward
  const line = (label: string, amount: number) =>
    h("div", { class: "reward-line" }, h("span", {}, label), h("span", {}, money(amount)))

  return h(
    "div",
    { class: "screen center" },
    h("h1", { class: "banner win" }, copy.cleared),
    h(
      "div",
      { class: "panel" },
      h("div", { class: "answer-note" }, copy.answerWas(state.round.answer)),
      h(
        "div",
        { class: "score-note" },
        copy.score(num(state.round.score), num(state.round.target)),
      ),
      reward && line(roundName(state.roundIndex), reward.base),
      reward && reward.unusedGuesses > 0 && line(copy.unusedGuesses, reward.unusedGuesses),
      reward && reward.interest > 0 && line(copy.interest, reward.interest),
      reward &&
        h(
          "div",
          { class: "reward-line total" },
          h("span", {}, copy.total),
          h("span", {}, money(reward.total)),
        ),
    ),
    h("button", { class: "primary", type: "button", onclick: () => on.collect() }, copy.collect),
  )
}

/* ---------------------------------------------------------------- the shop */

/**
 * What a range level buys, spelled out letter by letter.
 *
 * Shared between the shelf's card and the rules sheet's list because they are
 * the same sentence about the same four cards, and the sheet is most often
 * opened *from* the shop by a player deciding whether to buy the thing they are
 * looking at. Two copies of it drifted apart once already: the sheet enumerated
 * the letters from the day ranges landed and the card said "A–E letters", which
 * is the shorter of the two and the one being read under a price.
 */
const rangeText = (range: Range): string =>
  ui().shop.rangeText([...range.letters].join(" ").toUpperCase(), CHIPS_PER_LEVEL)

/**
 * What a card says about itself. Split out from the card because a pack lays out
 * the same items the shop sells, and a Steel E ought to read identically whether
 * it is being bought or being chosen.
 *
 * The tag is the answer to a question the shelf was not answering. Seven kinds of
 * thing are sold here and every one of them was the same rectangle: a name, a
 * sentence, a price. So "The Mint" and "Etch Heavy" were distinguishable only by
 * reading both sentences and knowing the game well enough to classify them. The
 * tag says the kind in one word before the name is read.
 *
 * It also said when a card had nowhere to land. The relic's no longer does.
 * "Relic · tray full" is a fact about the run rather than about the card, so it
 * was never on one card at a time: a full tray tags every relic on the shelf at
 * once, and a shelf of warnings reads as a shelf that is broken rather than as
 * stock. It also spends the name's own line. Five cards across a phone leaves
 * about 64px for a name, and the ticket was taking three words of it to say
 * something the tray directly below is already saying by having no empty seat in
 * it, about a purchase the player can make room for by selling on this very
 * screen. The refusal is said at the till instead, where it is one line about
 * the one card that was actually tapped.
 *
 * The consumable keeps its warning, and the difference is whether the screen can
 * act on it. A card is used in a round, so a full hand is a wall for the whole
 * shop visit, since nothing on this screen empties it, and the tag is the only place
 * that can say so before the gold is committed to a tap that will bounce.
 *
 * Rarity stays a color and never becomes a word. Relics were the exception, with
 * "Uncommon Relic" on the grounds that rarity is an economy there. But a tag
 * that says two things says the second one second, and the kind is what the tag
 * is for. The card was already announcing its rarity three ways over: the
 * border, the tag's own ink, and the tray the relic is bound for. The word was
 * the fourth telling, and the only one charging the name room on the line.
 *
 * The tip is the sentence behind the tag. One word is enough to sort five cards
 * into kinds and not nearly enough to say what a kind *is*. "Alphabet" names
 * nothing to a player who has not already bought one, so the ticket answers the
 * question it provokes when it is hovered or held.
 *
 * One line, and deliberately shorter than the card it hangs off. It says how
 * that kind of thing works and stops: not what this card does, which the body
 * copy under it is already saying, and not the fine print, which is what the
 * rules sheet and the codex are for. A panel that has to be *read* is one the
 * player has stopped hovering by the end of. It covers the shelf it is
 * explaining while it does it, so anything that would not fit in a breath was
 * dropped rather than compressed. The test holds the length.
 *
 * `swap` is the one thing a card says that is about the run rather than about
 * itself: an aimed modifier landing on a letter that is already carrying one
 * destroys what is there, and that is the only line on the shelf describing
 * something the player *loses* by tapping. It gets a field and a line of its own
 * rather than a clause on the end of `text`, because it used to be a clause on
 * the end of `text`, "E scores ×3 mult, and can break when it lands gray,
 * replacing Steel", which buried the consequence at the end of the longest
 * sentence on the card, in the same ink as the sales copy, past the point a
 * shelf of five cards is read to.
 */
export function describeItem(
  item: ShopItem,
  state: RunState,
): {
  title: string
  text: string
  rarity: string
  tag: string
  tip: string
  blocked: boolean
  swap: string
} {
  const copy = ui().shop
  let title = ""
  let text = ""
  let swap = ""
  let rarity = "common"
  let tag = ""
  /**
   * Written as one line each. The tip panel is `white-space: pre-line`, so a
   * catalog entry wrapped across source lines would arrive with the source's own
   * breaks in it; `en.ts` concatenates rather than using a template literal for
   * exactly that reason.
   */
  let tip = ""
  /** The tag is a warning rather than a label: this one has nowhere to go. */
  let blocked = false

  if (item.kind === "pack") {
    const pack = PACK_BY_ID.get(item.id)
    title = packCard(item.id).name
    text = packCard(item.id).text
    // Packs read as the rare thing on the shelf because they are the dearest and
    // the only one that asks a question back.
    rarity = "rare"
    tag = copy.tagPack
    tip = copy.tipPack(pack?.picks ?? 1)
  } else if (item.kind === "relic") {
    const relic = RELIC_BY_ID.get(item.id)
    title = relicCard(item.id).name
    text = relicCard(item.id).text
    rarity = relic?.rarity ?? "common"
    // Never blocked, whatever the tray holds. The refusal still happens. The
    // engine turns the purchase away with "no relic slots free" and the toast
    // says it. It just happens at the till rather than on the card.
    tag = copy.tagRelic
    tip = copy.tipRelic
  } else if (item.kind === "consumable") {
    title = consumableCard(item.id).name
    text = consumableCard(item.id).text
    blocked = state.consumables.length >= CONSUMABLE_SLOTS
    tag = blocked ? copy.tagConsumableFull : copy.tagConsumable
    tip = copy.tipConsumable
  } else if (item.kind === "mod") {
    const mod = MODIFIER_BY_ID.get(item.id)
    const card = modifierCard(item.id)
    rarity = mod?.rarity ?? "common"
    // One sentence for both versions of this card. The difference between them,
    // which is who picks the letter, is already the difference between the two
    // titles, and the tip is about the kind rather than about the card.
    tip = copy.tipMod
    if (item.letter === undefined) {
      // The shop's version: a card with no letter on it yet. Named for what it
      // is being bought as, a choice, because the price is the price of the
      // choice, and the letter it ends up on is the next screen.
      title = copy.modAnyTitle(card.name)
      // Two whole sentences rather than one with a clause appended, because the
      // letters are a restriction on the choice and a language that puts them
      // before the verb has nowhere to append to.
      text = mod?.letters
        ? copy.modAnyTextOnly(card.text, [...mod.letters].join(" ").toUpperCase())
        : copy.modAnyText(card.text)
      tag = copy.tagLetter
    } else {
      const letter = item.letter.toUpperCase()
      title = copy.modTitle(card.name, letter)
      text = copy.modText(letter, card.text)
      // A letter holds one modifier, so this is sometimes a trade rather than an
      // addition, and that has to be legible before the gold is gone.
      const current = state.letters[item.letter]?.mod
      const held = current ? MODIFIER_BY_ID.get(current) : undefined
      if (held && held.id !== item.id) swap = copy.swap(modifierCard(held.id).name, held.pip)
      // The pack's aimed version. Still a letter card, and the tag says so for
      // the same reason it does in the shop: the name reads "Gold E", which is
      // the letter, not the kind of thing being handed over.
      tag = copy.tagLetter
    }
  } else if (item.kind === "range") {
    const range = RANGE_BY_ID.get(item.id)
    // The one name that stayed in the engine: A–E is the letters it holds, said
    // in the punctuation every language uses for a span, and `ranges.test.ts`
    // asserts it as a fact about the partition rather than as copy.
    title = range
      ? copy.rangeTitle(range.name, rangeLevelOf(state, item.id) + 1)
      : copy.fallbackRange
    // Spelled out rather than named again. "A–E letters" is the title over
    // again in the body's ink, and it asks a player mid-shop to expand a dash
    // into five letters and check their own vocabulary against it, which is
    // the whole decision the card is selling. F–M is the one that settles it:
    // eight letters, and the ones that matter to a guess are the ones nobody
    // recites when they read the endpoints. So it reads exactly the way an
    // etching does, "A E I O U are worth +2 chips", and exactly the way the
    // same four ranges are already listed in the rules sheet.
    //
    // The whole slice, including any letter that has been destroyed. The range
    // is a fixed partition of the alphabet and the title names it as one, so a
    // list that quietly dropped C would read as a differently-cut range rather
    // than as a fact about this run; deadness is a mark on the key, which is
    // where the player has been reading it since the letter broke.
    text = range ? rangeText(range) : ""
    tag = copy.tagRange
    tip = copy.tipRange
  } else if (item.kind === "level") {
    const category = CATEGORY_BY_ID.get(item.id)
    const card = categoryCard(item.id)
    title = category ? copy.levelTitle(card.name, levelOf(state, item.id) + 1) : copy.fallbackLevel
    text = category ? copy.levelText(card.name, category.chips, category.mult) : ""
    tag = copy.tagShape
    tip = copy.tipShape
  } else {
    // Falls back to something readable rather than to `undefined`, so a save
    // written before etchings were sold by the group degrades quietly.
    const etching = ETCHING_BY_ID.get(item.id)
    title = etching ? etchingCard(etching).name : copy.fallbackEtching
    text = etching ? etchingCard(etching).text : ""
    tag = copy.tagEtching
    tip = copy.tipEtching
  }

  return { title, text, rarity, tag, tip, blocked, swap }
}

/**
 * The line a card grows when tapping it would throw something away.
 *
 * Carries the outgoing modifier's own color, because that is how the letter has
 * been saying "Steel" since the day it was bought: the ring on the key, the pip
 * in its corner and the dot on a played tile all read off `--mod`, and a warning
 * about Steel drawn in warning-red would be the first thing in the game to name
 * Steel in a color that is not Steel's.
 */
function swapLine(swap: string, modId: string | undefined): HTMLElement {
  return h("div", { class: "shop-item-swap", "data-mod": modId }, swap)
}

/**
 * What placing `modifier` on `letter` would destroy, or undefined if it would
 * destroy nothing. The question the confirmation exists to ask, in one place.
 *
 * One place because it was two, and the two disagreed. `app.ts` decided whether
 * to arm by testing the letter's `mod` against `undefined`; an empty letter
 * holds `null`, so every letter in the alphabet read as occupied and armed on
 * the first tap, while this file asked `modifierOf` and correctly drew no
 * question about it. What the player got was a tap that appeared to do nothing
 * on exactly the letters (the bare ones, most of the board) that were supposed
 * to place in one. Two answers to one question can be wrong in that shape; one
 * cannot.
 *
 * `placeableLetters` is part of the answer rather than a separate check above
 * it: a letter the engine is going to refuse (a Q under Echo, a broken letter,
 * one already carrying this very modifier) is not a trade to confirm. It has to
 * fall through to the dispatch and be refused with a reason, and it does that by
 * arriving here and being told there is nothing to lose.
 */
export function displacedAt(
  state: RunState,
  modifier: Modifier,
  letter: string,
): Modifier | undefined {
  if (!placeableLetters(state, modifier).includes(letter)) return undefined
  return modifierOf(state, letter)
}

/** The modifier a shelf card would displace, for `swapLine` to color. */
function displaced(item: ShopItem, state: RunState): string | undefined {
  if (item.kind !== "mod" || item.letter === undefined) return undefined
  const current = state.letters[item.letter]?.mod
  return current && current !== item.id ? current : undefined
}

function shopItemCard(item: ShopItem, index: number, state: RunState, on: Handlers): HTMLElement {
  const affordable = state.gold >= item.cost
  const { title, text, rarity, tag, tip, blocked, swap } = describeItem(item, state)

  return h(
    "button",
    {
      class: `shop-item kind-${item.kind} rarity-${rarity} ${item.kind === "pack" ? "pack" : ""} ${
        affordable ? "" : "broke"
      }`,
      // The stock deals in one card at a time rather than appearing all at once,
      // which is what makes a reroll feel like being dealt a new hand.
      style: `--deal:${index}`,
      type: "button",
      onclick: () => on.buy(index),
    },
    shopItemHead(title, tag, blocked, tip, rarity),
    h("div", { class: "shop-item-text" }, text),
    swap ? swapLine(swap, displaced(item, state)) : null,
    h("div", { class: "shop-item-cost" }, money(item.cost)),
  )
}

/**
 * The name, with the kind ticketed off to the right of it on the same line.
 *
 * The tag used to have a row of its own above the name, and on a two-column
 * shelf that row cost the card a line of the body copy the purchase is actually
 * decided on. Beside the name it reads as the ticket on the shelf edge rather
 * than as a second title, and the line it was taking goes back to saying what
 * the thing does.
 *
 * Baseline-aligned rather than centered, so a long name and a long tag
 * ("Consumable · slots full" is the worst pair the shelf can deal) still sit on
 * one line of type, with the tag wrapping under itself in the corner rather than
 * dragging the name off its own baseline. An empty `tag` means no ticket, which
 * is how a pack asks for the name on its own.
 *
 * The tip hangs off the ticket rather than off the whole card, because the card
 * is a buy button: a panel that opened over the shelf every time the pointer
 * crossed a price would be in the way of the thing it is explaining. An empty
 * `tip` leaves the ticket inert, which is what the pack sheet asks for. Its
 * backdrop sits above the tip's layer, so a tip raised from inside it would be
 * drawn behind the sheet that asked for it.
 *
 * The rarity rides along so the panel takes the card's own edge color, the way
 * a relic's does from the tray.
 */
function shopItemHead(
  title: string,
  tag: string,
  blocked: boolean,
  tip: string,
  rarity: string,
): HTMLElement {
  return h(
    "div",
    { class: "shop-item-head" },
    h("div", { class: "shop-item-name" }, title),
    tag
      ? h(
          "div",
          {
            class: `shop-item-kind${blocked ? " blocked" : ""}`,
            "data-tip": tip || undefined,
            "data-rarity": tip ? rarity : undefined,
          },
          tag,
        )
      : null,
  )
}

/**
 * An open pack, laid out over the shop.
 *
 * Deliberately not the ordinary `overlay` shell: that one dismisses when the
 * backdrop is tapped, which here would forfeit a pack that has already been paid
 * for. Walking away has to be a button you meant to press.
 */
export function packView(state: RunState, on: Handlers): HTMLElement | null {
  const copy = ui().pack
  const open = state.pack
  if (!open) return null
  const left = open.options.filter(Boolean).length

  return h(
    "div",
    { class: "overlay pack-overlay" },
    h(
      "div",
      { class: "sheet pack-sheet", ...SHEET_ATTRS },
      h("h2", { class: "sheet-title" }, packCard(open.id).name),
      h(
        "p",
        { class: "pack-hint" },
        open.picks > 1 ? copy.choosePicks(open.picks, left) : copy.choose(left),
      ),
      h(
        "div",
        { class: "pack-options" },
        ...open.options.map((item, index) => {
          if (!item)
            return h("div", { class: "shop-item sold", style: `--deal:${index}` }, copy.taken)
          const { title, text, rarity, swap } = describeItem(item, state)
          return h(
            "button",
            {
              class: `shop-item kind-${item.kind} rarity-${rarity}`,
              style: `--deal:${index}`,
              type: "button",
              onclick: () => on.pickPack(index),
            },
            // No tag at all on this sheet. A pack deals one kind of card, so on
            // the shelf's terms it would say "Letter" three times under a title
            // that already said Alphabet Pack. The last case it had left to say
            // was the pick that could be taken and then refused, a relic dealt
            // into a full tray, and that one was never really its to say:
            // `packContents` returns nothing for a relic pack the tray has no
            // room for, so the shop refuses to open it at all rather than
            // selling three cards that all bounce. If a pack ever deals a
            // second pick and the first one fills the tray, the refusal lands
            // where the shop's does now, at the till, with the pack still open
            // and the toast naming the card that was tapped.
            //
            // No tip either, and not only because there is no tag to hang it
            // on: this sheet is a held decision, and the pack's own title has
            // already said what kind of thing is being dealt.
            shopItemHead(title, "", false, "", rarity),
            h("div", { class: "shop-item-text" }, text),
            // The one warning that does stay on this sheet, and the difference
            // from the tag it replaced is that this pick does not bounce. A
            // relic dealt into a full tray is refused and the pack stays open,
            // so the till can say it afterwards and nothing has been lost; a
            // Glass E dealt onto a Steel E is honored, and the Steel is gone
            // before there is anything to say it to.
            //
            // A pack is also where this lands most often, because the pack is
            // the only thing left that rolls the pairing: the shop sells the
            // card unaimed and the player picks the letter on the next screen
            // with the whole alphabet in front of them, while a pack deals three
            // letters somebody else chose.
            swap ? swapLine(swap, displaced(item, state)) : null,
            // The price it would have carried in the stock, struck through: the
            // pack already charged for it, and seeing what it would have cost is
            // most of what makes opening one feel like a win.
            h("div", { class: "shop-item-cost free" }, money(item.cost)),
          )
        }),
      ),
      h(
        "div",
        { class: "shop-actions" },
        h(
          "button",
          { class: "secondary", type: "button", onclick: () => on.skipPack() },
          open.picks > 1 ? copy.skipSome : copy.skip,
        ),
      ),
    ),
  )
}

/**
 * The letter picker: a modifier in hand, waiting to be put somewhere.
 *
 * Shaped like the keyboard rather than like a list, because the question it
 * asks, "which letter do you actually type?", is one the player has already
 * been answering on that exact layout all run. Each key carries what the letter
 * is worth in chips and what it is already holding, so the trade a replacement
 * would make is visible before it is made.
 *
 * No skip and no backdrop dismissal, for the reason `packView` has none: the
 * gold is spent, and every legal letter was checked before it was taken.
 *
 * One tap on a free letter, two on a taken one. Everything above was already
 * true and was not enough: the mark saying a letter was carrying something was a
 * 10px pip and 70% opacity, the sentence saying what that meant was a footnote
 * under a keyboard, and the footnote was not even drawn for a restricted
 * modifier. Echo goes on six letters, so its note spent itself naming them and
 * the only warning about replacement went missing on the card most likely to
 * land on a letter that already had one. The tap itself said nothing at all: a
 * $13 Glass could eat a $12 Anchor on a fat thumb, or on the physical keyboard
 * on a single wrong keystroke, with no gesture in between that could be aimed
 * badly and no way back afterwards.
 *
 * So the taken key arms instead of placing, and the footnote's job moves to a
 * block that names both modifiers and asks. The free letters, which are most of
 * the alphabet on most visits and all of it on the first, keep their one tap,
 * because a confirmation on a choice that destroys nothing is the kind of thing
 * a player learns to tap through, and a player who taps through this one is
 * exactly who it is for.
 *
 * `arming` is a UI concern and lives in the UI: it is a half-finished gesture,
 * not a fact about the run, and putting it in `RunState` would write it into
 * saves and golden vectors for the sake of a thing that does not survive letting
 * go of the phone.
 */
export function placeView(
  state: RunState,
  on: Handlers,
  arming: string | null,
): HTMLElement | null {
  const copy = ui().place
  const held = state.placing
  const modifier = held ? MODIFIER_BY_ID.get(held) : undefined
  if (!modifier) return null
  const card = modifierCard(modifier.id)
  const allowed = new Set(placeableLetters(state, modifier))
  // What the armed letter stands to lose, worked out here rather than taken on
  // the caller's word. The field naming the letter outlives renders that the
  // sheet does not, so this asks the run the question again: a letter that is no
  // longer placeable, or no longer carrying anything, is not a trade to confirm,
  // and `armed` falls back to null rather than drawing a question about nothing.
  const losing = arming ? displacedAt(state, modifier, arming) : undefined
  const armed = losing ? arming : null

  const key = (letter: string) => {
    const entry = state.letters[letter]
    const current = entry?.mod ? MODIFIER_BY_ID.get(entry.mod) : undefined
    // Carried for the same reason the board's keys carry it: the figure below is
    // drawn one way when the letter is worth what it started as and another when
    // it has been bought up, and this sheet is read straight after the board.
    const bought = (entry?.etch ?? 0) + rangeChips(state, letter) > 0
    return h(
      "button",
      {
        class: [
          "key",
          "place-key",
          entry?.destroyed ? "broken" : "",
          current ? "taken" : "",
          letter === armed ? "arming" : "",
          bought ? "etched" : "",
        ]
          .filter(Boolean)
          .join(" "),
        "data-mod": current?.id,
        type: "button",
        disabled: !allowed.has(letter),
        // Both taps, and the arming is decided in `app.ts` rather than here:
        // the physical keyboard answers this sheet too, and a letter typed at it
        // has to arm on exactly the letters a tap arms on.
        onclick: () => on.placeMod(letter),
      },
      letter.toUpperCase(),
      // What the letter pays before any of this lands. It is the number the
      // choice actually turns on: Chip on a 1-chip vowel you type every guess
      // beats Chip on a 10-chip Z you type twice a run.
      h("span", { class: "place-chips" }, num(baseChips(state, letter))),
      current ? h("span", { class: "mod-pip" }, current.pip) : null,
    )
  }

  // What the sheet says under the keyboard when nothing is armed. Both sentences
  // can be true at once, which is the bug the old single-line version had: the
  // restriction used to be written *instead of* the replacement rule.
  const notes: string[] = []
  if (modifier.letters) {
    notes.push(copy.onlyOn(card.name, [...modifier.letters].join(" ").toUpperCase()))
  }
  // Said only when there is a key on screen it is about. On a run that has
  // bought one modifier, which is every run, once, it is a rule about a
  // situation that has not arisen, and the sheet is better off short.
  if ([...allowed].some((letter) => state.letters[letter]?.mod)) notes.push(copy.oneEach)

  return h(
    "div",
    { class: "overlay pack-overlay" },
    h(
      "div",
      { class: "sheet pack-sheet", ...SHEET_ATTRS },
      h("h2", { class: "sheet-title" }, card.name),
      h("p", { class: "pack-hint" }, copy.choose(card.text)),
      h(
        "div",
        // The incoming modifier's color, hung on the container so the armed key
        // can ring itself in it while every other key keeps its own. See the
        // `--incoming` note in the stylesheet for why it survives the override.
        { class: "keyboard place-keys", "data-mod": modifier.id },
        ...keyRows().map((row) => h("div", { class: "key-row" }, ...[...row].map(key))),
      ),
      armed && losing
        ? h(
            "div",
            // The outgoing modifier's color, on the block rather than on the
            // word inside it, so the edge and the name are lit by one decision.
            { class: "place-swap", "data-mod": losing.id },
            // Two sentences with a bolded name between them, and the space and
            // the full stop live here rather than in either of them. The name is
            // the last word of the first sentence in English and is not in any
            // of the other three, so a catalog entry that swallowed the marks
            // would be a sentence a translator cannot reorder.
            h(
              "p",
              { class: "place-swap-line" },
              `${copy.carrying(armed)} `,
              h("strong", {}, `${modifierCard(losing.id).name} ${losing.pip}`),
              `. ${copy.loses(card.name)}`,
            ),
            h(
              "div",
              { class: "shop-actions" },
              // Same order and the same shape as the quit sheet: the destructive
              // answer on the left in the danger ink, the one that changes
              // nothing on the right wearing the weight.
              h(
                "button",
                { class: "danger", type: "button", onclick: () => on.placeMod(armed) },
                copy.replace(modifierCard(losing.id).name),
              ),
              h(
                "button",
                { class: "primary", type: "button", onclick: () => on.cancelPlace() },
                copy.keep(modifierCard(losing.id).name),
              ),
            ),
          )
        : notes.length > 0
          ? h("p", { class: "place-note" }, notes.join(" "))
          : null,
    ),
  )
}

/**
 * The levels a run is holding, under the stock that sells more of them.
 *
 * The shop is where the decision is. "Twinned → Lv 3" for $8 is unanswerable
 * without knowing what else you have leveled, and the card itself can only
 * speak for the one shape it is selling. Always present rather than hidden
 * behind the levels being non-trivial, because the visit where you own nothing
 * is exactly the visit where you have not heard of the system.
 */
function shopShapes(state: RunState, on: Handlers): HTMLElement {
  const copy = ui().shop
  const leveled = CATEGORIES.filter((category) => levelOf(state, category.id) > 1)
  return h(
    "button",
    { class: "shapes-line", type: "button", onclick: () => on.openShapes() },
    h("span", { class: "shapes-line-label" }, copy.shapesLabel),
    h(
      "span",
      { class: "shapes-line-body" },
      leveled.length > 0
        ? leveled
            .map((c) => copy.shapesLevel(categoryCard(c.id).name, levelOf(state, c.id)))
            .join(" · ")
        : copy.shapesNone,
    ),
    h("span", { class: "shapes-line-more" }, "›"),
  )
}

export function shopView(state: RunState, on: Handlers): HTMLElement {
  const copy = ui().shop
  const shop = state.shop
  const reroll = shop ? rerollCost(shop) : 0

  // Drawn as seats rather than as a list, the way the board draws it, because
  // the shop is the one screen where the empty ones are the point: a shelf
  // offering a $8 relic to a player who cannot see whether they have room for it
  // is asking a question with the answer off screen. Full length, so the tray
  // that has no space left looks like a tray with no space left.
  const owned = Array.from({ length: difficultyOf(state).relicSlots }, (_, slot) => {
    const instance = state.relics[slot]
    const relic = instance ? RELIC_BY_ID.get(instance.id) : undefined
    if (!instance) return h("div", { class: "relic empty" })
    return h(
      "button",
      {
        class: `relic rarity-${relic?.rarity ?? "common"}`,
        "data-tip": relicCard(instance.id).text,
        "data-rarity": relic?.rarity ?? "common",
        type: "button",
        onclick: () => on.sell(slot),
      },
      h("span", { class: "relic-name" }, relicCard(instance.id).name),
      h("span", { class: "sell" }, copy.sell(money(sellValue(relic?.cost ?? 4)))),
    )
  })

  return h(
    "div",
    { class: "screen shop-screen" },
    h(
      "header",
      { class: "hud" },
      h("div", { class: "hud-round" }, h("div", { class: "round-name" }, copy.title)),
      h("div", { class: "hud-gold" }, money(state.gold)),
      menuButton(on),
    ),
    h(
      "div",
      { class: "shop-items" },
      ...(shop?.items ?? []).map((item, index) =>
        item
          ? shopItemCard(item, index, state, on)
          : h("div", { class: "shop-item sold", style: `--deal:${index}` }, copy.sold),
      ),
    ),
    shopShapes(state, on),
    h(
      "div",
      { class: "shop-actions" },
      h(
        "button",
        {
          class: "secondary",
          type: "button",
          disabled: state.gold < reroll,
          onclick: () => on.reroll(),
        },
        copy.reroll(money(reroll)),
      ),
      h(
        "button",
        { class: "primary", type: "button", onclick: () => on.nextRound() },
        copy.nextRound,
      ),
    ),
    h(
      "div",
      { class: "owned" },
      h(
        "div",
        { class: "owned-label" },
        (state.relics.length > 0 ? copy.ownedSellable : copy.owned)(
          state.relics.length,
          difficultyOf(state).relicSlots,
          state.consumables.length,
          CONSUMABLE_SLOTS,
        ),
      ),
      h("div", { class: "relics" }, ...owned),
    ),
    h("div", { class: "relic-tip" }),
    h("div", { class: "toast" }),
  )
}

/* ----------------------------------------------------------- run over */

/**
 * The end of a run, whichever end it is.
 *
 * The win is a fork rather than a finish. Stage `STAGES` is where the authored
 * game stops, not where the run has to: the targets keep growing geometrically
 * and the bosses keep coming, so a build that has beaten the game can be asked
 * how far it actually goes.
 *
 * The screen says plainly that the win survives either choice, rather than
 * inventing a stake to make the fork feel weightier. It has no stake to invent.
 * Nothing is wagered by playing on, and a screen that implied otherwise would be
 * lying to make a button look brave. What is really being asked is whether to
 * start something new or find out where this one breaks.
 */
export function endView(state: RunState, on: Handlers): HTMLElement {
  const copy = ui().end
  const offering = state.phase === "victory"
  const lost = state.phase === "game_over"
  return h(
    "div",
    { class: "screen center" },
    h("h1", { class: `banner ${offering ? "win" : "lose"}` }, offering ? copy.won : copy.lost),
    h(
      "div",
      { class: "panel" },
      lost && h("div", { class: "answer-note" }, ui().reward.answerWas(state.round.answer)),
      lost &&
        h(
          "div",
          { class: "score-note" },
          copy.short(
            num(state.round.score),
            num(state.round.target),
            num(state.round.target - state.round.score),
          ),
        ),
      lost && state.won && h("div", { class: "score-note won-note" }, copy.wonAndWent(STAGES)),
      h("div", { class: "score-note" }, copy.reached(state.stage, roundName(state.roundIndex))),
      cleared(state, offering),
      offering && h("p", { class: "endless-note" }, copy.endlessNote(STAGES)),
    ),
    offering &&
      h(
        "button",
        { class: "primary", type: "button", onclick: () => on.continueRun() },
        copy.endless,
      ),
    // Banking the win ends the run for good, so it goes where a finished run
    // goes: the title screen, with the save cleared behind it.
    h(
      "button",
      {
        class: offering ? "secondary" : "primary",
        type: "button",
        onclick: () => (offering ? on.quit() : on.newRun()),
      },
      offering ? copy.mainMenu : copy.newRun,
    ),
  )
}

/**
 * What the level meant, said at the end rather than only at the start.
 *
 * A win at ascension N is a different thing from a win, and the next rung is the
 * only reward for it, so the screen that offers the win is also the screen that
 * hands the next one over. The dial has always been there and every rung of it
 * is reachable, so what this line marks is earning one rather than taking it: the
 * next level is now the climb rather than a leap. A loss gets the level stated
 * flatly, since it is the terms the run was played under, not a consolation.
 *
 * There is always a next one now, which is the point of the endless half: the
 * last branch is the dial's ceiling rather than the ladder's top, and nobody is
 * going to see it.
 */
function cleared(state: RunState, won: boolean): HTMLElement | null {
  const copy = ui().end
  const level = state.ascension ?? 0
  if (!won) {
    return level > 0 ? h("div", { class: "score-note" }, ui().common.ascension(level)) : null
  }
  return h(
    "div",
    { class: "score-note asc-note" },
    level === 0
      ? copy.firstEarned
      : level < MAX_ASCENSION
        ? copy.earned(level, level + 1)
        : copy.topOfLadder(level),
  )
}

/* ----------------------------------------------------------------- title */

/**
 * Only shown when there is no run to resume. A save sends the player straight
 * back to the board instead. The title is a front door, not a toll booth, and
 * charging a tap for it every launch is exactly the friction a phone game
 * cannot afford.
 */
/**
 * The language, as one wrapping button rather than four.
 *
 * It was four, labelled, on the argument that a player who cannot read the
 * interface cannot read the button that changes it, so a cycle would ask them to
 * tap past languages they do not want while reading nothing. That argument
 * survives the change and is answered by it rather than by the count: every stop
 * on this cycle names itself, in its own script, so nobody is ever reading a
 * language they do not have. The worst case is three taps, each of which lands
 * on a legible word, against a labelled 2×2 block sitting under Play on the
 * first screen anybody sees — four buttons' worth of chrome charged to every
 * player forever so that the few who need it save two taps once.
 *
 * So it is a `.secondary` and nothing else, which is what puts it in line with
 * Play, How to play and Codex: `.screen.center .secondary` sizes all of them to
 * 22rem, and `.sheet-actions` stretches all of them across the pause sheet. The
 * flag is there to be found by someone scanning rather than reading; see
 * `LANG_FLAGS` for why the word beside it is the part that means anything.
 *
 * The accessible name is the one place the word "Language" still appears, and it
 * is the answer to a question the visible label leaves open: a button reading
 * "Español" could be the language in force or the one it switches to. It reads
 * as `Idioma: Español`, so the visible text is contained in the spoken name and
 * voice control still matches on it.
 *
 * See `cycleLanguage` in `app.ts` for the half of this that is not the label:
 * the interface changes now, the run keeps the words it was dealt from.
 */
function languageButton(on: Handlers, chrome: Chrome): HTMLElement {
  return h(
    "button",
    {
      class: "secondary lang-button",
      type: "button",
      "aria-label": `${ui().pause.language}: ${LANG_NAMES[chrome.lang]}`,
      onclick: () => on.cycleLanguage(),
    },
    // No `lang` attribute on either: the button names the language it is
    // written in now, which is the document's, and `setLang` has already said
    // so on `<html>`.
    h("span", { class: "lang-flag" }, LANG_FLAGS[chrome.lang]),
    LANG_NAMES[chrome.lang],
  )
}

/**
 * The card that stands in for a screen while its word list is still arriving.
 *
 * Reachable on exactly one path: a language switched and then a run started
 * before the fetch that the switch kicked off has landed. See `withWords`. It is
 * a sentence and not a spinner because on the connection where this is visible
 * at all it is visible for long enough to read.
 */
export function loadingView(): HTMLElement {
  return h(
    "div",
    { class: "screen center" },
    h("p", { class: "loading-note" }, ui().common.loading),
  )
}

export function titleView(on: Handlers, chrome: Chrome, meta: MetaState): HTMLElement {
  const copy = ui().title
  const common = ui().common
  return h(
    "div",
    { class: "screen center title" },
    h("h1", { class: "title-name" }, copy.name),
    h("p", { class: "title-tag" }, copy.tagline),
    record(meta, on),
    ladder(on, meta),
    h("button", { class: "primary", type: "button", onclick: () => on.newRun() }, common.play),
    h(
      "button",
      { class: "secondary", type: "button", onclick: () => on.openHelp() },
      common.howToPlay,
    ),
    h(
      "button",
      { class: "secondary", type: "button", onclick: () => on.openCodex() },
      common.codex,
    ),
    // Here as well as on the pause sheet, and this is the copy that matters more
    // of the two: the title screen is the first screen anybody sees, and a
    // player who has landed on the wrong language needs the way out of it before
    // they have started a run, not from a menu inside one.
    //
    // Last of the four rather than first, because the flag finds the eye on its
    // own and the three above it are what the screen is for. No deferred-words
    // line under it: `wordsDeferred` is false at the title by construction, and
    // it is the only screen where that is true of every language.
    languageButton(on, chrome),
    muteButton(on, chrome),
    h("p", { class: "title-build" }, buildStamp()),
  )
}

/**
 * What every run before this one added up to.
 *
 * Silent on a fresh profile: a row of zeros is worse than nothing, because it
 * tells a first-time player they are already behind on a scoreboard they have
 * not seen the game for yet. The first line appears after the first run, which
 * is also the first moment it says anything.
 *
 * Wins are dropped rather than shown as zero for the same reason, and read as a
 * count rather than as a word: "beaten" next to two other numbers is a sentence
 * pretending to be a statistic, and it is ambiguous about who beat whom.
 */
function record(meta: MetaState, on: Handlers): HTMLElement | null {
  const copy = ui().title
  if (meta.runs === 0) return null
  const parts = [
    copy.runs(meta.runs),
    ...(meta.wins > 0 ? [copy.wins(meta.wins)] : []),
    copy.bestStage(meta.bestStage),
  ]
  // A button rather than a line with a button beside it: the summary *is* the
  // link to the long version, so there is nothing extra on the title screen and
  // the thing a player would poke at anyway is the thing that opens.
  return h(
    "button",
    { class: "title-record", type: "button", onclick: () => on.openStats() },
    parts.join(" · "),
  )
}

/**
 * The long version of the record.
 *
 * Everything here is about the player rather than about a run, which is why it
 * hangs off the title screen and not off the HUD: mid-run, "which word do you
 * type most" is a distraction, and between runs it is the only thing to read.
 *
 * `pool` is the two word lists' sizes, passed in rather than counted here
 * because the lists are fetched and this module is built from content. Both
 * arrive as zero before they land, and a denominator is dropped rather than
 * shown as "of 0".
 *
 * Two of them because the screen counts two collections against two different
 * ceilings: the answers cracked against the answer list, and the words played
 * against the much longer list of what is legal to type. The second is the
 * larger number by seven times in English and the one that moves on a round that
 * went badly, which is most of why it is worth showing beside the first.
 *
 * `words` is the language those lists are in, which is the run's and not
 * necessarily the interface's — a Spanish menu can be sitting over an English
 * run. It is the right one of the two anyway: both counts are read as fractions
 * of `pool`, and `pool` is a fact about the lists that are loaded.
 */
export function statsView(
  meta: MetaState,
  pool: { answers: number; allowed: number },
  words: string,
  on: Handlers,
): HTMLElement {
  const copy = ui().stats
  const played = roundsPlayed(meta)
  const best = favoriteWord(meta)
  const solved = wordsFound(meta)
  const mean = meanSolve(meta)
  const cracked = crackedIn(meta, words)
  const typed = playedIn(meta, words)

  /** The tallest row in the chart. Never zero, so an empty chart cannot divide by it. */
  const peak = Math.max(1, meta.missed, ...meta.solves)

  const figure = (label: string, value: string) =>
    h("div", { class: "figure" }, h("strong", {}, value), h("span", {}, label))

  /**
   * Two readings of the same number, on purpose.
   *
   * The percentage is the share of every round ever played, which is the honest
   * one and is what a player would mean by "how often". The bar is drawn against
   * the tallest row instead. A distribution whose mode is 31% would otherwise
   * spend the whole chart in the left third of the track, and the shape (did the
   * answers come on the third guess or the fifth, is the peak sharp or flat) is
   * the entire reason a bar is here rather than a fourth column of digits. The
   * label carries the truth and the bar carries the shape.
   */
  const bar = (label: string, count: number, tone: string) => {
    const share = played > 0 ? count / played : 0
    return h(
      "div",
      { class: `breakdown-row ${tone}` },
      h("span", { class: "breakdown-label" }, label),
      h(
        "span",
        { class: "breakdown-track" },
        h("span", {
          class: "breakdown-fill",
          style: `--fill:${((count / peak) * 100).toFixed(1)}%`,
        }),
      ),
      h("span", { class: "breakdown-share" }, ui().common.percent(Math.round(share * 100))),
      h("span", { class: "breakdown-count" }, num(count)),
    )
  }

  const relics = favoriteRelics(meta).slice(0, 3)

  return overlay(
    on,
    h("h2", { class: "sheet-title" }, copy.title),
    h(
      "div",
      { class: "figures" },
      figure(copy.runs, num(meta.runs)),
      figure(copy.wins(meta.wins), num(meta.wins)),
      figure(copy.bestStage, num(meta.bestStage)),
      figure(copy.guesses, num(meta.guesses)),
      // Three across and two down rather than four across, which is what buys the
      // second row: the top one is the run record and the bottom one is the word
      // record, and a player who wants to know whether they are getting better at
      // the game reads the bottom row. An em dash rather than a zero before any
      // round has ended. 0% solved is a claim, and this player has not made it.
      figure(
        copy.solved,
        played > 0 ? ui().common.percent(Math.round((solved / played) * 100)) : ui().common.none,
      ),
      figure(copy.meanSolve, mean === null ? ui().common.none : mean.toFixed(1)),
    ),
    h(
      "p",
      { class: "stat-line" },
      h("strong", {}, copy.cracked(cracked)),
      ` ${pool.answers > 0 ? copy.crackedOf(cracked, num(pool.answers)) : copy.crackedBare(cracked)}`,
    ),
    // Below the cracked line rather than above it, though it is the bigger
    // number: that one is the achievement and this one is the ground covered
    // getting there. Hidden until a word has actually been played, because the
    // field is new and every record predating it starts this collection at zero
    // while the one above it may already be in the hundreds. Two fractions that
    // disagree about how long somebody has played would read as a bug in the
    // counting rather than as a counter that started late.
    typed > 0
      ? h(
          "p",
          { class: "stat-line" },
          h("strong", {}, copy.played(typed)),
          ` ${pool.allowed > 0 ? copy.playedOf(typed, num(pool.allowed)) : copy.playedBare(typed)}`,
        )
      : null,
    // Named only once some word has been played twice, which is the same
    // restraint the em dash above shows: "most played: CRANE · once" is true and
    // says nothing, and it is what a player with no habitual opener would see
    // every time, since a run forbids repeating a word and most guesses are
    // therefore one-offs. A favorite is a repetition or it is nothing.
    best && best.count > 1
      ? h(
          "p",
          { class: "stat-line" },
          `${copy.mostPlayed} `,
          h("strong", {}, best.word.toUpperCase()),
          ` · ${copy.times(best.count)}`,
        )
      : null,
    played > 0
      ? h(
          "div",
          { class: "breakdown" },
          h("h3", { class: "stat-head" }, copy.breakdown),
          ...meta.solves.flatMap((count, at) =>
            count > 0 ? [bar(copy.solvedIn(at), count, "won")] : [],
          ),
          meta.missed > 0 ? bar(copy.neverFound, meta.missed, "lost") : null,
          // The percentage that used to sit here is a figure now, so the foot is
          // free to say the thing a distribution structurally cannot: not how
          // often the word comes, but how long it kept coming. It is the only
          // number on this screen with any tension in it. Every other one can
          // just go up.
          h("p", { class: "stat-foot" }, streakLine(meta)),
        )
      : null,
    relics.length > 0
      ? h(
          "div",
          { class: "breakdown" },
          h("h3", { class: "stat-head" }, copy.favoriteRelics),
          ...relics.map((entry) =>
            h(
              "p",
              { class: "stat-line" },
              h("strong", {}, relicCard(entry.id).name),
              ` · ${copy.taken(entry.count)}`,
            ),
          ),
        )
      : null,
    h(
      "button",
      { class: "primary", type: "button", onclick: () => on.closeOverlay() },
      ui().common.close,
    ),
  )
}

/**
 * The streak as a sentence, because it is two numbers that only mean something
 * beside each other.
 *
 * "14" on its own is a boast with no date on it, and "3" on its own is noise.
 * "14 in a row, 3 now" is a player being told how far they are off their own
 * best, which is the only reading that earns the field its place in the record.
 */
export function streakLine(meta: MetaState): string {
  const copy = ui().stats
  if (meta.bestStreak === 0) return copy.noStreak
  // Level with the record and still climbing, so there is no gap to report and
  // reporting one anyway, "14 in a row, 14 now", reads as a bug rather than as
  // the best thing that has happened to this player.
  if (meta.streak === meta.bestStreak) return copy.streakBest(num(meta.streak))
  return meta.streak > 0
    ? copy.streakWithNow(num(meta.bestStreak), num(meta.streak))
    : copy.streak(num(meta.bestStreak))
}

/**
 * The difficulty dial, and the only thing on the title screen that is a decision.
 *
 * Present from the first launch, and open all the way to the top. Hiding the
 * ladder until a win taught the game's best idea to exactly the players who had
 * already finished it; showing it makes ten named rules part of what the game
 * looks like, not a reward for having seen the ending.
 *
 * Above what has been won the rung wears a lock, and the lock yields. The climb
 * is still the intended shape, since each rung is one new rule to learn and the
 * note under an unearned level says so, but a player who wants to open on Tyranny is
 * making a choice about their own evening, and a disabled button is the wrong
 * answer to that. A lock that states its reason and then lets you past is worth
 * more than a `+` that stops responding and explains nothing.
 *
 * The lock is worn by the `+` itself, and only on the rung at the edge of what
 * has been earned. That works out to exactly one rung, since `ahead` is true only
 * when `level === top`, and it is the only step that crosses the line, so it is
 * the only one that is a decision rather than a nudge. Above the line the `+` is
 * a plain `+` again: the player has already answered the question, and asking it
 * once a rung on the way to ascension 12 would be a door that never stops
 * closing.
 *
 * Only the very first rung says in words what beating it would open, under the
 * description where a rung's second sentence belongs. That is the one rung whose
 * player may not know the ladder goes anywhere; from there on the lock carries
 * the whole message, and a sentence repeating it at every rung would be the box
 * selling something the player has already bought.
 *
 * A stepper rather than a list of buttons, because the ladder is ordered and
 * cumulative, so the question is "how far up" and not "which one", and because the
 * ladder no longer has a length a list could have. The level's own rule is
 * spelled out under it; the ones below are named on the intro card of every
 * round, and written out in full in the codex.
 */
function ladder(on: Handlers, meta: MetaState): HTMLElement {
  const copy = ui().ladder
  const top = unlocked(meta)
  const level = chosenAscension(meta)
  const rule = ascensionAt(level)
  const locked = isLocked(meta, level)
  const ahead = level === top && level < MAX_ASCENSION
  // Only ever offered for the first rung. Past that the lock on the `+` says the
  // same thing in a glyph, and the player who has read it once does not need the
  // sentence again at every rung of a ladder twelve rungs long.
  const carrot = ahead && level === 0 ? copy.carrot : null

  const step = (label: string, to: number, live: boolean) =>
    h(
      "button",
      {
        class: "ladder-step",
        type: "button",
        disabled: !live,
        "aria-label": label === "−" ? copy.lower : copy.raise,
        onclick: () => on.setAscension(to),
      },
      label,
    )

  return h(
    "div",
    { class: `ladder ${level > 0 ? "lit" : ""} ${locked ? "locked" : ""}` },
    h(
      "div",
      { class: "ladder-row" },
      step("−", level - 1, level > 0),
      h(
        "div",
        { class: "ladder-level" },
        h(
          "span",
          { class: "ladder-name" },
          // The lock rides the name rather than sitting in the note alone, so
          // the state is legible from the one line the eye lands on first.
          locked && h("span", { class: "ladder-lock", "aria-label": copy.locked }, "🔒"),
          ui().common.ascension(level),
        ),
        // The second line of the dial is "what this rung is", and at zero there
        // is no rule to be, so the slot stays empty. Zero's carrot used to sit
        // here instead, which put the one line the box is trying to say in a
        // different place than every other rung says it.
        rule && h("span", { class: "ladder-rule" }, ascensionCard(rule).name),
      ),
      ahead
        ? h(
            "button",
            {
              class: "ladder-step shut",
              type: "button",
              "aria-label": copy.skipTo(level + 1),
              onclick: () => on.askAscend(level + 1),
            },
            "🔒",
          )
        : step("+", level + 1, level < MAX_ASCENSION),
    ),
    h(
      "p",
      { class: "ladder-text" },
      rule ? `${ascensionCard(rule).text}${level > 1 ? ` ${copy.andBelow}` : ""}` : copy.noRule,
    ),
    // One line under the description, and only ever the forward-looking one:
    // what beating this rung would open. Under the description rather than in the
    // dial because that is where a rung's second sentence goes, and the first
    // rung is not a special case worth reading differently from the rest.
    //
    // The warning that used to displace it here is gone. It said the player had
    // not won yet, in a box whose lock says the same thing in a glyph and whose
    // sheet had just said it in a sentence, and three tellings of "you have not
    // earned this" is the screen holding a grudge about a choice it offered.
    // Above the earned rung there is now no line at all, which is the right
    // amount to say to someone who has already been asked and has already
    // answered.
    carrot ? h("p", { class: "ladder-note" }, carrot) : null,
  )
}

/**
 * The lock, opened.
 *
 * Two lines: what this rung does, and that it is not the recommended way in. A
 * sheet standing between a player and a button they have already decided to
 * press earns its place by being read, and every version of this that argued its
 * case at length was skimmed instead.
 *
 * Gone from it: the line naming the rung that has not been beaten, and the one
 * promising the choice was reversible. The first was true and was the wrong
 * opening. It told the player what they had failed to do before it told them
 * anything they did not know. The second was answering a question nobody asked;
 * this is a stepper on a title screen, and a player who wants to know whether it
 * can be stepped back can step it back. Reassurance nobody needed is still a
 * line to read before the button.
 *
 * "Intended" rather than an argument about tuning, for the same reason. It is
 * what the sentence actually means, a player can weigh it in the time it takes
 * to read, and the button beside it says "anyway", which is the honest name for
 * the thing being pressed and carries the rest of the warning on its own. It is
 * also the word the rules sheet already uses for climbing a rung at a time, and
 * it puts the climb in the subject where "it is recommended to win" put nobody at
 * all, in the one sentence in the game that spoke like a form rather than to a
 * player.
 *
 * No disabled state, no delay, no second confirmation. The game already decided
 * a player may start anywhere, and a sheet that grudges the permission it is
 * granting is worse than no sheet.
 *
 * The rule of the rung being stepped onto is quoted rather than only its number,
 * because "ascension 8" means nothing to a player who has seen three of them and
 * "Four relic slots, not five" means something to anyone. It is the one piece of
 * information that turns this from a dare into a decision.
 *
 * It is only ever this one rung. The lock sits at the edge of what was earned,
 * so the leap it grants is always a single step, and past the edge the stepper
 * is plain again, and a player who has crossed the line does not get asked about it
 * once per rung on the way to ascension 40.
 */
export function ascendView(level: number, on: Handlers): HTMLElement {
  const copy = ui().ladder
  const rule = ascensionAt(level)
  return overlay(
    on,
    h("h2", { class: "sheet-title" }, copy.askTitle(level)),
    h(
      "div",
      { class: "sheet-body" },
      rule
        ? h(
            "p",
            { class: "sheet-lead" },
            h("strong", {}, `${copy.ruleLabel(ascensionCard(rule).name)} `),
            ascensionCard(rule).text,
            level > 1 ? ` ${copy.askAndBelow}` : "",
          )
        : null,
      h("p", {}, copy.intended),
    ),
    h(
      "div",
      { class: "sheet-actions" },
      h(
        "button",
        { class: "danger", type: "button", onclick: () => on.ascend() },
        copy.skipAnyway(level),
      ),
      h(
        "button",
        { class: "primary", type: "button", onclick: () => on.closeOverlay() },
        ui().common.back,
      ),
    ),
  )
}

/**
 * What this bundle actually is, in the one place a player will look for it.
 *
 * Both halves are inlined at build time; see vite.config.ts. The version comes
 * from package.json and the hash from the commit, and the hash is the one that
 * can be trusted: Pages redeploys on every push, but the version bump is its own
 * commit landing after the change it names, so for the gap between them the site
 * serves new code under the old number. Where there is no hash to be had the
 * version stands alone rather than trailing a bare separator.
 */
function buildStamp(): string {
  const version = `v${__BUILD_VERSION__}`
  return __BUILD_COMMIT__ ? `${version} · ${__BUILD_COMMIT__}` : version
}

/* -------------------------------------------------------------- overlays */

/**
 * What every sheet wears, whichever shell built it.
 *
 * `tabindex="-1"` is the load-bearing one: it is what lets focus be *put* on the
 * sheet when it opens without putting the sheet in the tab order itself, which
 * is how the keyboard gets inside a thing that was appended to the document
 * rather than navigated to. The two ARIA attributes say the same thing to a
 * screen reader that the backdrop says to everyone else: nothing behind this is
 * available until it is dealt with.
 */
export const SHEET_ATTRS = { role: "dialog", "aria-modal": "true", tabindex: -1 }

/**
 * Everything modal shares this shell. The backdrop closes on tap, which on a
 * phone is the gesture people reach for before they look for a button.
 */
function overlay(on: Handlers, ...body: (HTMLElement | string | false | null)[]): HTMLElement {
  return h(
    "div",
    { class: "overlay", onclick: () => on.closeOverlay() },
    h(
      "div",
      {
        class: "sheet",
        ...SHEET_ATTRS,
        // Taps inside the sheet are for the sheet; without this every button
        // press would also dismiss the thing it was pressed in.
        onclick: (event: Event) => event.stopPropagation(),
      },
      ...body,
    ),
  )
}

/**
 * One titled paragraph: the term in bold, then the sentence.
 *
 * The space between the two is emitted here rather than carried at the head of
 * every catalog entry, which is where it used to live. A leading space is
 * invisible in the source, and asking four translators to re-type one correctly
 * fourteen times is a diff nobody can review.
 */
const rule = ({ term, text }: Rule) =>
  h("div", { class: "rule" }, h("strong", {}, term), h("span", {}, ` ${text}`))

/** The same, for the paragraphs that quote a number the content files own. */
const ruleOf = <A extends unknown[]>({ term, text }: RuleOf<A>, ...args: A) =>
  rule({ term, text: text(...args) })

export function helpView(on: Handlers): HTMLElement {
  const copy = ui().help
  return overlay(
    on,
    h("h2", { class: "sheet-title" }, copy.title),
    h(
      "div",
      { class: "sheet-body" },
      h("p", {}, copy.lead),
      h("p", { class: "sheet-lead" }, copy.scored),
      rule(copy.chipsMult),
      rule(copy.letters),
      rule(copy.colors),
      rule(copy.solving),
      h("p", { class: "sheet-lead" }, copy.farming),
      rule(copy.solveLine),
      h("h3", { class: "sheet-heading" }, copy.runHeading),
      ruleOf(copy.target, STAGES, ROUNDS_PER_STAGE),
      ruleOf(copy.endless, STAGES),
      rule(copy.bosses),
      ruleOf(copy.ascensions, AUTHORED_ASCENSIONS),
      // Every figure arrives already spelled as money, so the sentence never has
      // to know where the `$` goes in the language it is being read in.
      ruleOf(
        copy.money,
        ROUND_PAYOUT.map(money).join(" / "),
        money(GOLD_PER_UNUSED_GUESS),
        money(INTEREST_PER),
        money(INTEREST_CAP),
      ),
      ruleOf(copy.relics, RELIC_SLOTS),
      rule(copy.packs),
      rule(copy.mods),
      // The list of what each one does lives in the codex rather than here. Two
      // screens restating the same table is how the two of them drift apart, and
      // this one is meant to be read once.
      h("p", { class: "sheet-lead" }, copy.codexNote),
    ),
    h(
      "div",
      { class: "sheet-actions" },
      h(
        "button",
        { class: "secondary", type: "button", onclick: () => on.openCodex() },
        copy.openCodex,
      ),
    ),
    h("button", { class: "primary", type: "button", onclick: () => on.closeOverlay() }, copy.gotIt),
  )
}

/* ----------------------------------------------------------------- codex */

/** One catalog entry: what it is called, what it does, and what it costs. */
function entry(name: string, text: string, note?: string): HTMLElement {
  return h(
    "div",
    { class: "codex-entry" },
    h(
      "div",
      { class: "codex-entry-head" },
      h("strong", {}, name),
      note ? h("span", { class: "codex-note" }, note) : null,
    ),
    h("span", { class: "codex-text" }, text),
  )
}

/**
 * One collapsible section, closed until asked for.
 *
 * Everything in the codex laid out flat runs to ten phone screens of scrolling,
 * which makes the catalog useless for the thing it is actually for, which is looking
 * one card up, mid-run, having forgotten what it does. Closed, the whole screen
 * is seven rows and a count, and the count is worth reading on its own: it is
 * the size of the game, stated.
 */
function section({ title, blurb }: Section, count: number, ...rows: HTMLElement[]): HTMLElement {
  return h(
    "details",
    { class: "codex-section" },
    h(
      "summary",
      { class: "codex-summary" },
      h("span", {}, title),
      h("span", { class: "codex-count" }, String(count)),
    ),
    h("p", { class: "codex-blurb" }, blurb),
    ...rows,
  )
}

/** The same, for the two sections whose blurb quotes a slot count. */
const sectionOf = <A extends unknown[]>(
  { title, blurb }: SectionOf<A>,
  count: number,
  args: A,
  ...rows: HTMLElement[]
) => section({ title, blurb: blurb(...args) }, count, ...rows)

const RARITIES: readonly Rarity[] = ["common", "uncommon", "rare", "legendary"]

/**
 * The catalog. Every section maps over the table it describes rather than
 * restating it, which is the same trick, and the same reason, as the modifier
 * list that used to live in `helpView`: a reference that is written out by hand
 * is a reference that goes stale the first time a number moves.
 *
 * Fully revealed rather than unlocked by discovery. The player who most needs to
 * read what The Auditor does is the one who has not met it yet, and a codex that
 * only tells you what you already know is a trophy cabinet, not a reference.
 *
 * Takes no run state on purpose: this is the catalog, not the board. A scaling
 * relic shows its rule here, never the number it happens to be holding.
 */
/**
 * The five word shapes, what they are worth, and which of them the word on the
 * board is.
 *
 * The system was invisible before this. One line under the grid named the shape
 * in play; nothing named the other four, nothing listed the levels a run had
 * bought, and nothing anywhere stated the rule that decides which shape a word
 * scores as. `categoryOf` returns the *rarest* match, so a word that is both
 * Twinned and Vowel Heavy scores as Vowel Heavy and a player leveling Twinned
 * would watch it not pay and never learn why.
 *
 * So the panel says all three things at once: every shape a word matches gets a
 * tag, the one that counts gets a louder one, and the list is in the order the
 * engine checks it in. The rule is not written down anywhere the player has to
 * be told it: it is the order of the rows.
 */
export function shapesView(state: RunState, on: Handlers, word: string): HTMLElement {
  const copy = ui().shapes
  const scoring = word ? categoryOf(word) : null

  return overlay(
    on,
    h("h2", { class: "sheet-title" }, copy.title),
    h(
      "p",
      { class: "sheet-lead" },
      scoring ? copy.scoresAs(word, categoryCard(scoring.id).name) : copy.anyWord,
    ),
    h("p", { class: "shapes-note" }, copy.note),
    h(
      "div",
      { class: "shapes" },
      ...CATEGORIES.map((category) => {
        const bonus = levelBonus(state, category)
        const card = categoryCard(category.id)
        const scores = scoring?.id === category.id
        const matched = !scores && word !== "" && category.matches(word)
        return h(
          "div",
          { class: `shape ${scores ? "scoring" : ""} ${matched ? "matched" : ""}` },
          h(
            "div",
            { class: "shape-head" },
            h("strong", {}, card.name),
            scores ? h("span", { class: "shape-tag" }, copy.scoring) : null,
            matched ? h("span", { class: "shape-tag also" }, copy.alsoMatches) : null,
            h("span", { class: "shape-level" }, ui().board.shapeLevel(bonus.level)),
          ),
          h("span", { class: "shape-text" }, card.text),
          h(
            "span",
            { class: "shape-pay" },
            bonus.chips > 0
              ? copy.payNow(bonus.chips, bonus.mult)
              : copy.payPerLevel(category.chips, category.mult),
          ),
        )
      }),
    ),
    h(
      "button",
      { class: "primary", type: "button", onclick: () => on.closeOverlay() },
      ui().common.close,
    ),
  )
}

export function codexView(on: Handlers): HTMLElement {
  const copy = ui().codex
  return overlay(
    on,
    h("h2", { class: "sheet-title" }, copy.title),
    h(
      "div",
      { class: "sheet-body" },
      h("p", { class: "sheet-lead" }, copy.lead),

      sectionOf(
        copy.relics,
        RELICS.length,
        [RELIC_SLOTS],
        // The rarity class goes on the wrapper rather than the header, so the
        // band's color reaches the cards under it the same way it reaches a
        // relic in the tray.
        ...RARITIES.flatMap((rarity) => {
          const relics = RELICS.filter((relic) => relic.rarity === rarity)
          if (relics.length === 0) return []
          return [
            h(
              "div",
              { class: `codex-band rarity-${rarity}` },
              h("div", { class: "codex-group" }, copy.rarity[rarity]),
              ...relics.map((relic) => {
                const card = relicCard(relic.id)
                return entry(card.name, card.text, money(relic.cost))
              }),
            ),
          ]
        }),
      ),

      section(
        copy.bosses,
        BOSSES.length,
        ...BOSS_TIERS.map((tier) =>
          h(
            "div",
            { class: "codex-band" },
            h(
              "div",
              { class: "codex-group" },
              copy.tierBand(copy.tier[tier], TIER_STAGES[tier].first, TIER_STAGES[tier].last),
            ),
            ...bossesIn(tier).map((boss) => entry(bossCard(boss.id).name, bossCard(boss.id).text)),
          ),
        ),
      ),

      section(
        copy.ascensions,
        ASCENSIONS.length,
        ...ASCENSIONS.map((rule) => {
          const card = ascensionCard(rule)
          return entry(card.name, card.text, ui().common.ascension(rule.level))
        }),
      ),

      section(
        copy.shapes,
        CATEGORIES.length,
        ...CATEGORIES.map((category) => {
          const card = categoryCard(category.id)
          return entry(card.name, card.text, copy.shapePer(category.chips, category.mult))
        }),
      ),

      section(
        copy.mods,
        MODIFIERS.length,
        ...MODIFIERS.map((mod) => {
          const card = modifierCard(mod.id)
          return entry(
            copy.modName(card.name, mod.pip),
            mod.letters
              ? copy.modTextOnly(card.text, [...mod.letters].join(" ").toUpperCase())
              : copy.modText(card.text),
            `${money(mod.choiceCost)} / ${money(mod.cost)}`,
          )
        }),
      ),

      section(
        copy.upgrades,
        ETCHINGS.length + RANGES.length,
        ...ETCHINGS.map((etching) => {
          const card = etchingCard(etching)
          return entry(card.name, card.text, money(etching.cost))
        }),
        ...RANGES.map((range) => entry(range.name, rangeText(range))),
      ),

      sectionOf(
        copy.consumables,
        CONSUMABLES.length,
        [CONSUMABLE_SLOTS],
        ...CONSUMABLES.map((consumable) => {
          const card = consumableCard(consumable.id)
          return entry(card.name, card.text, money(consumable.cost))
        }),
      ),

      section(
        copy.packs,
        PACKS.length,
        // "Choose one of three" already says how many you keep, so the count is
        // only worth printing on a pack that keeps more than one, which none
        // do yet, and which is exactly why it is derived rather than assumed.
        ...PACKS.map((pack) => {
          const card = packCard(pack.id)
          return entry(
            card.name,
            pack.picks > 1 ? copy.packTextPicks(card.text, pack.picks) : copy.packText(card.text),
            money(pack.cost),
          )
        }),
      ),
    ),
    h(
      "button",
      { class: "primary", type: "button", onclick: () => on.closeOverlay() },
      ui().common.close,
    ),
  )
}

export function menuView(on: Handlers, chrome: Chrome): HTMLElement {
  const copy = ui().pause
  const common = ui().common
  return overlay(
    on,
    h("h2", { class: "sheet-title" }, copy.title),
    h(
      "div",
      { class: "sheet-actions" },
      h(
        "button",
        { class: "secondary", type: "button", onclick: () => on.mute() },
        chrome.muted ? common.soundOff : common.soundOn,
      ),
      // Music gets its own switch rather than riding on the sound one: it plays
      // continuously, so it is the thing a player is most likely to want gone
      // while keeping the feedback that tells them what their guess scored.
      //
      // Muting sound silences it too, and the switch goes dead rather than
      // sitting there reading "Music on" over silence.
      h(
        "button",
        {
          class: "secondary",
          type: "button",
          disabled: chrome.muted,
          onclick: () => on.toggleMusic(),
        },
        chrome.muted || chrome.musicOff ? copy.musicOff : copy.musicOn,
      ),
      // How fast the game plays what it has to say, and unlike the letter values
      // below it, this one belongs on the sheet. The argument that moved the
      // decoration switch onto the board was that its effect could not be seen
      // from the screen it was set on; the answer here is that it cannot be seen
      // from *any* screen while it is being set, because there is nothing to
      // watch until the next guess scores. A control that can only be judged
      // after the sheet is closed may as well be where the other settings are.
      //
      // It reads as the multiplier it is rather than as a name for one. "Brisk"
      // needs a legend and invites a second opinion about what brisk means;
      // "×2" is the whole of the arithmetic, and a player who wants the cascade
      // out of the way can tap until the number is big enough.
      h(
        "button",
        { class: "secondary", type: "button", onclick: () => on.cycleSpeed() },
        copy.speed(chrome.speed),
      ),
      languageButton(on, chrome),
      // The honest footnote, on screen only while it is true. Changing the
      // language repaints every sentence here immediately and does not touch the
      // run: a run is dealt from one word list and keeps it, so the answer stays
      // in the language it was drawn from. Said here and not on the title
      // screen, because `wordsDeferred` needs a run open to be true at all.
      chrome.wordsDeferred ? h("p", { class: "lang-note" }, copy.wordsNextRun) : null,
      // Letter values are not here. The board is dense by design, with a value on
      // every key and a pip on every modifier, and that density is the scoring
      // game asking to be played; some of the time the player is doing the other
      // thing entirely, which is working out a five-letter word, and every
      // number on screen is noise. The switch for it is `decorToggle`, sitting
      // beside the chips × mult readout, because it was the one setting on this
      // sheet whose whole effect was hidden behind the sheet while it was being
      // set, and the readout is the densest numbers on the screen, so the
      // switch is next to the thing it is loudest about. That it has three
      // states now is a further reason to leave it there: a segmented control
      // here would be the readable way to show them, and it would show them on
      // top of the board the reader needs to see to choose between them.
      h(
        "button",
        { class: "secondary", type: "button", onclick: () => on.openHelp() },
        common.howToPlay,
      ),
      h(
        "button",
        { class: "secondary", type: "button", onclick: () => on.openCodex() },
        common.codex,
      ),
      h("button", { class: "danger", type: "button", onclick: () => on.askQuit() }, copy.quit),
    ),
    h(
      "button",
      { class: "primary", type: "button", onclick: () => on.closeOverlay() },
      copy.resume,
    ),
  )
}

/**
 * Quitting is the one irreversible button in the game (the save is deleted and
 * a run is an hour of decisions), so it asks, and the confirmation is worded as
 * what is lost rather than as a yes/no.
 */
export function quitView(state: RunState, on: Handlers): HTMLElement {
  const copy = ui().quit
  const round = roundName(state.roundIndex)
  return overlay(
    on,
    h("h2", { class: "sheet-title" }, copy.title),
    h(
      "div",
      { class: "sheet-body" },
      // Two whole sentences in the catalog rather than one with "of N" spliced
      // in, because a run past the last stage has no denominator and the clause
      // a language wants that number in is not always the one English puts it in.
      h(
        "p",
        {},
        state.won ? copy.bodyEndless(state.stage, round) : copy.body(state.stage, STAGES, round),
      ),
    ),
    h(
      "div",
      { class: "sheet-actions" },
      h("button", { class: "danger", type: "button", onclick: () => on.quit() }, copy.confirm),
      h("button", { class: "primary", type: "button", onclick: () => on.openMenu() }, copy.cancel),
    ),
  )
}
