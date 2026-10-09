/**
 * The soundtrack: two recorded tracks and the player's pick of which.
 *
 * The recordings are "promises" by kate (https://kate.garden/) and Kistol's
 * "Forget-me-not in F major" (CC0); see `tracks/CREDITS.md`. Forget-me-not was
 * the only track first, a piano piece written as a seamless loop. "promises"
 * replaced it and then joined it instead, so the choice is a setting on the
 * pause sheet rather than an argument settled in this file. It is not a
 * seamless loop: it is a finished piece with a cadence at its end, so its loop
 * is an ending and a beginning, with the lead-in and tail silences trimmed to
 * keep the gap between them to a breath. The screen does not change the music:
 * the round, the boss and the shop all hear the one track, carried straight on.
 *
 * There used to be a synthesized score as well, a generated melody per screen
 * (minor pentatonic for the run, major for the shop, a reedier wave and a
 * semitone-heavy change for the boss), scheduled ahead on the audio clock. It
 * was the whole soundtrack once, then the stand-in during the load, then only
 * the fallback for a browser that could not decode Ogg. It was cut from the
 * load first, because a fraction of a second of a different instrument in a
 * different key before the recording came in read as a glitch rather than as
 * a stand-in, and then cut altogether: the one browser left that needed it was
 * a Safari too old to decode Vorbis, and a player on one hears silence there
 * now, as they would in any game whose music did not load, rather than a
 * second soundtrack nobody else hears. The same went for the effects'
 * fallback; see the header of `audio.ts`.
 */

import { audioContext, audioMix } from "./audio"
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
  { id: "forget-me-not", title: "Forget Me Not", url: FORGET_ME_NOT_URL, level: 0.1 },
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

export class Music {
  private off: boolean
  private choice: Track
  private started = false
  /** Playing rather than suspended. Only meaningful once `started`. */
  private live = false
  /** The recording's own gain, straight into the mix and not into the room. */
  private deck: GainNode | null = null
  /**
   * The chosen recording, decoded. Only the one: four minutes of stereo PCM is
   * some eighty megabytes, and the other is a fetch from the HTTP cache away.
   */
  private track: AudioBuffer | null = null
  /** Which recording is being fetched or is in `track`, so a stale decode can be told. */
  private loading: TrackId | null = null
  private source: AudioBufferSourceNode | null = null
  /** Audio-clock time the track would have started had it never been paused. */
  private startedAt = 0
  /** Seconds into the track, kept across a suspend so it picks up where it left. */
  private offset = 0

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
    this.load(ctx)
  }

  /**
   * Tried once at boot, so a reload does not sit silent until the first tap.
   * Most of the time it is refused and `enable` from the first gesture is what
   * starts the music, but not always: the APK's WebView has the gesture
   * requirement switched off (Capacitor's `Bridge` does it), and Chrome waives
   * it for a site the player keeps coming back to. There is no asking Chrome
   * which, so it is tried; a refused context sits suspended with its clock
   * stopped, and the track is started against that clock, so it waits at its
   * first sample until the gesture resumes it.
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
    // Through the same limiter as the effects, so the two cannot sum into a
    // clip, and not into the room: the recording was mixed with its own, and a
    // second on top of it turned the previous track's sustain pedal into a wash.
    this.deck = ctx.createGain()
    this.deck.gain.setValueAtTime(0, ctx.currentTime)
    this.deck.connect(mix.out)
    this.load(ctx)
    this.start(ctx)
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
    const ctx = audioContext()
    if (!ctx) return
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
    this.deck?.disconnect()
    this.deck = null
  }

  /** Play the recording if it has decoded; if not, `load` plays it when it has. */
  private start(ctx: AudioContext): void {
    if (!this.deck) return
    this.live = true
    if (this.track) this.play(ctx, this.track)
  }

  /**
   * Fetch and decode the chosen recording, once per choice. A failure is
   * silence until the player picks the other track, which gets its own try.
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
      .catch(() => undefined)
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
}
