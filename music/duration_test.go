package main

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// cbrHeader is an MPEG-1 Layer III frame header: 128 kbps, 44.1 kHz,
// joint stereo.
var cbrHeader = []byte{0xff, 0xfb, 0x90, 0x64}

func writeMP3(t *testing.T, parts ...[]byte) string {
	t.Helper()
	name := filepath.Join(t.TempDir(), "a.mp3")
	if err := os.WriteFile(name, bytes.Join(parts, nil), 0o644); err != nil {
		t.Fatal(err)
	}
	return name
}

func TestMP3Duration(t *testing.T) {
	// 20 bytes of ID3v2 tag, then a second of 128 kbps audio.
	id3 := append([]byte("ID3\x04\x00\x00\x00\x00\x00\x14"), make([]byte, 20)...)
	audio := append(append([]byte{}, cbrHeader...), make([]byte, 16000-len(cbrHeader))...)
	if got := mp3Duration(writeMP3(t, id3, audio)); got != 1 {
		t.Errorf("constant bitrate: got %v, want 1", got)
	}

	// A Xing header counting 100 frames of 1152 samples at 44.1 kHz.
	xing := append(append([]byte{}, cbrHeader...), make([]byte, 32)...)
	xing = append(xing, "Xing\x00\x00\x00\x01"...)
	xing = binary.BigEndian.AppendUint32(xing, 100)
	xing = append(xing, make([]byte, 400)...)
	if got := mp3Duration(writeMP3(t, xing)); got != 2.6 {
		t.Errorf("Xing: got %v, want 2.6", got)
	}

	if got := mp3Duration(writeMP3(t, []byte("not an mp3 at all"))); got != 0 {
		t.Errorf("not an mp3: got %v, want 0", got)
	}
}

func TestLengthsFill(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "Artist", "Album")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	audio := append(append([]byte{}, cbrHeader...), make([]byte, 32000-len(cbrHeader))...)
	if err := os.WriteFile(filepath.Join(dir, "01 Song.mp3"), audio, 0o644); err != nil {
		t.Fatal(err)
	}
	artists, err := walkLibrary(root)
	if err != nil {
		t.Fatal(err)
	}
	var l lengths
	l.fill(root, artists)
	if got := artists[0].Albums[0].Tracks[0].Duration; got != 2 {
		t.Fatalf("got %v, want 2", got)
	}
}
