package main

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// id3 builds a tag of the given version holding the frames.
func id3(version byte, frames ...[]byte) []byte {
	body := bytes.Join(frames, nil)
	n := len(body)
	tag := make([]byte, 0, 10+n+64)
	tag = append(tag, 'I', 'D', '3', version, 0, 0,
		byte(n>>21&0x7f), byte(n>>14&0x7f), byte(n>>7&0x7f), byte(n&0x7f))
	tag = append(tag, body...)
	return append(tag, make([]byte, 64)...) // padding, then audio
}

// apic builds a picture frame; v2.4 sizes are syncsafe, v2.3 plain.
func apic(version byte, mime string, picType byte, desc []byte, img []byte) []byte {
	body := []byte{0}
	if len(desc) > 0 && desc[0] == 0xff {
		body[0] = 1 // UTF-16 with a BOM
	}
	body = append(body, mime...)
	body = append(body, 0, picType)
	body = append(body, desc...)
	if body[0] == 1 {
		body = append(body, 0, 0)
	} else {
		body = append(body, 0)
	}
	body = append(body, img...)
	n := len(body)
	size := make([]byte, 4)
	if version == 4 {
		size = []byte{byte(n >> 21 & 0x7f), byte(n >> 14 & 0x7f), byte(n >> 7 & 0x7f), byte(n & 0x7f)}
	} else {
		binary.BigEndian.PutUint32(size, uint32(n))
	}
	return append(append(append([]byte("APIC"), size...), 0, 0), body...)
}

func TestEmbeddedCover(t *testing.T) {
	other := []byte("\x89PNG-back")
	front := []byte("\xff\xd8\xff-front")
	cases := map[string][]byte{
		"v2.3":  id3(3, apic(3, "image/png", 4, []byte("back"), other), apic(3, "image/jpeg", 3, nil, front)),
		"v2.4":  id3(4, apic(4, "image/png", 4, nil, other), apic(4, "image/jpeg", 3, []byte("cover"), front)),
		"utf16": id3(3, apic(3, "image/jpeg", 3, []byte{0xff, 0xfe, 'c', 0}, front)),
	}
	for name, tag := range cases {
		t.Run(name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "a.mp3")
			if err := os.WriteFile(p, tag, 0o644); err != nil {
				t.Fatal(err)
			}
			img, mime, err := embeddedCover(p)
			if err != nil || mime != "image/jpeg" || !bytes.Equal(img, front) {
				t.Fatalf("got %q %q %v, want the front cover", img, mime, err)
			}
		})
	}
}

func TestFindCover(t *testing.T) {
	dir := t.TempDir()
	if got := findCover(dir, false); got != "" {
		t.Fatalf("an empty folder has art: %q", got)
	}
	front := []byte("\xff\xd8\xff")
	_ = os.WriteFile(filepath.Join(dir, "01 a.mp3"), id3(3), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "02 b.mp3"), id3(3, apic(3, "image/jpeg", 3, nil, front)), 0o644)
	if got := findCover(dir, false); filepath.Base(got) != "02 b.mp3" {
		t.Fatalf("the track with a picture: %q", got)
	}
	_ = os.WriteFile(filepath.Join(dir, "Folder.JPG"), front, 0o644)
	if got := findCover(dir, false); filepath.Base(got) != "Folder.JPG" {
		t.Fatalf("the folder's own image first: %q", got)
	}
	if got := findCover(dir, true); filepath.Base(got) != "02 b.mp3" {
		t.Fatalf("a loose folder's tracks only: %q", got)
	}
}
