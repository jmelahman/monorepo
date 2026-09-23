import type { Action, GameEvent, Refusal, RunState, WordSource } from "../engine"
import { MODIFIER_BY_ID, MULT_FOR_COLOR, reduce, startRun } from "../engine"
import { Sound } from "./audio"
import type { CoachStep } from "./coach"
import { coachAsks, coachSpent, coachStep } from "./coach"
import { clear, h, wait } from "./dom"
import { formatNumber as num } from "./format"
import type { Lang } from "./lang"
import {
  categoryLevel,
  consumableNote,
  growthBadge,
  loadLang,
  modPlaced,
  NEXT_LANG,
  payoutBadge,
  readLang,
  refusalText,
  setLang,
  ui,
} from "./lang"
import { chosenAscension, Profile } from "./meta"
import type { Mood } from "./music"
import { Music } from "./music"
import { atSpeed, loadSpeed, NEXT_SPEED, setSpeed } from "./speed"
import type { Chrome, Decor, Handlers } from "./views"
import {
  ascendView,
  codexView,
  displacedAt,
  endView,
  fillCategory,
  fillCoach,
  fillReadout,
  helpView,
  introView,
  loadingView,
  menuView,
  meterFill,
  NEXT_DECOR,
  packView,
  placeView,
  quitView,
  rewardView,
  roundView,
  shapesView,
  shopView,
  statsView,
  titleView,
  wordInPlay,
} from "./views"

/**
 * Bumping the suffix orphans every save in the wild, so treat it as a migration.
 *
 * v2 because the vocabulary moved under it. A v1 save spells the same run
 * `ante`, `blindIndex`, `blind`, `jokers`, and `{type:"next_blind"}`; `loadSave`
 * would read it as a run missing half its fields and hand back something that
 * looks playable and is not. Renaming the key is how that save gets refused
 * cleanly rather than half-understood. It costs whoever was mid-run at the
 * upgrade exactly one run. The record survives, which is where the things worth
 * keeping live.
 */
const SAVE_KEY = "5wild:run:v2"

/**
 * Which word list the saved run was dealt from.
 *
 * A sibling key rather than a field on `RunState`, which is engine state: the
 * engine is handed a `WordSource` and never learns there was a choice of them,
 * and adding a field would cost a `v3` bump and every save in the wild for a
 * fact the shell owns.
 *
 * It is not optional, which a run-scoped setting normally would be. On launch
 * the shell has a save and a setting and, if the two disagree, no way to tell
 * whether the setting was changed after the save was written or the save was
 * written under it — and it has to pick a list *before* the app exists. Absent
 * means a save from before this key, which was necessarily English.
 */
const RUN_LANG_KEY = "5wild:run:lang"

/**
 * The shell's half of the word lists: which one arrived, and how to ask for
 * another.
 *
 * Fetching stays in `main.ts`, whose whole job it is. This is the handle back to
 * it, so that a setting changed in a sheet can start a network request without
 * the app knowing what a URL is.
 */
export type WordLists = {
  /** The language the `words` passed alongside this were loaded for. */
  lang: Lang
  load: (lang: Lang) => Promise<WordSource>
}

/**
 * Set once the first round has been coached, or the coaching waved off.
 *
 * The last of the "has been here before" flags. `5wild:seen-help` used to sit
 * beside it, holding that the rules sheet had already interrupted a first
 * launch, and the two were deliberately kept apart because a sheet closed at the
 * title screen is not a round played. Then the sheet stopped interrupting
 * anything (see `start`), which left that key recording an event that no longer
 * happens. This is the one worth keeping: it is spent by playing, which is the
 * only evidence that the teaching landed.
 */
const COACH_KEY = "5wild:coached"

/**
 * How much the board draws on itself: one of `Decor`.
 *
 * Still spelled `plain` after the setting grew a third state, because the key
 * has never shipped: there is no save anywhere holding the `"1"` this used to
 * write, so there is nothing for a rename to rescue and nothing for the old
 * spelling to mean. An unreadable value falls back to `all`, which is also what
 * a first launch gets, so a store that has been blocked or scribbled on lands on
 * the board the game is designed around rather than a stripped one the player
 * never asked for.
 */
const PLAIN_KEY = "5wild:plain"

/**
 * Per-event pacing, in ms. Slow enough to read, fast enough to not be a cutscene.
 *
 * These, and every other duration below that is a motion rather than a reading
 * time, are what the game is *authored* at. What it plays at is these divided by
 * the animation speed the player chose; `beat` is where that happens and
 * `./speed` is why. Nothing here changes when the setting does.
 *
 * `solve` is the outlier on purpose: it is the last beat before the reward screen
 * takes the board away, and it has to outlast `COUNT_UP` by enough that the pile's
 * new total is legible standing still rather than glimpsed mid-climb.
 */
const PACE = { tile: 170, relic: 150, solve: 900, total: 400 }

/**
 * The tile turn. It runs longer than the gap between tiles on purpose, so the
 * reveals overlap into a cascade rather than a queue of separate flips.
 *
 * `total` must stay in step with the `.tile.flip` animation in the stylesheet.
 * CSS owns the motion, this owns when the class comes back off, and a mismatch
 * either clips the flip or leaves the tile stuck mid-turn.
 */
const FLIP = { total: 380, half: 190 }

/** How long the score counts up to its new value. */
const COUNT_UP = 340

/** How long a tile's `+chips +mult` badge lives. Matches `gain-rise` in the CSS. */
const GAIN = 760

/**
 * How long a refusal stays on screen. Counted here rather than in the
 * stylesheet, for the reason `toast` gives at length.
 */
const TOAST = 2200

/**
 * A press long enough to mean "what is this" rather than "do this", and how far
 * a finger may drift before it is a scroll instead. Both are the platform
 * conventions: iOS fires its own long-press at 500ms and Android at 400, so
 * landing under both keeps this from arriving after the browser's own menu.
 */
const HOLD = 350
const HOLD_SLOP = 10

const reducedMotion = (): boolean =>
  window.matchMedia?.("(prefers-reduced-motion: reduce)").matches ?? false

/**
 * Classes the app puts on a live screen that no freshly-built view will carry.
 *
 * They are here rather than inlined because the next one will be added by
 * somebody who has never read `reuseBoard` and will not think to. What they all
 * have in common is that they describe what is *happening* to a screen rather
 * than which screen it is: one names a shake in progress, the other a rebuild
 * that brought nothing new, and a board is a board through both.
 */
const TRANSIENT_SCREEN = ["shaking", "settled"]

/** Which screen this is, ignoring whatever is currently happening to it. */
const screenKind = (node: Element): string =>
  [...node.classList].filter((name) => !TRANSIENT_SCREEN.includes(name)).join(" ")

export class App {
  private state: RunState
  /** True while a scoring animation owns the screen; input is ignored. */
  private busy = false
  /** Set when the player taps mid-animation: the rest of it plays instantly. */
  private skipping = false
  /** True while the round's intro card is up, before the board is dealt. */
  private intro = false
  /** True when there is no run to return to and the front door is showing. */
  private atTitle: boolean
  /** The modal on top of everything, if any. */
  private overlay: "help" | "codex" | "shapes" | "stats" | "menu" | "quit" | "ascend" | null = null
  /**
   * Which sheet the last render put on screen, so this one can tell a sheet
   * arriving from the same sheet being rebuilt underneath the player's thumb.
   * Not `overlay` itself: that field has already been changed by the time a
   * render reads it, and two of the sheets are not named by it at all.
   */
  private sheetShown: string | null = null
  /** Which rung the open lock is offering. Only meaningful while `overlay` is "ascend". */
  private ascendTo = 0
  /**
   * The letter in the picker that has been tapped and is waiting to be confirmed,
   * because it is already carrying a modifier that placing would destroy.
   *
   * Here rather than in `RunState` because it is a half-finished gesture rather
   * than a fact about the run: it must not be saved, must not reach a golden
   * vector, and must not survive putting the phone down. The engine's `placing`
   * is the run's half of this: a modifier is genuinely in hand until it lands,
   * and that does survive a reload.
   */
  private arming: string | null = null
  /** The card or key whose tip is currently up, so re-entering it is not a change. */
  private hovered: HTMLElement | null = null
  /** Counts down the refusal on screen, and is restarted by the next one. */
  private toastTimer: ReturnType<typeof setTimeout> | undefined
  /** How loudly the letters are allowed to announce what they are worth. */
  private decor = loadDecor()
  /**
   * How fast the game plays what it has to say. Every duration below that is an
   * animation rather than a reading time goes through `beat`; see `./speed`.
   */
  private speed = loadSpeed()
  /**
   * The language the interface is in, which the words follow at the next run
   * rather than at this instant. See `cycleLanguage`.
   */
  private lang = loadLang()
  /** Which language `words` was loaded for. Equal to `lang` except while deferred. */
  private wordsLang: Lang
  /**
   * The next run's list, already on its way.
   *
   * `newRun` is called straight out of a click handler and the lists are a
   * ~100 KB fetch, so the alternative to holding this is a button that does
   * nothing for a second. Kicked off the moment the setting changes, which is
   * the earliest the answer is knowable and typically many seconds before it is
   * wanted; by the time it is, it has resolved. Null when nothing is deferred.
   */
  private incoming: Promise<WordSource> | null = null
  /** True only while a run is being held up waiting for one. See `withWords`. */
  private loading = false
  /**
   * True while the first round is still owed its explanation.
   *
   * The only piece of the coaching that is remembered anywhere. Everything else
   * (which beat is up, whether it has been seen before) is read off the run,
   * so this is one boolean rather than a cursor that could disagree with the
   * board. See `src/ui/coach.ts`.
   */
  private coachOwed = !seenCoach()
  private readonly sound = new Sound()
  private readonly music = new Music()
  /** The one thing here that outlives the run being played. */
  private readonly profile = new Profile()

