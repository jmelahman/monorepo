import { describe, expect, it } from "vitest"
import capacitor from "../capacitor.config.json"
import config from "../vite.config"

/*
 * These assertions look pedantic, but each one guards a failure mode that
 * presents as "the APK installs and shows a blank screen" with no error
 * anywhere in the build logs.
 */
describe("vite config", () => {
  it("emits relative asset paths so Capacitor can resolve them", () => {
    // Capacitor serves the bundle from https://localhost/ inside a WebView.
    // Absolute "/assets/..." URLs resolve against the wrong root and 404.
    expect(config.base).toBe("./")
  })

  it("builds into the directory Capacitor copies from", () => {
    // A stale or missing webDir means `cap sync` ships the wrong bundle or none.
    expect(config.build?.outDir).toBe(capacitor.webDir)
  })

  it("keeps the Android install key", () => {
    // Changing appId later installs a second app beside the first rather than
    // upgrading it. Java package segments cannot start with a digit, so the id
    // spells the 5 out.
    expect(capacitor.appId).toBe("com.jmelahman.fivewild")
  })
})
