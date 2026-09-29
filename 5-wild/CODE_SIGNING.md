# Code signing policy

Free code signing provided by [SignPath.io](https://about.signpath.io),
certificate by [SignPath Foundation](https://signpath.org).

The Windows installer attached to each [release](https://github.com/jmelahman/5-wild/releases)
is signed. It is built from this repository by GitHub Actions
(`.github/workflows/desktop.yml`) from the tagged commit, and nothing built
anywhere else is submitted for signing.

## Team roles

- Committers and reviewers: [Jamison Lahman](https://github.com/jmelahman)
- Approvers: [Jamison Lahman](https://github.com/jmelahman)

## Privacy

This program will not transfer any information to other networked systems
unless specifically requested by the user or the person installing or operating
it.

Specifically: the game ships with everything it needs, word lists included, and
plays with no network at all. Runs and records are kept on the device. Two
things can leave it, and only when the player asks:

- **Telemetry**, off unless switched on from the about or pause sheet. When on,
  it sends a replay of each finished run to a server run for this project: the
  seed, the difficulty, the word list's language, the game's version, how many
  runs have been played on this install, and every move of the run. No account,
  name or device identifier is part of it.
- **Bug reports**, which open GitHub's issue form in the browser, with the game
  version, platform and language filled in for the player to read before
  anything is posted.

Links to outside sites (the source, the credits) open in the system browser.
