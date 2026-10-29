// Package engine turns sound sources into one stream for the speaker.
//
// The signal flows left to right, one buffer of samples at a time:
//
//	recording ─▶ loop ─▶ resample ─▶ layer gain ─┐
//	                                             ├─▶ Σ ─▶ pause gain ─▶ master gain ─▶ speaker
//	another recording ─────────────▶ layer gain ─┘
//
// Usually one layer plays. Two play at once while switching sounds: the old
// one fades out as the new one fades in.
//
// A source is anything that implements beep.Streamer: it fills a slice of
// stereo samples, each channel a float64 in [-1, 1], whenever it is asked.
// The speaker asks the Mixer for about a tenth of a second at a time on its
// own goroutine, and the Mixer asks each layer for the same amount, scales it
// by that layer's gain, and adds the layers together. Adding is all mixing
// is: two sounds playing at once are two waveforms summed.
//
// Three ideas carry most of the weight here, and each has a file:
//
//   - gain.go: a gain never jumps. A sudden step in loudness is a step in the
//     waveform, and the ear hears a step as a click, so every change ramps
//     over a few milliseconds ("parameter smoothing"). Fades are the same ramp
//     stretched out over seconds.
//   - mixer.go: the audio goroutine must never wait long. It holds a lock only
//     while it computes one buffer, never allocates in the steady state, and
//     hands slow work (closing files) to another goroutine.
//   - recording.go: every source runs at the engine's sample rate. A 48 kHz
//     file read as if it were 44.1 kHz plays 8% slow and a semitone and a half
//     flat, which is what the player before this engine did to every Rocky
//     Mountain recording.
package engine
