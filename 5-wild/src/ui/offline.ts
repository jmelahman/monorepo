import { Capacitor } from "@capacitor/core"

/**
 * Registering the service worker, which is what lets the site be played with no
 * network once it has been opened with one. What the worker does is `src/sw.js`;
 * this is only who gets it.
 *
 * Not the shells. The APK and the desktop build serve the whole bundle from
 * inside themselves, so there is nothing for a cache to add, and both are
 * origins a worker *would* register on: Capacitor's `https://localhost` and
 * Windows' `https://tauri.localhost` are secure contexts like any other. A
 * worker there would be a second copy of the game that outlives the update
 * that replaced the first. They are told apart the way `reportUrl` tells them.
 *
 * Not `bun run dev` either: the worker is written by the build (see
 * `tools/offline.ts`), so there is none to register, and one left over from a
 * `bun run preview` on the same port would answer for the dev server's files.
 */
export function installOffline(): void {
  if (!import.meta.env.PROD || !("serviceWorker" in navigator)) return
  if (Capacitor.isNativePlatform()) return
  if (location.protocol === "tauri:" || location.hostname === "tauri.localhost") return
  // After the page has loaded, because installing downloads every file the
  // game has, the recordings among them, and the first visit is the one launch
  // where that would be competing with the page for the connection.
  const register = (): void => {
    // Unanswered on purpose: without it the game is the site it was before.
    navigator.serviceWorker.register(`${import.meta.env.BASE_URL}sw.js`).catch(() => {})
  }
  if (document.readyState === "complete") register()
  else addEventListener("load", register, { once: true })
}
