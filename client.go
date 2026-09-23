package typesafe

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

// Headers the SDK sets on every request. They are applied after any caller
// headers, so a caller cannot replace them.
const (
	// requestIDHeader identifies a request in the API's logs. Include its value
	// when reporting a problem to TypeSafe.
	requestIDHeader = "x-typesafe-request-id"

	sdkHeader        = "X-TypeSafe-SDK"
	sdkRuntimeHeader = "X-TypeSafe-Runtime"
	retryCountHeader = "X-TypeSafe-Retry-Count"
)

// A Client calls the TypeSafe API. Create one with [New]; it is safe for
// concurrent use and does not need to be closed.
type Client struct {
	apiKey       string
	baseURL      string
	defaultModel string
	timeout      time.Duration
	retry        RetryPolicy
	headers      http.Header
	httpClient   *http.Client
	logger       *slog.Logger
	logLevel     LogLevel

	rand  func() float64
	sleep func(context.Context, time.Duration) error
}

// New returns a client configured by opts, falling back to the TYPESAFE_
// environment variables and then to the SDK defaults. The zero-option form
// works when TYPESAFE_API_KEY is set:
//
//	client, err := typesafe.New()
//
// It returns an error if no API key is available or an option is invalid.
func New(opts ...Option) (*Client, error) {
	cfg := &config{
		timeout:  DefaultTimeout,
		retry:    DefaultRetryPolicy(),
		logLevel: DefaultLogLevel,
	}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(cfg); err != nil {
			return nil, err
		}
	}
	if err := cfg.resolve(); err != nil {
		return nil, err
	}

	c := &Client{
		apiKey:       cfg.apiKey,
		baseURL:      cfg.baseURL,
		defaultModel: cfg.defaultModel,
		timeout:      cfg.timeout,
		retry:        cfg.retry,
		headers:      cfg.headers,
		httpClient:   cfg.httpClient,
		logger:       cfg.logger,
		logLevel:     cfg.logLevel,
		rand:         cfg.rand,
		sleep:        cfg.sleep,
	}
	if c.httpClient == nil {
		// No timeout of its own: the SDK applies a per-attempt timeout through
		// the context, which a client-level timeout would cut across.
		c.httpClient = &http.Client{}
	}
	if c.logger == nil {
		c.logger = slog.Default()
	}
	if c.rand == nil {
		c.rand = defaultRand
	}
	if c.sleep == nil {
		c.sleep = sleepContext
	}
	return c, nil
}

// BaseURL reports the API root the client sends requests to.
func (c *Client) BaseURL() string { return c.baseURL }

// DefaultModel reports the model used by requests that do not name one.
func (c *Client) DefaultModel() string { return c.defaultModel }

// ResponseMeta describes the HTTP response a result was decoded from. It is
// attached to every successful result, so the raw body and request ID are
// always available without a second API surface.
type ResponseMeta struct {
	// StatusCode is the HTTP status code.
	StatusCode int
	// Header holds the response headers.
	Header http.Header
	// RequestID is the x-typesafe-request-id header, when present. Quote it
	// when reporting a problem to TypeSafe.
	RequestID string
	// Body is the raw response body.
	Body json.RawMessage
	// Attempts is the number of HTTP attempts made, including the successful one.
	Attempts int
	// Duration is the elapsed time across all attempts, including backoff.
	Duration time.Duration
	// HTTP is the final response, with its body rewound and readable.
	HTTP *http.Response
}

// requestOptions resolves per-call overrides against the client's settings.
func (c *Client) requestOptions(opts []RequestOption) (*requestConfig, error) {
	rc := &requestConfig{timeout: c.timeout, retry: c.retry}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(rc); err != nil {
			return nil, err
		}
	}
	return rc, nil
}

// headersFor builds the headers for one attempt. Caller headers are applied
// first and the SDK's own headers last, so the required ones always win.
func (c *Client) headersFor(rc *requestConfig, hasBody bool, attempt int) http.Header {
	h := make(http.Header)
	for name, values := range c.headers {
		h[http.CanonicalHeaderKey(name)] = append([]string(nil), values...)
	}
	for name, values := range rc.headers {
		h[http.CanonicalHeaderKey(name)] = append([]string(nil), values...)
	}
	h.Del(retryCountHeader)

	h.Set("Authorization", "Bearer "+c.apiKey)
	h.Set("Accept", "application/json")
	h.Set("User-Agent", userAgent)
	h.Set(sdkHeader, "go")
	h.Set(sdkRuntimeHeader, runtimeHeader)
	if hasBody {
		h.Set("Content-Type", "application/json")
	} else {
		h.Del("Content-Type")
	}
	if attempt > 0 {
		h.Set(retryCountHeader, strconv.Itoa(attempt))
	}
	return h
}

