/**
 * The soundtrack, generated rather than recorded.
 *
 * Same constraint as the sound effects: no audio files, nothing fetched, so the
 * game still has music in airplane mode and the APK does not grow by a megabyte
 * per track. What that buys instead of a recording is a score that never repeats
 * exactly and that changes shape with the screen: the shop is not the boss.
 *
 * Notes are scheduled ahead on the audio clock rather than fired from a timer,
 * because `setInterval` drifts by tens of milliseconds under load and a rhythm
 * built on it audibly stumbles every time the scoring animation runs. The timer
 * only decides *what* to queue; the audio clock decides when it sounds.
 */

import { audioContext, C5, step } from "./audio"

const MUSIC_KEY = "5wild:music"

/** How far ahead notes are queued, and how often the queue is topped up. */
const LOOKAHEAD_S = 0.25
const TICK_MS = 60

/** Eighth notes per bar. Everything here is in 4/4. */
const STEPS_PER_BAR = 8

export type Mood = "title" | "round" | "boss" | "shop" | "over"

type Palette = {
  bpm: number
  /** Semitones off C5 for the tonic. Low numbers sit under the effects. */
  key: number
  /** Scale degrees, in semitones. */
  scale: readonly number[]
  /** Root of the bar's chord, one per bar of a four-bar phrase. */
  changes: readonly number[]
  /** Chance a given eighth gets a melody note. */
  density: number
  wave: OscillatorType
  gain: number
}

/**
 * Minor pentatonic for the run, major for the shop. The scale is doing the
 * emotional work here. The shop is the only screen where nothing can go wrong,
 * and it is the only one that sounds like it.
 */
const MINOR = [0, 3, 5, 7, 10] as const
const MAJOR = [0, 2, 4, 7, 9] as const

const PALETTES: Record<Mood, Palette> = {
  // Slow, sparse, unresolved: a menu should feel like it is waiting for you.
  title: {
    bpm: 68,
    key: -12,
    scale: MINOR,
    changes: [0, 0, -4, -5],
    density: 0.2,
    wave: "sine",
    gain: 0.05,
  },
  round: {
    bpm: 96,
    key: -12,
    scale: MINOR,
    changes: [0, -4, -5, -4],
    density: 0.3,
    wave: "triangle",
    gain: 0.045,
  },
  // A semitone-heavy change and a reedier wave. It should be recognizable as
  // the boss before the card is read.
  boss: {
    bpm: 112,
    key: -13,
    scale: MINOR,
    changes: [0, -1, -5, -6],
    density: 0.4,
    wave: "sawtooth",
    gain: 0.04,
  },
  shop: {
    bpm: 84,
    key: -10,
    scale: MAJOR,
    changes: [0, 5, 3, 7],
    density: 0.35,
    wave: "triangle",
    gain: 0.05,
  },
  // Nothing left to play for, so the phrase sinks and never comes back up.
  over: {
    bpm: 60,
    key: -17,
    scale: MINOR,
    changes: [0, -2, -5, -7],
    density: 0.15,
    wave: "sine",
    gain: 0.05,
  },
}

/**
 * A tiny LCG rather than `Math.random`, so a phrase can be reproduced from a
 * step number, which is useful when a melody sounds wrong and needs hearing again.
 */
function noise(seed: number): number {
  const x = Math.sin(seed * 12.9898) * 43758.5453
  return x - Math.floor(x)
}

export class Music {
  private off: boolean
  private mood: Mood = "title"
  private started = false
  private timer: number | null = null
  private bus: GainNode | null = null
  /** Audio-clock time the next eighth note is due. */
  private nextAt = 0
  private step = 0

  constructor() {
    let stored: string | null = null
    try {
      stored = localStorage.getItem(MUSIC_KEY)
    } catch {
      // A blocked store just means the preference does not survive the session.
    }
    // Off unless the player has asked for it. A game that starts singing on its
    // own is a game opened on a bus with the volume up, and the soundtrack is
    // generative, so there is no track to recognize and want back, so silence is
    // the safer first impression. The stored preference still wins both ways.
    this.off = stored !== "1"
  }

  get isOff(): boolean {
    return this.off
  }

  /**
   * Called from the first real user gesture. Until then there is deliberately no
   * AudioContext at all: one built before a gesture starts life suspended, and
   * mobile browsers hold that against the page.
   */
  enable(): void {
    if (this.started || this.off) return
    const ctx = audioContext()
    if (!ctx) return
    void ctx.resume()
    this.started = true
    this.bus = ctx.createGain()
    this.bus.gain.setValueAtTime(0, ctx.currentTime)
    this.bus.gain.linearRampToValueAtTime(PALETTES[this.mood].gain, ctx.currentTime + 1.5)
    this.bus.connect(ctx.destination)
    this.nextAt = ctx.currentTime + 0.1
    this.run()
  }

