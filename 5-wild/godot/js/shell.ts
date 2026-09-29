/**
 * The desktop build's `app.ts`: the run, the sheets, the settings and the
 * record, with the browser taken out.
 *
 * `src/ui/app.ts` is two things wound together. One is the game's *controller*:
 * which screen a phase gets, what a tap dispatches, when the intro card shows,
 * when the coaching retires, what the record counts. The other is the page:
 * patching rows in place, flipping tiles with timers, restoring focus, tips on
 * hover and on long-press. The first half is here, line for line where it can
 * be, so the two builds answer every "what happens when" the same way. The
 * second half is Godot's, which draws the trees this hands out and animates
 * from the events it is told about.
 *
 * Everything crosses as JSON text, as the engine seam does. Each call returns
 * an `Effects`, which is what Godot has to do about it: draw again, play these
 * cues, raise this toast, walk this guess's scoring. What is on screen is
 * always `render()`, which is the views, which is the web build's.
 *
 * Kept out on purpose:
 *
 * - Telemetry. The replay log exists to answer balance questions from the web
 *   build's players, and a Steam build sending runs to a Cloudflare worker would
 *   be a new privacy story rather than a port. `Chrome.sharing` is null, which
 *   is how the views already spell "this build has no endpoint", so the switch
 *   is not drawn.
 * - `seal`. It hides the answer from devtools; a desktop save sits in a file
 *   any player can open, and a key shipped in the bundle hides it from nobody
 *   who would look there. The run is stored as plain JSON.
 * - Everything `app.ts` does to survive a browser: `reuseBoard`, `patchDraft`,
 *   `settled`, `holdFocus`. Those are answers to Gecko, to Android WebViews and
 *   to the full-rebuild model meeting CSS; Godot has none of those questions.
 */

import "./dom-shim"
import type { Action, GameEvent, RunState, WordSource } from "../../src/engine"
import { MODIFIER_BY_ID, MULT_FOR_COLOR, reduce, startRun } from "../../src/engine"
import type { Cue } from "../../src/ui/audio"
import type { CoachStep } from "../../src/ui/coach"
import { coachAsks, coachSpent, coachStep } from "../../src/ui/coach"
import { formatNumber as num } from "../../src/ui/format"
import type { Lang } from "../../src/ui/lang"
import {
  categoryLevel,
  consumableNote,
  growthBadge,
  LANGS,
  loadLang,
  modPlaced,
  NEXT_LANG,
  payoutBadge,
  readLang,
  refusalText,
  relicCard,
  setLang,
  ui,
} from "../../src/ui/lang"
import { chosenAscension, Profile } from "../../src/ui/meta"
import type { Speed } from "../../src/ui/speed"
import { loadSpeed, NEXT_SPEED, setSpeed } from "../../src/ui/speed"
import type { Theme } from "../../src/ui/theme"
import type { Chrome, Decor, Handlers, SoundLevel } from "../../src/ui/views"
import {
  aboutView,
  ascendView,
  codexView,
  creditsView,
  displacedAt,
  endView,
  helpView,
  introView,
  menuView,
  NEXT_DECOR,
  NEXT_SOUND,
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
} from "../../src/ui/views"
import type { FakeElement, Node } from "./dom-shim"
import { click, serialize, store } from "./dom-shim"

/* The web build's keys, so the two read the same record the same way. */
const SAVE_KEY = "5wild:run:v2"
const RUN_LANG_KEY = "5wild:run:lang"
const COACH_KEY = "5wild:coached"
const PLAIN_KEY = "5wild:plain"
const THEME_KEY = "5wild:theme"
const MUTE_KEY = "5wild:muted"
const MUSIC_KEY = "5wild:music"
const TRACK_KEY = "5wild:track"

/** `music.ts`'s two recordings, which Godot plays. Titles are the same in every language. */
const TRACKS = [
  { id: "promises", title: "promises" },
  { id: "forget-me-not", title: "Forget-me-not" },
] as const

type Overlay =
  | "help"
  | "codex"
  | "shapes"
  | "stats"
  | "menu"
  | "quit"
  | "ascend"
  | "about"
  | "credits"
  | null

/**
 * What Godot has to do after a call. Every field is optional and most calls
 * set one or two.
 *
 * `animate` is a submit's scoring, already walked: Godot draws the board
 * (which `render` returns with the guess already on it), holds the new row's
 * colours back, plays the steps, then calls `settle`. See `script`.
 */
