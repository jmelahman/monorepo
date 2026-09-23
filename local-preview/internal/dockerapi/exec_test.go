package dockerapi

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// fakeEngine serves just enough of the engine API on a unix socket to
// exercise the exec flow, including the hijacked attach: exec-start upgrades
// the connection and echoes whatever arrives on stdin back as a multiplexed
// stdout frame.
func fakeEngine(t *testing.T) string {
	t.Helper()
	// Not t.TempDir(): unix socket paths have a ~104-byte cap and deep test
	// dirs can exceed it.
	dir, err := os.MkdirTemp("", "dsock")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	sock := filepath.Join(dir, "docker.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /_ping", func(w http.ResponseWriter, r *http.Request) {})
	mux.HandleFunc("GET /info", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"SecurityOptions": []string{}}) //nolint:errcheck
	})
	mux.HandleFunc("POST /containers/{id}/exec", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Cmd []string
			Tty bool
		}
		json.NewDecoder(r.Body).Decode(&req) //nolint:errcheck
		if len(req.Cmd) == 0 {
			http.Error(w, `{"message":"no command"}`, http.StatusBadRequest)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"Id": "e1"}) //nolint:errcheck
	})
	mux.HandleFunc("POST /exec/e1/start", func(w http.ResponseWriter, r *http.Request) {
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Errorf("hijack: %v", err)
			return
		}
		defer conn.Close()
		conn.Write([]byte("HTTP/1.1 101 UPGRADED\r\nContent-Type: application/vnd.docker.raw-stream\r\nConnection: Upgrade\r\nUpgrade: tcp\r\n\r\n")) //nolint:errcheck
		buf := make([]byte, 64)
		n, err := conn.Read(buf)
		if err != nil {
			return
		}
		var hdr [8]byte
		hdr[0] = 1 // stdout
		binary.BigEndian.PutUint32(hdr[4:], uint32(n))
		conn.Write(hdr[:])  //nolint:errcheck
		conn.Write(buf[:n]) //nolint:errcheck
	})
	mux.HandleFunc("GET /exec/e1/json", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"Running": false, "ExitCode": 5}) //nolint:errcheck
	})
	srv := &http.Server{Handler: mux}
	go srv.Serve(ln) //nolint:errcheck
	t.Cleanup(func() { srv.Close() })
	return sock
}

// TestExecAttachRoundTrip drives the full exec flow — create, hijacked
// start, bidirectional bytes, inspect — against the fake engine, validating
// the hand-written HTTP/1.1 upgrade ExecStart performs on the raw socket.
func TestExecAttachRoundTrip(t *testing.T) {
	t.Setenv("DOCKER_HOST", "unix://"+fakeEngine(t))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, err := Connect(ctx)
	if err != nil {
		t.Fatal(err)
	}
	id, err := c.ExecCreate(ctx, "c1", ExecSpec{Cmd: []string{"cat"}, Stdin: true})
	if err != nil || id != "e1" {
		t.Fatalf("ExecCreate = %q, %v", id, err)
	}
	conn, br, err := c.ExecStart(ctx, id, false)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.Write([]byte("ping")); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if err := DemuxStream(br, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if stdout.String() != "ping" || stderr.Len() != 0 {
		t.Fatalf("stdout = %q stderr = %q, want the echo on stdout", stdout.String(), stderr.String())
	}
	code, running, err := c.ExecInspect(ctx, id)
	if err != nil || running || code != 5 {
		t.Fatalf("inspect = code %d running %v, %v", code, running, err)
	}
}

// TestExecCreateErrorSurfacesEngineMessage: the engine's {"message"} detail
// reaches the caller.
func TestExecCreateErrorSurfacesEngineMessage(t *testing.T) {
	t.Setenv("DOCKER_HOST", "unix://"+fakeEngine(t))
	ctx := context.Background()
	c, err := Connect(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.ExecCreate(ctx, "c1", ExecSpec{}); err == nil {
		t.Fatal("expected an error for an empty command")
	}
}
