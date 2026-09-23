import { BASE_GUESSES, RELIC_SLOTS } from "../content/rounds"
import { getBoss } from "./bosses"
import { found, keepGreens, useFound } from "./rules"
import type { Refusal, RunState } from "./state"

/**
 * Ascensions: the run's standing difficulty, chosen before it starts.
 *
 * Linear, in Slay the Spire's model, where ascension N plays every rule up to
 * and including N, rather than a weighted or random selection. Weighted sounds like
 * more content and is less: a player cannot learn a difficulty they cannot
 * predict, a build cannot be planned against a rule that might not appear, and
 * every combination becomes a balance surface nobody tested. Linear gives an
 * ordered curve, one new thing to learn per step, and ten configurations to
 * verify instead of a thousand and twenty-three.
 *
 * The ladder is in two halves. Rungs 1–10 are written by hand and each adds a
 * *rule*: five of them guess rules sharing their machinery with the bosses,
 * five of them bends in the run's own arithmetic. Above 10 there are no rules
 * left worth inventing, so the ladder stops pretending: every rung after the
 * tenth is the targets rising another notch, forever. That is deliberate. A
 * ladder with a top has a last thing to beat and then nothing; a ladder without
 * one is its own scoreboard, and "how high did you get" is a better long-run
 * question than "what was your final score", which depends mostly on how long a
 * won run was farmed.
 *
 * The order was rewritten once, and the principle it replaced is worth keeping
 * on the page because it reads plausibly and measured badly. The rungs used to
 * stack "in roughly the order they hurt": the guess rules first, because they
 * cost a play and not a run, then the economy, then the curve, then the tray.
 * That sorts the rungs by what they *take* rather than by what they cost, and
 * the guess rules turn out to take a great deal and cost almost nothing. The
 * ladder did not drop below its own A0 baseline until rung 6, and a run at A5
 * scored higher than a run at A0. Four rungs of the taught half were free or
 * better, and then three walls in a row.
 *
 * So the paying rungs come first now, spaced apart, and the rules that only
 * narrow what may be typed fill the gaps between them. Measured price of each
 * rule in mean final stage, taken at three to six different placements a piece,
 * because a rule's cost moves with where it sits and the ranking does not:
 *
 *   Finish It  -0.89    No Echoes    -0.10    Hunted    -0.05
 *   Crowded    -0.47    Once Only     0.00    Anchored   0.00
 *   Steeper    -0.36    Dead Weight   0.00    Tyranny   +0.15
 *   Lean Years -0.24
 *
 * Four rules carry the ladder and the rest of the authored half is texture.
 * That is a fact about the content, and no ordering fixes it; what an ordering
 * can do is keep two textural rungs from landing in a row and put the paying
 * ones where a player has something to lose to them. Rungs 8 and 10 are the
 * same rule twice and that is deliberate; see rung 10, which is the sharpest
 * thing in the game.
 *
 * Measured over 250 seeds a piece, reported as mean final stage. The stage rather
 * than the win rate, because above rung 6 the win rate is low enough that 250
 * seeds cannot separate two neighboring rungs and the stage can:
 *
 *   A0  4.92   A3  4.72   A6  4.35   A9   3.98
 *   A1  4.98   A4  4.72   A7  4.10   A10  2.93
 *   A2  5.10   A5  4.86   A8  4.11
 *
 * Read the top three as one number. At this sample the standard error on a mean
 * stage is around 0.16, so A1 and A2 sitting above A0 is not the ladder handing
 * anything back; it is two rungs that cost nothing being measured as costing
 * nothing, which is what the price table already says of them. What the table
 * does show is a first real step at rung 3 and no flat pair anywhere above it.
 *
 * The ladder now starts asking at rung 3 rather than rung 6. The total did not
 * move and could not have: every rung folds with `*=`, `+=` and `||=`, all
 * commutative, so A9 and A10 are byte-identical under any permutation of the
 * rungs below them. Reordering redistributes a fixed budget; it cannot add to
 * one, and a ladder made harder early is a ladder made flatter late.
 *
 * Two things about the instrument, both of which have already cost this file a
 * misjudgement.
 *
 * The numbers read off a bot that deduces honestly and will farm a round it
 * cannot solve. Every number here up to rung 8 was first taken with a bot that
 * reads the answer and always solves, and such a bot cannot price the top of
 * this ladder at all. It scored rungs 9 and 10 as byte-identical, on win rate,
 * final stage, rounds cleared and gold per round alike, because a player who
 * always finds the word is a player no must-solve rule can touch. What that
 * instrument called free is what the top of the ladder is now built around, and
 * what it called survivable at rung 9 was a wall.
 *
 * And a bot has no style. The prices above are one policy's. A second policy
 * that buys the gray-scoring relics and replays its best-scoring word prices
 * this ladder differently, and the difference is the whole argument for where
 * Once Only, Dead Weight and No Echoes now sit: each is 0.00 to a solver and
 * none of them is free. Read every zero in that table as "free to one kind of
 * player" and never as free. See each in place.
 *
 * The figures above were re-taken after `test/helpers/blind.ts` was repaired, and
 * the repair is the reason to trust them over the ones they replace. That player
 * could run out of words it was willing to type, return null, and be filed as a
 * death: a silent stall that no test caught and that only ever pushed a number
 * down. It had two causes and they sit at opposite ends of this ladder. At rung 0
 * it was Pyromaniac breaking letters the player kept trying to spell with, which
 * ended 33 of 250 runs. From rung 3 up it was the rules here refusing whole
 * classes of word with the alphabet still intact, which ended 169 of 250 at rung
 * 5 — more than two thirds of this table's mass, at the rungs the reshuffle
 * moved. Both are fixed, both fixes are in the committed harness, and the sweep
 * above reports zero stalls at every rung.
 *
 * So an old A0 of 4.79 was never the floor of anything; it was the instrument.
 * The rung *prices* in the earlier table are differences and mostly survived,
 * which is why they are still quoted per rule above, but the absolute column did
 * not: it read rung 5 at 4.73 against the 4.86 measured now, and understated the
 * middle of the ladder exactly where the stalls were densest.
 */
