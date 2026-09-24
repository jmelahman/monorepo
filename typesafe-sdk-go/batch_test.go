package typesafe

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestSystemOneBatch(t *testing.T) {
	t.Parallel()

	var inFlight, peak atomic.Int32
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := inFlight.Add(1)
		defer inFlight.Add(-1)
		for {
			p := peak.Load()
			if n <= p || peak.CompareAndSwap(p, n) {
				break
			}
		}
		// Hold the request long enough for the others to pile up behind the limit.
		time.Sleep(20 * time.Millisecond)

		var req struct{ State string }
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &req)
		if req.State == "fail" {
			jsonHandler(http.StatusBadRequest, `{"error":{"message":"bad state"}}`)(w, r)
			return
		}
		jsonHandler(http.StatusOK, `{"model":"jev-1","answers":{"spam":{"type":"noul","noul":0.`+
			req.State+`}},"usage":{}}`)(w, r)
	}))
	t.Cleanup(s.Close)
	c, _ := testClient(t, s.URL)

	states := []string{"1", "2", "fail", "4", "5"}
	reqs := make([]SystemOneRequest, len(states))
	for i, state := range states {
		reqs[i] = simpleRequest()
		reqs[i].State = state
	}

	results := c.SystemOneBatch(t.Context(), reqs, 2)
	if len(results) != len(reqs) {
		t.Fatalf("got %d results, want %d", len(results), len(reqs))
	}
	for i, r := range results {
		if states[i] == "fail" {
			var badRequest *BadRequestError
			if r.Response != nil || !errors.As(r.Err, &badRequest) {
				t.Errorf("results[%d] = (%v, %v), want only a *BadRequestError", i, r.Response, r.Err)
			}
			continue
		}
		if r.Err != nil {
			t.Errorf("results[%d].Err = %v, want nil: one failure must not affect the others", i, r.Err)
			continue
		}
		// Each answer echoes its state, which proves results are in input order.
		spam, err := r.Response.Noul("spam")
		if want := "0." + states[i]; err != nil || jsonNumber(spam.Noul) != want {
			t.Errorf("results[%d] noul = (%v, %v), want %s", i, spam.Noul, err, want)
		}
	}
	if got := peak.Load(); got > 2 {
		t.Errorf("peak concurrency = %d, want at most 2", got)
	}
}

func jsonNumber(f float64) string {
	b, _ := json.Marshal(f)
	return string(b)
}

func TestSystemOneBatchCancelledSendsNothing(t *testing.T) {
	t.Parallel()

	s := newScriptedServer(t) // no handlers: nothing may be sent
	c, _ := testClient(t, s.URL)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	results := c.SystemOneBatch(ctx, []SystemOneRequest{simpleRequest(), simpleRequest()}, 0)
	for i, r := range results {
		if r.Response != nil || !errors.Is(r.Err, context.Canceled) {
			t.Errorf("results[%d] = (%v, %v), want context.Canceled", i, r.Response, r.Err)
		}
	}
	if got := s.count(); got != 0 {
		t.Errorf("attempts = %d, want 0", got)
	}
}

func TestSystemOneBatchEmpty(t *testing.T) {
	t.Parallel()

	c, _ := testClient(t, "http://127.0.0.1:0")
	if got := c.SystemOneBatch(t.Context(), nil, 4); len(got) != 0 {
		t.Errorf("SystemOneBatch(nil) = %v, want no results", got)
	}
}

// Requests in a batch retry concurrently, which exercises the client's sleep
// from several goroutines at once. Run with -race.
func TestSystemOneBatchRetriesConcurrently(t *testing.T) {
	t.Parallel()

	s := httptest.NewServer(jsonHandler(http.StatusServiceUnavailable, `{"error":{"message":"overloaded"}}`))
	t.Cleanup(s.Close)
	c, delays := testClient(t, s.URL)

	reqs := []SystemOneRequest{simpleRequest(), simpleRequest(), simpleRequest(), simpleRequest()}
	for i, r := range c.SystemOneBatch(t.Context(), reqs, len(reqs)) {
		var serverErr *InternalServerError
		if !errors.As(r.Err, &serverErr) {
			t.Errorf("results[%d].Err = %v, want *InternalServerError", i, r.Err)
		}
	}
	if want := len(reqs) * DefaultRetryPolicy().MaxRetries; len(*delays) != want {
		t.Errorf("recorded %d retry delays, want %d", len(*delays), want)
	}
}
