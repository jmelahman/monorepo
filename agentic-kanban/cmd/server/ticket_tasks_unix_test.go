//go:build unix

package server

import (
	"fmt"
	"net/http"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestTicketTasksRunCtrlCStops checks that Ctrl-C during a foreground run
// stops the run on the server instead of just abandoning the stream. The
// fake server delivers the SIGINT itself once the output stream is open,
// which is after followTaskRun has registered for it.
func TestTicketTasksRunCtrlCStops(t *testing.T) {
	f := &fakeTasksAPI{}
	f.onOutput = func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: serving\n\n")
		w.(http.Flusher).Flush()
		if err := syscall.Kill(syscall.Getpid(), syscall.SIGINT); err != nil {
			t.Error(err)
		}
		// Hold the stream open until the CLI asks for the stop, as the
		// real server does until the process exits.
		deadline := time.After(5 * time.Second)
		for {
			f.mu.Lock()
			n := len(f.stopped)
			f.mu.Unlock()
			if n > 0 {
				break
			}
			select {
			case <-deadline:
				t.Error("CLI never stopped the run")
				return
			case <-time.After(10 * time.Millisecond):
			}
		}
		f.finish(101, "exited", 143)
		fmt.Fprint(w, "event: end\ndata: {}\n\n")
	}
	srv := newFakeTasksServer(t, f)

	out, errOut, err := runTasksCmd(t, srv.URL, "42", "--run", "Web")
	if err != nil {
		t.Fatalf("%v\n%s", err, errOut)
	}
	if len(f.stopped) != 1 || f.stopped[0] != 101 {
		t.Errorf("stopped = %v, want [101]", f.stopped)
	}
	if !strings.Contains(out, "serving") || !strings.Contains(errOut, "stopped run #101") {
		t.Errorf("stdout=%q stderr=%q", out, errOut)
	}
}