export type Ascension = {
  /** The level this arrives at. A run at level N plays every rule at or below N. */
  level: number
  /**
   * The two numbers the synthesized endless rung describes itself with, set
   * only on that rung. The authored ten are a lookup in `ascension[level]` and
   * need nothing; this one is computed from how far past the ladder the run has
   * climbed, so the numbers come from here and the sentence from the catalog.
   */
  endless?: { percent: number; total: number }
  /** Why a guess is refused, or null to allow it. */
  validate?: (word: string, state: RunState) => Refusal | null
  /** Ascension 10 alone: clearing the target stops being enough. */
  solveRequired?: true
  /** Gold this rule takes off every round's base payout. */
  payoutCut?: number
  /** What this rule multiplies every round's target by. */
  targets?: number
  /** Relic slots this rule takes off the tray. */
  relicCut?: number
  /** Whether a round cleared without the word being found pays nothing at all. */
  unpaidIfUnsolved?: true
}

export const ASCENSIONS: readonly Ascension[] = [
  {
    level: 1,
    // Wordle's hard mode, and the mildest thing here: it costs the player their
    // throwaway probing words, and nothing else.
    validate: (word, state) => useFound(word, found(state.round, "yellow")),
  },
  {
    level: 2,
    /*
     * Nobody guesses the same word twice on purpose, so to a player who is
     * deducing this is free: 0.00 of a mean final stage at every placement
     * measured. What it costs is the other line entirely.
     *
     * A run that buys the gray-scoring relics and replays one high-scoring word
     * spends 51.7% of its guesses on a word it has already said. This rung takes
     * that to 33.6% the round it arrives, and it never climbs back for the rest
     * of the ladder. It is at 2 because the degenerate half of that build — the
     * same five letters, six times, forever — is worth ending before a player
     * can mistake it for the game, and the rest of it is not. Rung 9 ends the
     * rest, and eight rungs is how long the interesting half gets to live.
     */
    validate: (word, state) =>
      state.round.guesses.some((guess) => guess.word === word)
        ? { code: "already_guessed_round" }
        : null,
  },
  {
    level: 3,
    /*
     * The curve itself, and the shape every rung above 10 repeats at a gentler
     * angle; see `ENDLESS_STEP`.
     *
     * 15% is picked against the stage growth rather than in the abstract. Targets
     * multiply by 2.2 a stage, so a notch this size is worth about a fifth of a
     * stage of standing pressure: enough to feel on the round it lands on, not
     * enough to end a run by itself.
     *
     * It sat at rung 7 and is most of the reason the entrance used to be free.
     * It is the only authored rung that takes nothing away — every other one
     * removes a resource and this one just moves the finish line — which is
     * exactly what makes it the right first thing to cost anything. A player two
     * rungs in meets a harder round rather than a smaller run, and a harder
     * round is a thing they already know how to think about.
     *
     * Measured at 7 it was -0.41 of a mean final stage, win rate 21.2% to 17.2%.
     * Measured here at 3 it is -0.36. Compounding is what it gives up by moving
     * down and half a tenth of a stage is the whole of the bill.
     */
    targets: 1.15,
  },
  {
    level: 4,
    // Greens, unpositioned: the step between hard mode and The Tyrant. Once
    // level 5 lands this can no longer fire on its own, since a letter kept in
    // its place is by definition still in the word.
    //
    // It prices at 0.00 for every policy measured, at every placement, and so
    // does the rung after it. Two rungs that ask nothing is a fact about the
    // content rather than the order; see rung 5 for why they are here rather
    // than spread apart.
    validate: (word, state) => useFound(word, found(state.round, "green")),
  },
  {
    level: 5,
    /*
     * The Tyrant, permanently. Shares its implementation with the boss rather
     * than restating it, so the run-long version cannot drift from the one the
     * player met on stage 4.
     *
     * It is also the only rung that measures *negative*: +0.07 to +0.27 of a
     * mean final stage across six placements, never once a cost. Forcing a green
     * to stay put is a constraint on a player who was moving them and a guardrail
     * on one who was not, and the bot is the second kind — a human who has been
     * playing carefully pays nothing here either, which is the honest reading of
     * a rule that only forbids a mistake.
     *
     * Left at 5 anyway. Moving it to 7 to break up the soft pair was measured
     * and was worse: the price stayed positive and landed *after* the ladder's
     * steepest step instead of before it, so a player who cleared rung 6 found 7
     * and 8 easier, and the whole climb read as going backwards. A soft rung
     * beside another soft rung, early, where the absolute difficulty is high and
     * the player is still learning the vocabulary, is the cheapest place on the
     * ladder to spend one. If the middle ever has to bite, this is the rung to
     * re-price rather than to re-place — and re-pricing it re-prices The Tyrant.
     */
    validate: (word, state) => keepGreens(word, state.round),
  },
  {
    level: 6,
    /*
     * The build, capped.
     *
     * This was written first as a cut to the *shelf*, the shop dealing one relic
     * instead of two, and 250 seeds said that rule does nothing at all: win rate
     * 17.2% either way, tray value $32.1 to $31.1, and the same 4.97 cards held at
     * the end. Which makes sense in hindsight. A run visits the shop around
     * nineteen times, so even one look a visit is nineteen looks for five slots;
     * what stops a tray filling is gold, not offers, and halving the offers
     * changes neither. It read as a difficulty rule and was scenery.
     *
     * Taking the slot instead bites: 17.2% to 12.4%, tray value $32.1 to $26.0,
     * 4.97 cards held to 3.98. The difference is that this one cannot be waited
     * out. Every relic bought past the fourth now has to displace one already
     * earning, so the rung asks a question the shelf-cut never did, not "did you
     * find it" but "is it better than what you have", and it asks it in every
     * shop for the rest of the run.
     *
     * The −19% of tray value is the honest measure of the cost, and it is worth
     * noting that a bot understates it: this player has no preference among
     * relics, so it loses an average slot where a human loses their fifth-best.
     *
     * It is the dearest rung below the capstone and the only paying one that
     * does not get cheaper for being moved down: -0.46 at rung 2, -0.52 at 4,
     * -0.55 at 5, -0.47 here at 6, and -0.39 back at 8 where it used to sit. So
     * the placement is a free choice among the first six, and it is spent on
     * teaching. A slot taken means nothing to a player who has never wanted a
     * fifth relic, and by rung 6 they have climbed five ladders' worth of runs
     * with a full tray and know exactly which card they are about to lose.
     */
    relicCut: 1,
  },
  {
    level: 7,
    /*
     * The money, behind the tray that has to be bought with it.
     *
     * A flat dollar off the base, $3/$4/$5 becoming $2/$3/$4, rather than the
     * obvious alternative, which was to stop paying for unused guesses. That
     * alternative would have been actively wrong. The unused-guess dollar is the
     * only reward in the game on a different axis from the score curve, and the
     * only measured lever that pulls a player toward solving early rather than
     * farming five wrong words to the target; deleting it here would have
     * tilted the entire upper ladder toward farming.
     *
     * Cutting the base does the opposite. At $2 instead of $3 the unused-guess
     * dollars are a larger share of the take, so the higher a run climbs the more
     * it is paid to finish early, which is the behavior the rest of the ladder
     * is about to start demanding outright at rung 10.
     *
     * A dollar sounds small and is a quarter of the base: $12 a lap becomes $9.
     * Measured against everything a run actually earns, unused-guess dollars and
     * interest included, which is why the headline number overstates it, income
     * falls 12%, from $9.29 a round to $8.20, and the win rate from 24.8% to
     * 21.2% over 250 seeds. Interest is capped at $5 and so cushions almost none
     * of it; what the run loses is close to one shop visit in eight.
     *
     * Those figures were taken at rung 6, where it read -0.40 of a mean final
     * stage. Here at 7, behind Crowded, it reads -0.24, and at rung 2 it read
     * -0.25. A tray already capped at four has less for the dollar to be taken
     * away from, so some of this rung's old price was Crowded's price arriving
     * early. That overlap is real and it is what putting the tray first costs.
     */
    payoutCut: 1,
  },
  {
    level: 8,
    /*
     * The hedge, priced in money before rung 10 prices it in the run.
     *
     * What stood here was "Five", five guesses instead of six, and it is worth
     * recording why it is gone, because it was the largest misjudgement on the
     * ladder. It reads like a sibling of the boss guess rules and it is not: it
     * hits three ways at once. The deduction gets a guess harder, the farming
     * line loses its biggest chip contributor, and the unused-guess dollars are
     * capped one lower on top of Lean Years having already taken a dollar off
     * the base. Measured against a bot that has to actually find the word it
     * cost 1.22 of a mean final stage in a single step, against 0.84 for the
     * eight rungs that then sat below it put together. One rung outweighed the
     * whole ladder under it, and in that order it landed on players who had just
     * been handed the relic-slot cut. The first boss killed 26% of runs at A9
     * and 43% at A10.
     *
     * It only ever looked reasonable because the bot that priced it reads the
     * answer off the state, so it paid for the missing guess in chips and never
     * once in a word it failed to find. That bot put the rung at 0.36.
     *
     * This asks the same question Finish It asks, in money first: clear the
     * target off five wrong words and the round is survived and pays nothing:
     * no base, no unused-guess dollars, no interest. What it takes is a habit,
     * two rungs before the capstone starts taking runs for the same habit.
     *
     * It prices at 0.00 against a solver at every placement, and that is the
     * point rather than a defect: this rung is not aimed at a player who finds
     * the word. Against the run that clears its target off five wrong ones it
     * took 0.40 of a mean final stage when it landed on a build still repeating
     * freely. Behind Once Only at rung 2 it takes 0.11. The two anti-farm rungs
     * overlap, the ladder cannot collect both prices, and moving either one
     * earlier partly refunds the other. That is the standing cost of ending the
     * degenerate build at rung 2, and it is worth paying.
     */
    unpaidIfUnsolved: true,
  },
  {
    level: 9,
    /*
     * The rule that ends a way of playing rather than costing a guess, which is
     * why it is this late and why it used to be rung 3.
     *
     * The opener that scores best is gone after stage one, and a run has to keep
     * finding new words that pay. This is why the run keeps a history at all.
     *
     * At rung 3 it deleted a whole line before the player had seen enough of the
     * game to know the line existed. A run that buys the gray-scoring relics and
     * replays its best-scoring words measures at least level with a run that
     * solves honestly, and finds the word in 72% of its rounds against the
     * solver's 92%: a genuinely different road to the same stages, not a cheese
     * and not a trap. This rung takes it to zero repeated guesses in the frame
     * it arrives. Rung 2 has already taken the degenerate half. What is left is
     * worth keeping legal for eight rungs and worth ending on the ninth.
     *
     * The move down cost about 0.10 of a mean final stage and nothing else,
     * which is the other half of why it was worth making: a rule this disliked
     * should not also be one of the four that hold the ladder up, and it is not.
     * A solver still replays a word from earlier in the run on 28% of its
     * guesses wherever this rule is absent. That is what the rung takes from
     * everyone, and it is why it is the one players argue about.
     */
    validate: (word, state) =>
      state.history?.includes(word) ? { code: "already_used_run" } : null,
  },
  {
    level: 10,
    /*
     * The sharpest rule in the game, and the reason it is the last authored one.
     * Every other round can be won by farming chips off five wrong guesses and
     * never finding the answer; this deletes that line, and with it the whole
     * deduction-versus-greed hedge. The word is the point again. Everything above
     * it is pressure rather than rule, so this is the last thing the ladder ever
     * has to teach, the right note to end the taught half on and the wrong one
     * to have buried in the middle of it.
     *
     * It is also the rung the old harness scored at exactly zero, with identical
     * win rate, final stage, rounds cleared and gold per round at A9 and A10, because
     * a bot that solves every round it can already obeys this rule and never
     * noticed it arrive. That was never evidence the rung is free; it is evidence
     * of what the rung taxes. A bot that deduces honestly and hedges prices it at
     * 3.98 mean final stage down to 2.93, a full stage and by some way the
     * largest single step on the ladder. What it costs is the hedge, and a
     * human move: the round where the word will not come, the pile is nearly
     * there, and two more wrong guesses would bank it anyway. Under this rule
     * that round is lost.
     *
     * It swallows rung 8 whole, and the measurement says so to the decimal: a
     * ladder carrying both rules scores identically at A10, on every column and
     * not merely the stage, to a ladder carrying only this one. There is nothing
     * left to withhold from a round that has already ended the run, so the
     * capstone is itself and rung 8 at once. That is escalation along one axis
     * rather than a rung going to waste: lose the money for not finding the word
     * at 8, then lose the run for it here. Whoever arrives has met the idea once
     * already, which is the only reason a rule this sharp can be the one nobody
     * gets warned about twice.
     */
    solveRequired: true,
  },
]

