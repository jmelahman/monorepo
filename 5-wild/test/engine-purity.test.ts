import { readdirSync, readFileSync } from "node:fs"
import { dirname, join, relative, resolve } from "node:path"
import { fileURLToPath } from "node:url"
import { describe, expect, it } from "vitest"

/**
 * Choosing one language for the engine and the UI means we lose the boundary a
 * language split would have given us for free. This test buys it back.
 *
 * The engine is the layer worth protecting: it survives any presentation
 * rewrite, it is what the balance simulator runs headless, and it is what a
 * future port would have to reproduce. All of that holds only while it stays
 * pure, so purity is enforced by CI rather than by discipline.
 */

const here = dirname(fileURLToPath(import.meta.url))
const SRC = resolve(here, "..", "src")
const ENGINE = join(SRC, "engine")
const CONTENT = join(SRC, "content")

function walk(dir: string): string[] {
  return readdirSync(dir, { withFileTypes: true }).flatMap((entry) => {
    const path = join(dir, entry.name)
    if (entry.isDirectory()) return walk(path)
    return entry.name.endsWith(".ts") ? [path] : []
  })
}

/** Comments discuss the very globals we ban, so they are not evidence. */
const stripComments = (source: string): string =>
  source.replace(/\/\*[\s\S]*?\*\//g, "").replace(/(^|[^:])\/\/.*$/gm, "$1")

const engineFiles = walk(ENGINE)
const contentFiles = walk(CONTENT)

describe("engine purity", () => {
  it("actually found the engine", () => {
    // Without this, a bad path would make every check below vacuously pass.
    expect(engineFiles.length).toBeGreaterThan(5)
    expect(contentFiles.length).toBeGreaterThan(0)
  })

  const files = [...engineFiles, ...contentFiles]

  it.each(files.map((file) => [relative(SRC, file), file]))(
    "%s imports nothing outside the engine",
    (_label, file) => {
      const source = stripComments(readFileSync(file, "utf8"))
      const specifiers = [...source.matchAll(/\bfrom\s+"([^"]+)"|\bimport\("([^"]+)"\)/g)].map(
        (match) => match[1] ?? match[2] ?? "",
      )

      for (const specifier of specifiers) {
        // A bare specifier is an npm or Node builtin dependency. The engine has
        // none, and that is the point: it can be lifted into any host as-is.
        expect(specifier.startsWith("."), `${specifier} is an external dependency`).toBe(true)

        const target = resolve(dirname(file), specifier)
        const allowed = target.startsWith(ENGINE) || target.startsWith(CONTENT)
        expect(allowed, `${specifier} escapes the engine`).toBe(true)
      }
    },
  )

  // Each of these would make a run unreproducible, or tie the rules to a screen.
  const BANNED = [
    /\bMath\s*\.\s*random\b/,
    /\bDate\s*\.\s*now\b/,
    /\bnew\s+Date\b/,
    /\bperformance\s*\.\s*now\b/,
    /\bwindow\b/,
    /\bdocument\b/,
    /\blocalStorage\b/,
    /\bsessionStorage\b/,
    /\bfetch\s*\(/,
    /\bcrypto\b/,
  ]

  it.each(files.map((file) => [relative(SRC, file), file]))(
    "%s touches no ambient nondeterminism",
    (_label, file) => {
      const source = stripComments(readFileSync(file, "utf8"))
      for (const pattern of BANNED) {
        expect(pattern.test(source), `${pattern} is not allowed in the engine`).toBe(false)
      }
    },
  )
})
