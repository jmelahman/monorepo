package gittest

import (
	"os"
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
