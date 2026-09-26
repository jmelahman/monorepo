//go:build linux

package tasks

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestStopScriptKillsTaggedTree runs stopScript against real processes: a
// tagged shell, a reparented grandchild, and a child that cleared its
// environment must all get SIGTERM, while an untagged process (and another
// run's) survives.
func TestStopScriptKillsTaggedTree(t *testing.T) {
	const id = 424242
	dir := t.TempDir()
	orphanFile, envlessFile := dir+"/orphan.pid", dir+"/envless.pid"

	// The inner `sh -c '... &'` exits immediately, so its sleep is
	// reparented away from the task shell — a PPid walk would miss it. The
	// `env -i` child stands in for `sudo`: no marker, but still a child.
	task := exec.Command("sh", "-c",
		`sh -c 'sleep 300 & echo $! >"$0"' "$1"; env -i sleep 300 & echo $! >"$2"; sleep 300`,
		"sh", orphanFile, envlessFile)
	task.Env = append(os.Environ(), taskRunMarker(id))
	other := exec.Command("sleep", "300")
	other.Env = append(os.Environ(), taskRunMarker(id+1))
	bystander := exec.Command("sleep", "300")
	for _, c := range []*exec.Cmd{task, other, bystander} {
		if err := c.Start(); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = c.Process.Kill(); _, _ = c.Process.Wait() })
	}
	orphan, envless := waitPid(t, orphanFile), waitPid(t, envlessFile)

	if out, err := exec.Command("sh", "-c", stopScript(id)).CombinedOutput(); err != nil {
		t.Fatalf("stop script: %v\n%s", err, out)
	}

	done := make(chan error, 1)
	go func() { done <- task.Wait() }()
	select {
	case err := <-done:
		if ee, ok := err.(*exec.ExitError); !ok || ee.Sys().(syscall.WaitStatus).Signal() != syscall.SIGTERM {
			t.Errorf("task shell: got %v, want SIGTERM", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("task shell survived stop")
	}
	for name, pid := range map[string]int{"reparented grandchild": orphan, "env-cleared child": envless} {
		for deadline := time.Now().Add(5 * time.Second); syscall.Kill(pid, 0) == nil; {
			if time.Now().After(deadline) {
				t.Fatalf("%s survived stop", name)
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	for name, c := range map[string]*exec.Cmd{"other run": other, "bystander": bystander} {
		if err := c.Process.Signal(syscall.Signal(0)); err != nil {
			t.Errorf("%s was killed: %v", name, err)
		}
	}
}

// waitPid reads the pid a test process writes to path, and SIGKILLs it at
// cleanup in case the stop under test missed it.
func waitPid(t *testing.T, path string) int {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); ; {
		if b, err := os.ReadFile(path); err == nil && strings.HasSuffix(string(b), "\n") {
			pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = syscall.Kill(pid, syscall.SIGKILL) })
			return pid
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s never written", path)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
