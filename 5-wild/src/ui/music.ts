/**
 * The soundtrack: two recorded tracks, the player's pick of which, and a
 * synthesized score for a browser that cannot decode either.
 *
 * The recordings are "promises" by kate (https://kate.garden/) and Kistol's
 * "Forget-me-not in F major" (CC0); see `tracks/CREDITS.md`. Forget-me-not was
 * the only track first, a piano piece written as a seamless loop. "promises"
 * replaced it and then joined it instead, so the choice is a setting on the
 * pause sheet rather than an argument settled in this file. It is not a
 * seamless loop: it is a finished piece with a cadence at its end, so its loop
 * is an ending and a beginning, with the lead-in and tail silences trimmed to
 * keep the gap between them to a breath. The moods do not change the music
 * either way, and the synthesis that used to tell the shop from the boss is only
 * the fallback for a browser that cannot decode Ogg.
 *
 * It was the stand-in during the load as well, playing from the first tap and
 * crossing into the recording once it decoded. On a warm cache that is a fraction of
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
import FORGET_ME_NOT_URL from "./tracks/forget-me-not.ogg?url"
import PROMISES_URL from "./tracks/promises.ogg?url"

const MUSIC_KEY = "5wild:music"
const TRACK_KEY = "5wild:track"

export type TrackId = "promises" | "forget-me-not"

type Track = {
  id: TrackId
  /** A title, the same in every language, so it is here and not in the catalog. */
  title: string
  url: string
  /**
   * The recording's gain. Both are peak-normalized to -1dBFS like the effects,
   * but a track is a bed and they are events: a tile reveal should land on top
   * of the music, not inside it. Forget-me-not sat below `DEFAULT_LEVEL` in
   * `audio.ts` at 0.1. "promises" averages 1.7dB quieter at the same peak
   * (-22.7dB mean against -21.0), and at 0.1 it sat too far back; 0.14, about
   * 3dB up, was still under it by ear. 0.17 is about 4.6dB up and over the
   * effects' gain, but the effects are transients at that peak and this is a
   * bed well under it on average, and the shared limiter keeps the sum from
   * clipping. Per track, so switching does not make one of them jump.
   */
  level: number
}

/** In the order the switch steps through them. The first is the default. */
const TRACKS: readonly Track[] = [
  { id: "promises", title: "promises", url: PROMISES_URL, level: 0.17 },
  { id: "forget-me-not", title: "Forget-me-not", url: FORGET_ME_NOT_URL, level: 0.1 },
]

const DEFAULT_TRACK = TRACKS[0] as Track

function trackOf(id: string | null): Track {
  return TRACKS.find((track) => track.id === id) ?? DEFAULT_TRACK
}

/**
 * How long the outgoing track takes to fade when the player switches. Shorter
 * than `TRACK_FADE_S`, because the tap asked for the other one and the fade out
 * is only there so it does not end in a click.
 */
