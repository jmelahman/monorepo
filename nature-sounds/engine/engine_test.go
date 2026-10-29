package engine

import (
	"math"
	"testing"
	"time"

	"github.com/gopxl/beep/v2"
)

const testRate = beep.SampleRate(48000)

// constant is a source that plays the same value on both channels forever,
// which makes the effect of every gain easy to read off the output.
type constant float64

func (c constant) Stream(samples [][2]float64) (int, bool) {
	for i := range samples {
		samples[i] = [2]float64{float64(c), float64(c)}
	}
	return len(samples), true
}

func (constant) Err() error { return nil }

// closeCounter records whether the mixer closed it.
type closeCounter struct {
	constant
	closed chan struct{}
}

func (c *closeCounter) Close() error {
	close(c.closed)
	return nil
}

// counter is a source that records how many samples it has produced.
type counter struct{ pulled int }

func (c *counter) Stream(samples [][2]float64) (int, bool) {
	for i := range samples {
		samples[i] = [2]float64{0.5, 0.5}
	}
	c.pulled += len(samples)
	return len(samples), true
}

func (*counter) Err() error { return nil }

func render(m *Mixer, n int) [][2]float64 {
	out := make([][2]float64, n)
	m.Stream(out)
	return out
}

func TestRampLandsExactly(t *testing.T) {
	r := newRamp(0)
	r.set(1, 3)
	got := []float64{r.next(), r.next(), r.next(), r.next(), r.next()}
	want := []float64{0, 1.0 / 3, 2.0 / 3, 1, 1}
	for i := range want {
		if math.Abs(got[i]-want[i]) > 1e-12 {
			t.Fatalf("sample %d: got %v, want %v", i, got[i], want[i])
		}
	}
	if !r.settled() {
		t.Fatal("ramp should be settled once it reaches its target")
	}
}

func TestMixSumsLayers(t *testing.T) {
	m := NewMixer(testRate)
	m.Add(constant(0.25), 0)
	m.Add(constant(0.25), 0)
	out := render(m, 10)
	if out[9][0] != 0.5 || out[9][1] != 0.5 {
		t.Fatalf("got %v, want 0.5 on both channels", out[9])
	}
}

func TestEmptyMixerIsSilent(t *testing.T) {
	m := NewMixer(testRate)
	out := [][2]float64{{1, 1}} // Stale data from a previous buffer.
	if n, ok := m.Stream(out); n != 1 || !ok {
		t.Fatalf("an empty mixer should keep streaming, got n=%d ok=%v", n, ok)
	}
	if out[0] != [2]float64{} {
		t.Fatalf("got %v, want silence", out[0])
	}
}

// A click is a big jump between neighbouring samples. Changing the volume
// mid-stream must never produce one.
func TestVolumeChangeDoesNotClick(t *testing.T) {
	m := NewMixer(testRate)
	m.Add(constant(0.5), 0)
	before := render(m, 100)
	m.SetVolume(-20)
	after := render(m, testRate.N(smoothing)+100)

	prev := before[len(before)-1][0]
	for i, s := range after {
		if jump := math.Abs(s[0] - prev); jump > 0.001 {
			t.Fatalf("sample %d jumped by %v", i, jump)
		}
		prev = s[0]
	}
	if want := 0.5 * dbToGain(-20); math.Abs(prev-want) > 1e-9 {
		t.Fatalf("settled at %v, want %v", prev, want)
	}
}

func TestFadeInStartsFromSilence(t *testing.T) {
	m := NewMixer(testRate)
	m.Add(constant(0.5), time.Second)
	out := render(m, testRate.N(time.Second)+1)
	if out[0][0] != 0 {
		t.Fatalf("first sample is %v, want 0", out[0][0])
	}
	if mid := out[testRate.N(time.Second/2)][0]; math.Abs(mid-0.25) > 1e-3 {
		t.Fatalf("halfway through the fade is %v, want 0.25", mid)
	}
	if last := out[len(out)-1][0]; last != 0.5 {
		t.Fatalf("after the fade is %v, want 0.5", last)
	}
}

func TestRemoveFadesOutThenCloses(t *testing.T) {
	m := NewMixer(testRate)
	src := &closeCounter{constant: 0.5, closed: make(chan struct{})}
	l := m.Add(src, 0)
	l.Remove(100 * time.Millisecond)

	render(m, testRate.N(50*time.Millisecond))
	if len(m.layers) != 1 {
		t.Fatal("layer was dropped before its fade finished")
	}
	render(m, testRate.N(60*time.Millisecond))
	if len(m.layers) != 0 {
		t.Fatal("layer was not dropped after its fade finished")
	}
	select {
	case <-src.closed:
	case <-time.After(time.Second):
		t.Fatal("source was not closed")
	}
}

func TestPauseHoldsSourcePosition(t *testing.T) {
	m := NewMixer(testRate)
	src := &counter{}
	m.Add(src, 0)

	m.SetPaused(true)
	fade := testRate.N(smoothing)
	render(m, fade)           // The pause ramp glides down, still pulling samples.
	silent := render(m, 1000) // Fully paused: must not pull.
	for _, s := range silent {
		if s != [2]float64{} {
			t.Fatal("paused mixer is not silent")
		}
	}
	if src.pulled != fade {
		t.Fatalf("source advanced %d samples, want only the %d of the fade", src.pulled, fade)
	}
}

// Feeding a 44.1 kHz tone through the resampler into a 48 kHz engine must
// keep its pitch: the same number of cycles per second of output.
func TestResampleKeepsPitch(t *testing.T) {
	const from, to, hz = beep.SampleRate(44100), beep.SampleRate(48000), 440.0
	var t0 int
	sine := beep.StreamerFunc(func(samples [][2]float64) (int, bool) {
		for i := range samples {
			v := math.Sin(2 * math.Pi * hz * float64(t0) / float64(from))
			samples[i] = [2]float64{v, v}
			t0++
		}
		return len(samples), true
	})
	out := make([][2]float64, to.N(time.Second))
	beep.Resample(resampleQuality, from, to, sine).Stream(out)

	crossings := 0
	for i := 1; i < len(out); i++ {
		if out[i-1][0] < 0 && out[i][0] >= 0 {
			crossings++
		}
	}
	if crossings < hz-2 || crossings > hz+2 {
		t.Fatalf("got %d cycles in one second, want %v", crossings, hz)
	}
}
