import { readdirSync, readFileSync } from "node:fs"
import { describe, expect, it } from "vitest"
import {
  hasMovingBackground,
  NEXT_SKIN,
  readLegacyTone,
  readSkin,
  resolveSkin,
  SKINS,
  type Skin,
  skinClass,
  TONE,
} from "../../src/ui/skin"

/**
 * The look setting. The class it writes is the whole interface to the
 * stylesheet, and the tone it resolves to is the whole interface to everything
 * keyed on `data-theme`, so what is worth pinning is the step from what is
 * stored (and where the window is) to one of five names, and the stylesheet's
 * half of the bargain: geometry in one place, dressing in the other.
 */
describe("skin", () => {
  it("reads a stored pick back as itself", () => {
    for (const skin of SKINS) expect(readSkin(skin)).toBe(skin)
  })

  it("reads absence and anything unknown as no pick", () => {
    for (const raw of [null, "", "Classic", "SMOKE", "casino", "felt", "light", "null", "[]"]) {
      expect(readSkin(raw)).toBeNull()
    }
  })

  it("reads a legacy theme only if it was one of the two", () => {
    expect(readLegacyTone("light")).toBe("light")
    expect(readLegacyTone("dark")).toBe("dark")
    for (const raw of [null, "", "system", "Light", "auto", "sepia"]) {
      expect(readLegacyTone(raw)).toBeNull()
    }
  })

  it("cycles through every skin and comes back", () => {
    let skin = SKINS[0] ?? "smoke"
    const seen = new Set<string>()
    for (let i = 0; i < SKINS.length; i++) {
      seen.add(skin)
      skin = NEXT_SKIN[skin]
    }
    expect(seen.size).toBe(SKINS.length)
    expect(skin).toBe(SKINS[0])
  })

  it("names one root class per look, the two smokes sharing theirs", () => {
    expect(new Set(SKINS.map(skinClass)).size).toBe(SKINS.length - 1)
    expect(skinClass("smoke-moving")).toBe(skinClass("smoke"))
    for (const skin of SKINS) {
      if (skin !== "smoke-moving") expect(skinClass(skin)).toBe(`skin-${skin}`)
    }
  })

  it("is light only where the phone's light palette is meant", () => {
    expect(TONE["classic-light"]).toBe("light")
    for (const skin of SKINS) if (skin !== "classic-light") expect(TONE[skin]).toBe("dark")
  })

  it("moves the background only for the moving smoke", () => {
    expect(SKINS.filter(hasMovingBackground)).toEqual(["smoke-moving"])
  })

  describe("the default", () => {
    const stand = (over: Partial<Parameters<typeof resolveSkin>[0]>): Skin =>
      resolveSkin({ picked: null, legacy: null, table: false, prefersLight: false, ...over })

    it("is Classic following the device on a phone", () => {
      expect(stand({ prefersLight: false })).toBe("classic-dark")
      expect(stand({ prefersLight: true })).toBe("classic-light")
    })

    it("is the moving Smoke on the desktop, whatever the device says", () => {
      expect(stand({ table: true, prefersLight: false })).toBe("smoke-moving")
      expect(stand({ table: true, prefersLight: true })).toBe("smoke-moving")
    })

    it("moves an old light pick to Classic light on either layout", () => {
      for (const table of [false, true]) {
        for (const prefersLight of [false, true]) {
          expect(stand({ legacy: "light", table, prefersLight })).toBe("classic-light")
        }
      }
    })

    it("moves an old dark pick to the board on a phone and the table on a desktop", () => {
      for (const prefersLight of [false, true]) {
        expect(stand({ legacy: "dark", table: false, prefersLight })).toBe("classic-dark")
        expect(stand({ legacy: "dark", table: true, prefersLight })).toBe("smoke-moving")
      }
    })

    it("is overruled by a pick, on every layout and whatever the device says", () => {
      for (const picked of SKINS) {
        for (const table of [false, true]) {
          for (const legacy of [null, "light", "dark"] as const) {
            expect(stand({ picked, legacy, table, prefersLight: table })).toBe(picked)
          }
        }
      }
    })
  })
})

/**
 * The stylesheet's half. The layout files carry no dressing, which is what lets
 * Classic be "the phone's look on the table's layout" by having nothing written
 * over it, and every rule in a skin's directory is under that skin's class,
 * which is what lets four looks share a bundle. Both are easy to break by
 * pasting a rule into the wrong directory and only visible in a browser.
 */
describe("table stylesheet split", () => {
  const dir = "src/styles/table"
  const skins = "src/styles/skins"
  const layout = readdirSync(dir).filter((f) => f.endsWith(".css") && f !== "index.css")
  // Split a selector list on the commas that are not inside :is()/:not().
  const topLevel = (list: string): string[] => {
    const parts: string[] = []
    let depth = 0
    let from = 0
    for (let i = 0; i < list.length; i++) {
      const c = list[i]
      if (c === "(") depth++
      else if (c === ")") depth--
      else if (c === "," && depth === 0) {
        parts.push(list.slice(from, i))
        from = i + 1
      }
    }
    parts.push(list.slice(from))
    return parts
  }
  const strip = (css: string): string => css.replace(/\/\*[\s\S]*?\*\//g, "")

  it("keeps colour, face, shadow and motion out of the layout files", () => {
    const dressing =
      /^\s*(background(-[a-z]+)?|box-shadow|text-shadow|color|font-family|border(-[a-z]+)?|-webkit-text-stroke|animation(-[a-z]+)?|transition(-[a-z]+)?|filter)\s*:/m
    for (const file of layout) {
      // Keyframes are global names and stay with the layout by design.
      const css = strip(readFileSync(`${dir}/${file}`, "utf8")).replace(
        /@keyframes[^{]+\{(?:[^{}]*\{[^{}]*\})*[^{}]*\}/g,
        "",
      )
      const hit = css
        .split("\n")
        .find((line) => dressing.test(line) && !/border-width|border-top-width/.test(line))
      expect(hit, `${file} carries dressing`).toBeUndefined()
    }
  })

  it("writes every rule of a skin under its class", () => {
    const skinned: Array<[string, string[]]> = [
      ...(["smoke", "tabletop"] as const).flatMap((skin) =>
        readdirSync(`${skins}/${skin}`)
          .filter((f) => f.endsWith(".css") && f !== "index.css" && f !== "fonts.css")
          .map((f): [string, string[]] => [
            `${skins}/${skin}/${f}`,
            [`:root.skin-${skin}`, `:root.table.skin-${skin}`],
          ]),
      ),
      [
        `${skins}/classic.css`,
        [
          ":root:is(.skin-classic-dark, .skin-classic-light)",
          ":root.table:is(.skin-classic-dark, .skin-classic-light)",
        ],
      ],
    ]
    for (const [file, prefixes] of skinned) {
      const css = strip(readFileSync(file, "utf8"))
        .replace(/@keyframes[^{]+\{(?:[^{}]*\{[^{}]*\})*[^{}]*\}/g, "")
        .replace(/@media[^{]+\{/g, "")
      const selectors = [...css.matchAll(/(^|\})\s*([^{}@]+)\{/g)].map((m) => (m[2] ?? "").trim())
      expect(selectors.length).toBeGreaterThan(0)
      for (const selector of selectors) {
        for (const part of topLevel(selector)) {
          expect(
            prefixes.some((prefix) => part.trim().includes(prefix)),
            `${file}: ${part.trim()}`,
          ).toBe(true)
        }
      }
    }
  })
})
