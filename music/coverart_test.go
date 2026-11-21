package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSameName(t *testing.T) {
	for _, c := range []struct {
		a, b string
		same bool
	}{
		{"These Days...", "These Days", true},
		{"Do What Thou Wilt.", "Do What Thou Wilt", true},
		{"RELAXER", "Relaxer", true},
		{"Alt-J", "alt‐J", true},
		{"Control System (Deluxe Edition)", "Control System", true},
		{"Simon & Garfunkel", "Simon and Garfunkel", true},
		{"Relaxer", "Relaxer Remixes", false},
		{"[17]", "17", false},
	} {
		if got := sameName(c.a, c.b); got != c.same {
			t.Errorf("sameName(%q, %q) = %v, want %v", c.a, c.b, got, c.same)
		}
	}
}

func TestFolderNames(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"K_I_D_S", "K I D S"},
		{"Florence _ The Machine", "Florence The Machine"},
		{"Hozier (Deluxe Edition) CD2", "Hozier (Deluxe Edition)"},
		{"Fire & Ice cd 1", "Fire & Ice"},
		{"CD1", "CD1"},
		{"Section_80", "Section 80"},
	} {
		if got := discless(folderName(c.in)); got != c.want {
			t.Errorf("discless(folderName(%q)) = %q, want %q", c.in, got, c.want)
		}
	}
}

// fakeCoverServices stands in for MusicBrainz and the Cover Art Archive:
// a search answering wrong albums before the right one, and art for two
// of them.
func fakeCoverServices(t *testing.T) {
	busy := true
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != coverAgent {
			t.Errorf("%s asked without the User-Agent", r.URL.Path)
		}
		switch r.URL.Path {
		case "/mb/release-group/":
			// The first search finds MusicBrainz busy.
			if busy {
				busy = false
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			if !strings.Contains(r.URL.Query().Get("query"), `releasegroup:"Control System"`) {
				_, _ = w.Write([]byte(`{"release-groups":[]}`))
				return
			}
			_, _ = w.Write([]byte(`{"release-groups":[
				{"id":"other-artist","title":"Control System","artist-credit":[{"name":"Someone Else","joinphrase":""}]},
				{"id":"remixes","title":"Control System Remixes","artist-credit":[{"name":"Ab-Soul","joinphrase":""}]},
				{"id":"no-art","title":"Control System","artist-credit":[{"name":"Ab-Soul","joinphrase":""}]},
				{"id":"right","title":"Control System","artist-credit":[{"name":"Ab-Soul","joinphrase":""}]}
			]}`))
		case "/caa/release-group/other-artist", "/caa/release-group/remixes":
			_, _ = w.Write([]byte(`{"images":[{"front":true}]}`))
		case "/caa/release-group/right":
			_, _ = w.Write([]byte(`{"images":[{"front":false},{"front":true}]}`))
		case "/caa/release-group/right/front-500":
			_, _ = w.Write([]byte("jpeg"))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	mb, caa := musicBrainz, coverArtArchive
	musicBrainz, coverArtArchive = srv.URL+"/mb", srv.URL+"/caa"
	t.Cleanup(func() { musicBrainz, coverArtArchive = mb, caa })
}

func TestFindCoverArt(t *testing.T) {
	fakeCoverServices(t)
	f := newCoverFetcher()
	f.gap, f.busy = 0, time.Millisecond
	ctx := context.Background()

	// The edition note is tried without, once the full name finds nothing.
	m, err := f.find(ctx, "Ab-Soul", "Control System (Deluxe)")
	if err != nil {
		t.Fatal(err)
	}
	if m.ID != "right" {
		t.Errorf("matched %q, want the group by the same artist with a front cover", m.ID)
	}
	// A disc's folder is its album's.
	if m, err := f.find(ctx, "Ab-Soul", "Control_System CD2"); err != nil || m.ID != "right" {
		t.Errorf("a disc's folder matched %q, %v", m.ID, err)
	}
	if _, err := f.find(ctx, "Ab-Soul", "These Days..."); !errors.Is(err, errNoMatch) {
		t.Errorf("an album the search lacks: %v, want errNoMatch", err)
	}

	dir := t.TempDir()
	if err := f.save(ctx, m, dir); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(filepath.Join(dir, "cover.jpg")); err != nil || string(b) != "jpeg" {
		t.Errorf("cover.jpg = %q, %v", b, err)
	}
	if got := findCover(dir, false); got != filepath.Join(dir, "cover.jpg") {
		t.Errorf("findCover = %q after saving", got)
	}
}
