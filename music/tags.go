package main

import (
	"bytes"
	"cmp"
	"encoding/binary"
	"io"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf16"
)

// What a file says of itself: its genre and year, from the ID3 tag an
// mp3 carries. Only the frames' headers and the two text frames are
// read, so the cover art a tag often holds is skipped over.

// fileTags reads an mp3's genre, as written, and year; "" and 0 when it
// has none.
func fileTags(name string) (genre string, year int) {
	f, err := os.Open(name)
	if err != nil {
		return "", 0
	}
	defer f.Close()
	genre, year = id3v2(f)
	if genre == "" || year == 0 {
		g, y := id3v1(f)
		genre = cmp.Or(genre, g)
		if year == 0 {
			year = y
		}
	}
	return genre, year
}

// id3v2 walks the tag at the head of the file, its 2.2, 2.3 or 2.4 form.
func id3v2(f io.ReaderAt) (genre string, year int) {
	head := make([]byte, 10)
	if _, err := f.ReadAt(head, 0); err != nil || !bytes.HasPrefix(head, []byte("ID3")) {
		return "", 0
	}
	ver, flags := head[3], head[5]
	end := 10 + synchsafe(head[6:10])
	at := int64(10)
	if flags&0x40 != 0 && ver >= 3 { // an extended header comes first
		ext := make([]byte, 4)
		if _, err := f.ReadAt(ext, at); err != nil {
			return "", 0
		}
		if ver == 3 {
			at += 4 + int64(binary.BigEndian.Uint32(ext))
		} else {
			at += synchsafe(ext)
		}
	}
	idLen, hdrLen := 4, 10
	genreID, yearIDs := "TCON", []string{"TYER", "TDRC", "TORY", "TDOR"}
	if ver == 2 {
		idLen, hdrLen = 3, 6
		genreID, yearIDs = "TCO", []string{"TYE", "TOR"}
	}
	hdr := make([]byte, hdrLen)
	for at+int64(hdrLen) <= end {
		if _, err := f.ReadAt(hdr, at); err != nil || hdr[0] == 0 {
			break
		}
		id := string(hdr[:idLen])
		var size int64
		switch ver {
		case 2:
			size = int64(hdr[3])<<16 | int64(hdr[4])<<8 | int64(hdr[5])
		case 3:
			size = int64(binary.BigEndian.Uint32(hdr[4:8]))
		default:
			size = synchsafe(hdr[4:8])
		}
		at += int64(hdrLen)
		if size <= 0 || at+size > end {
			break
		}
		if (id == genreID || slices.Contains(yearIDs, id)) && size < 1024 {
			body := make([]byte, size)
			if _, err := f.ReadAt(body, at); err == nil {
				text := id3Text(body)
				if id == genreID && genre == "" {
					genre = text
				} else if year == 0 {
					year = leadingYear(text)
				}
			}
		}
		at += size
	}
	return genre, year
}

// id3v1 reads the old tag at the very end of the file.
func id3v1(f *os.File) (genre string, year int) {
	info, err := f.Stat()
	if err != nil || info.Size() < 128 {
		return "", 0
	}
	b := make([]byte, 128)
	if _, err := f.ReadAt(b, info.Size()-128); err != nil || string(b[:3]) != "TAG" {
		return "", 0
	}
	if int(b[127]) < len(id3Genres) {
		genre = id3Genres[b[127]]
	}
	return genre, leadingYear(string(b[93:97]))
}

func synchsafe(b []byte) int64 {
	return int64(b[0]&0x7f)<<21 | int64(b[1]&0x7f)<<14 | int64(b[2]&0x7f)<<7 | int64(b[3]&0x7f)
}

// id3Text decodes a text frame, its first byte naming the encoding. Of
// several values (2.4 parts them with a NUL) the first is kept.
func id3Text(b []byte) string {
	if len(b) < 2 {
		return ""
	}
	enc, b := b[0], b[1:]
	var s string
	switch enc {
	case 1, 2: // UTF-16, with a byte order mark or big-endian
		big := enc == 2
		if len(b) >= 2 && b[0] == 0xfe && b[1] == 0xff {
			big, b = true, b[2:]
		} else if len(b) >= 2 && b[0] == 0xff && b[1] == 0xfe {
			big, b = false, b[2:]
		}
		u := make([]uint16, 0, len(b)/2)
		for i := 0; i+1 < len(b); i += 2 {
			if big {
				u = append(u, binary.BigEndian.Uint16(b[i:]))
			} else {
				u = append(u, binary.LittleEndian.Uint16(b[i:]))
			}
		}
		s = string(utf16.Decode(u))
	case 3:
		s = string(b)
	default: // ISO-8859-1
		r := make([]rune, len(b))
		for i, c := range b {
			r[i] = rune(c)
		}
		s = string(r)
	}
	s, _, _ = strings.Cut(s, "\x00")
	return strings.TrimSpace(s)
}

