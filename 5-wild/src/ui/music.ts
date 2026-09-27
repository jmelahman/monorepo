/**
 * The soundtrack: one recorded piano loop, and a synthesized score for a
 * browser that cannot decode it.
 *
 * The recording is Kistol's "Forget-me-not in F major" (CC0, see
 * `tracks/CREDITS.md`). Lofi was auditioned first and turned down flat; what
 * the game wanted was nearer Debussy, Brahms or a Chopin nocturne. Public-domain
 * recordings of those exist, but a nocturne played end to end has a final
 * cadence and a silence at every repeat, and this piece was written in that
 * style as a seamless loop, so it has neither. It costs 1.3MB, against a few
 * kilobytes for a clack, which is why it is the only track: the moods no longer
 * change the music, and the synthesis that used to tell the shop from the boss
 * is now only the fallback for a browser that cannot decode Ogg.
 *
 * It was the stand-in during the load as well, playing from the first tap and
 * crossing into the piano once it decoded. On a warm cache that is a fraction of
 * a second, and a fraction of a second of a different instrument in a different
 * key reads as a glitch rather than as a stand-in. So the load is silence now,
 * which is also what a player hears in every other game before the music starts.
 *
 * Notes are scheduled ahead on the audio clock rather than fired from a timer,
 * because `setInterval` drifts by tens of milliseconds under load and a rhythm
 * built on it audibly stumbles every time the scoring animation runs. The timer
 * only decides *what* to queue; the audio clock decides when it sounds.
 */

import { audioContext, audioMix, C5, step } from "./audio"
import TRACK_URL from "./tracks/forget-me-not.ogg?url"

const MUSIC_KEY = "5wild:music"

/**
 * The recording's gain. It is peak-normalized to -1dBFS like the effects, but
 * it is a bed and they are events, so it sits below `DEFAULT_LEVEL` in
 * `audio.ts`: a tile reveal should land on top of the piano, not inside it.
 */
const TRACK_LEVEL = 0.1

