package server

import (
	"os"
	"testing"

	"github.com/jmelahman/local-preview/internal/gittest"
)

// TestMain isolates the git environment; see gittest.IsolateEnv.
func TestMain(m *testing.M) {
	gittest.IsolateEnv()
	os.Exit(m.Run())
}