  constructor(
    private readonly root: HTMLElement,
    /**
     * Not `readonly` any more: a language change swaps it, at the next run. Every
     * `reduce` and `startRun` below already reads it per dispatch rather than
     * closing over it, so the swap is the whole of that change.
     */
    private words: WordSource,
    saved: RunState | null,
    private readonly lists: WordLists,
  ) {
    this.wordsLang = lists.lang
    // A run is built either way so the rest of the class never has to deal with
    // a null state; it simply is not persisted until the player commits to it.
    this.state = saved ?? startRun(rootSeed(), words).state
    this.atTitle = saved === null
    // The class and the property are on the document rather than in the render,
    // so they have to be put back on the way in, since the stylesheet is the
    // only thing that remembers.
    setDecor(this.decor)
    setSpeed(this.speed)
    // The shell already put the catalog up, since its own failure screen is
    // written in it; this is the same call and it is not free to skip. A save
    // whose language differs from the setting means the app opens with `words`
    // from one language and a screen in another, and the fetch for the other has
    // to be running before the player can reach the button that needs it.
    setLang(this.lang)
    this.deferWords()
    this.bindPhysicalKeyboard()
    this.bindAudioWake()
    this.bindTips()
  }

  /**
   * Audio may not start before the player has touched something, so the first
   * gesture of the session, whatever it was, is what starts the music. It also
   * stops on the way out: a phone that locks with the tab alive would otherwise
   * keep an oscillator running against the battery all night.
   */
  private bindAudioWake(): void {
    const wake = () => this.music.enable()
    document.addEventListener("pointerdown", wake, { once: true })
    document.addEventListener("keydown", wake, { once: true })
    document.addEventListener("visibilitychange", () => {
      if (document.hidden) this.music.suspend()
      else this.music.resume()
    })
  }

  /** The mood follows the screen, so the shop and the boss do not share a tune. */
  private get mood(): Mood {
    if (this.atTitle) return "title"
    if (this.state.phase === "game_over" || this.state.phase === "victory") return "over"
    if (this.state.phase === "shop" || this.state.phase === "reward") return "shop"
    return this.state.round.bossId ? "boss" : "round"
  }

  start(): void {
    if (this.atTitle) {
      // Nothing opens on top of this screen. The rules sheet used to, once per
      // install behind a `5wild:seen-help` flag, on the grounds that none of this
      // game's scoring is guessable from a Wordle board. That is still true, but
      // the sheet was answering it here, before a board existed, naming a mult to
      // someone who had never seen one, and the coaching now says that half a beat
      // at a time with the live number beside it. What the sheet still owns is the
      // run: shops, bosses and ascensions, which were being read even further ahead
      // of itself, and which sits one tap away under "How to play" on this screen
      // and in the pause menu.
      this.render()
      return
    }
    // A resumed run mid-round goes straight back to the board, since the player was
    // in the middle of a thought, and a card announcing the round they are
    // already playing would be in the way.
    this.intro = this.state.round.guesses.length === 0 && this.state.phase === "round"
    this.render()
  }

  /* ------------------------------------------------------------- dispatch */

  private dispatch(action: Action, typing?: "arriving" | "leaving"): void {
    if (this.busy) return
    const wasPhase = this.state.phase
    const before = this.state
    const { state, events } = reduce(this.state, action, this.words)

    const refusal = events.find((event) => event.type === "rejected")
    if (refusal) {
      this.refuse(refusal.refusal)
      return
    }

    this.state = state
    // Nothing is armed once there is nothing in hand. The commit path clears it
    // on its way through, so what this catches is the run ending underneath an
    // armed key, quitting from the menu over the top of the picker, which
    // would otherwise leave the next run's picker with one letter already half
    // pressed, and that letter would place on a single tap.
    if (!this.state.placing) this.arming = null
    this.tally(before)
    // Every arrival at a round from elsewhere gets the intro card, which is the
    // only thing that makes the shop and the board feel like separate places.
    if (this.state.phase === "round" && wasPhase !== "round") this.intro = true
    this.save()

    // The win is recorded where it is offered rather than where the run ends,
    // because those are no longer the same moment: a run can take the win and
    // then go looking for stage 20. The engine fires this once per run.
    //
    // The level comes off the run rather than off the record, so a win is banked
    // at the difficulty it was actually played at whatever has been chosen since.
    if (events.some((event) => event.type === "run_won")) {
      this.profile.won(this.state.ascension ?? 0)
    }

    const paid = events.some((event) => event.type === "gold")
    if (paid) this.sound.coin()

    // Both are a card leaving the player's hands and landing somewhere, and in
    // the shop there is no keyboard on screen to show where, so the toast is
    // the only confirmation that the Steel went on the E and not the R.
    const consumed = events.find((event) => event.type === "consumable")
    const placed = events.find((event) => event.type === "mod_placed")
    const label = consumed
      ? consumableNote(consumed.note)
      : placed
        ? modPlaced(placed.id, placed.letter)
        : undefined

    // A letter arriving or leaving is the one action that touches a single row
    // and nothing else on the screen, so it is the one action that patches
    // instead of re-rendering. Everything the guard checks is a reason the
    // screen might not be the board this assumes.
    const quiet = !paid && !label && !this.intro && !this.overlay && !this.atTitle
    if (typing && quiet && this.state.phase === "round" && this.patchDraft(typing)) return

    this.render()
    // After the render, not before: the node the bump lands on is built by it.
    if (paid) this.bump(".hud-gold")
    if (label) this.toast(label)
  }

  /**
   * Redraw the row being typed, in place.
   *
   * The alternative this replaces was a full render, which throws the
   * screen away and builds a new one. That is a lot of work to move one letter,
   * but the cost that shows is not the work: `.grid-wrap` is a size container and
   * `.grid` takes its width and its font-size from `cqh`/`cqw`. A container's size
   * is not known until it has been laid out, so a freshly-inserted grid has to be
   * styled twice: once against a container of unknown size, once against the
   * measured one. Both passes are meant to land in the same frame. Where they do
   * not, the first one resolves the fallback declarations, which are `width: 100%`
   * at `font-size: 1.25rem`, a board wider than the real one with letters at the
   * wrong size, and the second corrects it. Five times a word, that is a shake.
   *
   * Reported on Gecko, on a phone. The same build is steady in Chromium on the
   * desktop and in the Chromium WebView the APK runs in, and an earlier fix had
   * already ruled out the browser chrome (see the `svh` note in the stylesheet),
   * which leaves the engine's own timing as the thing that differs.
   *
   * Patching sidesteps the question rather than betting on the answer: the grid is
   * never rebuilt, so there is never a second pass to be late.
   *
   * This covers typing and nothing else, which was the whole of the report at the
   * time. Every other render still built a new container and still flashed; see
   * `reuseBoard`, which keeps the old one instead. That is the general answer and
   * this is the cheap one; the two overlap here on purpose, since a keystroke has
   * no reason to rebuild five rows to move one letter either way.
   *
   * Returns false if the board on screen is not the one this expects, leaving the
   * caller to fall back to a full render.
   */
  private patchDraft(letter: "arriving" | "leaving"): boolean {
    const round = this.state.round
    // `done` is the same condition the view uses to stop drawing a draft at all;
    // it cannot be reached by typing, but the two must not be allowed to disagree.
    if (round.done) return false
    const row = this.root.querySelector(`.grid .row[data-row="${round.guesses.length}"]`)
    if (!row || row.children.length !== round.answer.length) return false

    for (const [column, tile] of Array.from(row.children).entries()) {
      const typed = round.draft[column]
      const revealed = round.revealed[column]
      // Only the letter that just arrived lands. Backspace passes "leaving" and
      // nothing animates: a letter being taken away used to hand the animation
      // to the letter before it, which reads as the board twitching at a tile
      // the player did not touch.
      const lands = letter === "arriving" && column === round.draft.length - 1
      tile.className = typed
        ? `tile filled ${lands ? "land" : ""}`
        : revealed
          ? "tile ghost"
          : "tile"
      tile.textContent = (typed ?? revealed ?? "").toUpperCase()
    }
    // Two things outside the row change with a keystroke. The fifth letter is
    // what gives the word a shape, and naming it only after the guess was
    // submitted would be naming it one guess too late to be worth reading. The
    // readout moves on *every* letter, since every letter is worth chips.
    const slot = this.root.querySelector(".category-slot")
    if (slot) fillCategory(slot, this.state, this.handlers)
    const readout = this.root.querySelector(".readout")
    if (readout) fillReadout(readout, this.state)
    // Third thing, and the one with a number in it: the coaching card quotes the
    // running chip count while the word is being built. It also moves its own
    // anchor as it goes, which is why the light is reapplied rather than left.
    // It also has to be reapplied *after* `fillReadout` in any case, since that
    // rebuilds the readout's children and takes the class down with them.
    const coach = this.root.querySelector(".coach-slot")
    if (coach) fillCoach(coach, this.coach)
    this.lightCoach()
    return true
  }

  /**
   * Submitting is the one action with a story to tell, so it does not go
   * through `dispatch`: the board is drawn first, the event log is replayed
   * over it, and only then does the screen move on to the reward or the
   * game-over card.
   */
  private async submit(): Promise<void> {
    if (this.busy) return
    const before = this.state
    const { state, events } = reduce(this.state, { type: "submit" }, this.words)

    const refusal = events.find((event) => event.type === "rejected")
    if (refusal) {
      this.refuse(refusal.refusal)
      return
    }

    this.state = state
    // The cost of being the one action that skips `dispatch`, and it went unpaid
    // for as long as this path has existed: `tally` was called there and only
    // there, and a submit is the *only* action that plays a guess. So every
    // figure on the record screen that comes off a round — the guess tally, the
    // word counts, the solve histogram, the streaks, the collection — was dead,
    // and it read as a screen of zeroes that nobody could make move. Ahead of
    // `save`, in dispatch's order, since `save` reports the stage to the same
    // record and the two writes should land in the order they happened.
    this.tally(before)
    this.save()

    this.busy = true
    this.skipping = false
    this.render("round")
    await this.animate(events)
    this.busy = false
    this.render()

    if (this.state.phase === "game_over") this.sound.lose()
    else if (this.state.phase === "reward" || this.state.phase === "victory") this.sound.win()
  }

