# public/words

Two files per language, `answers.txt` and `allowed.txt`: lowercase, five
letters, sorted, one per line, trailing newline. The shell fetches them at
startup (`loadWords` in `src/main.ts`) and hands them to the engine, which never
loads anything itself. `answers` is what a round can deal; `allowed` is what the
player may type.

**Do not edit these files by hand.** They are the output of
`tools/gen-wordlists.ts`, which fetches every source pinned to a commit SHA so
that a rerun is byte-identical. A word added here directly vanishes the next time
anyone regenerates, and nobody will know it was ever there. Every change goes
through the generator, and the generator's comments are where the reasoning for
each list lives. Read them before adding to them; they are long because each
entry was argued for.

```sh
node tools/gen-wordlists.ts        # all four
node tools/gen-wordlists.ts en     # just one
```

It prints a count per list and a couple of notes (plurals skipped, names
skipped). Compare those against the last run. They are how you tell the filters
are still rejecting roughly what they were written to reject.

## Two pipelines

English and the other three are built differently because their sources are
different. It isn't untidiness.

- **English** takes every five-letter word in `dwyl/english-words` as allowed.
  Answers are the most frequent of those in the OpenSubtitles corpus, provided
  they are also a lowercase lemma, after a few suffix strips, in the small
  en_US hunspell dictionary. That filter is what keeps names out. 2300 answers.
- **es, fr, de** have no dwyl, so their hunspell dictionary is *expanded* from
  stems plus affix rules into a word list (`expanded()`), and the corpus's top
  50k is unioned in as guesses. 2000 answers each, not 2300; `ANSWER_COUNTS`
  explains why.

Every language then runs the same filters: LDNOOBW profanity, `HAND_PROFANE`,
`SLURS`, and a plural rule that keeps a free S off the last tile. Accents fold to
base letters and ß/œ/æ expand (`fold()`). That is the only reason WEISS, GROSS,
COEUR and SOEUR exist.

## Which knob to reach for

All in `tools/gen-wordlists.ts`. English has four hand lists. The others have
only the two shared ones.

| Problem | List | Affects |
|---|---|---|
| A real English word is refused as a guess | `HAND_ALLOWED` | allowed only |
| A real English word should be an answer and the filters eat it | `HAND_ADDED` | answers (must also be allowed; it throws otherwise) |
| An English answer is a name, trademark, borrowing or eye-dialect | `HAND_BLOCKED` | answers only |
| An answer is vulgar in *that* language | `HAND_PROFANE[lang]` | answers only |
| A word has no meaning in any of the four languages except as a slur | `SLURS[lang]` | answers **and** allowed |

Guesses are permissive on purpose, because refusing a word a player typed is
worse than accepting a rude or odd one. So the usual answer for a brand or a
newer word is "allowed, not an answer." TASER was the example that prompted this
file: it is missing from dwyl entirely, so English refuses it outright, while
French has it as an answer because *taser* is an ordinary French verb there. A
word is judged in the language it is read in. KRAUT is blocked in English and
kept in German, and that note in `HAND_PROFANE` must not be undone.

`SLURS` is the one place that overrides the permissive rule. Its bar is narrow
on purpose: CHINK, DYKES and NEGRO stay guessable because they have ordinary
readings. See the comment above it before adding anything.

## What a change costs

The lists are part of the game's contract, not static assets.

- **Changing `en/answers.txt`** changes which word every seed deals, because a
  round picks an index into this sorted list. That moves the golden vectors,
  which are replayed against the English lists specifically
  (`test/helpers/words.ts`). So bump `CONTENT_VERSION` in
  `src/content/version.ts`, run `bun run golden`, and read the diff, the same as
  a balance change. It also means telemetry replays from before the change get
  set aside rather than replayed.
- **Adding to `allowed`** only makes guesses legal. It cannot move a vector.
  After regenerating, `git diff --stat` should show `allowed.txt` changing alone.
  If `answers.txt` moved too, a hand list leaked into the ranking (CARBS did
  this once; see the `HAND_ALLOWED` comment).
- **Removing from `allowed`** can break a recorded vector or a telemetry replay
  that guessed that word. The engine refuses the guess and the replay quietly
  plays a shorter run. `golden.test.ts` catches it for vectors.
- **es / fr / de** have no golden vectors, but `CONTENT_VERSION` also gates
  telemetry replays, which carry the run's language. Changing their answers
  deserves the bump for that reason alone.

`test/wordlists.test.ts` pins the invariants: format, sorting, every answer
guessable, exact answer counts, allowed floors, the S-final rate, and a named
plural and witness per language. If a regeneration trips a count or a rate,
work out why before you move the number.
