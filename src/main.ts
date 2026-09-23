import "./style.css"
import type { WordSource } from "./engine"
import { App, loadRunLang, loadSave } from "./ui/app"
import { type Lang, loadLang, S, setLang } from "./ui/lang"

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

const saved = loadSave()

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
const wordsLang = saved ? (loadRunLang() ?? "en") : lang

loadWords(wordsLang)
  .then((words) => {
    new App(app, words, saved, { lang: wordsLang, load: loadWords }).start()
  })
  .catch((error: unknown) => {
    app.replaceChildren()
    const note = document.createElement("div")
    note.className = "screen center"
    note.textContent = S().ui.error.words(String(error))
    app.append(note)
  })
