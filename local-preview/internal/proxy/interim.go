package proxy

// Interim status pages ("Building preview…", "Starting frontend…", …).
//
// These used to be <meta http-equiv="refresh"> pages, which interact badly
// with Firefox's autorefresh blocker: every 2s refresh re-prompts "allow this
// page to automatically reload", and allowing it just lands back on the same
// prompt. Instead the page carries a small inline poller: it re-fetches its
// own URL with the X-Preview-Poll header, renders the JSON it gets back
// (state, detail, and — while a process is starting — an incremental run-log
// tail), and reloads exactly once, when a poll comes back without the
// X-Preview-Interim marker, i.e. when the real preview answered.
//
// Non-browser callers (no text/html Accept) keep the old contract: a JSON
// error body with Retry-After.

import (
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"os"
	"strconv"

	"github.com/jmelahman/local-preview/internal/supervise"
)

const (
	// pollHeader marks a request from the interim page's poller.
	pollHeader = "X-Preview-Poll"
	// pollAttemptHeader / pollOffsetHeader echo the previous chunk's cursor so
	// the poller receives only appended log bytes (see supervise.RunLog).
	pollAttemptHeader = "X-Preview-Log-Attempt"
	pollOffsetHeader  = "X-Preview-Log-Offset"
	// interimHeader marks any interim response; its absence on a poll response
	// is what tells the page the preview is ready and it should reload.
	interimHeader = "X-Preview-Interim"
)

// RunLogs is the slice of the runtime view the interim pages need to stream
// startup output. Both the local *supervise.Manager and the fleet registry
// satisfy it; leaving it unset just serves the pages without logs.
type RunLogs interface {
	RunLog(repoName, side, hash string, attempt int, offset int64) (supervise.RunLog, error)
}

// SetRunLogs wires the run-log source the interim "starting" pages stream
// from. Optional: without it the pages still poll for readiness.
func (rt *Router) SetRunLogs(logs RunLogs) { rt.runLogs = logs }

// ProcStatus is the slice of the runtime view the interim pages use to pick
// which side to narrate: a process-mode frontend spends its cold start inside
// its backend's init, and the page should say so.
type ProcStatus interface {
	Status(k supervise.Key) string
}

// SetProcStatus wires the process-status source. Optional: without it a
// starting frontend is narrated as the frontend even while its backend is the
// side doing the work.
func (rt *Router) SetProcStatus(ps ProcStatus) { rt.procStatus = ps }

// interim describes one interim state of a preview address.
type interim struct {
	state  string // "building" | "waking" | "starting" | "hydrating" | "failed"
	title  string
	detail string
	// Non-empty hash: polls include the run log of this artifact side.
	repo, side, hash string
	// Non-empty logPath: polls stream this local file instead (the build logs
	// — builds run on this node even in a fleet). logAttempt is the poll
	// protocol's attempt number for the file: advancing it (fe log = 1, be log
	// = 2) is what tells the page to clear the pane between build phases.
	logPath    string
	logAttempt int
}

func isPoll(r *http.Request) bool { return r.Header.Get(pollHeader) != "" }

// interimResponse answers a request that can't be served yet: JSON for the
// page's poller, the old Retry-After JSON for API callers, the poller page
// for browsers.
func (rt *Router) interimResponse(w http.ResponseWriter, r *http.Request, status int, it interim) {
	if isPoll(r) {
		rt.pollJSON(w, r, status, it)
		return
	}
	// Interim states are moments, not resources: any cache (browser, proxy)
	// replaying one would show "Building…" long after the preview is ready.
	w.Header().Set("Cache-Control", "no-store")
	if !wantsHTML(r) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Retry-After", "2")
		w.WriteHeader(status)
		fmt.Fprintf(w, `{"error":%q}`+"\n", it.title+": "+it.detail)
		return
	}
	w.Header().Set(interimHeader, it.state)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	fmt.Fprintf(w, interimHTML,
		html.EscapeString(it.title), html.EscapeString(it.title), html.EscapeString(it.detail))
}

// pollPayload is one poll response. attempt/offset are the run-log cursor the
// poller echoes back; content is only the bytes appended since.
type pollPayload struct {
	State     string `json:"state"`
	Title     string `json:"title"`
	Detail    string `json:"detail"`
	Attempt   int    `json:"attempt,omitempty"`
	Offset    int64  `json:"offset,omitempty"`
	Content   string `json:"content,omitempty"`
	Truncated bool   `json:"truncated,omitempty"`
}

func (rt *Router) pollJSON(w http.ResponseWriter, r *http.Request, status int, it interim) {
	p := pollPayload{State: it.state, Title: it.title, Detail: it.detail}
	attempt, _ := strconv.Atoi(r.Header.Get(pollAttemptHeader))
	offset, _ := strconv.ParseInt(r.Header.Get(pollOffsetHeader), 10, 64)
	if rt.runLogs != nil && it.hash != "" {
		if chunk, err := rt.runLogs.RunLog(it.repo, it.side, it.hash, attempt, offset); err == nil {
			p.Attempt = chunk.Attempt
			p.Offset = chunk.Offset
			p.Content = chunk.Content
			p.Truncated = chunk.Truncated
		}
	} else if it.logPath != "" {
		chunk := tailFile(it.logPath, it.logAttempt, attempt, offset)
		p.Attempt = chunk.Attempt
		p.Offset = chunk.Offset
		p.Content = chunk.Content
		p.Truncated = chunk.Truncated
	}
	w.Header().Set(interimHeader, it.state)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(p)
}