/** Where the hand-written half stops and the ladder starts repeating itself. */
export const AUTHORED_ASCENSIONS = ASCENSIONS.length

/**
 * What each rung above `AUTHORED_ASCENSIONS` multiplies the targets by, on top
 * of everything below it. Steeper's rule at roughly half Steeper's angle, and
 * the halving is the whole reason the endless half is worth having.
 *
 * It was written at 15% first, the same notch, on the theory that a repeated
 * rung repeating something *else* would be a new rule wearing a number, and the
 * ladder cannot owe the player ninety explanations. Compounding settled it. At
 * 1.15 the climb ran out in about five rungs: 5.7 stages cleared at A10, 2.2 at
 * A15, nothing alive by A22: a scoreboard with five marks on it. At 1.08 the
 * same bot spans sixteen: 5.54 stages at A10, 4.75 at A14, 3.28 at A18, 2.48 at
 * A20, 1.10 by A26, with wins still turning up as high as A20. About a quarter
 * of an stage a rung.
 *
 * Which is the property being bought. A rung has to be small enough that taking
 * the next one is a decision rather than a coin flip, and a ladder meant to be
 * the game's own win condition needs somewhere to put the players who are
 * better at this than the harness is.
 */
const ENDLESS_STEP = 1.08

/**
 * A ceiling on the dial, not a top to the ladder.
 *
 * The endless half is endless in the sense that matters, since nobody is going
 * to exhaust it, but a number that a save can carry and a stepper can walk needs
 * *some* bound, or a hand-edited record could ask for a target of Infinity and
 * get a run with no legal outcome. A hundred is far past anything survivable:
 * rung 100 carries 1.08^90 on top of Steeper's own 15%, which is a little over a
 * thousand times the authored curve, on a ladder where the measured climb dies
 * somewhere in the twenties.
 */
