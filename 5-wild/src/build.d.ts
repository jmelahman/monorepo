/**
 * Stamped in by vite's `define` at build time; see vite.config.ts. Declared
 * rather than imported so the values are inlined as literals and nothing has to
 * be read at runtime.
 */
declare const __BUILD_VERSION__: string
/** Short hash, or "" where the build had neither GITHUB_SHA nor a git checkout. */
declare const __BUILD_COMMIT__: string

interface ImportMetaEnv {
  /** Where opted-in run replays are posted; see `src/ui/telemetry.ts`. Overrides `DEPLOYED` there; "" turns the feature off. */
  readonly VITE_TELEMETRY_URL?: string
}
