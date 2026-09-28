/**
 * Sound effects: a list of named cues, each one a recording, bar three that
 * are synthesized.
 *
 * The game never names a noise, it names what happened (`tile`, `buy`,
 * `reject`) and hands over whatever the noise may want to know about it: which
 * column, what color, how many relics have already fired this guess. What that
 * sounds like is this file's business alone, which is what lets a cue be
 * re-voiced, or swapped for a sample, without `app.ts` hearing about it.
 *
 * **Recordings.** An audio file in `src/ui/sounds/` named after a cue voices
 * it, picked up by the glob below with no other change: `coin.ogg` voices
 * `coin`. A cue that has variants looks for the specific file first, so
 * `tile-green.ogg` beats `tile.ogg` for a green tile, and a cue with a pitch
 * (the tile's column, the relic ladder, the score) plays its recording that
 * many semitones up by playback rate, which is the classic way and the one
 * that keeps a chain of chip clacks climbing.
 *
 * **Synthesis.** `solve`, `break` and `consume` have no file, on purpose:
 * recordings were tried for each and lost to the synth (see their recipes).
 * What keeps them from sounding like bare oscillators through a volume fade is
 * cheap and all here: a filter on the tone so it has a shape rather than a
 * spectrum, a noise burst wherever a real object would click or thud, a shared
 * room so a chime rings instead of stopping, and a compressor on the way out.
 *
 * Every cue used to have a recipe, and the synth was the whole of the game's
 * sound before the recordings came, then their stand-in: until the files had
 * decoded, and for good on a file that would not. It was cut. The stand-in was
 * heard for a moment on a cold first load, as a different sound from the one
 * every later tap makes, and for good only on a Safari too old to decode
 * Vorbis, and two voicings of every cue was a lot of file to keep for that. A
 * recorded cue is silent until its file has decoded now, and a cue whose file
 * will not decode stays silent.
 *
 * What ships there now is CC0, mostly Kenney's packs and a few from Freesound,
 * chosen because public domain is the one licence with no quarrel with a
 * public GPL repo. `CREDITS.md` beside the files says which is which. They
 * were trimmed of leading silence, peak-normalized to -1dBFS and encoded as
 * Vorbis at q5; a replacement should be treated the same, or its entry in
 * `LEVELS` will not mean what the others do.
 *
 * The AudioContext is created lazily, never at import. The first thing to ask
 * for it is the music trying to start at boot (see `autostart`), which in most
 * browsers gets a suspended context that the first gesture then resumes.
 */

const MUTE_KEY = "5wild:muted"

let shared: AudioContext | null = null
let refused = false

/** The mix: everything ends in `out`, and `room` is the reverb send. */
type Mix = { out: AudioNode; room: AudioNode }
let mix: Mix | null = null

/**
 * The one AudioContext, shared by the effects and the music.
 *
 * Mobile browsers cap how many a page may hold and count a suspended one
 * against it, so this stays lazy and single: the music's boot attempt or the
 * first sound builds it, and a page with the music off that never makes a
 * noise never has one.
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

/**
 * The master bus and the room, built once beside the context.
 *
 * The compressor is a safety net rather than a sound: the threshold sits above
 * where any single cue peaks, so what it catches is a pile-up, the solve
 * arpeggio landing on the score riser landing on the music. Before it, those
 * summed straight into the speaker and a big guess crackled on a phone.
 *
 * The room is a convolver fed a second and a half of decaying noise, generated
 * here rather than shipped as an impulse file. It is not a convincing hall and
 * does not need to be; it is what stops a bell from ending like a light switch.
 */
export function audioMix(): Mix | null {
  if (mix) return mix
  const ctx = audioContext()
  if (!ctx) return null

  const limiter = ctx.createDynamicsCompressor()
  limiter.threshold.value = -12
  limiter.knee.value = 10
  limiter.ratio.value = 6
  limiter.attack.value = 0.003
  limiter.release.value = 0.2
  limiter.connect(ctx.destination)

  const room = ctx.createConvolver()
  const seconds = 1.5
  const length = Math.floor(ctx.sampleRate * seconds)
  const impulse = ctx.createBuffer(2, length, ctx.sampleRate)
  for (let channel = 0; channel < 2; channel++) {
    const data = impulse.getChannelData(channel)
    for (let i = 0; i < length; i++) {
      data[i] = (Math.random() * 2 - 1) * (1 - i / length) ** 3
    }
  }
  room.buffer = impulse
  const roomLevel = ctx.createGain()
  roomLevel.gain.value = 0.6
  room.connect(roomLevel).connect(limiter)

  mix = { out: limiter, room }
  return mix
}

