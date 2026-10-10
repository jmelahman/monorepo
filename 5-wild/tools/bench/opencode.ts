/**
 * The suite, played by OpenCode, one model after another.
 *
 *   bun tools/bench/opencode.ts --models opencode-go/kimi-k3,opencode-go/glm-5.3
 *       [--runs <n>] [--parallel <n>] [--attempts <n>] [--timeout <minutes>] [--suite <path>]
 *
 * The models are spelled as `opencode models` prints them. Each is filed under
 * the label `<model> / <provider>`, so `opencode-go/kimi-k3` is
 * `kimi-k3 / opencode-go`: the provider is part of the name because Zen and Go
 * serve some of the same models, and the benchmark has no way to know they are
 * the same weights behind both.
 *
 * This is the one place the repo knows a harness by name, and it knows it only
 * as a command line. The host still brings everything that is scored; what
 * this adds is the part a person was doing by hand: open the harness, paste the
 * briefing, wait, and open it again when it stopped early.
 *
 * **Each model plays from an empty directory outside the repo**, with an
 * `opencode.json` that allows the `5wild` server and denies everything else. `.agents/skills/benchmark/SKILL.md` asks a model not to read the word
 * lists; this does not ask. The answer is one `grep` away from any agent whose
 * working directory is this checkout, and a benchmark that depends on forty
 * models all declining to look is measuring obedience as well as play. The same
 * skill file is still the briefing, read from here and sent as the message, so
 * a model driven by this script and one driven by hand are told the same thing.
 *
 * **A run is as many sessions as it takes.** A model that stops to summarise
 * at round 6, or runs out of context, has not finished, and the host has kept
 * the run (`in-progress.json`; see `LiveRun` in `host.ts`). So the harness is
 * started again with a fresh session and `new_run` hands the same run back.
 * `--attempts` bounds that, and a seed that is still unfinished after them
 * stops the label rather than being skipped: the suite cursor would deal the
 * same seed next time anyway, and a label that silently stalls on seed 4 and
 * reports sixteen runs is a worse table than one that says where it stopped.
 * A fresh session rather than `--continue`, because the usual reason a session
 * ended is that its context was full, and continuing it starts from the same
 * place.
 *
 * **Each model gets an OpenCode server of its own, started first.** Not the
 * background service, which is one process and would hand two models whichever
 * `opencode.json` it read first. And not `run --standalone`, which was the
 * first version and failed in a way worth recording: it boots a server, sends
 * the prompt and connects the MCP host all at once, the host takes about a
 * second to come up, and GPT Luna answers in less. It was told six times that
 * it had no tools and said so six times, in four seconds each, while a slower
 * model on the same code found them on its second try and played. So the
 * server is started, its directory is woken with a request that costs
 * nothing, and no prompt goes out until its log says the host is connected.
 *
 * No spectator feed: videos are made afterwards from the episodes, by
 * `tools/bench/record.ts`, which is the only way to get one at a watchable pace
 * from a model that takes forty seconds a move.
 *
 * Everything the harness printed is kept under `bench/logs/<label>/` as the
 * JSON events OpenCode emits, tokens and cost included. Nothing reads them
 * yet. They are kept because a suite is hours of somebody's quota and the
 * question "what did that cost" is always asked afterwards.
 */

import { type ChildProcess, spawn } from "node:child_process"
import { createWriteStream, existsSync, mkdirSync, readFileSync, writeFileSync } from "node:fs"
import { tmpdir } from "node:os"
import { join, resolve } from "node:path"
import { loadSuite, nextSeed, played, RESULTS, ROOT, slug } from "./host"

const log = (...parts: unknown[]) => console.error("[opencode]", ...parts)

const args = process.argv.slice(2)
const flag = (name: string): string | undefined => {
  const at = args.indexOf(name)
  return at >= 0 ? args[at + 1] : undefined
}

const suitePath = flag("--suite")
const suite = loadSuite(suitePath)
const models = (flag("--models") ?? "")
  .split(",")
  .map((model) => model.trim())
  .filter(Boolean)
const runs = Number(flag("--runs") ?? suite.seeds.length)
// Two, not one per model. Every model here is on one subscription, and what
// that allows at once has not been measured; two is the number that was run.
// Raise it when a rate limit has actually been seen not to bite.
const parallel = Math.max(1, Number(flag("--parallel") ?? 2))
const attempts = Math.max(1, Number(flag("--attempts") ?? 6))
const timeoutMs = Number(flag("--timeout") ?? 90) * 60_000

if (!models.length || models.some((model) => !model.includes("/"))) {
  console.error(
    "usage: bun tools/bench/opencode.ts --models <provider/model>[,...] " +
      "[--runs n] [--parallel n] [--attempts n] [--timeout minutes] [--suite path]\n" +
      "       models are spelled as `opencode models` lists them",
  )
  process.exit(2)
}

