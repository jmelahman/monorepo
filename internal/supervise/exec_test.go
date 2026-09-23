package supervise

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

type nopStream struct{ bytes.Buffer }

// TestExecRequiresRunningProcess: exec against an idle key fails with the
// status in the message and writes nothing to the stream (the caller turns
// the error into FrameError itself).
func TestExecRequiresRunningProcess(t *testing.T) {
	f := newFixture(t)
	var stream nopStream
	err := f.m.Exec(context.Background(), BackendKey(f.repoID, "h"), ExecOptions{Cmd: []string{"sh"}}, &stream)
	if err == nil || !strings.Contains(err.Error(), `status "idle"`) {
		t.Fatalf("err = %v, want a not-running error naming the idle status", err)
	}
	if stream.Len() != 0 {
		t.Fatalf("stream got %d bytes; orchestration errors must not write frames", stream.Len())
	}
}

// TestExecRejectsEmptyCommand: no argv is an error before any docker work.
func TestExecRejectsEmptyCommand(t *testing.T) {
	f := newFixture(t)
	var stream nopStream
	if err := f.m.Exec(context.Background(), BackendKey(f.repoID, "h"), ExecOptions{}, &stream); err == nil {
		t.Fatal("expected an error for an empty command")
	}
}
