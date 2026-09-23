package typesafe

import (
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestExtractMessage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "empty body", body: "", want: "status code (no body)"},
		{name: "plain json string", body: `"nope"`, want: "nope"},
		{name: "error string", body: `{"error":"nope"}`, want: "nope"},
		{name: "error object", body: `{"error":{"message":"nope"}}`, want: "nope"},
		{name: "message", body: `{"message":"nope"}`, want: "nope"},
		{name: "detail string", body: `{"detail":"nope"}`, want: "nope"},
		{name: "detail object", body: `{"detail":{"message":"nope"}}`, want: "nope"},
		{
			name: "fastapi validation array drops the body segment",
			body: `{"detail":[{"loc":["body","questions","spam","criteria"],"msg":"field required"}]}`,
			want: "questions.spam.criteria: field required",
		},
		{
			name: "fastapi validation array joins several entries and indexes",
			body: `{"detail":[{"loc":["body","questions",0],"msg":"bad"},{"loc":["body","state"],"msg":"required"}]}`,
			want: "questions.0: bad; state: required",
		},
		{name: "unrecognized shape falls back to the body", body: `{"weird":true}`, want: `{"weird":true}`},
		{name: "invalid json falls back to the body", body: `<html>down</html>`, want: `<html>down</html>`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := extractMessage([]byte(tc.body)); got != tc.want {
				t.Errorf("extractMessage(%s) = %q, want %q", tc.body, got, tc.want)
			}
		})
	}
}

func TestExtractMessageTruncates(t *testing.T) {
	t.Parallel()

	body := strings.Repeat("x", maxErrorBodyLength+50)
	got := extractMessage([]byte(body))
	if want := maxErrorBodyLength + len("…"); len(got) != want {
		t.Errorf("len(extractMessage(long)) = %d, want %d", len(got), want)
	}
	if !strings.HasSuffix(got, "…") {
		t.Errorf("extractMessage(long) = %q, want it to end with an ellipsis", got)
	}
}

func TestNewAPIError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		status int
		as     any
	}{
		{http.StatusBadRequest, new(*BadRequestError)},
		{http.StatusUnauthorized, new(*AuthenticationError)},
		{http.StatusForbidden, new(*PermissionDeniedError)},
		{http.StatusNotFound, new(*NotFoundError)},
		{http.StatusUnprocessableEntity, new(*UnprocessableEntityError)},
		{http.StatusTooManyRequests, new(*RateLimitError)},
		{http.StatusInternalServerError, new(*InternalServerError)},
		{529, new(*InternalServerError)}, // the API's non-standard Overloaded
	}

	for _, tc := range tests {
		resp := &http.Response{StatusCode: tc.status, Header: http.Header{}}
		resp.Header.Set(requestIDHeader, "req_123")
		err := newAPIError("POST", "https://api.typesafe.ai/v1/systemone", resp, []byte(`{"error":"nope"}`))

		if !errors.As(err, tc.as) {
			t.Errorf("status %d produced %T, want %T", tc.status, err, tc.as)
		}
		var apiErr *APIError
		if !errors.As(err, &apiErr) {
			t.Fatalf("status %d does not unwrap to *APIError", tc.status)
		}
		if apiErr.RequestID != "req_123" {
			t.Errorf("RequestID = %q, want req_123", apiErr.RequestID)
		}
		if !strings.Contains(err.Error(), "req_123") || !strings.Contains(err.Error(), "nope") {
			t.Errorf("Error() = %q, want the message and request ID", err)
		}
	}
}

func TestRateLimitErrorCarriesRetryAfter(t *testing.T) {
	t.Parallel()

	resp := &http.Response{StatusCode: http.StatusTooManyRequests, Header: http.Header{}}
	resp.Header.Set("Retry-After", "4")
	err := newAPIError("POST", "https://api.typesafe.ai/v1/systemone", resp, nil)

	var rateErr *RateLimitError
	if !errors.As(err, &rateErr) {
		t.Fatalf("got %T, want *RateLimitError", err)
	}
	if rateErr.RetryAfter != 4*time.Second {
		t.Errorf("RetryAfter = %s, want 4s", rateErr.RetryAfter)
	}
}
