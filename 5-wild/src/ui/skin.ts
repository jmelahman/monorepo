/**
 * The desktop table's skin, as a player setting.
 *
 * The table is one layout and three dressings. The layout (where the rail, the
 * board and the keyboard sit, how big they are) is written once under
 * `:root.table`; the look (colours, type, how a button is drawn, what the room
 * behind it is) is written once per skin under `:root.table.skin-<name>`, so a
 * new skin is a directory of rules that restate no geometry. That is the split
 * `styles/table/index.css` describes, and this is its switch.
 *
 * - `smoke` is the default: a dark room, one accent, pixel-edged panels and a
 *   slow smoke behind them. The only skin with a moving background, and so the
 *   only one "Table lights" means anything to.
 * - `classic` is the phone's own look laid out on the table: no rule of the
 *   table's look applies to it, so the phone's partials show through. It is
 *   also the one skin with no effects at all (see `hasFx`), which is the point
 *   of it: a player who wants the desktop's room to hold still gets the board
 *   the phone has, at desktop size.
 * - `tabletop` is a board game's walnut, bone and terracotta.
 *
 * Meaningful only under `.table`: on a phone or the light theme the class is
 * on the root all the same and nothing is written under it, which is what
 * keeps the setting from needing to know whether it currently applies. It is
 * applied before the first render for the reason the theme is, so nobody sees
 * one skin flash into another, and it is always exactly one of the three
 * classes, never none: the stylesheet has no "no skin" branch to keep in step.
 *
 * The key is absent until a pick, and absent, unknown or a value from a build
 * that had other names all read as `smoke`, so a stale value costs a player the
 * default look and never a broken page.
 */

const KEY = "5wild:skin"

export type Skin = "smoke" | "classic" | "tabletop"

export const SKINS: readonly Skin[] = ["smoke", "classic", "tabletop"]

export const DEFAULT_SKIN: Skin = "smoke"

/** The row's one move, round the three in the order they are listed. */
export const NEXT_SKIN: Record<Skin, Skin> = {
  smoke: "classic",
  classic: "tabletop",
  tabletop: "smoke",
}

export const readSkin = (raw: string | null): Skin =>
  SKINS.find((skin) => skin === raw) ?? DEFAULT_SKIN

export function loadSkin(): Skin {
  try {
    return readSkin(localStorage.getItem(KEY))
  } catch {
    return DEFAULT_SKIN
  }
}

/** The root class a skin is written under. */
export const skinClass = (skin: Skin): string => `skin-${skin}`

/** The skin in force, read off the root, which is where the stylesheet reads it. */
export function currentSkin(): Skin {
  const classes = document.documentElement.classList
  return SKINS.find((skin) => classes.contains(skinClass(skin))) ?? DEFAULT_SKIN
}

/**
 * Whether the skin runs the table's effects: the background, the marquee and
 * everything `juiced()` gates. Classic does not, by definition, and every
 * effect asks through here rather than naming a skin, so the next skin that
 * wants stillness is one line in this file.
 */
export const hasFx = (): boolean => currentSkin() !== "classic"

/**
 * Apply it, and remember it if asked. The shell applies without writing, as a
 * first launch has chosen nothing and the key should stay absent until
 * something is.
 */
export function setSkin(skin: Skin, remember = true): void {
  const classes = document.documentElement.classList
  for (const other of SKINS) classes.toggle(skinClass(other), other === skin)
  if (!remember) return
  try {
    localStorage.setItem(KEY, skin)
  } catch {
    // The setting lasts the session instead of the install. Nothing else breaks.
  }
}