/** `opencode-go/kimi-k3` is the label `kimi-k3 / opencode-go`. */
const labelOf = (model: string): string => {
  const at = model.indexOf("/")
  return `${model.slice(at + 1)} / ${model.slice(0, at)}`
}

/** The skill, less its front matter, which is addressed to a skill loader and not to a player. */
const briefing = readFileSync(join(ROOT, ".agents/skills/benchmark/SKILL.md"), "utf8").replace(
  /^---\n[\s\S]*?\n---\n/,
  "",
)

const messageFor = (label: string): string =>
  [
    briefing.trim(),
    "",
    "---",
    "",
    `Your label is \`${label}\`. Pass exactly that to \`new_run\`.`,
    "Play one run to its end now. You have no other tools and nobody is watching to answer a question, " +
      "so do not ask one: keep calling `act` until a response contains `RUN ENDED`, then stop.",
    // Plumbing, not coaching. Under deny-by-default OpenCode's tool search
    // comes back empty although the calls work, and a model that searches
    // first concludes the server is missing and spends the session saying so.
    "In this harness the four tools are called from your `execute` tool, as " +
      '`await tools["5wild"].rules()`, `await tools["5wild"].new_run({ label })`, ' +
      '`await tools["5wild"].act({ command })` and `await tools["5wild"].observe()`. ' +
      "Call them that way directly; do not search for them first. If the first call answers " +
      "`Unknown tool`, make the same call again: the server takes a second to connect, and the " +
      "session can start before it has.",
  ].join("\n")

/**
 * The directory a model plays from: outside the repo, holding nothing but the
 * config. Deny by default and allow by name, rather than listing what to deny,
 * so a tool a later OpenCode adds is refused without anyone here hearing about
 * it. `external_directory` is named although `*` covers it because `--auto`
 * approves whatever is not *explicitly* denied, and that one is the way out of
 * an empty directory.
 *
 * What is allowed is `execute` and not the server, and that is OpenCode's
 * doing, worth knowing before reading a transcript: OpenCode 2 does not hand a
 * model MCP tools as tools. It hands it one tool that runs a script, in an
 * interpreter of its own with no `require`, no `process` and no `globalThis`,
 * and the server is an object inside it: `await tools["5wild"].act({command})`.
 * Both halves need allowing. Without `execute` there is nothing to call
 * the game from, and the model reports, accurately, that it has no tools;
 * without `5wild*` the script runs and finds `tools` empty. The second pattern
 * is spelled from what was seen to work, not from documentation. It also means a model may
 * put a loop around `act` and play a round in one call. That is the harness,
 * it is the same for every model on it, and it is why the label names it.
 */
function sandbox(label: string): string {
  const dir = join(tmpdir(), "5wild-bench", slug(label))
  mkdirSync(dir, { recursive: true })
  const host = ["bun", join(ROOT, "tools/bench/mcp.ts"), "--no-feed", "--label", label]
  if (suitePath) host.push("--suite", resolve(suitePath))
  const config = {
    mcp: { servers: { "5wild": { type: "local", command: host } } },
    permission: { "*": "deny", execute: "allow", "5wild*": "allow", external_directory: "deny" },
    share: "disabled",
    autoupdate: false,
  }
  writeFileSync(join(dir, "opencode.json"), `${JSON.stringify(config, null, 2)}\n`)
  return dir
}

const liveFile = (label: string) => join(RESULTS, slug(label), "in-progress.json")

// `PWD` as well as `cwd`, and the first run without it is why. OpenCode takes
// its directory from the variable, a child inherits the parent's, and so a
// session started from this checkout *was in this checkout*: no sandbox config,
// every tool allowed, `--auto` approving them, and the word lists a `grep`
// away. It showed only as a model that could not find the `5wild` server,
// which the repo's own config never declared.
const inside = (dir: string, extra: Record<string, string> = {}) => ({
  cwd: dir,
  env: { ...process.env, PWD: dir, ...extra },
})

type Server = { url: string; password: string; stop(): void }

/**
 * The model's own server, up and with the game attached, or null.
 *
 * Everything is read off the server's log because that is the only place any
 * of it is said: the address (the port is the system's choice, so two models
 * never ask for the same one), the password it makes up for itself, and the
 * line that says the host connected. A location is booted by the first request
 * that names its directory, so one is sent: listing sessions, which is free.
 */