// do performs a request with retries and decodes a JSON body into out.
func (c *Client) do(ctx context.Context, method, path string, payload, out any, opts ...RequestOption) (*ResponseMeta, error) {
	rc, err := c.requestOptions(opts)
	if err != nil {
		return nil, err
	}

	var body []byte
	if payload != nil {
		body, err = json.Marshal(payload)
		if err != nil {
			return nil, errf("encoding request body: %v", err)
		}
	}

	url := c.baseURL + path
	start := time.Now()
	deadline, hasDeadline := ctx.Deadline()

	var lastErr error
	for attempt := 0; ; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, joinContextError(lastErr, err)
		}

		meta, header, err := c.attempt(ctx, rc, method, url, path, body, attempt, start)
		if err == nil {
			meta.Attempts = attempt + 1
			if out != nil {
				if err := json.Unmarshal(meta.Body, out); err != nil {
					return nil, &ResponseError{Body: meta.Body, RequestID: meta.RequestID, err: err}
				}
			}
			return meta, nil
		}
		lastErr = err

		if attempt >= rc.retry.MaxRetries || !c.shouldRetry(rc.retry, err) {
			return nil, err
		}

		delay := rc.retry.backoff(attempt, header, c.rand, time.Now())
		if rc.retry.Budget > 0 && time.Since(start)+delay > rc.retry.Budget {
			return nil, err
		}
		// Never sleep into a deadline that will expire first: the caller is
		// better served by the last real API error than by a context error.
		if hasDeadline && time.Now().Add(delay).After(deadline) {
			return nil, err
		}

		c.log(ctx, LogInfo, "retrying request", "method", method, "path", path,
			"attempt", attempt+1, "delay", delay, "cause", err.Error())

		if err := c.sleep(ctx, delay); err != nil {
			// The caller gave up mid-backoff. Both facts matter, so both are
			// reported: errors.As finds the API error that prompted the retry
			// and errors.Is finds context.Canceled.
			return nil, joinContextError(lastErr, err)
		}
	}
}

// attempt performs one HTTP attempt, returning the response metadata on success
// and, on failure, the response headers that a retry delay may be read from.
func (c *Client) attempt(ctx context.Context, rc *requestConfig, method, url, path string, body []byte, attempt int, start time.Time) (*ResponseMeta, http.Header, error) {
	attemptCtx, cancel := context.WithTimeout(ctx, rc.timeout)
	// One attempt per call, so this defer cannot accumulate the way it would
	// inside the retry loop. The response body is fully buffered before
	// returning, so nothing outlives the cancel.
	defer cancel()

	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(attemptCtx, method, url, reader)
	if err != nil {
		return nil, nil, errf("building request: %v", err)
	}
	req.Header = c.headersFor(rc, body != nil, attempt)
	if body != nil {
		req.ContentLength = int64(len(body))
	}

	c.log(ctx, LogDebug, "sending request", "method", method, "path", path,
		"attempt", attempt, "headers", redactHeaders(req.Header), "body", string(body))

	began := time.Now()
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, nil, c.classifyTransportError(ctx, rc, method, url, err)
	}

	payload, readErr := io.ReadAll(resp.Body)
	closeErr := resp.Body.Close()
	if readErr == nil {
		readErr = closeErr
	}
	if readErr != nil {
		// A body that ended early is a transport failure, not a bad response.
		return nil, resp.Header, c.classifyTransportError(ctx, rc, method, url, readErr)
	}

	requestID := resp.Header.Get(requestIDHeader)
	c.log(ctx, LogInfo, "request complete", "method", method, "path", path,
		"status", resp.StatusCode, "duration", time.Since(began),
		"request_id", requestID, "attempt", attempt)
	c.log(ctx, LogDebug, "received response", "method", method, "path", path,
		"status", resp.StatusCode, "headers", redactHeaders(resp.Header), "body", string(payload))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, resp.Header, newAPIError(method, url, resp, payload)
	}

	// Rewind the body so the caller can read the response like any other.
	resp.Body = io.NopCloser(bytes.NewReader(payload))
	return &ResponseMeta{
		StatusCode: resp.StatusCode,
		Header:     resp.Header,
		RequestID:  requestID,
		Body:       payload,
		Duration:   time.Since(start),
		HTTP:       resp,
	}, resp.Header, nil
}

// classifyTransportError turns a transport failure into the SDK error that
// describes it: the caller's own cancellation, an attempt timeout, or a
// connection error.
func (c *Client) classifyTransportError(ctx context.Context, rc *requestConfig, method, url string, err error) error {
	// The parent context, not the per-attempt one, distinguishes the caller
	// giving up from an attempt running out of its own time.
	if parentErr := ctx.Err(); parentErr != nil {
		return errors.Join(errf("%s %s: %v", method, url, parentErr), parentErr)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return &TimeoutError{Timeout: rc.timeout, Method: method, URL: url, err: err}
	}
	return &ConnectionError{Method: method, URL: url, err: err}
}

// joinContextError reports that the caller's context ended, keeping the error
// that prompted the retry so both can be matched.
func joinContextError(lastErr, ctxErr error) error {
	if lastErr == nil {
		return ctxErr
	}
	return errors.Join(lastErr, ctxErr)
}

// shouldRetry reports whether an attempt's error is one the policy retries.
func (c *Client) shouldRetry(p RetryPolicy, err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return p.retriesStatus(apiErr.StatusCode)
	}
	var timeoutErr *TimeoutError
	if errors.As(err, &timeoutErr) {
		return p.RetryTimeouts
	}
	var connErr *ConnectionError
	if errors.As(err, &connErr) {
		return p.RetryConnectionErrors
	}
	return false
}

// log emits an SDK log record if level meets the configured verbosity.
func (c *Client) log(ctx context.Context, level LogLevel, msg string, args ...any) {
	if c.logLevel == LogOff || level < c.logLevel {
		return
	}
	c.logger.Log(ctx, level.slogLevel(), "typesafe: "+msg, args...)
}
