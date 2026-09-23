// Package errreport routes captured backend panics, HTTP 5xx errors, and
// frontend exceptions into kanban tickets on a dedicated "Errors" board.
//
// It is opt-in: the reporter no-ops unless Config.Enabled is true (set via
// `[errors] enabled = true` in .kanban.toml). A nil receiver is also a valid
// no-op so call sites can wire the reporter unconditionally without nil
// checks.
//
// Dedup: a sha256 fingerprint over (source, title, top stack frames) is
// stored on each ticket. Subsequent captures with the same fingerprint update
// the existing open ticket (appending a "Seen again at <time>" line, bumping
// the occurrence count in the title, and moving it back to the "New" column
// so the recurrence is visible) instead of creating a new one. Only archived
// tickets free the fingerprint — non-archived "Resolved" tickets still match,
// since unresolving a still-recurring error would be misleading.
package errreport

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/jmelahman/kanban/internal/db"
	"github.com/jmelahman/kanban/internal/slug"
)

// Config controls reporter behavior. Enabled gates all ticket creation;
// BoardName is the display name of the dedicated errors board (defaults to
// "Errors"). Resolve from .kanban.toml via ResolveConfig.
type Config struct {
	Enabled   bool
	BoardName string
}

// Reporter funnels captured errors into the errors board. Safe for
// concurrent use; a single instance lives for the process lifetime.
type Reporter struct {
	store *db.Store
	cfg   Config

	mu        sync.Mutex
	boardID   int64 // 0 until ensureBoard succeeds; cached afterwards
	newColID  int64 // first column ("New") on the errors board
	reporting bool  // re-entrancy guard: blocks recursion if Report's own writes log errors
}

// New constructs a Reporter. cfg.BoardName falls back to "Errors" when empty.
func New(store *db.Store, cfg Config) *Reporter {
	if cfg.BoardName == "" {
		cfg.BoardName = "Errors"
	}
	return &Reporter{store: store, cfg: cfg}
}

// Report files (or updates) a ticket for the given error.
//
// source identifies the capture site ("panic", "http", "boundary",
// "react-query", "frontend"); title is a one-line summary used as the ticket
// title; stack is a multi-line stack trace (may be empty); meta is an
// optional bag of context (request path, URL, user agent, …) rendered into
// the ticket body.
//
// Never propagates errors and never panics: a nil receiver, a disabled
// reporter, a re-entrant call, or a downstream DB failure all return
// silently after best-effort logging.
func (r *Reporter) Report(ctx context.Context, source, title, stack string, meta map[string]string) {
	if r == nil || !r.cfg.Enabled {
		return
	}
	r.mu.Lock()
	if r.reporting {
		r.mu.Unlock()
		return
	}
	r.reporting = true
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		r.reporting = false
		r.mu.Unlock()
		if rec := recover(); rec != nil {
			log.Printf("errreport: panic during Report: %v", rec)
		}
	}()

	boardID, newColID, err := r.ensureBoard(ctx)
	if err != nil {
		log.Printf("errreport: ensure errors board: %v", err)
		return
	}

	fp := computeFingerprint(source, title, stack)
	if existing, err := r.store.FindOpenTicketByFingerprint(ctx, boardID, fp); err == nil && existing != nil {
		occurrences := strings.Count(existing.Body, "Seen again at") + 2
		existing.Title = titleWithCount(stripOccurrenceCount(existing.Title), occurrences)
		existing.Body = appendOccurrence(existing.Body, time.Now().UTC())
		if err := r.store.UpdateTicket(ctx, existing); err != nil {
			log.Printf("errreport: update existing ticket %d: %v", existing.ID, err)
		}
		// Bump the ticket back to the top of "New" so a recurrence is visible
		// in the UI even if the user previously moved it to Investigating /
		// Resolved. A no-op when the ticket is already at (newCol, 0).
		if err := r.store.MoveTicket(ctx, existing.ID, newColID, 0); err != nil {
			log.Printf("errreport: bump existing ticket %d: %v", existing.ID, err)
		}
		return
	}

	t := &db.Ticket{
		BoardID:     boardID,
		ColumnID:    newColID,
		Title:       truncateTitle(title),
		Slug:        fingerprintSlug(fp),
		Body:        formatBody(source, stack, meta),
		Fingerprint: fp,
	}
	if err := r.store.CreateTicket(ctx, t); err != nil {
		log.Printf("errreport: create ticket: %v", err)
	}
}

// Capture grabs the current goroutine's stack and reports err. Convenience
// wrapper for instrumented call sites (handlers.reportError, …).
func (r *Reporter) Capture(ctx context.Context, source string, err error) {
	if r == nil || !r.cfg.Enabled || err == nil {
		return
	}
	r.Report(ctx, source, err.Error(), captureStack(2), nil)
}