function serve(label: string, dir: string, logPath: string): Promise<Server | null> {
  return new Promise((done) => {
    const out = createWriteStream(logPath)
    const child = spawn(
      "opencode",
      ["serve", "--port", "0", "--print-logs", "--log-level", "info"],
      {
        ...inside(dir),
        stdio: ["ignore", "pipe", "pipe"],
      },
    )
    let seen = ""
    let woken = false
    let settled = false
    const settle = (server: Server | null, note?: string) => {
      if (settled) return
      settled = true
      clearTimeout(timer)
      if (note) log(`${label}: ${note}`)
      if (!server) child.kill()
      done(server)
    }
    const timer = setTimeout(() => settle(null, "the 5wild server never connected"), 60_000)
    const read = (chunk: Buffer) => {
      out.write(chunk)
      if (settled) return
      seen += String(chunk)
      const url = /server listening on (\S+)/.exec(seen)?.[1]
      const password = /server password (\S+)/.exec(seen)?.[1]
      if (!url || !password) return
      if (!woken) {
        woken = true
        spawn("opencode", ["session", "list", "--server", url], {
          ...inside(dir, { OPENCODE_PASSWORD: password }),
          stdio: "ignore",
        }).on("error", () => {})
      }
      if (/mcp connected.*server=5wild/.test(seen)) {
        settle({ url, password, stop: () => child.kill() })
      }
    }
    child.stdout.on("data", read)
    child.stderr.on("data", read)
    child.on("error", (error) => settle(null, error.message))
    child.on("close", () => settle(null, "the OpenCode server stopped before it was ready"))
  })
}

/**
 * One session: hand the briefing to the model's server and wait for it to
 * stop. `node:child_process` rather than `Bun.spawn`, for the reason the feed
 * gives for `node:http`: nothing in `tools/bench` needs a particular runtime yet.
 */
function session(model: string, label: string, dir: string, server: Server, logPath: string) {
  return new Promise<void>((done) => {
    const out = createWriteStream(logPath)
    const child = spawn(
      "opencode",
      ["run", "--server", server.url, "--auto", "--format", "json", "-m", model, messageFor(label)],
      { ...inside(dir, { OPENCODE_PASSWORD: server.password }), stdio: ["ignore", "pipe", "pipe"] },
    )
    sessions.add(child)
    child.on("exit", () => sessions.delete(child))
    const timer = setTimeout(() => {
      log(`${label}: no end after ${timeoutMs / 60_000} minutes, stopping the session`)
      child.kill()
    }, timeoutMs)
    child.stdout.pipe(out, { end: false })
    // The one event worth saying out loud. A provider that refuses the model
    // (no credit, a free tier closed to `run`) ends the session in two seconds,
    // and without this the only sign is a label that used up its attempts.
    let said = false
    child.stdout.on("data", (chunk: Buffer) => {
      const error = said ? null : /"type":"error".*?"message":"([^"]*)"/.exec(String(chunk))
      if (!error) return
      said = true
      log(`${label}: ${error[1]}`)
    })
    child.stderr.pipe(out, { end: false })
    // `error` as well as `close`: a missing `opencode` binary never closes, and
    // would otherwise leave the label waiting out the whole timeout to say so.
    const finish = (note?: string) => {
      clearTimeout(timer)
      if (note) log(`${label}: ${note}`)
      out.end(() => done())
    }
    child.on("error", (error) => finish(error.message))
    child.on("close", () => finish())
  })
}

/** One model, through as many of its remaining suite seeds as `--runs` allows. */
async function play(model: string): Promise<void> {
  const label = labelOf(model)
  const dir = sandbox(label)
  const logs = join(ROOT, "bench", "logs", slug(label))
  mkdirSync(logs, { recursive: true })
  const server = await serve(label, dir, join(logs, "server.log"))
  if (!server) return
  servers.add(server)

  try {
    for (let run = 0; run < runs; run++) {
      const seed = nextSeed(suite, label)
      if (seed === null) {
        log(`${label}: the suite is complete`)
        return
      }
      let done = false
      for (let attempt = 1; attempt <= attempts && !done; attempt++) {
        const resumed = existsSync(liveFile(label))
        log(`${label}: seed ${seed}, session ${attempt}${resumed ? " (resuming)" : ""}`)
        const stamp = new Date().toISOString().replace(/[:.]/g, "-")
        await session(model, label, dir, server, join(logs, `seed-${seed}-${stamp}.jsonl`))
        done = played(label, seed, suite.ascension)
      }
      if (!done) {
        log(`${label}: seed ${seed} unfinished after ${attempts} sessions; stopping this label`)
        return
      }
      log(`${label}: seed ${seed} filed`)
    }
  } finally {
    server.stop()
    servers.delete(server)
  }
}

// A server outlives the script that started it unless it is told otherwise,
// and each one holds a game host. Ctrl-C on a suite should leave neither. Nor
// the sessions: a client whose server has gone is not a client that stops, and
// one left behind goes on spending the plan's quota on a run nobody will file.
const servers = new Set<Server>()
const sessions = new Set<ChildProcess>()
for (const signal of ["SIGINT", "SIGTERM"] as const) {
  process.on(signal, () => {
    for (const child of sessions) child.kill()
    for (const server of servers) server.stop()
    process.exit(130)
  })
}

// A pool rather than `Promise.all` over everything, for the reason at
// `parallel`. Models are taken in the order given, so the ones named first are
// the ones with results if the evening ends early.
const waiting = [...models]
await Promise.all(
  Array.from({ length: Math.min(parallel, waiting.length) }, async () => {
    for (let model = waiting.shift(); model; model = waiting.shift()) await play(model)
  }),
)
log("done. `bun run bench:report` for the table.")
