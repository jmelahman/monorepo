package kev

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// maxBody bounds a request's size, well above the most a state's tokens can
// take up.
const maxBody = 64 << 20

const systemOnePath = "/v1/systemone"

// Handler serves POST /v1/systemone, the API of Kev's server and of TypeSafe's
// hosted System One models.
func (m *Model) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST "+systemOnePath, m.systemOne)
	return mux
}

// Transport answers System One requests in this process, with no socket, so a
// client of the hosted API runs locally with no other change. The SDK insists on
// a key, which is ignored here:
//
//	typesafe.New(
//		typesafe.WithAPIKey("local"),
//		typesafe.WithHTTPClient(&http.Client{Transport: model.Transport()}),
//	)
func (m *Model) Transport() http.RoundTripper { return transport{m.Handler()} }

type transport struct{ h http.Handler }

func (t transport) RoundTrip(r *http.Request) (*http.Response, error) {
	w := &recorder{header: http.Header{}, code: http.StatusOK}
	if r.Body == nil {
		r = r.Clone(r.Context())
		r.Body = http.NoBody
	}
	t.h.ServeHTTP(w, r)
	r.Body.Close()
	if err := r.Context().Err(); err != nil {
		return nil, err
	}
	return &http.Response{
		Status:        strconv.Itoa(w.code) + " " + http.StatusText(w.code),
		StatusCode:    w.code,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        w.header,
		Body:          io.NopCloser(&w.body),
		ContentLength: int64(w.body.Len()),
		Request:       r,
	}, nil
}

type recorder struct {
	header http.Header
	body   bytes.Buffer
	code   int
}

func (w *recorder) Header() http.Header         { return w.header }
func (w *recorder) WriteHeader(code int)        { w.code = code }
func (w *recorder) Write(b []byte) (int, error) { return w.body.Write(b) }

func (m *Model) systemOne(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBody))
	if err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, err)
		return
	}
	req, err := parseRequest(body)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}
	start := time.Now()
	questions := make([]Question, len(req.questions))
	for i, q := range req.questions {
		questions[i] = q.q
	}
	probs, tokens, err := m.Decide(r.Context(), req.state, questions)
	if err != nil {
		var overflow *OverflowError
		if errors.As(err, &overflow) {
			writeError(w, http.StatusUnprocessableEntity, err)
		} else {
			writeError(w, http.StatusInternalServerError, err)
		}
		return
	}
	answers := make(map[string]any, len(probs))
	for i, q := range req.questions {
		answers[q.id] = q.answer(probs[i])
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"model":      req.model,
		"answers":    answers,
		"usage":      map[string]int{"input_tokens": tokens, "output_tokens": m.outputTokens(answers)},
		"latency_ms": float64(time.Since(start).Milliseconds()),
	})
}

// outputTokens is a billing-style figure, the tokens of the serialized answers:
// the model generates nothing.
func (m *Model) outputTokens(answers map[string]any) int {
	var b strings.Builder
	if err := json.NewEncoder(&b).Encode(answers); err != nil {
		return 0
	}
	return len(m.tok.Encode(b.String()))
}

func writeError(w http.ResponseWriter, code int, err error) {
	writeJSON(w, code, map[string]string{"detail": err.Error()})
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		return
	}
}