  /* ------------------------------------------------------------ animation */

  private async animate(events: GameEvent[]): Promise<void> {
    const screen = this.root.firstElementChild
    if (!(screen instanceof HTMLElement)) return

    const onSkip = () => {
      this.skipping = true
    }
    screen.addEventListener("pointerdown", onSkip)

    const row = screen.querySelector(`.row[data-row="${this.state.round.guesses.length - 1}"]`)
    const tiles = [...(row?.querySelectorAll(".tile") ?? [])]
    for (const tile of tiles) tile.classList.add("pending")
    // Held back the same way the colors are, and released with the last of
    // them: the boss's summary of a row is only meaningful after the row it
    // summarizes has been seen, and it would otherwise be legible for the whole
    // length of the cascade it is the answer to.
    const note = row?.querySelector(".row-note")
    note?.classList.add("pending")

    const chipsEl = screen.querySelector(".readout .chips")
    const multEl = screen.querySelector(".readout .mult")
    const scoreEl = screen.querySelector(".hud .score")
    // The figures already on screen, so each step can react to the half that
    // moved rather than to both. Which half moved is the information: a gray
    // tile pays chips and nothing else, a green one pays both, and a player who
    // never sees the difference has to be told it in a tutorial instead.
    let shownChips = 0
    let shownMult = 1
    const readout = (chips: number, mult: number) => {
      if (chipsEl && chips !== shownChips) {
        chipsEl.textContent = num(chips)
        this.replay(chipsEl, "bumped", 320)
      }
      if (multEl && mult !== shownMult) {
        multEl.textContent = num(mult)
        this.replay(multEl, "bumped", 320)
      }
      shownChips = chips
      shownMult = mult
    }
    // Wound back for the same reason the total below it is: the board was drawn
    // after the guess was committed, so the readout starts out holding the
    // figure this is about to build up to.
    if (chipsEl) chipsEl.textContent = num(0)
    if (multEl) multEl.textContent = num(1)

    // The bar under the total is driven off the same numbers the count-up walks
    // through, so it fills in step with the digits instead of trailing them.
    const meterEl = screen.querySelector<HTMLElement>(".hud .meter-fill")
    const scoreBox = screen.querySelector(".hud-score")
    const target = this.state.round.target
    const meter = (value: number) => {
      meterEl?.style.setProperty("--fill", String(meterFill(value, target)))
      scoreBox?.classList.toggle("met", value >= target)
    }

    // The state was committed before any of this ran, so the HUD is already
    // showing the total the animation is about to build up to. Wind it back to
    // the pre-guess figure first, or the reveal spoils its own punchline.
    const scored = events.find((event) => event.type === "guess_scored")
    if (scoreEl && scored) scoreEl.textContent = num(scored.total - scored.score)
    /** The figure on screen, so the solve bonus knows what it is multiplying. */
    let onScreen = scored ? scored.total - scored.score : this.state.round.score
    meter(onScreen)

    for (const event of events) {
      switch (event.type) {
        case "tile": {
          const tile = tiles[event.index]
          this.reveal(tile, event.index)
          // Held until the turn is half done, which is when the color appears.
          // Saying what the tile paid before showing what color it came up
          // would answer the question in the wrong order.
          this.tileGain(tile, event.gained)
          if (event.index === tiles.length - 1) this.revealNote(note)
          readout(event.chips, event.mult)
          await this.pace(PACE.tile)
          break
        }
        case "mod": {
          // The tile itself lights up rather than a card in the tray: the thing
          // that fired is the letter, and it is already on screen.
          const tile = tiles[event.index]
          tile?.classList.add("fired")
          this.floater(screen, payoutBadge(event.paid))
          this.sound.relic()
          readout(event.chips, event.mult)
          await this.pace(PACE.relic)
          tile?.classList.remove("fired")
          break
        }
        case "relic": {
          const slot = screen.querySelector(`.relic[data-slot="${event.slot}"]`)
          slot?.classList.add("fired")
          this.floater(screen, payoutBadge(event.paid))
          this.sound.relic()
          readout(event.chips, event.mult)
          await this.pace(PACE.relic)
          slot?.classList.remove("fired")
          break
        }
        case "category": {
          // Lights the line that was already naming this shape on the board, so
          // the label the player read before submitting is the thing that pays.
          const line = screen.querySelector(".category")
          line?.classList.add("fired")
          this.floater(screen, categoryLevel(event.id, event.level))
          this.sound.relic()
          readout(event.chips, event.mult)
          await this.pace(PACE.relic)
          line?.classList.remove("fired")
          break
        }
        case "relic_grew": {
          // Lands after the guess has finished scoring, because that is when it
          // happens: the round ended, and this card is worth more next time. No
          // readout, because nothing about this guess's chips or mult moved, which is
          // exactly what distinguishes growing from firing.
          const slot = screen.querySelector(`.relic[data-slot="${event.slot}"]`)
          slot?.classList.add("fired")
          this.floater(screen, growthBadge(event))
          this.sound.relic()
          await this.pace(PACE.relic)
          slot?.classList.remove("fired")
          break
        }
        case "solve_bonus": {
          // Arrives after the guess has already been counted onto the total, so
          // this is the pile itself multiplying, the biggest number movement in
          // the game, and the one the whole round was building toward.
          this.floater(screen, ui().board.solveFactor(event.factor))
          screen.querySelector(".readout")?.classList.add("solved")
          this.sound.solve()
          this.countUp(scoreEl, onScreen, event.total, meter)
          this.emphasize(screen, event.total / Math.max(1, this.state.round.target))
          onScreen = event.total
          await this.pace(PACE.solve)
          break
        }
        case "guess_scored": {
          // The single most important number in the game, so it is the one
          // thing that animates its value rather than snapping to it.
          const from = event.total - event.score
          this.countUp(scoreEl, from, event.total, meter)
          this.emphasize(screen, event.score / Math.max(1, this.state.round.target))
          this.sound.score(event.score / Math.max(1, this.state.round.target))
          onScreen = event.total
          await this.pace(PACE.total)
          break
        }
        case "letter_destroyed":
          this.floater(screen, ui().board.letterBroken(event.letter))
          this.sound.break()
          await this.pace(PACE.relic)
          break
        default:
          break
      }
    }

    for (const tile of tiles) tile.classList.remove("pending")
    // Belt and braces: a guess that produced no tile events at all, or a skip
    // taken before the cascade reached the end, must not leave the note hidden
    // until the next full rebuild happens to drop it.
    note?.classList.remove("pending")
    screen.removeEventListener("pointerdown", onSkip)
  }

  /**
   * Lets the row's note in at the trough of the last tile's turn, which is the
   * moment that tile's color appears. Same timing as `tileGain`, for the same
   * reason: the answer and the thing it is an answer to arrive together.
   */
  private revealNote(note: Element | null | undefined): void {
    if (!note) return
    if (this.skipping || reducedMotion()) {
      note.classList.remove("pending")
      return
    }
    setTimeout(() => note.classList.remove("pending"), this.beat(FLIP.half))
  }

  /**
   * Wordle's turn-over, done with a scale rather than a pair of stacked faces:
   * the color is swapped at the trough, where the tile is edge-on and there is
   * nothing to see, which is the whole trick.
   */
  private reveal(tile: Element | undefined, index: number): void {
    if (!tile) return
    const color =
      ["green", "yellow", "gray"].find((name) => tile.classList.contains(name)) ?? "gray"
    this.sound.tile(index, color)

    if (this.skipping || reducedMotion()) {
      tile.classList.remove("pending")
      return
    }

    tile.classList.add("flip")
    // Timers rather than awaits: the flips are meant to overlap, so this one
    // must keep running while the next tile starts. Both are harmless if the
    // screen is replaced first, since the node is simply detached by then.
    setTimeout(() => tile.classList.remove("pending"), this.beat(FLIP.half))
    setTimeout(() => tile.classList.remove("flip"), this.beat(FLIP.total))
  }

  /**
   * What a tile just paid, said at the tile.
   *
   * Every other effect in the game announces itself. A relic lights up and
   * floats its number, a modifier lights the letter it rode in on, and the
   * tiles, which are where most of a guess actually comes from, said nothing.
   * The readout moved and the player was left to infer which of the five
   * letters had moved it.
   *
   * Both halves are named, and the chips half is named even when it is zero,
   * because a zero is the whole point under a boss that stops paying for
   * vowels. The mult half only appears when there is one: gray pays no
   * multiplier, and its silence next to a green tile's `+3 mult` is the clearest
   * statement of the rule this game has.
   *
   * The badge hangs off the row and is placed from the tile's own box, which is
   * the same lesson learned twice: a child of the tile would be scaled edge-on
   * by the very flip it is announcing, and a grid item, even one placed
   * explicitly into the tile's cell, perturbs the auto-placement of the five
   * tiles around it and wraps the row. Absolute, off measurements, disturbs
   * neither.
   */
  private tileGain(tile: Element | undefined, chips: number): void {
    const row = tile?.parentElement
    if (!(tile instanceof HTMLElement) || !row) return
    const color = ["green", "yellow", "gray"].find((name) => tile.classList.contains(name))
    const mult = MULT_FOR_COLOR[(color ?? "gray") as keyof typeof MULT_FOR_COLOR]

    const show = () => {
      const node = document.createElement("div")
      node.className = "tile-gain"
      node.style.left = `${tile.offsetLeft + tile.offsetWidth / 2}px`
      // Starts just inside the tile rather than above it, so it is unambiguously
      // this tile's number before it rises away over the row above.
      node.style.top = `${tile.offsetTop + tile.offsetHeight * 0.18}px`
      node.append(h("span", { class: "gain-chips" }, `+${chips}`))
      if (mult > 0) node.append(h("span", { class: "gain-mult" }, `+${mult}`))
      row.append(node)
      setTimeout(() => node.remove(), this.beat(GAIN))
    }

    // Skipping runs the whole guess at once, so the badges would all land
    // together and then all expire together, and five of them stacked on one row
    // is noise, not information. The player asked for the end; give them it.
    if (this.skipping) return
    if (reducedMotion()) show()
    else setTimeout(show, this.beat(FLIP.half))
  }