export const MAX_ASCENSION = 100

/**
 * What a level actually means, for a screen that has to explain the choice.
 *
 * Synthesized above the authored ladder rather than stored, because storing
 * ninety near-identical entries would be a list nobody could read and a codex
 * nobody could scroll. The synthesized rung says what it does and what it has
 * come to in total, since "another 15%" stops being useful information around
 * the third time it is said.
 */
export function ascensionAt(level: number): Ascension | undefined {
  const authored = ASCENSIONS.find((rule) => rule.level === level)
  if (authored) return authored
  if (!Number.isInteger(level) || level <= AUTHORED_ASCENSIONS || level > MAX_ASCENSION) {
    return undefined
  }
  return {
    level,
    endless: {
      percent: Math.round((ENDLESS_STEP - 1) * 100),
      total: difficultyAt(level).targets,
    },
    targets: ENDLESS_STEP,
  }
}

/** A level anyone can be at: whole, not negative, not past the dial. */
export const clampAscension = (level: number): number =>
  Number.isFinite(level) ? Math.min(MAX_ASCENSION, Math.max(0, Math.floor(level))) : 0

/**
 * Everything the ladder bends about a run, folded into one object.
 *
 * One fold rather than a lookup at each call site, and that is the whole reason
 * this type exists. The alternative, `rulesFor(state).some(…)` wherever a
 * number is needed, worked while the only bend was `solveRequired`, and would
 * have meant the reducer asking the ladder five separate questions about five
 * separate fields, none of which could account for the endless half without
 * inventing ninety `Ascension` records to iterate over.
 */
