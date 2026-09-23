package session

import (
	"bufio"
	"bytes"
	"io"
	"net"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/gorilla/websocket"

	"github.com/jmelahman/kanban/internal/docker"
)

// addFakeBroker registers a broker whose "exec" is one end of a pipe, so the
// set can be exercised without a docker daemon. The other end is returned:
// it reads EOF once the broker closes its connection.
func addFakeBroker(t *testing.T, s *brokerSet, sessionID int64, kind, harnessID string) (*sessionPTY, net.Conn) {
	t.Helper()
	ours, theirs := net.Pipe()
	t.Cleanup(func() { ours.Close(); theirs.Close() })
	b := &sessionPTY{
		key:         brokerKey{sessionID: sessionID, kind: kind},
		attached:    &docker.AttachedExec{ID: "exec-" + kind, Conn: types.HijackedResponse{Conn: ours, Reader: bufio.NewReader(ours)}},
		set:         s,
		containerID: "c0ffee",
		harness:     harnessID,
		marker:      kind + "-TEST",
		buf:         newRingBuffer(16),
		clients:     map[*websocket.Conn]*clientView{},
	}
	s.mu.Lock()
	s.perSess[b.key] = b
	s.mu.Unlock()
	return b, theirs
}

func (s *brokerSet) get(sessionID int64, kind string) *sessionPTY {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.perSess[brokerKey{sessionID: sessionID, kind: kind}]
}

func TestCloseAgentUnless(t *testing.T) {
	t.Run("different_harness_is_closed", func(t *testing.T) {
		s := newBrokerSet(nil)
		agent, exec := addFakeBroker(t, s, 1, "agent", "claude")
		shell, _ := addFakeBroker(t, s, 1, "shell", "")
		other, _ := addFakeBroker(t, s, 2, "agent", "claude")

		if got := s.closeAgentUnless(1, "pi"); got != agent {
			t.Fatalf("closed %v; want the session's agent broker", got)
		}
		if s.get(1, "agent") != nil {
			t.Error("agent broker still registered; a new attach would rejoin it")
		}
		if !agent.closed {
			t.Error("agent broker not shut down")
		}
		if _, err := exec.Read(make([]byte, 1)); err != io.EOF {
			t.Errorf("exec connection read = %v; want EOF after close", err)
		}
		if s.get(1, "shell") != shell || shell.closed {
			t.Error("shell broker was touched")
		}
		if s.get(2, "agent") != other || other.closed {
			t.Error("another session's agent was touched")
		}
	})

	t.Run("same_harness_is_kept", func(t *testing.T) {
		s := newBrokerSet(nil)
		agent, _ := addFakeBroker(t, s, 1, "agent", "pi")
		if got := s.closeAgentUnless(1, "pi"); got != nil {
			t.Fatalf("closed %v; want nothing", got)
		}
		if s.get(1, "agent") != agent || agent.closed {
			t.Error("agent broker running the requested harness was closed")
		}
	})

	t.Run("nothing_running", func(t *testing.T) {
		s := newBrokerSet(nil)
		addFakeBroker(t, s, 1, "shell", "")
		if got := s.closeAgentUnless(1, "pi"); got != nil {
			t.Fatalf("closed %v; want nothing", got)
		}
	})
}

func TestRingBuffer_PartialFill(t *testing.T) {
	r := newRingBuffer(16)
	r.Write([]byte("hello"))
	if got, want := r.Snapshot(), []byte("hello"); !bytes.Equal(got, want) {
		t.Fatalf("got %q want %q", got, want)
	}
	r.Write([]byte(" world"))
	if got, want := r.Snapshot(), []byte("hello world"); !bytes.Equal(got, want) {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestRingBuffer_Wrap(t *testing.T) {
	r := newRingBuffer(8)
	r.Write([]byte("abcdef"))
	r.Write([]byte("ghijk")) // 11 bytes total; last 8 should be retained
	if got, want := r.Snapshot(), []byte("defghijk"); !bytes.Equal(got, want) {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestRingBuffer_OversizedSingleWrite(t *testing.T) {
	r := newRingBuffer(4)
	r.Write([]byte("0123456789"))
	if got, want := r.Snapshot(), []byte("6789"); !bytes.Equal(got, want) {
		t.Fatalf("got %q want %q", got, want)
	}
	// Subsequent small write should append to the latest tail.
	r.Write([]byte("ab"))
	if got, want := r.Snapshot(), []byte("89ab"); !bytes.Equal(got, want) {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestRingBuffer_ExactSize(t *testing.T) {
	r := newRingBuffer(4)
	r.Write([]byte("abcd"))
	if got, want := r.Snapshot(), []byte("abcd"); !bytes.Equal(got, want) {
		t.Fatalf("got %q want %q", got, want)
	}
	r.Write([]byte("ef"))
	if got, want := r.Snapshot(), []byte("cdef"); !bytes.Equal(got, want) {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestRingBuffer_Empty(t *testing.T) {
	r := newRingBuffer(8)
	if got := r.Snapshot(); len(got) != 0 {
		t.Fatalf("got %q want empty", got)
	}
}

func TestReplayPayload_PrependsResetWithSnapshot(t *testing.T) {
	got := replayPayload([]byte("hello"))
	want := []byte("\x1bchello")
	if !bytes.Equal(got, want) {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestReplayPayload_EmptySnapshotStillResets(t *testing.T) {
	got := replayPayload(nil)
	want := []byte("\x1bc")
	if !bytes.Equal(got, want) {
		t.Fatalf("got %q want %q", got, want)
	}
}