  /**
   * Counts a number up on screen, snapping instantly if the player skipped.
   *
   * `also` sees every intermediate value, so anything drawn from the same figure,
   * such as the progress bar, moves with the digits rather than after them.
   */
  private countUp(
    node: Element | null,
    from: number,
    to: number,
    also?: (value: number) => void,
  ): void {
    if (!node) return
    if (this.skipping || reducedMotion() || from === to) {
      node.textContent = num(to)
      also?.(to)
      return
    }
    const started = performance.now()
    // Read once rather than per frame: the setting cannot change mid-count, and
    // a climb whose span moved under it would ease out of the wrong number.
    const span = this.beat(COUNT_UP)
    const tick = (now: number) => {
      const progress = Math.min(1, (now - started) / span)
      // Ease out: most of the distance is covered early, so the number reads as
      // arriving rather than crawling.
      const eased = 1 - (1 - progress) ** 3
      const value = Math.round(from + (to - from) * eased)
      node.textContent = num(value)
      also?.(value)
      if (progress < 1 && !this.skipping) requestAnimationFrame(tick)
      else {
        node.textContent = num(to)
        also?.(to)
      }
    }
    requestAnimationFrame(tick)
  }

  /**
   * Weight of the reaction, scaled by what the guess was worth against the
   * target. A chip guess twitches; a guess that clears the round on its own
   * shakes the screen.
   */
  private emphasize(screen: HTMLElement, ratio: number): void {
    const readout = screen.querySelector(".readout")
    readout?.classList.remove("popped")
    void (readout as HTMLElement | null)?.offsetWidth
    if (readout instanceof HTMLElement) {
      readout.style.setProperty("--pop", String(1 + Math.min(0.5, ratio * 0.6)))
      readout.classList.add("popped")
    }
    if (ratio < 0.5 || reducedMotion()) return
    screen.style.setProperty("--shake", `${Math.min(8, 3 + ratio * 4).toFixed(1)}px`)
    screen.classList.add("shaking")
    setTimeout(() => screen.classList.remove("shaking"), this.beat(420))
  }

  /**
   * A duration as authored, at the speed the player asked for.
   *
   * Everything the scoring animation waits on comes through here, and every one
   * of those numbers has a partner in the stylesheet scaled by `--pace` from the
   * same setting, so a class still comes off its element exactly when the
   * animation it names ends. The constants keep their authored values rather
   * than being scaled once at load, because they are what the stylesheet is
   * written against and a `380` that sometimes means 127 is a comment that lies.
   *
   * It is deliberately not on the toast, the long press, or anything else that
   * is a duration without being a motion; see `./speed`.
   */
  private beat(ms: number): number {
    return atSpeed(ms, this.speed)
  }

  private pace(ms: number): Promise<void> {
    return this.skipping ? Promise.resolve() : wait(this.beat(ms))
  }

  private floater(screen: HTMLElement, text: string): void {
    const host = screen.querySelector(".readout")
    if (!host) return
    const node = document.createElement("div")
    node.className = "floater"
    node.textContent = text
    host.append(node)
    setTimeout(() => node.remove(), this.beat(900))
  }

  /**
   * A refusal, said twice: the toast gives the reason and the row moves.
   *
   * Wordle's shake is worth keeping because it answers the question a player
   * actually has, *which* of the things on screen was refused, in the half
   * second before they get round to reading the sentence.
   */
  private refuse(refusal: Refusal): void {
    this.toast(refusalText(refusal))
    if (this.state.phase !== "round") return
    const row = this.root.querySelector(`.row[data-row="${this.state.round.guesses.length}"]`)
    this.replay(row, "rejected", 420)
  }

  /** A one-shot class, restarted if it is already running. */
  private replay(node: Element | null, name: string, ms: number): void {
    if (!(node instanceof HTMLElement)) return
    node.classList.remove(name)
    // Forcing a reflow restarts the animation when two of these land in a row.
    void node.offsetWidth
    node.classList.add(name)
    setTimeout(() => node.classList.remove(name), this.beat(ms))
  }

  private bump(selector: string): void {
    this.replay(this.root.querySelector(selector), "bumped", 320)
  }

  /* ------------------------------------------------------------------ tips */

  /**
   * Read what a thing does without committing to it.
   *
   * Delegated from the root because the screen is thrown away and rebuilt on
   * every render, and keyed on `data-tip` rather than on the relic class,
   * because a letter needs the same panel as a card: the pips on a key say
   * `+3` and `?` and stop there, and the arithmetic behind them belongs
   * somewhere a player can reach mid-round.
   *
   * Hover is the desktop half. Touch gets `bindHeldTips`, because there is no
   * hovering a phone. The keyboard gets `bindFocusTips`, because it has neither
   * gesture and the tray is otherwise a row of cards it can reach and not read.
   *
   * Which half a pointer gets is decided per event, on `pointerType`, and not
   * once at startup on `(hover: hover)`, which is what this used to do, and
   * which is wrong on the one device the game ships as an app. An Android
   * WebView answers that query `true` on a phone with no mouse anywhere near it:
   * it reports the pointer capabilities of a desktop because nothing plumbs the
   * real ones through to it, and Chrome for Android on the same handset answers
   * `false`. Bound on that answer, the touch path ran *and* the hover path ran,
   * and the hover path is fatal to it: Chromium fires `pointerover` as the
   * finger lands and the whole `pointerleave` chain as it lifts. Traced in the
   * APK's engine with the query forced true: panel up at 48ms on the touch-down,
   * down at 49ms on the `pointerdown` that follows it, up again at 399ms when
   * the hold finally fired, and gone at 964ms the instant the finger left. A
   * player holding a key to ask what it pays got the answer snatched away on
   * release, and a plain tap got a panel flashed at them for one frame.
   *
   * `pointerType` cannot lie in the same way: it is a property of the event that
   * actually happened rather than a guess about the hardware. A hybrid, a
   * touchscreen laptop or a tablet with a trackpad, was mishandled by the old
   * gate for the same reason and is now simply two pointers, each with the half
   * that suits it. A pen is left to the hold: it can hover, but it taps far more
   * often than it hovers, and one that opened a panel on every tap would be the
   * WebView bug again with a smaller audience.
   */
  private bindTips(): void {
    this.bindHeldTips()
    this.bindFocusTips()
    this.root.addEventListener("pointerover", (event) => {
      if (event.pointerType !== "mouse") return
      this.showTip(this.tipHost(event.target))
    })
    // `pointerover` covers every move within the screen; this covers the one
    // move that fires nothing: straight out of the window.
    this.root.addEventListener("pointerleave", (event) => {
      if (event.pointerType !== "mouse") return
      this.showTip(null)
    })
  }

  /**
   * Press and hold, for the pointers that cannot hover.
   *
   * The hold has to swallow the tap that ends it, or asking what R is worth
   * types an R, which is the whole reason this is not simply "tap to show".
   * The click is caught in the capture phase at the root, which is upstream of
   * the button's own handler and so the only place that can stop it without the
   * key knowing anything about tips.
   *
   * Any movement cancels: a hold that has turned into a scroll is a scroll.
   */
  private bindHeldTips(): void {
    let timer: ReturnType<typeof setTimeout> | null = null
    let from: { x: number; y: number } | null = null
    /** Set by a hold that fired, and consumed by the click it belongs to. */
    let swallow = false

    const cancel = () => {
      if (timer !== null) clearTimeout(timer)
      timer = null
      from = null
    }

    this.root.addEventListener("pointerdown", (event) => {
      this.showTip(null)
      // Cleared here rather than only where it is consumed: a hold whose click
      // never arrives because the browser swallowed it for a scroll, would otherwise
      // leave this armed and eat an unrelated tap later.
      swallow = false
      if (event.pointerType === "mouse") return
      const host = this.tipHost(event.target)
      if (!host) return
      from = { x: event.clientX, y: event.clientY }
      timer = setTimeout(() => {
        swallow = true
        this.showTip(host)
      }, HOLD)
    })

    this.root.addEventListener("pointermove", (event) => {
      if (!from) return
      if (Math.hypot(event.clientX - from.x, event.clientY - from.y) > HOLD_SLOP) cancel()
    })
    this.root.addEventListener("pointerup", cancel)
    this.root.addEventListener("pointercancel", cancel)

    this.root.addEventListener(
      "click",
      (event) => {
        if (!swallow) return
        swallow = false
        event.stopPropagation()
        event.preventDefault()
      },
      true,
    )
  }

  /**
   * Tab to a thing, read what it does.
   *
   * The tray's cards are not pressable, since a relic is a card you consult and
   * not a button, so landing on one is the only way a keyboard has of asking, and
   * this is the answer. Bound to `data-tip` like the other two halves rather
   * than to the tray, because a key tabbed to asks the same question and the
   * sentence is already hanging on it.
   *
   * `:focus-visible` rather than plain focus is the whole subtlety here. A click
   * and a tap both focus what they hit, so without the guard a mouse would get
   * the tip twice over and a thumb would get one it could not put down: the
   * board patches rather than rebuilds while a word is being typed, so a tip
   * pinned by a tap on a key would sit over the round until the guess landed.
   * The browser already tracks whether the keyboard is what put focus there, and
   * this is that answer rather than a second guess at it.
   */
  private bindFocusTips(): void {
    this.root.addEventListener("focusin", (event) => {
      const target = event.target
      if (!(target instanceof HTMLElement) || !target.matches(":focus-visible")) return
      const host = this.tipHost(target)
      if (host) this.showTip(host)
    })
    // Only the tip that belongs to what is leaving. A pointer resting on a card
    // while the keyboard tabs away from something else is still hovering it, and
    // an unconditional clear here would take that panel down with the other one.
    this.root.addEventListener("focusout", (event) => {
      const target = event.target
      const host = target instanceof Element ? target.closest<HTMLElement>("[data-tip]") : null
      if (host && host === this.hovered) this.showTip(null)
    })
  }

