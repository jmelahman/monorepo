package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"
)

// Album art the library lacks comes from MusicBrainz, which names the
// album, and the Cover Art Archive, which holds its front cover. Neither
// wants a key; MusicBrainz asks for one request a second and a
// User-Agent that says who is asking.
var (
	musicBrainz     = "https://musicbrainz.org/ws/2"
	coverArtArchive = "https://coverartarchive.org"
)

const coverAgent = "music/1 ( https://github.com/jmelahman/monorepo )"

// coverFetcher looks albums up, one MusicBrainz request a second.
type coverFetcher struct {
	client *http.Client
	mu     sync.Mutex
	last   time.Time
	gap    time.Duration
}

func newCoverFetcher() *coverFetcher {
	return &coverFetcher{client: &http.Client{Timeout: 30 * time.Second}, gap: 1100 * time.Millisecond}
}

// coverMatch is the album MusicBrainz found for a folder.
type coverMatch struct {
	ID     string // the release group
	Artist string
	Title  string
}

var errNoMatch = errors.New("no matching album with art")

// find looks an album up by the names its folders go by. Only a release
// group whose title and artist match the folder's, once punctuation and
// edition notes are set aside, counts: a wrong cover is worse than none.
// It answers the first match the archive has a front cover for.
func (f *coverFetcher) find(ctx context.Context, artist, album string) (coverMatch, error) {
	titles := []string{album}
	if bare := stripEdition(album); bare != album && bare != "" {
		titles = append(titles, bare)
	}
	for _, title := range titles {
		groups, err := f.search(ctx, artist, title)
		if err != nil {
			return coverMatch{}, err
		}
		for _, g := range groups {
			if !sameName(g.Title, album) || !sameName(g.credit(), artist) {
				continue
			}
			ok, err := f.hasFront(ctx, g.ID)
			if err != nil {
				return coverMatch{}, err
			}
			if ok {
				return coverMatch{ID: g.ID, Artist: g.credit(), Title: g.Title}, nil
			}
		}
	}
	return coverMatch{}, errNoMatch
}

type releaseGroup struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Credit []struct {
		Name       string `json:"name"`
		JoinPhrase string `json:"joinphrase"`
	} `json:"artist-credit"`
}

// credit is the group's artist as MusicBrainz prints it.
func (g releaseGroup) credit() string {
	var b strings.Builder
	for _, c := range g.Credit {
		b.WriteString(c.Name + c.JoinPhrase)
	}
	return b.String()
}

func (f *coverFetcher) search(ctx context.Context, artist, title string) ([]releaseGroup, error) {
	q := fmt.Sprintf(`releasegroup:"%s" AND artist:"%s"`, phrase(title), phrase(artist))
	body, err := f.get(ctx, musicBrainz+"/release-group/?fmt=json&limit=10&query="+url.QueryEscape(q), true)
	if err != nil {
		return nil, err
	}
	defer body.Close()
	var out struct {
		Groups []releaseGroup `json:"release-groups"`
	}
	if err := json.NewDecoder(body).Decode(&out); err != nil {
		return nil, fmt.Errorf("musicbrainz: %w", err)
	}
	return out.Groups, nil
}

