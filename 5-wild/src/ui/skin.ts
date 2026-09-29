/**
 * The look of the game, as a player setting: five presets, on every layout.
 *
 * It used to be two settings. A theme (light or dark) was the phone's, and a
 * skin (Smoke, Classic, Tabletop) was the desktop table's, and the light theme
 * switched the table off altogether. That made a pair of questions with one
 * answer per screen shape, and left a phone player no way to ask for the
 * walnut. The two are one thing now, because light and dark are only two of the
 * looks: the phone's own board, in either tone, is Classic.
 *
 * - `smoke` is a dark room, one accent, pixel-edged panels and the smoke
 *   behind them drawn once, still. `smoke-moving` is the same room with the
 *   smoke drifting. They are one stylesheet: the second is a flag on the root
 *   (`moving-bg`, see `skinClass`), not a copy, so a change to the room is made
 *   once. Motion used to be its own setting, a row of its own and a key
 *   (`5wild:ambience`); it is a preset because it is a taste about the look and
 *   nothing else, and a second control for one look was a row every other look
 *   had to hide.
 * - `tabletop` is a board game's walnut, bone and terracotta, with a still
 *   room and only the motion of things being handled (see `Fx`).
 * - `classic-dark` and `classic-light` are the phone's two looks as they always
 *   were: no rule of any skin applies to them, so what shows is the phone's
 *   partials in the phone's tokens. Classic is also the look with no effects at
 *   all, which is the point of it.
 *
 * Layout is a different question and is not this one. Where the rail, the board
 * and the keyboard sit is `.table` on the root (`table.ts`, wide landscape
 * outside the APK), and every look is written to work on both: a look that
 * applies on any layout is under `:root.skin-<name>`, and only a rule that
 * depends on the table's geometry is under `:root.table.skin-<name>`.
 *
 * Exactly one `skin-*` class is on the root, never none (the two Smokes share
 * `skin-smoke`, and the moving one adds `moving-bg`): the stylesheet has no
 * "no skin" branch to keep in step. Its *tone* goes to `data-theme`, the one
 * attribute the phone's palette, the browser chrome and the Android launch
 * window already key on: `classic-light` is `light` and everything else is
 * `dark`, so none of them had to learn there are four looks.
 *
 * The key is absent until a pick, and until then the look is a default that
 * follows the window rather than a choice (see `resolveSkin`), so an install
 * that never touches the dial keeps behaving as the phone always has.
 */

const KEY = "5wild:skin"

/** The key the theme dial wrote before the two settings were one. Read, never written. */
const LEGACY_THEME_KEY = "5wild:theme"

/** The moving-background switch, retired into two presets. Read for migration; written only as `off` beside a still `smoke`. */
const AMBIENCE_KEY = "5wild:ambience"

export type Skin = "smoke" | "smoke-moving" | "tabletop" | "classic-dark" | "classic-light"

export type Tone = "light" | "dark"

export const SKINS: readonly Skin[] = [
  "smoke",
  "smoke-moving",
  "tabletop",
  "classic-dark",
  "classic-light",
]

/** The dial's one move, round the five in the order they are listed. */
export const NEXT_SKIN: Record<Skin, Skin> = {
  smoke: "smoke-moving",
  "smoke-moving": "tabletop",
  tabletop: "classic-dark",
  "classic-dark": "classic-light",
  "classic-light": "smoke",
}

/**
 * Light or dark, per look: what `data-theme` says, what the browser chrome and
 * the Android status bar are painted as, and which launch window the system
 * shows. Smoke and Tabletop are dark rooms and say so, which is what keeps the
 * phone's light palette from ever being applied under them.
 */
export const TONE: Record<Skin, Tone> = {
  smoke: "dark",
  "smoke-moving": "dark",
  tabletop: "dark",
  "classic-dark": "dark",
  "classic-light": "light",
}

/**
 * A stored pick, or null for "never picked". `classic` is the one name an
 * earlier build wrote that this one has no look called: it was the phone's
 * board on the table, and it was only ever offered on the dark table, so it is
 * Classic dark. Anything else unrecognised is no pick rather than a wrong one,
 * which costs a player the default look and never a broken page.
 */
export const readSkin = (raw: string | null): Skin | null =>
  raw === "classic" ? "classic-dark" : (SKINS.find((skin) => skin === raw) ?? null)

/** A stored theme from before the merge, or null. Only `light` and `dark` ever existed. */
export const readLegacyTone = (raw: string | null): Tone | null =>
  raw === "light" || raw === "dark" ? raw : null

export type Standing = {
  /** What the player chose with the dial, or null if they never have. */
  picked: Skin | null
  /** What they chose with the old dial, which only counts while `picked` is null. */
  legacy: Tone | null
  /** Whether the retired ambience switch was off, which only counts while `picked` is null. */
  still?: boolean
  /** Whether the table layout is on: the window, not the setting. */
  table: boolean
  /** What the device says, for the one case that asks it. */
  prefersLight: boolean
}

/**
 * The look on screen. A pick always wins. With none, three cases, in the order
 * the migration was specified:
 *
 * - An old `light` pick is Classic light everywhere, because the light theme was
 *   the clean board and a player who chose it chose that.
 * - An old `dark` pick is Classic dark on a phone, which is the board they had,
 *   and Smoke on the desktop, which is the table they had (moving unless they had
 *   switched the drift off): the dark theme on a
 *   desktop *was* the table, and Smoke is what that meant.
 * - Nothing at all is where every install starts. On a phone that is Classic
 *   following the device's light or dark exactly as before, `matchMedia` and
 *   all; on the desktop it is the moving Smoke, whatever the device says, since the table
 *   was never light.
 *
 * Only "nothing at all" reads `prefersLight`, and none of the three is stored:
 * a default that was written down would stop following the window the moment it
 * was, and a player who has never touched the dial would find a window resize
 * had made a decision for them.
 */
