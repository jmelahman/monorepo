package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// The YouTube Music account, as the web app's panel sets it up: a
// cookies.txt uploaded from a signed-in browser and the Supermix's share
// link, both kept under the state directory. Either one, once saved,
// wins over its MUSIC_YTM_* variable. The cookies never leave the
// server again; the panel only learns whether they are there.

const (
	ytCookiesFile = "ytm-cookies.txt"
	ytAccountFile = "ytm.json"
)

// playlistID is what a YouTube playlist id looks like.
var playlistID = regexp.MustCompile(`^[A-Za-z0-9_-]{10,64}$`)

// cookiesFile is the cookies.txt the account's mix signs in with: the
// uploaded one, or the environment's, or "" when neither has anything in
// it. The compose file mounts an empty file until .env names one.
func (m *mixer) cookiesFile() string {
	for _, f := range []string{m.savedCookies(), m.cookies} {
		if f == "" {
			continue
		}
		if fi, err := os.Stat(f); err == nil && fi.Size() > 0 {
			return f
		}
	}
	return ""
}

func (m *mixer) savedCookies() string {
	if m.state == "" {
		return ""
	}
	return filepath.Join(m.state, ytCookiesFile)
}

// supermixID is the account's Supermix playlist: the saved one, or the
// environment's.
func (m *mixer) supermixID() string {
	if id := m.savedSupermix(); id != "" {
		return id
	}
	return m.supermix
}

func (m *mixer) savedSupermix() string {
	if m.state == "" {
		return ""
	}
	var saved struct {
		Supermix string `json:"supermix"`
	}
	_ = loadJSON(filepath.Join(m.state, ytAccountFile), &saved)
	return saved.Supermix
}

// forget drops the mix kept from the old account, so the next one asks
// YouTube with the new.
func (m *mixer) forget() {
	m.mu.Lock()
	m.cached, m.failed = nil, ""
	m.mu.Unlock()
}

// parsePlaylist takes a share link (music.youtube.com/playlist?list=…, or
// a watch link with list=) or a bare id, and gives the id.
func parsePlaylist(s string) (string, error) {
	s = strings.TrimSpace(s)
	if strings.Contains(s, "://") || strings.Contains(s, "list=") {
		u, err := url.Parse(s)
		if err != nil {
			return "", errors.New("that link could not be read")
		}
		s = u.Query().Get("list")
	}
	if !playlistID.MatchString(s) {
		return "", errors.New("that isn't a playlist link or id; copy the Supermix's share link")
	}
	return s, nil
}

// checkCookies reads a cookies.txt (the Netscape format browsers' export
// extensions write) and says whether it holds YouTube's cookies.
func checkCookies(b []byte) error {
	youtube := 0
	sc := bufio.NewScanner(strings.NewReader(string(b)))
	sc.Buffer(nil, 1<<20)
	for sc.Scan() {
		line := strings.TrimPrefix(sc.Text(), "#HttpOnly_")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.Split(line, "\t")
		if len(f) != 7 {
			return errors.New("that isn't a cookies.txt; export one in the Netscape format")
		}
		if strings.HasSuffix(f[0], "youtube.com") {
			youtube++
		}
	}
	if youtube == 0 {
		return errors.New("that cookies.txt has no YouTube cookies; export it from music.youtube.com while signed in")
	}
	return nil
}

// writePrivate writes a file only the server can read, through a
// temporary file so a crash leaves the old copy or the new one.
func writePrivate(file string, b []byte) error {
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(file), ".ytm-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), file)
}

// ytAccount is what the panel shows: where each half comes from, never
// the cookies themselves.
type ytAccount struct {
	// Cookies is "saved" (uploaded here), "env" (MUSIC_YTM_COOKIES) or "".
	Cookies   string    `json:"cookies"`
	CookiesAt time.Time `json:"cookiesAt,omitzero"`
	// Supermix is the playlist id, and SupermixFrom "saved", "env" or "".
	Supermix     string `json:"supermix"`
	SupermixFrom string `json:"supermixFrom"`
	Ready        bool   `json:"ready"`
	// Failed is why the account's mix last failed, if it did.
	Failed string `json:"failed,omitempty"`
}

func (m *mixer) describe() ytAccount {
	var a ytAccount
	switch f := m.cookiesFile(); {
	case f == "":
	case f == m.savedCookies():
		a.Cookies = "saved"
		if fi, err := os.Stat(f); err == nil {
			a.CookiesAt = fi.ModTime().UTC()
		}
	default:
		a.Cookies = "env"
	}
	if id := m.savedSupermix(); id != "" {
		a.Supermix, a.SupermixFrom = id, "saved"
	} else if m.supermix != "" {
		a.Supermix, a.SupermixFrom = m.supermix, "env"
	}
	a.Ready = a.Cookies != "" && a.Supermix != ""
	m.mu.Lock()
	a.Failed = m.failed
	m.mu.Unlock()
	return a
}

func routeYTAccount(mux *http.ServeMux, m *mixer) {
	mux.HandleFunc("GET /api/yt/account", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, m.describe())
	})
	// The cookies.txt itself, as the request's body.
	mux.HandleFunc("PUT /api/yt/account/cookies", func(w http.ResponseWriter, r *http.Request) {
		if m.state == "" {
			http.Error(w, "the server has no state directory", http.StatusServiceUnavailable)
			return
		}
		b, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
		if err != nil {
			http.Error(w, "the cookies.txt is too large", http.StatusRequestEntityTooLarge)
			return
		}
		if err := checkCookies(b); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := writePrivate(m.savedCookies(), b); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		m.forget()
		writeJSON(w, m.describe())
	})
	mux.HandleFunc("DELETE /api/yt/account/cookies", func(w http.ResponseWriter, _ *http.Request) {
		if f := m.savedCookies(); f != "" {
			if err := os.Remove(f); err != nil && !errors.Is(err, os.ErrNotExist) {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		m.forget()
		writeJSON(w, m.describe())
	})
	// {"supermix": a share link or an id}; an empty one goes back to the
	// environment's.
	mux.HandleFunc("PUT /api/yt/account/supermix", func(w http.ResponseWriter, r *http.Request) {
		if m.state == "" {
			http.Error(w, "the server has no state directory", http.StatusServiceUnavailable)
			return
		}
		var body struct {
			Supermix string `json:"supermix"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&body); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		id := ""
		if strings.TrimSpace(body.Supermix) != "" {
			var err error
			if id, err = parsePlaylist(body.Supermix); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		}
		if err := saveJSON(filepath.Join(m.state, ytAccountFile), struct {
			Supermix string `json:"supermix"`
		}{id}); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		m.forget()
		writeJSON(w, m.describe())
	})
	// check asks YouTube for the account's Supermix now, so the panel can
	// say whether the cookies still sign in.
	mux.HandleFunc("POST /api/yt/account/check", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
		defer cancel()
		var out struct {
			OK     bool   `json:"ok"`
			Tracks int    `json:"tracks"`
			Error  string `json:"error,omitempty"`
		}
		if !m.accountSet() {
			out.Error = "the cookies and the Supermix link are both needed"
			writeJSON(w, out)
			return
		}
		mix, _, err := m.account(ctx, mixLength)
		m.mu.Lock()
		m.failed = ""
		if err != nil {
			m.failed = err.Error()
		}
		m.mu.Unlock()
		if err != nil {
			out.Error = err.Error()
		} else {
			out.OK, out.Tracks = true, len(mix)
		}
		writeJSON(w, out)
	})
}
