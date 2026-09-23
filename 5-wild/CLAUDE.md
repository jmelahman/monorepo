# CLAUDE.md

`CONTRIBUTING.md` covers running it, the layout, the release flow and the
golden-vector protocol. Read that first. This is what it does not say. (The
README is the game's front page and deliberately holds no technical detail
beyond how to install it.)

## The shape of the thing

`src/engine` is a pure reducer: `reduce(state, action, words)` returns a new
state and a list of events. It imports only itself and `src/content`, and
`test/engine-purity.test.ts` enforces that rather than trusting discipline. All
randomness goes through `derive(seed, ...coords)` in `rng.ts`: never
`Math.random`, never a clock.

`RunState` is plain JSON. It round-trips through `JSON.parse(JSON.stringify(…))`
and a test asserts it, so no Maps, Sets, Dates or class instances may enter it.
Saves and golden vectors both rest on that.

A run is a closed world. Anything outliving one belongs in `src/ui/meta.ts`, not
the engine. Ascension is the clear case: it is an *input* to `startRun`, the UI
remembers which level was chosen, and the engine never learns a browser exists.

**The engine authors no prose.** It used to author about a third of the game's:
every relic, boss, modifier, etching, consumable, pack, ascension and category
carried its own `name` and `text`, and all 44 `reject(…)` calls in `reduce.ts`
wrote English sentences that reached the player verbatim. Those are gone. A table
carries its `id` and its numbers; `reject` takes a `Refusal`, which is a code plus
whatever operands the sentence will need; and every string lives in
`src/ui/lang/`, keyed by that id or that code. The seam is not a style preference:
`test/engine-purity.test.ts` lets engine files import only from `src/engine` and
`src/content`, so a catalog that knew about locale could not live in either
without making the engine language-aware and a run's prose part of run state.

## Language

`src/ui/lang/types.ts` is the contract and `en.ts` is the reference filling of
it. A new language is one module of type `Strings` and nothing else: if it
compiles it is complete, because a missing key is a build error rather than an
`undefined` on a screen. Anything a translator would otherwise re-type out of the
balance tables is a parameter instead, so a nerf never has to be chased through
four files.

The catalog in force is a module-level value in `lang/index.ts`, not an argument
threaded through the views, and that is safe for the same reason everything else
here is: every dispatch rebuilds the whole screen, so there is never a
half-rendered tree holding strings from before a switch. `current` is the prose;
`currentLang` beside it is the *choice*, which is a different question, because
three of the four languages fall back to the English catalog until their
translations land — the keyboard layout has to read the choice, not the prose.

Two things that look like notation are prose: the abbreviation ladder
(`10^9` is a billion in English and a milliard in French) and the keyboard
(QWERTY, AZERTY, QWERTZ). Both live in the language layer. The ladder is *pushed*
into `format.ts` by `useLang` rather than read from it, because every catalog
imports `formatNumber` to build its own sentences and a read back would be a
cycle.

Accents fold to base letters everywhere — CAFÉ is CAFE, ñ is n — which is what
keeps the alphabet at 26 and leaves every letter-indexed system (etchings,
ranges, modifiers, `MIN_LIVE_LETTERS`, the keyboard) working untouched.

## The UI

No framework. `h()` in `src/ui/dom.ts` builds DOM, `views.ts` is pure
`state → HTMLElement`, `app.ts` owns dispatch.

**Every dispatch rebuilds the whole screen**, via `clear(this.root).append(view)`.
Two things follow: view code can assume it builds from scratch, and any state the
browser owns (focus, scroll position, an open `<details>`) is destroyed on
every render and has to be restored deliberately. `holdFocus()` is that
restoration for open sheets.

Two exceptions, both bought by the same defect: `.grid` sizes itself in
container-query units, and Gecko cannot resolve those against a container built
in the same pass, so a board that is rebuilt is a board that flashes at the wrong
size for a frame. `patchDraft` redraws the row being typed in place, and
`reuseBoard` keeps `.grid-wrap`, the container itself, across a round-to-round
render, splicing the new screen in around it. Neither reaches the views: the
reused node is an empty box, and everything drawn inside it is still built from
scratch.

The defect is Gecko's alone, which is worth knowing before blaming it for the
next flash: driven through the same renders at 20× CPU throttle, Blink never
once laid the board out at the fallback size, not with `reuseBoard` and not
without it. A board misbehaving in the APK is a different bug wearing the same
face.

