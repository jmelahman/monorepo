package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/gif" // decoded, then written as JPEG
	"image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unicode"
)

// An album's cover can be set by hand: picked from what MusicBrainz
// suggests, uploaded, or fetched from a link. It is written as the
// folder's cover.jpg, which is looked for before any other art, so
// nothing else in the folder is touched.

// coverMaxUpload bounds an image sent or fetched; covers are well under.
const coverMaxUpload = 20 << 20

// coverChoice is one release group the cover search suggests.
type coverChoice struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Artist string `json:"artist"`
	Type   string `json:"type,omitempty"`
	Year   string `json:"year,omitempty"`
}

// suggest asks MusicBrainz for release groups like the words given, by
// the artist when there is one. Unlike find it doesn't insist the names
// match: the person choosing sees the covers and picks.
func (f *coverFetcher) suggest(ctx context.Context, artist, album string) ([]coverChoice, error) {
	q := "releasegroup:(" + words(album) + ")"
	if a := words(artist); a != "" {
		q += " AND artist:(" + a + ")"
	}
	body, err := f.get(ctx, musicBrainz+"/release-group/?fmt=json&limit=16&query="+url.QueryEscape(q), true)
	if err != nil {
		return nil, err
	}
	defer body.Close()
	var out struct {
		Groups []struct {
			releaseGroup
			Type  string `json:"primary-type"`
			First string `json:"first-release-date"`
		} `json:"release-groups"`
	}
	if err := json.NewDecoder(body).Decode(&out); err != nil {
		return nil, fmt.Errorf("musicbrainz: %w", err)
	}
	choices := make([]coverChoice, 0, len(out.Groups))
	for _, g := range out.Groups {
		choices = append(choices, coverChoice{ID: g.ID, Title: g.Title, Artist: g.credit(), Type: g.Type, Year: strings.SplitN(g.First, "-", 2)[0]})
	}
	return choices, nil
}

// words is a name as plain search terms: a folder's underscores and
// Lucene's punctuation become spaces, a disc's number goes, and it is
// lowercased, so a title's "And" or "Not" isn't read as an operator.
func words(s string) string {
	s = strings.ToLower(discless(folderName(s)))
	return strings.Join(strings.FieldsFunc(s, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '\''
	}), " ")
}

// writeCover makes img the folder's cover.jpg: a JPEG as it came, any
// other picture re-encoded. The file goes to the folder's owner, as the
// tracks beside it do, and the server looks for the folder's art afresh.
func writeCover(dir string, img []byte) error {
	_, format, err := image.DecodeConfig(bytes.NewReader(img))
	if err != nil {
		return errNotImage
	}
	if format != "jpeg" {
		m, _, err := image.Decode(bytes.NewReader(img))
		if err != nil {
			return errNotImage
		}
		var b bytes.Buffer
		if err := jpeg.Encode(&b, m, &jpeg.Options{Quality: 90}); err != nil {
			return err
		}
		img = b.Bytes()
	}
	tmp, err := os.CreateTemp(dir, ".cover-*.jpg")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(img); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	_ = os.Chmod(tmp.Name(), 0o644)
	if info, err := os.Stat(dir); err == nil {
		if st, ok := info.Sys().(*syscall.Stat_t); ok {
			_ = os.Chown(tmp.Name(), int(st.Uid), int(st.Gid))
		}
	}
	if err := os.Rename(tmp.Name(), filepath.Join(dir, "cover.jpg")); err != nil {
		return err
	}
	covers.Lock()
	delete(covers.from, dir)
	covers.Unlock()
	return nil
}

var errNotImage = errors.New("that isn't a JPEG, PNG or GIF picture")

// fetchImage reads a picture from a link someone pasted.
func (f *coverFetcher) fetchImage(ctx context.Context, link string) ([]byte, error) {
	u, err := url.Parse(link)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, errors.New("the link must be an http or https address")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", coverAgent)
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("%s answered %s", u.Host, resp.Status)
	}
	return readLimited(resp.Body)
}

func readLimited(r io.Reader) ([]byte, error) {
	b, err := io.ReadAll(io.LimitReader(r, coverMaxUpload+1))
	if err != nil {
		return nil, err
	}
	if len(b) > coverMaxUpload {
		return nil, errors.New("that picture is over 20 MB")
	}
	return b, nil
}

// routeCoverEdit serves the cover search and the setting of a cover.
// albumDir names the folder an artist and album go by, or fails.
func routeCoverEdit(mux *http.ServeMux, albumDir func(artist, album string) (string, error)) {
	f := newCoverFetcher()
	mux.HandleFunc("GET /api/cover/search", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
		defer cancel()
		choices, err := f.suggest(ctx, q.Get("artist"), q.Get("album"))
		if err != nil {
			http.Error(w, "MusicBrainz didn't answer: "+err.Error(), http.StatusBadGateway)
			return
		}
		writeJSON(w, choices)
	})
	mux.HandleFunc("PUT /api/cover", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		dir, err := albumDir(q.Get("artist"), q.Get("album"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
		defer cancel()
		var img []byte
		if strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
			var body struct {
				ReleaseGroup string `json:"releaseGroup"`
				URL          string `json:"url"`
			}
			if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&body); err != nil {
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}
			switch {
			case body.ReleaseGroup != "":
				if strings.ContainsAny(body.ReleaseGroup, "/?#") {
					http.Error(w, "bad release group", http.StatusBadRequest)
					return
				}
				img, err = f.fetchImage(ctx, coverArtArchive+"/release-group/"+body.ReleaseGroup+"/front-500")
			case body.URL != "":
				img, err = f.fetchImage(ctx, body.URL)
			default:
				http.Error(w, "nothing to set the cover from", http.StatusBadRequest)
				return
			}
		} else {
			img, err = readLimited(r.Body)
		}
		if err != nil {
			http.Error(w, "couldn't get the picture: "+err.Error(), http.StatusBadGateway)
			return
		}
		if err := writeCover(dir, img); err != nil {
			code := http.StatusInternalServerError
			if errors.Is(err, errNotImage) {
				code = http.StatusUnsupportedMediaType
			}
			http.Error(w, err.Error(), code)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}
