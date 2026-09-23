package metrics

import "time"

// ObserveGitCommand records the latency and outcome of a local git operation.
//
// operation is a bounded, human-readable label (e.g. "worktree_add", "diff",
// "fetch") — keep the set small and fixed so the time-series cardinality stays
// flat. Pass `start` captured immediately before the operation began and the
// error it returned (nil on success). A non-nil error both lands in the
// duration histogram and increments the error counter, so the histogram
// measures total time spent regardless of outcome.
func ObserveGitCommand(operation string, start time.Time, err error) {
	gitCommandDuration.WithLabelValues(operation).Observe(time.Since(start).Seconds())
	if err != nil {
		gitCommandErrors.WithLabelValues(operation).Inc()
	}
}
