<h1 align="center">5 Wild</h1>

<p align="center"><b>A word-guessing roguelike.</b></p>

<p align="center">
  <a href="https://5-wild.com"><img alt="Play now" src="https://img.shields.io/badge/Play%20now-5--wild.com-538d4e?style=for-the-badge&logoColor=white"></a>
  <a href="https://github.com/jmelahman/5-wild/releases/latest/download/5-wild.apk"><img alt="Download APK" src="https://img.shields.io/badge/Download-APK-b59f3b?style=for-the-badge&logo=android&logoColor=white"></a>
</p>

<p align="center">
  <img src="assets/screenshot.png" alt="A boss round of 5 Wild in progress: The Tyrant demanding every guess reuse the greens found, five relics along the top, and a score of 7,972 against an 8,800 target" width="380">
</p>

---

Guess the five letter word, but every guess you play is also a hand you score.
Letters score points based on how rare they are, the green and yellow feedback
multiplies them, and the relics you buy between rounds quietly rewrite the
arithmetic underneath. It is a word game that turns into a numbers game.

A guess that narrows the word down is usually made of cheap letters and scores
almost nothing. A guess built to score plays the letters you have upgraded, and
spends one of the six you get.

Solving multiplies the round's whole pile by the guesses you had left, and the
guesses you never spent pay gold at the shop. Farming one more big hand costs you
both — and the target is often high enough that you have to.

## What you are playing with

🏺 **Relics** — twenty eight of them, bought between rounds, and they stack.
Snowball gains mult for every green you play; Q's Bargain triples J, Q, X and Z
until the worst letters in the alphabet are the ones you hunt for.

🔩 **Letter modifiers** — an upgrade bought onto a letter follows it for the rest
of the run. Steel doubles your mult, Glass triples it and might shatter, and
Wild pays you *more* for being wrong.

👹 **Bosses** — fifteen of them, one every third round, each breaking a rule you
were relying on. The Fog makes yellow and gray identical, The Tyrant demands
every guess reuse the greens you found, The Mirror shows your feedback back to
front.

🃏 **Consumables** — four one shot cards held until they matter. The Oracle hands
you a letter, The Hermit rules one out, The Magician promotes a gray to yellow,
The Fool scores your last guess all over again.

✒️ **Etchings and categories** — etchings make a whole group of letters worth
more chips for the rest of the run, and packs level up a word category, so the
shape you keep reaching for pays more every time you play it.

🪜 **Eight stages, then the ladder** — win a run and ten ascensions open above
it, each a new rule to fight rather than a bigger number. Ninety harder rungs
wait behind those.

🌍 **Four languages** — English, Spanish, French and German, each with its own
word list and its own keyboard, so a French run is AZERTY and a German one
QWERTZ.

## Play it

**Web** — <https://5-wild.com>

**Android** — [download the APK](https://github.com/jmelahman/5-wild/releases/latest/download/5-wild.apk)
straight from your phone's browser, no store account and no cable. Android asks
permission to install from the browser the first time. Everything ships inside
the package, word lists included, so it plays with no network at all; your run
and your records stay on the device.

**Source** — `npm ci && npm run dev` puts it on <http://localhost:5173>.
`CONTRIBUTING.md` has the rest.