/** Semitone ratios off a root, so chords are written as intervals not numbers. */
const step = (root: number, semitones: number) => root * 2 ** (semitones / 12)

const C5 = 523.25

/**
 * Major pentatonic, climbed by index. The tiles and the relic ladder both walk
 * it, because a chromatic climb (what the tiles used to do) is five semitones
 * in a row and sounds like a siren warming up, where five pentatonic steps
 * cannot land on a wrong note whatever order they come in.
 */
const PENTATONIC = [0, 2, 4, 7, 9] as const
const ladder = (n: number) => 12 * Math.floor(n / 5) + (PENTATONIC[n % 5] ?? 0)

/**
 * The relic ladder's ceiling, in rungs. Two octaves up the pentatonic from C5
 * is an A6 at 1760Hz, which is bright; past it is shrill, and a player who
 * has built a twenty-relic chain has earned a climax, not a dog whistle. The
 * rest of the chain holds the top note.
 */
const LADDER_TOP = 9

export type TileColor = "green" | "yellow" | "gray"
export type Trigger = "relic" | "mod" | "category" | "grew"

/**
 * Everything the game can ask to hear.
 *
 * `n` on a trigger is how many triggers have already fired this guess, so a
 * cascade climbs rather than repeating one blip, which is the difference
 * between a chain that builds and one that stutters.
 */
export type Cue =
  | { name: "key" }
  | { name: "back" }
  | { name: "reject" }
  | { name: "tile"; index: number; color: TileColor }
  | { name: "trigger"; kind: Trigger; n: number }
  | { name: "solve" }
  | { name: "score"; ratio: number }
  | { name: "break" }
  | { name: "coin" }
  | { name: "buy" }
  | { name: "sell" }
  | { name: "reroll"; shop?: boolean }
  | { name: "pack" }
  | { name: "pick" }
  | { name: "place" }
  | { name: "consume" }
  | { name: "sheet"; open: boolean }
  | { name: "intro"; boss: boolean }
  | { name: "win"; run?: boolean }
  | { name: "lose" }

export type CueName = Cue["name"]

/** The more specific sample name a cue looks for before its plain one. */
function variant(cue: Cue): string | null {
  switch (cue.name) {
    case "tile":
      return cue.color
    case "trigger":
      return cue.kind
    case "sheet":
      return cue.open ? "open" : "close"
    case "intro":
      return cue.boss ? "boss" : "round"
    // The shop's arrival is a reroll that nobody asked for, a shelf dealt
    // out, so it is the reroll cue. The recording is its own, a door bell: it
    // borrowed the reroll's card fan first and that was heard as wrong, and
    // without `reroll-shop.ogg` it would borrow it again.
    case "reroll":
      return cue.shop ? "shop" : null
    // A round cleared and the run won are one cue with two sizes. The round is
    // heard dozens of times a run and wants to be small; the run is heard once
    // or never and has earned a fanfare.
    case "win":
      return cue.run ? "run" : null
    default:
      return null
  }
}

/** Semitones a sample is repitched by, so a run of the same recording climbs. */
function transpose(cue: Cue): number {
  switch (cue.name) {
    case "tile":
      return PENTATONIC[cue.index] ?? 0
    case "trigger":
      return ladder(Math.min(cue.n, LADDER_TOP))
    case "score":
      return Math.min(12, Math.round(cue.ratio * 6))
    default:
      return 0
  }
}

/* ---------------------------------------------------------------- samples */

/**
 * Every audio file beside this module, by bare name. Eager and URL-only, so a
 * file costs a hashed path in the bundle and nothing more until it is fetched.
 * An empty or missing directory is an empty record, which is the default.
 */
