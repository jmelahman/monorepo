/**
 * The films `bench:record` made, put on YouTube.
 *
 *   bun tools/bench/upload.ts --login
 *   bun tools/bench/upload.ts <video.mp4>... [--privacy unlisted|private|public] [--dry]
 *
 * The videos are not source and do not belong in a repo, this one or the
 * blog's: a run is tens of megabytes and a suite is twenty of them per model.
 * So they live on somebody else's disk and the post embeds them by id.
 *
 * Nothing secret is kept in the checkout. The OAuth client and the refresh
 * token both sit in `~/.config/5wild-bench/`, which is outside every repo on
 * the machine rather than merely ignored by one, because an ignored file is
 * one `git add -f` or one moved directory from being published.
 *
 *   youtube-client.json   downloaded from Google Cloud Console: an OAuth
 *                         client of type "Desktop app", in a project with the
 *                         YouTube Data API v3 enabled
 *   youtube-token.json    written by `--login`
 *
 * No Google library. The whole conversation is three requests: a code for a
 * token, a token for an upload URL, and the bytes. `googleapis` is 100 MB of
 * generated clients to make them, and a dependency this tool would be the only
 * user of.
 *
 * The scope asked for is `youtube.upload` and nothing wider. That token can add
 * a video to the channel and cannot list, edit or delete what is already on it,
 * which is the right size for a file a script reads unattended.
 *
 * Unlisted unless told otherwise: an upload is the one thing in `tools/bench`
 * that leaves the machine, so the default is the one that can be embedded and
 * cannot be stumbled on. Two limits are Google's and worth knowing before
 * blaming this file. A Cloud project that has not passed the API audit has
 * every upload forced to private whatever is asked for here, and the fix is in
 * YouTube Studio by hand, or the audit. And an upload is charged against the
 * project's daily quota at a rate that, by default, allows only a handful a
 * day, so a suite's worth of films is several days or a quota request.
 *
 * `bench/videos/uploaded.json` is the ledger: file name to video id. It is what
 * makes a second run skip what the first one sent, since YouTube will happily
 * take the same film twice, and it is what the blog post reads its embeds
 * from. It is ignored with the rest of that directory.
 */

import { spawn } from "node:child_process"
import { chmodSync, existsSync, mkdirSync, readFileSync, statSync, writeFileSync } from "node:fs"
import { createServer } from "node:http"
import { homedir } from "node:os"
import { basename, dirname, join } from "node:path"
import type { Episode } from "../../src/bench/session"
import { RESULTS, ROOT } from "./host"

const log = (...parts: unknown[]) => console.error("[upload]", ...parts)

const args = process.argv.slice(2)
const flag = (name: string): string | undefined => {
  const at = args.indexOf(name)
  return at >= 0 ? args[at + 1] : undefined
}
const files = args.filter((arg, at) => !arg.startsWith("--") && args[at - 1] !== "--privacy")
const privacy = flag("--privacy") ?? "unlisted"
const dry = args.includes("--dry")
const login = args.includes("--login")

const HOME = join(homedir(), ".config", "5wild-bench")
const CLIENT = join(HOME, "youtube-client.json")
const TOKEN = join(HOME, "youtube-token.json")
const LEDGER = join(ROOT, "bench", "videos", "uploaded.json")
const SCOPE = "https://www.googleapis.com/auth/youtube.upload"

if ((!login && !files.length) || !["unlisted", "private", "public"].includes(privacy)) {
  console.error(
    "usage: bun tools/bench/upload.ts --login\n" +
      "       bun tools/bench/upload.ts <video.mp4>... [--privacy unlisted|private|public] [--dry]",
  )
  process.exit(2)
}

type Client = { client_id: string; client_secret: string }

function client(): Client {
  if (!existsSync(CLIENT)) {
    log(`no OAuth client at ${CLIENT}`)
    log("create one in Google Cloud Console (APIs & Services > Credentials > OAuth client ID,")
    log("type Desktop app), enable the YouTube Data API v3, and save the JSON it offers there")
    process.exit(1)
  }
  const file = JSON.parse(readFileSync(CLIENT, "utf8"))
  // The console wraps a desktop client in `installed`. A "Web application"
  // client is wrapped in `web` and will not do: it refuses a loopback redirect
  // on a port it was not told about, and the port here is whatever is free.
  const found = file.installed as Client | undefined
  if (!found?.client_id || !found.client_secret) {
    log(`${CLIENT} is not a Desktop app client (no "installed" block)`)
    process.exit(1)
  }
  return found
}

