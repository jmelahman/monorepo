package typesafe

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// maxErrorBodyLength caps how much of an unrecognized error body is quoted in an
// error message.
const maxErrorBodyLength = 200

// Error is a TypeSafe error that is not tied to an HTTP response, such as a
// configuration or validation failure.
type Error struct {
	Message string
}

func (e *Error) Error() string { return "typesafe: " + e.Message }

func errf(format string, a ...any) *Error {
	return &Error{Message: fmt.Sprintf(format, a...)}
}

// APIError is an error response from the API.
//
// The status-specific types [BadRequestError], [AuthenticationError],
// [PermissionDeniedError], [NotFoundError], [UnprocessableEntityError],
// [RateLimitError], and [InternalServerError] wrap an APIError, so
// errors.As(err, &apiErr) matches any of them.
type APIError struct {
	// StatusCode is the HTTP status code of the response.
	StatusCode int
	// Method and URL identify the request that failed. The URL has any
	// credentials, query, and fragment removed.
	Method string
	URL    string
	// RequestID is the x-typesafe-request-id header, when present.
	RequestID string
	// Header holds the response headers.
	Header http.Header
	// Body is the raw response body.
	Body []byte
	// Message is the human-readable message extracted from the body.
	Message string
}

func (e *APIError) Error() string {
	var b strings.Builder
	b.WriteString("typesafe: ")
	b.WriteString(e.Method)
	b.WriteString(" ")
	b.WriteString(e.URL)
	b.WriteString(": ")
	b.WriteString(e.Message)
	if e.RequestID != "" {
		b.WriteString(" (request_id=")
		b.WriteString(e.RequestID)
		b.WriteString(")")
	}
	return b.String()
}

// BadRequestError is a 400 response.
type BadRequestError struct{ *APIError }

// AuthenticationError is a 401 response. The API key is missing or invalid.
type AuthenticationError struct{ *APIError }

// PermissionDeniedError is a 403 response.
type PermissionDeniedError struct{ *APIError }

// NotFoundError is a 404 response.
type NotFoundError struct{ *APIError }

// UnprocessableEntityError is a 422 response. The request body failed
// validation; the message names the offending field.
type UnprocessableEntityError struct{ *APIError }

// RateLimitError is a 429 response.
type RateLimitError struct {
	*APIError
	// RetryAfter is the delay the server asked for, when it sent one.
	RetryAfter time.Duration
}

// InternalServerError is a 5xx response, including the API's non-standard
// 529 Overloaded.
type InternalServerError struct{ *APIError }

func (e *BadRequestError) Unwrap() error          { return e.APIError }
func (e *AuthenticationError) Unwrap() error      { return e.APIError }
func (e *PermissionDeniedError) Unwrap() error    { return e.APIError }
func (e *NotFoundError) Unwrap() error            { return e.APIError }
func (e *UnprocessableEntityError) Unwrap() error { return e.APIError }
func (e *RateLimitError) Unwrap() error           { return e.APIError }
func (e *InternalServerError) Unwrap() error      { return e.APIError }

// newAPIError builds the error type matching the response status.
func newAPIError(method, url string, resp *http.Response, body []byte) error {
	base := &APIError{
		StatusCode: resp.StatusCode,
		Method:     method,
		URL:        url,
		RequestID:  resp.Header.Get(requestIDHeader),
		Header:     resp.Header,
		Body:       body,
		Message:    fmt.Sprintf("%d %s", resp.StatusCode, extractMessage(body)),
	}
	switch base.StatusCode {
	case http.StatusBadRequest:
		return &BadRequestError{base}
	case http.StatusUnauthorized:
		return &AuthenticationError{base}
	case http.StatusForbidden:
		return &PermissionDeniedError{base}
	case http.StatusNotFound:
		return &NotFoundError{base}
	case http.StatusUnprocessableEntity:
		return &UnprocessableEntityError{base}
	case http.StatusTooManyRequests:
		err := &RateLimitError{APIError: base}
		if d, ok := parseRetryAfter(resp.Header, time.Now()); ok {
			err.RetryAfter = d
		}
		return err
	}
	if base.StatusCode >= 500 {
		return &InternalServerError{base}
	}
	return base
}

