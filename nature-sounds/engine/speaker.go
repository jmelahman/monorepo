package engine

import (
	"time"

	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/speaker"
)

// SampleRate is the rate the engine runs at. 48 kHz is what nearly every
// sound card runs at natively, so the operating system has nothing left to
// convert on the way out.
const SampleRate = beep.SampleRate(48000)

// bufferLatency is how much audio the speaker asks for at a time. A bigger
// buffer survives a busy CPU without dropouts; a smaller one makes key
// presses take effect sooner. A tenth of a second suits ambient sound, where
// nothing needs to be sample-accurate.
const bufferLatency = 100 * time.Millisecond

// Start opens the audio device and has it play the mixer.
func Start(m *Mixer) error {
	if err := speaker.Init(m.SampleRate(), m.SampleRate().N(bufferLatency)); err != nil {
		return err
	}
	speaker.Play(m)
	return nil
}

// Stop fades the mixer out, so quitting does not cut the sound off with a
// click, then closes the audio device.
func Stop(m *Mixer) {
	const fade = 150 * time.Millisecond
	m.FadeOut(fade)
	// The fade only happens as the speaker pulls buffers, so wait for it,
	// plus one buffer still queued in the device.
	time.Sleep(fade + bufferLatency)
	speaker.Clear()
	speaker.Close()
}
