import { existsSync, readdirSync, readFileSync } from "node:fs"
import { dirname, join, relative, resolve } from "node:path"
import { fileURLToPath } from "node:url"
import { describe, expect, it } from "vitest"

/**
 * That the benchmark can be loaded by the thing that loads it.
 *
 * Every other test here runs under vitest, which is Vite, and the benchmark's
 * entry points run under plain Bun (`bun tools/bench/mcp.ts`). The difference is
 * invisible until a module the bench reaches uses something only the bundler
 * supplies. `import.meta.glob` is the one this repo has: `emoji.ts` and
 * `audio.ts` fill their tables with it, Vite rewrites the call away at build
 * time, and at run time under Bun it is simply `undefined`. The bench took
 * `describeItem` from `views.ts`, `views.ts` took its pictures from `emoji.ts`,
 * and the MCP server died on its first import with every test green.
 *
 * So this walks the graph instead of running it: from each file in the two
 * bench directories, along every import that survives compilation, and fails on
 * the first file that calls the glob. Read as text, as `engine-purity.test.ts`
 * reads the engine, because the failure is precisely one that executing the
 * modules here cannot show.
 */

const here = dirname(fileURLToPath(import.meta.url))
const ROOT = resolve(here, "..", "..")
const ENTRIES = [join(ROOT, "src", "bench"), join(ROOT, "tools", "bench")]

/** Comments discuss the very call we ban, so they are not evidence. */
const stripComments = (source: string): string =>
  source.replace(/\/\*[\s\S]*?\*\//g, "").replace(/(^|[^:])\/\/.*$/gm, "$1")

const read = (file: string): string => stripComments(readFileSync(file, "utf8"))

/**
 * The relative imports of a file that exist at run time. `import type` is
 * erased, and it is most of how the bench touches the UI (`Step` from
 * `telemetry.ts`), so counting it would condemn files nothing ever loads.
 */
function imports(file: string): string[] {
  const found = [
    ...read(file).matchAll(/\b(?:import|export)\s+(?!type\b)[^"';]*?from\s+"(\.[^"]+)"/g),
  ]
  return found.flatMap((match) => {
    const target = resolve(dirname(file), match[1] ?? "")
    const hit = [`${target}.ts`, join(target, "index.ts"), target].find(
      (path) => path.endsWith(".ts") && existsSync(path),
    )
    return hit ? [hit] : []
  })
}

/** Every file reachable from `entry`, with the file that first led to it. */
function reach(entry: string): Map<string, string | null> {
  const seen = new Map<string, string | null>([[entry, null]])
  const queue = [entry]
  for (let file = queue.shift(); file; file = queue.shift()) {
    for (const next of imports(file)) {
      if (seen.has(next)) continue
      seen.set(next, file)
      queue.push(next)
    }
  }
  return seen
}

/** The chain of imports from the entry down to `file`, for the failure message. */
function trail(seen: Map<string, string | null>, file: string): string {
  const steps: string[] = []
  for (let at: string | null | undefined = file; at; at = seen.get(at)) {
    steps.unshift(relative(ROOT, at))
  }
  return steps.join(" → ")
}

const entries = ENTRIES.flatMap((dir) =>
  readdirSync(dir)
    .filter((name) => name.endsWith(".ts"))
    .map((name) => join(dir, name)),
)

describe("the bench loads without a bundler", () => {
  it("actually found the bench", () => {
    // Without this, a bad path would make the check below vacuously pass.
    expect(entries.length).toBeGreaterThan(8)
  })

  it("follows an import as far as the catalog", () => {
    // And without this, a regex that matched nothing would. `observe.ts` reads
    // its sentences from the language layer, three files down.
    const seen = reach(join(ROOT, "src", "bench", "observe.ts"))
    expect(seen.has(join(ROOT, "src", "ui", "lang", "en.ts"))).toBe(true)
  })

  it.each(entries.map((file) => [relative(ROOT, file), file]))(
    "%s reaches no import.meta.glob",
    (_label, entry) => {
      const seen = reach(entry)
      for (const file of seen.keys()) {
        expect(/\bimport\s*\.\s*meta\s*\.\s*glob\b/.test(read(file)), trail(seen, file)).toBe(false)
      }
    },
  )
})
