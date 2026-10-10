import type { GuessRecord, Modifier, Rarity, RunState, ShopItem } from "../engine"
import {
  ASCENSIONS,
  AUTHORED_ASCENSIONS,
  ascensionAt,
  BOSS_TIERS,
  BOSSES,
  baseChips,
  bossesIn,
  bossForStage,
  CATEGORIES,
  CONSUMABLE_SLOTS,
  CONSUMABLES,
  categoryOf,
  DISTINCT,
  difficultyOf,
  draftChips,
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
  PACKS,
  placeableLetters,
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
  roundTarget,
  rulesFor,
  STAGES,
  sellValue,
  solveBonusFor,
  TIER_STAGES,
} from "../engine"
import { withAmounts } from "./amounts"
import { describeItem, rangeText } from "./cards"
import type { CoachStep } from "./coach"
import { h } from "./dom"
import { emblem } from "./emblems"
import { emojied } from "./emoji"
import { money, formatNumber as num } from "./format"
import { type IconName, icon, roundToken, type TokenKind } from "./icons"
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
  lang,
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
import { SITE_URL } from "./seed"
import type { Skin } from "./skin"
import type { Speed } from "./speed"
import { sprite } from "./sprites"
import { isTable } from "./table"

export type Handlers = {
  key: (letter: string) => void
  enter: () => void
  back: () => void
  useConsumable: (index: number) => void
  collect: () => void
  buy: (index: number) => void
  sell: (index: number) => void
  drop: (index: number) => void
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
  /** Step down from sound with music, to sound, to off. See `NEXT_SOUND`. */
  cycleSound: () => void
  /** The effects alone. The music has its own switch and ignores this one. */
  toggleEffects: () => void
  toggleMusic: () => void
  /** Step to the other recording, from its top. */
  nextTrack: () => void
  /** The sharing switch, on the about sheet or the pause sheet. */
  setSharing: (on: boolean) => void
  /** Step the board down a level of decoration, wrapping back to all of it. */
  cycleDecor: () => void
  /** Step the animations up a rung, wrapping back to the speed they are drawn at. */
  cycleSpeed: () => void
  /** Move the look to the next of the four, wrapping round. */
  cycleSkin: () => void
  /** Step the interface to the next language, wrapping back to English. */
  cycleLanguage: () => void
  openMenu: () => void
  openHelp: () => void
  /**
   * Start the first round without the coaching, now and for good. Offered once,
   * on that round's intro card, beside the button that starts it with them.
   */
  skipCoach: () => void
  /**
   * A fresh run straight onto the first round's board with the coaching on,
   * past the intro card that would only ask again. From the rules sheet's note;
   * see `helpView`.
   */
  startTutorial: () => void
  openCodex: () => void
  /** The shape panel, from the board or from the shop. */
  openShapes: () => void
  /** The long record, from the title screen's one-line version of it. */
  openStats: () => void
  openAbout: () => void
  openCredits: () => void
  closeOverlay: () => void
  askQuit: () => void
  quit: () => void
  /** The lock on the ladder, opened far enough to read what is behind it. */
  askAscend: (level: number) => void
  /** Past the lock, having read it. */
  ascend: () => void
  /** The seed sheet, from the title screen's version stamp. */
  openSeed: () => void
  /**
   * The seed field's text, kept by the app on every keystroke *without* a
   * render: the render would rebuild the field under the caret. The sheet
   * reads it back into the field's `value` whenever it is rebuilt for another
   * reason.
   */
  editSeed: (text: string) => void
  /** Deal the run the field names, after the field says it names one. */
  playSeed: () => void
  /** A link to the run in hand, onto the clipboard. */
  copySeed: () => void
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
  /**
   * The run in hand's seed, as its code, for the pause sheet's line. Null where
   * there is no run of the player's to share: the title screen's scaffolding,
   * and a spectator's view of someone else's.
   */
  seed: string | null
  sound: SoundLevel
  /** The two switches `sound` is read from, for the pause sheet's rows. */
  effectsOff: boolean
  musicOff: boolean
  /** The chosen recording's title, which no language translates. */
  track: string
  decor: Decor
  speed: Speed
  /** The look on screen: a pick, or the default the window gave. */
  skin: Skin
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
  /**
   * Whether run replays are shared, `off` until the player turns it on, and
   * null when this build has nowhere to send them, in which case the switch is
   * not drawn. See `./telemetry`.
   */
  sharing: "on" | "off" | null
  /**
   * Sharing was just turned on, and the screen it was turned on from says
   * thanks. `fresh` for the render that answers the tap, which is the one the
   * heart pops on; `shown` for any rebuild after it, which keeps the line and
   * drops the pop, since tapping music under it is not a second yes.
   */
  thanked: "fresh" | "shown" | null
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
 * It sits at the foot of the screen, beside the hand and over the keys, because
 * the keys are most of what it governs, and within reach of the thumb already
 * on them. It sat beside the chips × mult readout while that was down there
 * too, and stayed behind when the readout went up into the header: a control
 * in the header is a reach across the screen, and the header is for reading.
 * The table is the exception: there is no thumb on the keys, the rail's foot is
 * the one row of buttons it has, and the switch sits there beside the gold and
 * the ☰, a square like it. See `hud`.
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

/**
 * How much the game says out loud, as the title screen's speaker shows it.
 *
 * Underneath are two switches, the effects and the music, and the pause sheet
 * shows them as two. The speaker is the shortcut: one button with no room for a
 * label, which cycles the three levels a player reaching for it wants: effects
 * with music, effects, off.
 *
 * It counts down, as the decoration cycle does, and from the top because the
 * music is on by default. It climbed at first, off to effects to music, and that
 * order put "no music" behind "no sound": the player most likely to reach for the
 * speaker is the one who wants the piano gone, and their first tap took the
 * clacks with it. Counting down, every tap takes away one layer, and the first
 * one takes away only the music, so the middle level is found by the tap that
 * wanted it rather than looked for.
 *
 * The fourth pair, music with the effects off, is reachable from the sheet
 * alone. The speaker still has to draw it, and it draws the note, since music is
 * what is playing, and steps it to off: one layer away, as from anywhere else.
 * It is not a stop on the cycle, because a fourth stop on a button with no label
 * is one the player has to count past.
 */
export type SoundLevel = "off" | "sound" | "music" | "musicOnly"

export const NEXT_SOUND: Record<SoundLevel, SoundLevel> = {
  music: "sound",
  sound: "off",
  off: "music",
  musicOnly: "off",
}

/**
 * The round's header: the shop's bar, seat for seat, with the score in the seat
 * the shop leaves empty, so the gold and the menu do not move between the two.
 *
 * A two-row header was tried with this redesign, the score large on a row of
 * its own over a full-width bar, and it cost the board more than it gave the
 * score: 30px of header is 5px off every tile when the board is bound by
 * height, which on most phones it is.
 *
 * The dock line, the shape and the chips × mult being typed, is the header's
 * third row, under the bar: the hand beside the pile it is about to join. See
 * `.hud .dock-line`.
 *
 * The boss's rule is the header's last row when there is one, rather than a
 * banner of its own below. It is a fact about the round, like the name beside
 * the score, and inside the header its height is a known quantity the board can
 * be told about, which is what keeps a boss round's tiles the size of every
 * other round's. See `--boss-band`.
 *
 * The table's rail is a column rather than a bar, and two things change shape
 * there. The round and its boss are one card, `.hud-head`, the rule continuing
 * under the name rather than sitting at the rail's foot a screen away from it,
 * and the card holds a boss card's height in every round, so the score starts
 * at the same height whether or not there is a rule above it. And `decor`, the
 * letter-values switch, joins the gold and the ☰ in the foot, which is the only
 * row the rail has for buttons now that sound, music and speed went back to
 * the pause sheet. On a phone it stays by the keys and this gets `null`. Asked
 * of `isTable()` at render, as `roundView` and `shopView` ask it.
 */
function hud(
  state: RunState,
  on: Handlers,
  dock: HTMLElement,
  coach: HTMLElement,
  decor: HTMLElement | null,
): HTMLElement {
  const round = state.round
  const board = ui().board
  const boss = getBoss(round.bossId)
  const table = isTable()
  const title = h(
    "div",
    { class: "hud-round" },
    // The intro card's token at the height of the title beside it, so the round
    // keeps the shape it was announced with once the card has gone.
    roundToken(tokenOf(state)),
    h(
      "div",
      { class: "hud-title" },
      h("div", { class: "round-name" }, roundName(state.roundIndex)),
      stageLine(state),
    ),
  )
  // Gold and menu as one group, pinned to the right edge. Loose in a
  // space-between row the gold took whatever gap was left over, which was a
  // different gap on every screen (x282 here, x253 in the shop at 390) and
  // never beside anything it belonged to. See `.hud-end`.
  const end = h(
    "div",
    { class: "hud-end" },
    h("div", { class: "hud-gold" }, money(state.gold)),
    decor,
    menuButton(on),
  )
  const rule =
    boss &&
    h(
      "div",
      // No tip. It carried the whole rule, for the two (The Silence in French
      // and in German) that take a third line on a 320px screen where the
      // band clamps at two, and everywhere else it said again what the band
      // already says, over the band, on every pass of the pointer. The rule
      // is also on the round's intro card and in the rules sheet, which is
      // where the clipped two can be read whole.
      { class: "boss" },
      // One paragraph inside the band rather than the band's own text, so the
      // band can hold a fixed height and centre whatever length of rule it
      // was handed. See `--boss-band`.
      h("p", {}, h("strong", {}, bossCard(boss.id).name), ` ${bossCard(boss.id).text}`),
    )
  return h(
    "header",
    { class: "hud" },
    table ? h("div", { class: "hud-head" }, title, rule) : title,
    h(
      "div",
      {
        class: `hud-score ${round.score >= round.target ? "met" : ""}`,
        // On the block and not on the bar, which is five pixels tall and no
        // target for a thumb to hold.
        "data-tip": solveSaid(state),
      },
      // One line, the score and what it is out of on a shared baseline, so the
      // seat is as short as the name beside it.
      h(
        "div",
        { class: "score-line" },
        h("div", { class: "score" }, num(round.score)),
        h("div", { class: "target" }, board.target(num(round.target))),
      ),
    ),
    table ? null : end,
    // The same fact as the two numbers in the seat, in the form a glance can
    // take in, and the header's full width. A sibling of the seat rather than
    // its child so it can cross all three tracks; see `.round-screen .hud
    // .meter`. The scoring animation drives it frame by frame, so it fills as
    // the total climbs rather than jumping to the answer.
    meter(state),
    dock,
    table ? null : rule,
    coach,
    // Last in the DOM on the table, where it is last on screen too: the rail's
    // foot. Tab follows the DOM and not the flex `order` that draws the rail,
    // so built in the bar's seat it put the ☰ and the switch ahead of the shape,
    // which is the rail's first control on screen and the one a keyboard
    // player is reaching for. On a phone the ☰ is the first control on screen,
    // top right, and stays first in Tab as well.
    table ? end : null,
  )
}

/**
 * Which token a round wears. Three rounds, three shapes, and the shape carries
 * the warning before the name is read, which matters most for the one that
 * changes the rules. By position rather than by boss, for the track on the
 * intro card, which draws rounds that have not been dealt yet.
 */
const TOKEN_AT: readonly TokenKind[] = ["normal", "elite", "boss"]

function tokenOf(state: RunState): TokenKind {
  return getBoss(state.round.bossId) ? "boss" : (TOKEN_AT[state.roundIndex] ?? "normal")
}

/** The stage under the round's name, on the round screen and the shop's. */
function stageLine(state: RunState): HTMLElement {
  const board = ui().board
  return h(
    "div",
    { class: "stage" },
    state.won ? board.stageEndless(state.stage) : board.stage(state.stage, STAGES),
    // The terms the whole run is being played under, in the space of two
    // characters. It shares its color with the boss banner because it is
    // the same kind of fact, something bending what a guess may be, and
    // the rules it stands for are named in full on every intro card.
    state.ascension ? h("span", { class: "stage-asc" }, board.ascensionTag(state.ascension)) : null,
  )
}

/** Shared with the animation controller, so the bar and the number agree. */
/**
 * The header's bar, and behind its fill, fainter, how far solving on the next
 * guess would take it.
 *
 * That second length was a chip under the board, "×5 → 150", with a check when
 * it cleared. It came up here because the board's line wanted the room (the
 * shape name was cut to an ellipsis at 360 with it there) and because the
 * question it answers, "would solving now win this", is a question about the
 * bar. It was here once before, green, and was cut for reading as a second
 * score. So it is drawn in the fill's own colour at a fraction of its strength:
 * a projection of the blue rather than a rival to it, and no green, which the
 * bar keeps for the round actually won.
 *
 * The figures the chip printed are the score block's tip and this bar's label,
 * which is where a screen reader was always going to read them from.
 */
function meter(state: RunState): HTMLElement {
  const round = state.round
  const solve = round.score > 0 ? solveFloor(state) : null
  const said = solveSaid(state)
  return h(
    "div",
    { class: "meter", ...(said && { role: "img", "aria-label": said }) },
    solve &&
      h("div", { class: "meter-solve", style: `--fill:${meterFill(solve.floor, round.target)}` }),
    h("div", { class: "meter-fill", style: `--fill:${meterFill(round.score, round.target)}` }),
  )
}

/** "solve ×5 → 150", or nothing while the pile is empty and there is no floor. */
function solveSaid(state: RunState): string | undefined {
  const round = state.round
  const solve = round.score > 0 ? solveFloor(state) : null
  if (!solve) return undefined
  const board = ui().board
  const floor = (solve.floor >= round.target ? board.solveFloorClears : board.solveFloor)(
    num(solve.floor),
  )
  return `${board.solveFactor(solve.factor)} ${floor}`
}

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
    // On the face the figure goes after the name, on the name's line, and in
    // the colour of what it adds to rather than with its unit word. A row of its
    // own under a sprite and a name came to 51px in a 40px seat, so the figure
    // was the thing cut off, on the one card in the tray worth re-reading
    // between rounds. The colour is the readout's own (blue points, red mult,
    // gold money) and so already a word the player reads; the unit is in full
    // in the tip and the label, which is where `detail` still goes.
    //
    // The sign stays, though it costs the name its last letters: without it
    // Hot Streak fit its 66px line in Tabletop's face, with it the name ends
    // "Hot Str…". A bare "12" beside a name reads as a fixed stat, which is
    // wrong twice over for the two relics whose figure is a pool counting
    // *down*, and with the unit word gone the `+` is the one cue left that is
    // not a colour. The name is in full in the tip.
    const figure =
      growth && (growth.unit === "gold" ? `+${money(growth.amount)}` : `+${num(growth.amount)}`)
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
      ...cardArt("relic", instance.id),
      growth && figure
        ? h(
            "span",
            { class: "relic-line" },
            h("span", { class: "relic-name" }, card.name),
            h("span", { class: `relic-detail amt-${growth.unit}` }, figure),
          )
        : h("span", { class: "relic-name" }, card.name),
    )
  })
  return h("div", { class: "relics" }, ...slots)
}