const SWITCH_FADE_S = 0.3

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
  private choice: Track
  private mood: Mood = "title"
  private started = false
  /** Playing rather than suspended. Only meaningful once `started`. */
  private live = false
  private timer: number | null = null
  private bus: GainNode | null = null
  /** The recording's own gain, straight into the mix and not into the room. */
  private deck: GainNode | null = null
  /**
   * The chosen recording, decoded. Only the one: four minutes of stereo PCM is
   * some eighty megabytes, and the other is a fetch from the HTTP cache away.
   */
  private track: AudioBuffer | null = null
  /** Which recording is being fetched or is in `track`, so a stale decode can be told. */
  private loading: TrackId | null = null
  /** The recording would not fetch or decode, so the score plays instead, until a switch. */
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
    let chosen: string | null = null
    try {
      stored = localStorage.getItem(MUSIC_KEY)
      chosen = localStorage.getItem(TRACK_KEY)
    } catch {
      // A blocked store just means the preference does not survive the session.
    }
    this.choice = trackOf(chosen)
    // On unless the player has turned it off. It was off by default, on the
    // argument that a game which starts singing on its own is a game opened on
    // a bus with the volume up, and it lost two ways. The effects were on by
    // default all along, so the bus heard the game regardless; and a soundtrack
    // behind a switch in the pause sheet was one most players never learned
    // existed. Nothing plays before the first tap in any case, since no audio
    // may, and the recording fades in under the effects rather than starting on
    // them. The stored preference still wins both ways.
    this.off = stored === "0"
  }

  get isOff(): boolean {
    return this.off
  }

  get title(): string {
    return this.choice.title
  }

  /**
   * Step to the next track, from its top. Written even while the music is off,
   * so the switch can be set first and heard when the music comes back.
   */
  nextTrack(): void {
    const at = TRACKS.indexOf(this.choice)
    this.choice = TRACKS[(at + 1) % TRACKS.length] as Track
    try {
      localStorage.setItem(TRACK_KEY, this.choice.id)
    } catch {
      // See the constructor. The switch works, it just will not be remembered.
    }
    const ctx = audioContext()
    this.offset = 0
    this.track = null
    this.failed = false
    this.loading = null
    if (!ctx || !this.started) return
    if (this.source) {
      // Faded rather than cut, and let go of at once, so `play` can start the
      // next one over the tail of this one without waiting on it.
      const source = this.source
      this.source = null
      if (this.deck) {
        this.deck.gain.cancelScheduledValues(ctx.currentTime)
        this.deck.gain.setValueAtTime(this.deck.gain.value, ctx.currentTime)
        this.deck.gain.linearRampToValueAtTime(0.0001, ctx.currentTime + SWITCH_FADE_S)
      }
      source.stop(ctx.currentTime + SWITCH_FADE_S)
      source.onended = () => source.disconnect()
    }
    // The score, if the last choice had failed to it: silenced until the new
    // one has loaded or failed in its turn.
    if (this.timer !== null) {
      clearInterval(this.timer)
      this.timer = null
    }
    if (this.bus) {
      this.bus.gain.cancelScheduledValues(ctx.currentTime)
      this.bus.gain.setValueAtTime(0.0001, ctx.currentTime)
    }
    this.load(ctx)
  }

  /**
   * Tried once at boot, so a reload does not sit silent until the first tap.
   * Most of the time it is refused and `enable` from the first gesture is what
   * starts the music, but not always: the APK's WebView has the gesture
   * requirement switched off (Capacitor's `Bridge` does it), and Chrome waives
   * it for a site the player keeps coming back to. There is no asking Chrome
   * which, so it is tried; a refused context sits suspended with its clock
   * stopped, and everything here schedules against that clock, so the track
   * waits at its first sample and the score writes nothing ahead, until the
   * gesture resumes it.
   *
   * Firefox can say, and by default says no, so there it is not tried at all:
   * the attempt would buy nothing but a console warning.
   */
  autostart(): void {
    const policy = (
      navigator as { getAutoplayPolicy?: (kind: "audiocontext") => string }
    ).getAutoplayPolicy?.("audiocontext")
    if (policy === "disallowed") return
    this.enable()
  }

  /**
   * Called from the first real user gesture, if `autostart` has not already
   * built everything. The gesture's own job is `resume`, in the handler, which
   * is the only place iOS Safari will honour it.
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
    // The recording skips the room: it was mixed with its own, and a second on
    // top of it turned the previous track's sustain pedal into a wash.
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
    // it would put the opening bars on every round, and which track plays is
    // the player's choice, not the screen's.
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
   * Fetch and decode the chosen recording, once per choice. A failure hands the
   * music to the synthesized score until the player picks the other track,
   * which is a plainer game and never a silent one.
   *
   * A switch can land while a decode is in flight, and the decode that finishes
   * then is for a track nobody wants any more; `loading` is checked on the way
   * back so it is dropped rather than played over the new one.
   */
  private load(ctx: AudioContext): void {
    const want = this.choice.id
    if (this.loading === want) return
    this.loading = want
    void fetch(this.choice.url)
      .then((response) => response.arrayBuffer())
      .then((bytes) => ctx.decodeAudioData(bytes))
      .then((buffer) => {
        if (this.loading !== want) return
        this.track = buffer
        if (this.live) this.play(ctx, buffer)
      })
      .catch(() => {
        if (this.loading !== want) return
        this.failed = true
        // Suspended, `resume` will find the flag and start the score itself.
        if (this.live) {
          this.live = false
          this.start(ctx)
        }
      })
  }

  /**
   * `loop` on the source rather than a restart on `ended`: the source is the
   * only thing that can join the last sample to the first without a gap of
   * its own. Forget-me-not was authored to run into itself. "promises" is not
   * a seamless loop, but the silence at either end was trimmed in the
   * re-encode, so the only pause at its join is the one the piece's ending
   * leaves.
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
    this.deck.gain.linearRampToValueAtTime(this.choice.level, ctx.currentTime + TRACK_FADE_S)
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
