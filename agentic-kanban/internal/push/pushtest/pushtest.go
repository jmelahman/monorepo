// Package pushtest fakes the browser and push-service halves of Web Push
// for tests.
package pushtest

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/jmelahman/kanban/internal/db"
	"github.com/jmelahman/kanban/internal/push"
)

// Service is a fake push service. Deliveries to "/gone/…" answer 410, as a
// real service does for a subscription the browser dropped. Deliveries to
// "/slow/…" hang until Release is called (or the test ends).
type Service struct {
	*httptest.Server
	mu      sync.Mutex
	hits    []string
	release chan struct{}
	once    sync.Once
	// Delivered receives the path of every accepted delivery.
	Delivered chan string
}

func NewService(t *testing.T) *Service {
	t.Helper()
	s := &Service{Delivered: make(chan string, 16), release: make(chan struct{})}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.hits = append(s.hits, r.URL.Path)
		s.mu.Unlock()
		if strings.HasPrefix(r.URL.Path, "/gone/") {
			w.WriteHeader(http.StatusGone)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/slow/") {
			<-s.release
		}
		w.WriteHeader(http.StatusCreated)
		s.Delivered <- r.URL.Path
	}))
	t.Cleanup(func() {
		s.Release()
		s.Close()
	})
	return s
}

// Release lets every "/slow/…" delivery finish.
func (s *Service) Release() { s.once.Do(func() { close(s.release) }) }

// Sender returns a push service that can reach this fake. The default client
// refuses non-public addresses, which a test server always has.
func (s *Service) Sender(store *db.Store) *push.Service {
	svc := push.New(store)
	svc.SetHTTPClient(s.Client())
	return svc
}

// Hits returns the path of every request received so far.
func (s *Service) Hits() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.hits...)
}

// Subscription builds a subscription at path with real client keys, so the
// payload encryption in the sender succeeds.
func (s *Service) Subscription(t *testing.T, path string, events ...string) db.PushSubscription {
	t.Helper()
	key, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	auth := make([]byte, 16)
	if _, err := rand.Read(auth); err != nil {
		t.Fatal(err)
	}
	return db.PushSubscription{
		Endpoint: s.URL + path,
		P256dh:   base64.RawURLEncoding.EncodeToString(key.PublicKey().Bytes()),
		Auth:     base64.RawURLEncoding.EncodeToString(auth),
		Events:   events,
	}
}
