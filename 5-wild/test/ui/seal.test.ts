import { describe, expect, it } from "vitest"
import { seal, unseal } from "../../src/ui/seal"

describe("the sealed save", () => {
  const plain = JSON.stringify({ seed: 42, round: { answer: "CRANE" }, note: "CAFÉ ñ" })

  it("round-trips, accents and all", () => {
    expect(unseal(seal(plain))).toBe(plain)
  })

  it("does not show the answer to someone reading storage", () => {
    const sealed = seal(plain)
    expect(sealed).not.toContain("CRANE")
    expect(sealed).not.toContain("answer")
  })

  it("never seals the same run to the same text twice", () => {
    // The nonce is what stops a player learning the answer's ciphertext by
    // saving the same round over and over.
    expect(seal(plain)).not.toBe(seal(plain))
  })

  it("passes a save from before sealing through untouched", () => {
    // Old builds wrote bare JSON, and a player mid-run at the upgrade keeps it.
    expect(unseal(plain)).toBe(plain)
  })

  it("refuses a sealed value that does not decode", () => {
    expect(() => unseal("s1.!!!")).toThrow()
  })

  it("survives a value too long to spread into fromCharCode", () => {
    const long = "x".repeat(300_000)
    expect(unseal(seal(long))).toBe(long)
  })
})
