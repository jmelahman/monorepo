package api_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/jmelahman/kanban/internal/db"
)

// project_dir becomes the container's working directory *and* the
// ${localWorkspaceFolder} a repo-supplied devcontainer.json can hand to
// dockerd as a bind source, so an escape is a security boundary, not just a
// bad value. Every write path validates it.
func TestCreateBoard_ProjectDirValidation(t *testing.T) {
	cases := []struct {
		name       string
		body       map[string]any
		wantStatus int
		wantDir    string
		wantErrIn  string
	}{
		{
			name:       "accepted and normalized",
			body:       map[string]any{"repo_path": "/r", "project_dir": "./services/api/"},
			wantStatus: 201,
			wantDir:    "services/api",
		},
		{
			name:       "escaping rejected",
			body:       map[string]any{"repo_path": "/r", "project_dir": "../../etc"},
			wantStatus: 400,
			wantErrIn:  "stay inside",
		},
		{
			name:       "absolute rejected",
			body:       map[string]any{"repo_path": "/r", "project_dir": "/etc"},
			wantStatus: 400,
			wantErrIn:  "relative to the repository root",
		},
		{
			name:       "requires repo_path",
			body:       map[string]any{"mount_path": "/mnt", "project_dir": "services/api"},
			wantStatus: 400,
			wantErrIn:  "requires repo_path",
		},
		{
			name:       "rejects mount_path",
			body:       map[string]any{"repo_path": "/r", "mount_path": "/mnt", "project_dir": "services/api"},
			wantStatus: 400,
			wantErrIn:  "mutually exclusive",
		},
	}

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t)
			body := map[string]any{"name": fmt.Sprintf("Board %d", i)}
			for k, v := range tc.body {
				body[k] = v
			}
			resp := e.post("/api/boards", body)
			assertStatus(t, resp, tc.wantStatus)
			if tc.wantErrIn != "" {
				if got := string(readBody(t, resp)); !strings.Contains(got, tc.wantErrIn) {
					t.Errorf("body = %s; want it to contain %q", got, tc.wantErrIn)
				}
				return
			}
			if got := decodeJSON[db.Board](t, resp); got.ProjectDir != tc.wantDir {
				t.Errorf("project_dir = %q; want %q", got.ProjectDir, tc.wantDir)
			}
		})
	}
}

func TestUpdateBoard_ProjectDirValidation(t *testing.T) {
	t.Run("set and clear", func(t *testing.T) {
		e := newEnv(t)
		b := e.seedBoard("Mono")
		path := fmt.Sprintf("/api/boards/%d", b.ID)

		resp := e.patch(path, map[string]any{"project_dir": "services/api/"})
		assertStatus(t, resp, 200)
		if got := decodeJSON[db.Board](t, resp); got.ProjectDir != "services/api" {
			t.Fatalf("project_dir = %q; want %q", got.ProjectDir, "services/api")
		}

		resp = e.patch(path, map[string]any{"project_dir": ""})
		assertStatus(t, resp, 200)
		if got := decodeJSON[db.Board](t, resp); got.ProjectDir != "" {
			t.Errorf("project_dir = %q; want it cleared", got.ProjectDir)
		}
	})

	t.Run("escaping rejected", func(t *testing.T) {
		e := newEnv(t)
		b := e.seedBoard("Mono")
		resp := e.patch(fmt.Sprintf("/api/boards/%d", b.ID), map[string]any{"project_dir": "a/../.."})
		assertStatus(t, resp, 400)
	})

	t.Run("setting mount_path on a board that already has project_dir is rejected", func(t *testing.T) {
		// The invalid pair arrived at from the other side: validating only
		// the field being written would let this through.
		e := newEnv(t)
		b := e.seedBoard("Mono")
		path := fmt.Sprintf("/api/boards/%d", b.ID)

		assertStatus(t, e.patch(path, map[string]any{"project_dir": "services/api"}), 200)

		resp := e.patch(path, map[string]any{"mount_path": "/mnt"})
		assertStatus(t, resp, 400)
		if got := string(readBody(t, resp)); !strings.Contains(got, "mutually exclusive") {
			t.Errorf("body = %s; want it to contain %q", got, "mutually exclusive")
		}
	})

	t.Run("clearing repo_path on a board with project_dir is rejected", func(t *testing.T) {
		e := newEnv(t)
		b := e.seedBoard("Mono")
		path := fmt.Sprintf("/api/boards/%d", b.ID)

		assertStatus(t, e.patch(path, map[string]any{"project_dir": "services/api"}), 200)
		assertStatus(t, e.patch(path, map[string]any{"repo_path": "", "mount_path": "/mnt"}), 400)
	})
}
