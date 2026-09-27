# Music credits

Kept apart from `src/ui/sounds/` on purpose: everything in there is globbed and
decoded as an effect on the first sound, and a two-minute track decoded to PCM
is some forty megabytes nobody asked for.

| File | Source | Licence |
| --- | --- | --- |
| `forget-me-not.ogg` | [Forget-me-not in F major](https://opengameart.org/content/forget-me-not) (looped version) by [Kistol](https://opengameart.org/users/kistol), OpenGameArt | CC0 1.0 |

Raised 3.6dB to peak at -1dBFS, like the effects, and re-encoded as Vorbis at
q2 (1.3MB against 3.3MB as uploaded). It was not trimmed: the loop is authored
to run its last sample into its first, and the sample count is unchanged
(5,472,810 at 44.1kHz), so `loop` on the buffer source still joins it cleanly.
Solo piano holds up at q2 in a way a mix with cymbals would not.
