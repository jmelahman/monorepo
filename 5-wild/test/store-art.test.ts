import { readFileSync, statSync } from "node:fs"

import { describe, expect, it } from "vitest"

import { BACKDROP } from "../src/ui/theme"

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

describe("link preview art", () => {
  /*
   * Rendered by the same script into public/, and pinned for a quieter reason
   * than Play's: index.html states og.png's size in og:image:width and height,
   * and a preview reader that trusts those lays the card out before the image
   * arrives, so a re-render at another size is a card drawn at the wrong shape.
   */
  it("renders og.png at the size index.html declares", () => {
    const html = readFileSync("index.html", "utf8")
    const declared = (prop: string) =>
      Number(html.match(new RegExp(`property="og:image:${prop}" content="(\\d+)"`))?.[1])
    expect(pngSize("public/og.png")).toEqual({
      width: declared("width"),
      height: declared("height"),
    })
  })

  it("renders the touch icon at 180x180, opaque", () => {
    // iOS rounds it itself, the same as Play, so the full-bleed mark is right.
    expect(pngSize("public/apple-touch-icon.png")).toEqual({ width: 180, height: 180 })
    expect(pngColorType("public/apple-touch-icon.png")).toBe(2)
  })
})

describe("install art", () => {
  /*
   * public/manifest.webmanifest is what makes Chrome offer to install the site,
   * and every way of getting it wrong is silent: the menu entry is simply not
   * there, on a phone, with the reason three screens deep in a desktop
   * devtools panel. So the conditions Chrome checks are checked here.
   */
  const manifest = JSON.parse(readFileSync("public/manifest.webmanifest", "utf8")) as {
    start_url: string
    scope: string
    display: string
    background_color: string
    theme_color: string
    icons: { src: string; sizes: string; purpose: string }[]
  }

  it("is linked from the page", () => {
    expect(readFileSync("index.html", "utf8")).toContain(
      '<link rel="manifest" href="./manifest.webmanifest" />',
    )
  })

  it("opens in a window of its own", () => {
    // `browser`, the default, is the one value that is not installable.
    expect(manifest.display).toBe("standalone")
  })

  it("names nothing by an absolute path", () => {
    // The same reason vite's `base` is "./": a path from the root is right at
    // 5-wild.com and wrong anywhere the bundle is served from a directory.
    // These resolve against the manifest's own URL, which sits beside the page.
    expect(manifest.start_url).toBe("./")
    expect(manifest.scope).toBe("./")
    // And no `id`, which is the one member that cannot be relative: it is read
    // against the origin whatever it says, so "./" there is the root again.
    // Left out, the install is identified by `start_url`, which is right.
    expect(manifest).not.toHaveProperty("id")
    for (const icon of manifest.icons) expect(icon.src).not.toMatch(/^\/|:/)
  })

  it("has every icon it lists, at the size it lists", () => {
    for (const icon of manifest.icons) {
      const [width, height] = icon.sizes.split("x").map(Number)
      expect(pngSize(`public/${icon.src}`)).toEqual({ width, height })
    }
  })

  it("lists the two sizes Chrome will not install without", () => {
    const any = manifest.icons.filter((icon) => icon.purpose === "any").map((icon) => icon.sizes)
    expect(any).toContain("192x192")
    expect(any).toContain("512x512")
  })

  it("hands a masking launcher the full-bleed mark and no other", () => {
    // The same proof as the listing icon's, in both directions: a maskable icon
    // with clear corners is cut to a shape with notches in it, and a full-bleed
    // one drawn unmasked is a square tile among rounded ones.
    for (const icon of manifest.icons) {
      expect(pngColorType(`public/${icon.src}`)).toBe(icon.purpose === "maskable" ? 2 : 6)
    }
  })

  it("launches on the backdrop the page paints", () => {
    // The splash an installed site opens on is `background_color`, drawn before
    // the page exists, the same position Android's `launchBackground` is in.
    // A manifest holds one colour where the game has two tones, so a light
    // player gets one dark frame; it is the dark one because three of the four
    // looks are. `theme_color` only lasts until index.html's own meta is read.
    expect(manifest.background_color).toBe(BACKDROP.dark)
    expect(manifest.theme_color).toBe(BACKDROP.dark)
  })
})
