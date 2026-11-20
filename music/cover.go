package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

// coverNames are the images a folder's cover goes by, in the order they
// are looked for; jellyfin reads the same names.
var coverNames = []string{"cover.jpg", "cover.png", "folder.jpg", "folder.png", "front.jpg", "front.png", "album.jpg"}

// coverMaxTag bounds how much of a file is read for its tag, so a
// corrupt size field can't make a request read a whole file.
const coverMaxTag = 16 << 20

// covers remembers, per folder, which file its art came from, so a grid
// of albums is one directory scan each rather than a tag read per track.
// A folder found bare is looked at again after coverRecheck, in case a buy
// has since brought it art.
var covers = struct {
	sync.Mutex
	from map[string]coverSource
}{from: map[string]coverSource{}}

type coverSource struct {
	path  string // the image or track, "" for none
	found time.Time
}

const coverRecheck = 10 * time.Minute

// serveCover answers the art for a folder of the library: an image
// beside the tracks, or else the first picture embedded in one of them.
func serveCover(w http.ResponseWriter, r *http.Request, dir string, loose bool) {
	covers.Lock()
	cached, known := covers.from[dir]
	covers.Unlock()
	src := cached.path
	if !known || (src == "" && time.Since(cached.found) > coverRecheck) {
		src = findCover(dir, loose)
		covers.Lock()
		covers.from[dir] = coverSource{src, time.Now()}
		covers.Unlock()
	}
	if src == "" {
		http.NotFound(w, r)
		return
	}
	// The art changes only when the library does; a day is plenty.
	w.Header().Set("Cache-Control", "max-age=86400")
	if !isAudio(src) {
		http.ServeFile(w, r, src)
		return
	}
	img, mime, err := embeddedCover(src)
	if err != nil {
		covers.Lock()
		delete(covers.from, dir)
		covers.Unlock()
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", mime)
	http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(img))
}

// findCover picks a folder's art: a named image first, then the first
// track, in order, that carries a picture. A loose folder is an artist's
// own, whose tracks are singles; only those are looked at, not the
// albums beneath it.
func findCover(dir string, loose bool) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	if !loose {
		for _, name := range coverNames {
			i := slices.IndexFunc(entries, func(e os.DirEntry) bool { return strings.EqualFold(e.Name(), name) })
			if i >= 0 && !entries[i].IsDir() {
				return filepath.Join(dir, entries[i].Name())
			}
		}
	}
	slices.SortFunc(entries, func(a, b os.DirEntry) int { return natural(a.Name(), b.Name()) })
	for _, e := range entries {
		if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".mp3") {
			continue
		}
		p := filepath.Join(dir, e.Name())
		if _, _, err := embeddedCover(p); err == nil {
			return p
		}
	}
	return ""
}

var errNoCover = errors.New("no embedded picture")

// embeddedCover reads the picture an MP3's ID3v2 tag carries: APIC in
// v2.3 and v2.4, PIC in v2.2. A front cover wins over any other picture.
func embeddedCover(name string) ([]byte, string, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, "", err
	}
	defer f.Close()
	var hdr [10]byte
	if _, err := io.ReadFull(f, hdr[:]); err != nil {
		return nil, "", err
	}
	if string(hdr[:3]) != "ID3" {
		return nil, "", errNoCover
	}
	version, flags := hdr[3], hdr[5]
	size := syncsafe(hdr[6:10])
	if size > coverMaxTag {
		return nil, "", errNoCover
	}
	tag := make([]byte, size)
	if _, err := io.ReadFull(f, tag); err != nil {
		return nil, "", err
	}
	if flags&0x80 != 0 {
		// Unsynchronised: every 0xFF 0x00 was written for a 0xFF.
		tag = bytes.ReplaceAll(tag, []byte{0xFF, 0x00}, []byte{0xFF})
	}
	if flags&0x40 != 0 && version >= 3 && len(tag) >= 4 {
		// An extended header, skipped by its own size.
		ext := int(binary.BigEndian.Uint32(tag[:4]))
		if version == 4 {
			ext = syncsafe(tag[:4])
		} else {
			ext += 4
		}
		if ext > len(tag) {
			return nil, "", errNoCover
		}
		tag = tag[ext:]
	}
	return pictureFrame(tag, version)
}

// pictureFrame walks a tag's frames for the picture.
func pictureFrame(tag []byte, version byte) ([]byte, string, error) {
	idLen, hdrLen := 4, 10
	if version == 2 {
		idLen, hdrLen = 3, 6
	}
	var found []byte
	var mime string
	for len(tag) >= hdrLen && tag[0] != 0 {
		id := string(tag[:idLen])
		var size int
		switch version {
		case 2:
			size = int(tag[3])<<16 | int(tag[4])<<8 | int(tag[5])
		case 3:
			size = int(binary.BigEndian.Uint32(tag[4:8]))
		default:
			size = syncsafe(tag[4:8])
		}
		if size < 0 || hdrLen+size > len(tag) {
			break
		}
		body := tag[hdrLen : hdrLen+size]
		tag = tag[hdrLen+size:]
		if id != "APIC" && id != "PIC" {
			continue
		}
		img, m, front, ok := parsePicture(body, version == 2)
		if !ok {
			continue
		}
		if front {
			return img, m, nil
		}
		if found == nil {
			found, mime = img, m
		}
	}
	if found == nil {
		return nil, "", errNoCover
	}
	return found, mime, nil
}

// parsePicture splits an APIC (or v2.2 PIC) frame's body into the image
// and its type, and says whether it is the front cover.
func parsePicture(b []byte, v22 bool) (img []byte, mime string, front, ok bool) {
	if len(b) < 2 {
		return nil, "", false, false
	}
	enc := b[0]
	b = b[1:]
	if v22 {
		if len(b) < 3 {
			return nil, "", false, false
		}
		switch strings.ToUpper(string(b[:3])) {
		case "PNG":
			mime = "image/png"
		default:
			mime = "image/jpeg"
		}
		b = b[3:]
	} else {
		end := bytes.IndexByte(b, 0)
		if end < 0 {
			return nil, "", false, false
		}
		mime = strings.ToLower(string(b[:end]))
		b = b[end+1:]
		switch mime {
		case "", "image/jpg", "jpg", "jpeg":
			mime = "image/jpeg"
		case "png":
			mime = "image/png"
		}
	}
	if len(b) < 1 {
		return nil, "", false, false
	}
	front = b[0] == 3
	b = b[1:]
	// The description ends in one zero byte, or two in the UTF-16
	// encodings, aligned to the character.
	if enc == 1 || enc == 2 {
		i := 0
		for ; i+1 < len(b); i += 2 {
			if b[i] == 0 && b[i+1] == 0 {
				break
			}
		}
		if i+1 >= len(b) {
			return nil, "", false, false
		}
		b = b[i+2:]
	} else {
		end := bytes.IndexByte(b, 0)
		if end < 0 {
			return nil, "", false, false
		}
		b = b[end+1:]
	}
	if len(b) == 0 || !strings.HasPrefix(mime, "image/") {
		return nil, "", false, false
	}
	return b, mime, front, true
}

// syncsafe reads ID3's 28-bit integers, seven bits to a byte.
func syncsafe(b []byte) int {
	return int(b[0]&0x7f)<<21 | int(b[1]&0x7f)<<14 | int(b[2]&0x7f)<<7 | int(b[3]&0x7f)
}