  /**
   * The `[data-tip]` a pointer is over, or null.
   *
   * A tile still waiting to turn over is not one. The row is drawn complete and
   * then held back, with `.pending` coming off tile by tile as the score walks
   * across it, so between the submit and the cascade the board is carrying five tips
   * that describe colors nobody has been shown yet. The stylesheet already
   * holds the modifier's dot back for exactly this reason; a panel that answered
   * early would spoil the same reveal in sentences instead of in a dot.
   *
   * Cleared rather than deferred, because there is nothing to defer to: the next
   * pointer move over a tile that has since turned brings its tip up normally,
   * and a player waiting on the flip is watching it rather than reading.
   */
  private tipHost(target: EventTarget | null): HTMLElement | null {
    const host = target instanceof Element ? target.closest<HTMLElement>("[data-tip]") : null
    return host?.classList.contains("pending") ? null : host
  }

  private showTip(host: HTMLElement | null): void {
    if (host === this.hovered) return
    this.hovered = host
    const tip = this.root.querySelector<HTMLElement>(".relic-tip")
    if (!tip) return
    if (!host) {
      tip.classList.remove("show")
      return
    }

    tip.textContent = host.dataset.tip ?? ""
    // Borrowing the card's rarity keeps the two reading as one object, which a
    // sibling of the tray cannot do by inheritance. A key has no rarity and
    // takes the common edge, which is the neutral one.
    tip.className = `relic-tip rarity-${host.dataset.rarity ?? "common"}`

    // Measured before it is shown, which `visibility: hidden` allows and
    // `display: none` would not: the height decides which side it goes on.
    const card = host.getBoundingClientRect()
    const box = tip.getBoundingClientRect()
    const gap = 6
    const edge = 8
    // Below by preference, since in a round the tray sits under the HUD with the
    // whole board beneath it. The shop keeps its relics at the foot of the
    // screen, and there this flips.
    const below = card.bottom + gap + box.height + edge <= window.innerHeight
    const centered = card.left + card.width / 2 - box.width / 2
    const left = Math.min(Math.max(edge, centered), window.innerWidth - box.width - edge)

    tip.style.setProperty("--slide", below ? "0.25rem" : "-0.25rem")
    tip.style.top = `${Math.round(below ? card.bottom + gap : card.top - gap - box.height)}px`
    tip.style.left = `${Math.round(left)}px`
    tip.classList.add("show")
  }

  /**
   * Say why, and keep saying it for long enough to be read.
   *
   * The dwell is counted here rather than written as a keyframe, and that is the
   * whole point of the method. A refusal is the only channel this game has for
   * telling a player their tap was wrong, and as a `2200ms forwards` animation
   * ending at `opacity: 0` it was at the mercy of the blanket reduced-motion
   * kill-switch in the stylesheet: `animation-duration: 1ms !important` runs the
   * whole thing in a millisecond, `forwards` holds the last keyframe, and the
   * last keyframe is the one where the message is gone. Sampled through a full
   * dwell with the preference set, the toast read `opacity: 0.00` at every
   * point (+0, +60, +250, +1000, +2000, +2400ms), against 1.00 → 0.33 → 0.00
   * without it. Not shortened: deleted, in the frame it was raised. That is
   * "Remove animations" on an Android phone, which the APK's WebView passes
   * straight through, and it took the sentence with it.
   *
   * So the class stays on for as long as a timer says and the stylesheet is left
   * with a transition, which the kill-switch may flatten to 1ms with nothing
   * lost: a message that appears instantly and stays 2.2 seconds is exactly what
   * somebody who turned animations off asked for. Only the fade is decoration.
   * The dwell is the message.
   *
   * Two in a row no longer restart a fade-in: the timer is reset and the text
   * swapped under a panel that never left. The repeat is not lost: `refuse`
   * shakes the row every time, which is the faster half of that answer anyway,
   * and a pop here would have to be a transform, which is already spoken for by
   * the `translateX(-50%)` holding the panel in the middle of the screen.
   */
  private toast(message: string): void {
    const host = this.root.querySelector(".toast")
    if (!host) return
    host.textContent = message
    host.classList.add("show")
    clearTimeout(this.toastTimer)
    this.toastTimer = setTimeout(() => host.classList.remove("show"), TOAST)
  }

  /* --------------------------------------------------------------- render */

  private readonly handlers: Handlers = {
    key: (letter) => {
      this.sound.key()
      this.dispatch({ type: "type_letter", letter }, "arriving")
    },
    enter: () => void this.submit(),
    back: () => this.dispatch({ type: "backspace" }, "leaving"),
    useConsumable: (index) => this.dispatch({ type: "use_consumable", index }),
    collect: () => this.dispatch({ type: "collect" }),
    buy: (index) => this.dispatch({ type: "buy", index }),
    sell: (index) => this.dispatch({ type: "sell_relic", index }),
    reroll: () => this.dispatch({ type: "reroll" }),
    nextRound: () => this.dispatch({ type: "next_round" }),
    continueRun: () => this.dispatch({ type: "continue_run" }),
    pickPack: (index) => this.dispatch({ type: "pick_pack", index }),
    skipPack: () => this.dispatch({ type: "skip_pack" }),
    /**
     * The picker's one tap, or the first of its two.
     *
     * A letter with nothing on it places immediately, which is the ordinary case
     * and the whole alphabet on the first visit. A letter already carrying a
     * modifier arms instead, and the sheet turns into a question naming what
     * would be lost; the second tap on the same letter, or the Replace button,
     * which comes back through here with the same letter, is what places it.
     *
     * Both the keys and the physical keyboard land here, which is the reason the
     * decision is made in this method rather than in the view. Typing a letter
     * at this sheet is a real input path, and it is the one where a wrong answer
     * is cheapest to give: a G aimed at an F destroys whatever was on the G with
     * no gesture in between that could have been aimed badly.
     */
    placeMod: (letter) => {
      this.sound.key()
      const armed = this.arming === letter
      // The same call the sheet makes to decide whether to draw the question, so
      // that the tap and the screen it produces cannot mean different things.
      const modifier = this.state.placing ? MODIFIER_BY_ID.get(this.state.placing) : undefined
      const trade =
        modifier !== undefined && displacedAt(this.state, modifier, letter) !== undefined

      if (trade && !armed) {
        this.arming = letter
        this.render()
        return
      }
      this.arming = null
      this.dispatch({ type: "place_mod", letter })
    },
    // The modifier stays in hand. This backs out of the letter, not out of the
    // purchase, which the engine would not allow anyway.
    cancelPlace: () => {
      this.arming = null
      this.render()
    },
    newRun: () => {
      // The one place the two clocks meet. Everything after the await is what
      // `newRun` has always done; the await itself is almost always already
      // settled, because the fetch started when the setting did.
      void this.withWords(() => this.startFresh())
    },
    // Kept on the record rather than in a field of this class, because it
    // outlives the session: the dial is where the player last left it, not where
    // this launch found it. `newRun` reads the same place, so there is one
    // answer to what level the next run starts at.
    setAscension: (level) => {
      this.profile.chose(level)
      this.render()
    },
    play: () => {
      this.intro = false
      this.render()
    },
    mute: () => {
      // The effects switch carries the music with it when it silences the game,
      // because a player reaching for it wants quiet, not a quieter mix. Turning
      // sound back on only revives music that was not switched off on its own.
      const muted = this.sound.toggleMute()
      if (muted) this.music.suspend()
      else this.music.resume()
      this.render()
    },
    toggleMusic: () => {
      this.music.toggle()
      this.render()
    },
    cycleDecor: () => {
      this.decor = NEXT_DECOR[this.decor]
      setDecor(this.decor)
      this.render()
    },
    cycleSpeed: () => {
      this.speed = NEXT_SPEED[this.speed]
      setSpeed(this.speed)
      this.render()
    },
    /**
     * The interface changes now; the words change at the next run.
     *
     * Two clocks because they answer to different things. The interface is
     * repainted on every dispatch anyway, so a new catalog costs one render and
     * nothing else. The words are the run: the answer was drawn from one list and
     * every guess is validated against it, so swapping the list under a live run
     * strands the answer outside `allowed` and the only honest thing left to do
     * is discard the run. A settings tap must not cost a run, so it does not.
     *
     * What that permits is a Spanish interface over an English run, which is odd
     * but is at least true, and it ends by itself at the next `newRun`. The
     * button says so while it is the case; see `wordsDeferred`.
     */
    cycleLanguage: () => {
      this.lang = NEXT_LANG[this.lang]
      setLang(this.lang)
      this.deferWords()
      this.render()
    },
    openMenu: () => {
      // Mid-animation the screen belongs to the scoring; the button is on the
      // HUD the whole time, so this is a reachable tap rather than a theory.
      if (this.busy) return
      this.overlay = "menu"
      this.render()
    },
    openHelp: () => {
      this.overlay = "help"
      this.render()
    },
    // Declined rather than deferred. The offer is made once, on the intro card
    // of the round the cards would run on, so a flag that let them come back
    // would be letting them back in through a door that is never opened again.
    //
    // It plays the round on the way past, because the button it sits beside is
    // the one that starts it: two buttons where only one starts the game would
    // be a screen that answers a question and then asks the player to confirm
    // they still want to play.
    skipCoach: () => {
      this.coachOwed = false
      markCoachSeen()
      this.intro = false
      this.render()
    },
    openCodex: () => {
      this.overlay = "codex"
      this.render()
    },
    openShapes: () => {
      this.overlay = "shapes"
      this.render()
    },
    openStats: () => {
      this.overlay = "stats"
      this.render()
    },
    closeOverlay: () => {
      this.overlay = null
      this.render()
    },
    askQuit: () => {
      this.overlay = "quit"
      this.render()
    },
    // Mirrors askQuit/quit: the sheet is opened with the thing it is about, and
    // a second handler commits it. The level is held here rather than passed
    // back through the button because the sheet is rebuilt on every render and
    // the button that opened it is long gone by the time the answer arrives.
    askAscend: (level) => {
      this.ascendTo = level
      this.overlay = "ascend"
      this.render()
    },
    ascend: () => {
      this.profile.chose(this.ascendTo)
      this.overlay = null
      this.render()
    },
    quit: () => {
      clearSave()
      // Back to a run nobody is playing, purely so `state` stays non-null. The
      // player gets one at the title screen when they ask for it.
      this.state = startRun(rootSeed(), this.words).state
      this.atTitle = true
      this.overlay = null
      this.intro = false
      this.render()
    },
  }