It was, and it is worth naming so the next one is not misfiled either. Blink
flashed the board on the APK after every guess and every decor tap, and none of
it was this: not the fallback size but the *pre-resolution* one, one frame at
`width: 344px` / `font-size: 30.96px` against a correct 339.84 / 30.5859 at
360×800, which is the `100%` branch of `min(100%, 100cqh * 5/6)` and the `45cqw`
branch of the font. That is the container query resolving on the second pass, exactly as
designed, and then being *transitioned into* rather than landed on because the
reduced-motion block had armed a transition on every property in the document.
`.grid-wrap` measured 407.81px on the good frames and the bad ones alike, so the
container was never wrong and `reuseBoard` was never going to help; it keeps the
box and replaces the `.grid` inside it. CPU speed was not a variable either: 6×
throttling without the preference gave zero bad frames, 1× with it gave them. If
the board flashes again, read `getAnimations()` on the bad frame before anything
else: it named `width` and `font-size` on `.grid` outright.

Mobile first: portrait, thumb-reachable, sheets rising from the bottom edge
rather than centered. Anything with a `data-tip` gets the hover panel on a mouse
and a long-press one on a finger, so a new affordance usually only needs the
attribute. Which of the two a pointer gets is decided per event on
`pointerType`, and never on a media query: the Android WebView the APK runs in
answers `(hover: hover)` like a desktop, on a phone with no mouse in the room,
and a gate built on that answer handed a thumb both halves at once; see
`bindTips` for what that did to the panel. Anything the app has to ask about the
*input* is worth distrusting; the queries about the screen are sound.

The stylesheet answers `prefers-reduced-motion` with one blanket block, and that
block is the single most expensive thing in the file to get wrong, because the
APK's WebView passes Android's "Remove animations" straight through and no
desktop browser has it set. Three separate bugs have come out of it, all the same
mistake: it once said `animation-duration: 1ms` and `transition-duration: 1ms`,
and a duration short enough to look like stillness is still a duration.

An animation with `forwards` fill is not calmed by 1ms, it is *completed* by it,
in the frame it starts, so any keyframe sequence ending at `opacity: 0` deletes
its element rather than slowing it, and that took the refusal toast and every
`+chips +mult` badge in a cascade. An animation with a delay keeps the delay, so
`.shop-item`'s 70ms-per-card stagger held five `backwards`-filled cards at
`opacity: 0` for up to 280ms with no animation left to explain it. And a
*transition* duration on `*` does not shorten the transitions this file writes,
since every one of those names its properties. It lands beside an untouched
`transition-property: all` and arms a transition on every element and every
property in the document, which is what the board flash below turned out to be.

So the block now says `animation: none` and `transition: none`, and the standing
rule is that it must stop motion rather than compress it. If an animation carries
information rather than decorating it, own its lifetime in JS and leave CSS only
the fade; see `TOAST` in `app.ts`. If killing it drops something the keyframes
were the only source of, carve it out by name; see `.floater, .tile-gain`, whose
horizontal centering lives nowhere but its `-50%`.

How long motion lasts is a player setting, and it is one number in two
languages. `src/ui/speed.ts` owns it, writes it to the root as `--pace` (a scale
on *duration*, so ×2 is `0.5`), and every one-shot animation in the stylesheet is
`calc(Xms * var(--pace))` while every timer in `app.ts` goes through `beat`,
which divides by the same setting. Anything paced on one side must be paced on
the other, or a class comes off a tile that is still flipping. Three things stay
off it: reading time (`TOAST`), input conventions (`HOLD`), and the two
animations that loop forever, which nobody is waiting on. And the ladder stops at
×3 rather than at an off switch, because zero does not still a `forwards`
animation, it finishes it, which is the same bug as `1ms` wearing a friendlier
face.

The same render that `reuseBoard` protects can be defeated by a class. It keeps
the container only when the live screen and the new one are the same kind, and a
live screen collects classes describing what is happening to it, and `shaking`
lasts 420ms after a big guess, against a final render awaited on 400. Compare
screens on `screenKind`, not on the class string, and add anything transient to
`TRANSIENT_SCREEN`.

`settled` is the other one in that list, and it is what the rebuild does to an
animation rather than to a layout. A node that has not changed is still a *new*
node, so it plays its entry animation again, which is right on a screen the
player just arrived at and wrong on a button inside a sheet: tapping sound, music
or the speed dial changed one word of one label, and the backdrop went to
`opacity: 0` and the sheet rebuilt 24px low on every tap, taking the shop's five
cards with it. `settled` marks the two things a render knows arrived at nothing —
a sheet that was already open, and the screen behind an open sheet — and the
stylesheet answers it with `animation: none`. Two questions, not one: a reroll
deals a new shelf onto the same shop screen, so "same screen" alone would
silence the render the animation is for. What may listen to the mark is an
animation meaning *this just appeared*, on a node a view builds; a class a
handler adds after a render (`.tile.land`, from `patchDraft`) would be silenced
for as long as the mark outlives the render that set it. See `settled` in
`app.ts` and the `rebuilds` block in the stylesheet.

