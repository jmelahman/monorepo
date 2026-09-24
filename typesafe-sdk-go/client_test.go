package typesafe

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// scriptedServer answers each attempt with the next handler in the script and
// records the requests it received, so a test can assert on both.
type scriptedServer struct {
	*httptest.Server

	mu       sync.Mutex
	script   []http.HandlerFunc
	attempts int
	requests []*http.Request
	bodies   []string
}

func newScriptedServer(t *testing.T, script ...http.HandlerFunc) *scriptedServer {
	t.Helper()
	s := &scriptedServer{script: script}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)

		s.mu.Lock()
		n := s.attempts
		s.attempts++
		s.requests = append(s.requests, r.Clone(r.Context()))
		s.bodies = append(s.bodies, string(body))
		s.mu.Unlock()

		if n >= len(s.script) {
			t.Errorf("unexpected attempt %d; the script has %d handlers", n+1, len(s.script))
			w.WriteHeader(http.StatusTeapot)
			return
		}
		s.script[n](w, r)
	}))
	t.Cleanup(s.Close)
	return s
}

func (s *scriptedServer) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.attempts
}

func (s *scriptedServer) request(i int) *http.Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.requests[i]
}

// jsonHandler replies with a status and body.
func jsonHandler(status int, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set(requestIDHeader, "req_abc123")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}
}

const okBody = `{
	"model": "jev-1",
	"answers": {"spam": {"type": "noul", "noul": 0.9}},
	"usage": {"input_tokens": 12, "output_tokens": 3}
}`

func simpleRequest() SystemOneRequest {
	return SystemOneRequest{
		State:     "Buy now!",
		Questions: Questions{"spam": Noul("Is this spam?")},
	}
}

// testClient builds a client pointed at a test server, with the sleep seam
// recording delays instead of waiting.
func testClient(t *testing.T, baseURL string, opts ...Option) (*Client, *[]time.Duration) {
	t.Helper()
	var (
		mu     sync.Mutex
		delays []time.Duration
	)
	base := []Option{
		WithAPIKey("sk-test"),
		WithBaseURL(baseURL),
		withRand(noJitter),
		withSleep(func(ctx context.Context, d time.Duration) error {
			// Locked because SystemOneBatch calls SystemOne concurrently.
			mu.Lock()
			delays = append(delays, d)
			mu.Unlock()
			return ctx.Err()
		}),
	}
	c, err := New(append(base, opts...)...)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c, &delays
}

func TestSystemOne(t *testing.T) {
	t.Parallel()

	s := newScriptedServer(t, jsonHandler(http.StatusOK, okBody))
	c, _ := testClient(t, s.URL)

	res, err := c.SystemOne(t.Context(), simpleRequest())
	if err != nil {
		t.Fatalf("SystemOne: %v", err)
	}

	spam, err := res.Noul("spam")
	if err != nil {
		t.Fatalf("Noul: %v", err)
	}
	if spam.Noul != 0.9 {
		t.Errorf("Noul = %v, want 0.9", spam.Noul)
	}
	if res.Usage != (Usage{InputTokens: 12, OutputTokens: 3}) {
		t.Errorf("Usage = %+v, want {12 3}", res.Usage)
	}
	if res.Meta.RequestID != "req_abc123" || res.Meta.Attempts != 1 {
		t.Errorf("Meta = %+v, want request ID req_abc123 and 1 attempt", res.Meta)
	}
	if res.Meta.HTTP == nil {
		t.Fatal("Meta.HTTP = nil, want the response")
	}
	// The response body is rewound, so the caller can read it like any other.
	raw, err := io.ReadAll(res.Meta.HTTP.Body)
	if err != nil || len(raw) == 0 {
		t.Errorf("reading Meta.HTTP.Body = (%d bytes, %v), want the buffered body", len(raw), err)
	}

	// The request carries the client's default model and the questions.
	var sent map[string]any
	if err := json.Unmarshal([]byte(s.bodies[0]), &sent); err != nil {
		t.Fatalf("decoding the sent body: %v", err)
	}
	if sent["model"] != DefaultModel {
		t.Errorf("sent model = %v, want %q", sent["model"], DefaultModel)
	}
	if sent["state"] != "Buy now!" {
		t.Errorf("sent state = %v, want the request state", sent["state"])
	}
	if req := s.request(0); req.URL.Path != systemOnePath || req.Method != "POST" {
		t.Errorf("sent %s %s, want POST %s", req.Method, req.URL.Path, systemOnePath)
	}
}

