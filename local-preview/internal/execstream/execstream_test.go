package execstream

import (
	"bytes"
	"io"
	"testing"
)

func TestFrameRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	fw := NewWriter(&buf)
	if err := fw.WriteFrame(FrameStdout, []byte("hello")); err != nil {
		t.Fatal(err)
	}
	if err := fw.WriteFrame(FrameStdinEOF, nil); err != nil {
		t.Fatal(err)
	}
	if err := fw.WriteFrame(FrameExit, []byte{7}); err != nil {
		t.Fatal(err)
	}

	f, err := ReadFrame(&buf)
	if err != nil || f.Type != FrameStdout || string(f.Payload) != "hello" {
		t.Fatalf("frame = %+v, %v", f, err)
	}
	f, err = ReadFrame(&buf)
	if err != nil || f.Type != FrameStdinEOF || len(f.Payload) != 0 {
		t.Fatalf("frame = %+v, %v", f, err)
	}
	f, err = ReadFrame(&buf)
	if err != nil || f.Type != FrameExit || len(f.Payload) != 1 || f.Payload[0] != 7 {
		t.Fatalf("frame = %+v, %v", f, err)
	}
	if _, err := ReadFrame(&buf); err != io.EOF {
		t.Fatalf("err = %v, want EOF", err)
	}
}

// TestWriteFrameChunksLargePayloads: a payload beyond MaxPayload arrives as
// several frames whose payloads concatenate back to the original.
func TestWriteFrameChunksLargePayloads(t *testing.T) {
	var buf bytes.Buffer
	big := bytes.Repeat([]byte("x"), MaxPayload+MaxPayload/2)
	if err := NewWriter(&buf).WriteFrame(FrameStdout, big); err != nil {
		t.Fatal(err)
	}
	var got []byte
	frames := 0
	for buf.Len() > 0 {
		f, err := ReadFrame(&buf)
		if err != nil {
			t.Fatal(err)
		}
		if f.Type != FrameStdout {
			t.Fatalf("frame type = %d, want stdout", f.Type)
		}
		got = append(got, f.Payload...)
		frames++
	}
	if frames != 2 || !bytes.Equal(got, big) {
		t.Fatalf("got %d frames, %d bytes; want 2 frames reassembling the payload", frames, len(got))
	}
}

// TestReadFrameRejectsOversizedHeader: a peer can't force an arbitrary
// allocation by declaring a huge payload.
func TestReadFrameRejectsOversizedHeader(t *testing.T) {
	hdr := []byte{FrameStdout, 0xFF, 0xFF, 0xFF, 0xFF}
	if _, err := ReadFrame(bytes.NewReader(hdr)); err == nil {
		t.Fatal("expected an error for an over-cap payload length")
	}
}

func TestResizePayloadRoundTrip(t *testing.T) {
	cols, rows, err := DecodeResize(ResizePayload(120, 40))
	if err != nil || cols != 120 || rows != 40 {
		t.Fatalf("resize = %dx%d, %v", cols, rows, err)
	}
	if _, _, err := DecodeResize([]byte{1, 2, 3}); err == nil {
		t.Fatal("expected an error for a short resize payload")
	}
}
