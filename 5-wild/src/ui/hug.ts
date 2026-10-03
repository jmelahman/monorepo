/**
 * A wrapped name, held to the width of its own text.
 *
 * A block whose text wraps is as wide as the room it was offered, not as wide
 * as the lines it ended up with: "Compound Interest" beside its figure in a
 * 121px card (Classic, 1440x900) is laid out in an 80px box, breaks into two
 * lines the longer of which is 61px, and keeps the 80. The name is centred in
 * that box, so the spare width comes out on both sides of it, and the half on
 * the figure's side was 11.5px of gap where a one-line name has 2. Measured,
 * the gap is 2.1px, and within half a pixel of that in the other two looks.
 *
 * The stylesheet cannot ask for the other width. `min-content` is the longest
 * *word*, which puts a three-word name on three lines; `fit-content` is what
 * it already has; and nothing in CSS names "the widest line after wrapping".
 * So it is measured: a range over the name's text hands back one rectangle per
 * line box, and the span from the leftmost edge to the rightmost is the width
 * the text is using. Written back as the name's `width`, the same breaks still
 * fit, and the row's centring closes the gap.
 *
 * Only a name that wrapped. One line is already the width of its text, and on
 * a phone every name with a figure is one line (see `.relic-line` in
 * `relics.css`), so there it is measured and left alone.
 *
 * The width is in pixels and the room it was measured in is not: a seat is
 * `clamp(5.25rem, 8.4vw, 8rem)` wide, and a window can be dragged from 1100 to
 * 1600 without a render, since `watchTable` renders only across the layout's
 * line. Measured once at the render, "First Draft" was fitted to its two 28px
 * lines in an 84px seat and kept them in a 128px one, where the name and its
 * figure need 82 on one line. So nothing is measured at the render at all. Each
 * card is handed to a `ResizeObserver`, which reports once when it starts
 * watching and again whenever the seat changes size, both times after layout
 * and before paint, so the name is never seen at the width it was offered and
 * never keeps a width from a seat it is no longer in. Fitting the name cannot
 * set the observer off again: the seat's size is the tray's, not its text's.
 *
 * A face arriving is the one change the observer does not see, since it
 * resizes the ink and not the seat. The looks' fonts are `font-display: swap`,
 * so a first render can be measured in the fallback, and `hugNames` is the
 * same measurement asked for outright. See `render` and `bindFaces` in
 * `app.ts`.
 */

/** Made on first use, so importing this module asks nothing of a DOM. */
let seats: ResizeObserver | null = null

/** Keep every wrapped name under `root` fitted until the next call, which is the next render. */
export function watchNames(root: ParentNode): void {
  // The last render's cards are gone from the document, and whether an
  // observer still holding them keeps them from being collected is each
  // engine's own business, with three engines here. Let go of all of them
  // rather than find out: every render rebuilds the tray, so the cards below
  // are the only ones there are.
  seats?.disconnect()
  for (const name of names(root)) {
    const card = name.closest(".relic")
    if (!card) continue
    seats ??= new ResizeObserver((entries) => {
      for (const entry of entries) hugNames(entry.target)
    })
    seats.observe(card)
  }
}

/** Fit every wrapped name under `root` now. */
export function hugNames(root: ParentNode): void {
  for (const name of names(root)) hug(name)
}

const names = (root: ParentNode): NodeListOf<HTMLElement> =>
  root.querySelectorAll<HTMLElement>(".relic-line .relic-name")

function hug(name: HTMLElement): void {
  // Back to what the stylesheet gives it, or the second call measures the
  // first call's answer, and a name with room for one line now never finds out.
  name.style.width = ""
  const box = name.getBoundingClientRect()
  const range = document.createRange()
  range.selectNodeContents(name)
  let left = Number.POSITIVE_INFINITY
  let right = Number.NEGATIVE_INFINITY
  const tops = new Set<number>()
  for (const line of range.getClientRects()) {
    // The clamp hides a third line without unmaking it, and a line cut to an
    // ellipsis still reports its whole length. Neither is text on the card.
    if (line.top >= box.bottom - 1) continue
    tops.add(Math.round(line.top))
    left = Math.min(left, line.left)
    right = Math.max(right, Math.min(line.right, box.right))
  }
  if (tops.size < 2) return
  // Rounded up: a width a fraction under the line's is a line that no longer
  // fits, and the name breaks again somewhere worse.
  name.style.width = `${Math.ceil(right - left)}px`
}
