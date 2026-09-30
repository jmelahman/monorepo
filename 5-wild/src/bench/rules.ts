/**
 * The briefing a benchmarked model reads before its first move.
 *
 * The rules half is the rules sheet, sentence for sentence, fed the same
 * numbers `helpView` feeds it, so that "how to play" means one thing whether a
 * person or a model is reading it. Only the sheet's instructions to a thumb are
 * left out: "hold the score", "tap one to read it" and the faint bar describe a
 * screen this reader does not have, and the observation says the same things
 * in words.
 *
 * The how-to-act half is the only prose in the benchmark that is not the
 * game's, and it is short on purpose. It says what the commands are and what a
 * run is scored on, and it gives no strategy: the benchmark is asking what the
 * model makes of the game, and a hint here would be a hint to every model.
 */

import {
  AUTHORED_ASCENSIONS,
  CONTENT_VERSION,
  GOLD_PER_UNUSED_GUESS,
  INTEREST_CAP,
  INTEREST_PER,
  RELIC_SLOTS,
  ROUND_PAYOUT,
  ROUNDS_PER_STAGE,
  STAGES,
} from "../engine"
import { money } from "../ui/format"
import { ui } from "../ui/lang"

export function rulesText(): string {
  const copy = ui().help
  const rule = (term: string, text: string) => `- ${term}: ${text}`
  return [
    "# 5 Wild",
    "",
    copy.lead,
    copy.scored,
    "",
    rule(copy.chipsMult.term, copy.chipsMult.text),
    rule(copy.letters.term, copy.letters.text),
    rule(copy.colors.term, copy.colors.text),
    rule(copy.solving.term, copy.solving.text),
    "",
    copy.farming,
    "",
    `## ${copy.runHeading}`,
    "",
    rule(copy.target.term, copy.target.text(STAGES, ROUNDS_PER_STAGE, AUTHORED_ASCENSIONS)),
    rule(copy.endless.term, copy.endless.text(STAGES)),
    rule(copy.bosses.term, copy.bosses.text),
    rule(copy.ascensions.term, copy.ascensions.text(AUTHORED_ASCENSIONS)),
    rule(
      copy.money.term,
      copy.money.text(
        ROUND_PAYOUT.map(money).join(" / "),
        money(GOLD_PER_UNUSED_GUESS),
        money(INTEREST_PER),
        money(INTEREST_CAP),
      ),
    ),
    rule(
      copy.relics.term,
      `Up to ${RELIC_SLOTS}, and they fire left to right, so the order matters.`,
    ),
    rule(copy.packs.term, copy.packs.text),
    rule(copy.mods.term, copy.mods.text),
    "",
    ACTING,
  ].join("\n")
}

const ACTING = `## Playing through this interface

You cannot see the answer. Each observation shows the board, your gold, relics,
consumables, the shop or pack when one is open, and a LEGAL COMMANDS list for
the current screen. Send exactly one command per move:

- guess <word>   play a five-letter word (it must be in the game's word list)
- use <n>        use consumable n during a round
- collect        bank a cleared round's reward
- buy <n>        buy shop item n
- sell <n>       sell relic n
- drop <n>       throw away consumable n in the shop, for nothing
- reroll         reroll the shop
- next           leave the shop for the next round
- pick <n>       take card n from an open pack
- skip           walk away from an open pack
- place <letter> stick the modifier you just bought on a letter
- continue       play on past the win into the endless stages
- end            stop at the victory screen and keep the win

Numbers count from 1. A refused command changes nothing and the next
observation says why. Too many refusals in a row ends the run.

The run is scored by how many rounds you clear before it ends, then by
whether you win and by your total score.`

/**
 * What a result was measured against, as eight hex digits.
 *
 * The briefing and the content version together, because either moving makes
 * two runs incomparable: a reworded rule is a different prompt and a rebalanced
 * relic is a different game. FNV-1a rather than a real digest because this has
 * to run in the browser and in Node with nothing imported, and it only has to
 * tell versions apart, not resist anyone.
 */
export function promptHash(): string {
  const text = `${CONTENT_VERSION}\n${rulesText()}`
  let hash = 0x811c9dc5
  for (let i = 0; i < text.length; i++) {
    hash ^= text.charCodeAt(i)
    hash = Math.imul(hash, 0x01000193)
  }
  return (hash >>> 0).toString(16).padStart(8, "0")
}
