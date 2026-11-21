package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

// A buy moves through the queue as queued, buying, then done or failed.
const (
	buyQueued = "queued"
	buyBuying = "buying"
	buyDone   = "done"
	buyFailed = "failed"
)

// buyBackoff is how long a failed attempt waits before the next one; a
// buy that has used them all is failed for good. Retrying is safe: the
// store provider fetches a track the account already owns again for free.
var buyBackoff = []time.Duration{time.Minute, 5 * time.Minute, 30 * time.Minute}

// buyAttempt bounds one run of the store provider: search, match, the
// download itself.
const buyAttempt = 5 * time.Minute

// buyKeep is how many finished buys the queue remembers for the clients.
const buyKeep = 50

// buyResult is what the store provider prints when a buy lands.
type buyResult struct {
	Artist       string `json:"artist"`
	Album        string `json:"album"`
	Title        string `json:"title"`
	File         string `json:"file"`
	AlreadyOwned bool   `json:"alreadyOwned"`
}

// buy is one purchase, as it is saved and as the clients see it.
type buy struct {
	ID       string     `json:"id"`
	Artist   string     `json:"artist"`
	Title    string     `json:"title"`
	Status   string     `json:"status"`
	Attempts int        `json:"attempts"`
	Error    string     `json:"error,omitempty"`
	Result   *buyResult `json:"result,omitempty"`
	Created  time.Time  `json:"created"`
	Next     time.Time  `json:"next"`           // when a queued buy may run
	Link     string     `json:"link,omitempty"` // the store link the last attempt bought from
}

// storeLink is an album page in the store found by hand for a track the
// provider's own search can't find. The provider buys from it instead.
type storeLink struct {
	Artist string    `json:"artist"`
	Title  string    `json:"title"`
	Link   string    `json:"link"`
	Saved  time.Time `json:"saved"`
}

// buyQueue runs purchases through the store provider one at a time, so
// two never race the account's balance, outside any request, so a client
// that hangs up doesn't cut a download short. It is saved under --state
// after every change and picks up where it left off.
type buyQueue struct {
	store string // the store provider's executable
	music string // the owned library it buys into
	proxy string // the HTTP proxy the provider's traffic leaves through, if any
	file  string // where the queue is saved
	links string // where the store links are saved

	mu    sync.Mutex
	buys  []*buy      // oldest first
	saved []storeLink // newest first
	wake  chan struct{}
}

