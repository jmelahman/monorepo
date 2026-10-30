package api

import (
	"errors"
	"fmt"
	"mime"
	"net/http"
	"strings"

	"github.com/jmelahman/kanban/internal/db"
	"github.com/jmelahman/kanban/internal/releases"
)

// maxReleaseRunBody bounds a release request, which is a few short strings.
const maxReleaseRunBody = 1 << 14

// releaseErrorCode maps a releases error to its status.
func releaseErrorCode(err error) int {
	switch {
	case errors.Is(err, db.ErrNotFound), errors.Is(err, releases.ErrUnknown):
		return http.StatusNotFound
	case errors.Is(err, releases.ErrInvalid):
		return http.StatusBadRequest
	case errors.Is(err, releases.ErrConflict):
		return http.StatusConflict
	}
	return http.StatusInternalServerError
}

// releasesReady writes an error if the service wasn't wired up.
func (h *handlers) releasesReady(w http.ResponseWriter) bool {
	if h.releases == nil {
		h.httpError(w, errors.New("releases are unavailable"), http.StatusServiceUnavailable)
		return false
	}
	return true
}

func (h *handlers) getReleases(w http.ResponseWriter, r *http.Request) {
	if !h.releasesReady(w) {
		return
	}
	sum, err := h.releases.Summary(r.Context())
	if err != nil {
		h.httpError(w, err, 500)
		return
	}
	writeJSON(w, 200, sum)
}

func (h *handlers) refreshReleases(w http.ResponseWriter, r *http.Request) {
	if !h.releasesReady(w) {
		return
	}
	if err := h.releases.Refresh(r.Context()); err != nil {
		h.httpError(w, err, 500)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

type releaseReminderReq struct {
	RepoPath string `json:"repo_path"`
	Prefix   string `json:"prefix"`
	Bump     string `json:"bump"`
	Suffix   string `json:"suffix"`
	Note     string `json:"note"`
}

func (h *handlers) createReleaseReminder(w http.ResponseWriter, r *http.Request) {
	if !h.releasesReady(w) {
		return
	}
	req, err := decodeBody[releaseReminderReq](r)
	if err != nil {
		h.httpError(w, err, 400)
		return
	}
	reminder := &db.ReleaseReminder{RepoPath: req.RepoPath, Prefix: req.Prefix, Bump: req.Bump, Suffix: req.Suffix, Note: req.Note}
	if err := h.releases.AddReminder(r.Context(), reminder); err != nil {
		h.httpError(w, err, releaseErrorCode(err))
		return
	}
	writeJSON(w, 201, reminder)
}

type patchReleaseReminderReq struct {
	Bump   *string `json:"bump"`
	Suffix *string `json:"suffix"`
	Note   *string `json:"note"`
}

func (h *handlers) patchReleaseReminder(w http.ResponseWriter, r *http.Request) {
	if !h.releasesReady(w) {
		return
	}
	req, err := decodeBody[patchReleaseReminderReq](r)
	if err != nil {
		h.httpError(w, err, 400)
		return
	}
	reminder, err := h.store.GetReleaseReminder(r.Context(), pathID(r))
	if err != nil {
		h.httpError(w, err, releaseErrorCode(err))
		return
	}
	if req.Bump != nil {
		reminder.Bump = *req.Bump
	}
	if req.Suffix != nil {
		reminder.Suffix = *req.Suffix
	}
	if req.Note != nil {
		reminder.Note = *req.Note
	}
	if err := h.releases.UpdateReminder(r.Context(), reminder); err != nil {
		h.httpError(w, err, releaseErrorCode(err))
		return
	}
	writeJSON(w, 200, reminder)
}

func (h *handlers) deleteReleaseReminder(w http.ResponseWriter, r *http.Request) {
	if !h.releasesReady(w) {
		return
	}
	if err := h.releases.DismissReminder(r.Context(), pathID(r)); err != nil {
		h.httpError(w, err, releaseErrorCode(err))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *handlers) previewRelease(w http.ResponseWriter, r *http.Request) {
	if !h.releasesReady(w) {
		return
	}
	q := r.URL.Query()
	p, err := h.releases.Preview(r.Context(), q.Get("repo"), q.Get("prefix"), q.Get("bump"), q.Get("suffix"))
	if err != nil {
		h.httpError(w, err, releaseErrorCode(err))
		return
	}
	writeJSON(w, 200, p)
}

type runReleaseReq struct {
	Repo       string `json:"repo"`
	Prefix     string `json:"prefix"`
	Bump       string `json:"bump"`
	Suffix     string `json:"suffix"`
	Version    string `json:"version"`
	Rev        string `json:"rev"`
	ReminderID *int64 `json:"reminder_id"`
}

// publishRequestError says why a request that publishes must be refused. The
// API has no auth, so a page on another site could otherwise post here from
// the browser of whoever is running it.
func publishRequestError(r *http.Request) error {
	if strings.EqualFold(r.Header.Get("Sec-Fetch-Site"), "cross-site") {
		return errors.New("cross-site requests can't release")
	}
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		return errors.New("Content-Type must be application/json")
	}
	return nil
}

func (h *handlers) runRelease(w http.ResponseWriter, r *http.Request) {
	if !h.releasesReady(w) {
		return
	}
	if err := publishRequestError(r); err != nil {
		h.httpError(w, err, http.StatusForbidden)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxReleaseRunBody)
	req, err := decodeBody[runReleaseReq](r)
	if err != nil {
		h.httpError(w, err, 400)
		return
	}
	run, err := h.releases.Run(r.Context(), releases.RunRequest{
		RepoPath: req.Repo, Prefix: req.Prefix, Bump: req.Bump, Suffix: req.Suffix,
		Version: req.Version, Rev: req.Rev, ReminderID: req.ReminderID,
	})
	if err != nil {
		h.httpError(w, err, releaseErrorCode(err))
		return
	}
	writeJSON(w, 202, run)
}

func (h *handlers) releaseRunOutput(w http.ResponseWriter, r *http.Request) {
	if !h.releasesReady(w) {
		return
	}
	id := pathID(r)
	if _, err := h.store.GetReleaseRun(r.Context(), id); err != nil {
		h.httpError(w, err, releaseErrorCode(err))
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		h.httpError(w, errors.New("streaming unsupported"), 500)
		return
	}
	writeSSEHeaders(w)
	flusher.Flush()
	err := h.releases.Follow(r.Context(), id, func(line string) {
		fmt.Fprintf(w, "data: %s\n\n", line)
		flusher.Flush()
	})
	if err != nil {
		fmt.Fprintf(w, "data: reading the log: %v\n\n", err)
	}
	if r.Context().Err() == nil {
		fmt.Fprintf(w, "event: end\ndata: {}\n\n")
		flusher.Flush()
	}
}