  private get chrome(): Chrome {
    return {
      muted: this.sound.isMuted,
      musicOff: this.music.isOff,
      decor: this.decor,
      speed: this.speed,
      lang: this.lang,
      wordsDeferred: this.wordsDeferred,
      coach: this.coach,
      coachOffer: this.coachOffer,
    }
  }

  /**
   * Whether the pause sheet owes the player the sentence about when words change.
   *
   * Both halves, and both are load-bearing. A run has to be open, or there is
   * nothing being deferred and the line is a warning about nothing — which is
   * the state the title screen is in, and it is where the language button is most
   * likely to be used. And the languages have to actually differ, so that
   * cycling all the way around mid-run leaves no line behind claiming a change
   * that has un-happened.
   */
  private get wordsDeferred(): boolean {
    return !this.atTitle && this.lang !== this.wordsLang
  }

  /**
   * Start fetching the list the next run will want, or stop caring about one.
   *
   * Called whenever either side of the comparison moves: the setting, in
   * `cycleLanguage`, and the run's list, in `withWords`. Switching away and back
   * drops the request rather than holding a stale promise for a language that is
   * no longer wanted; the browser has the response cached either way, so the
   * cost of having asked is one request that nothing awaits.
   */
  private deferWords(): void {
    this.incoming = this.lang === this.wordsLang ? null : this.lists.load(this.lang)
  }

  /**
   * Put the deferred list in hand, then do the thing that needed it.
   *
   * The screen goes to a loading card only if there is genuinely something to
   * wait for, which on every ordinary path there is not: the fetch started when
   * the setting changed, seconds or minutes ago, and this await settles in the
   * same tick. A first switch on a slow connection is the case that sees the
   * card, and it is the case that would otherwise see a Play button that did
   * nothing.
   *
   * A failed fetch keeps the list already in hand and plays on in it, rather
   * than refusing to start a run. The interface is in the new language and the
   * words are not, which is exactly the state the deferral describes anyway, and
   * the next attempt will try the fetch again.
   */
  private async withWords(then: () => void): Promise<void> {
    const incoming = this.incoming
    if (incoming) {
      this.loading = true
      this.render()
      try {
        this.words = await incoming
        this.wordsLang = this.lang
      } catch {
        // Left in the old language, and `deferWords` below will ask again.
      }
      this.loading = false
      this.deferWords()
    }
    then()
  }

  /** The body of `newRun`, once there is a word list to start one from. */
  private startFresh(): void {
    this.state = startRun(rootSeed(), this.words, chosenAscension(this.profile.stats)).state
    this.atTitle = false
    this.overlay = null
    this.intro = true
    // Counted here rather than in the constructor: the class always holds a
    // run so that nothing downstream has to handle a null one, and most of
    // those are scaffolding the player never sees.
    this.profile.started()
    // Persisted before the first keypress: a fresh run is already a run, and
    // closing the app on the intro card should not silently reroll the word.
    this.save()
    this.render()
  }

  /**
   * Whether the intro card on screen is the one that has to ask about the
   * tutorial.
   *
   * `coachAsks` is the run's half: this is the first round and nothing has been
   * played in it. Everything ANDed in front is the session's, and it is the same
   * list `coach` carries for the same reason, bar `busy`: the intro card is up
   * before there is anything to animate.
   */
  private get coachOffer(): boolean {
    if (!this.coachOwed || !this.intro || this.overlay || this.atTitle) return false
    return coachAsks(this.state)
  }

  /**
   * The coaching card the board should be showing, if any.
   *
   * `coachStep` decides what the *run* has to say; everything ANDed in front of
   * it is what the *screen* is doing, and each one is a screen the card would be
   * wrong on. Mid-animation is the interesting one: the beat after a guess lands
   * is true the instant the guess is recorded, and showing it then would put the
   * card's `chips × mult = score` on screen several seconds before the tiles
   * finish turning over to reveal it, the arithmetic spoiled ahead of the thing
   * it is describing.
   */
  private get coach(): CoachStep | null {
    if (!this.coachOwed || this.busy || this.intro || this.overlay || this.atTitle) return null
    return coachStep(this.state)
  }

  /**
   * Mark the anchor the card is talking about.
   *
   * Run after the render rather than inside the views, because the anchor is a
   * selector into a screen that does not exist until the render has finished,
   * and because the thing being marked belongs to another view entirely. The
   * card is over the board; the `?` it names is in the readout, and the score it
   * names is up in the HUD. Nothing else on the screen ties two views together,
   * so nothing else needs the class threaded through both.
   *
   * Clearing first covers the patch path, where the previous beat's anchor is
   * still lit and is very often a different element: typing the first letter
   * moves the card from the readout as a whole to the chip count inside it.
   */
  private lightCoach(): void {
    for (const lit of this.root.querySelectorAll(".coached")) lit.classList.remove("coached")
    const step = this.coach
    if (!step) return
    this.root.querySelector(step.anchor)?.classList.add("coached")
  }

  /** `as` forces the round board to stay on screen while its scoring plays out. */
  private render(as?: "round"): void {
    const phase = as ?? this.state.phase
    // Checked here rather than beside the card, and it is deliberately the wider
    // question of the two: `coach` goes quiet on plenty of screens the tutorial
    // has not finished with, such as a sheet, the intro card or the scoring
    // animation, and retiring it on any of those would end it early. This asks whether the
    // round it lives in has gone past it for good, which only a run can answer,
    // so the title screen's scaffolding run is excluded rather than consulted.
    if (this.coachOwed && !this.atTitle && coachSpent(this.state)) {
      this.coachOwed = false
      markCoachSeen()
    }
    this.music.set(this.mood)
    // The card the pointer was over is about to stop existing, and a stale node
    // here would read as "still hovering" and suppress the next tip.
    this.hovered = null
    // Read here, at the top, because everything below this line is the rebuild
    // that destroys the focused node. The name it carries is the only part of it
    // that survives, and `holdFocus` spends it on the far side.
    const keeping =
      document.activeElement instanceof HTMLElement
        ? document.activeElement.dataset.focus
        : undefined
    const view = this.loading
      ? loadingView()
      : this.atTitle
        ? titleView(this.handlers, this.chrome, this.profile.stats)
        : phase === "round" && this.intro && !as
          ? introView(this.state, this.handlers, this.chrome)
          : phase === "reward"
            ? rewardView(this.state, this.handlers)
            : phase === "shop"
              ? shopView(this.state, this.handlers)
              : phase === "game_over" || phase === "victory"
                ? endView(this.state, this.handlers)
                : roundView(this.state, this.handlers, this.chrome)

    // Overlays sit beside the screen rather than replacing it, so the board is
    // still visible behind the sheet and the player keeps their bearings.
    //
    // A menu the player asked for outranks the open pack, so they can still quit
    // or read the rules mid-decision; the pack is waiting underneath when they
    // close it, because the engine will not let the shop move on until it is.
    const sheet =
      this.overlay === "help"
        ? helpView(this.handlers)
        : this.overlay === "codex"
          ? codexView(this.handlers)
          : this.overlay === "shapes"
            ? shapesView(this.state, this.handlers, phase === "round" ? wordInPlay(this.state) : "")
            : this.overlay === "stats"
              ? statsView(
                  this.profile.stats,
                  { answers: this.words.answers.length, allowed: this.words.allowed.size },
                  this.wordsLang,
                  this.handlers,
                )
              : this.overlay === "menu"
                ? menuView(this.handlers, this.chrome)
                : this.overlay === "quit"
                  ? quitView(this.state, this.handlers)
                  : this.overlay === "ascend"
                    ? ascendView(this.ascendTo, this.handlers)
                    : // Both are held decisions the engine will not let the shop move
                      // past, and the two cannot be open at once, since buying is refused
                      // while either is. Order is arbitrary; only exclusivity matters.
                      (placeView(this.state, this.handlers, this.arming) ??
                      packView(this.state, this.handlers))

    // Which sheet that is, since the answer decides whether it may announce
    // itself below. `overlay` names seven of them. The two the fallback builds
    // have no name of their own, and are told apart by the run field that
    // decides which of the pair exists at all, which is `placeView`'s own guard.
    const kind = this.overlay ?? (!sheet ? null : this.state.placing ? "place" : "pack")
    // Asked before the swap, because it is a question about the screen that is
    // about to stop being the one on screen.
    const settled = this.settled(view, kind)
    // The sheet's half of the same question, and the simpler one: this node is
    // always the one this render built, since nothing reuses a sheet.
    if (sheet && kind === this.sheetShown) sheet.classList.add("settled")
    this.sheetShown = kind

    if (!this.reuseBoard(view)) clear(this.root).append(view)
    if (sheet) this.root.append(sheet)
    // On whatever is standing there afterwards rather than on `view`, because
    // `reuseBoard` may have kept the live screen and used this one only for its
    // children, and a mark left on a node that was thrown away is no mark at
    // all. Toggled rather than added for the same reason from the other side: a
    // kept screen is still carrying whatever the last render decided, and this
    // render may have decided otherwise. Nothing is painted between the append
    // and this line, so an animation suppressed here never had a frame.
    this.root.firstElementChild?.classList.toggle("settled", settled)
    this.holdFocus(keeping)
    this.lightCoach()
  }