type Effects = {
  render?: boolean
  cues?: Cue[]
  toast?: string
  /** A refusal: shake the row being typed. */
  shake?: boolean
  /** A selector whose node should bump, as `app.ts`'s `bump`. */
  bump?: string
  animate?: Script
  /** A link to open outside the game. */
  open?: string
}

let effects: Effects = {}
const cue = (c: Cue) => {
  effects.cues ??= []
  effects.cues.push(c)
}

/**
 * The word lists Godot has handed over, by language.
 *
 * The web build fetches the next run's list the moment the setting changes, so
 * that `newRun` rarely waits. Here the lists are files beside the game and
 * reading one is a millisecond, so `wanted` names what is missing and Godot
 * fills it in before the call that asked has returned to the player.
 */
const lists = new Map<Lang, WordSource>()
const lines = (text: string): string[] => text.split("\n").filter(Boolean)

class Shell {
  state!: RunState
  words!: WordSource
  wordsLang: Lang = "en"
  busy = false
  intro = false
  atTitle = true
  overlay: Overlay = null
  sheetHeard: Overlay = null
  ascendTo = 0
  arming: string | null = null
  decor: Decor = "all"
  speed: Speed = 1
  theme: Theme = "dark"
  /** Whether a theme was ever picked; until then the device's look is followed. */
  themePicked = false
  lang: Lang = "en"
  muted = false
  musicOff = false
  track = 0
  coachOwed = true
  /** The screen the scoring animation is being played over, while it is. */
  renderAs: "round" | null = null
  profile!: Profile

  boot(deviceTheme: Theme): void {
    this.decor = loadDecor()
    this.speed = loadSpeed()
    const picked = get(THEME_KEY)
    this.themePicked = picked === "light" || picked === "dark"
    this.theme = this.themePicked ? (picked as Theme) : deviceTheme
    this.lang = loadLang()
    setLang(this.lang)
    setSpeed(this.speed)
    this.muted = get(MUTE_KEY) === "1"
    this.musicOff = get(MUSIC_KEY) === "0"
    this.track = Math.max(
      0,
      TRACKS.findIndex((track) => track.id === get(TRACK_KEY)),
    )
    this.coachOwed = get(COACH_KEY) !== "1"
    this.profile = new Profile()

    const saved = loadSave()
    // A save from before the key was English, as on the web.
    const runLang = saved ? (readLang(get(RUN_LANG_KEY)) ?? "en") : this.lang
    this.wordsLang = lists.has(runLang) ? runLang : this.lang
    this.words = need(lists.get(this.wordsLang), `no words for ${this.wordsLang}`)
    this.state = saved ?? startRun(rootSeed(), this.words).state
    this.atTitle = saved === null
    if (!this.atTitle) {
      this.intro = this.state.round.guesses.length === 0 && this.state.phase === "round"
    }
  }

  /* ------------------------------------------------------------- dispatch */

  dispatch(action: Action): void {
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
    if (!this.state.placing) this.arming = null
    this.tally(before)
    if (this.state.phase === "round" && wasPhase !== "round") {
      this.intro = true
      cue({ name: "intro", boss: Boolean(this.state.round.bossId) })
    }
    this.save()
    if (events.some((event) => event.type === "run_won")) {
      this.profile.won(this.state.ascension ?? 0)
    }

    const paid = events.some((event) => event.type === "gold")
    const heard = actionCue(action, events)
    if (heard) cue(heard)
    const consumed = events.find((event) => event.type === "consumable")
    const placed = events.find((event) => event.type === "mod_placed")
    const label = consumed
      ? consumableNote(consumed.note)
      : placed
        ? modPlaced(placed.id, placed.letter)
        : undefined

    effects.render = true
    if (paid) effects.bump = ".hud-gold"
    if (label) effects.toast = label
  }

  /** As `app.ts`'s, minus the animation, which Godot plays and then calls `settle`. */
  submit(): void {
    if (this.busy) return
    const before = this.state
    const { state, events } = reduce(this.state, { type: "submit" }, this.words)
    const refusal = events.find((event) => event.type === "rejected")
    if (refusal) {
      this.refuse(refusal.refusal)
      return
    }
    this.state = state
    this.tally(before)
    this.save()
    this.busy = true
    this.renderAs = "round"
    effects.render = true
    effects.animate = script(events, this.state.round)
  }

