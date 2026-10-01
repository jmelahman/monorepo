import { Capacitor } from "@capacitor/core"

/**
 * Whether the game is laid out as a table or as a phone.
 *
 * This is a question about geometry and only geometry. It used to be a question
 * about the look as well (the table was a dark room, and the light theme
 * switched it off), and that is what the skin setting took over: any look can
 * now be drawn on either layout, and Classic on the table's layout is what a
 * desktop player who wants the phone's board gets. So `.table` means where the
 * rail, the board and the keyboard sit, and `src/ui/skin.ts` means what they
 * are made of.
 *
 * The other half of the old promise still holds and is what keeps a phone
 * safe: every layout rule the table adds is written under `:root.table`, and
 * every scene built around that layout asks `isTable()` first, so a phone
 * runs the layout and the motion it always ran, in whatever look it is in.
 *
 * Two questions, both of which have to say yes:
 *
 * - Not the APK. Asked of Capacitor, which knows, rather than of the screen:
 *   a tablet in landscape is a wide screen and still the phone product.
 * - A landscape screen with room for a table. A query about the *screen*, which
 *   CLAUDE.md is right to trust, never one about the input: a desktop browser
 *   narrowed to a column is a phone-shaped window and gets the phone's layout,
 *   and the Tauri window, which opens wide, gets the table.
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

const hasRoom = (): boolean => typeof matchMedia === "function" && matchMedia(ROOM).matches

/** True when the table layout is on. Effects read this at the moment they fire. */
export const isTable = (): boolean => document.documentElement.classList.contains("table")

/** Recompute and apply. */
export function syncTable(): void {
  document.documentElement.classList.toggle("table", !Capacitor.isNativePlatform() && hasRoom())
}

/**
 * Call back when the window crosses the room query, which is the one thing that
 * can change the answer mid-game. The look is told rather than left to notice,
 * because a legacy dark pick still maps differently on the table (see
 * `resolveSkin`), so the layout has to have landed before the look is asked.
 */
export function watchTable(onChange: () => void): void {
  if (typeof matchMedia !== "function") return
  matchMedia(ROOM).addEventListener("change", () => {
    syncTable()
    onChange()
  })
}