// extractMessage pulls a human-readable message out of an error body, trying the
// shapes the API is known to return before falling back to the raw body.
func extractMessage(body []byte) string {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return "status code (no body)"
	}

	var parsed any
	if err := json.Unmarshal(body, &parsed); err != nil {
		return truncate(trimmed)
	}

	if s, ok := parsed.(string); ok {
		return s
	}
	obj, ok := parsed.(map[string]any)
	if !ok {
		return truncate(trimmed)
	}

	// {"error": "..."} or {"error": {"message": "..."}}
	switch v := obj["error"].(type) {
	case string:
		return v
	case map[string]any:
		if s, ok := v["message"].(string); ok {
			return s
		}
	}

	// {"message": "..."}
	if s, ok := obj["message"].(string); ok {
		return s
	}

	// {"detail": ...}, the FastAPI shape this API uses for validation errors.
	switch v := obj["detail"].(type) {
	case string:
		return v
	case map[string]any:
		if s, ok := v["message"].(string); ok {
			return s
		}
	case []any:
		if s := formatValidationErrors(v); s != "" {
			return s
		}
	}

	return truncate(trimmed)
}

// formatValidationErrors renders a FastAPI validation error list as
// "questions.q.criteria: field required; other.loc: msg".
func formatValidationErrors(items []any) string {
	parts := make([]string, 0, len(items))
	for _, item := range items {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		msg, _ := entry["msg"].(string)
		loc, _ := entry["loc"].([]any)

		segments := make([]string, 0, len(loc))
		for _, l := range loc {
			switch s := l.(type) {
			case string:
				if s == "body" { // structural, not useful to the caller
					continue
				}
				segments = append(segments, s)
			case float64:
				segments = append(segments, fmt.Sprintf("%d", int(s)))
			}
		}
		switch {
		case len(segments) > 0 && msg != "":
			parts = append(parts, strings.Join(segments, ".")+": "+msg)
		case msg != "":
			parts = append(parts, msg)
		}
	}
	return strings.Join(parts, "; ")
}

func truncate(s string) string {
	if len(s) <= maxErrorBodyLength {
		return s
	}
	return s[:maxErrorBodyLength] + "…"
}

// ConnectionError is a failure to reach the API, including a response body that
// ended early. It is retried by default.
type ConnectionError struct {
	// Method and URL identify the request that failed.
	Method string
	URL    string
	err    error
}

func (e *ConnectionError) Error() string {
	return fmt.Sprintf("typesafe: %s %s: connection error: %v", e.Method, e.URL, e.err)
}

func (e *ConnectionError) Unwrap() error { return e.err }

// TimeoutError is an attempt that exceeded its timeout. It is retried by default.
//
// A timeout on the caller's own context is reported as that context's error
// instead, so errors.Is(err, context.DeadlineExceeded) identifies it.
type TimeoutError struct {
	// Timeout is the per-attempt timeout that elapsed.
	Timeout time.Duration
	Method  string
	URL     string
	err     error
}

func (e *TimeoutError) Error() string {
	return fmt.Sprintf("typesafe: %s %s: timed out after %s", e.Method, e.URL, e.Timeout)
}

func (e *TimeoutError) Unwrap() error { return e.err }

// ResponseError is a successful HTTP response whose body did not match what the
// endpoint is documented to return.
type ResponseError struct {
	// Field names the part of the body that was wrong, such as
	// "answers.billing.probabilities".
	Field string
	// Body is the raw response body.
	Body []byte
	// RequestID is the x-typesafe-request-id header, when present.
	RequestID string
	err       error
}

func (e *ResponseError) Error() string {
	var b strings.Builder
	b.WriteString("typesafe: invalid response")
	if e.Field != "" {
		b.WriteString(" at ")
		b.WriteString(e.Field)
	}
	if e.err != nil {
		b.WriteString(": ")
		b.WriteString(e.err.Error())
	}
	if e.RequestID != "" {
		b.WriteString(" (request_id=")
		b.WriteString(e.RequestID)
		b.WriteString(")")
	}
	return b.String()
}

func (e *ResponseError) Unwrap() error { return e.err }
