package git

import (
	"os"
	"testing"

	"github.com/jmelahman/kanban/internal/gittest"
	"github.com/jmelahman/kanban/internal/sshsig"
)

// TestMain isolates git from the caller's environment and gitconfig; see
// gittest.IsolateEnv.
//
// It also answers for kanban as gpg.ssh.program, as main.go does: with signing
// on, run() points git at os.Executable(), which under `go test` is this
// binary.
func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == "-Y" {
		os.Exit(sshsig.Main(os.Args[1:]))
	}
	gittest.IsolateEnv()
	os.Exit(m.Run())
}
