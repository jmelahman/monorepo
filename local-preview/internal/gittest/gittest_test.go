package gittest

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMain(m *testing.M) {
	IsolateEnv()
	os.Exit(m.Run())
}

func TestIsolateEnvDropsWhatHooksInherit(t *testing.T) {
	for _, k := range []string{"GIT_DIR", "GIT_INDEX_FILE", "GIT_PREFIX", "GIT_CONFIG_PARAMETERS"} {
		t.Setenv(k, "from-hook")
	}
	IsolateEnv()
	for _, k := range []string{"GIT_DIR", "GIT_INDEX_FILE", "GIT_PREFIX", "GIT_CONFIG_PARAMETERS"} {
		if v, ok := os.LookupEnv(k); ok {
			t.Errorf("%s = %q, want it unset", k, v)
		}
	}
}

func TestRunCommitsAsTheTestIdentity(t *testing.T) {
	dir := t.TempDir()
	Run(t, dir, "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(dir, "README"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	Run(t, dir, "add", "-A")
	Run(t, dir, "commit", "-qm", "initial")
	if got, want := Run(t, dir, "log", "-1", "--format=%an <%ae>"), "test <test@example.com>"; got != want {
		t.Fatalf("author = %q, want %q", got, want)
	}
}
