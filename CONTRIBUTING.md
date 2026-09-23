# Contributing

Everything here is about the code. The README is the game; `CLAUDE.md` is the
design notes, and is worth reading before changing anything the engine owns.

## Running it

```sh
npm ci
npm run dev        # http://localhost:5173
```

The same three commands CI runs:

```sh
npm run typecheck
npm run check      # Biome lint + format
npm test           # Vitest
```

## Layout

```
src/engine/     pure TypeScript rules engine: no DOM, no clocks, no ambient RNG
src/content/    letter tables and round curves
src/ui/         DOM rendering, the scoring animation, and the language catalogs
public/words/   answer and allowed-guess lists, one directory per language
test/           unit tests, golden vectors, and the engine-purity guard
tools/          word-list and icon generation
assets/         icon source art and the README screenshot
android/        Capacitor's Android project, committed
```

`src/engine` is deliberately portable: it imports nothing outside itself and
`src/content`, and touches no ambient nondeterminism.
`test/engine-purity.test.ts` enforces that in CI rather than by discipline. The
engine writes no prose either, so every string a player sees lives in
`src/ui/lang/`, keyed by an id or a refusal code.

The launcher icons, the launch screen and the browser favicon are all rendered
from `assets/*.svg` by `tools/gen-icons.sh`. The PNGs it writes are committed,
since Gradle cannot render an SVG and CI has no renderer, so edit the source art,
run the script, and commit what changes.

## Releasing

The web build is wrapped by Capacitor. Publishing a GitHub release builds a
signed APK and attaches it, which is the whole publishing step:

```sh
npm version patch --no-git-tag-version   # and commit it
gh release create "v$(npm pkg get version --workspaces=false | tr -d '"')" --generate-notes
```

The tag is `v` plus the version in `package.json`, and that pair moves once per
phase of work: a phase that changed nothing a player can see does not need a
release, but anything that does gets a patch bump so the phone has a build to
install and a number to name it by.

Every push also builds the APK to prove it still compiles, but that one is
unsigned and cannot be installed: the signing key belongs to the release path
alone. It lives in the `ANDROID_KEYSTORE_BASE64` repo secret and is what allows
a new version to replace an installed one. Android treats a package signed by a
different key as a different app and refuses the upgrade, so losing that key
means every future install is a fresh one with the save wiped.

Building locally additionally needs a JDK and the Android SDK:

```sh
npm run apk        # build + cap sync + gradlew assembleDebug
```

## Publishing to Play

Play has not accepted APKs for new apps since 2021, so the release workflow
builds an App Bundle alongside the APK and leaves it as a workflow artifact
named `5-wild-aab`. Promoting a version is: cut the release as above, open that
run in Actions, download the artifact, upload it in the Play Console. The `.aab`
is deliberately not attached to the GitHub release: it is not installable, and
sitting beside the `.apk` it would only be downloaded by mistake.

The listing graphics are rendered from `assets/*.svg` and committed, the same
arrangement as the launcher icons and for the same reason:

```sh
./tools/gen-store-art.sh   # assets/store/{icon,feature-graphic}.png
```

Play's sizes are exact rather than minimums and it checks them at the upload
form, which is the worst place to find out; `test/store-art.test.ts` checks them
here instead. The listing icon has its own source, `assets/icon-store.svg`,
because Play applies its own rounding and a file with the radius already baked
in shows up notched.

Screenshots are the one listing asset that cannot be generated from source art,
and the privacy policy Play requires of every app is served from
`public/privacy/` at <https://5-wild.com/privacy/>.

The signing story has one trap worth stating plainly. Play re-signs what you
upload, so unless the existing keystore is handed to Play App Signing at the
moment the app is created, a choice with no later undo, the Play build and the
sideloaded APK are signed by different keys, which makes them *different apps* to
Android. Anyone holding an install from the releases page would then have to
uninstall to move to Play, and the save goes with it.

## Golden vectors

`test/golden/vectors.json` holds recorded runs: a seed and a list of actions in,
every guess's chips, mult and score out, plus the gold timeline and what the run
was holding when it ended. Bots in `test/golden/scenarios.ts` author them; the
test replays the recorded actions, never the bots, so a scenario can be rewritten
without moving the baseline.

They exist to make a balance change legible. Edit a letter's chips, a target
curve, a relic's arithmetic, and a hundred numbers move at once; the vectors turn
that into a diff you can read. So a deliberate change is three steps:

```sh
# bump CONTENT_VERSION in src/content/version.ts
npm run golden     # re-record
git diff test/golden/vectors.json
```

That diff is the balance change, stated in points rather than in source. The
vectors refuse to run against a version they were not recorded at, so forgetting
the bump fails loudly instead of quietly rewriting the baseline. The JSON is
excluded from Biome for that reason: the recorder writes one array element per
line, which is what makes the diff readable, and the formatter would fold them
back onto one.

They are also the portability contract: any reimplementation of these rules, a
port to another language or a rewrite, is correct exactly when it reproduces this
file.

### What a vector holds

The input is a seed, an ascension when the run was not the ordinary game, and the
list of actions. The output is everything the run can still be asked about once
it is over: a line per scored guess, giving the stage and round it belonged to, the
boss it was played against on the one round in three that has one, the word, and
the chips, mult, solve bonus and score that produced the number, then the gold
timeline with the reason the engine gave for each delta, the itemised reward for
every round cleared, and finally the state the run ended holding. That last part
is relics, etched letters, modifiers, category levels, alphabet ranges, whatever
the growing relics banked, and which letters were destroyed.