export function resolveSkin({ picked, legacy, still, table, prefersLight }: Standing): Skin {
  if (picked) return picked
  const room: Skin = still ? "smoke" : "smoke-moving"
  if (legacy === "light") return "classic-light"
  if (legacy === "dark") return table ? room : "classic-dark"
  if (table) return room
  return prefersLight ? "classic-light" : "classic-dark"
}

/**
 * What to store for a `smoke` saved before motion was a preset. Those players
 * were looking at the drifting room unless they had switched it off, so an
 * absent or `on` ambience makes it `smoke-moving` and `off` leaves it `smoke`,
 * which is now the still one. Anything else is not a smoke and passes through.
 */
export const migrateSmoke = (raw: string | null, ambience: string | null): string | null =>
  raw === "smoke" && ambience !== "off" ? "smoke-moving" : raw

/**
 * Whether the retired ambience key said off. Read for a player who never picked
 * a look and had turned the drift off on the default room.
 */
export function loadStill(): boolean {
  try {
    return localStorage.getItem(AMBIENCE_KEY) === "off"
  } catch {
    return false
  }
}

export function loadLegacyTone(): Tone | null {
  try {
    return readLegacyTone(localStorage.getItem(LEGACY_THEME_KEY))
  } catch {
    return null
  }
}

/**
 * The pick, migrated. The one place a read writes: a stored `smoke` from before
 * motion was a preset is rewritten as `smoke-moving` (unless the retired
 * ambience key says off), and its key removed. The key is what tells an old
 * `smoke` from a new one, since the two spell the same word: `saveSkin` writes
 * `off` beside a still `smoke` for exactly that, so a `smoke` with no `off`
 * beside it is always one an earlier build wrote. With no pick the key is left
 * where it is, for `loadStill`.
 */
export function loadSkin(): Skin | null {
  try {
    const raw = localStorage.getItem(KEY)
    const ambience = localStorage.getItem(AMBIENCE_KEY)
    const stored = migrateSmoke(raw, ambience)
    if (stored !== raw && stored !== null) {
      localStorage.setItem(KEY, stored)
      localStorage.removeItem(AMBIENCE_KEY)
    }
    return readSkin(stored)
  } catch {
    return null
  }
}

export function saveSkin(skin: Skin): void {
  try {
    localStorage.setItem(KEY, skin)
    // A pick settles what the retired ambience switch used to say, and the
    // still Smoke leaves the one word that says it (see `loadSkin`).
    if (skin === "smoke") localStorage.setItem(AMBIENCE_KEY, "off")
    else localStorage.removeItem(AMBIENCE_KEY)
  } catch {
    // The setting lasts the session instead of the install. Nothing else breaks.
  }
}

/** The root class a skin is written under. Both Smokes are the one stylesheet's. */
export const skinClass = (skin: Skin): string => `skin-${skin === "smoke-moving" ? "smoke" : skin}`

/** The flag the moving Smoke adds beside its skin class. */
export const MOVING_CLASS = "moving-bg"

/** The skin in force, read off the root, which is where the stylesheet reads it. */
export function currentSkin(): Skin {
  const classes = document.documentElement.classList
  const found = SKINS.find((skin) => skin !== "smoke-moving" && classes.contains(skinClass(skin)))
  if (found === "smoke" && classes.contains(MOVING_CLASS)) return "smoke-moving"
  return found ?? "classic-dark"
}

/**
 * How much the skin animates. Three levels, because two looks of the four want
 * some motion and no two want the same amount:
 *
 * - `full` is the Smoke Room: the smoke, the reel-stop deal, the payline, the
 *   jackpot, particles, shake, scene transitions. Everything `juiced()` gates.
 * - `tactile` is Tabletop: things that are handled. A tile is set down and
 *   settles, a key gives under a finger, a counter ticks. Nothing is thrown off
 *   them: no sparks, no shake, no light crossing a row. A wooden board game
 *   does not have a jackpot sign, and the room it is drawn in holds still.
 * - `none` is Classic in either tone, the phone's own board.
 *
 * Every effect asks through `hasFx` or `hasTactile` rather than naming a skin,
 * so which look gets what is this table and nothing else.
 *
 * Only the *background* of `full` reaches the phone (both Smokes, the still one as a single frame). The scenes are built
 * around the table's geometry (a payline across a rail-and-board layout, a
 * jackpot sign over its scorecard), and `juiced()` and `tactile()` still ask
 * for the table as well, so a phone in Smoke has the look and the smoke and the
 * same tile flips as ever.
 */
export type Fx = "none" | "tactile" | "full"

const FX: Record<Skin, Fx> = {
  smoke: "full",
  "smoke-moving": "full",
  tabletop: "tactile",
  "classic-dark": "none",
  "classic-light": "none",
}

export const fxLevel = (): Fx => FX[currentSkin()]

/** The full show: the background and everything `juiced()` gates. */
export const hasFx = (): boolean => fxLevel() === "full"

/** At least the handled-object motion (`full` includes it). */
export const hasTactile = (): boolean => fxLevel() !== "none"

/** Whether a look's background drifts. Only the moving Smoke; the still one is drawn once. */
export const hasMovingBackground = (skin: Skin): boolean => skin === "smoke-moving"

/** Whether a look draws the smoke at all, still or moving: the canvas exists for both. */
export const hasSmoke = (skin: Skin): boolean => skin === "smoke" || skin === "smoke-moving"
