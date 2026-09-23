package metrics

import (
	"errors"
	"testing"
	"time"

	dto "github.com/prometheus/client_model/go"
)

func TestObserveGitCommand(t *testing.T) {
	// Distinct operation labels keep this test insensitive to any other git
	// metrics recorded against the shared package registry.
	const okOp = "test_ok_op"
	const errOp = "test_err_op"

	ObserveGitCommand(okOp, time.Now(), nil)
	ObserveGitCommand(errOp, time.Now(), errors.New("boom"))

	// Both ops land in the duration histogram (one sample each), regardless of
	// outcome.
	for _, op := range []string{okOp, errOp} {
		if got := histogramSampleCount(t, op); got != 1 {
			t.Errorf("histogram sample count for %q = %d; want 1", op, got)
		}
	}

	// Only the failed op increments the error counter.
	if got := errorCount(t, okOp); got != 0 {
		t.Errorf("ok op error count = %v; want 0", got)
	}
	if got := errorCount(t, errOp); got != 1 {
		t.Errorf("err op error count = %v; want 1", got)
	}
}

// findMetric returns the gathered metric whose `operation` label matches op,
// from the metric family named name. Fails the test if absent.
func findMetric(t *testing.T, name, op string) *dto.Metric {
	t.Helper()
	mfs, err := Registry.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	for _, mf := range mfs {
		if mf.GetName() != name {
			continue
		}
		for _, m := range mf.GetMetric() {
			for _, lp := range m.GetLabel() {
				if lp.GetName() == "operation" && lp.GetValue() == op {
					return m
				}
			}
		}
	}
	t.Fatalf("metric %q with operation=%q not found", name, op)
	return nil
}

func histogramSampleCount(t *testing.T, op string) uint64 {
	t.Helper()
	return findMetric(t, "kanban_git_command_duration_seconds", op).GetHistogram().GetSampleCount()
}

func errorCount(t *testing.T, op string) float64 {
	t.Helper()
	mfs, _ := Registry.Gather()
	for _, mf := range mfs {
		if mf.GetName() != "kanban_git_command_errors_total" {
			continue
		}
		for _, m := range mf.GetMetric() {
			for _, lp := range m.GetLabel() {
				if lp.GetName() == "operation" && lp.GetValue() == op {
					return m.GetCounter().GetValue()
				}
			}
		}
	}
	return 0 // counter vec doesn't emit a series until first Inc — absent == 0
}