// hasFront says whether the archive holds a front cover for the group.
func (f *coverFetcher) hasFront(ctx context.Context, id string) (bool, error) {
	body, err := f.get(ctx, coverArtArchive+"/release-group/"+id, false)
	if errors.Is(err, errNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer body.Close()
	var out struct {
		Images []struct {
			Front bool `json:"front"`
		} `json:"images"`
	}
	if err := json.NewDecoder(body).Decode(&out); err != nil {
		return false, fmt.Errorf("cover art archive: %w", err)
	}
	for _, im := range out.Images {
		if im.Front {
			return true, nil
		}
	}
	return false, nil
}

// save writes the group's front cover into dir as cover.jpg, which the
// archive's 500px thumbnails always are, and has the server look again.
func (f *coverFetcher) save(ctx context.Context, m coverMatch, dir string) error {
	body, err := f.get(ctx, coverArtArchive+"/release-group/"+m.ID+"/front-500", false)
	if err != nil {
		return err
	}
	defer body.Close()
	tmp, err := os.CreateTemp(dir, ".cover-*.jpg")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	// Covers are well under this; it only stops a runaway answer.
	if _, err := io.Copy(tmp, io.LimitReader(body, 16<<20)); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	_ = os.Chmod(tmp.Name(), 0o644)
	if err := os.Rename(tmp.Name(), filepath.Join(dir, "cover.jpg")); err != nil {
		return err
	}
	covers.Lock()
	delete(covers.from, dir)
	covers.Unlock()
	return nil
}

var errNotFound = errors.New("not found")

// get fetches a URL, waiting out MusicBrainz's pace when paced.
func (f *coverFetcher) get(ctx context.Context, u string, paced bool) (io.ReadCloser, error) {
	if paced {
		f.mu.Lock()
		wait := f.gap - time.Since(f.last)
		if wait > 0 {
			select {
			case <-time.After(wait):
			case <-ctx.Done():
				f.mu.Unlock()
				return nil, ctx.Err()
			}
		}
		f.last = time.Now()
		f.mu.Unlock()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", coverAgent)
	req.Header.Set("Accept", "application/json")
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	switch {
	case resp.StatusCode == http.StatusNotFound:
		resp.Body.Close()
		return nil, errNotFound
	case resp.StatusCode/100 != 2:
		resp.Body.Close()
		return nil, fmt.Errorf("%s: %s", req.URL.Host, resp.Status)
	}
	return resp.Body, nil
}

// phrase escapes a name for a quoted Lucene phrase.
func phrase(s string) string {
	return strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s)
}

// editionNote is a trailing note in brackets, like "(Deluxe Edition)".
var editionNote = regexp.MustCompile(`\s*[\(\[][^\)\]]*[\)\]]\s*$`)

func stripEdition(s string) string {
	for {
		t := editionNote.ReplaceAllString(s, "")
		if t == s {
			return strings.TrimSpace(t)
		}
		s = t
	}
}

// sameName compares names as letters and digits alone, edition notes
// aside, so "These Days..." is "These Days" and "Alt-J" is "alt‐J".
func sameName(a, b string) bool {
	return fold(stripEdition(a)) == fold(stripEdition(b)) && fold(a) != ""
}

func fold(s string) string {
	var b strings.Builder
	for _, r := range strings.ReplaceAll(s, "&", "and") {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}

// bareAlbums lists the library's album folders with no art at all.
func bareAlbums(music string) ([][2]string, error) {
	artists, err := walkLibrary(music)
	if err != nil {
		return nil, err
	}
	var out [][2]string
	for _, a := range artists {
		for _, al := range a.Albums {
			dir := filepath.Join(music, a.Name, al.Name)
			if info, err := os.Stat(dir); err != nil || !info.IsDir() {
				continue // the Singles gathered from loose tracks
			}
			if findCover(dir, false) == "" {
				out = append(out, [2]string{a.Name, al.Name})
			}
		}
	}
	return out, nil
}

// runCovers is `music covers`: it looks up every album without art and
// says what it found, saving the covers only when told to.
func runCovers(args []string) {
	fl := flag.NewFlagSet("covers", flag.ExitOnError)
	music := fl.String("music", defaultMusic(), "the owned library")
	save := fl.Bool("save", false, "write each match's cover.jpg into its album's folder")
	_ = fl.Parse(args) // ExitOnError
	bare, err := bareAlbums(*music)
	if err != nil {
		fail(err)
	}
	ctx := context.Background()
	f := newCoverFetcher()
	found := 0
	for _, a := range bare {
		m, err := f.find(ctx, a[0], a[1])
		switch {
		case errors.Is(err, errNoMatch):
			fmt.Printf("none   %s / %s\n", a[0], a[1])
			continue
		case err != nil:
			fmt.Printf("error  %s / %s: %v\n", a[0], a[1], err)
			continue
		}
		found++
		fmt.Printf("found  %s / %s  ->  %s — %s  https://musicbrainz.org/release-group/%s\n", a[0], a[1], m.Artist, m.Title, m.ID)
		if *save {
			if err := f.save(ctx, m, filepath.Join(*music, a[0], a[1])); err != nil {
				fmt.Printf("error  %s / %s: saving: %v\n", a[0], a[1], err)
			}
		}
	}
	verb := "would save"
	if *save {
		verb = "saved"
	}
	fmt.Printf("\n%d albums without art: %s %d covers\n", len(bare), verb, found)
}

// fetchCoverFor gives a bought track's album art when it has none; the
// buy has already landed, so a miss is only logged.
func fetchCoverFor(ctx context.Context, f *coverFetcher, res *buyResult) {
	dir := filepath.Dir(res.File)
	if res.File == "" || findCover(dir, false) != "" {
		return
	}
	artist, album := filepath.Base(filepath.Dir(dir)), filepath.Base(dir)
	m, err := f.find(ctx, artist, album)
	if err == nil {
		err = f.save(ctx, m, dir)
	}
	if err != nil {
		slog.Info("covers: none saved", "artist", artist, "album", album, "err", err)
		return
	}
	slog.Info("covers: saved", "artist", artist, "album", album, "release-group", m.ID)
}