  /** The animation is over: the screen moves on to wherever the run now is. */
  settle(): void {
    if (!this.busy) return
    this.busy = false
    this.renderAs = null
    effects.render = true
    if (this.state.phase === "game_over") cue({ name: "lose" })
    else if (this.state.phase === "reward" || this.state.phase === "victory") {
      cue({ name: "win", run: this.state.phase === "victory" })
    }
  }

  refuse(refusal: Parameters<typeof refusalText>[0]): void {
    effects.toast = refusalText(refusal)
    cue({ name: "reject" })
    if (this.state.phase === "round") effects.shake = true
  }

  /* ------------------------------------------------------------ handlers */

  readonly handlers: Handlers = {
    key: (letter) => {
      cue({ name: "key" })
      this.dispatch({ type: "type_letter", letter })
    },
    enter: () => this.submit(),
    back: () => {
      cue({ name: "back" })
      this.dispatch({ type: "backspace" })
    },
    useConsumable: (index) => this.dispatch({ type: "use_consumable", index }),
    collect: () => this.dispatch({ type: "collect" }),
    buy: (index) => this.dispatch({ type: "buy", index }),
    sell: (index) => this.dispatch({ type: "sell_relic", index }),
    reroll: () => this.dispatch({ type: "reroll" }),
    nextRound: () => this.dispatch({ type: "next_round" }),
    continueRun: () => this.dispatch({ type: "continue_run" }),
    pickPack: (index) => this.dispatch({ type: "pick_pack", index }),
    skipPack: () => this.dispatch({ type: "skip_pack" }),
    placeMod: (letter) => {
      cue({ name: "key" })
      const armed = this.arming === letter
      const modifier = this.state.placing ? MODIFIER_BY_ID.get(this.state.placing) : undefined
      const trade =
        modifier !== undefined && displacedAt(this.state, modifier, letter) !== undefined
      if (trade && !armed) {
        this.arming = letter
        effects.render = true
        return
      }
      this.arming = null
      this.dispatch({ type: "place_mod", letter })
    },
    cancelPlace: () => {
      this.arming = null
      effects.render = true
    },
    newRun: () => this.startFresh(),
    setAscension: (level) => {
      this.profile.chose(level)
      effects.render = true
    },
    play: () => {
      this.intro = false
      effects.render = true
    },
    cycleSound: () => {
      const next = NEXT_SOUND[this.soundLevel]
      this.setMuted(next === "off")
      this.setMusicOff(next !== "music")
      effects.render = true
    },
    toggleEffects: () => {
      this.setMuted(!this.muted)
      effects.render = true
    },
    toggleMusic: () => {
      this.setMusicOff(!this.musicOff)
      effects.render = true
    },
    nextTrack: () => {
      this.track = (this.track + 1) % TRACKS.length
      put(TRACK_KEY, TRACKS[this.track]?.id ?? "promises")
      effects.render = true
    },
    setSharing: () => {},
    cycleDecor: () => {
      this.decor = NEXT_DECOR[this.decor]
      put(PLAIN_KEY, this.decor)
      effects.render = true
    },
    cycleSpeed: () => {
      this.speed = NEXT_SPEED[this.speed]
      setSpeed(this.speed)
      effects.render = true
    },
    toggleTheme: () => {
      this.theme = this.theme === "light" ? "dark" : "light"
      this.themePicked = true
      put(THEME_KEY, this.theme)
      effects.render = true
    },
    cycleLanguage: () => {
      this.lang = NEXT_LANG[this.lang]
      setLang(this.lang)
      effects.render = true
    },
    openMenu: () => {
      if (this.busy) return
      this.open("menu")
    },
    openHelp: () => this.open("help"),
    skipCoach: () => {
      this.coachOwed = false
      put(COACH_KEY, "1")
      this.intro = false
      effects.render = true
    },
    startTutorial: () => this.startFresh(false),
    openCodex: () => this.open("codex"),
    openShapes: () => this.open("shapes"),
    openStats: () => this.open("stats"),
    openAbout: () => this.open("about"),
    openCredits: () => this.open("credits"),
    closeOverlay: () => this.open(null),
    askQuit: () => this.open("quit"),
    askAscend: (level) => {
      this.ascendTo = level
      this.open("ascend")
    },
    ascend: () => {
      this.profile.chose(this.ascendTo)
      this.open(null)
    },
    quit: () => {
      clearSave()
      this.state = startRun(rootSeed(), this.words).state
      this.atTitle = true
      this.overlay = null
      this.intro = false
      effects.render = true
    },
  }