/**
 * The hand, drawn as its seats whether or not anything is in them, for the
 * tray's reason and one of the board's own. It was no row at all while the hand
 * was empty, and the board is whatever the chrome leaves, so buying a card in
 * the shop moved every tile of the next round down and shrank it by the row's
 * height, and playing the last one moved them back, mid-round, under the thumb.
 * An empty seat costs the ordinary round the row's height, the same trade the
 * boss band makes, and at the relic tray's height; see `--tray-h`.
 */
function consumableRow(state: RunState, on: Handlers): HTMLElement {
  const seats = Array.from({ length: CONSUMABLE_SLOTS }, (_, index) => {
    const instance = state.consumables[index]
    if (!instance) return h("div", { class: "consumable empty" })
    const card = consumableCard(instance.id)
    return h(
      "button",
      {
        class: "consumable",
        type: "button",
        // The rule is clamped to the band's one line, which every rule in the
        // English catalog outruns at 390px, so the whole of it is a press away.
        "data-tip": card.text,
        // Seated by index: the hand closes up behind a played card, so the name
        // lands on the one that slid into its place. See `holdFocus`.
        "data-focus": `consumable-${index}`,
        onclick: () => on.useConsumable(index),
      },
      // In a wrapper that is nothing (`display: contents`) except on a phone,
      // where the two run on as one clamped paragraph; see `consumables.css`.
      h(
        "span",
        { class: "consumable-face" },
        h("span", { class: "consumable-name" }, card.name),
        h("span", { class: "consumable-text" }, ...withAmounts(card.text)),
      ),
    )
  })
  return h("div", { class: "consumables" }, ...seats)
}

/* --------------------------------------------------------------- the round */

function grid(state: RunState): HTMLElement {
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
  )
}

/**
 * Where the first round explains itself: hung from the foot of the header,
 * directly under the figures it is talking about.
 *
 * Everything the card names is in the header, the chips × mult readout, the
 * score and its bar, and a sentence about "the ?" wants to be read beside the
 * `?`. It lay on the board before this, first pinned to the board's foot, from
 * when the readout was under the board and the foot was the nearest edge to
 * it, then hung under the row being typed, and both left a gap between the card
 * and its referent that grew with every guess: the board's top edge is a band's
 * reservation and a row or three away from the header, and the card read as a
 * note about the board rather than about the numbers.
 *
 * A child of the header rather than of the screen, so it is positioned against
 * the box that holds every anchor, and hung from the anchor's own foot; see
 * `aimCoach`. Under the readout it lies over the board's top edge, which is the
 * band's reservation on a round without a boss (the tutorial's round never has
 * one) and the first row's top edge on a narrow phone where a card runs to
 * three lines. Under the score or the bar it lies over the header's lower lines.
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
  // The same beat as the card already up is a new figure, not a new card, so
  // it is rewritten where it stands. Rebuilt, the node was new on every letter
  // and replayed `coach-in` each time, and the rare-letters card, the one that
  // counts chips as they are typed, blinked and dropped in again per keystroke
  // on the very line the player was reading. A change of beat still builds a
  // fresh card, because that is a new sentence and the entry says so.
  const live = slot.querySelector<HTMLElement>(".coach")
  if (coach && live?.dataset.step === coach.id) {
    const text = live.querySelector(".coach-text")
    if (text) text.textContent = coach.text
    return
  }
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
    // leads with. A boss that bends chips does it per column (The Margin), by
    // what has already been spent (The Miser) or by the whole word (The
    // Cliché), and one number sitting on a key cannot say any of them without
    // lying about the rest; the tip has room for the sentence and prints it.
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
            {
              class: "key wide",
              type: "button",
              "aria-label": ui().board.del,
              onclick: () => on.back(),
            },
            icon("backspace"),
          ),
      ),
    ),
  )
}

/**
 * What solving on the next guess is guaranteed to bank, or nothing when there
 * is no next guess to solve on or it would multiply by less than one.
 *
 * Drawn as the header bar's faint projection; see `meter`. The tutorial's
 * solve card quotes the same figure, so the sum it shows is the bar's length.
 */
export function solveFloor(state: RunState): { factor: number; floor: number } | null {
  const round = state.round
  const factor = solveBonusFor(state, round.maxGuesses - round.guesses.length - 1)
  if (round.done || round.guesses.length >= round.maxGuesses || factor < 1) return null
  return { factor, floor: Math.round(round.score * factor) }
}

/**
 * What shape of word is on the board, and what that shape is currently worth.
 *
 * Balatro names the hand you have selected before you play it, and that label is
 * how a player learns the hand list without being taught it. This is the same
 * job: a category at level one pays nothing, so without this line there would be
 * no way to discover the system exists until after buying into it.
 *
 * Reads the draft once it is a full word, and otherwise the last guess. The
 * shape chip and the shapes sheet both ask this, so the two never disagree
 * about which word they are describing.
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
 * The slot is always in the document because the draft row is patched in place
 * rather than re-rendered (see `patchDraft`), and this line has to keep up with
 * it: the patch refills it and never has to know where to reinsert it.
 */