// routeBuys serves the queue for the store provider at store, or answers
// 503 when none is configured. Buys go through the queue rather than the
// request: a purchase outlasts nginx's patience, and a hung-up client
// must not cut it short.
func routeBuys(ctx context.Context, mux *http.ServeMux, store, proxy, music, state string) {
	var q *buyQueue
	if store != "" {
		var err error
		if q, err = newBuyQueue(store, music, state); err != nil {
			fail(err)
		}
		q.proxy = proxy
		go q.run(ctx)
	}
	mux.HandleFunc("POST /api/buy", func(w http.ResponseWriter, r *http.Request) {
		if q == nil {
			http.Error(w, "no store provider configured", http.StatusServiceUnavailable)
			return
		}
		var req struct {
			Artist string `json:"artist"`
			Title  string `json:"title"`
		}
		r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
		err := json.NewDecoder(r.Body).Decode(&req)
		req.Artist, req.Title = strings.TrimSpace(req.Artist), strings.TrimSpace(req.Title)
		if err != nil || req.Artist == "" || req.Title == "" {
			http.Error(w, "artist and title required", http.StatusBadRequest)
			return
		}
		b, err := q.add(req.Artist, req.Title)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(b)
	})
	mux.HandleFunc("GET /api/buys", func(w http.ResponseWriter, _ *http.Request) {
		if q == nil {
			http.Error(w, "no store provider configured", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(q.list())
	})
	// Dismissing forgets a buy that isn't running.
	mux.HandleFunc("DELETE /api/buys/{id}", func(w http.ResponseWriter, r *http.Request) {
		if q == nil {
			http.Error(w, "no store provider configured", http.StatusServiceUnavailable)
			return
		}
		if err := q.dismiss(r.PathValue("id")); err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /api/buys/links", func(w http.ResponseWriter, _ *http.Request) {
		if q == nil {
			http.Error(w, "no store provider configured", http.StatusServiceUnavailable)
			return
		}
		q.mu.Lock()
		links := slices.Clone(q.saved)
		q.mu.Unlock()
		writeJSON(w, links)
	})
	// Saving a link buys the track from it straight away; an empty link
	// forgets the track's.
	mux.HandleFunc("PUT /api/buys/links", func(w http.ResponseWriter, r *http.Request) {
		if q == nil {
			http.Error(w, "no store provider configured", http.StatusServiceUnavailable)
			return
		}
		var req storeLink
		r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
		err := json.NewDecoder(r.Body).Decode(&req)
		req.Artist, req.Title, req.Link = strings.TrimSpace(req.Artist), strings.TrimSpace(req.Title), strings.TrimSpace(req.Link)
		if err != nil || req.Artist == "" || req.Title == "" {
			http.Error(w, "artist and title required", http.StatusBadRequest)
			return
		}
		if u, err := url.Parse(req.Link); req.Link != "" && (err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "") {
			http.Error(w, "the link must be a store page's http(s) address", http.StatusBadRequest)
			return
		}
		if err := q.setLink(req.Artist, req.Title, req.Link); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if req.Link == "" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		b, err := q.add(req.Artist, req.Title)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(b)
	})
}

func newBuyQueue(store, music, state string) (*buyQueue, error) {
	q := &buyQueue{
		store: store,
		music: music,
		file:  filepath.Join(state, "buys.json"),
		links: filepath.Join(state, "store-links.json"),
		wake:  make(chan struct{}, 1),
	}
	if err := loadJSON(q.file, &q.buys); err != nil {
		return nil, err
	}
	if err := loadJSON(q.links, &q.saved); err != nil {
		return nil, err
	}
	// A buy that was running when the server stopped runs again.
	for _, b := range q.buys {
		if b.Status == buyBuying {
			b.Status = buyQueued
		}
	}
	return q, nil
}

// add queues a buy, or hands back the one already waiting or running for
// the same track. A failed one is queued afresh rather than listed twice,
// and one waiting out a failure runs now.
func (q *buyQueue) add(artist, title string) (buy, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	same := func(b *buy) bool { return strings.EqualFold(b.Artist, artist) && strings.EqualFold(b.Title, title) }
	for _, b := range slices.Backward(q.buys) {
		if !same(b) {
			continue
		}
		if b.Status == buyDone {
			break // bought before: this is another buy
		}
		if b.Status == buyFailed {
			b.Status, b.Attempts = buyQueued, 0
		}
		if b.Status == buyQueued {
			b.Next = time.Now()
		}
		// Older failures of the same track are this one's history.
		q.buys = slices.DeleteFunc(q.buys, func(o *buy) bool { return o != b && same(o) && o.Status == buyFailed })
		if err := q.saveLocked(); err != nil {
			return buy{}, err
		}
		q.poke()
		return *b, nil
	}
	id := make([]byte, 8)
	_, _ = rand.Read(id)
	now := time.Now()
	b := &buy{ID: hex.EncodeToString(id), Artist: artist, Title: title, Status: buyQueued, Created: now, Next: now}
	q.buys = append(q.buys, b)
	if err := q.saveLocked(); err != nil {
		q.buys = q.buys[:len(q.buys)-1]
		return buy{}, err
	}
	q.poke()
	return *b, nil
}

// dismiss forgets a buy, unless it is running.
func (q *buyQueue) dismiss(id string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	i := slices.IndexFunc(q.buys, func(b *buy) bool { return b.ID == id })
	if i < 0 {
		return nil
	}
	if q.buys[i].Status == buyBuying {
		return errors.New("the buy is running")
	}
	q.buys = slices.Delete(q.buys, i, i+1)
	return q.saveLocked()
}

// setLink saves the store link for a track, or forgets it when link is
// empty.
func (q *buyQueue) setLink(artist, title, link string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	saved := slices.DeleteFunc(slices.Clone(q.saved), func(l storeLink) bool {
		return strings.EqualFold(l.Artist, artist) && strings.EqualFold(l.Title, title)
	})
	if link != "" {
		saved = slices.Insert(saved, 0, storeLink{Artist: artist, Title: title, Link: link, Saved: time.Now()})
	}
	if err := saveJSON(q.links, saved); err != nil {
		return err
	}
	q.saved = saved
	return nil
}

// linkForLocked is the store link saved for a track, if any.
func (q *buyQueue) linkForLocked(artist, title string) string {
	for _, l := range q.saved {
		if strings.EqualFold(l.Artist, artist) && strings.EqualFold(l.Title, title) {
			return l.Link
		}
	}
	return ""
}

// list returns the buys, newest first.
func (q *buyQueue) list() []buy {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := make([]buy, 0, len(q.buys))
	for _, b := range slices.Backward(q.buys) {
		out = append(out, *b)
	}
	return out
}

func (q *buyQueue) poke() {
	select {
	case q.wake <- struct{}{}:
	default:
	}
}

// run works the queue until ctx ends, sleeping until the next buy is due
// or a new one arrives.
func (q *buyQueue) run(ctx context.Context) {
	for {
		b, wait := q.next()
		if b == nil {
			t := time.NewTimer(wait)
			select {
			case <-ctx.Done():
				t.Stop()
				return
			case <-q.wake:
				t.Stop()
			case <-t.C:
			}
			continue
		}
		res, err := q.attempt(ctx, b.Artist, b.Title, b.Link)
		if ctx.Err() != nil {
			// Stopped mid-buy; it runs again on the next start.
			return
		}
		q.finish(b.ID, res, err)
	}
}

// next marks the first due buy as buying and returns a copy of it, or
// says how long until one is due.
func (q *buyQueue) next() (*buy, time.Duration) {
	q.mu.Lock()
	defer q.mu.Unlock()
	wait := time.Hour
	now := time.Now()
	for _, b := range q.buys {
		if b.Status != buyQueued {
			continue
		}
		if d := b.Next.Sub(now); d > 0 {
			wait = min(wait, d)
			continue
		}
		b.Status = buyBuying
		b.Attempts++
		b.Link = q.linkForLocked(b.Artist, b.Title)
		if err := q.saveLocked(); err != nil {
			slog.Error("buys: save", "err", err)
		}
		c := *b
		return &c, 0
	}
	return nil, wait
}

// attempt runs the store provider once for a track, from the store link
// found for it by hand when there is one.
func (q *buyQueue) attempt(ctx context.Context, artist, title, link string) (*buyResult, error) {
	cctx, cancel := context.WithTimeout(ctx, buyAttempt)
	defer cancel()
	// The values ride in the flags themselves, so a title that starts
	// with a dash can't be read as a flag of its own.
	args := []string{"buy", "--artist=" + artist, "--title=" + title, "--dir=" + q.music}
	if link != "" {
		args = append(args, "--link="+link)
	}
	cmd := exec.CommandContext(cctx, q.store, args...)
	// A provider that leaves a browser or a download behind it must not
	// hold the queue past the attempt's own limit.
	cmd.WaitDelay = 10 * time.Second
	if q.proxy != "" {
		// Only the provider leaves through the proxy (the homelab's VPN),
		// and with it set the provider has no other way out: a VPN that
		// is down fails the attempt rather than buying in the clear.
		cmd.Env = append(os.Environ(),
			"HTTPS_PROXY="+q.proxy, "HTTP_PROXY="+q.proxy, "https_proxy="+q.proxy, "http_proxy="+q.proxy,
			"NO_PROXY=", "no_proxy=")
	}
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(string(out) + " " + errBuf.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, errors.New(msg)
	}
	var res buyResult
	if err := json.Unmarshal(out, &res); err != nil {
		return nil, errors.New("the store provider's answer could not be read: " + err.Error())
	}
	return &res, nil
}

// finish records an attempt's outcome: done, queued again after a wait,
// or failed once the waits run out.
func (q *buyQueue) finish(id string, res *buyResult, err error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	i := slices.IndexFunc(q.buys, func(b *buy) bool { return b.ID == id })
	if i < 0 {
		return
	}
	b := q.buys[i]
	switch {
	case err == nil:
		b.Status, b.Result, b.Error = buyDone, res, ""
		slog.Info("buys: bought", "artist", res.Artist, "album", res.Album, "title", res.Title, "owned", res.AlreadyOwned)
	case b.Attempts <= len(buyBackoff):
		b.Status, b.Error = buyQueued, err.Error()
		b.Next = time.Now().Add(buyBackoff[b.Attempts-1])
		slog.Warn("buys: retrying", "artist", b.Artist, "title", b.Title, "attempt", b.Attempts, "err", err)
	default:
		b.Status, b.Error = buyFailed, err.Error()
		slog.Error("buys: failed", "artist", b.Artist, "title", b.Title, "err", err)
	}
	q.pruneLocked()
	if err := q.saveLocked(); err != nil {
		slog.Error("buys: save", "err", err)
	}
}

// pruneLocked forgets the oldest finished buys past buyKeep.
func (q *buyQueue) pruneLocked() {
	finished := 0
	for _, b := range q.buys {
		if b.Status == buyDone || b.Status == buyFailed {
			finished++
		}
	}
	q.buys = slices.DeleteFunc(q.buys, func(b *buy) bool {
		if finished > buyKeep && (b.Status == buyDone || b.Status == buyFailed) {
			finished--
			return true
		}
		return false
	})
}

// saveLocked writes the queue.
func (q *buyQueue) saveLocked() error {
	return saveJSON(q.file, q.buys)
}
