package api

import (
	"context"
	"fmt"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/jmelahman/local-preview/internal/db"
	"github.com/jmelahman/local-preview/internal/execstream"
	"github.com/jmelahman/local-preview/internal/supervise"
)

// fakeRuntime satisfies RuntimeView with programmable status and exec.
type fakeRuntime struct {
	status string
	execFn func(k supervise.Key, opts supervise.ExecOptions, stream io.ReadWriter) error
	gotKey supervise.Key
}

func (f *fakeRuntime) Status(supervise.Key) string { return f.status }
func (f *fakeRuntime) LastFailure(supervise.Key) (supervise.Failure, bool) {
	return supervise.Failure{}, false
}
func (f *fakeRuntime) CrashedKeys() []supervise.Key { return nil }
func (f *fakeRuntime) Stats(context.Context, supervise.Key) *supervise.ProcessStats {
	return nil
}
func (f *fakeRuntime) RunLog(string, string, string, int, int64) (supervise.RunLog, error) {
	return supervise.RunLog{}, nil
}
func (f *fakeRuntime) Stop(supervise.Key, string) {}
func (f *fakeRuntime) StopRepo(int64, string)     {}
func (f *fakeRuntime) Exec(_ context.Context, k supervise.Key, opts supervise.ExecOptions, stream io.ReadWriter) error {
	f.gotKey = k
	if f.execFn != nil {
		return f.execFn(k, opts, stream)
	}
	return nil
}

// readyDeploy creates a repo and a ready deploy with a backend hash.
func readyDeploy(t *testing.T, deps Deps) db.Deploy {
	t.Helper()
	r, err := deps.Store.CreateRepo("demo", "/src/demo", "/data/repos/demo.git", "ready")
	if err != nil {
		t.Fatal(err)
	}
	dep, err := deps.Store.CreateDeploy(r.ID, "0123456789abcdef0123456789abcdef01234567", db.DeployMeta{})
	if err != nil {
		t.Fatal(err)
	}
	if err := deps.Store.SetDeployHashes(dep.ID, "", "behash1234567890", "", ""); err != nil {
		t.Fatal(err)
	}
	if err := deps.Store.SetDeployReady(dep.ID); err != nil {
		t.Fatal(err)
	}
	return dep
}

// TestExecRejectsBeforeUpgrade: everything checkable answers as a plain HTTP
// error, not a broken socket — missing cmd, unknown deploy, idle process.
func TestExecRejectsBeforeUpgrade(t *testing.T) {
	deps, _ := newTestDeps(t)
	rt := &fakeRuntime{status: supervise.StatusIdle}
	deps.Runtime = rt
	mux := NewMux(deps)
	dep := readyDeploy(t, deps)

	if rec := doJSON(t, mux, "GET", fmt.Sprintf("/api/deploys/%d/exec", dep.ID), ""); rec.Code != 400 {
		t.Fatalf("missing cmd = %d, want 400: %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, mux, "GET", "/api/deploys/999/exec?cmd=sh", ""); rec.Code != 404 {
		t.Fatalf("unknown deploy = %d, want 404", rec.Code)
	}
	if rec := doJSON(t, mux, "GET", fmt.Sprintf("/api/deploys/%d/exec?cmd=sh", dep.ID), ""); rec.Code != 409 {
		t.Fatalf("idle process = %d, want 409: %s", rec.Code, rec.Body.String())
	}
}

// TestExecUpgradeRoundTrip: with a running process, the endpoint upgrades to
// a WebSocket, hands the session to the runtime view with the right key and
// options, and the frames flow back to the caller.
func TestExecUpgradeRoundTrip(t *testing.T) {
	deps, _ := newTestDeps(t)
	var gotOpts supervise.ExecOptions
	done := make(chan struct{})
	rt := &fakeRuntime{status: supervise.StatusRunning}
	rt.execFn = func(_ supervise.Key, opts supervise.ExecOptions, stream io.ReadWriter) error {
		gotOpts = opts
		defer close(done)
		fw := execstream.NewWriter(stream)
		if err := fw.WriteFrame(execstream.FrameStdout, []byte("out")); err != nil {
			return err
		}
		return fw.WriteFrame(execstream.FrameExit, []byte{3})
	}
	deps.Runtime = rt
	dep := readyDeploy(t, deps)

	srv := httptest.NewServer(NewMux(deps))
	defer srv.Close()
	conn, err := execstream.Dial(context.Background(),
		fmt.Sprintf("%s/api/deploys/%d/exec?cmd=sh&cmd=-c&cmd=id&tty=1&stdin=1&term=xterm", srv.URL, dep.ID), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	f, err := execstream.ReadFrame(conn)
	if err != nil || f.Type != execstream.FrameStdout || string(f.Payload) != "out" {
		t.Fatalf("frame = %+v, %v", f, err)
	}
	f, err = execstream.ReadFrame(conn)
	if err != nil || f.Type != execstream.FrameExit || f.Payload[0] != 3 {
		t.Fatalf("frame = %+v, %v", f, err)
	}
	<-done
	want := supervise.BackendKey(dep.RepoID, "behash1234567890")
	if rt.gotKey != want {
		t.Fatalf("runtime saw key %+v, want %+v", rt.gotKey, want)
	}
	if len(gotOpts.Cmd) != 3 || gotOpts.Cmd[0] != "sh" || !gotOpts.TTY || !gotOpts.Stdin || gotOpts.Term != "xterm" {
		t.Fatalf("runtime saw opts %+v", gotOpts)
	}
}
