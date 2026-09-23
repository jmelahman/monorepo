package api_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeKanbanToml drops a .kanban.toml into the env's repo so the per-board
// merge/sync config resolves from it.
func writeKanbanToml(t *testing.T, e *testEnv, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(e.repoPath, ".kanban.toml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func errorBody(t *testing.T, e *testEnv, path string, body any, wantCode int) string {
	t.Helper()
	resp := e.post(path, body)
	assertStatus(t, resp, wantCode)
	return decodeJSON[map[string]string](t, resp)["error"]
}

// A strategy the board disables must not be advertised by the validation
// error: naming it costs the caller a round trip to discover it's off.
func TestMergeStrategy_ErrorNamesOnlyEnabledStrategies(t *testing.T) {
	redirectUserConfig(t)
	e := newEnv(t)
	writeKanbanToml(t, e, "[merge]\nallow_merge_commit = false\nallow_rebase = false\n")
	tk := e.seedTicket(e.seedBoard("Merge Cfg"), "T")
	path := fmt.Sprintf("/api/tickets/%d/merge", tk.ID)

	got := errorBody(t, e, path, map[string]any{"strategy": "rebase"}, 400)
	if !strings.Contains(got, "disabled for this board") || !strings.Contains(got, "enabled: squash") {
		t.Fatalf("disabled strategy error = %q, want it to name the enabled set", got)
	}

	// "merge" is a *sync* strategy; the merge equivalent is "merge-commit".
	// The error must list what this board takes, not the static set.
	got = errorBody(t, e, path, map[string]any{"strategy": "merge"}, 400)
	if got != "strategy must be squash" {
		t.Fatalf("unknown strategy error = %q, want %q", got, "strategy must be squash")
	}

	// An enabled strategy clears validation and fails later, on the missing
	// session — proof the gate above rejected on config, not on spelling.
	got = errorBody(t, e, path, map[string]any{"strategy": "squash"}, 404)
	if got != "no session for ticket" {
		t.Fatalf("enabled strategy error = %q, want it to reach the session lookup", got)
	}
}

// Sync defaults to rebase, but a board that disables rebase leaves exactly
// one way to sync — take it rather than failing on a default nobody chose.
func TestSyncStrategy_DisabledDefaultFallsBackToTheOnlyOption(t *testing.T) {
	redirectUserConfig(t)
	e := newEnv(t)
	writeKanbanToml(t, e, "[sync]\nallow_rebase = false\n")
	tk := e.seedTicket(e.seedBoard("Sync Cfg"), "T")

	// Reaching the session lookup means a strategy was resolved.
	got := errorBody(t, e, fmt.Sprintf("/api/tickets/%d/sync", tk.ID), map[string]any{}, 404)
	if got != "no session for ticket" {
		t.Fatalf("omitted strategy error = %q, want it to resolve to merge", got)
	}
}

// merge.default_strategy supplies the strategy when the request omits one.
func TestMergeStrategy_ConfiguredDefault(t *testing.T) {
	redirectUserConfig(t)
	e := newEnv(t)
	writeKanbanToml(t, e, "[merge]\ndefault_strategy = \"squash\"\n")
	tk := e.seedTicket(e.seedBoard("Default Cfg"), "T")

	got := errorBody(t, e, fmt.Sprintf("/api/tickets/%d/merge", tk.ID), map[string]any{}, 404)
	if got != "no session for ticket" {
		t.Fatalf("omitted strategy error = %q, want the configured default to apply", got)
	}
}

// With every strategy enabled and no default configured, an omitted strategy
// is genuinely ambiguous — say so, and list the choices.
func TestMergeStrategy_RequiredWhenAmbiguous(t *testing.T) {
	redirectUserConfig(t)
	e := newEnv(t)
	tk := e.seedTicket(e.seedBoard("No Default"), "T")

	got := errorBody(t, e, fmt.Sprintf("/api/tickets/%d/merge", tk.ID), map[string]any{}, 400)
	if got != "strategy is required; enabled: merge-commit, squash, or rebase" {
		t.Fatalf("ambiguous omitted strategy error = %q", got)
	}
}

// A default the same config disables, with more than one strategy left, is a
// misconfiguration the caller can't fix by retrying — name the key's value.
func TestMergeStrategy_DisabledDefaultIsReported(t *testing.T) {
	redirectUserConfig(t)
	e := newEnv(t)
	writeKanbanToml(t, e, "[merge]\nallow_rebase = false\ndefault_strategy = \"rebase\"\n")
	tk := e.seedTicket(e.seedBoard("Bad Default"), "T")

	got := errorBody(t, e, fmt.Sprintf("/api/tickets/%d/merge", tk.ID), map[string]any{}, 400)
	if got != "default strategy rebase is disabled for this board; enabled: merge-commit or squash" {
		t.Fatalf("disabled default error = %q", got)
	}
}

// A board that enables exactly one strategy leaves nothing to choose.
func TestMergeStrategy_SingleEnabledIsImplied(t *testing.T) {
	redirectUserConfig(t)
	e := newEnv(t)
	writeKanbanToml(t, e, "[merge]\nallow_merge_commit = false\nallow_rebase = false\n")
	tk := e.seedTicket(e.seedBoard("Squash Only"), "T")

	got := errorBody(t, e, fmt.Sprintf("/api/tickets/%d/merge", tk.ID), map[string]any{}, 404)
	if got != "no session for ticket" {
		t.Fatalf("omitted strategy error = %q, want squash to be implied", got)
	}
}

func TestMergeStrategy_AllDisabled(t *testing.T) {
	redirectUserConfig(t)
	e := newEnv(t)
	writeKanbanToml(t, e, "[merge]\nallow_merge_commit = false\nallow_squash = false\nallow_rebase = false\n")
	tk := e.seedTicket(e.seedBoard("No Merge"), "T")

	got := errorBody(t, e, fmt.Sprintf("/api/tickets/%d/merge", tk.ID), map[string]any{"strategy": "squash"}, 400)
	if got != "every strategy is disabled for this board" {
		t.Fatalf("all-disabled error = %q", got)
	}
}