func leadingYear(s string) int {
	s = strings.TrimSpace(s)
	if len(s) < 4 {
		return 0
	}
	y, err := strconv.Atoi(s[:4])
	if err != nil || y < 1000 || y > 3000 {
		return 0
	}
	return y
}

// id3Genres are ID3v1's numbered genres, which a tag may still give as
// "(17)" or "17".
var id3Genres = []string{
	"Blues", "Classic Rock", "Country", "Dance", "Disco", "Funk", "Grunge", "Hip-Hop", "Jazz", "Metal",
	"New Age", "Oldies", "Other", "Pop", "R&B", "Rap", "Reggae", "Rock", "Techno", "Industrial",
	"Alternative", "Ska", "Death Metal", "Pranks", "Soundtrack", "Euro-Techno", "Ambient", "Trip-Hop", "Vocal", "Jazz+Funk",
	"Fusion", "Trance", "Classical", "Instrumental", "Acid", "House", "Game", "Sound Clip", "Gospel", "Noise",
	"Alternative Rock", "Bass", "Soul", "Punk", "Space", "Meditative", "Instrumental Pop", "Instrumental Rock", "Ethnic", "Gothic",
	"Darkwave", "Techno-Industrial", "Electronic", "Pop-Folk", "Eurodance", "Dream", "Southern Rock", "Comedy", "Cult", "Gangsta",
	"Top 40", "Christian Rap", "Pop/Funk", "Jungle", "Native American", "Cabaret", "New Wave", "Psychedelic", "Rave", "Showtunes",
	"Trailer", "Lo-Fi", "Tribal", "Acid Punk", "Acid Jazz", "Polka", "Retro", "Musical", "Rock & Roll", "Hard Rock",
}

var numberedGenre = regexp.MustCompile(`^\((\d+)\)(.*)$`)

// genreAliases folds the spellings one genre goes by into one.
var genreAliases = map[string]string{
	"hip hop": "hip-hop", "hiphop": "hip-hop", "rap": "hip-hop", "hip-hop/rap": "hip-hop",
	"hip hop/rap": "hip-hop", "rap/hip hop": "hip-hop", "rap/hip-hop": "hip-hop", "hip-hop & rap": "hip-hop",
	"rnb": "r&b", "r & b": "r&b", "r&b/soul": "r&b", "rhythm and blues": "r&b",
	"drum and bass": "drum & bass", "drum n bass": "drum & bass", "drum'n'bass": "drum & bass",
	"dnb": "drum & bass", "d&b": "drum & bass", "drum&bass": "drum & bass",
	"nu-metal": "nu metal", "numetal": "nu metal",
	"electronica": "electronic", "electronic music": "electronic", "electro": "electronic",
	"synth-pop": "synthpop", "synth pop": "synthpop",
	"trip hop": "trip-hop", "triphop": "trip-hop",
	"lofi": "lo-fi", "lo fi": "lo-fi",
	"kpop": "k-pop", "k pop": "k-pop",
	"alternrock": "alternative rock", "alt rock": "alternative rock", "alt-rock": "alternative rock",
	"alternative & punk": "alternative", "alt": "alternative",
	"indie rock & pop": "indie", "soundtracks": "soundtrack",
}

// genresDropped say nothing of how a track sounds.
var genresDropped = map[string]bool{"": true, "other": true, "unknown": true, "genre": true, "misc": true, "none": true}

// normalizeGenre puts a genre the way the labels keep them: lower case,
// one spelling for each, "" for one that says nothing.
func normalizeGenre(g string) string {
	g = strings.TrimSpace(g)
	if m := numberedGenre.FindStringSubmatch(g); m != nil {
		if rest := strings.TrimSpace(m[2]); rest != "" {
			g = rest
		} else if n, _ := strconv.Atoi(m[1]); n < len(id3Genres) {
			g = id3Genres[n]
		}
	} else if n, err := strconv.Atoi(g); err == nil {
		g = ""
		if n >= 0 && n < len(id3Genres) {
			g = id3Genres[n]
		}
	}
	g = strings.Join(strings.Fields(strings.ToLower(g)), " ")
	if a, ok := genreAliases[g]; ok {
		g = a
	}
	if genresDropped[g] {
		return ""
	}
	return g
}

// genreTitle is how a genre reads in a mix's name.
func genreTitle(g string) string {
	switch g {
	case "r&b":
		return "R&B"
	case "edm", "idm", "ebm":
		return strings.ToUpper(g)
	}
	b := []byte(g)
	up := true
	for i, c := range b {
		if up && c >= 'a' && c <= 'z' {
			b[i] = c - 'a' + 'A'
		}
		up = c == ' ' || c == '-' || c == '/' || c == '(' || c == '&'
	}
	s := string(b)
	return strings.NewReplacer("Uk ", "UK ", "Dj ", "DJ ", "&Amp;", "&").Replace(s)
}