func TestForcedHeaders(t *testing.T) {
	t.Parallel()

	s := newScriptedServer(t, jsonHandler(http.StatusOK, okBody))
	c, _ := testClient(t, s.URL, WithDefaultHeaders(http.Header{
		"authorization":  {"Bearer hijacked"},
		"X-Trace":        {"client"},
		"content-type":   {"text/plain"},
		retryCountHeader: {"7"},
	}))

	_, err := c.SystemOne(t.Context(), simpleRequest(),
		WithRequestHeaders(http.Header{"X-Trace": {"call"}, "User-Agent": {"hijacked"}}))
	if err != nil {
		t.Fatalf("SystemOne: %v", err)
	}

	got := s.request(0).Header
	checks := map[string]string{
		"Authorization":  "Bearer sk-test",
		"Accept":         "application/json",
		"User-Agent":     userAgent,
		"Content-Type":   "application/json",
		sdkHeader:        "go",
		sdkRuntimeHeader: runtimeHeader,
		"X-Trace":        "call", // per-call headers override the client's
	}
	for name, want := range checks {
		if have := got.Get(name); have != want {
			t.Errorf("%s = %q, want %q", name, have, want)
		}
	}
	if have := got.Get(retryCountHeader); have != "" {
		t.Errorf("%s = %q on the first attempt, want it absent", retryCountHeader, have)
	}
}

func TestRetriesServerErrors(t *testing.T) {
	t.Parallel()

	s := newScriptedServer(t,
		jsonHandler(http.StatusInternalServerError, `{"error":"boom"}`),
		jsonHandler(529, `{"error":"overloaded"}`),
		jsonHandler(http.StatusOK, okBody),
	)
	c, delays := testClient(t, s.URL)

	res, err := c.SystemOne(t.Context(), simpleRequest())
	if err != nil {
		t.Fatalf("SystemOne: %v", err)
	}
	if got, want := s.count(), 3; got != want {
		t.Errorf("attempts = %d, want %d", got, want)
	}
	if res.Meta.Attempts != 3 {
		t.Errorf("Meta.Attempts = %d, want 3", res.Meta.Attempts)
	}
	if want := []time.Duration{500 * time.Millisecond, time.Second}; !equalDurations(*delays, want) {
		t.Errorf("delays = %v, want %v", *delays, want)
	}
	// The retry count header appears only on retries, and counts them.
	for attempt, want := range []string{"", "1", "2"} {
		if got := s.request(attempt).Header.Get(retryCountHeader); got != want {
			t.Errorf("attempt %d %s = %q, want %q", attempt, retryCountHeader, got, want)
		}
	}
}

func TestGivesUpAfterMaxRetries(t *testing.T) {
	t.Parallel()

	s := newScriptedServer(t,
		jsonHandler(http.StatusInternalServerError, `{"error":"boom"}`),
		jsonHandler(http.StatusInternalServerError, `{"error":"boom"}`),
		jsonHandler(http.StatusInternalServerError, `{"error":"still boom"}`),
	)
	c, _ := testClient(t, s.URL)

	_, err := c.SystemOne(t.Context(), simpleRequest())
	var serverErr *InternalServerError
	if !errors.As(err, &serverErr) {
		t.Fatalf("SystemOne = %T (%v), want *InternalServerError", err, err)
	}
	if !strings.Contains(err.Error(), "still boom") {
		t.Errorf("SystemOne = %q, want the last attempt's message", err)
	}
	if got, want := s.count(), 3; got != want {
		t.Errorf("attempts = %d, want %d", got, want)
	}
}

func TestDoesNotRetryClientErrors(t *testing.T) {
	t.Parallel()

	s := newScriptedServer(t, jsonHandler(http.StatusUnprocessableEntity,
		`{"detail":[{"loc":["body","questions","spam"],"msg":"field required"}]}`))
	c, _ := testClient(t, s.URL)

	_, err := c.SystemOne(t.Context(), simpleRequest())
	var validationErr *UnprocessableEntityError
	if !errors.As(err, &validationErr) {
		t.Fatalf("SystemOne = %T (%v), want *UnprocessableEntityError", err, err)
	}
	if !strings.Contains(err.Error(), "questions.spam: field required") {
		t.Errorf("SystemOne = %q, want the validation detail", err)
	}
	if got := s.count(); got != 1 {
		t.Errorf("attempts = %d, want 1 (4xx must not be retried)", got)
	}
}

func TestRetryAfterIsHonored(t *testing.T) {
	t.Parallel()

	rateLimited := func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "2")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = io.WriteString(w, `{"error":"slow down"}`)
	}
	s := newScriptedServer(t, rateLimited, jsonHandler(http.StatusOK, okBody))
	c, delays := testClient(t, s.URL)

	if _, err := c.SystemOne(t.Context(), simpleRequest()); err != nil {
		t.Fatalf("SystemOne: %v", err)
	}
	if want := []time.Duration{2 * time.Second}; !equalDurations(*delays, want) {
		t.Errorf("delays = %v, want %v", *delays, want)
	}
}