  /**
   * Whether this render is rebuilding the screen rather than bringing one.
   *
   * Every dispatch rebuilds the whole screen, so a node that has not changed is
   * still a new node, and a new node plays its entry animation. That is what an
   * entry animation is for and it is right nearly everywhere, because nearly
   * every render is a screen the player has just arrived at. It is wrong in the
   * one place the player is looking hardest: a button inside a sheet. Sound,
   * music and the animation speed change one word of one label, and the render
   * behind that word dropped the backdrop to `opacity: 0` and rebuilt the sheet
   * 24px low, so the sheet blinked out and rose again on every tap. Over the
   * shop it took the shelf with it, all five cards to `opacity: 0` and dealt
   * back in one at a time behind a sheet that was not going anywhere.
   *
   * The two halves of that are two different questions, so they are asked
   * separately and answered on two different nodes. A *sheet* is settled when
   * the same sheet was up before this render: menu to help still rises, because
   * that is a different sheet, and so does the first one over a screen. A
   * *screen* is settled when it is the same screen as the live one and a sheet
   * is open on one side of the render or the other, which is the whole of "the
   * player is working in a sheet and the scenery behind them did not move". Both
   * sides matter: without the second the shelf re-deals when the menu closes,
   * and without the first it re-deals when it opens.
   *
   * Deliberately not "the same screen" on its own, which would be shorter and
   * would silence the two renders that are the point of the animation: a reroll
   * deals a new shelf, and it is the same shop screen it was dealt onto.
   *
   * What the marks do is in the stylesheet, and the rule for what may listen to
   * them is written there: the animation has to mean "this just appeared", and
   * the node has to be one a view builds rather than one a handler decorates
   * after the fact.
   */
  private settled(view: HTMLElement, kind: string | null): boolean {
    if (kind === null && this.sheetShown === null) return false
    const live = this.root.firstElementChild
    return live instanceof HTMLElement && screenKind(live) === screenKind(view)
  }

  /**
   * Hang the new screen around the old board's container rather than building a
   * new one.
   *
   * `.grid-wrap` is a size container and `.grid` takes its width and its
   * font-size from `cqh`/`cqw`, so the board cannot be styled until the wrap has
   * been measured, and a wrap this render built has not been. On Gecko the
   * first pass over the new board finds no container to ask, drops both `cq`
   * declarations as unresolvable and lands on the fallbacks underneath them:
   * `width: 100%` at `font-size: 1.25rem`. Measured at 360×800, which is what
   * most Android phones report, that is 344×413 where the board is 337×404, with
   * 20px letters where they should be 30.3px: a board seven pixels wider and
   * nine taller than the one beside it a frame ago, with the letters visibly
   * jumping inside it. A second pass corrects it. On a desktop both land in the
   * same frame and nothing shows; on a phone the correction arrives late, and
   * late is what makes it a flash. `patchDraft` already sidesteps this for
   * typing, which is where it was first seen; this is the same sidestep for
   * every other render.
   *
   * It was reported against the decoration switch, which is the cleanest case
   * there is: it toggles two classes on the document root, everything those
   * classes hide is absolutely positioned or drawn inside a fixed box, so not
   * one thing on the round screen changes size, and the board jumped anyway.
   * That is the tell that the trigger is the rebuild rather than anything being
   * rebuilt.
   *
   * So the container is the one node the rebuild is not allowed to have. It is
   * emptied and refilled with this render's own board, and the rest of the
   * screen is spliced in around it, which leaves it holding a frame, and so a
   * size worth reading, from one render to the next. Nothing else is kept:
   * every other node is thrown away as before, views still build from scratch,
   * and none of them need to know this happens.
   *
   * Reusing it is safe because it holds nothing worth rebuilding. It carries no
   * listeners, no attributes but its class, and, the part that makes it work
   * rather than merely tidy, its size cannot depend on what is inside it,
   * because that is what `container-type: size` means. The measurement it hands
   * the new board is the one the new board would have been given.
   *
   * Only a round following a round qualifies. A screen change rebuilds the wrap
   * with everything else, so the first frame of a round still resolves against
   * an unmeasured container, but that is a whole screen arriving, not a board
   * moving under a thumb that is already aiming at it.
   */
  private reuseBoard(view: HTMLElement): boolean {
    const live = this.root.firstElementChild
    // The same kind of screen on both sides, or there is nothing to line up. A
    // board only ever stands in for a board.
    //
    // Compared on kind rather than on the class string, because the live screen
    // has been on screen and the new one has not, and things happen to screens
    // that are on screen. `emphasize` is the case that proved it: a guess worth
    // half the target puts `shaking` on the screen root for 420ms, and the
    // render that ends the cascade is awaited on `PACE.total`, which is 400.
    // Twenty milliseconds is not a race, it is a rule: the render lands inside
    // the shake every time, found a class the new view had no reason to carry,
    // and refused the reuse. It stays a rule at every animation speed, since
    // both numbers are divided by the same setting; at ×3 the margin is seven
    // milliseconds and the ordering is the one it always was. The board was
    // rebuilt against an unmeasured container on the one guess in the round
    // worth watching, which is both the most conspicuous moment available and
    // the hardest to catch, since the screen is already moving. Keeping the node
    // fixes the shake too: it used to be thrown away mid-animation and stop dead.
    if (!(live instanceof HTMLElement) || screenKind(live) !== screenKind(view)) return false
    const kept = live.querySelector(":scope > .grid-wrap")
    const built = view.querySelector(":scope > .grid-wrap")
    if (!kept || !built) return false

    // The board is this render's, as it always was. Only the box it is drawn in
    // is last render's.
    kept.replaceChildren(...built.childNodes)
    // Everything else goes, including any sheet that was open over it, removed
    // one at a time rather than by `replaceChildren`, which would take the kept
    // node out with the rest and put it back afterwards. A node that leaves the
    // document loses the frame this whole exercise is about.
    for (const node of [...this.root.children]) if (node !== live) node.remove()
    for (const node of [...live.childNodes]) if (node !== kept) node.remove()
    // What is left is a screen holding one node, so the new screen is dealt out
    // either side of it, in the order the view put them in. That order is not
    // cosmetic: the wrap is a flex item that claims whatever the items above and
    // below it leave, which is the space the board measures itself against.
    let above = true
    for (const node of [...view.childNodes]) {
      if (node === built) above = false
      else if (above) live.insertBefore(node, kept)
      else live.append(node)
    }
    return true
  }

  /**
   * Put the keyboard inside the sheet, and keep it there, or, with no sheet
   * open, hand the board's focus back to the control that had it.
   *
   * Called after every render rather than only on the one that opened the sheet,
   * because a render throws the whole screen away and builds a new one. The
   * focused node goes in the bin with it, and without this the tab order silently
   * resets to the top of the document behind the backdrop on every action.
   *
   * The sheet itself takes focus rather than its first button: a screen reader
   * then starts at the title and reads down, where landing on "Open the codex"
   * would announce the last thing in the sheet as though it were the point of it.
   *
   * On the board the same rebuild is the problem and a name is the answer. The
   * node is gone, so there is nothing to hold; what survives is the `data-focus`
   * it was carrying, and the new screen is asked for the node wearing the same
   * one. It is opt-in for the reason `data-tip` is: most of the board is letter
   * keys, focus on those is an accident of a tap rather than a place the player
   * meant to be, and the game reads every key off `window` anyway. A control the
   * player deliberately tabbed to is the exception, and it says so itself.
   *
   * The sheet still wins where both apply. A button that opens a sheet is asking
   * for the sheet to be read, not to keep a ring on itself behind the backdrop.
   */
  private holdFocus(keeping?: string): void {
    const sheet = this.root.querySelector<HTMLElement>(".sheet")
    if (!sheet) {
      if (keeping) this.root.querySelector<HTMLElement>(`[data-focus="${keeping}"]`)?.focus()
      return
    }
    if (!sheet.contains(document.activeElement)) sheet.focus()
  }

  /* ----------------------------------------------------------------- save */

  /**
   * Everything the record learns from one action, read off the two states.
   *
   * Off the states rather than off the events, because none of these have an
   * event of their own and inventing four would be putting the profile's
   * questions into the engine's vocabulary. What the record wants to know,
   * which guess found the word and whether a relic is new to the tray, is a
   * difference between two runs, and both runs are right here.
   *
   * Both callers, and that is the thing to keep true: `dispatch` and `submit`
   * are the two places a run advances, and a new one would be a third. Anything
   * that moves the state without coming through here is a round the record never
   * hears about.
   */
  private tally(before: RunState): void {
    const round = this.state.round
    // The round survives into the reward screen, so "same round, one more
    // guess" is a real comparison right up to the moment `next_round` swaps it.
    const played = round.guesses[before.round.guesses.length]
    if (played) this.profile.guessed(played.word, this.wordsLang)
    if (round.solved && !before.round.solved) {
      this.profile.solved(round.answer, round.guesses.length, this.wordsLang)
    } else if (round.done && !before.round.done) {
      this.profile.missed()
    }
    // A relic can arrive from the shop or out of a pack, and the tray is the one
    // place both routes end. Held ids rather than an index, because selling one
    // renumbers the rest.
    const held = new Set(before.relics.map((relic) => relic.id))
    for (const relic of this.state.relics) {
      if (!held.has(relic.id)) this.profile.took(relic.id)
    }
  }

  private save(): void {
    try {
      localStorage.setItem(SAVE_KEY, JSON.stringify(this.state))
      // Beside the run and written in the same breath, because the pair is only
      // meaningful together: a save whose language is a guess is a save that can
      // be resumed against the wrong dictionary, where every legal word is
      // refused and nothing on screen says why.
      localStorage.setItem(RUN_LANG_KEY, this.wordsLang)
    } catch {
      // A full or disabled store costs the player their resume, not their run.
    }
    // Every moment the run is worth persisting is a moment its stage may have
    // moved, so the record rides along here rather than keeping its own watch.
    // It writes only when the mark actually moves.
    this.profile.reached(this.state.stage)
  }

