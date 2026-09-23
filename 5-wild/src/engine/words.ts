import type { Color, Tile } from "./state"

/**
 * Wordle's coloring, including the part everyone gets wrong the first time:
 * duplicate letters. Greens are claimed first, then yellows draw only from the
 * copies left over. Guessing LOLLY against SILLY marks the leading L *gray*:
 * both of the answer's Ls are already spoken for by the two green ones. A
 * single pass cannot produce that; hence two.
 */
export function computeFeedback(guess: string, answer: string): Color[] {
  const colors: Color[] = Array.from(guess, () => "gray")
  const remaining = new Map<string, number>()

  for (const letter of answer) remaining.set(letter, (remaining.get(letter) ?? 0) + 1)

  for (let i = 0; i < guess.length; i++) {
    if (guess[i] === answer[i]) {
      colors[i] = "green"
      const letter = guess[i]
      if (letter !== undefined) remaining.set(letter, (remaining.get(letter) ?? 0) - 1)
    }
  }

  for (let i = 0; i < guess.length; i++) {
    if (colors[i] === "green") continue
    const letter = guess[i]
    if (letter === undefined) continue
    const left = remaining.get(letter) ?? 0
    if (left > 0) {
      colors[i] = "yellow"
      remaining.set(letter, left - 1)
    }
  }

  return colors
}

/**
 * `color` drives scoring, `shown` drives the screen. They are equal except when
 * a boss lies to the player. The Fog hides yellows without disarming them.
 */
export function toTiles(guess: string, colors: readonly Color[]): Tile[] {
  return Array.from(guess, (letter, i) => {
    const color = colors[i] ?? "gray"
    return { letter, color, shown: color }
  })
}

/**
 * The best color known for each letter so far, for painting the keyboard.
 *
 * `shown`, because the keyboard is the board's own summary and the two must
 * agree: a boss that hides a yellow on the board would be caught out by a key
 * that did not hide it.
 *
 * The Magician's tile is the one place they are allowed to disagree, and it goes
 * the other way from a boss: the board shows the color because the card really
 * did paint it, and the keyboard reads the letter back as the gray it was,
 * because the keyboard is a running claim about the *answer* and this letter may
 * be in no word at all. See `Tile.promoted`, and `rules.ts` for the harder
 * version of the same reason.
 *
 * Gray rather than skipped: a promoted tile was gray before the card touched it,
 * which is real feedback and the only place it now survives. Dropping the letter
 * instead would leave a key looking untried when the round has in fact ruled it
 * out, and would make the card cost the player a deduction it never claimed to.
 */
export function keyboardColors(guesses: ReadonlyArray<{ tiles: readonly Tile[] }>) {
  const rank: Record<Color, number> = { gray: 0, yellow: 1, green: 2 }
  const best = new Map<string, Color>()
  for (const guess of guesses) {
    for (const tile of guess.tiles) {
      const shown = tile.promoted ? "gray" : tile.shown
      const current = best.get(tile.letter)
      if (current === undefined || rank[shown] > rank[current]) {
        best.set(tile.letter, shown)
      }
    }
  }
  return best
}