const SAMPLE_URLS: Record<string, string> = Object.fromEntries(
  Object.entries(
    import.meta.glob<string>("./sounds/*.{ogg,mp3,wav,m4a,opus,webm}", {
      eager: true,
      query: "?url",
      import: "default",
    }),
  ).map(([path, url]) => [path.replace(/^.*\/|\.[^.]+$/g, ""), url]),
)

/* ------------------------------------------------------------------ voices */

/** A pitched voice. `to` bends the pitch; `lp`/`lpTo` sweep a lowpass across it. */
type Tone = {
  freq: number
  to?: number
  ms: number
  type?: OscillatorType
  gain?: number
  delay?: number
  /** Attack time. Short enough to be a transient, long enough not to click. */
  attack?: number
  lp?: number
  lpTo?: number
  /** Share sent to the room. */
  wet?: number
}

/** A burst of filtered noise: the click, thud, scrape or whoosh of a thing. */
type Noise = {
  ms: number
  band: number
  bandTo?: number
  q?: number
  kind?: BiquadFilterType
  gain?: number
  delay?: number
  attack?: number
  wet?: number
}

class Synth {
  private noiseBuffer: AudioBuffer | null = null

  constructor(
    private readonly ctx: AudioContext,
    private readonly mix: Mix,
  ) {}

  tone(voice: Tone): void {
    const { ctx } = this
    const start = ctx.currentTime + (voice.delay ?? 0) / 1000
    const end = start + voice.ms / 1000
    const amp = this.envelope(start, end, voice.gain ?? 0.05, voice.attack ?? 5, voice.wet)

    let into: AudioNode = amp
    if (voice.lp !== undefined) {
      const filter = ctx.createBiquadFilter()
      filter.type = "lowpass"
      filter.Q.value = 0.9
      filter.frequency.setValueAtTime(voice.lp, start)
      if (voice.lpTo !== undefined) filter.frequency.exponentialRampToValueAtTime(voice.lpTo, end)
      filter.connect(amp)
      into = filter
    }

    const osc = ctx.createOscillator()
    osc.type = voice.type ?? "triangle"
    osc.frequency.setValueAtTime(voice.freq, start)
    if (voice.to !== undefined) osc.frequency.exponentialRampToValueAtTime(voice.to, end)
    osc.connect(into)
    osc.start(start)
    osc.stop(end + 0.02)
  }

  noise(voice: Noise): void {
    const { ctx } = this
    const start = ctx.currentTime + (voice.delay ?? 0) / 1000
    const end = start + voice.ms / 1000
    const amp = this.envelope(start, end, voice.gain ?? 0.03, voice.attack ?? 2, voice.wet)

    const filter = ctx.createBiquadFilter()
    filter.type = voice.kind ?? "bandpass"
    filter.Q.value = voice.q ?? 1
    filter.frequency.setValueAtTime(voice.band, start)
    if (voice.bandTo !== undefined) filter.frequency.exponentialRampToValueAtTime(voice.bandTo, end)

    const source = ctx.createBufferSource()
    source.buffer = this.whiteNoise()
    source.loop = true
    source.connect(filter).connect(amp)
    // A random offset into the same second of noise, so two bursts in a row are
    // not the same burst twice.
    source.start(start, Math.random())
    source.stop(end + 0.02)
  }

  /**
   * Attack fast, decay to silence: an abrupt stop on a running oscillator is a
   * click, and a click on every tile is what makes web audio grating.
   */
  private envelope(
    start: number,
    end: number,
    peak: number,
    attackMs: number,
    wet: number | undefined,
  ): GainNode {
    const amp = this.ctx.createGain()
    amp.gain.setValueAtTime(0.0001, start)
    amp.gain.exponentialRampToValueAtTime(peak, start + attackMs / 1000)
    amp.gain.exponentialRampToValueAtTime(0.0001, end)
    amp.connect(this.mix.out)
    if (wet) {
      const send = this.ctx.createGain()
      send.gain.value = wet
      amp.connect(send).connect(this.mix.room)
    }
    return amp
  }

