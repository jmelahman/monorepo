package engine

import (
	"io"
	"sync"
	"time"

	"github.com/gopxl/beep/v2"
)

const (
	// smoothing is how long an ordinary gain change (a volume key, a pause)
	// takes to glide into place. Around 20-50 ms is short enough to feel
	// instant and long enough that the ear hears no click.
	smoothing = 30 * time.Millisecond

	// minVolumeDB is where the volume control bottoms out. Below it the
	// master gain is exactly zero instead of an inaudible 0.001.
	minVolumeDB = -40
	maxVolumeDB = 6
)

// Mixer sums any number of layers into one stereo stream.
//
// It implements beep.Streamer, so the speaker pulls samples from it, but it
// never touches the speaker itself. That keeps it testable: a test calls
// Stream directly and inspects the numbers it gets back.
//
// Every method is safe to call from any goroutine. They all take mu, the same
// lock Stream holds while it fills a buffer, so a change lands between two
// buffers and never halfway through one.
type Mixer struct {
	mu   sync.Mutex
	rate beep.SampleRate

	layers []*Layer
	master ramp // The volume control.
	pause  ramp // 1 while playing, 0 while paused.

	paused   bool
	volumeDB float64

	// scratch is where each layer renders before it is scaled and added to
	// the output. It is reused across calls: allocating on the audio
	// goroutine invites a garbage-collector pause at the worst moment, and a
	// late buffer is an audible dropout.
	scratch [][2]float64
}

// Layer is one source playing in a Mixer, with its own gain.
type Layer struct {
	mixer *Mixer
	src   beep.Streamer
	gain  ramp

	// leaving is set by Remove: once the fade-out lands on zero, the mixer
	// drops the layer and closes its source.
	leaving bool
}

// NewMixer returns an empty Mixer running at rate. Every source added to it
// must already produce samples at that rate; see OpenRecording.
func NewMixer(rate beep.SampleRate) *Mixer {
	return &Mixer{
		rate:   rate,
		master: newRamp(1),
		pause:  newRamp(1),
	}
}

// SampleRate is the rate every layer's source must produce.
func (m *Mixer) SampleRate() beep.SampleRate {
	return m.rate
}

// Add starts src playing, fading in from silence over fade.
//
// If src implements io.Closer, the mixer closes it once the layer is removed.
func (m *Mixer) Add(src beep.Streamer, fade time.Duration) *Layer {
	l := &Layer{mixer: m, src: src, gain: newRamp(0)}
	m.mu.Lock()
	defer m.mu.Unlock()
	l.gain.set(1, m.samples(fade))
	m.layers = append(m.layers, l)
	return l
}

// Remove fades the layer out over fade and then drops it. Calling Add for a
// new layer and Remove for the old one with the same fade is a crossfade.
func (l *Layer) Remove(fade time.Duration) {
	m := l.mixer
	m.mu.Lock()
	defer m.mu.Unlock()
	l.leaving = true
	l.gain.set(0, m.samples(max(fade, smoothing)))
}

// SetPaused pauses or resumes everything.
//
// Pausing ramps the output down first, then stops pulling from the layers
// entirely, so a recording resumes from where it was paused rather than from
// wherever it would have reached had it kept playing silently.
func (m *Mixer) SetPaused(paused bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.paused = paused
	target := 1.0
	if paused {
		target = 0
	}
	m.pause.set(target, m.samples(smoothing))
}

// Paused reports whether the mixer is paused.
func (m *Mixer) Paused() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.paused
}

// SetVolume sets the master volume in decibels, clamped to [-40, +6].
// 0 dB plays sources at their recorded level; the bottom of the range is
// silence. It returns the volume it settled on.
func (m *Mixer) SetVolume(db float64) float64 {
	db = min(max(db, minVolumeDB), maxVolumeDB)
	gain := dbToGain(db)
	if db <= minVolumeDB {
		gain = 0
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.volumeDB = db
	m.master.set(gain, m.samples(smoothing))
	return db
}

// Volume is the master volume in decibels.
func (m *Mixer) Volume() float64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.volumeDB
}

// FadeOut glides the master gain to silence over d, as a sleep timer does.
// SetVolume afterwards brings the sound back.
func (m *Mixer) FadeOut(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.master.set(0, m.samples(d))
}

// Stream fills samples with the sum of every layer. It is called by the
// speaker's goroutine; it never runs out, it just plays silence when there is
// nothing to play.
func (m *Mixer) Stream(samples [][2]float64) (n int, ok bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	clear(samples)

	// Fully paused: leave every source where it is. While the pause ramp is
	// still gliding down, fall through and keep mixing so the sound fades
	// rather than stops dead.
	if m.paused && m.pause.settled() {
		return len(samples), true
	}

	if cap(m.scratch) < len(samples) {
		m.scratch = make([][2]float64, len(samples))
	}
	buf := m.scratch[:len(samples)]

	kept := m.layers[:0] // Filter in place: no allocation.
	for _, l := range m.layers {
		if m.mixLayer(l, samples, buf) {
			kept = append(kept, l)
		} else {
			retire(l.src)
		}
	}
	// Drop the pointers past the end so finished layers can be collected.
	clear(m.layers[len(kept):])
	m.layers = kept

	for i := range samples {
		g := m.master.next() * m.pause.next()
		samples[i][0] *= g
		samples[i][1] *= g
	}
	return len(samples), true
}

// mixLayer renders one layer into buf and adds it to out. It reports whether
// the layer should keep playing.
func (m *Mixer) mixLayer(l *Layer, out, buf [][2]float64) bool {
	n, ok := l.src.Stream(buf)
	// A source that comes up short has ended (a non-looping file, or a read
	// error). Whatever it did produce still plays; the rest is silence.
	clear(buf[n:])
	for i := range out {
		g := l.gain.next()
		out[i][0] += buf[i][0] * g
		out[i][1] += buf[i][1] * g
	}
	if !ok || n < len(buf) {
		return false
	}
	return !l.leaving || !l.gain.settled()
}

// Err is always nil: a failing source is dropped rather than stopping the
// whole mix.
func (m *Mixer) Err() error {
	return nil
}

func (m *Mixer) samples(d time.Duration) int {
	return m.rate.N(d)
}

// retire closes a finished source off the audio goroutine, because closing a
// file is a system call that can take arbitrarily long.
func retire(src beep.Streamer) {
	if c, ok := src.(io.Closer); ok {
		go func() { _ = c.Close() }() // Nothing useful to do with the error here.
	}
}
