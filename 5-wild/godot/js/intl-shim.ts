/**
 * QuickJS has no `Intl`, and `src/ui/lang/en.ts` builds an `Intl.PluralRules`
 * the moment it loads. The golden replay needs that catalog for one sentence,
 * The Silence's note, so this gives it English and nothing else: `one` at
 * exactly 1, `other` everywhere else, which is CLDR's whole rule for `en`.
 *
 * Imported first by `golden.ts`, so it runs before the catalog does. The game's
 * own bundle has no catalog in it and needs none of this.
 */

class PluralRules {
  constructor(locale: string) {
    if (locale !== "en") throw new Error(`intl-shim: no plural rules for ${locale}`)
  }
  select(count: number): "one" | "other" {
    return count === 1 ? "one" : "other"
  }
}

const scope = globalThis as { Intl?: unknown }
scope.Intl ??= { PluralRules }
