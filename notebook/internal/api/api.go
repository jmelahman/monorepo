// Package api is the notebook's HTTP API: the parts of the app a browser
// page can't do by itself, chiefly running Claude Code.
package api

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/jmelahman/notebook/internal/claude"
	"github.com/jmelahman/notebook/internal/publish"
	"github.com/jmelahman/notebook/internal/sources"
	"github.com/jmelahman/notebook/internal/speech"
)

// MaxSourceChars caps the text sent with one request, about 250k tokens.
const MaxSourceChars = 1_000_000

// Server serves /api/.
type Server struct {
	Claude    *claude.Runner
	Sources   *sources.Client
	Publisher *publish.Publisher
	// Voices speak on this computer, unless TTSURL names an
	// OpenAI-compatible speech server, such as Kokoro-FastAPI, to use instead.
	Voices   *speech.Voices
	TTSURL   string
	TTSModel string
	// Hosts are the Host headers accepted besides loopback ones, against DNS
	// rebinding.
	Hosts []string
}

// Handler returns the API's routes.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/status", s.status)
	mux.HandleFunc("POST /api/discover", s.discover)
	mux.HandleFunc("POST /api/fetch", s.fetch)
	mux.HandleFunc("POST /api/script", s.script)
	mux.HandleFunc("POST /api/tts", s.tts)
	mux.HandleFunc("POST /api/publish", s.publish)
	return mux
}

// Guard refuses requests that another site's page could have made: a Host
// that isn't this server's (DNS rebinding), or a cross-origin write. The
// API spends the Claude subscription and writes files, so only the
// notebook's own page may call it.
func (s *Server) Guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}
		if !isLoopback(host) && !slices.Contains(s.Hosts, host) {
			http.Error(w, "unknown host "+host+" (allow it with --host)", http.StatusForbidden)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			if o := r.Header.Get("Origin"); o != "" {
				u, err := url.Parse(o)
				if err != nil || u.Host != r.Host {
					http.Error(w, "cross-origin request refused", http.StatusForbidden)
					return
				}
			}
			if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
				http.Error(w, "cross-site request refused", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func isLoopback(host string) bool {
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	return ip != nil && ip.IsLoopback()
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, err error) {
	writeJSON(w, code, map[string]string{"error": err.Error()})
}

func readJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 4*MaxSourceChars)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("bad request: %w", err))
		return false
	}
	return true
}

// events streams server-sent events to a fetch() on the page.
type events struct {
	w  http.ResponseWriter
	rc *http.ResponseController
	mu sync.Mutex
}

func newEvents(w http.ResponseWriter) *events {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	return &events{w: w, rc: http.NewResponseController(w)}
}

func (e *events) send(name string, v any) {
	b, err := json.Marshal(v)
	if err != nil {
		b, _ = json.Marshal(map[string]string{"error": err.Error()})
		name = "error"
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	fmt.Fprintf(e.w, "event: %s\ndata: %s\n\n", name, b)
	e.rc.Flush()
}

func (e *events) fail(err error) {
	e.send("error", map[string]string{"error": err.Error()})
}

// runner is the server's Claude runner with a request's model and effort.
func (s *Server) runner(model, effort string) *claude.Runner {
	r := *s.Claude
	r.Model = cmp.Or(model, r.Model)
	if slices.Contains([]string{"low", "medium", "high", "xhigh", "max"}, effort) {
		r.Effort = effort
	}
	return &r
}

func (s *Server) status(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	type part struct {
		OK     bool   `json:"ok"`
		Detail string `json:"detail"`
	}
	var st struct {
		Claude  part     `json:"claude"`
		Publish part     `json:"publish"`
		TTS     part     `json:"tts"`
		Models  []string `json:"models"`
	}
	st.Models = claude.Models
	st.Claude.OK, st.Claude.Detail = s.Claude.Status(ctx)
	if err := s.Publisher.Ready(); err != nil {
		st.Publish.Detail = err.Error()
	} else {
		st.Publish.OK, st.Publish.Detail = true, s.Publisher.Dir
	}
	switch s.TTSURL {
	case "":
		st.TTS.OK, st.TTS.Detail = s.Voices.Available()
	default:
		st.TTS.OK, st.TTS.Detail = true, s.TTSURL
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(s.TTSURL, "/")+"/v1/models", nil)
		if err != nil {
			st.TTS.OK, st.TTS.Detail = false, "bad speech server address: "+err.Error()
			break
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			st.TTS.OK, st.TTS.Detail = false, "speech server unreachable: "+err.Error()
			break
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			st.TTS.OK, st.TTS.Detail = false, "speech server: "+resp.Status
		}
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) discover(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Topic   string   `json:"topic"`
		Count   int      `json:"count"`
		Exclude []string `json:"exclude"`
		Model   string   `json:"model"`
		Effort  string   `json:"effort"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.Topic) == "" {
		writeError(w, http.StatusBadRequest, errors.New("no topic"))
		return
	}
	req.Count = min(max(req.Count, 1), 20)
	ev := newEvents(w)
	res, err := s.runner(req.Model, req.Effort).Run(r.Context(), claude.Request{
		System: discoverSystem,
		Prompt: discoverPrompt(req.Topic, req.Count, req.Exclude),
		Tools:  []string{"WebSearch", "WebFetch"},
		Schema: discoverSchema,
		OnTool: func(name string, input json.RawMessage) {
			var in struct {
				Query string `json:"query"`
				URL   string `json:"url"`
			}
			_ = json.Unmarshal(input, &in) // progress only; a tool without these is fine
			ev.send("progress", map[string]string{"tool": name, "query": in.Query, "url": in.URL})
		},
	})
	if err != nil {
		ev.fail(err)
		return
	}
	ev.send("result", res.Structured)
}

func (s *Server) fetch(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL string `json:"url"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	src, err := s.Sources.Fetch(r.Context(), req.URL)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, src)
}

