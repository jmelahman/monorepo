/**
 * Letter chip values: rarity-inverse, Scrabble-like.
 *
 * This table is where the game's central tension is born. AROSE is 5 chips and
 * a superb opening probe; JAZZY is 33 chips and deduction poison. The player
 * feels the conflict between "guess well" and "score well" on turn one.
 *
 * Written as groups rather than 26 entries so the shape of the curve stays
 * readable, and so the formatter cannot explode it into a wall of lines.
 */
const CHIP_GROUPS: ReadonlyArray<readonly [number, string]> = [
  [1, "aeioulnstr"],
  [2, "dg"],
  [3, "bcmp"],
  [4, "fhvwy"],
  [5, "k"],
  [8, "jx"],
  [10, "qz"],
]

export const LETTER_CHIPS: Readonly<Record<string, number>> = Object.fromEntries(
  CHIP_GROUPS.flatMap(([chips, letters]) => [...letters].map((letter) => [letter, chips])),
)

/**
 * Green is worth three of these, yellow one. Gray is worth nothing, on purpose.
 *
 * Content rather than engine because it is a balance table, and because both the
 * scoring pipeline and the Wild modifier, which scores a tile as if it were
 * green, have to agree on what a color is worth.
 */
export const MULT_FOR_COLOR = { green: 3, yellow: 1, gray: 0 } as const

export const ALPHABET = "abcdefghijklmnopqrstuvwxyz"
export const VOWELS = "aeiou"

/**
 * Nothing breaks a letter below this many live ones. Both breakers, the
 * Pyromaniac and a Glass letter that fails its roll, check it, because an
 * alphabet too thin to spell with is a dead run rather than a hard one.
 */
export const MIN_LIVE_LETTERS = 15

export const isVowel = (letter: string): boolean => VOWELS.includes(letter)