export type Difficulty = {
  /** Multiplier on every round's target. */
  targets: number
  /** Gold off every round's base payout. */
  payoutCut: number
  /** Relics the tray holds. */
  relicSlots: number
  /** The round's guess allowance, before a boss tightens it. */
  guesses: number
  /** Whether a round at target but unsolved pays nothing. */
  unpaidIfUnsolved: boolean
  /** Whether a round at target but unsolved is still a loss. */
  mustSolve: boolean
}

/** The terms of a run at a level. Pure in the level, so nothing caches it. */
export function difficultyAt(level: number): Difficulty {
  const at = clampAscension(level)
  const difficulty: Difficulty = {
    targets: 1,
    payoutCut: 0,
    relicSlots: RELIC_SLOTS,
    guesses: BASE_GUESSES,
    unpaidIfUnsolved: false,
    mustSolve: false,
  }
  for (const rule of ASCENSIONS) {
    if (rule.level > at) break
    difficulty.targets *= rule.targets ?? 1
    difficulty.payoutCut += rule.payoutCut ?? 0
    difficulty.relicSlots -= rule.relicCut ?? 0
    difficulty.unpaidIfUnsolved ||= rule.unpaidIfUnsolved ?? false
    difficulty.mustSolve ||= rule.solveRequired ?? false
  }
  difficulty.targets *= ENDLESS_STEP ** Math.max(0, at - AUTHORED_ASCENSIONS)
  return difficulty
}