  private whiteNoise(): AudioBuffer {
    if (this.noiseBuffer) return this.noiseBuffer
    const buffer = this.ctx.createBuffer(1, this.ctx.sampleRate, this.ctx.sampleRate)
    const data = buffer.getChannelData(0)
    for (let i = 0; i < data.length; i++) data[i] = Math.random() * 2 - 1
    this.noiseBuffer = buffer
    return buffer
  }

  /**
   * A struck bell: the fundamental plus the inharmonic partial at 2.76× that is
   * the difference between a bell and a sine wave, which dies away first, as it
   * does on a real one.
   */
  bell(freq: number, ms: number, gain: number, delay = 0, wet = 0.3): void {
    this.tone({ freq, ms, type: "sine", gain, delay, wet })
    this.tone({ freq: freq * 2.76, ms: ms * 0.4, type: "sine", gain: gain * 0.3, delay, wet })
  }
}

/* ----------------------------------------------------------------- recipes */

type SynthName = "solve" | "break" | "consume"
type Recipe<K extends SynthName> = (s: Synth, cue: Extract<Cue, { name: K }>) => void

/**
 * One recipe per synthesized cue. Every other cue is its recording or nothing:
 * a cue with neither is silent, which is what a new cue is until it has a file.
 */
const RECIPES: { [K in SynthName]: Recipe<K> } = {
  // The biggest number in the game gets the biggest sound: a bell arpeggio
  // left ringing in the room, with a shimmer over the top as it lands.
  //
  // Kept on the synth when everything round it went to recordings. A steel
  // jingle was tried here and heard as a regression. The likeliest reason is
  // that a solve is always the end of a round and `win` follows it within the
  // cascade's last beat: two jingles from one pack back to back, and at ×3 on
  // top of each other.
  solve: (s) => {
    const notes = [0, 4, 7, 12, 16]
    for (const [i, interval] of notes.entries()) {
      const last = i === notes.length - 1
      s.bell(step(C5, interval), last ? 900 : 300, 0.05, i * 65, 0.45)
    }
    s.noise({
      ms: 500,
      band: 7000,
      kind: "highpass",
      gain: 0.012,
      delay: 260,
      attack: 80,
      wet: 0.5,
    })
  },

  // A crack and a fall: something lost, so it goes down and it goes dark.
  // Like solve, this ships as synth. A heavy pane of glass was recorded in its
  // place and heard as worse than this.
  break: (s) => {
    s.noise({ ms: 220, band: 2200, bandTo: 300, q: 0.7, gain: 0.06 })
    s.tone({
      freq: step(C5, -5),
      to: step(C5, -17),
      ms: 280,
      type: "sawtooth",
      gain: 0.04,
      lp: 1400,
      lpTo: 250,
    })
  },

  // Something used up and turned into an effect: a glide into the room.
  // Ships as synth, like solve and break. The recording it had was Kenney's
  // sci-fi `maximize_003`, the last of that pack left in the game.
  consume: (s) => {
    s.tone({ freq: C5, to: step(C5, 19), ms: 280, type: "sine", gain: 0.03, wet: 0.45 })
    s.noise({ ms: 260, band: 6000, bandTo: 9500, q: 0.8, gain: 0.012, wet: 0.4 })
  },
}

/* ------------------------------------------------------------------- sound */

/**
 * How loud each recording plays, by file name. The recordings arrive
 * peak-normalized to -1dBFS (see the header of this file), so this table is
 * the whole of the mix for them, and it is here rather than baked into the
 * files so that tuning it is an edit, not a re-encode.
 *
 * Pitched against the three synthesized stings, which peak around 0.05 per
 * voice, and against the soundtrack, which plays at 0.1 to 0.17 (see `TRACKS`
 * in `music.ts`). A recording left at full scale would bury the soundtrack
 * under every keystroke. Anything unlisted plays at `DEFAULT_LEVEL`.
 */