/** Fade in and out of the recording, in seconds. */
const TRACK_FADE_S = 1.5

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
  /** Playing rather than suspended. Only meaningful once `started`. */
  private live = false
  private timer: number | null = null
  private bus: GainNode | null = null
  /** The recording's own gain, straight into the mix and not into the room. */
  private deck: GainNode | null = null
  private track: AudioBuffer | null = null
  private loading = false
  /** The recording would not fetch or decode, so the score plays instead, for good. */
  private failed = false
  private source: AudioBufferSourceNode | null = null
  /** Audio-clock time the track would have started had it never been paused. */
  private startedAt = 0
  /** Seconds into the track, kept across a suspend so it picks up where it left. */
  private offset = 0
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
    // On unless the player has turned it off. It was off by default, on the
    // argument that a game which starts singing on its own is a game opened on
    // a bus with the volume up, and it lost two ways. The effects were on by
    // default all along, so the bus heard the game regardless; and a soundtrack
    // behind a switch in the pause sheet was one most players never learned
    // existed. Nothing plays before the first tap in any case, since no audio
    // may, and the piano fades in under the effects rather than starting on
    // them. The stored preference still wins both ways.
    this.off = stored === "0"
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
    const mix = audioMix()
    if (!ctx || !mix) return
    void ctx.resume()
    this.started = true
    this.bus = ctx.createGain()
    this.bus.gain.setValueAtTime(0, ctx.currentTime)
    // Through the same limiter as the effects, so the two cannot sum into a
    // clip, and a little into the same room, which is most of what stops a
    // sparse sine melody sounding like a test tone.
    this.bus.connect(mix.out)
    const send = ctx.createGain()
    send.gain.value = 0.35
    this.bus.connect(send).connect(mix.room)
    // The recording skips the room: it was made in one, and a second on top
    // of it turned the sustain pedal into a wash.
    this.deck = ctx.createGain()
    this.deck.gain.setValueAtTime(0, ctx.currentTime)
    this.deck.connect(mix.out)
    this.load(ctx)
    this.start(ctx)
  }

  set(mood: Mood): void {
    if (mood === this.mood) return
    this.mood = mood
    const ctx = audioContext()
    // The recording carries straight on across a change of screen. Restarting
    // it would put the opening bars on every round, and there is no other
    // track to change to.
    if (!this.bus || !ctx || this.source) return
    // Ride the gain across the change rather than cutting: the palettes differ
    // in tempo, so a hard switch lands mid-beat and reads as a glitch.
    this.bus.gain.cancelScheduledValues(ctx.currentTime)
    this.bus.gain.setValueAtTime(this.bus.gain.value, ctx.currentTime)
    this.bus.gain.linearRampToValueAtTime(0.0001, ctx.currentTime + 0.25)
    this.bus.gain.linearRampToValueAtTime(PALETTES[mood].gain, ctx.currentTime + 1)
  }

  /** Music alone; the effects keep their own switch. */
  setOff(off: boolean): void {
    if (off === this.off) return
    this.off = off
    try {
      localStorage.setItem(MUSIC_KEY, this.off ? "0" : "1")
    } catch {
      // See above. The switch works, it just will not be remembered.
    }
    if (this.off) this.stop()
    else this.enable()
  }

  /** Silence without forgetting the preference, for muting and for tab-away. */
  suspend(): void {
    this.live = false
    if (this.timer !== null) {
      clearInterval(this.timer)
      this.timer = null
    }
    const ctx = audioContext()
    if (!ctx) return
    if (this.bus) {
      this.bus.gain.cancelScheduledValues(ctx.currentTime)
      this.bus.gain.setValueAtTime(0.0001, ctx.currentTime)
    }
    // Stopped rather than faded to nothing: a looping source left running
    // under a zero gain would keep the context busy on a phone in a pocket.
    if (this.source && this.track) {
      this.offset = (ctx.currentTime - this.startedAt) % this.track.duration
      this.source.stop()
      this.source.disconnect()
      this.source = null
    }
    if (this.deck) {
      this.deck.gain.cancelScheduledValues(ctx.currentTime)
      this.deck.gain.setValueAtTime(0, ctx.currentTime)
    }
  }

  resume(): void {
    if (this.off || !this.started || this.live) return
    const ctx = audioContext()
    if (ctx) this.start(ctx)
  }

  private stop(): void {
    this.suspend()
    this.started = false
    this.bus?.disconnect()
    this.bus = null
    this.deck?.disconnect()
    this.deck = null
  }

  /** Play whichever the music currently is: the recording if it is ready, else the score. */
  private start(ctx: AudioContext): void {
    if (!this.bus) return
    this.live = true
    if (this.track) {
      this.play(ctx, this.track)
      return
    }
    // Still loading: live, and silent until `load` has something to play.
    if (!this.failed) return
    this.bus.gain.cancelScheduledValues(ctx.currentTime)
    this.bus.gain.setValueAtTime(this.bus.gain.value, ctx.currentTime)
    this.bus.gain.linearRampToValueAtTime(PALETTES[this.mood].gain, ctx.currentTime + 1.5)
    this.nextAt = Math.max(this.nextAt, ctx.currentTime + 0.1)
    this.run()
  }

  /**
   * Fetch and decode the recording, once. A failure hands the music to the
   * synthesized score for good, which is a plainer game and never a silent one.
   */
  private load(ctx: AudioContext): void {
    if (this.loading) return
    this.loading = true
    void fetch(TRACK_URL)
      .then((response) => response.arrayBuffer())
      .then((bytes) => ctx.decodeAudioData(bytes))
      .then((buffer) => {
        this.track = buffer
        if (this.live) this.play(ctx, buffer)
      })
      .catch(() => {
        this.failed = true
        // Suspended, `resume` will find the flag and start the score itself.
        if (this.live) {
          this.live = false
          this.start(ctx)
        }
      })
  }

  /**
   * `loop` on the source rather than a restart on `ended`: the file was
   * authored to run its last sample into its first, and the source is the
   * only thing that can join them without a gap. The re-encode kept the
   * sample count exact for the same reason.
   */
  private play(ctx: AudioContext, track: AudioBuffer): void {
    if (!this.deck || this.source) return
    const source = ctx.createBufferSource()
    source.buffer = track
    source.loop = true
    source.connect(this.deck)
    source.start(ctx.currentTime, this.offset)
    this.startedAt = ctx.currentTime - this.offset
    this.source = source
    this.deck.gain.cancelScheduledValues(ctx.currentTime)
    this.deck.gain.setValueAtTime(0.0001, ctx.currentTime)
    this.deck.gain.linearRampToValueAtTime(TRACK_LEVEL, ctx.currentTime + TRACK_FADE_S)
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
