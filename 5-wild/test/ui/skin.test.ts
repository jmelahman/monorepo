import { readdirSync, readFileSync } from "node:fs"
import { afterEach, describe, expect, it } from "vitest"
import { DEFAULT_SKIN, loadSkin, NEXT_SKIN, readSkin, SKINS, skinClass } from "../../src/ui/skin"

/**
 * The table's skin setting. The class it writes is the whole interface to the
 * stylesheet, so what is worth pinning is the step from a stored string to one
 * of three names (a stale value must land on the default, never on nothing) and
 * the stylesheet's half of the bargain: geometry in one place, dressing in the
 * other.
 */
describe("skin", () => {
  afterEach(() => {
    Reflect.deleteProperty(globalThis, "localStorage")
  })

  it("reads a stored pick back as itself", () => {
    for (const skin of SKINS) expect(readSkin(skin)).toBe(skin)
  })

  it("reads absence and anything unknown as the default", () => {
    expect(DEFAULT_SKIN).toBe("smoke")
    for (const raw of [null, "", "Classic", "SMOKE", "casino", "felt", "null", "[]"]) {
      expect(readSkin(raw)).toBe(DEFAULT_SKIN)
    }
  })

  it("cycles through every skin and comes back", () => {
    let skin = SKINS[0] ?? DEFAULT_SKIN
    const seen = new Set<string>()
    for (let i = 0; i < SKINS.length; i++) {
      seen.add(skin)
      skin = NEXT_SKIN[skin]
    }
    expect(seen.size).toBe(SKINS.length)
    expect(skin).toBe(SKINS[0])
  })

  it("names one root class per skin", () => {
    expect(new Set(SKINS.map(skinClass)).size).toBe(SKINS.length)
    for (const skin of SKINS) expect(skinClass(skin)).toBe(`skin-${skin}`)
  })

  it("has the default when the store cannot be read", () => {
    Object.defineProperty(globalThis, "localStorage", {
      value: {
        getItem: () => {
          throw new Error("SecurityError")
        },
      },
      configurable: true,
    })
    expect(loadSkin()).toBe(DEFAULT_SKIN)
  })

  it("loads what was stored", () => {
    Object.defineProperty(globalThis, "localStorage", {
      value: { getItem: () => "classic" },
      configurable: true,
    })
    expect(loadSkin()).toBe("classic")
  })
})

/**
 * The stylesheet's half. The layout files carry no dressing, which is what lets
 * Classic be "the phone's look on the table's layout" by having nothing written
 * over it, and every rule in a skin's directory is under that skin's class,
 * which is what lets three dressings share a bundle. Both are easy to break by
 * pasting a rule into the wrong directory and only visible in a browser.
 */
describe("table stylesheet split", () => {
  const dir = "src/styles/table"
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
      if (file === "classic-skin.css") continue
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
    const skinned: Array<[string, string]> = [
      ...readdirSync(`${dir}/smoke`)
        .filter((f) => f.endsWith(".css") && f !== "index.css" && f !== "fonts.css")
        .map((f): [string, string] => [`${dir}/smoke/${f}`, "skin-smoke"]),
      [`${dir}/classic-skin.css`, "skin-classic"],
    ]
    for (const [file, cls] of skinned) {
      const css = strip(readFileSync(file, "utf8"))
        .replace(/@keyframes[^{]+\{(?:[^{}]*\{[^{}]*\})*[^{}]*\}/g, "")
        .replace(/@media[^{]+\{/g, "")
      const selectors = [...css.matchAll(/(^|\})\s*([^{}@]+)\{/g)].map((m) => (m[2] ?? "").trim())
      expect(selectors.length).toBeGreaterThan(0)
      for (const selector of selectors) {
        for (const part of topLevel(selector)) {
          expect(part.trim(), `${file}: ${part.trim()}`).toContain(`:root.table.${cls}`)
        }
      }
    }
  })
})
