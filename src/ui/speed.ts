/**
 * How fast the game plays its own animations.
 *
 * The scoring cascade is the game explaining itself: a tile turns, its
 * `+chips +mult` floats off it, a relic lights up and says what it did, and the
 * total counts up to the number all of that made. On a first run every beat of
 * that is worth waiting for. On a fiftieth it is a cutscene standing between
 * the player and the next guess, and the only way past it was to tap the screen
 * and skip the rest, which is all or nothing and has to be asked for again on
 * every guess.
 *
 * This is the dial that was missing between those two. It is one number, and it
 * scales both halves of every animation: the timers in `app.ts` that decide when
 * a class comes back off, and the keyframe durations in the stylesheet, which
 * read it as `--pace`. One setting drives both because they are two ends of the
 * same motion, and `FLIP.total` waiting 380ms on a flip that now takes 127 is
 * exactly the mismatch the note on that constant warns about.
 *
 * Three things are durations and are deliberately not on this dial:
 *
 * - Reading time. The refusal toast sits for 2200ms because that is how long a
 *   sentence takes to read, and a player asking the board to hurry up has not
 *   asked to be told less legibly why their word was refused.
 * - Input conventions. The long press is 350ms because iOS fires its own at 500
 *   and Android at 400. That is a fact about the platform, not about this game.
 * - The two loops that never end, the legendary sheen and the coaching's
 *   breathing outline. Nothing waits on either, so running them faster makes an
 *   ambient thing insistent and buys back no time at all.
 *
 * Measured in Chrome at 390x844, one guess timed from Enter to the last badge
 * leaving the screen: 1361ms at ×1 and 529ms at ×3, with `flip` reading 0.38s
 * against 0.127s and `gain-rise` 0.76s against 0.253s on the frames in between,
 * which is the stylesheet moving with the timers rather than beside them. The
 * same guess with `prefers-reduced-motion` set takes 1404ms and 527ms, because
 * the beats are still paced for someone reading them one at a time even when
 * nothing is moving, and every animation on screen still computes to `none`
 * there: the kill-switch outranks a `calc()` duration exactly as it outranked a
 * literal one, and `.tile-gain` kept the `-50%` the carve-out hands back.
 *
 * There is no "off" rung, and that is the one value this file is not allowed to
 * reach. A duration of zero does not stop a `forwards` animation, it *completes*
 * it in the frame it starts, which deletes every element whose keyframes end at
 * `opacity: 0`: the whole `+chips +mult` cascade, and the floaters that name
 * what each relic paid. That is the bug the reduced-motion block in the
 * stylesheet was rewritten twice to be rid of, and a speed setting reaching zero
 * would walk it back in through the front door. Stillness already has an answer
 * and it is spelled `animation: none`, under a preference the player sets once;
 * this is a dial, and it stays on the multiplying side of one.
 */

const SPEED_KEY = "5wild:speed"

/**
 * The rungs, each one being the multiplier it is.
 *
 * The value *is* the setting: 2 means twice as fast and prints as "×2", so there
 * is no table mapping a name onto a number and no way for the two to disagree.
 * Three of them because a cycling button is one tap per step and a ladder with
 * ×1.5 on it costs every player a tap past a rung almost nobody can see; the
 * gap between ×1 and ×2 is the one anybody asked for.
 *
 * 1 leads because it is the speed every duration in this game is written at.
 * The stylesheet's `--pace: 1`, the fallback in `readSpeed`, and the constants
 * in `app.ts` are all the same claim, and a first launch has to land on it.
 */
export type Speed = 1 | 2 | 3

export const SPEEDS: readonly Speed[] = [1, 2, 3]

/** One button, one step per tap, wrapping at the end. As the decoration toggle. */
export const NEXT_SPEED: Record<Speed, Speed> = { 1: 2, 2: 3, 3: 1 }

/**
 * A duration, at a speed. The whole of the arithmetic, in one place, so the
 * stylesheet's `calc(380ms * var(--pace))` and a `setTimeout` in `app.ts` are
 * provably the same sum rather than two roundings that agree most of the time.
 */
export const atSpeed = (ms: number, speed: Speed): number => Math.round(ms / speed)

/**
 * Whatever was stored, read as a speed.
 *
 * Anything unrecognized is normal speed, which is also what a first launch gets,
 * so a store that has been blocked or scribbled on lands on the game as it is
 * authored rather than on a pace the player never chose.
 */
export const readSpeed = (raw: string | null): Speed =>
  SPEEDS.find((speed) => String(speed) === raw) ?? 1

export function loadSpeed(): Speed {
  try {
    return readSpeed(localStorage.getItem(SPEED_KEY))
  } catch {
    return 1
  }
}

/**
 * Apply it to the document, and remember it.
 *
 * A custom property on the root rather than a class, which is what every other
 * presentational setting here uses. `.plain` and `.quiet` are classes because
 * what they switch is a rule the stylesheet can state on its own: draw this pip,
 * or do not. This one is a number, and it is the same number `app.ts` divides
 * its timers by. Three classes carrying three factors would write that number
 * into the stylesheet as well as here, and a pair that drifts is a flip whose
 * class comes off before its animation ends, which leaves the tile stuck
 * mid-turn. So the setting is written once, as a property, and the stylesheet
 * multiplies by it.
 *
 * The property is a scale on *duration* rather than the speed itself, so it is
 * the reciprocal: ×2 is `--pace: 0.5`. Multiplying is what a stylesheet can do
 * everywhere; dividing by a `var()` inside `calc()` is a newer thing to ask of
 * an engine, and an engine that refuses it drops the whole declaration, which
 * would leave every animation at a duration of zero. That is the failure this
 * file exists to avoid, so it is not worth a tidier property name.
 */
export function setSpeed(speed: Speed): void {
  document.documentElement.style.setProperty("--pace", String(1 / speed))
  try {
    localStorage.setItem(SPEED_KEY, String(speed))
  } catch {
    // The setting lasts the session instead of the install. Nothing else breaks.
  }
}
