package dockerapi

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"testing"
)

// fakeDaemon serves a Docker Engine API stub on a unix socket and points
// DOCKER_HOST at it, so Connect dials the stub instead of a real daemon.
func fakeDaemon(t *testing.T, mux *http.ServeMux) {
	t.Helper()
	sock := filepath.Join(t.TempDir(), "docker.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	mux.HandleFunc("/_ping", func(w http.ResponseWriter, r *http.Request) {})
	mux.HandleFunc("/info", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"SecurityOptions":[]}`)
	})
	srv := &http.Server{Handler: mux}
	go srv.Serve(ln) //nolint:errcheck
	t.Cleanup(func() { srv.Close() })
	t.Setenv("DOCKER_HOST", "unix://"+sock)
}

func TestSampleStats(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/containers/abc123/stats", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("one-shot") != "true" || r.URL.Query().Get("stream") != "false" {
			http.Error(w, "want one-shot non-streaming query", http.StatusBadRequest)
			return
		}
		fmt.Fprint(w, `{
			"cpu_stats": {
				"cpu_usage": {"total_usage": 4000000},
				"system_cpu_usage": 900000000,
				"online_cpus": 8
			},
			"memory_stats": {
				"usage": 50000000,
				"stats": {"inactive_file": 10000000},
				"limit": 33000000000
			}
		}`)
	})
	fakeDaemon(t, mux)

	cli, err := Connect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	s, err := cli.SampleStats(context.Background(), "abc123")
	if err != nil {
		t.Fatal(err)
	}
	want := ContainerStats{
		CPUTotal:         4000000,
		SystemCPUTotal:   900000000,
		OnlineCPUs:       8,
		MemoryBytes:      40000000, // usage minus inactive_file
		MemoryLimitBytes: 33000000000,
	}
	if s != want {
		t.Fatalf("stats = %+v, want %+v", s, want)
	}
}

// Cgroup v1 daemons report the reclaimable cache as total_inactive_file.
func TestSampleStatsCgroupV1(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/containers/v1/stats", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{
			"cpu_stats": {"cpu_usage": {"total_usage": 1}, "system_cpu_usage": 2},
			"memory_stats": {"usage": 300, "stats": {"total_inactive_file": 100}, "limit": 1000}
		}`)
	})
	fakeDaemon(t, mux)

	cli, err := Connect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	s, err := cli.SampleStats(context.Background(), "v1")
	if err != nil {
		t.Fatal(err)
	}
	if s.MemoryBytes != 200 || s.OnlineCPUs != 0 {
		t.Fatalf("stats = %+v, want MemoryBytes 200, OnlineCPUs 0", s)
	}
}