Purely presentational settings live as a class on the document root
and are switched off in the stylesheet (see `.plain` and `.quiet`) rather than
threaded through the views, which the full rebuild would otherwise make every
view's business.

Tests run in Node with no DOM: there is no jsdom. `test/ui` covers pure logic
only; rendering and interaction get validated in a real browser instead.

## Changing balance

`CONTRIBUTING.md` has the mechanics: bump `CONTENT_VERSION`, `npm run golden`,
read the diff. What it does not state is the standard of evidence. Balance comments here
cite run counts because the numbers came from actually simulating the change, and
a nerf argued from intuition will read as out of place beside them. A throwaway
harness over a few hundred seeds costs minutes: write one, quote what it said,
delete it. Batch balance edits into a single commit so the vector diff reads as
one deliberate move.

Watch for tests that pin balance incidentally. A shelf reshuffle moves outcomes
on seeds nobody was thinking about, and the right repair is usually to assert the
distribution rather than to renumber the expectation.

## Comments

The house style is heavy and deliberately so: comments explain *why*, and record
what was tried and rejected along with the numbers that settled it. Match the
density and the voice of the file you are in. Code written in this codebase's
style but not its prose is half-finished.

## Storage

`5wild:run:v2` (the run save), `5wild:run:lang` beside it, `5wild:meta:v2` (the
record), and six settings: `5wild:plain`, `5wild:speed`, `5wild:lang`,
`5wild:muted`, `5wild:music`, `5wild:coached`. All are booleans except three:
`5wild:plain` holds one of `all`, `minimal` or `none`, how much of the scoring
game the board draws on itself; `5wild:speed` holds `1`, `2` or `3`, how many
times faster than authored the animations play; and `5wild:lang` holds one of
`en`, `es`, `fr`, `de`. Adding an optional field needs no key bump; changing what
an existing field means does.

`5wild:run:lang` is the only key that is neither a setting nor part of a blob,
and it is required rather than optional, which is unusual enough to say why. The
language drives two things on two clocks: the interface changes the instant it is
set, and the words change at the *next* run, because a run is dealt from one word
list and every guess is validated against it, so swapping the list under a live
run strands the answer outside `allowed` and the only honest repair is to discard
the run. A settings tap must not cost a run. That means a save and the setting can
legitimately disagree, and on launch the shell has to pick a word list *before*
the app exists and cannot tell from those two alone which way the disagreement
runs. So the run's own language is written beside it. Absent means a save from
before the key, which was necessarily English. See `setLanguage` and `withWords`
in `app.ts`.

`5wild:coached` is the odd one: it is the only flag that is not a setting. It
records that the first-round tutorial has been spent, whether by being played
through or by being declined on the intro card that offers it. There was a
`5wild:seen-help` beside it, meaning the rules sheet had already interrupted a
first launch, and the two were kept apart because a sheet closed at the title
screen is not a round played. It is gone: the sheet no longer opens itself, so
nothing was left to remember. The coaching teaches the scoring half at the moment
each piece of it first becomes true, and the rest of the sheet (shops, bosses,
ascensions) is a button on the title screen and in the pause menu for whoever
wants it. Old installs keep the orphaned key; nothing reads it. Everything else
about the coaching is derived from the run; see `src/ui/coach.ts`.

Renaming a field is the case in between, and the two keys answered it
differently when antes became stages and jokers became relics. The run save
bumped: a v1 save spells half its fields differently, and a run read as half
present is worse than a run refused. The record did not: it is independent
counters, `loadMeta` already reads them one at a time, so it reads the old
spelling where the new one is missing and keeps everything, including the
`cleared` that gates the ladder, which a bump would have thrown away to make a
point about schemas.

`cracked` became `crackedBy` the same way, and it is the one of the three that
changed shape rather than spelling: a flat list of every answer ever found is
read against one language's answer pool, so a Spanish word in it is a fraction
that can pass 100%, and ACTOR — an answer in English and in Spanish both — was
one crack credited for two. Keyed by language, and the old flat list is filed
under `en`, which is not a guess: a record from before the languages was
necessarily English. It is merged only when `crackedBy` is absent entirely.
Merging per-language would resurrect the same words on every launch, because the
dead `cracked` stays in the blob and this build has already read it once.

## Backlog

`plans/roadmap.md` is the real one, phases each with a postmortem saying what
the change did to the vectors, and `TODO` at the root is scratch. Both are
gitignored, so a fresh clone will not have either.
