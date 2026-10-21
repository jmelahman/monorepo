package publish

import (
	"bytes"
	"context"
	"encoding/binary"
	"math"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// tone is a short 16-bit mono WAV of a tone.
func tone() []byte {
	const rate, n = 24000, 2400
	var b bytes.Buffer
	b.WriteString("RIFF")
	write := func(v any) {
		if err := binary.Write(&b, binary.LittleEndian, v); err != nil {
			panic(err)
		}
	}
	write(uint32(36 + 2*n))
	b.WriteString("WAVEfmt ")
	write(struct {
		Size                      uint32
		Format, Channels          uint16
		Rate, ByteRate            uint32
		BlockAlign, BitsPerSample uint16
	}{16, 1, 1, rate, 2 * rate, 2, 16})
	b.WriteString("data")
	write(uint32(2 * n))
	for i := range n {
		write(int16(8000 * math.Sin(2*math.Pi*440*float64(i)/rate)))
	}
	return b.Bytes()
}

func TestPublishAtOnceKeepsBoth(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("no ffmpeg")
	}
	p := &Publisher{Dir: t.TempDir()}
	ep := Episode{Podcast: "Notes: one", Title: "Pilot?", Date: time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC)}
	paths := make([]string, 3)
	var wg sync.WaitGroup
	for i := range paths {
		wg.Go(func() {
			path, err := p.Publish(context.Background(), ep, bytes.NewReader(tone()))
			if err != nil {
				t.Error(err)
			}
			paths[i] = path
		})
	}
	wg.Wait()
	seen := map[string]bool{}
	for _, path := range paths {
		if seen[path] {
			t.Errorf("two episodes were written to %s", path)
		}
		seen[path] = true
	}
	if want := filepath.Join(p.Dir, "Notes one", "2026-10-21 Pilot.mp3"); !seen[want] {
		t.Errorf("published %v, want one at %s", paths, want)
	}
	if left, _ := filepath.Glob(filepath.Join(p.Dir, "Notes one", ".publish-*")); len(left) > 0 {
		t.Errorf("left behind %v", left)
	}
}
