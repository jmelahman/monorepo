/**
 * The worker's two answers to a resend, held to the real `schema.sql`.
 *
 * On 2026-09-30 one run arrived ten times. The worker allowed five origins, the
 * client posts `text/plain` so no preflight ever refused anyone, and a page on
 * a sixth origin had its run filed and was then denied the header that let it
 * read the 204. It kept the run and sent it again on every launch. The fix is
 * two halves and either one alone would have ended the loop, so each is pinned
 * separately: any origin can read the answer, and a run the table already
 * holds is answered 204 rather than failing the unique index with a 500 the
 * client would retry forever.
 *
 * It runs under the root `vitest` (so `bun run test` covers it) but lives here
 * rather than in `test/`, because the root `tsc` has no Workers types and
 * would trip over `D1Database` in the file it imports. D1 is SQLite, so
 * `node:sqlite` stands in for it: same dialect, same `json_extract`, same
 * `INSERT OR IGNORE`.
 */

import { readFileSync } from "node:fs"
import { dirname, join } from "node:path"
import { DatabaseSync } from "node:sqlite"
import { fileURLToPath } from "node:url"
import { beforeEach, describe, expect, it } from "vitest"
import worker from "../src/index"

const SCHEMA = readFileSync(join(dirname(fileURLToPath(import.meta.url)), "../schema.sql"), "utf8")

/** Just the surface the worker touches: `prepare(sql).bind(...).run()`. */
function d1(db: DatabaseSync) {
  return {
    prepare: (sql: string) => ({
      bind: (...values: (string | number)[]) => ({
        run: async () => db.prepare(sql).run(...values),
      }),
    }),
  }
}

const run = (overrides: Record<string, unknown> = {}) => ({
  v: 1,
  content: 1,
  build: "0.9.0",
  commit: "b9b33007",
  words: "en",
  seed: 123456789,
  ascension: 0,
  nth: 1,
  end: "lost",
  won: false,
  stage: 1,
  round: 0,
  steps: [{ type: "guess", word: "crane" }],
  ...overrides,
})

let db: DatabaseSync
let env: Parameters<typeof worker.fetch>[1]

const post = (body: unknown, origin?: string) =>
  worker.fetch(
    new Request("https://telemetry.example/runs", {
      method: "POST",
      headers: { "content-type": "text/plain", ...(origin ? { origin } : {}) },
      body: JSON.stringify(body),
    }),
    env,
  )

const rows = () => (db.prepare("SELECT COUNT(*) AS n FROM runs").get() as { n: number }).n

beforeEach(() => {
  db = new DatabaseSync(":memory:")
  db.exec(SCHEMA)
  env = {
    DB: d1(db),
    LIMITER: { limit: async () => ({ success: true }) },
  } as unknown as typeof env
})

describe("telemetry worker", () => {
  // The origins are the ones that matter: the site, and the three that were
  // missing or could be (a Capacitor scheme change, an opaque `null` from a
  // sandboxed frame, no header at all).
  it.each([
    "https://5-wild.com",
    "capacitor://localhost",
    "https://some-mirror.example",
    "null",
    undefined,
  ])("lets a page on %s read the answer", async (origin) => {
    const response = await post(run(), origin)
    expect(response.status).toBe(204)
    expect(response.headers.get("access-control-allow-origin")).toBe("*")
  })

  it("answers refusals readably too, so the client can tell them from a dead connection", async () => {
    const response = await post({ v: 2 }, "https://some-mirror.example")
    expect(response.status).toBe(400)
    expect(response.headers.get("access-control-allow-origin")).toBe("*")
  })

  it("files a resent run once and answers every copy 204", async () => {
    for (let i = 0; i < 10; i++) expect((await post(run())).status).toBe(204)
    expect(rows()).toBe(1)
  })

  it("keeps the first copy when a resend differs from it", async () => {
    // A retried send is byte for byte the same, but the index is on the run's
    // identity rather than its bytes, so this pins which one survives.
    await post(run({ stage: 1 }))
    await post(run({ stage: 2 }))
    expect(db.prepare("SELECT stage FROM runs").all()).toEqual([{ stage: 1 }])
  })

  it.each([
    ["seed", { seed: 987654321 }],
    ["place in the count", { nth: 2 }],
    ["build", { commit: "4e4abfbc" }],
  ])("files two runs that differ only in %s as two", async (_, overrides) => {
    await post(run())
    await post(run(overrides))
    expect(rows()).toBe(2)
  })

  it("migrates a table the resend loop already filled", () => {
    // The table as it stood before the index: ten copies of one run and one
    // other. Running the schema again has to clear the copies before it can
    // build the index, and has to be a no-op the time after.
    db.exec("DROP INDEX runs_once")
    const insert = db.prepare(
      `INSERT INTO runs (day, v, content, build, build_commit, words, ascension, nth, ended, won, stage, round, payload)
       VALUES ('2026-09-30', 1, 1, '0.9.0', ?, 'en', 0, ?, 'lost', 0, 1, 0, ?)`,
    )
    for (let i = 0; i < 10; i++) insert.run("b9b33007", 1, JSON.stringify(run()))
    insert.run("b9b33007", 2, JSON.stringify(run({ nth: 2 })))

    db.exec(SCHEMA)
    expect(rows()).toBe(2)
    expect(db.prepare("SELECT MIN(id) AS id FROM runs WHERE nth = 1").get()).toEqual({ id: 1 })

    db.exec(SCHEMA)
    expect(rows()).toBe(2)
  })
})
