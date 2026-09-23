package workerapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// fakeRegistrar records register/deregister calls.
type fakeRegistrar struct {
	mu        sync.Mutex
	added     map[string]string // endpoint -> host
	instances map[string]string // endpoint -> instance-id
	removed   []string
	regErr    error
}

func newFakeRegistrar() *fakeRegistrar {
	return &fakeRegistrar{added: map[string]string{}, instances: map[string]string{}}
}

func (f *fakeRegistrar) Register(endpoint, host, instanceID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.regErr != nil {
		return f.regErr
	}
	f.added[endpoint] = host
	f.instances[endpoint] = instanceID
	return nil
}

func (f *fakeRegistrar) Deregister(endpoint string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.removed = append(f.removed, endpoint)
	delete(f.added, endpoint)
	return nil
}

func (f *fakeRegistrar) host(endpoint string) (string, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	h, ok := f.added[endpoint]
	return h, ok
}

func testControlServer(t *testing.T, reg Registrar) (*ControlClient, func()) {
	t.Helper()
	srv := httptest.NewServer(NewControlServer(reg, secret).Handler())
	c := NewControlClient(srv.URL, secret, srv.Client())
	return c, srv.Close
}

func TestRegisterDeregisterRoundTrip(t *testing.T) {
	reg := newFakeRegistrar()
	c, done := testControlServer(t, reg)
	defer done()

	const ep, host, iid = "http://10.0.1.5:9100", "10.0.1.5", "i-0abc123"
	if err := c.Register(context.Background(), ep, host, iid); err != nil {
		t.Fatal(err)
	}
	if got, ok := reg.host(ep); !ok || got != host {
		t.Fatalf("after register: host=%q ok=%v, want %q true", got, ok, host)
	}
	if got := reg.instances[ep]; got != iid {
		t.Fatalf("after register: instance-id=%q, want %q", got, iid)
	}

	if err := c.Deregister(context.Background(), ep); err != nil {
		t.Fatal(err)
	}
	if _, ok := reg.host(ep); ok {
		t.Fatalf("endpoint still registered after deregister")
	}
}

// A register with no explicit host falls back to the endpoint URL's host, so a
// worker can announce just its base URL.
func TestRegisterDerivesHostFromEndpoint(t *testing.T) {
	reg := newFakeRegistrar()
	c, done := testControlServer(t, reg)
	defer done()

	const ep = "http://10.0.1.9:9100"
	if err := c.Register(context.Background(), ep, "", ""); err != nil {
		t.Fatal(err)
	}
	if got, _ := reg.host(ep); got != "10.0.1.9" {
		t.Fatalf("derived host = %q, want 10.0.1.9", got)
	}
}

func TestRegisterRejectsEmptyEndpoint(t *testing.T) {
	reg := newFakeRegistrar()
	c, done := testControlServer(t, reg)
	defer done()

	if err := c.Register(context.Background(), "", "", ""); err == nil {
		t.Fatal("want error for empty endpoint")
	}
	if len(reg.added) != 0 {
		t.Fatalf("registrar recorded %d entries, want 0", len(reg.added))
	}
}

// The registration surface is RCE-adjacent: a wrong or missing secret must be
// rejected, exactly like the worker API.
func TestControlServerRejectsBadSecret(t *testing.T) {
	reg := newFakeRegistrar()
	srv := httptest.NewServer(NewControlServer(reg, secret).Handler())
	defer srv.Close()

	// Wrong secret via a client dialing the same server.
	bad := NewControlClient(srv.URL, "wrong", srv.Client())
	if err := bad.Register(context.Background(), "http://10.0.1.5:9100", "10.0.1.5", ""); err == nil {
		t.Fatal("want unauthorized with wrong secret")
	}
	if len(reg.added) != 0 {
		t.Fatalf("registrar mutated despite bad secret: %v", reg.added)
	}

	// No Authorization header at all.
	req, _ := http.NewRequest(http.MethodPost, srv.URL+pathRegister, http.NoBody)
	res, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", res.StatusCode)
	}
}

// An empty server secret must fail closed (never authenticate).
func TestControlServerEmptySecretFailsClosed(t *testing.T) {
	reg := newFakeRegistrar()
	srv := httptest.NewServer(NewControlServer(reg, "").Handler())
	defer srv.Close()

	c := NewControlClient(srv.URL, "", srv.Client())
	if err := c.Register(context.Background(), "http://10.0.1.5:9100", "10.0.1.5", ""); err == nil {
		t.Fatal("want rejection when server secret is empty")
	}
}
