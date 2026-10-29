package engine

import "math"

// ramp is a gain that glides to its target instead of jumping to it.
//
// Multiplying a waveform by a gain that changes from 1 to 0 between two
// samples cuts it off mid-swing, and that discontinuity is a broadband click.
// Spreading the change over a few hundred samples turns the step into a slope
// too gentle to hear. The same mechanism with a longer ramp is a fade.
//
// A ramp is advanced once per sample on the audio goroutine, so it holds no
// lock of its own: the Mixer's lock guards it.
type ramp struct {
	current float64
	target  float64
	step    float64 // Added to current each sample while the glide lasts.
	left    int     // Samples until current lands on target.
}

func newRamp(value float64) ramp {
	return ramp{current: value, target: value}
}

// set starts a glide from wherever the gain is now to target, arriving after
// the given number of samples. Retargeting mid-glide starts from the current
// value, so reversing a fade halfway is as smooth as finishing it.
func (r *ramp) set(target float64, samples int) {
	r.target = target
	if samples <= 0 {
		r.current = target
		r.left = 0
		return
	}
	r.step = (target - r.current) / float64(samples)
	r.left = samples
}

// next returns this sample's gain and advances the glide by one sample.
func (r *ramp) next() float64 {
	g := r.current
	if r.left > 0 {
		r.left--
		r.current += r.step
		// Count samples rather than compare floats: adding step N times does
		// not land exactly on the target (0.1 has no exact binary form), so
		// the last step snaps to it. A fade to silence then ends on a true 0.
		if r.left == 0 {
			r.current = r.target
		}
	}
	return g
}

// settled reports whether the glide has arrived at its target.
func (r *ramp) settled() bool {
	return r.left == 0
}

// dbToGain converts decibels to the amplitude factor a sample is multiplied by.
//
// Loudness is heard on a ratio scale, so volume controls step in decibels:
// each -6 dB halves the amplitude, wherever on the dial it happens. Stepping
// the amplitude linearly instead would make the top of the dial do nothing
// audible and the bottom jump straight to silence.
func dbToGain(db float64) float64 {
	return math.Pow(10, db/20)
}
