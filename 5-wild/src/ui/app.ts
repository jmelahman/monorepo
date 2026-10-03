import type { Action, GameEvent, Refusal, RunState, WordSource } from "../engine"
import { MODIFIER_BY_ID, reduce, startRun } from "../engine"
import { admin, loadCheckpoint, stashCheckpoint } from "./admin"
import { withAmounts } from "./amounts"
import type { Cue } from "./audio"
import { audioContext, Sound } from "./audio"
import type { CoachStep } from "./coach"
import { coachAsks, coachSpent, coachStep } from "./coach"
import { clear } from "./dom"
import { setMood } from "./fx/background"
import { ms, replay, setMotionSpeed } from "./fx/motion"
import * as board from "./fx/scenes/board"
import { playEvents } from "./fx/scenes/events"
import { playScoring } from "./fx/scenes/scoring"
import { arrived, leaving } from "./fx/scenes/transitions"
import { bindShake } from "./fx/shake"
import { begin } from "./fx/timeline"
import type { Lang } from "./lang"
import {
  consumableNote,
  loadLang,
  modPlaced,
  NEXT_LANG,
  readLang,
  refusalText,
  setLang,
  ui,
} from "./lang"
import { chosenAscension, Profile, unlocked } from "./meta"
import { Music } from "./music"
import { seal, unseal } from "./seal"
import type { SeededOffer } from "./seed"
import { parseSeed, readPastedLink, seedCode, seedLink } from "./seed"
import { currentSkin, NEXT_SKIN } from "./skin"
import { loadSpeed, NEXT_SPEED, setSpeed } from "./speed"
import { isTable, watchTable } from "./table"
import type { Consent, RunEnd, RunLog, Step } from "./telemetry"
import {
  beginLog,
  file,
  flush,
  loadConsent,
  loadLog,
  payload,
  saveLog,
  setConsent,
  enabled as sharingEnabled,
  stepFor,
} from "./telemetry"
import { setSkin } from "./theme"
import type { Chrome, Decor, Handlers, SeedSheet, SoundLevel } from "./views"
import {
  aboutView,
  ascendView,
  codexView,
  creditsView,
  displacedAt,
  endView,
  fillCategory,
  fillCoach,
  fillReadout,
  helpView,
  introView,
  loadingView,
  menuView,
  NEXT_DECOR,
  NEXT_SOUND,
  packView,
  placeView,
  quitView,
  rewardView,
  roundView,
  seedView,
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

/**
 * What a shop or reward action sounds like, read off what it did rather than
 * what was asked, since the same tap can open a pack or buy a relic. First
 * match wins, most specific first: buying a modifier is a purchase *and* opens
 * the placing sheet, and the stamp that follows is the sound that matters, so
 * the purchase is the fallback, not the headline.
 *
 * Typing and backspace are not here: they sound on the tap, before the
 * dispatch, and a refusal never reaches this far.
 */
function actionCue(action: Action, events: readonly GameEvent[]): Cue | null {
  for (const event of events) {
    if (event.type === "mod_placed") return { name: "place" }
    if (event.type === "consumable") return { name: "consume" }
    if (event.type === "pack_opened") return { name: "pack" }
    if (event.type === "pack_picked" && event.taken) return { name: "pick" }
    // Collecting a round pays its reward on the way in, so this has to beat
    // the coin below: the shelf being dealt is what the player is looking at.
    if (event.type === "shop_entered") return { name: "reroll", shop: true }
  }
  // Checked on the action, because a reroll the tray paid for moves no gold
  // and so leaves no event to find.
  if (action.type === "reroll") return { name: "reroll" }
  // A drop moves nothing and so says nothing in events; backspace's sound,
  // since taking a thing away is what both of them are.
  if (action.type === "drop_consumable") return { name: "back" }
  const gold = events.find((event) => event.type === "gold")
  if (!gold) return null
  if (gold.reason === "purchase") return { name: "buy" }
  if (gold.reason === "sold") return { name: "sell" }
  return { name: "coin" }
}

/** Which screen this is, ignoring whatever is currently happening to it. */
const screenKind = (node: Element): string =>
  [...node.classList].filter((name) => !TRANSIENT_SCREEN.includes(name)).join(" ")

export class App {
  private state: RunState
  /** True while a scoring animation owns the screen; input is ignored. */
  private busy = false
  /** True while the round's intro card is up, before the board is dealt. */
  private intro = false
  /** True when there is no run to return to and the front door is showing. */
  private atTitle: boolean
  /** The modal on top of everything, if any. */
  private overlay:
    | "help"
    | "codex"
    | "shapes"
    | "stats"
    | "menu"
    | "quit"
    | "ascend"
    | "about"
    | "credits"
    | "seed"
    | null = null
  /**
   * Which sheet the last render put on screen, so this one can tell a sheet
   * arriving from the same sheet being rebuilt underneath the player's thumb.
   * Not `overlay` itself: that field has already been changed by the time a
   * render reads it, and two of the sheets are not named by it at all.
   */
  private sheetShown: string | null = null
  /**
   * Which named sheet the last render announced, for the whoosh. Only the
   * named ones: the pack and the placing sheet are opened by a purchase that
   * has already made its own noise, and a whoosh under it would be a second
   * sound for one tap.
   */
  private sheetHeard: typeof this.overlay = null
  /** Which rung the open lock is offering. Only meaningful while `overlay` is "ascend". */
  private ascendTo = 0
  /**
   * What is typed in the seed field, held here because the field is rebuilt
   * with every render and would otherwise forget it. Not saved: a half-typed
   * code is a gesture, like `arming`.
   */
  private seedDraft = ""
  /**
   * The run a `?seed=` link asked for, which carries the ascension and word
   * list the code does not. It applies while the field still names its seed;
   * a code typed over it is the player's own, and is dealt on the player's own
   * terms: the dial's level and the interface's language, as Play would be.
   */
  private seedOffer: SeededOffer | null = null
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
   * animation rather than a reading time goes through `ms` in `./fx/motion`,
   * which `setMotionSpeed` keeps in step with this; see `./speed`.
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
  /**
   * The replay of the run in hand, kept whatever the player has said about
   * sharing: it costs a few KB, and a switch turned on mid-run can then offer
   * the whole run rather than its back half. Null for a run with no beginning
   * on record, which is a save from before this existed or the scaffolding run
   * behind the title screen; neither is ever sent. See `./telemetry`.
   */
  private log: RunLog | null
  private consent: Consent = loadConsent()
  /**
   * The thanks for a yes, until the player moves on from the screen it was said
   * on. See `Chrome.thanked`; cleared by anything that opens, closes or leaves.
   */
  private thanked: "fresh" | "shown" | null = null
  /**
   * True when the run on screen is someone else's: a model on the benchmark
   * host, or an episode it recorded. See `watch` and `src/ui/spectate.ts`.
   *
   * The one promise it makes is that a spectator writes nothing. The player's
   * run, record, replay and coaching sit in this same origin's storage, and a
   * tab left watching a model for an afternoon must not come back as the
   * player's save, with the model's stage on their record. So the four methods
   * every write goes through (`save`, `tally`, `logStep`, `finish`) each refuse
   * at the top, rather than every caller remembering to ask.
   */
  private watching = false
  /** Set only while the feed itself is moving the run; the player's input is not. */
  private feeding = false

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
    this.log = loadLog(saved)
    // Whatever an earlier session finished without a connection.
    void flush()
    // The class and the property are on the document rather than in the render,
    // so they have to be put back on the way in, since the stylesheet is the
    // only thing that remembers.
    setDecor(this.decor)
    setSpeed(this.speed)
    setMotionSpeed(this.speed)
    bindShake(this.root)
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
    this.bindMouse()
    this.bindLayout()
  }

  /**
   * Rebuild the screen when the window crosses into or out of the table.
   *
   * `.table` is a class on the root and was once the whole of the difference, so
   * a resize needed nothing but the stylesheet. It is not the whole of it now:
   * the views ask `isTable()` at render, and the rail builds different DOM from
   * the phone's header (the round card wrapping the boss's rule, the decor
   * switch in the rail's foot rather than in the hand's row, the foot last in
   * the header so Tab reaches the shape before the ☰). A window dragged across
   * the line kept the old layout's nodes under the new layout's rules, and the ☰
   * and the switch landed wherever the other layout's CSS put them. Styling
   * both arrangements onto one DOM was the alternative, and it cannot be done
   * whole: the switch lives in two different parents and Tab order is the
   * DOM's, which no media query reaches. So the screen is rebuilt, which every
   * dispatch already does.
   *
   * Not while a guess is scoring: the animation holds nodes from the render it
   * started on, and it ends in a render of its own, which will be the new
   * layout's. `watchTable` lands the class before calling back, so the views
   * read the new answer.
   */
  private bindLayout(): void {
    watchTable(() => {
      if (!this.busy) this.render()
    })
  }

  /**
   * Tell the stylesheet whether the pointer in use is a mouse, so hover states
   * can be drawn for it and for nothing else.
   *
   * A class on the root rather than `@media (hover: hover)`, for the reason
   * `bindTips` gives at length: the APK's WebView answers that query `true` on a
   * phone, and a `:hover` gated on it is one the tapped element keeps after the
   * finger lifts. Here that is worse than usual, since the screen is rebuilt
   * under a still finger and the stuck state lands on whatever button is drawn
   * where the last one was. The event says what actually happened, so the class
   * follows the last pointer used: a hybrid that puts down the trackpad for the
   * screen loses its hovers on the first touch and gets them back on the next
   * move of the mouse. A pen is not a mouse here, as in `bindTips`.
   *
   * `pointerover` as well as `pointerdown` because a mouse announces itself by
   * moving long before it clicks, and a touch fires `pointerover` as the finger
   * lands, ahead of the `:hover` it would otherwise pick up. On the document,
   * not the root, so a pointer that arrives over the margins counts too.
   */
  private bindMouse(): void {
    const note = (event: PointerEvent) =>
      document.documentElement.classList.toggle("mouse", event.pointerType === "mouse")
    document.addEventListener("pointerover", note, { capture: true, passive: true })
    document.addEventListener("pointerdown", note, { capture: true, passive: true })
  }

  /**
   * Audio may not start before the player has touched something, so the music
   * is tried at boot and the first gesture of the session, whatever it was,
   * wakes whatever the browser refused. The key that reloaded the page is not
   * one: it landed on the page before this one. It also stops on the way out:
   * a phone that locks with the tab alive would otherwise keep an oscillator
   * running against the battery all night.
   */
  private bindAudioWake(): void {
    const wake = () => {
      void audioContext()?.resume()
      this.music.enable()
    }
    document.addEventListener("pointerdown", wake, { once: true })
    document.addEventListener("keydown", wake, { once: true })
    document.addEventListener("visibilitychange", () => {
      if (document.hidden) this.music.suspend()
      else this.music.resume()
    })
    this.music.autostart()
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

  /* ------------------------------------------------------------ spectating */

  /**
   * Hand the run to a feed. Called once, instead of `start`, by the spectator.
   *
   * What still answers the player is everything that changes how the game
   * looks and sounds (the pause sheet, the dials, the rules), because those are
   * settings and the player owns them. What does not is everything that would
   * move the run or start another: those handlers are swapped for nothing here,
   * in one list, and `dispatch` and `submit` refuse anything the feed did not
   * send, which covers the keyboard, physical and drawn, without either of them
   * having to know.
   *
   * The coaching is switched off rather than guarded: it would narrate a
   * model's first round to a spectator as though they were playing it, and the
   * flag it clears lives in this object, so turning it off writes nothing.
   */
  watch(): void {
    this.watching = true
    this.coachOwed = false
    this.log = null
    this.atTitle = false
    this.loading = true
    const idle = () => {}
    for (const name of [
      "play",
      "newRun",
      "startTutorial",
      "skipCoach",
      "quit",
      "askQuit",
      "ascend",
      "askAscend",
      "setAscension",
      "placeMod",
      "cancelPlace",
      "openSeed",
      "playSeed",
      "copySeed",
    ] as const) {
      this.handlers[name] = idle
    }
    this.render()
  }

  /**
   * Deal the run the host dealt, from the same seed. The words come first when
   * the host is playing in another language than this tab opened in, through
   * the same shell handle a language change uses.
   */
  async watchRun(seed: number, ascension: number, lang: Lang): Promise<void> {
    this.loading = true
    this.render()
    if (lang !== this.wordsLang) {
      this.words = await this.lists.load(lang)
      this.wordsLang = lang
    }
    this.state = startRun(seed, this.words, ascension).state
    this.loading = false
    this.overlay = null
    this.arming = null
    this.intro = true
    this.sound.cue({ name: "intro", boss: false })
    this.render()
  }

  /**
   * Play one of the host's moves, at the pace a person would watch it, and say
   * whether this tab's engine took it. A refusal means the tab and the host are
   * not running the same game (another content version, another list), and
   * the spectator stops there rather than drawing a run that is not the one
   * being played.
   *
   * Through the same two doors the player's input uses, so every animation, cue
   * and toast is the one a player would have seen: letters one at a time
   * through `dispatch`, so each lands on the board, and the guess through
   * `submit`, which is the scoring scene. Accepted is read off the state
   * changing, since both doors swallow a refusal into a toast and return
   * nothing.
   */
  async watchStep(step: Step): Promise<boolean> {
    const wait = (authored: number) => new Promise((done) => setTimeout(done, ms(authored)))
    // The intro card is the player's to dismiss, and here there is no player:
    // long enough to read the round's name and target, then the board.
    if (this.intro) {
      await wait(1400)
      this.intro = false
      this.render()
      await wait(400)
    }
    while (this.busy) await wait(100)
    this.feeding = true
    try {
      if (step.type === "guess") {
        for (const letter of step.word) {
          const before = this.state
          this.sound.cue({ name: "key" })
          this.dispatch({ type: "type_letter", letter }, "arriving")
          if (this.state === before) return false
          await wait(140)
        }
        await wait(250)
        const before = this.state
        await this.submit()
        if (this.state === before) return false
        // The screen the guess led to, whatever it is, before the next move
        // takes it away: a reward card flashed for one frame is a reward
        // nobody watching saw.
        await wait(this.state.phase === "round" ? 500 : 1600)
        return true
      }
      const before = this.state
      this.dispatch(step)
      if (this.state === before) return false
      await wait(900)
      return true
    } finally {
      this.feeding = false
    }
  }

  /* ------------------------------------------------------------- dispatch */

  private dispatch(action: Action, typing?: "arriving" | "leaving"): void {
    if (this.busy) return
    if (this.watching && !this.feeding) return
    const wasPhase = this.state.phase
    const before = this.state
    const { state, events } = reduce(this.state, action, this.words)

    const refusal = events.find((event) => event.type === "rejected")
    if (refusal) {
      this.refuse(refusal.refusal)
      return
    }

    this.state = state
    this.logStep(action, before)
    // Nothing is armed once there is nothing in hand. The commit path clears it
    // on its way through, so what this catches is the run ending underneath an
    // armed key, quitting from the menu over the top of the picker, which
    // would otherwise leave the next run's picker with one letter already half
    // pressed, and that letter would place on a single tap.
    if (!this.state.placing) this.arming = null
    this.tally(before)
    // Every arrival at a round from elsewhere gets the intro card, which is the
    // only thing that makes the shop and the board feel like separate places.
    if (this.state.phase === "round" && wasPhase !== "round") {
      this.intro = true
      this.sound.cue({ name: "intro", boss: Boolean(this.state.round.bossId) })
      if (!this.watching) stashCheckpoint(this.state, this.wordsLang)
    }
    this.save()

    // The win is recorded where it is offered rather than where the run ends,
    // because those are no longer the same moment: a run can take the win and
    // then go looking for stage 20. The engine fires this once per run.
    //
    // The level comes off the run rather than off the record, so a win is banked
    // at the difficulty it was actually played at whatever has been chosen since.
    //
    // A seed from a link can be played at any level, and the ladder is climbed a
    // rung at a time, so a seeded win above the dial's reach is still a win on
    // the record but raises no rung: one friend's level-20 link would otherwise
    // unlock the whole ladder for someone who never played past the first.
    // At or below the reach it climbs as any win does, since the player could
    // have picked that level for a run of their own.
    if (!this.watching && events.some((event) => event.type === "run_won")) {
      const level = this.state.ascension ?? 0
      this.profile.won(level, !this.log?.seeded || level <= unlocked(this.profile.stats))
    }

    const paid = events.some((event) => event.type === "gold")
    const heard = actionCue(action, events)
    if (heard) this.sound.cue(heard)

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
    // The table's half of the same thought: what the action did, played over
    // the screen that shows it. Returns at once on a phone.
    const screen = this.root.firstElementChild
    if (screen instanceof HTMLElement) {
      playEvents(action, events, { root: this.root, screen, state: this.state, sound: this.sound })
    }
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
      if (lands && tile instanceof HTMLElement) board.typed(tile)
    }
    if (letter === "leaving" && row instanceof HTMLElement) board.erased(row)
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
    if (this.watching && !this.feeding) return
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
    this.logStep({ type: "submit" }, before)
    this.save()
    // Filed the moment it is decided rather than when the end screen is left,
    // because a player who closes the app on the loss never leaves it.
    if (this.state.phase === "game_over") this.finish("lost")

    this.busy = true
    begin()
    this.render("round")
    const screen = this.root.firstElementChild
    // The scene is decoration over a state that is already decided and saved,
    // so a throw inside it must cost the show and never the input: without the
    // `finally` a scene that tripped on a DOM shape it did not expect left
    // `busy` up, and every tap and key after it was ignored until a reload.
    try {
      if (screen instanceof HTMLElement) {
        await playScoring(events, { root: this.root, screen, state: this.state, sound: this.sound })
      }
    } catch (error) {
      console.error(error)
    } finally {
      this.busy = false
    }
    this.render()
    // The screen the guess led to (the reward, the end card, or the same board)
    // gets the same after-the-render playback as any dispatched action, which
    // is where the table's round-won and game-over moments live. The scoring
    // events come along with the rest; the scenes pick out what is theirs.
    const after = this.root.firstElementChild
    if (after instanceof HTMLElement) {
      playEvents({ type: "submit" }, events, {
        root: this.root,
        screen: after,
        state: this.state,
        sound: this.sound,
      })
    }

    if (this.state.phase === "game_over") this.sound.cue({ name: "lose" })
    else if (this.state.phase === "reward" || this.state.phase === "victory") {
      this.sound.cue({ name: "win", run: this.state.phase === "victory" })
    }
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
    this.sound.cue({ name: "reject" })
    if (this.state.phase !== "round") return
    const row = this.root.querySelector(`.row[data-row="${this.state.round.guesses.length}"]`)
    // The row sways and that is all: the red frame and the sparks the table
    // threw off it went when the owner asked for a calmer refusal.
    replay(row, "rejected", 420)
  }

  private bump(selector: string): void {
    replay(this.root.querySelector(selector), "bumped", 320)
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

    // Nodes rather than `textContent`, so a tip marks its mult amounts the way
    // the card it came off does. See `withAmounts`.
    tip.replaceChildren(...withAmounts(host.dataset.tip ?? ""))
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
    // Toward the middle of the screen, and the other way only when that side
    // has no room. It was below by preference, from when the round's relic
    // tray sat under the HUD, and the flip was for the shop's tray at the foot.
    // The round's tray is at the foot now too, over the keys, and a panel
    // opening below it would land on the keyboard, under the thumb holding the
    // card; the header's own tips still open downward, over the board.
    const lower = card.top + card.height / 2 > window.innerHeight / 2
    const below = lower
      ? card.top - gap - box.height < edge
      : card.bottom + gap + box.height + edge <= window.innerHeight
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
    const host = this.root.querySelector<HTMLElement>(".toast") ?? this.toastHost()
    host.textContent = message
    // On a phone's round, hung immediately under the board rather than lifted
    // off the keyboard by `--toast-lift`. The board sits flush to the header
    // with the slack falling below it (see `.grid` in `board.css`), so the
    // sentence lands next to the row it is refusing, where the eye already is,
    // instead of over the hand and the relics. Measured here rather than
    // written in the stylesheet because the toast is `fixed` and a sibling of
    // the board, so nothing in CSS knows where the board ends. Set on every
    // call, since the node is the render's and the board's foot moves with the
    // chrome above it. The table keeps its lift, as does every screen with no
    // board.
    const board = isTable() ? null : this.root.querySelector(".round-screen .grid")
    if (board) {
      host.style.top = `${Math.round(board.getBoundingClientRect().bottom + 12)}px`
      host.style.bottom = "auto"
    }
    host.classList.add("show")
    clearTimeout(this.toastTimer)
    this.toastTimer = setTimeout(() => host.classList.remove("show"), TOAST)
  }

  /**
   * A host for a screen that did not draw one. The round and the shop draw
   * their own, where their stylesheets can lift it clear of the keys, and every
   * other screen gets this one at the foot of the window. It used to be that
   * those screens had nowhere to say anything, and nothing did, until the seed
   * sheet: it opens over any screen a link lands on, and its refusal and the
   * copy button's "Link copied" are the only signs either tap did anything. The
   * next render clears it with everything else.
   */
  private toastHost(): HTMLElement {
    const host = document.createElement("div")
    host.className = "toast"
    this.root.append(host)
    // Laid out once at `opacity: 0`, or the class added next lands in the same
    // frame and the fade has nothing to fade from.
    void host.offsetWidth
    return host
  }

  /* --------------------------------------------------------------- render */

  private readonly handlers: Handlers = {
    key: (letter) => {
      this.sound.cue({ name: "key" })
      this.dispatch({ type: "type_letter", letter }, "arriving")
    },
    enter: () => void this.submit(),
    back: () => {
      this.sound.cue({ name: "back" })
      this.dispatch({ type: "backspace" }, "leaving")
    },
    useConsumable: (index) => this.dispatch({ type: "use_consumable", index }),
    collect: () => this.dispatch({ type: "collect" }),
    buy: (index) => this.dispatch({ type: "buy", index }),
    sell: (index) => this.dispatch({ type: "sell_relic", index }),
    drop: (index) => this.dispatch({ type: "drop_consumable", index }),
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
      this.sound.cue({ name: "key" })
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
    cycleSound: () => {
      // Both switches are written every step, not only the one that moved: each
      // level is a pair, and two of the steps move both halves at once, off back
      // up to music and music-only down to off.
      const next = NEXT_SOUND[this.soundLevel]
      this.sound.setMuted(next === "off")
      this.music.setOff(next !== "music")
      this.render()
    },
    // The effects switch used to carry the music with it when it silenced the
    // game. It no longer does: the pause sheet has a switch for each, and one
    // that reached into the other made "music, no effects" the one pair no tap
    // could set. Quiet in one tap is the title speaker's job now.
    toggleEffects: () => {
      this.sound.setMuted(!this.sound.isMuted)
      this.render()
    },
    toggleMusic: () => {
      this.music.setOff(!this.music.isOff)
      this.render()
    },
    nextTrack: () => {
      this.music.nextTrack()
      this.render()
    },
    // One handler for the switch on the about and pause sheets, since they are
    // the same answer given in two places.
    setSharing: (on) => {
      setConsent(on ? "on" : "off")
      this.thanked = on ? "fresh" : null
      this.consent = loadConsent() ?? (on ? "on" : "off")
      void flush()
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
      setMotionSpeed(this.speed)
      this.render()
    },
    // From the class on the root rather than a field: the shell applied the
    // look there before the first paint, and it is the one place the stylesheet
    // reads it from, so it cannot disagree with what is on screen. That is also
    // what makes the first tap right before there is a pick: the look the player
    // sees, whether they chose it or the window did, is the one this leaves.
    cycleSkin: () => {
      setSkin(NEXT_SKIN[currentSkin()])
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
      this.thanked = null
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
    // Straight to the board: the intro card's only question is the one this
    // button has just answered, and asking it again a screen later would be a
    // confirmation dialog for a tap that was not a mistake.
    startTutorial: () => {
      void this.withWords(() => this.startFresh(false))
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
    openAbout: () => {
      this.thanked = null
      this.overlay = "about"
      this.render()
    },
    openCredits: () => {
      this.overlay = "credits"
      this.render()
    },
    closeOverlay: () => {
      this.thanked = null
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
    openSeed: () => {
      this.overlay = "seed"
      this.render()
    },
    // Stored without a render, which would rebuild the field under the caret on
    // every keystroke. A pasted link is the exception: it arrives in one event,
    // and it changes the terms the sheet states, so it becomes the offer a link
    // opened in the address bar would have made, and is drawn as its code.
    editSeed: (text) => {
      const pasted = readPastedLink(text)
      if (!pasted) {
        this.seedDraft = text
        return
      }
      this.seedOffer = pasted
      this.seedDraft = seedCode(pasted.seed)
      this.render()
    },
    playSeed: () => {
      const seed = parseSeed(this.seedDraft)
      if (seed === null) {
        this.toast(ui().seed.badCode)
        return
      }
      const { ascension, words } = this.seedTerms(seed)
      void this.startSeeded(seed, ascension, words)
    },
    // The link rather than the code, for the reason `seedLine` gives. The toast
    // is the only sign the tap did anything, since a clipboard has no face; a
    // refused write (no permission, an old WebView) puts the code up instead,
    // which is at least a thing that can be read off and typed.
    copySeed: () => {
      const link = seedLink(this.state.seed, this.state.ascension ?? 0, this.wordsLang)
      const code = ui().seed.line(seedCode(this.state.seed))
      const write = navigator.clipboard?.writeText(link)
      if (!write) {
        this.toast(code)
        return
      }
      write.then(
        () => this.toast(ui().seed.copied),
        () => this.toast(code),
      )
    },
    ascend: () => {
      this.profile.chose(this.ascendTo)
      this.overlay = null
      this.render()
    },
    quit: () => {
      // Quitting a won run from the victory screen is how a win is banked, so
      // `quit` here covers both "gave up" and "took the win and left"; `won` on
      // the payload is what tells them apart.
      this.finish("quit")
      this.thanked = null
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

  /**
   * Read off the two stored switches rather than kept as a third, so there is
   * nothing to fall out of step with them.
   */
  private get soundLevel(): SoundLevel {
    if (this.sound.isMuted) return this.music.isOff ? "off" : "musicOnly"
    return this.music.isOff ? "sound" : "music"
  }

  private get chrome(): Chrome {
    return {
      seed: this.atTitle || this.watching ? null : seedCode(this.state.seed),
      sound: this.soundLevel,
      effectsOff: this.sound.isMuted,
      musicOff: this.music.isOff,
      track: this.music.title,
      decor: this.decor,
      speed: this.speed,
      skin: currentSkin(),
      lang: this.lang,
      wordsDeferred: this.wordsDeferred,
      coach: this.coach,
      coachOffer: this.coachOffer,
      sharing: !sharingEnabled() ? null : (this.consent ?? "off"),
      thanked: this.thanked,
    }
  }

  private get seedSheet(): SeedSheet {
    const { ascension, words } = this.seedTerms(parseSeed(this.seedDraft))
    return {
      draft: this.seedDraft,
      ascension,
      words: words === this.lang ? null : words,
      // A lost run is still on screen but already over, so a link opened on its
      // end screen replaces nothing. A won one is not: endless is still on offer.
      inRun: !this.atTitle && this.state.phase !== "game_over",
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

  /**
   * Open the seed sheet on a link's run, over whatever the launch would have
   * shown. Called by the shell once, before `start`; the sheet is an offer and
   * not a deal, because a link opened by a player mid-run must not cost them the
   * run before they have read what it is.
   */
  offerSeed(offer: SeededOffer | string): void {
    // A string is a link whose code did not read, offered as typed so Play can
    // say what is wrong with it; its terms are the player's own until it does.
    if (typeof offer === "string") this.seedDraft = offer
    else {
      this.seedOffer = offer
      this.seedDraft = seedCode(offer.seed)
    }
    this.overlay = "seed"
  }

  /**
   * The level and the list a seed will be dealt from: the link's, while the
   * field still names the link's seed, and otherwise the player's own, the
   * dial and the interface language, which is exactly what Play would use.
   */
  private seedTerms(seed: number | null): { ascension: number; words: Lang } {
    const offer = this.seedOffer
    if (offer && seed === offer.seed) return { ascension: offer.ascension, words: offer.words }
    return { ascension: chosenAscension(this.profile.stats), words: this.lang }
  }

  /**
   * Deal a chosen seed from a chosen list.
   *
   * Not through `withWords`, although Play is, because the two disagree about
   * a failed fetch. Play shrugs one off and deals from the list in hand, which is
   * right for a random run and wrong for this one: another list is another run
   * under the same code, the "same seed, different word" the link exists to
   * prevent. So a failure here deals nothing, and hands back the sheet exactly as
   * it was, code and link terms and all, since the player's next move is to try
   * again and a sheet emptied by the attempt would have lost the link's level and
   * list for good. A retype cannot bring those back.
   *
   * The sheet comes down while the list loads, because it would otherwise sit
   * over the loading card with a live Play button, and a second tap would start a
   * second deal racing the first.
   *
   * Any list but the interface's becomes the run's, the way the spectator's does
   * in `watchRun`; `deferWords` then starts the interface's own on its way for
   * the run after, so the pause sheet's note about words changing is true.
   */
  private async startSeeded(seed: number, ascension: number, words: Lang): Promise<void> {
    if (words !== this.wordsLang) {
      this.overlay = null
      this.loading = true
      this.render()
      try {
        // The interface's list may already be on its way; asking again would
        // fetch it twice.
        this.words = await (words === this.lang && this.incoming
          ? this.incoming
          : this.lists.load(words))
        this.wordsLang = words
      } catch (error) {
        this.loading = false
        this.overlay = "seed"
        // A rejected `incoming` would be handed back to every later attempt.
        this.deferWords()
        this.render()
        this.toast(ui().error.words(String(error)))
        return
      }
      this.loading = false
      this.deferWords()
    }
    this.seedOffer = null
    this.seedDraft = ""
    this.startFresh(true, { seed, ascension })
  }

  /**
   * The body of `newRun`, once there is a word list to start one from.
   * `intro` false skips the round's intro card; see `startTutorial`.
   */
  private startFresh(intro = true, seeded?: { seed: number; ascension: number }): void {
    // A run replaced without ending is still a run. This is also where a run
    // lands whose app was killed mid-round and never reopened to it.
    this.finish("abandoned")
    this.thanked = null
    this.state = seeded
      ? startRun(seeded.seed, this.words, seeded.ascension).state
      : startRun(rootSeed(), this.words, chosenAscension(this.profile.stats)).state
    this.atTitle = false
    this.overlay = null
    this.intro = intro
    if (intro) this.sound.cue({ name: "intro", boss: false })
    // Counted here rather than in the constructor: the class always holds a
    // run so that nothing downstream has to handle a null one, and most of
    // those are scaffolding the player never sees.
    this.profile.started()
    this.log = beginLog(this.state, this.wordsLang, this.profile.stats.runs, seeded !== undefined)
    stashCheckpoint(this.state, this.wordsLang)
    // Persisted before the first keypress: a fresh run is already a run, and
    // closing the app on the intro card should not silently reroll the word.
    this.save()
    this.render()
  }

  /**
   * Put the run back to the start of its latest round. Admin only; see `./admin`.
   *
   * The log goes with it: a replay whose steps run past the rewind replays into
   * a different run, and one quietly rewound would be a balance figure nobody
   * played. The record is left alone, so a retried round is tallied twice,
   * which a tester can live with and a player never sees.
   */
  private retryRound(): void {
    if (this.watching || this.busy) return
    const checkpoint = loadCheckpoint(this.wordsLang)
    if (!checkpoint) return
    this.log = null
    this.thanked = null
    this.state = checkpoint
    this.atTitle = false
    this.overlay = null
    this.arming = null
    this.intro = true
    this.save()
    this.render()
  }

  /**
   * Whether the rules sheet leads with the tutorial.
   *
   * Only at the title, which is only ever shown with no run saved, so starting
   * one from here costs nothing. Anywhere else a run is open, and the tutorial
   * is either already running on it (the first round, before the third guess,
   * is the only place a run can be while it is still owed) or spent for good.
   */
  private get tutorialOffer(): boolean {
    return this.coachOwed && this.atTitle
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
    const anchor = this.root.querySelector(step.anchor)
    anchor?.classList.add("coached")
    const card = this.root.querySelector<HTMLElement>(".coach")
    if (anchor && card) aimCoach(card, anchor)
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
              ? shopView(this.state, this.handlers, this.coach)
              : phase === "game_over" || phase === "victory"
                ? endView(this.state, this.handlers, this.chrome.seed)
                : roundView(this.state, this.handlers, this.chrome)

    // Overlays sit beside the screen rather than replacing it, so the board is
    // still visible behind the sheet and the player keeps their bearings.
    //
    // A menu the player asked for outranks the open pack, so they can still quit
    // or read the rules mid-decision; the pack is waiting underneath when they
    // close it, because the engine will not let the shop move on until it is.
    const sheet =
      this.overlay === "help"
        ? helpView(this.handlers, this.tutorialOffer)
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
                : this.overlay === "about"
                  ? aboutView(this.handlers, this.chrome)
                  : this.overlay === "credits"
                    ? creditsView(this.handlers)
                    : this.overlay === "quit"
                      ? quitView(this.state, this.handlers)
                      : this.overlay === "ascend"
                        ? ascendView(this.ascendTo, this.handlers)
                        : this.overlay === "seed"
                          ? seedView(this.seedSheet, this.handlers)
                          : // Both are held decisions the engine will not let the shop move
                            // past, and the two cannot be open at once, since buying is refused
                            // while either is. Order is arbitrary; only exclusivity matters.
                            (placeView(this.state, this.handlers, this.arming) ??
                            packView(this.state, this.handlers))

    // Which sheet that is, since the answer decides whether it may announce
    // itself below. `overlay` names nine of them. The two the fallback builds
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
    if (this.overlay !== this.sheetHeard) {
      this.sound.cue({ name: "sheet", open: this.overlay !== null })
      this.sheetHeard = this.overlay
    }

    // Measured before the old screen goes, since after it there is nothing
    // left to measure. Free on a phone: the table's hooks return at once.
    const live = this.root.firstElementChild
    const was = leaving(this.root, live ? screenKind(live) : null)
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
    arrived(this.root, screenKind(view), was)
    // The background's mood, from the same facts the view was chosen from. The
    // boss reddens the table from its intro card on, and the reward screen after
    // it is back to the round's own colour.
    setMood(
      this.atTitle || this.loading
        ? "round"
        : phase === "shop"
          ? "shop"
          : phase === "victory" || phase === "game_over"
            ? phase
            : phase === "round" && this.state.round.bossId
              ? "boss"
              : "round",
      sheet !== null,
    )
    this.holdFocus(keeping)
    this.lightCoach()
    // The pop was this render's; any after it keeps the line and not the pop.
    if (this.thanked === "fresh") this.thanked = "shown"
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
    // both numbers are divided by the same setting; at ×10 the margin is two
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
   * Its body rather than the sheet, where it has one, which is the box that
   * scrolls. The browser scrolls the nearest scroll box at or *above* focus, and
   * the body is below the sheet, so on the sheet the arrows and the page keys
   * had nothing to move and the rules could be read one screen deep. The body
   * still starts above every button in it, and a reader loses the title, which
   * is the cost.
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
   *
   * Except over a name the sheet itself is wearing. The pause sheet's settings
   * are rebuilt by their own taps, and sending focus back to the sheet's top
   * after each one made a keyboard player tab down through the list again to
   * press the speed dial a second time. Looked up inside the sheet only, so a
   * name on the board behind it can never pull focus out from under the sheet.
   */
  private holdFocus(keeping?: string): void {
    const sheet = this.root.querySelector<HTMLElement>(".sheet")
    const named = (scope: ParentNode) =>
      keeping ? scope.querySelector<HTMLElement>(`[data-focus="${keeping}"]`) : null
    if (!sheet) {
      named(this.root)?.focus()
      return
    }
    if (!sheet.contains(document.activeElement)) {
      const body = sheet.querySelector<HTMLElement>(".sheet-body")
      ;(named(sheet) ?? body ?? sheet).focus()
    }
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
    if (this.watching) return
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

  /**
   * Both advance points call this, as they call `tally`, and for the same
   * reason: a run that moves without coming through here is a replay with a
   * hole in it, which replays into a different run.
   */
  private logStep(action: Action, before: RunState): void {
    if (this.watching) return
    const step = stepFor(action, before)
    if (step && this.log) this.log.steps.push(step)
  }

  /**
   * Close the log and hand it to the consent to deal with. Clearing it is what
   * makes this safe to call from every exit: the loss files the run, and the
   * new run that follows the loss finds nothing left to call abandoned.
   */
  private finish(end: RunEnd): void {
    if (this.watching || !this.log) return
    file(payload(this.log, this.state, end), this.consent)
    this.log = null
    saveLog(null)
    void flush()
  }

  private save(): void {
    if (this.watching) return
    try {
      localStorage.setItem(SAVE_KEY, seal(JSON.stringify(this.state)))
      // Beside the run and written in the same breath, because the pair is only
      // meaningful together: a save whose language is a guess is a save that can
      // be resumed against the wrong dictionary, where every legal word is
      // refused and nothing on screen says why.
      localStorage.setItem(RUN_LANG_KEY, this.wordsLang)
    } catch {
      // A full or disabled store costs the player their resume, not their run.
    }
    // The third of the set, and written here for the same reason the language
    // is: a save resumed without its log is a run that can never be sent.
    saveLog(this.log)
    // Every moment the run is worth persisting is a moment its stage may have
    // moved, so the record rides along here rather than keeping its own watch.
    // It writes only when the mark actually moves.
    this.profile.reached(this.state.stage)
  }

  private bindPhysicalKeyboard(): void {
    // Desktop convenience during development; harmless on a phone.
    window.addEventListener("keydown", (event) => {
      if (event.metaKey || event.ctrlKey || event.altKey) return
      // Escape puts a tip down, and does nothing else while one is up. A tip
      // brought up by tabbing had no way down but tabbing on, since it belongs
      // to focus and focus stays put, so a player who tabbed to a relic to read
      // it was left with the panel over the board until they moved somewhere
      // else. Focus is left where it was, so the next Tab carries on from the
      // card rather than from the top of the page; the tip comes back on the
      // next focus or hover, because `hovered` is cleared with it.
      //
      // Above the sheet branch on purpose: a tip over an open sheet is the
      // nearer thing, and an Escape that closed the sheet under it would take
      // two things down to answer one key. The second Escape closes the sheet.
      //
      // Not a timeout, which was the other way to do it. A tip is read at the
      // reader's pace, and one timed for the relic that says six words would
      // vanish under the letter whose tip runs to five lines.
      // Ahead of everything, sheets included: the round most worth retrying is
      // the one whose game-over card is up. `code` rather than `key`, since
      // Alt+R is ® on a Mac. See `./admin`.
      if (admin && event.altKey && event.code === "KeyR") {
        this.retryRound()
        event.preventDefault()
        return
      }
      if (event.key === "Escape" && this.hovered) {
        this.showTip(null)
        event.preventDefault()
        return
      }
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
        // A text field is the one thing in a sheet that wants the rest of the
        // keyboard. Swallowed like everything else, the seed field could take
        // no letter at all; let past, the field's own handler owns Enter.
        if (event.target instanceof HTMLInputElement) return
        // Enter and Space are how a keyboard presses the button it has tabbed
        // to, and preventing them here is what made "Quit run" unreachable
        // without a mouse: the trap would walk focus onto it and neither key
        // would fire. So the swallow stops short of the two keys that are an
        // activation, and only when focus is inside the sheet. With focus
        // anywhere else Space is a page scroll behind the backdrop, which is
        // the thing modality is for.
        //
        // The scrolling keys stop short of it for the same reason. The rules
        // and the codex are longer than any phone, and swallowed with
        // everything else they let a keyboard open either and read the first
        // screen and nothing after it. Let through, the browser scrolls the
        // nearest scroll box at or above focus, which is the sheet's body
        // whenever focus is in it: the sheet opens with focus on the body
        // itself (see `holdFocus`), and a button or a codex heading tabbed to
        // is inside it. That is the browser's own step, smoothing and paging
        // rather than a copy of them; a hand-rolled `scrollBy` here was tried
        // first and was all of that again in 25 lines.
        //
        // Into the body and nowhere else. The pause sheet has no body, and the
        // nearest scroll box above a button in it is the screen behind the
        // backdrop, which an arrow there would scroll: the very thing the
        // swallow is for. Space keeps the wider gate, since on a button it is
        // a press and not a scroll.
        const focused = document.activeElement
        const inside = focused instanceof HTMLElement && focused.closest(".sheet") !== null
        const reading = focused instanceof HTMLElement && focused.closest(".sheet-body") !== null
        if (inside && (event.key === "Enter" || event.key === " ")) return
        if (reading && SCROLL_KEYS.has(event.key)) return
        event.preventDefault()
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
 * The keys a browser scrolls with, which an open sheet lets through to it when
 * focus is inside; see the overlay branch of `bindPhysicalKeyboard`. Space is
 * both: on a button it presses, anywhere else in the body it pages.
 */
const SCROLL_KEYS = new Set(["ArrowDown", "ArrowUp", "PageDown", "PageUp", "Home", "End", " "])

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
  // focus on the way in, and it sits before everything it contains. So does its
  // body, which holds focus on the way in where there is one (see
  // `holdFocus`) and is not a tab stop, so the browser's own step back from it
  // went to whatever was tabbable before the sheet: the board behind it.
  const body = sheet.querySelector(".sheet-body")
  const leaving = event.shiftKey
    ? active === first || active === sheet || active === body || !sheet.contains(active)
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

/**
 * Hang the card under the thing it is talking about, and point its tail at it.
 *
 * Measured rather than known, because the card spans the header and nothing in
 * the view knows where the figure it names came to rest: the readout sits at
 * the dock's right end, the score in the header's middle track, and both move
 * with the language and the digits.
 *
 * Under the anchor rather than under the header, for the two later beats. The
 * readout is the header's last line, so for the first three the two are the
 * same edge; the score and its bar are not, and hung from the header's foot
 * the card about the score sat under the readout, a line and a bar below the
 * number it was quoting. Under the anchor it lies over the header's lower
 * lines instead, which are the readout the card is not about while it is up.
 * The gap is read off the lit ring rather than written down, since the tray
 * draws its ring inside its own padding and everything else 5px outside: the
 * ring's reach plus a pixel, so the tail meets the ring rather than crossing
 * it. Under the readout that is 6px, the header's bottom padding, and the card
 * lands on the header's foot exactly.
 *
 * Above the anchor instead when the anchor is below the board, which is the
 * decoration switch on a phone and nothing else so far: hung under it the card would lie
 * over the keyboard the player is about to type on, and the board's empty
 * rows above it are the one place on the screen nobody is reading. `above`
 * turns the tail over to point down.
 *
 * Both rects are read in the same frame, so a transform on the screen (the
 * shake after a big guess) moves both and cancels. The tail is kept 14px in from
 * either corner, where the card's rounding would leave the point hanging off a
 * curve rather than an edge.
 *
 * Asked on the same pass the anchor is lit, which is every render and every
 * keystroke, and the readout widens as the chip count gains a digit, so the
 * point follows it rather than aiming at where the figure was.
 */
function aimCoach(card: HTMLElement, anchor: Element): void {
  const slot = card.parentElement
  if (!slot) return
  const target = anchor.getBoundingClientRect()
  const ring = getComputedStyle(anchor)
  const gap = Math.max(0, parseFloat(ring.outlineOffset) + parseFloat(ring.outlineWidth)) + 1
  if (isTable()) {
    aimCoachOnTable(card, slot, anchor, target, gap)
    return
  }
  const header = slot.offsetParent
  if (!header) return
  const top = header.getBoundingClientRect().top
  const above = target.top > window.innerHeight / 2
  card.classList.toggle("above", above)
  slot.style.top = above
    ? `${target.top - top - gap - card.offsetHeight}px`
    : `${target.bottom - top + gap}px`
  const box = card.getBoundingClientRect()
  const inset = 14
  const x = target.left + target.width / 2 - box.left
  card.style.setProperty("--tail", `${Math.max(inset, Math.min(box.width - inset, x))}px`)
}

/**
 * The desktop's half of `aimCoach`. The phone hangs the card from the header's
 * foot because the header is a strip across the top; the table's header is the
 * rail, a column the height of the window, and its foot is the bottom of the
 * screen. The banked beat, which rings the whole header, put its card there,
 * at y=892 in a 900px window: every word of "7 × 5 = 35" was written, and none
 * of it was on screen. The rail also clips what overflows it, so a card aimed
 * anywhere outside it was cut away.
 *
 * So on the table the slot is `fixed` (`table/coach.css`) and the card stands
 * beside the rail, out over the gutter between it and the board, level with
 * whatever it names and with its tail pointing back at it. The owner asked for
 * it there. Level with the anchor's middle, held inside the window, which for
 * the whole rail is the middle of the screen and for everything in it is the
 * figure being read back.
 *
 * The decor switch is in the rail on the table, in its foot, so its card stands
 * beside the rail like the rest, held inside the window by its bottom edge. An
 * anchor outside the rail (the shop's shelf and relics) keeps the phone's rule, above it or below it by which half of the
 * window it is in, and is centred on it rather than laid across it.
 *
 * `fixed` is measured from the viewport unless an ancestor is transformed, and
 * the shake after a big guess transforms the screen, so the origin is read off
 * the slot itself, parked at 0,0, rather than assumed.
 */
function aimCoachOnTable(
  card: HTMLElement,
  slot: HTMLElement,
  anchor: Element,
  target: DOMRect,
  gap: number,
): void {
  const rail = anchor.closest(".hud")
  // Beside the rail the card has the gutter up to the board and no more, which
  // at 1440×900 is 290px against the stylesheet's 20rem: at 320 it ran 26px
  // over the board's first column. Fitted to the gutter where that leaves a
  // card worth reading, and left at 20rem, over the board's edge, where a
  // narrow window leaves less than 14rem.
  // The tail's reach out of the card's side, half the diagonal of the 9px
  // square `coach.css` turns 45°, and 4px clear of the rail's edge besides.
  const tail = 10
  const board = rail && document.querySelector(".grid")?.getBoundingClientRect()
  const room = board ? board.left - rail.getBoundingClientRect().right - gap - tail - 12 : 0
  slot.style.width = room >= 224 ? `${Math.min(room, 360)}px` : ""
  slot.style.left = "0px"
  slot.style.top = "0px"
  const origin = slot.getBoundingClientRect()
  const width = card.offsetWidth
  const height = card.offsetHeight
  const edge = 12
  const inset = 14
  const within = (v: number, lo: number, hi: number) => Math.max(lo, Math.min(hi, v))
  let x: number
  let y: number
  if (rail) {
    x = rail.getBoundingClientRect().right + gap + tail
    y = within(target.top + target.height / 2 - height / 2, edge, innerHeight - height - edge)
    card.style.setProperty(
      "--tail",
      `${within(target.top + target.height / 2 - y, inset, height - inset)}px`,
    )
  } else {
    // Aimed at what the anchor holds rather than at the anchor. The table's
    // shelf is the whole stage, 12px from the window's top to 12px from its
    // foot, with five cards across its middle, and hung under the shelf the
    // first shop's card stood at y=890 in a 900px window, where the round's
    // banked card had stood. Under the cards it lies over the empty stage.
    // The ring stays on the anchor, which is what the sentence is about.
    const box = contentBox(anchor) ?? target
    let above = box.top > innerHeight / 2
    // And the other side when the rule's side would leave the window, which
    // nothing on the phone ever needed and a tall box on the table can.
    if (above ? box.top - gap - height < edge : box.bottom + gap + height > innerHeight - edge)
      above = !above
    card.classList.toggle("above", above)
    x = within(box.left + box.width / 2 - width / 2, edge, innerWidth - width - edge)
    y = above ? box.top - gap - height : box.bottom + gap
    card.style.setProperty(
      "--tail",
      `${within(box.left + box.width / 2 - x, inset, width - inset)}px`,
    )
  }
  card.classList.toggle("side", rail !== null)
  if (rail) card.classList.remove("above")
  slot.style.left = `${x - origin.left}px`
  slot.style.top = `${y - origin.top}px`
}

/** The box round an element's children as drawn, or null for an element with none. */
function contentBox(el: Element): DOMRect | null {
  const rects = [...el.children].map((child) => child.getBoundingClientRect())
  if (rects.length === 0) return null
  const left = Math.min(...rects.map((r) => r.left))
  const top = Math.min(...rects.map((r) => r.top))
  const right = Math.max(...rects.map((r) => r.right))
  const bottom = Math.max(...rects.map((r) => r.bottom))
  return new DOMRect(left, top, right - left, bottom - top)
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
    const parsed: unknown = JSON.parse(unseal(raw))
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
