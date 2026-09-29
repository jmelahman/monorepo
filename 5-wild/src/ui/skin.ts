/**
 * The look of the game, as a player setting: four presets, on every layout.
 *
 * It used to be two settings. A theme (light or dark) was the phone's, and a
 * skin (Smoke, Classic, Tabletop) was the desktop table's, and the light theme
 * switched the table off altogether. That made a pair of questions with one
 * answer per screen shape, and left a phone player no way to ask for the
 * walnut. The two are one thing now, because light and dark are only two of the
 * looks: the phone's own board, in either tone, is Classic.
 *
 * - `smoke` is a dark room, one accent, pixel-edged panels and smoke drifting
 *   behind them. Its motion is not a second preset: a look that differs from
 *   another only in whether it moves would be a dial stop that asks the player
 *   nothing about taste, and `prefers-reduced-motion` already gives the same
 *   room drawn once.
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
 * Exactly one `skin-*` class is on the root, never none: the stylesheet has no
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

export type Skin = "smoke" | "tabletop" | "classic-dark" | "classic-light"

export type Tone = "light" | "dark"

export const SKINS: readonly Skin[] = ["smoke", "classic-dark", "classic-light", "tabletop"]

/** The dial's one move, round the four in the order they are listed. */
export const NEXT_SKIN: Record<Skin, Skin> = {
  smoke: "classic-dark",
  "classic-dark": "classic-light",
  "classic-light": "tabletop",
  tabletop: "smoke",
}

/**
 * Light or dark, per look: what `data-theme` says, what the browser chrome and
 * the Android status bar are painted as, and which launch window the system
 * shows. Smoke and Tabletop are dark rooms and say so, which is what keeps the
 * phone's light palette from ever being applied under them.
 */
export const TONE: Record<Skin, Tone> = {
  smoke: "dark",
  tabletop: "dark",
  "classic-dark": "dark",
  "classic-light": "light",
}

/**
 * A stored pick, or null for "never picked". Anything unrecognised is no pick
 * rather than a wrong one, which costs a player the default look and never a
 * broken page.
 */
export const readSkin = (raw: string | null): Skin | null =>
  SKINS.find((skin) => skin === raw) ?? null

/** A stored theme from before the merge, or null. Only `light` and `dark` ever existed. */
export const readLegacyTone = (raw: string | null): Tone | null =>
  raw === "light" || raw === "dark" ? raw : null

export type Standing = {
  /** What the player chose with the dial, or null if they never have. */
  picked: Skin | null
  /** What they chose with the old dial, which only counts while `picked` is null. */
  legacy: Tone | null
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
 *   and Smoke on the desktop, which is the table they had: the dark theme on a
 *   desktop *was* the table, and Smoke is what that meant.
 * - Nothing at all is where every install starts. On a phone that is Classic
 *   following the device's light or dark exactly as before, `matchMedia` and
 *   all; on the desktop it is the Smoke Room, whatever the device says, since the table
 *   was never light.
 *
 * Only "nothing at all" reads `prefersLight`, and none of the three is stored:
 * a default that was written down would stop following the window the moment it
 * was, and a player who has never touched the dial would find a window resize
 * had made a decision for them.
 */
export function resolveSkin({ picked, legacy, table, prefersLight }: Standing): Skin {
  if (picked) return picked
  if (legacy === "light") return "classic-light"
  if (legacy === "dark") return table ? "smoke" : "classic-dark"
  if (table) return "smoke"
  return prefersLight ? "classic-light" : "classic-dark"
}

export function loadLegacyTone(): Tone | null {
  try {
    return readLegacyTone(localStorage.getItem(LEGACY_THEME_KEY))
  } catch {
    return null
  }
}

export function loadSkin(): Skin | null {
  try {
    return readSkin(localStorage.getItem(KEY))
  } catch {
    return null
  }
}

export function saveSkin(skin: Skin): void {
  try {
    localStorage.setItem(KEY, skin)
  } catch {
    // The setting lasts the session instead of the install. Nothing else breaks.
  }
}

/** The root class a skin is written under. */
export const skinClass = (skin: Skin): string => `skin-${skin}`

/** The skin in force, read off the root, which is where the stylesheet reads it. */
export function currentSkin(): Skin {
  const classes = document.documentElement.classList
  return SKINS.find((skin) => classes.contains(skinClass(skin))) ?? "classic-dark"
}

/**
 * How much the skin animates. Three levels, because two looks of the four want
 * some motion and no two want the same amount:
 *
 * - `full` is the Smoke Room: the smoke, the reel-stop deal, the
 *   particles, shake, scene transitions. Everything `juiced()` gates.
 * - `tactile` is Tabletop: things that are handled. A tile is set down and
 *   settles, a key gives under a finger, a counter ticks. Nothing is thrown off
 *   them: no sparks, no shake, no light crossing a row. A wooden board game
 *   does not throw sparks, and the room it is drawn in holds still.
 * - `none` is Classic in either tone, the phone's own board.
 *
 * Every effect asks through `hasFx` or `hasTactile` rather than naming a skin,
 * so which look gets what is this table and nothing else.
 *
 * Only the *background* of `full` reaches the phone. The scenes are built
 * around the table's geometry (chip stacks under the rail's readout, a
 * placard in its relic tray), and `juiced()` and `tactile()` still ask
 * for the table as well, so a phone in Smoke has the look and the smoke and the
 * same tile flips as ever.
 */
export type Fx = "none" | "tactile" | "full"

const FX: Record<Skin, Fx> = {
  smoke: "full",
  tabletop: "tactile",
  "classic-dark": "none",
  "classic-light": "none",
}

export const fxLevel = (): Fx => FX[currentSkin()]

/** The full show: the background and everything `juiced()` gates. */
export const hasFx = (): boolean => fxLevel() === "full"

/** At least the handled-object motion (`full` includes it). */
export const hasTactile = (): boolean => fxLevel() !== "none"
