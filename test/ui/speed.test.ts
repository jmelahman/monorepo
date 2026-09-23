import { describe, expect, it } from "vitest"
import { atSpeed, NEXT_SPEED, readSpeed, SPEEDS, type Speed } from "../../src/ui/speed"

/**
 * The animation dial, which is two numbers agreeing across a language boundary.
 *
 * `app.ts` divides its timers by the setting and the stylesheet multiplies its
 * durations by the reciprocal, and the failure when those disagree is silent:
 * a `flip` class removed before its animation ends leaves a tile frozen edge-on
 * for the rest of the round, and a `tile-gain` node removed early takes the
 * `+chips +mult` badge off the screen mid-rise. Neither throws, neither shows up
 * in a test of the engine, and both look like a rendering bug rather than an
 * arithmetic one. So the arithmetic is checked here, where it is one function.
 */
describe("animation speed", () => {
  it("starts at the speed the game is drawn at", () => {
    // Not merely a default: the stylesheet declares `--pace: 1`, every duration
    // in it is authored at that, and a first launch has to land on the game as
    // designed rather than on a pace nobody chose.
    expect(SPEEDS[0]).toBe(1)
    expect(atSpeed(380, 1)).toBe(380)
  })

  it("divides a duration by the speed", () => {
    expect(atSpeed(380, 2)).toBe(190)
    expect(atSpeed(900, 3)).toBe(300)
  })

  it("never rounds a duration away to nothing", () => {
    // A zero is not a fast animation, it is a completed one: anything ending at
    // `opacity: 0` is deleted in the frame it starts, which is the whole reason
    // this ladder has no "off" rung. 110ms is the shortest thing the game
    // animates, the letter landing in a tile.
    for (const speed of SPEEDS) expect(atSpeed(110, speed)).toBeGreaterThan(0)
  })

  it("cycles every rung and comes back", () => {
    // One button, so every rung has to be reachable from every other by tapping,
    // and a rung that is not on the cycle is a setting a player can leave and
    // never return to.
    const seen: Speed[] = []
    let speed: Speed = SPEEDS[0] as Speed
    for (let step = 0; step < SPEEDS.length; step++) {
      seen.push(speed)
      speed = NEXT_SPEED[speed]
    }
    expect(seen).toEqual([...SPEEDS])
    expect(speed).toBe(SPEEDS[0])
  })

  it("reads a stored setting back as itself", () => {
    for (const stored of SPEEDS) expect(readSpeed(String(stored))).toBe(stored)
  })

  it("reads anything else as normal speed", () => {
    // The list is what a store can actually hand back: nothing written yet, a
    // rung this build no longer offers, a number that is not one, and the string
    // that would divide a duration by zero if `Number` were trusted with it.
    for (const raw of [null, "", "1.5", "4", "0", "-1", "fast", "NaN", "[]"]) {
      expect(readSpeed(raw)).toBe(1)
    }
  })
})
