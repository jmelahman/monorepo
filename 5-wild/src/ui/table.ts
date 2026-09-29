import { Capacitor } from "@capacitor/core"

/**
 * Whether the game is drawn as a table or as a phone.
 *
 * There are two looks and they are two products, decided by the owner rather
 * than by taste: the phone keeps the clean, Wordle-like board in both themes,
 * and a desktop gets the game table, the felt, the chunky type and every effect
 * in `./fx`. Nothing about the table is allowed to reach a phone, and the way
 * that is guaranteed rather than hoped for is that every rule the table adds is
 * written under `:root.table` and every effect asks `isTable()` first, so a
 * phone runs exactly the stylesheet and the motion it ran before the table was
 * built.
 *
 * Three questions, all of which have to say yes:
 *
 * - Not the APK. Asked of Capacitor, which knows, rather than of the screen:
 *   a tablet in landscape is a wide screen and still the phone product.
 * - A landscape screen with room for a table. A query about the *screen*, which
 *   CLAUDE.md is right to trust, never one about the input: a desktop browser
 *   narrowed to a column is a phone-shaped window and gets the phone's layout,
 *   and the Tauri window, which opens wide, gets the table.
 * - The dark theme. The table is a dark room, and the light theme on a desktop
 *   stays what it is on a phone: the clean board, which costs nothing to keep
 *   and leaves a player who wants the quiet version one tap from it.
 *
 * The answer is a class on the root, like `.plain` and `.quiet`, so no view has
 * to know it exists; every dispatch rebuilds the screen from the same views
 * either way, and the stylesheet decides what they look like.
 */

/**
 * 60rem is 960px, which admits a 1366x768 laptop after the browser's own chrome
 * and turns away a portrait tablet; 34rem of height is the shortest window the
 * table layout still fits a six-row board and a keyboard into side by side.
 */
const ROOM = "(min-width: 60rem) and (min-height: 34rem) and (orientation: landscape)"

let watching = false

const hasRoom = (): boolean => typeof matchMedia === "function" && matchMedia(ROOM).matches

/** True when the table look is on. Effects read this at the moment they fire. */
export const isTable = (): boolean => document.documentElement.classList.contains("table")

/**
 * Recompute and apply. Called by the theme whenever it resolves, and by the
 * window whenever it crosses the room query, which are the only two things
 * that can change the answer mid-game.
 */
export function syncTable(): void {
  if (!watching && typeof matchMedia === "function") {
    watching = true
    matchMedia(ROOM).addEventListener("change", syncTable)
  }
  const root = document.documentElement
  const on = !Capacitor.isNativePlatform() && hasRoom() && root.dataset.theme === "dark"
  root.classList.toggle("table", on)
}