async function token(body: Record<string, string>): Promise<Record<string, unknown>> {
  const res = await fetch("https://oauth2.googleapis.com/token", {
    method: "POST",
    headers: { "content-type": "application/x-www-form-urlencoded" },
    body: new URLSearchParams(body),
  })
  const json = (await res.json()) as Record<string, unknown>
  if (!res.ok) throw new Error(`token: ${res.status} ${JSON.stringify(json)}`)
  return json
}

/**
 * The loopback flow: Google sends the browser back to a port on this machine
 * with the code in the query. It replaced the copy-a-code-from-the-page flow,
 * which Google switched off, so there is no version of this without a listener.
 *
 * `prompt=consent` is not politeness. Google hands out a refresh token only on
 * a consent it actually showed, so a second `--login` on an account that has
 * already agreed would come back with an access token good for an hour and
 * nothing to renew it with.
 */
async function authorize(): Promise<void> {
  const { client_id, client_secret } = client()
  const code = await new Promise<{ code: string; redirect: string }>((done, fail) => {
    const server = createServer((req, res) => {
      const url = new URL(req.url ?? "/", "http://127.0.0.1")
      const code = url.searchParams.get("code")
      const error = url.searchParams.get("error")
      if (!code && !error) {
        res.writeHead(404).end()
        return
      }
      res.writeHead(200, { "content-type": "text/plain" })
      res.end(code ? "Signed in. This tab can be closed." : `Refused: ${error}`)
      server.close()
      if (code) done({ code, redirect })
      else fail(new Error(`consent refused: ${error}`))
    })
    let redirect = ""
    server.listen(0, "127.0.0.1", () => {
      const address = server.address()
      const port = typeof address === "object" && address ? address.port : 0
      redirect = `http://127.0.0.1:${port}`
      const url =
        "https://accounts.google.com/o/oauth2/v2/auth?" +
        new URLSearchParams({
          client_id,
          redirect_uri: redirect,
          response_type: "code",
          scope: SCOPE,
          access_type: "offline",
          prompt: "consent",
        })
      log("open this and choose the channel's account:")
      console.error(`\n${url}\n`)
      // A convenience and nothing rests on it: on a machine with no opener the
      // URL above is still on the screen.
      spawn("xdg-open", [url], { stdio: "ignore", detached: true })
        .on("error", () => {})
        .unref()
    })
  })
  const got = await token({
    code: code.code,
    client_id,
    client_secret,
    redirect_uri: code.redirect,
    grant_type: "authorization_code",
  })
  if (typeof got.refresh_token !== "string") throw new Error("Google sent no refresh token")
  mkdirSync(HOME, { recursive: true })
  writeFileSync(TOKEN, `${JSON.stringify({ refresh_token: got.refresh_token }, null, 2)}\n`)
  chmodSync(TOKEN, 0o600)
  log(`signed in; token at ${TOKEN}`)
}

async function access(): Promise<string> {
  if (!existsSync(TOKEN)) {
    log("not signed in: bun tools/bench/upload.ts --login")
    process.exit(1)
  }
  const { client_id, client_secret } = client()
  const { refresh_token } = JSON.parse(readFileSync(TOKEN, "utf8")) as { refresh_token: string }
  const got = await token({ client_id, client_secret, refresh_token, grant_type: "refresh_token" })
  return got.access_token as string
}

/**
 * What the film is of, from the episode it was made from. `bench:record` names
 * a video after its episode, `<slug>-seed-N-aA`, so the name is the way back:
 * the label's own spelling (`gpt-6-luna / opencode-go`) and the result are in
 * the episode and nowhere in the file name. A video with no episode behind it
 * is refused rather than titled by guesswork, since the title is a claim about
 * a run.
 */
