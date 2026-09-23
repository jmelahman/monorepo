/**
 * Sound, synthesized rather than sampled.
 *
 * Every noise in the game is a few oscillator cycles built on demand, so the
 * bundle ships no audio assets, nothing is fetched at runtime, and the whole
 * thing survives airplane mode, the same constraint that made the word lists
 * bundled rather than served.
 *
 * The AudioContext is created lazily on the first sound, never at import: a
 * context constructed before a user gesture starts life suspended, and browsers
 * count that against the page.
 */

const MUTE_KEY = "5wild:muted"

let shared: AudioContext | null = null
let refused = false

/**
 * The one AudioContext, shared by the effects and the music.
 *
 * Mobile browsers cap how many a page may hold and count a suspended one
 * against it, so this stays lazy: the first sound builds it, and a page that
 * never makes a noise never has one.
 */
export function audioContext(): AudioContext | null {
  if (shared || refused) return shared
  const Ctor =
    window.AudioContext ??
    (window as { webkitAudioContext?: typeof AudioContext }).webkitAudioContext
  if (!Ctor) {
    refused = true
    return null
  }
  try {
    shared = new Ctor()
  } catch {
    // No audio is a degraded experience, not a broken game.
    refused = true
  }
  return shared
}

/** Semitone ratios off a root, so chords are written as intervals not numbers. */
export const step = (root: number, semitones: number) => root * 2 ** (semitones / 12)

export const C5 = 523.25

/** A single oscillator sweep. `to` bends the pitch; omit it for a flat tone. */
type Voice = {
  freq: number
  to?: number
  ms: number
  type?: OscillatorType
  gain?: number
  delay?: number
}

export class Sound {
  private muted = false

  constructor() {
    try {
      this.muted = localStorage.getItem(MUTE_KEY) === "1"
    } catch {
      // A blocked store just means the preference does not survive the session.
    }
  }

  get isMuted(): boolean {
    return this.muted
  }

  toggleMute(): boolean {
    this.muted = !this.muted
    try {
      localStorage.setItem(MUTE_KEY, this.muted ? "1" : "0")
    } catch {
      // See above. Muting still works, it just will not be remembered.
    }
    // Unmuting is itself a user gesture, so it is the cheapest moment to wake
    // a context that was created and then suspended.
    if (!this.muted) void audioContext()?.resume()
    return this.muted
  }

  private play(...voices: Voice[]): void {
    if (this.muted) return
    const ctx = audioContext()
    if (!ctx) return
    if (ctx.state === "suspended") void ctx.resume()

    for (const voice of voices) {
      const start = ctx.currentTime + (voice.delay ?? 0) / 1000
      const end = start + voice.ms / 1000
      const osc = ctx.createOscillator()
      const amp = ctx.createGain()

      osc.type = voice.type ?? "triangle"
      osc.frequency.setValueAtTime(voice.freq, start)
      if (voice.to !== undefined) osc.frequency.exponentialRampToValueAtTime(voice.to, end)

      // Attack fast, decay to silence: an abrupt stop on a running oscillator
      // is a click, and a click on every tile is what makes web audio grating.
      const peak = voice.gain ?? 0.09
      amp.gain.setValueAtTime(0.0001, start)
      amp.gain.exponentialRampToValueAtTime(peak, start + 0.008)
      amp.gain.exponentialRampToValueAtTime(0.0001, end)

      osc.connect(amp).connect(ctx.destination)
      osc.start(start)
      osc.stop(end + 0.02)
    }
  }

  /** Typing. Quiet and short, since it fires more than anything else in the game. */
  key(): void {
    this.play({ freq: 220, ms: 35, type: "square", gain: 0.025 })
  }

  /** Tiles resolve left to right, so the pitch climbs with the column. */
  tile(index: number, color: string): void {
    const root = color === "green" ? step(C5, 4) : color === "yellow" ? C5 : step(C5, -5)
    this.play({ freq: step(root, index), ms: 90, gain: 0.05 })
  }

  /** Relics get a brighter voice so they read as a different kind of event. */
  relic(): void {
    this.play({ freq: step(C5, 7), to: step(C5, 12), ms: 130, type: "sawtooth", gain: 0.045 })
  }

  /** The solve bonus is the biggest number on screen; it gets an arpeggio. */
  solve(): void {
    this.play(
      { freq: C5, ms: 90 },
      { freq: step(C5, 4), ms: 90, delay: 70 },
      { freq: step(C5, 7), ms: 90, delay: 140 },
      { freq: step(C5, 12), ms: 220, delay: 210 },
    )
  }

  /** Scored total. Pitched by how far past the target it landed. */
  score(ratio: number): void {
    const lift = Math.min(12, Math.round(ratio * 6))
    this.play({ freq: step(C5, lift), to: step(C5, lift + 5), ms: 180, gain: 0.06 })
  }

  coin(): void {
    this.play({ freq: step(C5, 12), ms: 60, gain: 0.05 }, { freq: step(C5, 19), ms: 90, delay: 55 })
  }

  win(): void {
    this.play(
      { freq: C5, ms: 320, gain: 0.05 },
      { freq: step(C5, 4), ms: 320, gain: 0.05, delay: 60 },
      { freq: step(C5, 7), ms: 380, gain: 0.05, delay: 120 },
    )
  }

  lose(): void {
    this.play({ freq: step(C5, -12), to: step(C5, -24), ms: 700, type: "sine", gain: 0.07 })
  }

  /** A letter broken off the keyboard: a loss, so it falls rather than rises. */
  break(): void {
    this.play({ freq: step(C5, -5), to: step(C5, -17), ms: 260, type: "sawtooth", gain: 0.05 })
  }
}
