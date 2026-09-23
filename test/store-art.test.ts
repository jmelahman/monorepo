import { readFileSync, statSync } from "node:fs"

import { describe, expect, it } from "vitest"

/*
 * The Play listing graphics are committed PNGs rendered by tools/gen-store-art.sh,
 * and these assertions are the reason they can be trusted without re-running it.
 *
 * Play's sizes are exact rather than minimums, and it enforces them at the
 * upload form, which is the last step of shipping, after the tag is pushed and
 * the release is cut. Finding out there that the feature graphic is 1024x512
 * costs a re-render and a second trip through a form that does not remember
 * what you already typed. Finding out here costs nothing.
 *
 * They also guard the subtler failure: someone edits assets/*.svg, runs
 * gen-icons.sh because that is the script they remember, and ships a listing
 * whose icon is the old mark. The dimensions would still pass, so the icon's
 * opacity check below carries that weight instead: a transparent corner means
 * the file came from icon.svg rather than icon-store.svg.
 */

/** Width and height out of a PNG's IHDR, which is always the first chunk. */
function pngSize(path: string): { width: number; height: number } {
  const buf = readFileSync(path)
  // 8-byte signature, 4-byte chunk length, 4-byte "IHDR", then the two uint32s.
  expect(buf.subarray(12, 16).toString("latin1")).toBe("IHDR")
  return { width: buf.readUInt32BE(16), height: buf.readUInt32BE(20) }
}

/** The PNG color type byte, also in IHDR. 6 is RGBA, 2 is RGB. */
function pngColorType(path: string): number {
  return readFileSync(path).readUInt8(25)
}

describe("play store art", () => {
  it("renders the listing icon at exactly 512x512", () => {
    expect(pngSize("assets/store/icon.png")).toEqual({ width: 512, height: 512 })
  })

  it("renders the feature graphic at exactly 1024x500", () => {
    expect(pngSize("assets/store/feature-graphic.png")).toEqual({ width: 1024, height: 500 })
  })

  it("paints the listing icon corner to corner", () => {
    /*
     * Play masks the icon itself, applying the rounding and the shadow to
     * suit whatever surface it is drawing on, so an upload with transparent
     * corners shows as a green shape with four notches bitten out of it. That
     * is exactly what assets/icon.svg would produce, since a legacy launcher
     * draws its square unmasked and needs the radius baked in. RGB rather than
     * RGBA is the cheap proof that the full-bleed source was the one rendered.
     */
    expect(pngColorType("assets/store/icon.png")).toBe(2)
  })

  it("keeps both files under Play's upload caps", () => {
    // 1MB for the icon, 15MB for the feature graphic. Flat color lands nowhere
    // near either, but a source that grew a photo or a filter would, quietly.
    expect(statSync("assets/store/icon.png").size).toBeLessThan(1024 * 1024)
    expect(statSync("assets/store/feature-graphic.png").size).toBeLessThan(15 * 1024 * 1024)
  })
})