const (
	// A fresh view of a build log starts at its last 16 KiB — enough context
	// without dumping a whole verbose build into the page at once.
	buildLogTailBytes  = 16 << 10
	buildLogChunkBytes = 64 << 10
)

// tailFile is the build-log analogue of supervise.RunLog over one plain local
// file: echoing (attempt, offset) yields only appended bytes, and any cursor
// from a different attempt (a different file — the fe→be phase switch) resets
// the view to the file's tail. A missing file is an empty view, not an error:
// the build simply hasn't opened its log yet.
func tailFile(path string, viewAttempt, reqAttempt int, offset int64) supervise.RunLog {
	chunk := supervise.RunLog{Attempt: viewAttempt}
	f, err := os.Open(path)
	if err != nil {
		return chunk
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return chunk
	}
	size := info.Size()

	start := int64(0)
	if reqAttempt == viewAttempt && offset >= 0 && offset <= size {
		start = offset
	} else if size > buildLogTailBytes {
		start = size - buildLogTailBytes
		chunk.Truncated = true
	}

	buf := make([]byte, min(size-start, buildLogChunkBytes))
	n, err := f.ReadAt(buf, start)
	if err != nil && err != io.EOF {
		return chunk
	}
	chunk.Content = string(buf[:n])
	chunk.Offset = start + int64(n)
	return chunk
}

// interimHTML takes (title, title, detail), pre-escaped. The script is fully
// static — nothing request-derived is interpolated into it.
const interimHTML = `<!doctype html><html><head><title>%s</title>
<style>
body{font-family:system-ui,sans-serif;display:flex;min-height:100vh;align-items:center;justify-content:center;background:#111;color:#eee;margin:0}
main{max-width:44rem;width:100%%;padding:2rem}
h1{font-size:1.25rem;display:flex;align-items:center;gap:.6rem}
p{color:#aaa;line-height:1.5}
.muted{color:#666;font-size:.85rem}
#err{color:#f88}
.spin{width:1rem;height:1rem;border:2px solid #555;border-top-color:#eee;border-radius:50%%;animation:s 1s linear infinite;flex:none}
@keyframes s{to{transform:rotate(360deg)}}
pre{background:#000;border:1px solid #333;border-radius:6px;padding:1rem;max-height:24rem;overflow:auto;white-space:pre-wrap;word-break:break-word;font-size:.8rem;line-height:1.4;color:#ccc}
</style></head><body><main>
<h1><span class=spin id=spin></span><span id=title>%s</span></h1>
<p id=detail>%s</p>
<pre id=log hidden></pre>
<p id=err hidden></p>
<p class=muted id=hint>This page will load the preview automatically when it's ready.<span id=elapsed></span></p>
<p class=muted id=slow hidden>Still waiting for a server to join the fleet — it may be at its node cap, or cloud capacity is tight right now.</p>
</main>
<script>
(() => {
  const $ = id => document.getElementById(id);
  let attempt = 0, offset = 0;
  const t0 = Date.now();
  const fmtElapsed = ms => {
    const s = Math.floor(ms / 1000), m = Math.floor(s / 60);
    return m > 0 ? m + 'm ' + (s - m * 60) + 's' : s + 's';
  };
  const tick = async () => {
    $('elapsed').textContent = ' Waiting ' + fmtElapsed(Date.now() - t0) + '.';
    let res;
    try {
      res = await fetch(location.href, {
        headers: {
          'X-Preview-Poll': '1',
          'X-Preview-Log-Attempt': String(attempt),
          'X-Preview-Log-Offset': String(offset),
        },
        cache: 'no-store',
        redirect: 'manual',
      });
    } catch { setTimeout(tick, 1000); return; }
    // No interim marker: the preview (or a real error page) answered — load it.
    if (!res.headers.get('X-Preview-Interim')) { location.reload(); return; }
    let s = null;
    try { s = await res.json(); } catch {}
    if (s) {
      if (s.title) { document.title = s.title; $('title').textContent = s.title; }
      if (s.attempt) {
        if (s.attempt !== attempt) { attempt = s.attempt; $('log').textContent = ''; }
        if (s.content) {
          const el = $('log');
          const stick = el.hidden || el.scrollTop + el.clientHeight >= el.scrollHeight - 4;
          el.hidden = false;
          el.textContent += s.content;
          if (stick) el.scrollTop = el.scrollHeight;
        }
        offset = s.offset || 0;
      }
      if (s.state === 'failed') {
        $('spin').hidden = true;
        $('detail').hidden = true;
        $('hint').hidden = true;
        $('slow').hidden = true;
        $('err').textContent = s.detail + ' Reload the page to retry.';
        $('err').hidden = false;
        return; // terminal: keep the captured logs on screen
      }
      if (s.detail) $('detail').textContent = s.detail;
      // A node boot is 2-4 minutes; past that, say why it might be stuck.
      if (s.state === 'waking' && Date.now() - t0 > 240000) $('slow').hidden = false;
    }
    setTimeout(tick, 1000);
  };
  setTimeout(tick, 1000);
})();
</script>
</body></html>
`