func checkSources(sources []Source) error {
	if len(sources) == 0 {
		return errors.New("no sources selected")
	}
	n := 0
	for _, s := range sources {
		n += utf8.RuneCountInString(s.Text)
	}
	if n > MaxSourceChars {
		return fmt.Errorf("the selected sources are too long together (%d characters; the most is %d)", n, MaxSourceChars)
	}
	return nil
}

// Segment is part of an episode's outline.
type Segment struct {
	Heading string  `json:"heading"`
	Plan    string  `json:"plan"`
	Minutes float64 `json:"minutes"`
}

// Line is one turn of the script.
type Line struct {
	Speaker string `json:"speaker"`
	Text    string `json:"text"`
}

type outline struct {
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Segments    []Segment `json:"segments"`
}

// script writes an episode in one Claude Code session: the outline first,
// then each segment in turn, so the sources are sent once and every segment
// is written knowing what came before it.
func (s *Server) script(w http.ResponseWriter, r *http.Request) {
	var req ScriptRequest
	if !readJSON(w, r, &req) {
		return
	}
	if err := checkSources(req.Sources); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	req.Minutes = min(max(req.Minutes, 2), 60)
	for i, def := range [2]string{"Alex", "Sam"} {
		req.Hosts[i].Name = cmp.Or(strings.TrimSpace(req.Hosts[i].Name), def)
	}
	ctx := r.Context()
	run := s.runner(req.Model, req.Effort)
	ev := newEvents(w)

	ev.send("progress", map[string]string{"step": "outline"})
	res, err := run.Run(ctx, claude.Request{
		System: scriptSystem,
		Prompt: outlinePrompt(req),
		Schema: outlineSchema,
		Keep:   true,
	})
	if err != nil {
		ev.fail(fmt.Errorf("outline: %w", err))
		return
	}
	var out outline
	if err := json.Unmarshal(res.Structured, &out); err != nil || len(out.Segments) == 0 {
		ev.fail(fmt.Errorf("outline: unusable reply: %s", res.Structured))
		return
	}
	ev.send("outline", out)

	session := res.SessionID
	var tail []Line
	for i, seg := range out.Segments {
		ev.send("progress", map[string]any{"step": "segment", "segment": i, "of": len(out.Segments)})
		prompt := segmentPrompt(i, seg, len(out.Segments), tail, req.Hosts)
		res, err := run.Run(ctx, claude.Request{System: scriptSystem, Prompt: prompt, Schema: linesSchema, Resume: session})
		if errors.Is(err, claude.ErrNoSession) || (err == nil && res.SessionID == "") {
			// Start over from the sources and the outline.
			outlineJSON, _ := json.Marshal(out)
			res, err = run.Run(ctx, claude.Request{
				System: scriptSystem,
				Prompt: outlinePrompt(req) + "\n\nThe outline, already planned:\n" + string(outlineJSON) + "\n\n" + prompt,
				Schema: linesSchema,
				Keep:   true,
			})
		}
		if err != nil {
			ev.fail(fmt.Errorf("segment %d: %w", i+1, err))
			return
		}
		var got struct {
			Lines []Line `json:"lines"`
		}
		if err := json.Unmarshal(res.Structured, &got); err != nil {
			ev.fail(fmt.Errorf("segment %d: unusable reply: %w", i+1, err))
			return
		}
		ev.send("lines", map[string]any{"segment": i, "lines": got.Lines})
		session = res.SessionID
		tail = got.Lines[max(0, len(got.Lines)-4):]
	}
	ev.send("done", struct{}{})
}

// tts speaks a line, faster than the browser can: on this computer, or on
// the speech server.
func (s *Server) tts(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Voice string  `json:"voice"`
		Input string  `json:"input"`
		Speed float64 `json:"speed"`
		Model string  `json:"model"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if s.TTSURL == "" {
		audio, err := s.Voices.Speak(r.Context(), req.Input, req.Voice, req.Speed)
		if err != nil {
			if r.Context().Err() == nil {
				writeError(w, http.StatusInternalServerError, err)
			}
			return
		}
		w.Header().Set("Content-Type", "audio/wav")
		_, _ = w.Write(audio)
		return
	}
	body, _ := json.Marshal(map[string]any{
		"model":           cmp.Or(req.Model, s.TTSModel),
		"voice":           req.Voice,
		"input":           req.Input,
		"speed":           cmp.Or(req.Speed, 1),
		"response_format": "wav",
	})
	up, err := http.NewRequestWithContext(r.Context(), http.MethodPost, strings.TrimRight(s.TTSURL, "/")+"/v1/audio/speech", bytes.NewReader(body))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	up.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(up)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		writeError(w, http.StatusBadGateway, fmt.Errorf("speech server: %s: %s", resp.Status, bytes.TrimSpace(msg)))
		return
	}
	w.Header().Set("Content-Type", cmp.Or(resp.Header.Get("Content-Type"), "audio/wav"))
	_, _ = io.Copy(w, resp.Body)
}

func (s *Server) publish(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 2<<30)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	f, _, err := r.FormFile("audio")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	defer f.Close()
	path, err := s.Publisher.Publish(r.Context(), publish.Episode{
		Podcast:     r.FormValue("podcast"),
		Title:       r.FormValue("title"),
		Description: r.FormValue("description"),
	}, f)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	slog.Info("published", "path", path)
	writeJSON(w, http.StatusOK, map[string]string{"path": path})
}