/** The same, read off a run. Absent means zero, which means the plain game. */
export const difficultyOf = (state: RunState): Difficulty => difficultyAt(state.ascension ?? 0)

/**
 * A scaled target, rounded to a readable number.
 *
 * To the nearest ten rather than the nearest hundred `roundTargets` uses,
 * because 15% of the first target is 45 and rounding that to 0 or 100 would make
 * the first rung of the endless half either free or twice what it says. Ten is
 * fine enough to keep every rung distinct and coarse enough that the number on
 * the card still reads as a target rather than as a computation.
 */
export const scaleTarget = (target: number, scale: number): number =>
  scale === 1 ? target : Math.round((target * scale) / 10) * 10

/**
 * The rules a run is playing under, for the screens that name them.
 *
 * Authored only. The endless rungs are deliberately not here: they carry no
 * `validate` and no `solveRequired`, so they would change nothing about a guess,
 * and a run at ascension 30 would list twenty entries of "Steeper" on its intro
 * card. What the endless half does is a number, and the screens say the number.
 */
export function rulesFor(state: RunState): readonly Ascension[] {
  const level = state.ascension ?? 0
  return level > 0 ? ASCENSIONS.filter((rule) => rule.level <= level) : []
}

/** Ascension 10: a round at target but unsolved is still a loss. */
export const mustSolve = (state: RunState): boolean => difficultyOf(state).mustSolve

/**
 * Every rule the guess has to survive, in one place and one order.
 *
 * The order is by scope: the run's rules before the round's, and within the
 * run's, the order they were learned. A player who breaks two rules at once
 * gets told about the one that holds every round of the run rather than the one
 * that expires with this boss, and, the part that actually matters, gets told
 * the *same* thing every time, because the sequence never depends on which rule
 * happened to be checked first.
 */
export function validateGuess(word: string, state: RunState): Refusal | null {
  for (const rule of rulesFor(state)) {
    const refusal = rule.validate?.(word, state)
    if (refusal) return refusal
  }
  return getBoss(state.round.bossId)?.validate?.(word, state.round) ?? null
}

/** Whether anything at all is restricting what may be typed this round. */
export const guessRestricted = (state: RunState): boolean =>
  Boolean(getBoss(state.round.bossId)?.validate) || rulesFor(state).some((rule) => rule.validate)
