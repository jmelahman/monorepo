package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/jmelahman/kanban/internal/git"
	"github.com/jmelahman/kanban/internal/harness"
	"github.com/jmelahman/kanban/internal/kanbantoml"
)

// Config — a generic read/write surface over the layered kanban config
// (project .kanban.toml + user config.toml). Mirrors `git config`: --global
// targets the user file, --local targets a board's project file. The narrower
// /api/settings endpoint stays for the web Settings UI; both write through
// kanbantoml.EditFile so they share its per-file lock and atomic write.

type configEntry struct {
	Key      string `json:"key"`
	Value    any    `json:"value"`
	Source   string `json:"source"` // local | global | default | runtime
	Writable bool   `json:"writable"`
}

type configResp struct {
	Scope   string        `json:"scope"`
	Board   string        `json:"board,omitempty"`
	Entries []configEntry `json:"entries"`
}

type configPatchReq struct {
	Scope string                     `json:"scope"`
	Board string                     `json:"board"`
	Set   map[string]json.RawMessage `json:"set"`
	Unset []string                   `json:"unset"`
}

func (h *handlers) getConfig(w http.ResponseWriter, r *http.Request) {
	scope := r.URL.Query().Get("scope")
	if scope == "" {
		scope = "effective"
	}
	board := r.URL.Query().Get("board")
	resp, status, err := h.configView(r.Context(), scope, board)
	if err != nil {
		h.httpError(w, err, status)
		return
	}
	writeJSON(w, 200, resp)
}

func (h *handlers) patchConfig(w http.ResponseWriter, r *http.Request) {
	req, err := decodeBody[configPatchReq](r)
	if err != nil {
		h.httpError(w, err, 400)
		return
	}

	path, status, err := h.configTargetPath(r.Context(), req.Scope, req.Board)
	if err != nil {
		h.httpError(w, err, status)
		return
	}

	// Validate + normalize every set value before touching the file so a bad
	// key in the batch doesn't leave a partial write.
	sets := make(map[string]any, len(req.Set))
	signTouched := false
	for key, raw := range req.Set {
		v, err := kanbantoml.NormalizeValue(key, raw)
		if err != nil {
			h.httpError(w, err, 400)
			return
		}
		if status, err := h.guardKey(key, v); err != nil {
			h.httpError(w, err, status)
			return
		}
		if key == "git.sign_commits" {
			signTouched = true
		}
		sets[key] = v
	}
	for _, key := range req.Unset {
		if _, _, ok := kanbantoml.Lookup(key); !ok {
			h.httpError(w, fmt.Errorf("unknown config key %q", key), 400)
			return
		}
		if status, err := h.guardKey(key, nil); err != nil {
			h.httpError(w, err, status)
			return
		}
		if key == "git.sign_commits" {
			signTouched = true
		}
	}

	if err := kanbantoml.EditFile(path, sets, req.Unset); err != nil {
		h.httpError(w, err, 500)
		return
	}

	// git.sign_commits is a process-wide runtime toggle the server reads from
	// the user layer (kanbantoml.Load("")); refresh it after a global edit so
	// the change takes effect without a restart, mirroring updateSettings.
	if signTouched && req.Scope == "global" {
		git.SetCommitSigning(readUserSignCommits())
	}

	resp, status, err := h.configView(r.Context(), req.Scope, req.Board)
	if err != nil {
		h.httpError(w, err, status)
		return
	}
	writeJSON(w, 200, resp)
}

// guardKey applies the runtime-only validations that can't live in the
// kanbantoml schema (they depend on server state or other packages). It runs
// for both set (v is the normalized value) and unset (v is nil).
func (h *handlers) guardKey(key string, v any) (int, error) {
	switch key {
	case "harness.id":
		if id, ok := v.(string); ok && id != "" && !harness.IsKnown(id) {
			return 400, fmt.Errorf("unknown harness %q", id)
		}
	case "worktrees.root":
		if h.config.HasWorktreesDirOverride() {
			return http.StatusConflict, fmt.Errorf("worktrees root is locked by --worktrees-dir or $KANBAN_WORKTREES_DIR")
		}
	}
	return 0, nil
}

