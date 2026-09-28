import { readFileSync } from "node:fs"
import { afterEach, describe, expect, it } from "vitest"
import {
  BACKDROP,
  loadTheme,
  OTHER_THEME,
  readTheme,
  resolveTheme,
  THEMES,
} from "../../src/ui/theme"

/**
 * The theme setting, which is a look the player picked or, until they pick,
 * none at all, in which case the device decides. So the part worth checking is
 * the step between them, where a pick or its absence becomes `light` or `dark`.
 */
describe("theme", () => {
  afterEach(() => {
    Reflect.deleteProperty(globalThis, "localStorage")
  })

  it("starts with no pick", () => {
    expect(readTheme(null)).toBeNull()
  })

  it("toggles between the two looks", () => {
    for (const theme of THEMES) {
      expect(OTHER_THEME[theme]).not.toBe(theme)
      expect(OTHER_THEME[OTHER_THEME[theme]]).toBe(theme)
    }
  })

  it("reads a stored pick back as itself", () => {
    for (const stored of THEMES) expect(readTheme(stored)).toBe(stored)
  })

  it("reads anything else as no pick", () => {
    for (const raw of ["", "system", "Light", "DARK", "auto", "sepia", "null", "[]"]) {
      expect(readTheme(raw)).toBeNull()
    }
  })

  it("asks the device only when nothing was picked", () => {
    expect(resolveTheme(null, true)).toBe("light")
    expect(resolveTheme(null, false)).toBe("dark")
    // A player who picked one is not overruled by the phone.
    for (const prefersLight of [true, false]) {
      expect(resolveTheme("light", prefersLight)).toBe("light")
      expect(resolveTheme("dark", prefersLight)).toBe("dark")
    }
  })

  it("launches Android on the same backdrop the page paints", () => {
    // The launch window and the WebView's own background are `launchBackground`,
    // one per theme, and both are on screen before the page exists. A pair that
    // disagrees with `BACKDROP` is a flash on every launch, and nothing but this
    // reads both files.
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
    expect(loadTheme()).toBeNull()
  })

  it("loads what was stored", () => {
    Object.defineProperty(globalThis, "localStorage", {
      value: { getItem: (key: string) => (key === "5wild:theme" ? "light" : null) },
      configurable: true,
    })
    expect(loadTheme()).toBe("light")
  })
})