function describe(video: string): { title: string; description: string } | null {
  const match = /^(.+)-seed-(\d+)-a(\d+)\.(mp4|webm)$/.exec(basename(video))
  if (!match) return null
  const [, label = "", seed, ascension] = match
  const path = join(RESULTS, label, `seed-${seed}-a${ascension}.json`)
  if (!existsSync(path)) return null
  const episode = JSON.parse(readFileSync(path, "utf8")) as Episode
  const { result } = episode
  const model = episode.label.split(" / ")[0]
  const outcome = result.won ? "won the run" : `died in stage ${result.stage}`
  return {
    // YouTube's limit is 100 characters and no label here comes near it.
    title: `${model} plays 5 Wild: seed ${episode.seed}, ascension ${episode.ascension}`,
    description: [
      `${episode.label} playing 5 Wild, a roguelike word game, as a benchmark.`,
      "",
      `Seed ${episode.seed}, ascension ${episode.ascension}: ${outcome}, ` +
        `${result.roundsCleared} rounds cleared, score ${result.score.toLocaleString("en-US")}, ` +
        `${result.guesses} guesses.`,
      "",
      "The model plays through four text tools and sees only what the game tells it. " +
        "This is the real game replaying its moves at a faster animation speed, not a live capture.",
      "",
      `Build ${episode.commit}, content ${episode.content}.`,
    ].join("\n"),
  }
}

/**
 * A resumable upload used unresumed: one request for a session URL, one PUT of
 * the whole file, read into memory, which at 25 MB a film is no hardship. The
 * simpler multipart upload would have this file building a MIME body by hand
 * around those bytes, and the chunked form of this one is for files and
 * connections worse than these.
 */
async function send(video: string, bearer: string): Promise<string> {
  const about = describe(video)
  if (!about) throw new Error("no episode behind this name")
  const size = statSync(video).size
  const type = video.endsWith(".webm") ? "video/webm" : "video/mp4"
  const start = await fetch(
    "https://www.googleapis.com/upload/youtube/v3/videos?uploadType=resumable&part=snippet,status",
    {
      method: "POST",
      headers: {
        authorization: `Bearer ${bearer}`,
        "content-type": "application/json; charset=UTF-8",
        "x-upload-content-type": type,
        "x-upload-content-length": String(size),
      },
      body: JSON.stringify({
        // 20 is Gaming. The category is an id and not a name in this API.
        snippet: { ...about, categoryId: "20" },
        status: { privacyStatus: privacy, selfDeclaredMadeForKids: false },
      }),
    },
  )
  const session = start.headers.get("location")
  if (!start.ok || !session) throw new Error(`start: ${start.status} ${await start.text()}`)
  const put = await fetch(session, {
    method: "PUT",
    headers: { "content-type": type, "content-length": String(size) },
    body: readFileSync(video),
  })
  const json = (await put.json()) as { id?: string; status?: { privacyStatus?: string } }
  if (!put.ok || !json.id) throw new Error(`upload: ${put.status} ${JSON.stringify(json)}`)
  // Said out loud because it is silent otherwise: an unaudited project's upload
  // succeeds, as private, and the embed in the post is a grey box.
  if (json.status?.privacyStatus && json.status.privacyStatus !== privacy)
    log(`${basename(video)}: asked for ${privacy}, YouTube made it ${json.status.privacyStatus}`)
  return json.id
}

if (login) {
  await authorize()
  if (!files.length) process.exit(0)
}

const ledger: Record<string, string> = existsSync(LEDGER)
  ? JSON.parse(readFileSync(LEDGER, "utf8"))
  : {}
let bearer = ""
let failed = 0

for (const video of files) {
  const name = basename(video)
  if (ledger[name]) {
    log(`${name}: already up, https://youtu.be/${ledger[name]}`)
    continue
  }
  const about = describe(video)
  if (!about) {
    log(`${name}: no episode in bench/results for it; skipped`)
    failed++
    continue
  }
  if (dry) {
    console.error(`\n${name} (${privacy})\n  ${about.title}\n\n${about.description}\n`)
    continue
  }
  try {
    bearer ||= await access()
    const id = await send(video, bearer)
    ledger[name] = id
    // Written after each one and not at the end: the upload cannot be taken
    // back, so a crash on the third film must not forget the first two.
    mkdirSync(dirname(LEDGER), { recursive: true })
    writeFileSync(LEDGER, `${JSON.stringify(ledger, null, 2)}\n`)
    log(`${name}: https://youtu.be/${id}`)
  } catch (error) {
    log(`${name}: ${error instanceof Error ? error.message : error}`)
    failed++
  }
}

process.exit(failed ? 1 : 0)
