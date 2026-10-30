package git

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jmelahman/git-orchard/internal/gittest"
)

func newRepo(t *testing.T) Repo {
	t.Helper()
	gittest.Isolate(t)
	r := Repo{Dir: t.TempDir()}
	if _, err := r.Output("init", "--quiet"); err != nil {
		t.Fatal(err)
	}
	return r
}

func TestEnv(t *testing.T) {
	r := newRepo(t)
	r.Env = []string{"GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=orchard.test", "GIT_CONFIG_VALUE_0=fromenv"}
	out, err := r.Output("config", "orchard.test")
	if err != nil {
		t.Fatal(err)
	}
	if out != "fromenv" {
		t.Errorf("got %q", out)
	}

	// OutputEnv adds to Env rather than replacing it.
	out, err = r.OutputEnv([]string{"GIT_AUTHOR_NAME=Someone"}, "var", "GIT_AUTHOR_IDENT")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, "Someone ") {
		t.Errorf("got %q", out)
	}
	if out, err = r.OutputEnv([]string{"GIT_AUTHOR_NAME=Someone"}, "config", "orchard.test"); err != nil || out != "fromenv" {
		t.Errorf("got %q, %v", out, err)
	}
}

func TestLog(t *testing.T) {
	r := newRepo(t)
	var log bytes.Buffer
	r.Log = &log
	if err := r.Run("rev-parse", "--is-inside-work-tree"); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(log.String()); got != "true" {
		t.Errorf("got %q", got)
	}
}

func TestCtx(t *testing.T) {
	r := newRepo(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r.Ctx = ctx
	_, err := r.Output("status")
	if !errors.Is(err, context.Canceled) {
		t.Errorf("got %v, want context.Canceled", err)
	}
}