func TestPerCallRetryOverride(t *testing.T) {
	t.Parallel()

	s := newScriptedServer(t, jsonHandler(http.StatusInternalServerError, `{"error":"boom"}`))
	c, _ := testClient(t, s.URL)

	_, err := c.SystemOne(t.Context(), simpleRequest(), WithRequestRetry(RetryPolicy{}))
	if err == nil {
		t.Fatal("SystemOne = nil error, want the server error")
	}
	if got := s.count(); got != 1 {
		t.Errorf("attempts = %d, want 1 with retries disabled for the call", got)
	}
}

func TestRetriesConnectionErrors(t *testing.T) {
	t.Parallel()

	var attempts int
	stub := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		attempts++
		if attempts < 3 {
			return nil, io.ErrUnexpectedEOF
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{},
			Body:       io.NopCloser(strings.NewReader(okBody)),
			Request:    r,
		}, nil
	})
	c, _ := testClient(t, "https://api.test", WithHTTPClient(&http.Client{Transport: stub}))

	if _, err := c.SystemOne(t.Context(), simpleRequest()); err != nil {
		t.Fatalf("SystemOne: %v", err)
	}
	if attempts != 3 {
		t.Errorf("attempts = %d, want 3", attempts)
	}
}

func TestConnectionErrorIsTyped(t *testing.T) {
	t.Parallel()

	stub := roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return nil, io.ErrUnexpectedEOF
	})
	c, _ := testClient(t, "https://api.test",
		WithHTTPClient(&http.Client{Transport: stub}), WithRetry(RetryPolicy{}))

	_, err := c.SystemOne(t.Context(), simpleRequest())
	var connErr *ConnectionError
	if !errors.As(err, &connErr) {
		t.Fatalf("SystemOne = %T (%v), want *ConnectionError", err, err)
	}
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Errorf("SystemOne does not unwrap to the transport error")
	}
}

func TestCallerCancellationIsReported(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	stub := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		cancel()
		<-r.Context().Done()
		return nil, r.Context().Err()
	})
	c, _ := testClient(t, "https://api.test", WithHTTPClient(&http.Client{Transport: stub}))

	_, err := c.SystemOne(ctx, simpleRequest())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("SystemOne = %v, want it to wrap context.Canceled", err)
	}
}

func TestAttemptTimeoutIsRetried(t *testing.T) {
	t.Parallel()

	var attempts int
	stub := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		attempts++
		if attempts == 1 {
			<-r.Context().Done() // exceed the per-attempt timeout
			return nil, r.Context().Err()
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{},
			Body:       io.NopCloser(strings.NewReader(okBody)),
			Request:    r,
		}, nil
	})
	c, _ := testClient(t, "https://api.test",
		WithHTTPClient(&http.Client{Transport: stub}), WithTimeout(20*time.Millisecond))

	if _, err := c.SystemOne(t.Context(), simpleRequest()); err != nil {
		t.Fatalf("SystemOne: %v", err)
	}
	if attempts != 2 {
		t.Errorf("attempts = %d, want 2", attempts)
	}
}

func TestDeadlineShorterThanBackoffSurfacesTheAPIError(t *testing.T) {
	t.Parallel()

	s := newScriptedServer(t, jsonHandler(http.StatusInternalServerError, `{"error":"boom"}`))
	c, delays := testClient(t, s.URL)

	// The next backoff is 500ms, well past this deadline: the caller is better
	// served by the real API error than by a context error.
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()

	_, err := c.SystemOne(ctx, simpleRequest())
	var serverErr *InternalServerError
	if !errors.As(err, &serverErr) {
		t.Fatalf("SystemOne = %T (%v), want *InternalServerError", err, err)
	}
	if len(*delays) != 0 {
		t.Errorf("delays = %v, want no sleep into an expiring deadline", *delays)
	}
	if got := s.count(); got != 1 {
		t.Errorf("attempts = %d, want 1", got)
	}
}

func TestRetryBudgetStopsRetries(t *testing.T) {
	t.Parallel()

	s := newScriptedServer(t, jsonHandler(http.StatusInternalServerError, `{"error":"boom"}`))
	p := DefaultRetryPolicy()
	p.Budget = 100 * time.Millisecond // shorter than the 500ms first backoff
	c, _ := testClient(t, s.URL, WithRetry(p))

	if _, err := c.SystemOne(t.Context(), simpleRequest()); err == nil {
		t.Fatal("SystemOne = nil error, want the server error")
	}
	if got := s.count(); got != 1 {
		t.Errorf("attempts = %d, want 1 within the budget", got)
	}
}