The standard for adding a column is whether a rule can be wrong without moving a
score. `levels`, `ranges` and `grown` are all there for that reason, and so is
the boss: `bossForStage` is a shuffle of a difficulty band keyed by seed, and
five of the fifteen change nothing a guess records: The Fog and The Mirror only
repaint the feedback, The Tyrant, The Glutton and The Purist only refuse words
that were never played. A port that shuffled the band differently could meet one
of those instead of another and score every guess identically. The name is what
tells them apart.

### What they do not hold

**Permission.** `reduce` refuses an illegal action by returning the state
untouched and emitting a `rejected` event, so a replay that hits one does not
stop, it quietly plays a shorter run. A rule that got *stricter* is therefore
caught, because the guess never lands and the score list comes up short. A rule
that got *looser* is invisible: every action in the file was legal when it was
recorded, and permission cannot be tested by exercising it. `golden.test.ts`
asserts that no replayed action was refused so that the first half at least fails
by name rather than by a mysteriously truncated run, but the second half needs an
ordinary unit test asserting the refusal, and always will.

**Anything no bot ever did.** Coverage is a side effect of the scenarios, not a
property of the format. What sixteen runs and 391 scored guesses currently
reach:

```
bosses       15/15
consumables    4/4
relics        25/28   bloodhound, anagrammer, keystone unseen
modifiers      8/9    chip unseen
```

Those gaps are luck of the draw rather than anything structural, and `chip`
being the survivor makes the point: it is the *most* common entry in the
modifier table and no recorded run has ever carried one.

Watch the names rather than the count, because the count hides the movement.
Reweighting `MOD_TABLE` from eleven entries to sixteen swapped `wild` into the
covered column and `anchor` out of it while the total sat unchanged at 6 of 9,
and the card that fell out was the one that pass had just resized. Nothing went
red; the diff read as a shop change, which it was.

Closing a gap means a scenario that goes looking. `rare-smith` rerolls until it
can afford a Steel or a Glass, `wild-smith` and `anchor-smith` do the same for
their cards, and `mystic` spends consumables in an order that lets The Fool have
a guess behind it to rescore. Worth writing when a rule in that corner changes;
not worth writing speculatively. The three hunters are also near-identical in
shape by now, and want folding into one helper before a fourth is copied from
them.

Victory is the deliberate hole. No vector ends with `outcome: "victory"`:
`victor` reaches it on seed 5517 and answers `continue_run`, which the engine
refuses anywhere else, so what pins the win is the refusal assertion rather than
the outcome column. Recording a run that stopped at the victory screen would have
made `continue_run` the one action in the game no vector could contain.

**Correctness.** Everything in `expected` is read back off the engine, so a wrong
number computed by a wrong engine agrees with itself perfectly and re-records
without complaint. Vectors detect *change*; they say nothing about whether the
new number is the right one. The bump-and-diff ritual above is the whole defense,
and it only works if somebody reads the diff.

**The UI.** Nothing here touches rendering, animation, focus or storage. The
engine is the portable part and the vectors are its contract; the browser is
tested in a browser.

**What the game is like to play.** Every scenario types `state.round.answer`,
which is right for a recorder and useless for a balance question. That one has
its own instrument.

## The blind simulator

```sh
npm run sim                                        # 300 seeds, two policies
SIM=1 SIM_SEEDS=60 npx vitest run test/sim.test.ts  # narrower, while iterating
```

`test/helpers/blind.ts` is a player handed the run with `round.answer`
destructured off it, genuinely absent from the object rather than merely
unread, and `test/sim.test.ts` asserts the absence. It narrows a candidate pool
with the engine's own `computeFeedback`, and it reads `tile.shown` rather than
`tile.color`, so The Fog and The Mirror mislead it exactly as they mislead a
player. Two policies differ only in how they spend the guess budget: `solver`
guesses to find the word, `farmer` buys chips with the first two. They share
every line of shop code, so a gap between them is attributable to the guessing.

This answers what the vectors cannot. Not "does the engine still compute 471"
but "how far does somebody who has to *find* the word actually get". At
`CONTENT_VERSION` 21, over 300 seeds:

```
                solver   farmer
won run           7.3%     3.3%
median stage         4        4
rounds banked     3563     2875
by solving       92.2%    87.1%
guesses/round     4.30     5.55
```

Two things came out of it. **Boss rounds are a third of the rounds and 72% of
the deaths**. 199 of the solver's 278 losses land on round 3, which is what the
`stage.round` histogram in the report exists to show. Round 3 is also the
stage's steepest target, 600 against round 1's 300, so that figure alone
cannot tell a hard boss from a hard curve, and a throwaway probe with
`bossForStage` stubbed to `null` was what separated them. Without bosses, round
3 takes 89 of 258 losses: 34.5%, flat, for a round that is a third of the
rounds. The win rate doubles to 14.0% and the median run reaches stage 6 rather
than 4. The curve is not what kills. The boss is, and it costs half the wins.

And **the income line is dominated**: farming gets worse the harder it is
played, because mult comes from colors and colors come from being right, so a
guess thrown at chips alone scores its tiles at ×1 and buys nothing toward the
next one. Income is worth taking when a guess you wanted anyway happens to be
rich; it does not survive being made the plan.

`npm test` runs the same thing over a dozen seeds with deliberately loose
assertions: a smoke alarm for an edit that made the game unwinnable or trivial,
not a measurement of the curve. Pinning the curve here would recreate exactly the
problem the balance notes warn about: a balance expectation living in a test
nobody thinks of as a balance test.

Treat it as a *policy*, not an oracle. Its numbers move when the policy moves,
and it found its own worst bug that way. Ranking guesses by information with
chips as a strict tie-break reads as sensible and is not, because information is
a sum running into the thousands, so exact ties never occur and the chip term
never fired at all. That player was blind to chips in a game about chips and died
on stage one of 39 runs in 300. Quote the policy's constants alongside any number
taken from it.