function categorySlot(state: RunState, on: Handlers): HTMLElement {
  const slot = h("div", { class: "category-slot" })
  fillCategory(slot, state, on)
  return slot
}

/**
 * Refill the slot from the draft. Called on every keystroke.
 *
 * Always a chip, naming the shape of the word in play (see `wordInPlay`): the
 * last guess's until the next word is whole, so a Cluster stays Cluster through
 * the first four letters of the word after it. It used to be nothing until
 * there was a word, so it came and went with the round's first guess and drew
 * the eye to a line that had nothing new to say. Before any guess it is
 * Distinct, which is not a placeholder: it is the shape every word has until it
 * is shown to have a rarer one, and its level is what a plain word would score
 * at.
 */
export function fillCategory(slot: Element, state: RunState, on: Handlers): void {
  slot.replaceChildren()
  const word = wordInPlay(state)

  const board = ui().board
  const category = word ? categoryOf(word) : DISTINCT
  const bonus = levelBonus(state, category)
  // The Cliché, and the only place on the board that can say which shape it
  // took: the boss's card has to describe every run, so it names the rule and
  // not the shape, and the player cannot be asked to have kept count of their
  // own run. Struck through on a whole word of that shape, which is the moment
  // it matters, with the boss's line on the tip for whoever wonders why.
  const boss = getBoss(state.round.bossId)
  const barred = word !== "" && (boss?.voids?.(word, state) ?? false)
  // A button rather than a div, because this line is the only place the shape
  // system announces itself during a round, and a player who wants to know what
  // the other four shapes are has nowhere else to press.
  slot.append(
    h(
      "button",
      {
        class: barred ? "category barred" : "category",
        type: "button",
        "aria-label": `${categoryCard(category.id).name} ${board.shapeLevel(bonus.level)}`,
        "data-tip": barred && boss ? bossCard(boss.id).text : undefined,
        onclick: () => on.openShapes(),
      },
      h("span", { class: "category-name" }, categoryCard(category.id).name),
      // The level, said as a level ("Lv 2"), and nothing after it. The chip
      // used to spell out what the level pays, "+10 +2 mult", which on a seat
      // this narrow was the first thing the ellipsis ate and took the name down
      // with it. The number it quoted is on the shapes sheet a tap away, and
      // the readout beside the chip is where the payment actually shows.
      h("span", { class: "category-level" }, board.shapeLevel(bonus.level)),
      icon("chevron"),
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
  const table = isTable()
  const handLine = h(
    "div",
    { class: "hand-line" },
    consumableRow(state, on),
    table ? null : decorToggle(on, chrome),
  )
  return h(
    "div",
    { class: "screen round-screen" },
    // The shape of the word and the chips × mult it is being typed into, as
    // one line in the header. What solving would bank sat here too, as a chip
    // of its own, and is the header bar's projection now; see `meterSolve`.
    // The decor switch left it for the hand's row: `fillReadout` calls
    // `replaceChildren` on the readout every keystroke, which is why the
    // switch was only ever the readout's sibling, never its child, and why
    // moving it cost nothing.
    hud(
      state,
      on,
      h("div", { class: "dock-line" }, categorySlot(state, on), readoutSlot(state)),
      // The coaching card, hung from the header's foot; see `coachSlot`.
      coachSlot(chrome.coach),
      table ? decorToggle(on, chrome) : null,
    ),
    grid(state),
    // Everything a thumb presses outside the keys, at the keys: the hand and
    // the switch, then the relics flush to the keyboard. See `.hand-line`.
    // Tab order is the DOM's, and the table's grid draws the relics first and
    // the hand beside them, so on the table the relics are built first as well:
    // from the rail's ☰, the header's last control, Tab reaches relics, then
    // consumables. The phone draws
    // them hand-first, and so builds them hand-first, with the switch after.
    // Read at render, as `shopView` reads it for its own tray.
    ...(table ? [relicRow(state), handLine] : [handLine, relicRow(state)]),
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
  const token = tokenOf(state)

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
      class: `screen intro ${asking ? "asking" : ""}`,
      ...(asking ? {} : { onclick: () => on.play() }),
    },
    h(
      "div",
      { class: "intro-stage" },
      state.won ? copy.stageEndless(state.stage) : copy.stage(state.stage, STAGES),
    ),
    stageTrack(state),
    h(
      "div",
      { class: "intro-body" },
      h(
        "div",
        { class: `intro-card ${boss ? "boss-card" : ""}` },
        roundToken(token),
        // A boss is named for itself, and the round it is, which is the name
        // every other card leads with, drops to a label over it: the name is
        // the thing that is about to change the rules.
        boss && h("div", { class: "intro-kind" }, roundName(state.roundIndex)),
        h(
          "h1",
          { class: "intro-name" },
          boss ? bossCard(boss.id).name : roundName(state.roundIndex),
        ),
        boss && h("p", { class: "intro-rule" }, bossCard(boss.id).text),
        h("div", { class: "intro-label" }, copy.scoreAtLeast),
        h("div", { class: "intro-target" }, num(state.round.target)),
        h(
          "div",
          { class: "intro-stats" },
          h(
            "div",
            { class: "intro-stat" },
            h("span", { class: "intro-stat-value" }, `${state.round.maxGuesses}`),
            h("span", { class: "intro-stat-label" }, copy.guesses(state.round.maxGuesses)),
          ),
          h(
            "div",
            { class: "intro-stat" },
            // The payout the run will actually be handed, not the one the table
            // lists: ascension 7 takes a dollar off it, and a card that promises
            // $3 before a round and pays $2 after it is the worst kind of wrong.
            h(
              "span",
              { class: "intro-stat-value gold" },
              money(
                Math.max(0, (ROUND_PAYOUT[state.roundIndex] ?? 0) - difficultyOf(state).payoutCut),
              ),
            ),
            h("span", { class: "intro-stat-label" }, copy.reward),
          ),
        ),
        standing(state),
      ),
    ),
    h(
      "div",
      { class: "intro-actions" },
      h(
        "button",
        { class: "primary", type: "button", onclick: () => on.play() },
        asking ? copy.coachYes : copy.play,
      ),
      // Second and quieter. Why it is phrased as the whole tutorial rather than
      // as this card is written down beside the string.
      asking &&
        h(
          "button",
          { class: "secondary", type: "button", onclick: () => on.skipCoach() },
          copy.coachNo,
        ),
    ),
  )
}

/**
 * The stage's three rounds, in a row above the card.
 *
 * The card says what this round asks, and nothing on the screen said where it
 * sits: a player arriving at an elite round could not see that a boss comes
 * next, nor that it asks for half as much again, until the shop in between had
 * already been spent. That is the decision the shop is for.
 *
 * Every target comes from `roundTarget`, the reducer's own arithmetic, rather
 * than from the table here, so the two rounds not yet dealt carry the ascension
 * the round in hand does, and the boss's own cut.
 *
 * The boss slot names the boss, from the stage's first round. It used to be
 * drawn as a boss and not as the boss, on the grounds that naming it early would
 * promise a draw not yet made, but the draw is the seed and the stage and
 * nothing else (see `bossForStage`), so it was made when the run began, and the
 * only thing hiding it did was turn the two shops before a boss into a hedge.
 * Knowing The Vandal is coming is what makes a letter modifier bought the round
 * before a real decision rather than a bad one nobody could see.
 *
 * Its rule is spelled out in the cell, not on a hover tip: the tip was the
 * first version, and a rule the player has to know to go looking for is one a
 * thumb never finds. The cell takes it at the foot of the track's type, and the
 * track grows to hold it, which the screen can afford: the card under it is
 * centred in whatever height is left, and at 360x800 the longest rule in any
 * language (French's Silence, 104 characters) still left Play where it was.
 * The cell reads as three rows: the kind and the boss's name, the rule, and the
 * target and the status, that last pushed to the cell's foot so all three cards
 * still end on the same line however tall the boss's runs.
 */
function stageTrack(state: RunState): HTMLElement {
  const copy = ui().intro
  const boss = bossCard(bossForStage(state))
  return h(
    "ol",
    { class: "stage-track", "aria-label": copy.track },
    ...TOKEN_AT.map((kind, index) => {
      const cleared = index < state.roundIndex
      const current = index === state.roundIndex
      const status = cleared
        ? copy.cleared
        : current
          ? copy.current
          : index === state.roundIndex + 1
            ? copy.upNext
            : ""
      return h(
        "li",
        {
          class: `track-round ${kind} ${cleared ? "cleared" : current ? "current" : ""}`,
          ...(current ? { "aria-current": "step" } : {}),
        },
        h(
          "div",
          { class: "track-head" },
          h("div", { class: "track-name" }, roundToken(kind), copy.trackRound[index] ?? ""),
          kind === "boss" && h("div", { class: "track-boss" }, boss.name),
        ),
        kind === "boss" && h("div", { class: "track-rule" }, boss.text),
        h(
          "div",
          { class: "track-foot" },
          h("div", { class: "track-target" }, num(roundTarget(state, index))),
          h("div", { class: "track-status" }, cleared && icon("check"), status),
        ),
      )
    }),
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
      reward?.relics ? line(copy.relics, reward.relics) : null,
      reward?.saved &&
        h("div", { class: "answer-note" }, copy.savedBy(relicCard(reward.saved).name)),
      reward?.firstGuess && h("div", { class: "answer-note" }, copy.firstGuess),
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
 * Where a level card takes the thing it levels: "Lv 3 → Lv 4", the step it is
 * sold as. On the table it is the card's picture, since a word shape has no
 * better drawing than the number it is about to become.
 */
function ladderLine(level: number): HTMLElement | null {
  if (level <= 0) return null
  const copy = ui().shop
  return h(
    "div",
    { class: "shop-item-ladder" },
    h("span", { class: "ladder-from" }, copy.level(level)),
    h("span", { class: "ladder-arrow", "aria-hidden": "true" }, "→"),
    h("span", { class: "ladder-to" }, copy.level(level + 1)),
  )
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

/**
 * Which drawing rides in a card's ticket, one per kind. The card's own picture,
 * beside its name, is a different question (which one, not what sort); see
 * `cardHead`.
 *
 * Keyed on the kind rather than on the tag string, because the tag is prose and
 * changes with the language and with the tray ("Consumable · slots full"), and
 * the picture should not.
 */
const KIND_ICON: Record<ShopItem["kind"], IconName> = {
  relic: "gem",
  consumable: "flask",
  pack: "box",
  range: "alphabet",
  mod: "letter",
  level: "shape",
  etch: "etching",
}

/**
 * The card's name with its picture beside it. Every card the shelf deals has
 * a drawing of its own, and the kind's drawing stands in only for an id the
 * tables have not caught up with. That is not only for the look of the thing:
 * the shelf is two columns, and a card with no picture beside one with a
 * picture lined its name up a picture's width to the left of its neighbour's.
 * Modifiers, word shapes and ranges wore their kind's drawing for a while, and
 * all nine modifiers, Chip to Glass, were the same diamond on a tile.
 *
 * Beside the name rather than over it. On a line of its own the picture cost
 * every card on the shelf a 28px row, and the pack and the picks in an open
 * pack, which are short full-width cards, had to be laid out on a grid of their
 * own to keep it from being most of their height. Beside the name it costs a
 * name that wraps a line sooner, which is the trade `.shop-item-head` answers.
 */
const cardHead = (item: ShopItem, title: string): HTMLElement =>
  h(
    "div",
    { class: "shop-item-head" },
    ...cardArt(item.kind, item.id),
    h("div", { class: "shop-item-name" }, title),
  )

/**
 * A card's picture, drawn twice: the line emblem and the pixel sprite, side by
 * side, and the stylesheet shows one. The view does not know which look is on,
 * which is the point. The choice used to be a skin's alone (Smoke wants the
 * sprite, Tabletop engraves the emblem onto a bone tile, Classic tints the
 * emblem it always had), and asking the skin at render time would leave the
 * shelf drawn for the wrong look after the one thing that changes a look
 * without a render: the window crossing the table's breakpoint under an
 * unpicked default. The price is one hidden `<svg>` of a few hundred bytes per
 * card, on a screen that rebuilds a dozen of them. `.sprite` is `display: none`
 * outside Smoke (`shop.css`), and every skin that wants the sprite says so.
 *
 * Every card has a third picture, the emoji Classic shows, and it is not a
 * third node: it scales to the emblem's own box, so it rides inside the emblem
 * and the stylesheet picks between the two there. See `emojied` in `emoji.ts`.
 */
const cardArt = (kind: ShopItem["kind"], id: string): Node[] => {
  const line = emblem(kind, id)
  const art: Node[] = [line ? emojied(line, kind, id) : icon(KIND_ICON[kind])]
  const pixels = sprite(kind, id)
  if (pixels) art.push(pixels)
  return art
}

/**
 * A card on the shelf, drawn as a price tag: what it is, pictured and named, and
 * what it does, top down, and what it costs at the foot, where the reading ends.
 *
 * Two columns of these rather than one of rows. The rows gave the sentence more
 * width, and spent it: five full-width rows and a tray ran past the fold at 390,
 * and a shelf you scroll is a shelf whose last card you forget. Half a phone is
 * enough for the sentence at 13.5px, which the cards before the rows never were
 * at 11px.
 */
function shopItemCard(item: ShopItem, index: number, state: RunState, on: Handlers): HTMLElement {
  const affordable = state.gold >= item.cost
  const { title, text, rarity, tag, tip, blocked, swap, level } = describeItem(item, state)

  return h(
    "button",
    {
      class: `shop-item kind-${item.kind} rarity-${rarity} ${affordable ? "" : "broke"}`,
      // The stock deals in one card at a time rather than appearing all at once,
      // which is what makes a reroll feel like being dealt a new hand.
      style: `--deal:${index}`,
      "data-flip": `shelf-${index}`,
      type: "button",
      // Dimmed, and still a button. The tap is how a thumb asks why, and the
      // answer is the till's "not enough gold" toast; a `disabled` card would
      // swallow the tap and leave a phone with no way to ask at all. The shortfall
      // rides on the card as a tip for the pointer that can hover, and for the
      // long-press, rather than printed under the price: "$1 short" on every card
      // the player cannot reach was a second price tag in a quieter ink, and it
      // made the dim cards the busiest thing on the shelf.
      "aria-disabled": affordable ? undefined : "true",
      "data-tip": affordable ? undefined : ui().shop.short(money(item.cost - state.gold)),
      "data-rarity": affordable ? undefined : rarity,
      // A bought card leaves a sold tag in its seat, which takes no focus, so
      // this is a seat in a row and the nearest card left stands in. See `holdFocus`.
      "data-focus": `shelf-${index}`,
      onclick: () => on.buy(index),
    },
    cardHead(item, title),
    h("div", { class: "shop-item-text" }, ...withAmounts(text)),
    ladderLine(level),
    swap ? swapLine(swap, displaced(item, state)) : null,
    // Said on a line of its own, where the ticket used to say it as
    // "Consumable · slots full": at the foot beside a price, that pair ran to
    // two lines on every card it was on.
    blocked ? h("div", { class: "shop-item-full" }, ui().shop.slotsFull) : null,
    h(
      "div",
      { class: "shop-item-foot" },
      h("div", { class: "shop-item-cost" }, money(item.cost)),
      kindTicket(item, tag, blocked, tip, rarity),
    ),
  )
}

/**
 * The place a card stood before it was bought, or a pick already taken from a
 * pack. It stays at all so the cards after it do not shuffle up a seat under a
 * thumb that is about to tap again; the engine nulls the slot, so there is
 * nothing left to say in it but that it went.
 */
function soldCard(index: number, label: string): HTMLElement {
  return h("div", { class: "shop-item sold", style: `--deal:${index}` }, icon("check"), label)
}

/**
 * The kind ticket, the drawing and the word in the card's rarity color, at the
 * foot opposite the price.
 *
 * It has been in three places. Beside the name at the top right, word and all,
 * it left the name ~100px of a ~150px card at 390, and relic names are one long
 * word often enough that they broke mid-word ("Lexicograp/her"). As the drawing
 * alone in that corner it fit, and gave up the word. The foot is where the room
 * is: a price is three characters, and the rest of that line was empty on
 * every card. The name gets the whole width back. On a 320 phone
 * "Verbrauchskarte" still does not fit beside a price, and the foot wraps it
 * under rather than squeezing either one.
 *
 * The tip hangs off the ticket rather than off the whole card, because the card
 * is a buy button: a panel that opened over the shelf every time the pointer
 * crossed a price would be in the way of the thing it is explaining. The one
 * exception is a card that cannot be bought, which carries its shortfall as a
 * tip of its own; see `shopItemCard`. An empty `tag` draws the ticket as the
 * drawing alone, which is what the pack sheet asks for; an empty `tip` leaves
 * it inert, since the sheet's backdrop sits above the tip's layer and a tip
 * raised from inside it would be drawn behind the sheet that asked for it.
 *
 * The rarity rides along so the panel takes the card's own color, the way a
 * relic's does from the tray.
 */
function kindTicket(
  item: ShopItem,
  tag: string,
  blocked: boolean,
  tip: string,
  rarity: string,
): HTMLElement {
  return h(
    "span",
    {
      class: `shop-item-kind${blocked ? " blocked" : ""}`,
      "data-tip": tip || undefined,
      "data-rarity": tip ? rarity : undefined,
    },
    icon(KIND_ICON[item.kind]),
    tag ? h("span", {}, tag) : null,
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
          if (!item) return soldCard(index, copy.taken)
          const { title, text, rarity, swap, level } = describeItem(item, state)
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
            //
            // So the ticket is the drawing alone here, which is still worth its
            // corner: it is what the shelf taught the rarity color on.
            cardHead(item, title),
            h("div", { class: "shop-item-text" }, ...withAmounts(text)),
            ladderLine(level),
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
            h(
              "div",
              { class: "shop-item-foot" },
              h("div", { class: "shop-item-cost free" }, money(item.cost)),
              kindTicket(item, "", false, "", rarity),
            ),
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
    // One span a shape rather than one string, so the table can draw each as a
    // chip; the phone runs them together on a line, dotted by the stylesheet.
    h(
      "span",
      { class: "shapes-line-body" },
      ...(leveled.length > 0
        ? leveled.map((c) =>
            h(
              "span",
              { class: "shapes-chip" },
              copy.shapesLevel(categoryCard(c.id).name, levelOf(state, c.id)),
            ),
          )
        : [copy.shapesNone]),
    ),
    h("span", { class: "shapes-line-more", "aria-hidden": "true" }, "›"),
  )
}

export function shopView(state: RunState, on: Handlers, coach: CoachStep | null): HTMLElement {
  const copy = ui().shop
  const shop = state.shop
  const reroll = shop ? rerollCost(state, shop) : 0

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
        // By id rather than slot: selling shifts the rest left, and a key that
        // follows the card is what lets the table slide them into the gap.
        "data-flip": `relic-${instance.id}`,
        "data-tip": relicCard(instance.id).text,
        "data-rarity": relic?.rarity ?? "common",
        type: "button",
        "data-focus": `relic-${slot}`,
        onclick: () => on.sell(slot),
      },
      ...cardArt("relic", instance.id),
      h("span", { class: "relic-name" }, relicCard(instance.id).name),
      h("span", { class: "sell" }, copy.sell(money(sellValue(relic?.cost ?? 4)))),
    )
  })

  // The round's consumable row, card for card: name over rule, and no row at
  // all while the hand is empty. The board draws empty seats, because there a
  // row that came and went moved the tiles; nothing here is aimed at. A full
  // hand needs no empty seats to say so; the shelf card that will not fit says
  // "No free slot" itself.
  //
  // Buttons, as the relics beside them are, and a tap here throws the card away
  // rather than playing it: a card is only played during a round (the engine
  // refuses it anywhere else), and a hand with no way to make room turned every
  // better card on the shelf into one the player could only walk past. One tap
  // and no question, the relic's own terms, and the label says what the tap does
  // the way the relic's price does.
  const cards =
    state.consumables.length > 0
      ? h(
          "div",
          { class: "consumables" },
          ...state.consumables.map((instance, index) => {
            const card = consumableCard(instance.id)
            return h(
              "button",
              {
                class: "consumable",
                type: "button",
                // The rule is clamped on the table, so the whole of it is a
                // hover or a press away, as it is on the board's hand.
                "data-tip": card.text,
                "data-focus": `consumable-${index}`,
                onclick: () => on.drop(index),
              },
              h("span", { class: "consumable-name" }, card.name),
              h("span", { class: "consumable-text" }, ...withAmounts(card.text)),
              h("span", { class: "drop" }, copy.drop),
            )
          }),
        )
      : null
  const tray = [h("div", { class: "relics" }, ...owned), cards]
  const table = isTable()

  return h(
    "div",
    { class: "screen shop-screen" },
    h(
      "header",
      { class: "hud" },
      h(
        "div",
        { class: "hud-round" },
        h("div", { class: "round-name" }, copy.title),
        stageLine(state),
      ),
      // The round's header, slot for slot, with the score's seat left empty:
      // there is no score in a shop. The seat once held the bar at the round's
      // height, and there is no bar here now either, nor room kept for one,
      // since the round's dock line put a row between the two headers anyway;
      // see the shop section of the stylesheet. The gold no longer depends on it for its
      // place; the grid in `.hud` pins it beside the menu on both screens.
      h("div", { class: "hud-score", "aria-hidden": "true" }),
      h(
        "div",
        { class: "hud-end" },
        h("div", { class: "hud-gold" }, money(state.gold)),
        menuButton(on),
      ),
      // The first visit's card, from the header for the round's reason: the
      // header is the one box both screens have, and `aimCoach` measures from
      // it to whichever anchor the beat names.
      coachSlot(coach),
    ),
    // What the run already holds, straight under the header and drawn exactly
    // as the round draws it, because that is where the player already knows
    // these things live and what they look like there. It was a tray of its own
    // at the foot for a while, labelled and re-proportioned, beside the buttons
    // so a relic sold was a thumb's width from Next round; a second look for
    // the same five tiles cost more than the reach saved. The only difference
    // from the board is the price on each relic, since here a tap sells it.
    //
    // On the phone only. The table's grid draws the tray in the bottom row,
    // beside Next round, and a grid places what it is told to without moving
    // the tab order, which is the DOM's: Tab went from the menu straight into
    // the relics at the foot of the window and then climbed back up to the
    // shelf. CSS has no answer to that (`order` and grid placement both leave
    // focus in source order, and a positive `tabindex` would pull the tray ahead
    // of the header), so the view puts the tray where the table draws it. Read
    // at render, which every dispatch is, so a window dragged across the room
    // query keeps the old order only until the next tap.
    ...(table ? [] : tray),
    h(
      "div",
      { class: "shop-items" },
      ...(shop?.items ?? []).map((item, index) =>
        item ? shopItemCard(item, index, state, on) : soldCard(index, copy.sold),
      ),
    ),
    // Below the shelf, not in the tray with the relics, because the round has
    // no such row to match, and at the foot rather than under the pack so the
    // slack on a tall phone opens between it and the shelf: tucked under the
    // last card it read as a sixth thing for sale, which it has been once.
    shopShapes(state, on),
    ...(table ? tray : []),
    h(
      "div",
      { class: "shop-actions" },
      h(
        "button",
        {
          class: "secondary",
          type: "button",
          disabled: state.gold < reroll,
          // Rerolling is the one press a player makes several times running,
          // and it rebuilds the shop it is standing in. See `holdFocus`.
          "data-focus": "reroll",
          onclick: () => on.reroll(),
        },
        icon("reroll"),
        copy.reroll,
        // Gold, like every other price on the screen. It was the button's own
        // ink, which made the one price the player pays over and over the only
        // one that did not look like money.
        h("span", { class: "reroll-cost" }, money(reroll)),
      ),
      h(
        "button",
        { class: "primary", type: "button", onclick: () => on.nextRound() },
        copy.nextRound,
      ),
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
export function endView(state: RunState, on: Handlers, seed: string | null): HTMLElement {
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
      // Only when the score is what fell short. At ascension 10 a run also ends
      // on a word left unsolved with the target met, and there "short by" would
      // print zero or a negative number for a loss the points had nothing to do
      // with; the answer above is the whole of what that ending has to say.
      lost &&
        state.round.score < state.round.target &&
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
    // Under the panel rather than in it: the panel is what happened, and this is
    // what to do with it. A lost run is the one most worth handing a friend.
    // `seed` is the chrome's, which is null for a spectator: the copy is idle
    // while watching, and a button that does nothing is worse than none.
    seed !== null && seedLine(seed, on),
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
function languageButton(on: Handlers, chrome: Chrome, look = "secondary"): HTMLElement {
  return h(
    "button",
    {
      class: `${look} lang-button`,
      type: "button",
      "aria-label": `${ui().pause.language}: ${LANG_NAMES[chrome.lang]}`,
      // The same name as the pause sheet's switch, for the same reason: a
      // cycle is up to three presses to the language wanted. See `holdFocus`.
      "data-focus": "lang",
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
    // Not `center`: the screen is a column that starts at the top and puts its
    // footer on the floor, and centring it would float the whole stack in the
    // middle with the footer's slack split above and below it.
    { class: "screen title" },
    h(
      "div",
      { class: "title-mast" },
      // The name set as the first row of a board, with the rows above it still
      // empty: the screen before a run is the board before a guess, and the two
      // ghost rows say "a board" before anyone has read a word. They are drawn
      // rather than said, so they are hidden from everything but the eye.
      h(
        "div",
        { class: "title-ghost", "aria-hidden": "true" },
        ...[0, 1].map(() =>
          h(
            "div",
            { class: "title-ghost-row" },
            ...Array.from({ length: 5 }, () => h("span", { class: "title-ghost-tile" })),
          ),
        ),
      ),
      h(
        "div",
        { class: "title-word" },
        // The row itself, lit the way a solved guess is lit. The digit is the
        // green, the wild in the name and the one letter "placed"; the four
        // letters are gray, which is the colour of a guess the board has read
        // and has nothing to say about. It was all five green with a gold digit,
        // which is a won round and read as a logo rather than a board. The space
        // is dropped rather than drawn: a sixth column would be a row a board
        // does not have. The heading keeps the name as its accessible text.
        h(
          "h1",
          { class: "title-name", "aria-label": copy.name },
          ...[...copy.name.replace(/\s/g, "")].map((ch) =>
            h(
              "span",
              { class: `title-tile ${/\d/.test(ch) ? "wild" : ""}`, "aria-hidden": "true" },
              ch,
            ),
          ),
        ),
        // The other half of the game, pinned to the corner of the row the way a
        // scored guess wears its badge: chips in their blue, mult in its red.
        // Decoration, so figures and not a sentence, and nothing to translate.
        h(
          "div",
          { class: "title-score", "aria-hidden": "true" },
          h("span", { class: "title-chips" }, "+120"),
          h("span", { class: "title-times" }, "×"),
          h("span", { class: "title-mult" }, "4"),
        ),
      ),
      h("p", { class: "title-tag" }, copy.tagline),
      record(meta, on),
    ),
    ladder(on, meta),
    h(
      "button",
      { class: "primary title-play", type: "button", onclick: () => on.newRun() },
      common.play,
    ),
    // Side by side rather than stacked under Play: two full-width grays under a
    // full-width green made three buttons of one weight, and these two are where
    // a player goes instead of playing, not after.
    h(
      "div",
      { class: "title-more" },
      h(
        "button",
        { class: "secondary", type: "button", onclick: () => on.openHelp() },
        icon("help"),
        common.howToPlay,
      ),
      h(
        "button",
        { class: "secondary", type: "button", onclick: () => on.openCodex() },
        icon("book"),
        common.codex,
      ),
    ),
    // Here as well as on the pause sheet, and this is the copy that matters more
    // of the two: the title screen is the first screen anybody sees, and a
    // player who has landed on the wrong language needs the way out of it before
    // they have started a run, not from a menu inside one.
    //
    // On the floor with the sound and the build stamp, because those three are
    // settings of the device rather than of the game, and the flag finds the eye
    // on its own wherever it sits. No deferred-words line beside it:
    // `wordsDeferred` is false at the title by construction, and it is the only
    // screen where that is true of every language.
    h(
      "div",
      { class: "title-foot" },
      languageButton(on, chrome, "title-pill"),
      // The hash as its own node so a phone can drop it; see `.title-build-commit`.
      //
      // It is also the door to seeded runs, and a door nobody new will find, which
      // is the point. The title screen has one green button and two grays, and a
      // seed field among them would be asking every first-time player a question
      // only a returning one can answer. A player handed a code by a friend is
      // told where to put it; a player who taps the version out of curiosity
      // finds it and is none the worse. The APK and the desktop build can open no
      // link, so this field is also the only way in for them.
      h(
        "button",
        {
          class: "title-build",
          type: "button",
          "aria-label": `v${__BUILD_VERSION__}. ${ui().seed.open}`,
          onclick: () => on.openSeed(),
        },
        `v${__BUILD_VERSION__}`,
        __BUILD_COMMIT__
          ? h("span", { class: "title-build-commit" }, ` · ${__BUILD_COMMIT__}`)
          : null,
      ),
      // The ⓘ beside the speaker rather than on its own corner: both are
      // round, unlabelled and about the game rather than a run, and the floor
      // has three places, not four.
      h(
        "div",
        { class: "title-dials" },
        h(
          "button",
          {
            class: "title-dial",
            type: "button",
            "aria-label": ui().about.title,
            onclick: () => on.openAbout(),
          },
          icon("info"),
        ),
        soundButton(on, chrome),
        skinButton(on, chrome),
      ),
    ),
  )
}

/**
 * The title screen's look dial, after the speaker and on its terms: the face is the look
 * on screen, a tap moves to the next of the four, and the words are in `aria-label`. The
 * pause sheet has the same setting as a labelled row; this is the copy for the
 * first screen, where a player squinting at a bright page in a dark room should
 * not have to start a run to find the switch.
 *
 * Sun and moon stay the two Classics' faces because they are what this button
 * has always been, and a player who knew the theme dial still finds it where it
 * was, doing what its face says; the two rooms get a drift of smoke and a
 * tile, which say what they are dressed in rather than what the light is.
 */
const SKIN_ICON: Record<Skin, IconName> = {
  smoke: "smoke",
  tabletop: "tile",
  "classic-dark": "moon",
  "classic-light": "sun",
}

function skinButton(on: Handlers, chrome: Chrome): HTMLElement {
  const copy = ui().pause
  return h(
    "button",
    {
      class: "title-dial",
      type: "button",
      "aria-label": `${copy.skin}: ${copy.skins[chrome.skin]}`,
      "data-focus": "skin",
      onclick: () => on.cycleSkin(),
    },
    icon(SKIN_ICON[chrome.skin]),
  )
}

/**
 * The title screen's sound toggle: the speaker alone, in a circle the size of a
 * thumb.
 *
 * The intro card had a word-shaped one too, "Sound on" in a pill under Play,
 * and it went when the card grew a stage track: the menu is one tap away on
 * every screen after it, and a toggle on a card that is itself a tap target had
 * to stop its own click from starting the round. Here it sits in a footer beside
 * the language pill, where a second pill with the word in it made the floor a
 * row of labels nobody was reading. The label moves to `aria-label`, and names the state the
 * way the word did, so a screen reader hears what a sighted player sees drawn.
 */
const SOUND_ICON: Record<SoundLevel, IconName> = {
  off: "muted",
  sound: "sound",
  music: "music",
  musicOnly: "music",
}

function soundButton(on: Handlers, chrome: Chrome): HTMLElement {
  return h(
    "button",
    {
      class: "title-dial",
      type: "button",
      "aria-label": ui().common.sound[chrome.sound],
      "data-focus": "sound",
      onclick: () => on.cycleSound(),
    },
    icon(SOUND_ICON[chrome.sound]),
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
 * row where every other rung's sentence goes, since zero has none of its own. That is the one rung whose
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
        // A row of two, so that walking to either end, where the step that got
        // there goes `disabled`, hands focus to the other. See `holdFocus`.
        "data-focus": `ladder-${label === "−" ? 0 : 1}`,
        onclick: () => on.setAscension(to),
      },
      label,
    )

  return h(
    "div",
    {
      class: `ladder ${level > 0 ? "lit" : ""} ${locked ? "locked" : ""}`,
    },
    h(
      "div",
      { class: "ladder-row" },
      // Down on the left and up on the right, with the rung between them, so
      // the row reads as the dial it is: the direction of each press is where
      // the thumb goes, and the name is the one thing neither thumb is on.
      step("−", level - 1, level > 0),
      h(
        "div",
        { class: "ladder-level" },
        h(
          "span",
          { class: "ladder-name" },
          ui().common.ascension(level),
          // The lock rides the name rather than sitting in the note alone, so
          // the state is legible from the one line the eye lands on first.
          locked &&
            h(
              "span",
              { class: "ladder-lock", role: "img", "aria-label": copy.locked },
              icon("lock"),
            ),
        ),
        // The second line of the label is "what this rung is". Zero says it is
        // the standard game, rather than leaving the seat empty: an empty seat
        // centred the name a line lower than on every other rung, and it jumped
        // on the press that left zero.
        h("span", { class: "ladder-rule" }, rule ? ascensionCard(rule).name : copy.base),
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
            icon("lock"),
          )
        : step("+", level + 1, level < MAX_ASCENSION),
    ),
    // Every rung has a sentence, in the same seat, so the card is one shape
    // from zero to the top. Zero once had none, on the grounds that "no extra
    // rules" is the absence of a rule spelled out; but its card was then 38px
    // shorter than rung one's, Play moved by that much on the first press up,
    // and the lone line it did carry sat under the row where every other
    // rung's sentence floats in the middle of the space. Two lines of
    // standard-game prose were the cheaper fix than a second layout.
    //
    // Before the first win zero's sentence is the carrot instead, the one
    // forward-looking line on the screen: what beating this rung would open.
    // The warning that used to sit here above the earned rung is gone. It said
    // the player had not won yet, in a box whose lock says the same thing in a
    // glyph and whose sheet had just said it in a sentence, and three tellings
    // of "you have not earned this" is the screen holding a grudge about a
    // choice it offered.
    h(
      "p",
      { class: `ladder-text ${carrot ? "carrot" : ""}` },
      rule
        ? `${ascensionCard(rule).text}${level > 1 ? ` ${copy.andBelow}` : ""}`
        : (carrot ?? copy.baseText),
    ),
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
      SHEET_BODY_ATTRS,
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
 * Both halves are inlined at build time; see vite.config.ts. The version is
 * the last release tag and the hash is the commit, and the hash is the one that
 * can be trusted: Pages redeploys on every push, but a release is cut once per
 * phase, so between two tags the site serves new code under the old number.
 * Where there is no hash to be had the version stands alone rather than
 * trailing a bare separator.
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
 * What every sheet's scroll box wears. `tabindex="-1"` for the reason the sheet
 * has one, so that focus can be put on it when the sheet opens, and for the
 * reason that is wanted: a keyboard scrolls the box at or above focus, never
 * one below it. See `holdFocus`. Firefox and Chrome put a scroll box in the tab
 * order on their own, and an explicit -1 takes it out again, which keeps the
 * stops a Tab makes in a sheet its buttons and nothing else.
 */
export const SHEET_BODY_ATTRS = { class: "sheet-body", tabindex: -1 }

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

export function helpView(on: Handlers, offerTutorial: boolean): HTMLElement {
  const copy = ui().help
  return overlay(
    on,
    h("h2", { class: "sheet-title" }, copy.title),
    h(
      "div",
      SHEET_BODY_ATTRS,
      // First, and above the rules rather than beside the buttons at the foot,
      // because the player it is for is the one who opened this sheet to learn
      // the game, and the tutorial is a better way to do that than the page
      // under it: the rules say "green is +3 mult" to someone who has not yet
      // seen a mult, and the tutorial says it with one on the screen. Only
      // while the tutorial is still owed, which is the caller's to know.
      offerTutorial &&
        h(
          "div",
          { class: "help-tutorial" },
          h("p", {}, copy.tutorial),
          h(
            "button",
            { class: "primary", type: "button", onclick: () => on.startTutorial() },
            copy.tutorialStart,
          ),
        ),
      h("p", {}, copy.lead),
      h("p", { class: "sheet-lead" }, copy.scored),
      rule(copy.chipsMult),
      rule(copy.letters),
      rule(copy.colors),
      rule(copy.solving),
      h("p", { class: "sheet-lead" }, copy.farming),
      rule(copy.solveLine),
      h("h3", { class: "sheet-heading" }, copy.runHeading),
      ruleOf(copy.target, STAGES, ROUNDS_PER_STAGE, AUTHORED_ASCENSIONS),
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

/**
 * One catalog entry: what it is called, what it does, and what it costs, and
 * its picture where it has one.
 *
 * The picture is `cardArt`'s, the same nodes the shelf shows, so the codex
 * answers "which one was that" for a player who remembers the sprout and not
 * the name, which is most of what a relic is remembered by. Everything that
 * can be bought has one. Bosses and ascensions have no picture anywhere and
 * get no empty box here for one: the class is on the entry, not a blank column
 * in every row.
 */
function entry(name: string, text: string, note?: string, art?: Node[]): HTMLElement {
  return h(
    "div",
    { class: art ? "codex-entry pictured" : "codex-entry" },
    art ? h("span", { class: "codex-art" }, ...art) : null,
    h(
      "div",
      { class: "codex-entry-head" },
      h("strong", {}, name),
      note ? h("span", { class: "codex-note" }, note) : null,
    ),
    h("span", { class: "codex-text" }, ...withAmounts(text)),
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
          // Two rows of two: what it is called and how far it is leveled, then
          // what it asks of a word and what it pays for one. See `.shape`.
          h(
            "div",
            { class: "shape-head" },
            h("strong", {}, card.name),
            scores ? h("span", { class: "shape-tag" }, copy.scoring) : null,
            matched ? h("span", { class: "shape-tag also" }, copy.alsoMatches) : null,
          ),
          h("span", { class: "shape-level" }, ui().board.shapeLevel(bonus.level)),
          h("span", { class: "shape-text" }, card.text),
          h(
            "span",
            { class: "shape-pay" },
            bonus.chips > 0
              ? copy.payNow(bonus.chips, bonus.mult, bonus.times)
              : copy.payPerLevel(category.chips, category.mult, category.growth),
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
      SHEET_BODY_ATTRS,
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
                return entry(card.name, card.text, money(relic.cost), cardArt("relic", relic.id))
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
          return entry(
            card.name,
            card.text,
            copy.shapePer(category.chips, category.mult, category.growth),
            cardArt("level", category.id),
          )
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
            money(mod.cost),
            cardArt("mod", mod.id),
          )
        }),
      ),

      section(
        copy.upgrades,
        ETCHINGS.length + RANGES.length,
        ...ETCHINGS.map((etching) => {
          const card = etchingCard(etching)
          return entry(card.name, card.text, money(etching.cost), cardArt("etch", etching.id))
        }),
        ...RANGES.map((range) =>
          entry(range.name, rangeText(range), undefined, cardArt("range", range.id)),
        ),
      ),

      sectionOf(
        copy.consumables,
        CONSUMABLES.length,
        [CONSUMABLE_SLOTS],
        ...CONSUMABLES.map((consumable) => {
          const card = consumableCard(consumable.id)
          return entry(
            card.name,
            card.text,
            money(consumable.cost),
            cardArt("consumable", consumable.id),
          )
        }),
      ),

      section(
        copy.packs,
        PACKS.length,
        // "Choose 1 of 3" already says how many you keep, so the count is
        // only worth printing on a pack that keeps more than one, which none
        // do yet, and which is exactly why it is derived rather than assumed.
        ...PACKS.map((pack) => {
          const card = packCard(pack.id)
          return entry(
            card.name,
            pack.picks > 1 ? copy.packTextPicks(card.text, pack.picks) : copy.packText(card.text),
            money(pack.cost),
            cardArt("pack", pack.id),
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
    // The run's code rides the title's row, opposite it, rather than under the
    // settings where it first went. There it was one more 44px row in a sheet
    // already at its height cap, and the settings list, which clips rather than
    // scrolls and is the first thing to shrink, paid for it with its last row:
    // the sharing switch, cut in half. The title's row has the width to spare
    // and was spending none of its height on anything but the word.
    h(
      "div",
      { class: "sheet-top" },
      h("h2", { class: "sheet-title" }, copy.title),
      chrome.seed !== null && seedLine(chrome.seed, on),
    ),
    // The four settings are one quiet list rather than four more buttons in the
    // stack. Drawn as buttons they were the same size, color and weight as Resume
    // and Quit, so the sheet read as eight equal choices when it is two kinds of
    // thing: dials the player adjusts and leaves, and ways out of the sheet.
    // Full-width rows rather than a 2×2 grid, because "Velocidad de animación ×1"
    // and "Sonido desactivado" do not fit half a phone.
    h(
      "div",
      { class: "settings" },
      setting(
        { "data-focus": "sound", onclick: () => on.toggleEffects() },
        copy.sound,
        !chrome.effectsOff,
      ),
      // Music gets its own switch rather than riding on the sound one: it plays
      // continuously, so it is the thing a player is most likely to want gone
      // while keeping the feedback that tells them what their guess scored.
      //
      // And the two are independent here, as they are nowhere else. The title
      // screen's speaker cycles three levels, effects with music, effects, off,
      // and music with no effects is not one of them, because a fourth stop on a
      // button with no label is a stop the player has to count past. It is a real
      // preference, though (music while reading, say), so these switches can
      // reach it, and the speaker draws it as the note and steps it to off. This
      // switch used to go dead while sound was off, which is what had made that
      // pair unreachable.
      setting(
        { "data-focus": "music", onclick: () => on.toggleMusic() },
        copy.music,
        !chrome.musicOff,
      ),
      // Which recording, named, and a tap steps to the other. Under the music
      // switch because it is that switch's detail, and live while the music is
      // off, so a player can pick the track before turning it back on and hear
      // the one they chose rather than the one they were escaping.
      setting({ "data-focus": "track", onclick: () => on.nextTrack() }, copy.track, chrome.track),
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
      // out of the way can tap until the number is big enough. Written here and
      // not in the catalogs: `×` is the glyph the modifier pips use, and it is
      // the same in all four.
      setting(
        { "data-focus": "speed", onclick: () => on.cycleSpeed() },
        copy.speed,
        `×${chrome.speed}`,
      ),
      // One row for the whole look, on every layout: the four presets are the
      // two tones and the two rooms, so there is no separate light or dark.
      setting(
        { "data-focus": "skin", onclick: () => on.cycleSkin() },
        copy.skin,
        copy.skins[chrome.skin],
      ),
      // The same flag and endonym as the title screen's pill, as this row's
      // value. No `lang` attribute, for the reason `languageButton` gives.
      setting(
        { "data-focus": "lang", onclick: () => on.cycleLanguage() },
        copy.language,
        h(
          "span",
          { class: "lang-value" },
          h("span", { class: "lang-flag" }, LANG_FLAGS[chrome.lang]),
          LANG_NAMES[chrome.lang],
        ),
      ),
      // Run sharing, the same switch as the one on the about sheet. It is a
      // choice about the game rather than this run, which argued for the about
      // sheet alone, but that sheet is only on the title screen, and a player who
      // wants to change their mind mid-run should not have to quit to do it.
      // Absent in a build with nowhere to send anything, like the other.
      chrome.sharing !== null && sharingSwitch(on, chrome),
      // Letter values are not here. The board is dense by design, with a value on
      // every key and a pip on every modifier, and that density is the scoring
      // game asking to be played; some of the time the player is doing the other
      // thing entirely, which is working out a five-letter word, and every
      // number on screen is noise. The switch for it is `decorToggle`, on the
      // board itself (by the keys on a phone, in the rail's foot on the table),
      // because it was the one setting on this sheet whose whole effect was
      // hidden behind the sheet while it was being set. That it has three
      // states now is a further reason to leave it there: a segmented control
      // here would be the readable way to show them, and it would show them on
      // top of the board the reader needs to see to choose between them.
    ),
    // The honest footnote, on screen only while it is true. Changing the
    // language repaints every sentence here immediately and does not touch the
    // run: a run is dealt from one word list and keeps it, so the answer stays
    // in the language it was drawn from. Said here and not on the title
    // screen, because `wordsDeferred` needs a run open to be true at all.
    chrome.wordsDeferred ? h("p", { class: "lang-note" }, copy.wordsNextRun) : null,
    chrome.thanked && thanks(chrome.thanked),
    h(
      "div",
      { class: "sheet-actions" },
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

/** Where "Source code" goes. The privacy page links the same repository. */
const SOURCE_URL = "https://github.com/jmelahman/5-wild"

/**
 * The sheet behind the title screen's ⓘ: run sharing, the credits, the source.
 *
 * Its own sheet rather than three more rows on the pause sheet, because none of
 * the three is about the run the pause sheet interrupts. Sharing is on both:
 * here so it can be found without starting a run, and on the pause sheet so it
 * can be changed without leaving one.
 *
 * The switch comes with its note because this is where a player lands who
 * wants to know what they agreed to, and "Run sharing on" alone does not say.
 */
export function aboutView(on: Handlers, chrome: Chrome): HTMLElement {
  const copy = ui().about
  return overlay(
    on,
    h("h2", { class: "sheet-title" }, copy.title),
    // Absent entirely from a build with nowhere to send anything. An unanswered
    // question draws as off, because off is what it is: nothing is sent until
    // someone says yes.
    chrome.sharing !== null && h("div", { class: "settings" }, sharingSwitch(on, chrome)),
    chrome.sharing !== null &&
      (chrome.thanked
        ? thanks(chrome.thanked)
        : h("p", { class: "sheet-note" }, `${copy.sharingNote} `, privacyLink())),
    h(
      "div",
      { class: "sheet-actions" },
      h(
        "button",
        { class: "secondary", type: "button", onclick: () => on.openCredits() },
        copy.credits,
      ),
      // A link, not a button that calls `window.open`: the Capacitor shell sends
      // a navigation off its own origin to the system browser, and a link is the
      // one thing a browser tab already knows how to open elsewhere.
      h(
        "a",
        { class: "secondary sheet-link", href: SOURCE_URL, target: "_blank", rel: "noopener" },
        copy.source,
      ),
      h(
        "a",
        { class: "secondary sheet-link", href: reportUrl(), target: "_blank", rel: "noopener" },
        copy.report,
      ),
    ),
    h(
      "button",
      { class: "primary", type: "button", onclick: () => on.closeOverlay() },
      ui().common.close,
    ),
  )
}

/** One row, two sheets. The labels live under `about` because that is its home. */
function sharingSwitch(on: Handlers, chrome: Chrome): HTMLElement {
  const copy = ui().about
  return setting(
    { "data-focus": "sharing", onclick: () => on.setSharing(chrome.sharing !== "on") },
    copy.sharing,
    chrome.sharing === "on",
  )
}

/**
 * The privacy page's section on sharing, which lists every field a run carries.
 * Absolute, and out to the browser, rather than the copy bundled at `privacy/`:
 * inside the APK that would replace the game in its own WebView, with no back
 * button to return by. It is in English only, like the page.
 */
const PRIVACY_URL = `${SITE_URL}/privacy/#sharing`

function privacyLink(): HTMLElement {
  const copy = ui().about.privacy
  return h(
    "span",
    {},
    copy.before,
    h(
      "a",
      { class: "inline-link", href: PRIVACY_URL, target: "_blank", rel: "noopener" },
      copy.link,
    ),
    copy.after,
  )
}

/**
 * What a yes gets back. A player who agreed to send something has done the
 * game a favour, and a switch that silently flips is a favour nobody noticed.
 *
 * In place rather than as a toast: the toast is how this game says a move was
 * refused, and a thank-you in the same panel would read, for its first half
 * second, as having done something wrong.
 */
function thanks(when: "fresh" | "shown"): HTMLElement {
  return h(
    "p",
    { class: `share-thanks ${when === "fresh" ? "fresh" : ""}` },
    icon("heart"),
    h("span", {}, ui().about.thanks),
  )
}

/**
 * A new issue on the bug form in `.github/ISSUE_TEMPLATE/bug_report.yml`, with
 * the three fields a player would have to go and look up already filled in:
 * the build stamp off the title screen, where they play, and the language.
 * Each query key is a field `id` in that form.
 *
 * Nothing leaves the device by opening it. The player sees every word on
 * GitHub's page before anything is posted, which is why this can carry the
 * platform when the telemetry, which is sent unseen, carries no such thing.
 *
 * The shells are told from the site by where they serve the bundle: Capacitor
 * from `https://localhost`, which the dev server's plain-http `localhost` is
 * not, and the desktop build from Tauri's: `tauri://localhost` on Linux,
 * `https://tauri.localhost` on Windows (see desktop/src-tauri/src/main.rs).
 */
function reportUrl(): string {
  const params = new URLSearchParams({
    template: "bug_report.yml",
    version: buildStamp(),
    platform:
      location.protocol === "tauri:" || location.hostname === "tauri.localhost"
        ? "Desktop app"
        : location.protocol === "https:" && location.hostname === "localhost"
          ? "Android app"
          : "Browser",
    language: lang(),
  })
  return `${SOURCE_URL}/issues/new?${params}`
}

/**
 * Who made what. The effects are CC0, so none of that is owed; it is here
 * because the people are worth naming. The music links to its composers. `sounds/CREDITS.md` and
 * `tracks/CREDITS.md` are the ledger of which file came from where, and this is
 * the reader's version of the same two tables.
 */
const FREESOUND = ["tonmayster", "Paloma.SSSS", "NachtmahrTV", "plasterbrain"]

/** Both tracks the pause sheet can switch between, in the order it steps through them. */
const MUSIC = [
  { piece: "promises", artist: "kate", url: "https://kate.garden/" },
  {
    piece: "Forget-me-not in F major",
    artist: "Kistol",
    url: "https://opengameart.org/users/kistol",
  },
]

/** A `rule` row, each artist's name a link; see `privacyLink` for why a link. */
function musicCredit(): HTMLElement {
  const copy = ui().credits
  return h(
    "div",
    { class: "rule" },
    h("strong", {}, copy.music),
    ...MUSIC.map(({ piece, artist, url }) => {
      const { before, after } = copy.musicText(piece)
      return h(
        "span",
        {},
        ` ${before}`,
        h("a", { class: "inline-link", href: url, target: "_blank", rel: "noopener" }, artist),
        after,
      )
    }),
  )
}

/** The typefaces' designers (IBM Plex, Jost), named in every language as the other credits' people are. */
const FONT_DESIGNER = "Mike Abbink, Bold Monday, Owen Earl"

/** Whose the emoji are (`emoji.ts`); the handful named `5w-` are drawn here and owe nobody. */
const EMOJI_MAKER = "Microsoft"

export function creditsView(on: Handlers): HTMLElement {
  const copy = ui().credits
  const people = new Intl.ListFormat(lang(), { type: "conjunction" }).format(FREESOUND)
  return overlay(
    on,
    h("h2", { class: "sheet-title" }, copy.title),
    h(
      "div",
      SHEET_BODY_ATTRS,
      rule({ term: copy.madeBy, text: "Jamison Lahman" }),
      musicCredit(),
      rule({ term: copy.sounds, text: copy.soundsText(people) }),
      rule({ term: copy.licence, text: copy.licenceText }),
      // The typeface ships with the desktop table only, but the credit is owed
      // by the build that carries the file, not by the look that draws with it.
      rule({ term: copy.font, text: `${FONT_DESIGNER}: ${copy.fontText}` }),
      // Owed on the same terms: Classic alone draws them, and every build ships
      // them. MIT asks that its notice travel with the copies, and the bundle
      // carries the pictures as strings with no file beside them to say so.
      rule({ term: copy.emoji, text: `${EMOJI_MAKER}: ${copy.emojiText}` }),
    ),
    // Back to the sheet it was opened from rather than closed outright, since
    // that is the only way in.
    h("button", { class: "primary", type: "button", onclick: () => on.openAbout() }, copy.back),
  )
}

/**
 * One row of the pause sheet's settings list.
 *
 * `on` is present only for the two that are switches, and draws the track at
 * the row's end. The label already says the state ("Sound on"), which is the
 * trouble with it alone: a button reading "Sound on" is equally a report and an
 * instruction, and a player has to tap it to find out which. The track answers
 * that without a word a translator has to be asked for, and `aria-checked` says
 * the same to a screen reader.
 *
 * `data-focus` on each, so a keyboard player toggling sound keeps their place in
 * the list across the rebuild instead of being thrown back to the sheet's top.
 */
/**
 * One row of a settings list, and every row is the same two columns: what the
 * setting is called on the left, what it is set to on the right. A switch (`end`
 * a boolean) is its own value. Anything else is written out, quieter than the
 * name, with a chevron after it, because a value on the right of a row reads as
 * information until something says the row can be pressed.
 *
 * The rows used to be sentences instead ("Music off", "Track: promises",
 * "Animation speed ×2"), which put the part that changes somewhere different on
 * every row and, in Spanish and German, far enough along to wrap. Naming the
 * setting and not its state is also what a switch wants read aloud: the role
 * and `aria-checked` already say on or off, so "Music off, switch, off" said
 * it twice.
 */
function setting(
  attrs: { "data-focus": string; disabled?: boolean; onclick: () => void },
  label: string,
  end?: boolean | Node | string,
): HTMLElement {
  const toggle = typeof end === "boolean"
  return h(
    "button",
    {
      class: "setting",
      type: "button",
      role: toggle ? "switch" : undefined,
      "aria-checked": toggle ? String(end) : undefined,
      ...attrs,
    },
    h("span", { class: "setting-label" }, label),
    toggle
      ? h("span", { class: "switch", "aria-hidden": "true" })
      : end === undefined
        ? null
        : h(
            "span",
            { class: "setting-value" },
            end,
            h("span", { class: "setting-more", "aria-hidden": "true" }, "›"),
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
      SHEET_BODY_ATTRS,
      // Two whole sentences in the catalog rather than one with "of N" spliced
      // in, because a run past the last stage has no denominator and the clause
      // a language wants that number in is not always the one English puts it in.
      h(
        "p",
        {},
        state.won ? copy.bodyEndless(state.stage, round) : copy.body(state.stage, STAGES, round),
      ),
    ),
    // Built to the pause sheet's shape, Quit in `.sheet-actions` and the way
    // back below it on the sheet's own gap, because this sheet replaces that one
    // with the same button under the same thumb. Both inside the stack, as they
    // were, put 0.5rem under Quit where the pause sheet has 0.75rem, and the
    // button the player just pressed landed 4px lower than where they pressed it.
    h(
      "div",
      { class: "sheet-actions" },
      h("button", { class: "danger", type: "button", onclick: () => on.quit() }, copy.confirm),
    ),
    h("button", { class: "primary", type: "button", onclick: () => on.openMenu() }, copy.cancel),
  )
}

/**
 * The run in hand's code, and a tap copies a link that deals it again.
 *
 * A link rather than the bare code, because a link is the thing a friend can
 * act on without being told where the field is, and it carries the ascension
 * and the word list, which the code alone does not. The code is still on the
 * face, so it can be read aloud or typed into a phone that cannot open links.
 */
function seedLine(code: string, on: Handlers): HTMLElement {
  const copy = ui().seed
  return h(
    "button",
    {
      class: "seed-line",
      type: "button",
      "data-focus": "seed",
      "aria-label": `${copy.line(code)}. ${copy.copy}`,
      onclick: () => on.copySeed(),
    },
    h("span", {}, copy.line(code)),
    icon("copy"),
  )
}

/** What the seed sheet needs that is not in the catalog. */
export type SeedSheet = {
  /** The field's text, as the app last heard it. */
  draft: string
  /** The level the run will be dealt at. */
  ascension: number
  /** The list it will be dealt from, only when that is not the interface's. */
  words: Lang | null
  /** Whether a run is open, which Play would end. */
  inRun: boolean
}

/**
 * The seed sheet: the version stamp's destination, and a link's.
 *
 * The build is written out in full at the foot, commit and all, because the
 * stamp that opened this hides the commit on a phone and the commit is the half
 * a bug report needs; a player who came here to read the version finds it.
 *
 * The field is the only `<input>` in the game, and the rebuild is why it takes
 * the care it does: the value is written from `draft` on every render, and
 * typing tells the app without asking for one. See `editSeed`.
 */
export function seedView(sheet: SeedSheet, on: Handlers): HTMLElement {
  const copy = ui().seed
  return overlay(
    on,
    h("h2", { class: "sheet-title" }, copy.title),
    h("p", { class: "sheet-note" }, copy.blurb),
    h(
      "label",
      { class: "seed-field" },
      h("span", { class: "seed-field-label" }, copy.field),
      h("input", {
        class: "seed-input",
        type: "text",
        "data-focus": "seed-input",
        value: sheet.draft,
        // No `maxlength`: it cut a pasted link to its first nine characters,
        // which is never a code. `parseSeed` is where length is judged.
        autocomplete: "off",
        autocapitalize: "characters",
        autocorrect: "off",
        spellcheck: "false",
        enterkeyhint: "go",
        // A code that could be real. 35 bits spell 31, so every seed's first
        // character is 0 or 1, and a placeholder starting with a K would teach
        // the one shape the field refuses.
        placeholder: "1K7QX2M",
        oninput: (event: Event) => on.editSeed((event.target as HTMLInputElement).value),
        onkeydown: (event: Event) => {
          if ((event as KeyboardEvent).key !== "Enter") return
          // Stopped here as well as prevented: by the time it would bubble to the
          // window the sheet is gone and the run's intro card is up, and any key
          // at all deals that card's board, so the Enter that dealt the run
          // skipped the card announcing it.
          event.preventDefault()
          event.stopPropagation()
          on.playSeed()
        },
      }),
    ),
    h(
      "p",
      { class: "sheet-note seed-terms" },
      ui().common.ascension(sheet.ascension),
      sheet.words && ` · ${copy.words(LANG_NAMES[sheet.words])}`,
    ),
    sheet.inRun && h("p", { class: "sheet-note" }, copy.abandons),
    h("p", { class: "sheet-note seed-build" }, buildStamp()),
    h(
      "div",
      { class: "sheet-actions" },
      h(
        "button",
        { class: "secondary", type: "button", onclick: () => on.closeOverlay() },
        ui().common.close,
      ),
    ),
    h("button", { class: "primary", type: "button", onclick: () => on.playSeed() }, copy.play),
  )
}