  private open(overlay: Overlay): void {
    this.overlay = overlay
    effects.render = true
  }

  private setMuted(muted: boolean): void {
    this.muted = muted
    put(MUTE_KEY, muted ? "1" : "0")
  }

  private setMusicOff(off: boolean): void {
    this.musicOff = off
    put(MUSIC_KEY, off ? "0" : "1")
  }

  get soundLevel(): SoundLevel {
    if (this.muted) return this.musicOff ? "off" : "musicOnly"
    return this.musicOff ? "sound" : "music"
  }

  /* ---------------------------------------------------------------- keys */

  /**
   * A physical key, as `bindPhysicalKeyboard` reads one: a letter, or one of
   * `enter`, `back`, `escape`.
   *
   * The order of the questions is `app.ts`'s, because each one is a screen on
   * which the key means something different: an open sheet eats everything but
   * Escape, the picker turns letters into placements, the intro card turns
   * anything into "play". What `app.ts` does with focus and Tab is Godot's.
   */
  key(key: string): void {
    const letter = /^[a-z]$/.test(key) ? key : null
    if (this.overlay) {
      if (key === "escape") this.handlers.closeOverlay()
      return
    }
    if (this.atTitle) return
    if (this.state.placing) {
      if (this.arming && key === "escape") {
        this.handlers.cancelPlace()
        return
      }
      if (letter) {
        this.handlers.placeMod(letter)
        return
      }
    }
    if (key === "escape") {
      this.handlers.openMenu()
      return
    }
    if (this.state.phase !== "round") return
    if (this.intro) {
      if (this.coachOffer && key !== "enter") return
      this.handlers.play()
      return
    }
    if (key === "enter") this.handlers.enter()
    else if (key === "back") this.handlers.back()
    else if (letter) this.handlers.key(letter)
  }

  /* ---------------------------------------------------------- run control */

  private startFresh(intro = true): void {
    // The deferred list, which Godot has already put in hand: see `wanted`.
    const next = lists.get(this.lang)
    if (next) {
      this.words = next
      this.wordsLang = this.lang
    }
    this.state = startRun(rootSeed(), this.words, chosenAscension(this.profile.stats)).state
    this.atTitle = false
    this.overlay = null
    this.intro = intro
    if (intro) cue({ name: "intro", boss: false })
    this.profile.started()
    this.save()
    effects.render = true
  }

  get wordsDeferred(): boolean {
    return !this.atTitle && this.lang !== this.wordsLang
  }

  /** Languages whose lists are wanted and not yet handed over. */
  wanted(): Lang[] {
    return [this.lang].filter((lang) => !lists.has(lang))
  }

  get tutorialOffer(): boolean {
    return this.coachOwed && this.atTitle
  }

  get coachOffer(): boolean {
    if (!this.coachOwed || !this.intro || this.overlay || this.atTitle) return false
    return coachAsks(this.state)
  }

  get coach(): CoachStep | null {
    if (!this.coachOwed || this.busy || this.intro || this.overlay || this.atTitle) return null
    return coachStep(this.state)
  }

  get chrome(): Chrome {
    return {
      sound: this.soundLevel,
      effectsOff: this.muted,
      musicOff: this.musicOff,
      track: TRACKS[this.track]?.title ?? "",
      decor: this.decor,
      speed: this.speed,
      theme: this.theme,
      lang: this.lang,
      wordsDeferred: this.wordsDeferred,
      coach: this.coach,
      coachOffer: this.coachOffer,
      sharing: null,
      thanked: null,
    }
  }

  /* -------------------------------------------------------------- render */

