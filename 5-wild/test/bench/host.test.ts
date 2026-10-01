import { relative, resolve } from "node:path"
import { describe, expect, it } from "vitest"
import { RESULTS, slug } from "../../tools/bench/host"

describe("a label's directory", () => {
  it("keeps the readable labels it always had", () => {
    expect(slug("claude-opus-5-5 via claude code, text")).toBe(
      "claude-opus-5-5-via-claude-code-text",
    )
    expect(slug("sonnet-4.5")).toBe("sonnet-4.5")
  })

  // Every episode, ad-hoc run and checkpoint path is `RESULTS/<slug>/...`, so
  // the slug is the one segment a harness controls.
  it.each([".", "..", "...", "../..", "../etc", ".hidden", "", "   ", "日本語", "a/../../b"])(
    "keeps %j inside bench/results",
    (label) => {
      const dir = resolve(RESULTS, slug(label))
      const inside = relative(RESULTS, dir)
      expect(inside).not.toBe("")
      expect(inside.startsWith("..")).toBe(false)
      expect(inside).not.toMatch(/[/\\]/)
    },
  )
})