// configTargetPath resolves the file a write targets. global -> the user
// config; local -> the chosen board's project .kanban.toml. Returns an HTTP
// status alongside the error for the handler to surface.
func (h *handlers) configTargetPath(ctx context.Context, scope, boardIdent string) (string, int, error) {
	switch scope {
	case "global":
		path, err := kanbantoml.UserPath()
		if err != nil {
			return "", 500, err
		}
		return path, 0, nil
	case "local":
		repoPath, status, err := h.localRepoPath(ctx, boardIdent)
		if err != nil {
			return "", status, err
		}
		return kanbantoml.ProjectPath(repoPath), 0, nil
	default:
		return "", 400, fmt.Errorf("scope must be %q or %q", "local", "global")
	}
}

// localRepoPath resolves a board identifier to the repo directory whose
// .kanban.toml local config lives in, rejecting boards that have no usable
// repo on disk.
func (h *handlers) localRepoPath(ctx context.Context, boardIdent string) (string, int, error) {
	if boardIdent == "" {
		return "", 400, fmt.Errorf("board is required for local scope")
	}
	board, err := h.resolveBoardIdent(ctx, boardIdent)
	if err != nil {
		return "", 404, fmt.Errorf("board %q not found", boardIdent)
	}
	if board.RepoPath == "" {
		return "", http.StatusUnprocessableEntity, fmt.Errorf("board %q has no repo_path; local config requires a git repository", boardIdent)
	}
	if info, err := os.Stat(board.RepoPath); err != nil || !info.IsDir() {
		return "", http.StatusUnprocessableEntity, fmt.Errorf("board repo path %q does not exist", board.RepoPath)
	}
	return board.RepoPath, 0, nil
}

// configView builds the response for a scope. effective merges the layers and
// annotates each key's source plus read-only runtime values; local/global show
// only the keys actually set in that single file.
func (h *handlers) configView(ctx context.Context, scope, boardIdent string) (configResp, int, error) {
	switch scope {
	case "effective":
		repoPath := ""
		if boardIdent != "" {
			// A board without a usable repo just contributes no local layer.
			if rp, _, err := h.localRepoPath(ctx, boardIdent); err == nil {
				repoPath = rp
			}
		}
		project, user := kanbantoml.LoadLayers(repoPath)
		merged := kanbantoml.Load(repoPath)
		return configResp{Scope: scope, Board: boardIdent, Entries: h.effectiveEntries(merged, project, user)}, 0, nil
	case "global":
		_, user := kanbantoml.LoadLayers("")
		return configResp{Scope: scope, Entries: layerEntries(user, "global")}, 0, nil
	case "local":
		repoPath, status, err := h.localRepoPath(ctx, boardIdent)
		if err != nil {
			return configResp{}, status, err
		}
		project, _ := kanbantoml.LoadLayers(repoPath)
		return configResp{Scope: scope, Board: boardIdent, Entries: layerEntries(project, "local")}, 0, nil
	default:
		return configResp{}, 400, fmt.Errorf("scope must be one of %q, %q, %q", "effective", "local", "global")
	}
}

// effectiveEntries lists every schema key (set or not) with its merged value
// and originating layer, followed by the read-only runtime settings.
func (h *handlers) effectiveEntries(merged, project, user kanbantoml.File) []configEntry {
	entries := make([]configEntry, 0, len(kanbantoml.Schema())+4)
	for _, spec := range kanbantoml.Schema() {
		val, set := kanbantoml.GetValue(merged, spec.Key)
		source := "default"
		if set {
			if _, inUser := kanbantoml.GetValue(user, spec.Key); inUser {
				source = "global"
			} else {
				source = "local"
			}
		} else {
			val = nil
		}
		entries = append(entries, configEntry{Key: spec.Key, Value: val, Source: source, Writable: true})
	}
	entries = append(entries,
		configEntry{Key: "data_dir", Value: h.config.DataDir, Source: "runtime", Writable: false},
		configEntry{Key: "worktrees_dir", Value: h.config.WorktreesDir(), Source: "runtime", Writable: false},
		configEntry{Key: "port_range_start", Value: h.config.PortRangeStart, Source: "runtime", Writable: false},
		configEntry{Key: "port_range_end", Value: h.config.PortRangeEnd, Source: "runtime", Writable: false},
	)
	return entries
}

// layerEntries lists only the keys actually set in a single config file.
func layerEntries(f kanbantoml.File, source string) []configEntry {
	entries := make([]configEntry, 0)
	for _, spec := range kanbantoml.Schema() {
		if val, set := kanbantoml.GetValue(f, spec.Key); set {
			entries = append(entries, configEntry{Key: spec.Key, Value: val, Source: source, Writable: true})
		}
	}
	return entries
}