  /** Which screen, and which sheet over it: `app.ts`'s `render`, choosing only. */
  render(): { screen: FakeElement; sheet: FakeElement | null } {
    if (this.coachOwed && !this.atTitle && coachSpent(this.state)) {
      this.coachOwed = false
      put(COACH_KEY, "1")
    }
    const phase = this.renderAs ?? this.state.phase
    const on = this.handlers
    const screen = this.atTitle
      ? titleView(on, this.chrome, this.profile.stats)
      : phase === "round" && this.intro && !this.renderAs
        ? introView(this.state, on, this.chrome)
        : phase === "reward"
          ? rewardView(this.state, on)
          : phase === "shop"
            ? shopView(this.state, on, this.coach)
            : phase === "game_over" || phase === "victory"
              ? endView(this.state, on)
              : roundView(this.state, on, this.chrome)

    const sheet =
      this.overlay === "help"
        ? helpView(on, this.tutorialOffer)
        : this.overlay === "codex"
          ? codexView(on)
          : this.overlay === "shapes"
            ? shapesView(this.state, on, phase === "round" ? wordInPlay(this.state) : "")
            : this.overlay === "stats"
              ? statsView(
                  this.profile.stats,
                  { answers: this.words.answers.length, allowed: this.words.allowed.size },
                  this.wordsLang,
                  on,
                )
              : this.overlay === "menu"
                ? menuView(on, this.chrome)
                : this.overlay === "about"
                  ? aboutView(on, this.chrome)
                  : this.overlay === "credits"
                    ? creditsView(on)
                    : this.overlay === "quit"
                      ? quitView(this.state, on)
                      : this.overlay === "ascend"
                        ? ascendView(this.ascendTo, on)
                        : (placeView(this.state, on, this.arming) ?? packView(this.state, on))

    if (this.overlay !== this.sheetHeard) {
      cue({ name: "sheet", open: this.overlay !== null })
      this.sheetHeard = this.overlay
    }
    return {
      screen: screen as unknown as FakeElement,
      sheet: sheet as unknown as FakeElement | null,
    }
  }

  /* ----------------------------------------------------------------- save */

  private tally(before: RunState): void {
    const round = this.state.round
    const played = round.guesses[before.round.guesses.length]
    if (played) this.profile.guessed(played.word, this.wordsLang)
    if (round.solved && !before.round.solved) {
      this.profile.solved(round.answer, round.guesses.length, this.wordsLang)
    } else if (round.done && !before.round.done) {
      this.profile.missed()
    }
    const held = new Set(before.relics.map((relic) => relic.id))
    for (const relic of this.state.relics) {
      if (!held.has(relic.id)) this.profile.took(relic.id)
    }
  }

  private save(): void {
    put(SAVE_KEY, JSON.stringify(this.state))
    put(RUN_LANG_KEY, this.wordsLang)
    this.profile.reached(this.state.stage)
  }
}

/**
 * One beat of a guess being scored, as `animate` in `app.ts` walks it. Every
 * figure is already a string in the language in force and every sentence is
 * already written, so GDScript formats nothing and says nothing: it lights
 * what it is told to, shows what it is handed, and waits `wait` milliseconds
 * (at speed ×1; Godot divides by the speed, as `beat` does).
 */
type Step = {
  /** A tile of the new row turns over to this colour, with its tile cue. */
  flip?: { index: number; color: string; chips: string; mult: string | null }
  /** The row's boss note is let in with this flip. */
  note?: boolean
  /** Lights a tile, a relic seat or the category line for the step. */
  fire?: { at: "tile" | "relic" | "category"; index: number }
  /** Rises off the readout. */
  float?: string
  /** The readout's two halves; `null` where that half did not move. */
  chips?: string | null
  mult?: string | null
  /** The readout goes gold: the solve bonus. */
  solved?: boolean
  /** The total counts from one figure to the other; `num` formats each frame. */
  count?: { from: number; to: number }
  /** `emphasize`: how hard the readout pops and whether the screen shakes. */
  ratio?: number
  cue?: Cue
  wait: number
}

type Script = {
  row: number
  target: number
  /** What the readout and the total are wound back to before the first step. */
  chips: string
  mult: string
  score: number
  steps: Step[]
}

/** `app.ts`'s pacing, in milliseconds at speed ×1. */
const PACE = { tile: 170, relic: 150, solve: 900, total: 400 }

/**
 * `animate` in `app.ts`, done ahead of time. It is the same walk over the same
 * events, making the same choices (which half of the readout moved, which rung
 * the trigger cue climbs to, what floats), recorded instead of performed.
 */