func TestInvalidJSONIsAResponseError(t *testing.T) {
	t.Parallel()

	s := newScriptedServer(t, jsonHandler(http.StatusOK, `{"answers": `))
	c, _ := testClient(t, s.URL)

	_, err := c.SystemOne(t.Context(), simpleRequest())
	var respErr *ResponseError
	if !errors.As(err, &respErr) {
		t.Fatalf("SystemOne = %T (%v), want *ResponseError", err, err)
	}
	if respErr.RequestID != "req_abc123" {
		t.Errorf("RequestID = %q, want req_abc123", respErr.RequestID)
	}
}

func TestResponseMismatchIsReported(t *testing.T) {
	t.Parallel()

	body := `{"model":"jev-1","answers":{"other":{"type":"noul","noul":0.5}},"usage":{}}`
	s := newScriptedServer(t, jsonHandler(http.StatusOK, body))
	c, _ := testClient(t, s.URL)

	_, err := c.SystemOne(t.Context(), simpleRequest())
	var mismatch *ResponseMismatchError
	if !errors.As(err, &mismatch) {
		t.Fatalf("SystemOne = %T (%v), want *ResponseMismatchError", err, err)
	}
	if mismatch.RequestID != "req_abc123" {
		t.Errorf("RequestID = %q, want req_abc123", mismatch.RequestID)
	}
	if len(mismatch.Problems) != 2 {
		t.Errorf("Problems = %v, want the missing and the extra answer", mismatch.Problems)
	}
}

func TestSystemOneValidatesLocally(t *testing.T) {
	t.Parallel()

	s := newScriptedServer(t) // no handlers: nothing may be sent
	c, _ := testClient(t, s.URL)

	tests := []struct {
		name string
		req  SystemOneRequest
	}{
		{"no state", SystemOneRequest{Questions: Questions{"a": Noul("?")}}},
		{"no questions", SystemOneRequest{State: "x"}},
		{"one-level score", SystemOneRequest{State: "x",
			Questions: Questions{"a": Score("?", ScoreCriteria{"only"})}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := c.SystemOne(t.Context(), tc.req); err == nil {
				t.Errorf("SystemOne(%s) = nil error, want a local validation error", tc.name)
			}
		})
	}
	if got := s.count(); got != 0 {
		t.Errorf("attempts = %d, want 0: validation must happen before sending", got)
	}
}

func TestListModels(t *testing.T) {
	t.Parallel()

	body := `{"models":[{"name":"jev-1","description":"The first Jev","release_date":"2026-01-15"}]}`
	s := newScriptedServer(t, jsonHandler(http.StatusOK, body))
	c, _ := testClient(t, s.URL)

	list, err := c.ListModels(t.Context())
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if len(list.Models) != 1 || list.Models[0].Name != "jev-1" {
		t.Fatalf("Models = %+v, want one jev-1", list.Models)
	}
	if list.Meta.RequestID != "req_abc123" {
		t.Errorf("Meta.RequestID = %q, want req_abc123", list.Meta.RequestID)
	}

	req := s.request(0)
	if req.Method != "GET" || req.URL.Path != modelsPath {
		t.Errorf("sent %s %s, want GET %s", req.Method, req.URL.Path, modelsPath)
	}
	// A bodyless request must not claim a JSON body.
	if got := req.Header.Get("Content-Type"); got != "" {
		t.Errorf("Content-Type = %q on a GET, want it absent", got)
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func equalDurations(got, want []time.Duration) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func TestCancellationDuringBackoffIsReported(t *testing.T) {
	t.Parallel()

	s := newScriptedServer(t,
		jsonHandler(http.StatusInternalServerError, `{"error":"boom"}`),
		jsonHandler(http.StatusOK, okBody),
	)

	ctx, cancel := context.WithCancel(t.Context())
	c, err := New(
		WithAPIKey("sk-test"), WithBaseURL(s.URL), withRand(noJitter),
		// Cancel while the client is waiting to retry.
		withSleep(func(context.Context, time.Duration) error {
			cancel()
			return context.Canceled
		}),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, err = c.SystemOne(ctx, simpleRequest())
	if !errors.Is(err, context.Canceled) {
		t.Errorf("SystemOne = %v, want it to wrap context.Canceled", err)
	}
	// The error that prompted the retry is worth keeping too.
	var serverErr *InternalServerError
	if !errors.As(err, &serverErr) {
		t.Errorf("SystemOne = %v, want it to also carry the server error", err)
	}
}
