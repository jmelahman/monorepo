package api_test

import (
	"os"
	"testing"

	"github.com/jmelahman/kanban/internal/kanbantest"
)

// TestMain isolates git, the kanban user config and docker from the caller's
// machine; see kanbantest.IsolateEnv.
func TestMain(m *testing.M) {
	kanbantest.IsolateEnv()
	os.Exit(m.Run())
}
