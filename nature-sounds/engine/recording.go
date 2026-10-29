package engine

import (
	"errors"
	"fmt"
	"os"

	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/mp3"
)

// ErrDecode means a file opened but is not a playable mp3, most likely a
// download that was cut short.
var ErrDecode = errors.New("cannot decode recording")

// resampleQuality is how many neighbouring samples beep's resampler fits a
// curve through to estimate each new sample. Higher is cleaner and costs more
// CPU; 4 is inaudibly clean for ambient sound.
const resampleQuality = 4

// Recording is an mp3 file playing on a loop, converted to the engine's rate.
type Recording struct {
	beep.Streamer

	decoder beep.StreamSeekCloser // Owns the file; closing it closes the file.
}

// OpenRecording opens the mp3 at path as an endless loop at rate.
//
// Each sample in a file is one snapshot of the waveform, and the sample rate
// says how many snapshots make a second. The NPS recordings disagree: the
// Rocky Mountain ones are 48000 Hz and the Yellowstone ones 44100 Hz. Handed
// to a 44.1 kHz speaker unconverted, a 48 kHz file is played back 44100
// snapshots per second, so one second of recording lasts 48000/44100 = 1.088
// seconds and every pitch drops by 12·log2(1.088) ≈ 1.5 semitones.
//
// Resampling fixes it by computing what the waveform would have been at the
// speaker's instants instead: for each output sample it fits a polynomial
// through the neighbouring input samples and reads the curve at that moment.
func OpenRecording(path string, rate beep.SampleRate) (*Recording, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	// mp3.Decode takes ownership of f: closing the decoder closes the file.
	// beep's decoders always produce stereo, copying a mono file's one
	// channel to both sides.
	decoder, format, err := mp3.Decode(f)
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("%w %s: %w", ErrDecode, path, err)
	}

	// Loop before resampling. The loop seeks the decoder back to the start
	// when it runs out, and only the decoder can seek; downstream of a
	// resampler, a "position" no longer maps cleanly onto the file.
	//
	// The seam is a plain cut from the last sample to the first. Field
	// recordings rarely end where they began, so the seam can tick; blending
	// the tail into the head (a crossfade loop) is the natural next step.
	loop, err := beep.Loop2(decoder)
	if err != nil {
		_ = decoder.Close()
		return nil, fmt.Errorf("loop %s: %w", path, err)
	}

	var s = loop
	if format.SampleRate != rate {
		s = beep.Resample(resampleQuality, format.SampleRate, rate, loop)
	}

	return &Recording{Streamer: s, decoder: decoder}, nil
}

// Close releases the file. The Mixer calls it when the layer is removed.
func (r *Recording) Close() error {
	return r.decoder.Close()
}