function script(events: readonly GameEvent[], round: RunState["round"]): Script {
  const row = round.guesses.length - 1
  const target = round.target
  const tiles = round.guesses[row]?.tiles ?? []
  const scored = events.find((event) => event.type === "guess_scored")
  let onScreen = scored ? scored.total - scored.score : 0
  let shownChips = 0
  let shownMult = 1
  let fired = 0
  const readout = (chips: number, mult: number): Pick<Step, "chips" | "mult"> => {
    const out = {
      chips: chips === shownChips ? null : num(chips),
      mult: mult === shownMult ? null : num(mult),
    }
    shownChips = chips
    shownMult = mult
    return out
  }
  const last = events.reduce((most, event) => (event.type === "tile" ? event.index : most), -1)
  const steps: Step[] = []
  for (const event of events) {
    switch (event.type) {
      case "tile": {
        // What the tile is drawn as, which is what `app.ts` reads off its
        // class: under a lying boss that is `shown`, not what scored.
        const color = tiles[event.index]?.shown ?? "gray"
        const mult = MULT_FOR_COLOR[color]
        steps.push({
          flip: {
            index: event.index,
            color,
            chips: `+${num(event.gained)}`,
            mult: mult > 0 ? `+${num(mult)}` : null,
          },
          note: event.index === last,
          ...readout(event.chips, event.mult),
          cue: { name: "tile", index: event.index, color },
          wait: PACE.tile,
        })
        break
      }
      case "mod":
      case "relic":
      case "category":
        steps.push({
          fire: {
            at: event.type === "mod" ? "tile" : event.type,
            index: event.type === "mod" ? event.index : event.type === "relic" ? event.slot : 0,
          },
          float:
            event.type === "category"
              ? categoryLevel(event.id, event.level)
              : payoutBadge(event.paid),
          ...readout(event.chips, event.mult),
          cue: { name: "trigger", kind: event.type, n: fired++ },
          wait: PACE.relic,
        })
        break
      case "relic_grew":
        steps.push({
          fire: { at: "relic", index: event.slot },
          float: growthBadge(event),
          cue: { name: "trigger", kind: "grew", n: fired++ },
          wait: PACE.relic,
        })
        break
      case "solve_bonus":
        steps.push({
          float: ui().board.solveFactor(event.factor),
          solved: true,
          count: { from: onScreen, to: event.total },
          ratio: event.total / Math.max(1, target),
          cue: { name: "solve" },
          wait: PACE.solve,
        })
        onScreen = event.total
        break
      case "guess_scored":
        steps.push({
          count: { from: event.total - event.score, to: event.total },
          ratio: event.score / Math.max(1, target),
          cue: { name: "score", ratio: event.score / Math.max(1, target) },
          wait: PACE.total,
        })
        onScreen = event.total
        break
      case "letter_destroyed":
      case "relic_destroyed":
        steps.push({
          float:
            event.type === "letter_destroyed"
              ? ui().board.letterBroken(event.letter)
              : ui().board.relicGone(relicCard(event.id).name),
          cue: { name: "break" },
          wait: PACE.relic,
        })
        break
      default:
        break
    }
  }
  return {
    row,
    target,
    chips: num(0),
    mult: num(1),
    score: scored ? scored.total - scored.score : 0,
    steps,
  }
}

/** `app.ts`'s `actionCue`, unchanged. */
function actionCue(action: Action, events: readonly GameEvent[]): Cue | null {
  for (const event of events) {
    if (event.type === "mod_placed") return { name: "place" }
    if (event.type === "consumable") return { name: "consume" }
    if (event.type === "pack_opened") return { name: "pack" }
    if (event.type === "pack_picked" && event.taken) return { name: "pick" }
    if (event.type === "shop_entered") return { name: "reroll", shop: true }
  }
  if (action.type === "reroll") return { name: "reroll" }
  const gold = events.find((event) => event.type === "gold")
  if (!gold) return null
  if (gold.reason === "purchase") return { name: "buy" }
  if (gold.reason === "sold") return { name: "sell" }
  return { name: "coin" }
}

const get = (key: string): string | null => localStorage.getItem(key)
const put = (key: string, value: string): void => localStorage.setItem(key, value)

function loadDecor(): Decor {
  const saved = get(PLAIN_KEY)
  return saved === "minimal" || saved === "none" ? saved : "all"
}

function clearSave(): void {
  localStorage.removeItem(SAVE_KEY)
  localStorage.removeItem(RUN_LANG_KEY)
}

function loadSave(): RunState | null {
  try {
    const raw = get(SAVE_KEY)
    if (!raw) return null
    const parsed: unknown = JSON.parse(raw)
    if (typeof parsed !== "object" || parsed === null) return null
    const state = parsed as RunState
    return typeof state.seed === "number" && typeof state.phase === "string" && state.round
      ? state
      : null
  } catch {
    // A sealed save from the web build, or garbage: either way, not a run.
    return null
  }
}

