package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"math/rand/v2"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync"
	"time"
)

// mixLength is how many tracks a mix holds.
const mixLength = 50

// mixPerArtist keeps one artist from filling the library mix.
const mixPerArtist = 3

// mixKeep is how long a streamed mix is kept before YouTube is asked for
// another; asking afresh is slow, and the mix should hold still while it
// plays.
const mixKeep = 3 * time.Hour

// mixer builds the two mixes: the library mix, from what is owned, and
// the supermix, streamed from YouTube Music. Both leave out what the
// listener has excluded.
type mixer struct {
	music string
	shelf *shelf
	// cookies is a cookies.txt from a browser signed in to YouTube Music,
	// and supermix the account's Supermix playlist; with both, the
	// supermix is the account's own.
	cookies, supermix string

	mu     sync.Mutex
	cached []ytTrack
	source string
	at     time.Time
}

func routeMixes(mux *http.ServeMux, m *mixer) {
	mux.HandleFunc("GET /api/mix/library", func(w http.ResponseWriter, _ *http.Request) {
		artists, err := walkLibrary(m.music)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, m.library(flatten(artists), time.Now()))
	})
	mux.HandleFunc("GET /api/yt/supermix", func(w http.ResponseWriter, r *http.Request) {
		mix, source, err := m.streamed(r.Context(), r.URL.Query().Get("fresh") != "")
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		// account: the YouTube Music account's own; radio: seeded from
		// the library.
		w.Header().Set("X-Mix-Source", source)
		writeJSON(w, mix)
	})
}

// library picks the library mix: the same all day, so a phone keeping it
// fetches one new set a day. Played tracks weigh more, the last two days'
// less, and no artist has more than a few.
func (m *mixer) library(all []libTrack, now time.Time) []libTrack {
	rng := dailyRand("library-mix", now)
	type scored struct {
		t libTrack
		k float64
	}
	var picks []scored
	recent := now.Add(-48 * time.Hour)
	for _, t := range all {
		if m.shelf.isExcluded(t.Path) {
			continue
		}
		p := m.shelf.playsOf(t.Path)
		w := 1 + float64(min(p.N, 20))/4
		if p.Last.After(recent) {
			w /= 4
		}
		// A weighted draw without replacement: the largest keys win, and
		// a heavier track's key tends larger.
		picks = append(picks, scored{t, math.Pow(rng.Float64(), 1/w)})
	}
	slices.SortFunc(picks, func(a, b scored) int { return -cmpFloat(a.k, b.k) })
	per := map[string]int{}
	out := []libTrack{}
	for _, p := range picks {
		if len(out) == mixLength {
			break
		}
		if per[p.t.Artist] == mixPerArtist {
			continue
		}
		per[p.t.Artist]++
		out = append(out, p.t)
	}
	spread(out, rng)
	return out
}

func cmpFloat(a, b float64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// spread shuffles the mix, then moves apart an artist's tracks that land
// next to each other where a later track can take the place.
func spread(ts []libTrack, rng *rand.Rand) {
	rng.Shuffle(len(ts), func(i, j int) { ts[i], ts[j] = ts[j], ts[i] })
	for i := 1; i < len(ts); i++ {
		if ts[i].Artist != ts[i-1].Artist {
			continue
		}
		for j := i + 1; j < len(ts); j++ {
			if ts[j].Artist != ts[i-1].Artist {
				ts[i], ts[j] = ts[j], ts[i]
				break
			}
		}
	}
}

// streamed is the supermix: the account's own when it is set up and
// answers, otherwise a radio seeded from the library. It is kept for a
// while unless a fresh one is asked for.
func (m *mixer) streamed(ctx context.Context, fresh bool) ([]ytTrack, string, error) {
	m.mu.Lock()
	if !fresh && m.cached != nil && time.Since(m.at) < mixKeep {
		mix, source := m.cached, m.source
		m.mu.Unlock()
		return m.withoutExcluded(mix), source, nil
	}
	m.mu.Unlock()

	mix, source, err := m.account(ctx)
	if err != nil {
		slog.Warn("supermix: the account's mix failed; using the library radio", "err", err)
	}
	if mix == nil {
		source = "radio"
		if mix, err = ytSupermix(ctx, m.music, m.shelf); err != nil {
			return nil, "", err
		}
	}
	m.mu.Lock()
	m.cached, m.source, m.at = mix, source, time.Now()
	m.mu.Unlock()
	return m.withoutExcluded(mix), source, nil
}

func (m *mixer) withoutExcluded(mix []ytTrack) []ytTrack {
	return slices.DeleteFunc(slices.Clone(mix), func(t ytTrack) bool { return m.shelf.isExcluded("yt:" + t.VideoID) })
}

// account fetches the account's own Supermix, or nothing when it isn't
// set up. yt-dlp writes the cookies back when it is done, so it gets a
// copy and the file stays read-only.
func (m *mixer) account(ctx context.Context) ([]ytTrack, string, error) {
	if m.cookies == "" || m.supermix == "" {
		return nil, "", nil
	}
	src, err := os.Open(m.cookies)
	if err != nil {
		return nil, "", err
	}
	defer src.Close()
	tmp, err := os.CreateTemp("", "ytm-cookies-*.txt")
	if err != nil {
		return nil, "", err
	}
	defer os.Remove(tmp.Name())
	n, err := io.Copy(tmp, src)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return nil, "", err
	}
	if n == 0 {
		// The compose file mounts an empty file until one is set.
		return nil, "", nil
	}
	url := m.supermix
	if !strings.Contains(url, "://") {
		url = "https://music.youtube.com/playlist?list=" + url
	}
	mix, err := ytFlat(ctx, url, mixLength, "--cookies", tmp.Name())
	if err != nil {
		return nil, "", err
	}
	if len(mix) == 0 {
		return nil, "", errors.New("the account's supermix came back empty")
	}
	return mix, "account", nil
}

// ytSupermix builds a mix like the store apps' own: a seed from the
// owned library, expanded through YouTube's radio into a run of tracks
// the library doesn't have to own yet.
func ytSupermix(ctx context.Context, musicDir string, s *shelf) ([]ytTrack, error) {
	artists, err := walkLibrary(musicDir)
	if err != nil {
		return nil, err
	}
	all := slices.DeleteFunc(flatten(artists), func(t libTrack) bool { return s.isExcluded(t.Path) })
	if len(all) == 0 {
		return nil, errors.New("the owned library is empty; nothing to seed a mix from")
	}
	seed := all[rand.IntN(len(all))]

	// Find the seed on YouTube, then let the radio do the rest.
	// Track numbers only muddy the search.
	name := trackNumber.ReplaceAllString(seed.Name, "")
	found, err := ytFlat(ctx, "ytsearch1:"+seed.Artist+" "+name, 1)
	if err != nil || len(found) == 0 {
		return nil, fmt.Errorf("no YouTube match for %s — %s", seed.Artist, name)
	}
	seedID := found[0].VideoID
	mix, err := ytFlat(ctx, "https://www.youtube.com/watch?v="+seedID+"&list=RD"+seedID, mixLength+1)
	if err != nil {
		return nil, err
	}
	// The radio starts on its own seed, which the library already owns;
	// the mix is what follows it.
	mix = slices.DeleteFunc(mix, func(t ytTrack) bool { return t.VideoID == seedID })
	if len(mix) == 0 {
		return nil, fmt.Errorf("YouTube has no radio for %s — %s", seed.Artist, name)
	}
	return mix, nil
}