const LEVELS: Readonly<Record<string, number>> = {
  // Typed in runs of five, dozens of times a round, so it sits lowest but for
  // backspace. 0.07 was heard as a touch loud.
  key: 0.05,
  // The refusal's soft thud, a different take of it, well under it: undoing a
  // letter is the smallest gesture in the game and the refusal is the game
  // saying no, so they share a voice and not a volume. At 0.05 it was the right
  // sound and still too present for something pressed in runs. A chip set
  // down was tried in its place and lost to the thud.
  back: 0.035,
  reject: 0.12,
  tile: 0.12,
  trigger: 0.12,
  score: 0.14,
  solve: 0.15,
  break: 0.14,
  "sheet-open": 0.06,
  "sheet-close": 0.05,
  // A desk bell, and a whisper of one: it marks every round, and it is the
  // quiet half of the bell that opens a boss, so the gap between them is the
  // point. Chosen at 0.05 by ear.
  "intro-round": 0.05,
  "intro-boss": 0.18,
  // A shop door bell, held back (see `DISTANT`): the shop opening is news the
  // screen already carries, and a bell at the game's default level rang like
  // a notification rather than a door.
  "reroll-shop": 0.07,
  win: 0.16,
  lose: 0.16,
}
const DEFAULT_LEVEL = 0.12

/**
 * Recordings placed across the room rather than at the ear. Turning one down
 * only makes it quieter; what makes a sound read as far off is losing its top
 * end, which air and distance take first, and hearing more of the room than of
 * the thing. So `lp` is a low-pass cutoff in Hz and `room` the share sent to
 * the same reverb the synth rings in, which the recordings otherwise bypass.
 * Reverb alone, without the other two, makes a sound bigger, not further.
 */
const DISTANT: Readonly<Record<string, { lp: number; room: number }>> = {
  "reroll-shop": { lp: 2800, room: 0.5 },
}

export class Sound {
  private muted = false
  private synth: Synth | null = null
  private samples = new Map<string, AudioBuffer>()
  private loading = false

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

  setMuted(muted: boolean): void {
    if (muted === this.muted) return
    this.muted = muted
    try {
      localStorage.setItem(MUTE_KEY, this.muted ? "1" : "0")
    } catch {
      // See above. Muting still works, it just will not be remembered.
    }
    // Unmuting is itself a user gesture, so it is the cheapest moment to wake
    // a context that was created and then suspended.
    if (!this.muted) void audioContext()?.resume()
  }

  cue(cue: Cue): void {
    if (this.muted) return
    const ctx = audioContext()
    const out = audioMix()
    if (!ctx || !out) return
    if (ctx.state === "suspended") void ctx.resume()
    this.load(ctx)

    const tag = variant(cue)
    const specific = tag ? `${cue.name}-${tag}` : null
    const name = specific && this.samples.has(specific) ? specific : cue.name
    const sample = this.samples.get(name)
    if (sample) {
      const source = ctx.createBufferSource()
      const amp = ctx.createGain()
      source.buffer = sample
      source.playbackRate.value = 2 ** (transpose(cue) / 12)
      amp.gain.value = LEVELS[name] ?? DEFAULT_LEVEL
      const far = DISTANT[name]
      if (far) {
        const filter = ctx.createBiquadFilter()
        filter.type = "lowpass"
        filter.frequency.value = far.lp
        const send = ctx.createGain()
        send.gain.value = far.room
        source.connect(filter).connect(amp).connect(out.out)
        amp.connect(send).connect(out.room)
      } else {
        source.connect(amp).connect(out.out)
      }
      source.start()
      return
    }

    if (!(cue.name in RECIPES)) return
    this.synth ??= new Synth(ctx, out)
    // The mapped type guarantees each recipe takes its own cue; TypeScript
    // cannot follow that through an index by a union, hence the widening.
    ;(RECIPES[cue.name as SynthName] as Recipe<SynthName>)(this.synth, cue as never)
  }

  /**
   * Fetch and decode every sample, once, on the first sound. Failures are
   * silent and per file: a missing or undecodable recording leaves its cue
   * silent, which is a quieter game, never a broken one. That includes Ogg on
   * a Safari too old to decode it.
   */
  private load(ctx: AudioContext): void {
    if (this.loading) return
    this.loading = true
    for (const [name, url] of Object.entries(SAMPLE_URLS)) {
      void fetch(url)
        .then((response) => response.arrayBuffer())
        .then((bytes) => ctx.decodeAudioData(bytes))
        .then((buffer) => {
          this.samples.set(name, buffer)
        })
        .catch(() => undefined)
    }
  }
}