  set(mood: Mood): void {
    if (mood === this.mood) return
    this.mood = mood
    const ctx = audioContext()
    if (!this.bus || !ctx) return
    // Ride the gain across the change rather than cutting: the palettes differ
    // in tempo, so a hard switch lands mid-beat and reads as a glitch.
    this.bus.gain.cancelScheduledValues(ctx.currentTime)
    this.bus.gain.setValueAtTime(this.bus.gain.value, ctx.currentTime)
    this.bus.gain.linearRampToValueAtTime(0.0001, ctx.currentTime + 0.25)
    this.bus.gain.linearRampToValueAtTime(PALETTES[mood].gain, ctx.currentTime + 1)
  }

  /** Toggles music alone; the effects keep their own switch. */
  toggle(): boolean {
    this.off = !this.off
    try {
      localStorage.setItem(MUSIC_KEY, this.off ? "0" : "1")
    } catch {
      // See above. The switch works, it just will not be remembered.
    }
    if (this.off) this.stop()
    else this.enable()
    return this.off
  }

  /** Silence without forgetting the preference, for muting and for tab-away. */
  suspend(): void {
    if (this.timer !== null) {
      clearInterval(this.timer)
      this.timer = null
    }
    const ctx = audioContext()
    if (this.bus && ctx) {
      this.bus.gain.cancelScheduledValues(ctx.currentTime)
      this.bus.gain.setValueAtTime(0.0001, ctx.currentTime)
    }
  }

  resume(): void {
    if (this.off || !this.started || this.timer !== null) return
    const ctx = audioContext()
    if (!ctx || !this.bus) return
    this.bus.gain.linearRampToValueAtTime(PALETTES[this.mood].gain, ctx.currentTime + 0.75)
    this.nextAt = Math.max(this.nextAt, ctx.currentTime + 0.1)
    this.run()
  }

  private stop(): void {
    this.suspend()
    this.started = false
    this.bus?.disconnect()
    this.bus = null
  }

  private run(): void {
    this.timer = window.setInterval(() => this.fill(), TICK_MS)
    this.fill()
  }

  /** Queue every note that comes due inside the lookahead window. */
  private fill(): void {
    const ctx = audioContext()
    if (!ctx || !this.bus) return
    const palette = PALETTES[this.mood]
    const eighth = 30 / palette.bpm

    // A long stall, from a backgrounded tab or a slow frame, leaves `nextAt` in the
    // past, and catching up note by note would dump the whole backlog at once.
    if (this.nextAt < ctx.currentTime) this.nextAt = ctx.currentTime + 0.05

    while (this.nextAt < ctx.currentTime + LOOKAHEAD_S) {
      this.emit(palette, this.step, this.nextAt)
      this.nextAt += eighth
      this.step++
    }
  }

  private emit(palette: Palette, index: number, at: number): void {
    const bar = Math.floor(index / STEPS_PER_BAR)
    const beat = index % STEPS_PER_BAR
    const chord = palette.changes[bar % palette.changes.length] ?? 0
    const root = palette.key + chord

    // Bass on the downbeat and the half: the pulse everything else hangs off.
    if (beat === 0 || beat === 4) {
      this.voice(at, step(C5, root - 12), 60 / palette.bpm, "sine", beat === 0 ? 0.55 : 0.3)
    }

    // A fifth held across the bar, quiet enough to read as room rather than
    // as a part. It is what keeps the sparse melody from sounding like typing.
    if (beat === 0) {
      this.voice(at, step(C5, root - 5), 240 / palette.bpm, palette.wave, 0.14)
    }

    if (noise(index * 1.7 + chord) < palette.density) {
      const degrees = palette.scale
      const pick = degrees[Math.floor(noise(index * 3.1) * degrees.length)] ?? 0
      const octave = noise(index * 5.3) < 0.3 ? 12 : 0
      this.voice(at, step(C5, root + pick + octave), 45 / palette.bpm, palette.wave, 0.22)
    }
  }

  /** `start` is an audio-clock time, always in the future, never `currentTime`. */
  private voice(
    start: number,
    freq: number,
    seconds: number,
    wave: OscillatorType,
    gain: number,
  ): void {
    const ctx = audioContext()
    if (!ctx || !this.bus) return
    const end = start + seconds

    const osc = ctx.createOscillator()
    const amp = ctx.createGain()
    osc.type = wave
    osc.frequency.setValueAtTime(freq, start)

    // Slower attack than the effects use: this sits behind them, and a sharp
    // transient here would compete with the tile reveals rather than bed them.
    amp.gain.setValueAtTime(0.0001, start)
    amp.gain.exponentialRampToValueAtTime(gain, start + Math.min(0.08, seconds / 3))
    amp.gain.exponentialRampToValueAtTime(0.0001, end)

    osc.connect(amp).connect(this.bus)
    osc.start(start)
    osc.stop(end + 0.05)
  }
}