const rootSeed = (): number => Math.floor(Math.random() * 2 ** 31)

function need<T>(value: T | undefined, what: string): T {
  if (value === undefined) throw new Error(`shell: ${what}`)
  return value
}

/* ------------------------------------------------------------------- seam */

const shell = new Shell()
/** The tree last drawn, so a click can be handed the node it landed on. */
let drawn: FakeElement | null = null

/** Runs one input and returns what Godot has to do about it, as JSON. */
function run(input: () => void): string {
  effects = {}
  input()
  const out = effects
  effects = {}
  return JSON.stringify(out)
}

const api = {
  /** The whole store, as `{key: value}` JSON, before `boot`. */
  load(items: string): string {
    store.items.clear()
    for (const [key, value] of Object.entries(JSON.parse(items) as Record<string, string>)) {
      store.items.set(key, value)
    }
    store.dirty = false
    return ""
  },

  /**
   * The languages whose lists Godot has to hand over before the next input.
   * Before `boot`, the interface's and the saved run's.
   */
  wanted(): string {
    if (shell.profile) return JSON.stringify(shell.wanted())
    const lang = loadLang()
    const run = get(SAVE_KEY) ? (readLang(get(RUN_LANG_KEY)) ?? "en") : lang
    return JSON.stringify([...new Set([lang, run])].filter((entry) => !lists.has(entry)))
  },

  words(lang: string, answers: string, allowed: string): string {
    const known = readLang(lang)
    if (!known) return "0"
    lists.set(known, { answers: lines(answers), allowed: new Set(lines(allowed)) })
    return String(lists.get(known)?.answers.length ?? 0)
  },

  boot: (deviceTheme: string): string =>
    run(() => shell.boot(deviceTheme === "light" ? "light" : "dark")),

  /**
   * The screen and its sheet as JSON trees, plus what the stylesheet would
   * otherwise have read off the document root: the theme, the decor, the
   * speed, and the coaching card's anchor.
   */
  render(): string {
    const { screen, sheet } = shell.render()
    // One tree, so one set of handler ids covers both.
    const holder = document.createElement("div") as unknown as FakeElement
    holder.append(screen, ...(sheet ? [sheet] : []))
    // `lightCoach`, done on the tree: the card names its anchor by selector.
    const anchor = shell.coach?.anchor
    if (anchor) holder.querySelector(anchor)?.classList.add("coached")
    drawn = holder
    const tree = serialize(holder)
    const kids = typeof tree === "string" ? [] : (tree.k ?? [])
    const cues = effects.cues ?? []
    effects.cues = []
    return JSON.stringify({
      screen: kids[0] ?? null,
      sheet: (kids[1] as Node | undefined) ?? null,
      theme: shell.theme,
      decor: shell.decor,
      speed: shell.speed,
      lang: shell.lang,
      muted: shell.muted,
      musicOff: shell.musicOff,
      track: TRACKS[shell.track]?.id ?? "promises",
      busy: shell.busy,
      // Sheet sounds are decided while choosing the sheet, so they ride here.
      cues,
    })
  },

  click: (id: string): string =>
    run(() => {
      if (drawn) click(Number(id), drawn)
    }),

  key: (key: string): string => run(() => shell.key(key)),

  settle: (): string => run(() => shell.settle()),

  /** The store as JSON if anything was written since the last ask, else "". */
  flush(): string {
    if (!store.dirty) return ""
    store.dirty = false
    return JSON.stringify(Object.fromEntries(store.items))
  },

  /** For tests and the agent bridge: the run as the engine holds it. */
  state: (): string => JSON.stringify(shell.state),

  /** For tests: a run from a known seed, as `newRun` would start it. */
  startSeeded(seed: string): string {
    return run(() => {
      shell.state = startRun(Number(seed), shell.words, 0).state
      shell.atTitle = false
      shell.intro = false
      shell.overlay = null
      effects.render = true
    })
  },

  langs: (): string => JSON.stringify(LANGS),

  /** A figure as the language in force writes it, for the count-up's frames. */
  num: (value: string): string => num(Number(value)),
}

;(globalThis as { fivewildShell?: typeof api }).fivewildShell = api
