package api_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// TestContract_NoNullArrays enforces the wire-format invariant: list-typed
// JSON fields are always serialized as `[]`, never `null`. This is the
// regression guard for nil-slice → JSON null bugs that crash `.map()` in the
// frontend. The empty-board case is the one that actually triggers the bug.
func TestContract_NoNullArrays(t *testing.T) {
	e := newEnv(t)

	seededBoard := e.seedBoard("Seeded")
	ticket := e.seedTicket(seededBoard, "T")
	sess := e.seedSession(ticket)
	emptyBoard := e.seedBoard("Empty")

	cases := []struct {
		name  string
		path  string
		check func(t *testing.T, body []byte)
	}{
		{"listBoards", "/api/boards", expectTopLevelArray},
		{"boardState_seeded", fmt.Sprintf("/api/boards/%d/state", seededBoard.ID), expectArrayFields("columns", "tickets", "sessions")},
		{"boardState_empty", fmt.Sprintf("/api/boards/%d/state", emptyBoard.ID), expectArrayFields("columns", "tickets", "sessions")},
		{"discoverTasks", fmt.Sprintf("/api/sessions/%d/discover-tasks", sess.ID), expectTopLevelArray},
		{"listTaskRuns_empty", fmt.Sprintf("/api/sessions/%d/task-runs", sess.ID), expectTopLevelArray},
		{"listPorts_empty", fmt.Sprintf("/api/sessions/%d/ports", sess.ID), expectTopLevelArray},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := e.get(tc.path)
			assertStatus(t, resp, 200)
			tc.check(t, readBody(t, resp))
		})
	}
}

func expectTopLevelArray(t *testing.T, body []byte) {
	t.Helper()
	s := strings.TrimSpace(string(body))
	if !strings.HasPrefix(s, "[") {
		t.Fatalf("body = %s; want top-level array", s)
	}
}

func expectArrayFields(fields ...string) func(*testing.T, []byte) {
	return func(t *testing.T, body []byte) {
		t.Helper()
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(body, &raw); err != nil {
			t.Fatalf("not a json object: %v\nbody: %s", err, body)
		}
		for _, f := range fields {
			v, ok := raw[f]
			if !ok {
				t.Errorf("missing field %q in %s", f, body)
				continue
			}
			s := strings.TrimSpace(string(v))
			if !strings.HasPrefix(s, "[") {
				t.Errorf("field %q = %s; want array (got null or scalar)", f, s)
			}
		}
	}
}
