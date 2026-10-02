package github

import (
	"os"
	"testing"

	"github.com/jmelahman/kanban/internal/gittest"
)

// TestMain isolates git from the caller's environment and gitconfig; see
// gittest.IsolateEnv.
func TestMain(m *testing.M) {
	gittest.IsolateEnv()
	os.Exit(m.Run())
}