  private bindPhysicalKeyboard(): void {
    // Desktop convenience during development; harmless on a phone.
    window.addEventListener("keydown", (event) => {
      if (event.metaKey || event.ctrlKey || event.altKey) return
      // Tab is the one key an open sheet does not swallow, and the reason this
      // check sits above the overlay branch rather than inside it: the pack and
      // the letter picker are modal too, and they are held by the engine rather
      // than by `overlay`. Everything wearing `.sheet` traps the same way.
      const sheet = this.root.querySelector<HTMLElement>(".sheet")
      if (sheet && event.key === "Tab") {
        trapTab(sheet, event)
        return
      }
      if (this.overlay) {
        // A sheet is modal, so it swallows everything and Escape dismisses it.
        if (event.key === "Escape") {
          this.handlers.closeOverlay()
          event.preventDefault()
          return
        }
        // Enter and Space are how a keyboard presses the button it has tabbed
        // to, and preventing them here is what made "Quit run" unreachable
        // without a mouse: the trap would walk focus onto it and neither key
        // would fire. So the swallow stops short of the two keys that are an
        // activation, and only when focus is inside the sheet. With focus
        // anywhere else Space is a page scroll behind the backdrop, which is
        // the thing modality is for.
        const focused = document.activeElement
        const pressing =
          (event.key === "Enter" || event.key === " ") &&
          focused instanceof HTMLElement &&
          focused.closest(".sheet") !== null
        if (!pressing) event.preventDefault()
        return
      }
      if (this.atTitle) return
      // A control that keeps its focus across a rebuild also owns the two keys
      // that press it, for the same reason the sheet branch above hands them to
      // whatever is focused inside it.
      //
      // Without this, Enter on the tabbed-to switch never reaches the button at
      // all: the round below takes it, `preventDefault` cancels the click the
      // browser was about to synthesize from it, and a keypress the player aimed
      // at a switch submits their guess instead. Space happened to work, because
      // the board has no use for Space and lets it fall through, so the switch
      // answered one of the two keys that mean "press this" and silently did
      // something else with the other.
      //
      // Hung off `data-focus` rather than off "is a button", because the letter
      // keys are buttons too and Enter over a letter key is how the guess gets
      // submitted with a hand still on the keyboard.
      const active = document.activeElement
      if (
        (event.key === "Enter" || event.key === " ") &&
        active instanceof HTMLElement &&
        active.hasAttribute("data-focus")
      ) {
        return
      }
      // A modifier in hand asks a letter question, and this is where letters
      // come from: the one thing outside a round a keypress can answer.
      if (this.state.placing) {
        // Escape is the keyboard's Keep button. It is only bound while a
        // replacement is armed, because that is the only moment this sheet has
        // anything to back out of. The modifier itself is bought and the engine
        // will not take it back, so an Escape at the bare picker would be a key
        // that looks like a way out and is not.
        if (this.arming && event.key === "Escape") {
          this.handlers.cancelPlace()
          event.preventDefault()
          return
        }
        if (/^[a-zA-Z]$/.test(event.key)) {
          this.handlers.placeMod(event.key.toLowerCase())
          event.preventDefault()
          return
        }
      }
      if (this.state.phase !== "round") return
      if (this.intro) {
        // Ordinarily any key at all deals the board, which is what an intro card
        // is for. While it is asking about the tutorial it is a question, and a
        // question must not be answered by a player leaning on the keyboard, so
        // only the two keys that mean "the default" take it. Anything aimed at a
        // focused control belongs to the control: without this, tabbing to Skip
        // and pressing Enter would be read here first and start the round with
        // the tips still on, which is the opposite of what was pressed.
        if (this.coachOffer) {
          if (event.target instanceof HTMLButtonElement) return
          if (event.key !== "Enter" && event.key !== " ") return
        }
        this.handlers.play()
        event.preventDefault()
        return
      }
      // Typing a letter is a statement that the round has the player's
      // attention, so the switch above stops holding it. Without this the two
      // rules meet in a trap: focus survives the press that set the board the
      // way you wanted, you type a word into the round the ordinary way, and
      // Enter presses the button still quietly holding focus instead of playing
      // the guess. Handing focus back to the body is also where the board wants
      // it: every key the game reads is bound to `window`.
      //
      // Spelled out as the two keys that edit a guess rather than as "anything
      // that got this far", because Tab gets this far too, and blurring on the
      // way past would drop the tab order back to the top of the document at the
      // exact moment the player was trying to walk it.
      //
      // A card tabbed to for its tip is the same statement read the other way.
      // The tip goes down with the focus that opened it, and it has to: a guess
      // typed a letter at a time never rebuilds the screen (see `patchDraft`),
      // so a panel left open over the board would stay there until Enter.
      const typing = event.key === "Backspace" || /^[a-zA-Z]$/.test(event.key)
      if (
        typing &&
        active instanceof HTMLElement &&
        (active.hasAttribute("data-focus") || active.hasAttribute("data-tip"))
      ) {
        active.blur()
      }
      // Through the handlers rather than straight to `dispatch`, so a typed
      // letter takes the same path whichever keyboard it came from.
      if (event.key === "Enter") void this.submit()
      else if (event.key === "Backspace") this.handlers.back()
      else if (/^[a-zA-Z]$/.test(event.key)) this.handlers.key(event.key.toLowerCase())
      else return
      event.preventDefault()
    })
  }
}

/**
 * Everything in a sheet a Tab can land on, in the order Tab would visit it.
 *
 * Disabled buttons are excluded because the browser skips them anyway, and a
 * trap that thinks a disabled button is the last stop would wrap early, and the
 * quit sheet and the shop both carry buttons that disable themselves.
 */
const focusableIn = (sheet: HTMLElement): HTMLElement[] => [
  ...sheet.querySelectorAll<HTMLElement>(
    'button:not([disabled]), a[href], input:not([disabled]), [tabindex]:not([tabindex="-1"])',
  ),
]

/**
 * Keep Tab inside the open sheet.
 *
 * Without this, tabbing off the last button walks into the screen *behind* the
 * backdrop, onto buttons that are visible, pressable, and were supposed to be
 * unreachable until the sheet was dealt with. The wrap is the whole of what
 * makes a sheet modal to a keyboard rather than only to a mouse.
 */
function trapTab(sheet: HTMLElement, event: KeyboardEvent): void {
  const focusable = focusableIn(sheet)
  const active = document.activeElement
  if (focusable.length === 0) {
    // Nothing to move to, so the only correct move is not to move.
    event.preventDefault()
    return
  }
  const first = focusable[0]
  const last = focusable[focusable.length - 1]
  // Forward off the end, backward off the front, or adrift outside the sheet
  // entirely. Backward from the sheet itself counts as off the front: it holds
  // focus on the way in, and it sits before everything it contains.
  const leaving = event.shiftKey
    ? active === first || active === sheet || !sheet.contains(active)
    : active === last || !sheet.contains(active)
  if (!leaving) return
  event.preventDefault()
  ;(event.shiftKey ? last : first)?.focus()
}

function clearSave(): void {
  try {
    localStorage.removeItem(SAVE_KEY)
    // The pair goes together. Left behind it would be a language belonging to no
    // run, and the next save writes over it before anything reads it, but a key
    // that outlives what it describes is a thing to explain later.
    localStorage.removeItem(RUN_LANG_KEY)
  } catch {
    // Nothing to do: the run is gone from memory either way.
  }
}

/**
 * Which list the saved run was dealt from, for the shell to fetch before there
 * is an app to ask.
 *
 * Null where there is no answer, which the caller reads as English: see
 * `RUN_LANG_KEY`. Deliberately not defaulted here, so that "no save" and "a save
 * from the build before this key" stay distinguishable at the one place that can
 * tell them apart.
 */
export function loadRunLang(): Lang | null {
  try {
    return readLang(localStorage.getItem(RUN_LANG_KEY))
  } catch {
    return null
  }
}

/**
 * How many of the per-letter marks are drawn, and the switch for it.
 *
 * A class on the root element rather than a flag threaded through the views,
 * for two reasons. The screen is rebuilt from scratch on every render, so
 * anything the views had to remember would have to be passed to all of them;
 * and what this hides is entirely presentational, since a pip is still a pip and
 * is just not being drawn, so the stylesheet is where the decision belongs. The
 * classes cover the keyboard, the board and any decoration added later, without
 * any of them being told the setting exists.
 */
function loadDecor(): Decor {
  try {
    const saved = localStorage.getItem(PLAIN_KEY)
    return saved === "minimal" || saved === "none" ? saved : "all"
  } catch {
    return "all"
  }
}

/**
 * Two independent classes rather than one plus a modifier, and neither implies
 * the other: `quiet` is the middle state's rules, `plain` is the bare board's,
 * and exactly one of them is on at a time.
 *
 * The nesting version, with `plain` always accompanied by `quiet` since the bare
 * board hides a superset, spreads one state across two selectors and leaves the
 * stylesheet depending on this function to keep setting both. Independent
 * classes cost a repeated `display: none` and make each block readable on its
 * own, which is what a rule you have to check against a screenshot needs to be.
 */
function setDecor(decor: Decor): void {
  const root = document.documentElement.classList
  root.toggle("quiet", decor === "minimal")
  root.toggle("plain", decor === "none")
  try {
    localStorage.setItem(PLAIN_KEY, decor)
  } catch {
    // The setting lasts the session instead of the install. Nothing else breaks.
  }
}

function seenCoach(): boolean {
  try {
    return localStorage.getItem(COACH_KEY) === "1"
  } catch {
    // A blocked store means the coaching runs again, which is the safe way to be
    // wrong about it: it costs a returning player five short cards on the first
    // round of a run and nothing after.
    return false
  }
}

function markCoachSeen(): void {
  try {
    localStorage.setItem(COACH_KEY, "1")
  } catch {
    // The coaching lasts the session instead of the install.
  }
}

export function loadSave(): RunState | null {
  try {
    const raw = localStorage.getItem(SAVE_KEY)
    if (!raw) return null
    const parsed: unknown = JSON.parse(raw)
    // Just enough of a shape check to survive a save from an older build.
    if (typeof parsed !== "object" || parsed === null) return null
    const state = parsed as RunState
    return typeof state.seed === "number" && typeof state.phase === "string" && state.round
      ? state
      : null
  } catch {
    return null
  }
}

const rootSeed = (): number => Math.floor(Math.random() * 2 ** 31)
