import { describe, expect, it } from "vitest"
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
    // capacitor.config.ts webDir must match, or `cap sync` ships a stale bundle.
    expect(config.build?.outDir).toBe("dist")
  })
})
