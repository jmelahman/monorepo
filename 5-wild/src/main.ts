import "./style.css"
import type { WordSource } from "./engine"
import { App, loadRunLang, loadSave } from "./ui/app"
import { startBackground } from "./ui/fx/background"
import { type Lang, loadLang, S, setLang } from "./ui/lang"
import { installOffline } from "./ui/offline"
import { readSeedParam } from "./ui/seed"
import { spectate } from "./ui/spectate"
import { initLook } from "./ui/theme"

/**
 * The shell. Its whole job is to load the two things the engine refuses to
 * load itself, the word lists, and hand them to a pure state machine.
 *
 * It picks the language too, which is the same job wearing a different hat: the
 * language decides *which* lists, so the choice has to be made out here, before
 * the fetch, rather than inside the app that the fetch is being done for.
 */

const app = document.querySelector<HTMLDivElement>("#app")
if (!app) throw new Error("#app missing from index.html")

const loadWords = async (lang: Lang): Promise<WordSource> => {
  const base = import.meta.env.BASE_URL
  const read = async (name: string): Promise<string[]> => {
    const response = await fetch(`${base}words/${lang}/${name}.txt`)
    if (!response.ok) throw new Error(`${lang}/${name}.txt: ${response.status}`)
    return (await response.text()).split("\n").filter(Boolean)
  }
  const [answers, allowed] = await Promise.all([read("answers"), read("allowed")])
  return { answers, allowed: new Set(allowed) }
}

// Before the fetch, because the failure screen below is written in it, and
// before the render, because every string on every screen reads it.
const lang = loadLang()
setLang(lang)

// Before the fetch, because the page is painted while it runs, and a light
// player shown a dark page for the length of a download has seen a flash. One
// call for the layout, the look and its tone, since which look is the default
// depends on the layout and the tone depends on the look; applied and not
// written, as nothing has been chosen yet. It also sets the look's faces
// downloading, beside the word list rather than behind it.
initLook()

// After the look, because the background follows the skin class it sets, and
// creates nothing unless that look has a moving one.
startBackground()

// Nothing below waits on it: it registers once the page has loaded, and a
// browser that refuses is a browser that plays the site as it always did.
installOffline()

/**
 * `?watch=<host>` opens the game as a spectator on the benchmark feed rather
 * than as the player's own. See `src/ui/spectate.ts`.
 *
 * Decided before the save is read, because a spectator does not resume one:
 * it is handed a run by the host, and the player's own run stays where it is,
 * untouched, for the next time this page is opened without the parameter. The
 * host deals in English, so that is the list it starts with.
 */
const watching = new URLSearchParams(location.search).get("watch")

const saved = watching ? null : loadSave()

/**
 * `?seed=<code>` is someone's run, shared from their pause sheet or end screen.
 * See `src/ui/seed.ts`. It opens the seed sheet rather than dealing the run,
 * because the player may have one of their own in hand. Taken off the address
 * once read, so a reload is the player's game again and not the offer twice;
 * the sheet stays up until they answer it.
 */
// A code that does not read (a link cut short in a chat, a hand-typed typo)
// still opens the sheet, holding what arrived: landing on the title with no
// word about the link would read as the link being ignored, and the field is
// where it can be put right.
const shared = watching ? null : new URLSearchParams(location.search).get("seed")
const offer = shared === null ? null : (readSeedParam(location.search) ?? shared)
if (offer !== null) {
  const url = new URL(location.href)
  for (const key of ["seed", "a", "w"]) url.searchParams.delete(key)
  history.replaceState(history.state, "", url)
}

/**
 * Which list the game opens with, which is not always the language it opens in.
 *
 * A run is dealt from one word list and keeps it for its whole life, so a save
 * made in English stays English however the setting has moved since. `newRun` is
 * where the two get back in step. See `wordsDeferred` in `app.ts`.
 *
 * A save with no language beside it is a save from before this key existed, and
 * that is not a guess: English was the only list there was.
 */
const wordsLang = watching ? "en" : saved ? (loadRunLang() ?? "en") : lang

loadWords(wordsLang)
  .then((words) => {
    const game = new App(app, words, saved, { lang: wordsLang, load: loadWords })
    if (watching) spectate(game, watching)
    else {
      if (offer !== null) game.offerSeed(offer)
      game.start()
    }
  })
  .catch((error: unknown) => {
    app.replaceChildren()
    const note = document.createElement("div")
    note.className = "screen center"
    note.textContent = S().ui.error.words(String(error))
    app.append(note)
  })
