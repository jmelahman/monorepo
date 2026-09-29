import { readFileSync } from "node:fs"
import { afterEach, describe, expect, it } from "vitest"
import { loadLegacyTone, loadSkin, SKINS, TONE } from "../../src/ui/skin"
import { BACKDROP } from "../../src/ui/theme"

/**
 * What the look's tone is keyed to. The setting itself is `skin.test.ts`; what
 * is checked here is the storage it reads (two keys, one of them retired) and
 * the tone's contract with the parts of the shell that live outside the page.
 */
describe("theme", () => {
  afterEach(() => {
    Reflect.deleteProperty(globalThis, "localStorage")
  })

  const store = (items: Record<string, string>): void => {
    Object.defineProperty(globalThis, "localStorage", {
      value: { getItem: (key: string) => items[key] ?? null },
      configurable: true,
    })
  }

  it("paints the page behind every look a tone has a colour for", () => {
    for (const skin of SKINS) expect(BACKDROP[TONE[skin]]).toMatch(/^#[0-9a-f]{6}$/)
  })

  it("launches Android on the same backdrop the page paints", () => {
    // The launch window and the WebView's own background are `launchBackground`,
    // one per tone, and both are on screen before the page exists. A pair that
    // disagrees with `BACKDROP` is a flash on every launch, and nothing but this
    // reads both files. The look reaches them as its tone (`ThemePlugin` is
    // handed `light` or `dark`), so four looks still have two launch windows.
    const res = "android/app/src/main/res"
    const launch = (dir: string): string =>
      readFileSync(`${res}/${dir}/colors.xml`, "utf8")
        .match(/name="launchBackground">(#[0-9A-Fa-f]{6})</)?.[1]
        ?.toLowerCase() ?? ""
    expect(launch("values")).toBe(BACKDROP.dark)
    expect(launch("values-notnight")).toBe(BACKDROP.light)
  })

  it("has no pick when the store cannot be read", () => {
    Object.defineProperty(globalThis, "localStorage", {
      value: {
        getItem: () => {
          throw new Error("SecurityError")
        },
      },
      configurable: true,
    })
    expect(loadSkin()).toBeNull()
    expect(loadLegacyTone()).toBeNull()
  })

  it("loads a pick from the skin key", () => {
    store({ "5wild:skin": "tabletop" })
    expect(loadSkin()).toBe("tabletop")
    expect(loadLegacyTone()).toBeNull()
  })

  it("still reads the theme key an earlier build wrote", () => {
    store({ "5wild:theme": "light" })
    expect(loadSkin()).toBeNull()
    expect(loadLegacyTone()).toBe("light")
  })
})
