package main

import (
	"bytes"
	"encoding/binary"
	"io"
	"math"
	"os"
	"path/filepath"
	"sync"
)

// lengths reads how long each track plays, for the library's totals, and
// what its tag says of its genre and year. An mp3 says how long in its
// first frame (a Xing, Info or VBRI header) or, at a constant bitrate,
// by its size; either way only the head of the file is read. What was
// read is kept while the file's size and time stay put.
type lengths struct {
	mu   sync.Mutex
	seen map[string]length
}

type length struct {
	size, added int64
	secs        float64
	genre       string // as the tag has it
	year        int
}

// fill sets each track's Duration, reading the files it hasn't yet.
func (l *lengths) fill(root string, artists []artist) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.seen == nil {
		l.seen = map[string]length{}
	}
	for i := range artists {
		for j := range artists[i].Albums {
			for k := range artists[i].Albums[j].Tracks {
				t := &artists[i].Albums[j].Tracks[k]
				t.Duration = l.ofLocked(root, *t).secs
			}
		}
	}
}

// of is what was read of one track's file.
func (l *lengths) of(root string, t track) length {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.ofLocked(root, t)
}

func (l *lengths) ofLocked(root string, t track) length {
	if l.seen == nil {
		l.seen = map[string]length{}
	}
	got, ok := l.seen[t.Path]
	if !ok || got.size != t.Size || got.added != t.Added {
		name := filepath.Join(root, filepath.FromSlash(t.Path))
		got = length{size: t.Size, added: t.Added, secs: mp3Duration(name)}
		got.genre, got.year = fileTags(name)
		l.seen[t.Path] = got
	}
	return got
}

// mp3Duration is how many seconds an mp3 plays, or 0 when it can't tell.
func mp3Duration(name string) float64 {
	f, err := os.Open(name)
	if err != nil {
		return 0
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return 0
	}
	head := make([]byte, 10)
	if _, err := io.ReadFull(f, head); err != nil {
		return 0
	}
	// An ID3v2 tag comes first, its size in four seven-bit bytes.
	var start int64
	if bytes.HasPrefix(head, []byte("ID3")) {
		start = 10 + int64(head[6]&0x7f)<<21 | int64(head[7]&0x7f)<<14 | int64(head[8]&0x7f)<<7 | int64(head[9]&0x7f)
		if head[5]&0x10 != 0 { // a footer repeats the header
			start += 10
		}
	}
	buf := make([]byte, 8<<10)
	n, _ := f.ReadAt(buf, start)
	buf = buf[:n]
	audio := info.Size() - start
	if tail := make([]byte, 3); info.Size() > 128 {
		if _, err := f.ReadAt(tail, info.Size()-128); err == nil && string(tail) == "TAG" {
			audio -= 128 // an ID3v1 tag at the end
		}
	}
	for i := 0; i+4 <= len(buf); i++ {
		if fr, ok := frameAt(buf[i:]); ok {
			return math.Round(fr.duration(buf[i:], audio-int64(i))*10) / 10
		}
	}
	return 0
}

// frame is an MPEG audio frame's header, as far as the length needs it.
type frame struct {
	v1         bool // MPEG-1, rather than 2 or 2.5
	mono       bool
	kbps, rate int
}

var (
	kbpsV1 = [16]int{0, 32, 40, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320}
	kbpsV2 = [16]int{0, 8, 16, 24, 32, 40, 48, 56, 64, 80, 96, 112, 128, 144, 160}
)

// frameAt reads a Layer III frame header at the start of b.
func frameAt(b []byte) (frame, bool) {
	if b[0] != 0xff || b[1]&0xe0 != 0xe0 {
		return frame{}, false
	}
	version, layer := b[1]>>3&3, b[1]>>1&3
	bitrate, rate := b[2]>>4, b[2]>>2&3
	if version == 1 || layer != 1 || bitrate == 0 || bitrate == 15 || rate == 3 {
		return frame{}, false
	}
	fr := frame{v1: version == 3, mono: b[3]>>6 == 3, rate: [3]int{44100, 48000, 32000}[rate]}
	switch version {
	case 3:
		fr.kbps = kbpsV1[bitrate]
	case 2:
		fr.kbps, fr.rate = kbpsV2[bitrate], fr.rate/2
	default: // 2.5
		fr.kbps, fr.rate = kbpsV2[bitrate], fr.rate/4
	}
	return fr, true
}

// duration is how long the file plays from this frame on: the frame
// count a VBR header gives, or else the bytes left at this bitrate.
func (fr frame) duration(b []byte, audio int64) float64 {
	samples := 1152
	side := 32
	switch {
	case !fr.v1 && fr.mono:
		samples, side = 576, 9
	case !fr.v1:
		samples, side = 576, 17
	case fr.mono:
		side = 17
	}
	if x := 4 + side; len(b) >= x+12 {
		if tag := string(b[x : x+4]); (tag == "Xing" || tag == "Info") && b[x+7]&1 != 0 {
			return float64(binary.BigEndian.Uint32(b[x+8:])) * float64(samples) / float64(fr.rate)
		}
	}
	if len(b) >= 36+18 && string(b[36:40]) == "VBRI" {
		return float64(binary.BigEndian.Uint32(b[36+14:])) * float64(samples) / float64(fr.rate)
	}
	return float64(audio) * 8 / float64(fr.kbps*1000)
}
