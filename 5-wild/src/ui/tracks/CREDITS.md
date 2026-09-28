# Music credits

Kept apart from `src/ui/sounds/` on purpose: everything in there is globbed and
decoded as an effect on the first sound, and a four-minute track decoded to PCM
is some eighty megabytes nobody asked for.

| File | Source | Licence |
| --- | --- | --- |
| `forget-me-not.ogg` | [Forget-me-not in F major](https://opengameart.org/content/forget-me-not) (looped version) by [Kistol](https://opengameart.org/users/kistol), OpenGameArt | CC0 1.0 |
| `promises.ogg` | "promises" by [kate](https://kate.garden/), from [kate-jiang/kate.garden](https://github.com/kate-jiang/kate.garden/blob/main/public/music/promises.mp3) | Used with the artist's permission |

`forget-me-not.ogg` was raised 3.6dB to peak at -1dBFS, like the effects, and
re-encoded as Vorbis at q2 (1.3MB against 3.3MB as uploaded). It was not
trimmed: the loop is authored to run its last sample into its first, and the
sample count is unchanged (5,472,810 at 44.1kHz), so `loop` on the buffer source
still joins it cleanly. Solo piano holds up at q2 in a way a mix with cymbals
would not.

`promises.ogg` was converted from the 320kbps MP3 (9.9MB) to Vorbis at q2
(2.6MB), raised 1.4dB to peak at -1dBFS like the effects. Not authored as a
loop: 0.45s of lead-in and 0.7s of tail silence were trimmed, with a 20ms fade
in and a 100ms fade out so the join does not click, leaving 4:05.85.

`promises` is the one file in the game that is not public domain. No licence is
published beside it; kate gave the project permission directly. Anyone forking
the repo does not inherit that permission and should drop the file, which
leaves `forget-me-not.ogg` playing alone.

The player picks between the two on the pause sheet; see `TRACKS` in
`src/ui/music.ts`.