// ensureBoard returns the cached errors-board id and "New" column id,
// creating both lazily on first call. Holds r.mu so concurrent first calls
// don't race to create duplicate boards.
func (r *Reporter) ensureBoard(ctx context.Context) (int64, int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.boardID != 0 && r.newColID != 0 {
		return r.boardID, r.newColID, nil
	}

	boardSlug := slug.Make(r.cfg.BoardName, "errors")
	board, err := r.store.GetBoardBySlug(ctx, boardSlug)
	if errors.Is(err, db.ErrNotFound) {
		board = &db.Board{
			Name:       r.cfg.BoardName,
			Slug:       boardSlug,
			BaseBranch: "main",
		}
		if err := r.store.CreateBoardRaw(ctx, board); err != nil {
			return 0, 0, fmt.Errorf("create board: %w", err)
		}
		columns := []db.Column{
			{BoardID: board.ID, Name: "New", Position: 0},
			{BoardID: board.ID, Name: "Investigating", Position: 1},
			{BoardID: board.ID, Name: "Resolved", Position: 2},
		}
		for i := range columns {
			if err := r.store.CreateColumn(ctx, &columns[i]); err != nil {
				return 0, 0, fmt.Errorf("create column %s: %w", columns[i].Name, err)
			}
		}
		r.boardID = board.ID
		r.newColID = columns[0].ID
		return r.boardID, r.newColID, nil
	}
	if err != nil {
		return 0, 0, fmt.Errorf("lookup board: %w", err)
	}

	cols, err := r.store.ListColumns(ctx, board.ID)
	if err != nil {
		return 0, 0, fmt.Errorf("list columns: %w", err)
	}
	if len(cols) == 0 {
		return 0, 0, fmt.Errorf("errors board %q has no columns", r.cfg.BoardName)
	}
	r.boardID = board.ID
	r.newColID = cols[0].ID
	return r.boardID, r.newColID, nil
}

// computeFingerprint hashes source + title + the first 3 non-runtime stack
// frames into a hex-encoded sha256 prefix. Stable across processes, so a
// recurring error always maps to the same fingerprint regardless of restarts.
func computeFingerprint(source, title, stack string) string {
	h := sha256.New()
	h.Write([]byte(source))
	h.Write([]byte{':'})
	h.Write([]byte(title))
	h.Write([]byte{':'})
	for _, f := range topStackFrames(stack, 3) {
		h.Write([]byte(f))
		h.Write([]byte{'\n'})
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// topStackFrames extracts up to n source-location lines from a debug.Stack()
// or runtime.CallersFrames-rendered trace, skipping goroutine headers and
// runtime/* frames so the fingerprint stays stable when wrappers change.
func topStackFrames(stack string, n int) []string {
	if stack == "" {
		return nil
	}
	out := make([]string, 0, n)
	lines := strings.Split(stack, "\n")
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if ln == "" || strings.HasPrefix(ln, "goroutine ") {
			continue
		}
		// Function-name lines from debug.Stack() don't start with whitespace
		// in our trimmed form; the file:line lines do. We only care about
		// file:line lines (more stable than function names which include
		// closure suffixes).
		if !strings.Contains(ln, ":") {
			continue
		}
		// Skip runtime frames so a panic's reflect/runtime epilogue doesn't
		// drown out the user code at the top.
		if strings.Contains(ln, "/runtime/") || strings.HasPrefix(ln, "runtime.") {
			continue
		}
		out = append(out, ln)
		if len(out) >= n {
			break
		}
	}
	return out
}

// captureStack returns the current goroutine's call stack as a multi-line
// "func\n\tfile:line" string, skipping the requested number of frames plus
// captureStack itself. Mirrors runtime/debug's format closely enough for
// fingerprinting and ticket-body display.
func captureStack(skip int) string {
	pcs := make([]uintptr, 32)
	n := runtime.Callers(skip+1, pcs)
	if n == 0 {
		return ""
	}
	frames := runtime.CallersFrames(pcs[:n])
	var b strings.Builder
	for {
		f, more := frames.Next()
		fmt.Fprintf(&b, "%s\n\t%s:%d\n", f.Function, f.File, f.Line)
		if !more {
			break
		}
	}
	return b.String()
}

func truncateTitle(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 128 {
		return s[:125] + "..."
	}
	if s == "" {
		return "(unknown error)"
	}
	return s
}

// occurrenceSuffixRE matches a trailing " (×N)" recurrence counter so dedup
// can rewrite it on each new occurrence without the count compounding.
var occurrenceSuffixRE = regexp.MustCompile(` \(×\d+\)$`)

func stripOccurrenceCount(title string) string {
	return occurrenceSuffixRE.ReplaceAllString(title, "")
}

func titleWithCount(baseTitle string, count int) string {
	if count <= 1 {
		return baseTitle
	}
	return fmt.Sprintf("%s (×%d)", baseTitle, count)
}

func formatBody(source, stack string, meta map[string]string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "**source:** `%s`\n", source)
	fmt.Fprintf(&b, "**first seen:** %s\n", time.Now().UTC().Format(time.RFC3339))
	if len(meta) > 0 {
		b.WriteString("\n")
		for k, v := range meta {
			if v == "" {
				continue
			}
			fmt.Fprintf(&b, "- **%s:** %s\n", k, v)
		}
	}
	if stack != "" {
		b.WriteString("\n```\n")
		b.WriteString(strings.TrimRight(stack, "\n"))
		b.WriteString("\n```\n")
	}
	return b.String()
}

func appendOccurrence(body string, t time.Time) string {
	suffix := fmt.Sprintf("\n\n---\nSeen again at %s", t.Format(time.RFC3339))
	return body + suffix
}

func fingerprintSlug(fp string) string {
	if len(fp) > 8 {
		fp = fp[:8]
	}
	return "err-" + fp
}
