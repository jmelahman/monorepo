# Writing for this site

Posts are the `*.md` files in this directory. When drafting or revising one, write in Jamison's voice. These rules come from how he rewrote the first draft of `blog/llms-play-5-wild.md`; the examples are his edits (draft → what he changed it to).

## Voice

- **First person, with opinions.** Say why I care and what I think: "Two of my favorite LLM benchmarks are…", "I like the idea of…", "I don't care much if…". Don't open with detached analysis of the topic.
- **Hedge honestly.** "seems", "probably", "I think", "in the ballpark", "I vaguely recall", "I'm not too sure". Don't state a conclusion more firmly than five data points allow, but don't write a paragraph of statistical disclaimers either.
- **Conversational, not polished.** Contractions ("isn't", "you've"). The occasional parenthetical aside or joke is welcome: "(but that would also be no fun)".
- **Include the builder's view.** Why something was built, what it was designed for, what should change: "I designed that upgrade for…", "From a balancing perspective, I think this can be improved…".

## Say it plainly

- **No metaphors or clever phrasing.** State the literal thing.
  - "putting a door on it. That door is an MCP server" → "I originally built a spectator mode… via an MCP server"
  - "It chose to lose battles to win the war" → "It chose to lose rounds in order to win the game"
  - "The last rule is the sharpest." → "Ascension 10 punishes poor solving, but solving alone doesn't win."
  - "`rules`: the whole briefing, once" → "`rules`: a how to play guide"
- **No rhetorical constructions.** Avoid "X, not Y" framings, punchy one-line closers ("because they had to."), colon reveals, and fragment lists ("No shell, no files, no web"). Write the full sentence: "There is no shell, file access, web search or subagents, so agents can't write a solver."
- **No signposting.** Cut lines like "A few of the play styles in more detail." and thesis paragraphs that preview the conclusions. Start the section.
- **Use the domain's real terms**, not paraphrases: "ascension 0" not "the base game", "non-bosses" not "ordinary rounds", "scales with", "build", "economy", "RNG", "soft-lock", "win condition", "SOTA".
- **Anchor new concepts to something familiar**, briefly: "(like Scrabble)", "(Wordle's hard mode)", "relics (Balatro's jokers)".

## Content

- **Interpret, don't list numbers.** The charts carry the data. Prose says what it means and why: "Opus had the lowest average number of guesses per round allowing it to increase its income and supercharge its scoring engine", not "Opus earned 8.9 gold a round to Sonnet's 6.7 and rerolled 18 times to Sonnet's once". Keep a number only when the number is the story (CRANE in 100 of 120 rounds).
- **Cut minutiae.** Operational detail nobody needs (which runs were resumed on which plan) and extra examples after the first good one get deleted. Whole sections go if they don't earn their place.
- **Caveats are short and go at the end.**
- Sentences can run long and chain cause and effect ("…, allowing it to…", "…, causing…"). Don't chop them into staccato.

## Mechanics

- American spelling (favorite, colored, gray).
- Digits and symbols: "$4", "11 minutes", "34s", "30%", "3 of 5", "5/5". Don't spell numbers out.
- Results in past tense ("fell apart", "disqualified").
- Full model names with versions on first use (Opus 5.5, GPT-6 Luna).
- Sentence-case headings. Findings as a bold lead-in sentence followed by the paragraph.
- Long runs of links become a bulleted list, not an inline sentence.
- Link to other sections of the post where it helps (`[repeats a letter](#word-shapes)`).

## When asked to review

"Grammar only" means grammar: fix errors, flag unfinished sentences, and leave wording and content alone.
