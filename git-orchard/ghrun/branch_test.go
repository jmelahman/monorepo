package ghrun

import (
	"slices"
	"strings"
	"testing"
	"time"
)

func TestLatestPerWorkflow(t *testing.T) {
	at := func(h int) time.Time { return time.Date(2026, 1, 1, h, 0, 0, 0, time.UTC) }
	workflows := []Workflow{
		{ID: 1, Name: "Tests", State: "active"},
		{ID: 2, Name: "Release", State: "active"},
		{ID: 3, Name: "Old", State: "disabled_manually"},
	}
	runs := []Run{
		{ID: 10, WorkflowID: 1, Workflow: "Tests", Event: "push", CreatedAt: at(1)},
		{ID: 11, WorkflowID: 1, Workflow: "Tests", Event: "push", CreatedAt: at(3)},
		{ID: 12, WorkflowID: 1, Workflow: "Tests", Event: "pull_request", CreatedAt: at(4)},
		{ID: 13, WorkflowID: 1, Workflow: "Tests", Event: "schedule", CreatedAt: at(2)},
		{ID: 20, WorkflowID: 2, Workflow: "Release", Event: "push", CreatedAt: at(1)},
		{ID: 30, WorkflowID: 3, Workflow: "Old", Event: "push", CreatedAt: at(5)},
		// A deleted workflow, which gh workflow list leaves out.
		{ID: 40, WorkflowID: 4, Workflow: "Gone", Event: "push", CreatedAt: at(5)},
	}
	var got []int64
	for _, r := range LatestPerWorkflow(workflows, runs) {
		got = append(got, r.ID)
	}
	if want := []int64{20, 11}; !slices.Equal(got, want) {
		t.Errorf("got runs %v, want %v", got, want)
	}
}

func TestFailed(t *testing.T) {
	for _, tc := range []struct {
		status, conclusion string
		want               bool
	}{
		{"completed", "failure", true},
		{"completed", "timed_out", true},
		{"completed", "startup_failure", true},
		{"completed", "cancelled", false},
		{"completed", "success", false},
		{"completed", "skipped", false},
		{"in_progress", "", false},
	} {
		if got := (Run{Status: tc.status, Conclusion: tc.conclusion}).Failed(); got != tc.want {
			t.Errorf("%s/%s: Failed() = %t, want %t", tc.status, tc.conclusion, got, tc.want)
		}
	}
}

// failedLog builds a gh run view --log-failed log of job's lines.
func failedLog(job string, lines ...string) string {
	var b strings.Builder
	for i, l := range lines {
		b.WriteString(job + "\tUNKNOWN STEP\t")
		if i == 0 {
			b.WriteString("\ufeff") // as GitHub starts each job's log
		}
		b.WriteString("2026-10-24T23:58:40.8553668Z " + l + "\n")
	}
	return b.String()
}

func TestExcerpts(t *testing.T) {
	log := failedLog("test",
		"Current runner version: '2.338.0'",
		"##[group]Run actions/checkout@v5",
		"with:",
		"##[endgroup]",
		"Syncing repository",
		"##[group]Run go test ./...",
		"go test ./...",
		"##[endgroup]",
		"##[group]Downloading modules",
		"go: downloading example.com/m v1.0.0",
		"##[endgroup]",
		"--- FAIL: TestThing (0.00s)",
		"    thing_test.go:12: got \x1b[31m1\x1b[0m, want ^[[32m2^[[0m",
		"##[start-action display=Run x;id=x]",
		"FAIL",
		"##[error]Process completed with exit code 1.",
		// An if: always() step, after the failure.
		"##[group]Run actions/upload-artifact@v4",
		"Uploading",
		"##[endgroup]",
		"Uploaded",
		"Post job cleanup.",
		"Cleaning up orphan processes",
	) + failedLog("lint",
		"##[group]Run golangci-lint",
		"##[endgroup]",
		"a.go:1: one",
		"a.go:2: two",
		"a.go:3: three",
		"",
		"Post job cleanup.",
	)

	got := Excerpts(log, 2)
	if len(got) != 2 {
		t.Fatalf("got %d excerpts, want 2: %+v", len(got), got)
	}
	test, lint := got[0], got[1]
	if test.Job != "test" || test.Step != "Run go test ./..." {
		t.Errorf("got job %q step %q, want test and Run go test ./...", test.Job, test.Step)
	}
	if lint.Job != "lint" || lint.Step != "Run golangci-lint" {
		t.Errorf("got job %q step %q, want lint and Run golangci-lint", lint.Job, lint.Step)
	}
	if want := []string{"a.go:2: two", "a.go:3: three"}; !slices.Equal(lint.Lines, want) || lint.Omitted != 1 {
		t.Errorf("got lint lines %q, %d omitted; want %q, 1 omitted", lint.Lines, lint.Omitted, want)
	}

	all := Excerpts(log, 0)[0]
	// The step's own group is its script and inputs, which Step sums up.
	want := []string{
		"--- FAIL: TestThing (0.00s)",
		"    thing_test.go:12: got 1, want 2",
		"FAIL",
		"##[error]Process completed with exit code 1.",
	}
	if !slices.Equal(all.Lines, want) || all.Omitted != 0 {
		t.Errorf("got lines\n%s\nwant\n%s", strings.Join(all.Lines, "\n"), strings.Join(want, "\n"))
	}
}

func TestExcerptsWithoutError(t *testing.T) {
	// Without an error, the failing step is the last, before cleanup.
	got := Excerpts(failedLog("build",
		"##[group]Run make",
		"##[endgroup]",
		"make: *** [all] Killed",
		"Post job cleanup.",
		"##[group]Run cleanup",
	), 0)
	if len(got) != 1 || got[0].Step != "Run make" || !slices.Equal(got[0].Lines, []string{"make: *** [all] Killed"}) {
		t.Errorf("got %+v", got)
	}
}

func TestExcerptsGroups(t *testing.T) {
	got := Excerpts(failedLog("test",
		"##[group]Run make",
		"##[endgroup]",
		"building",
		"##[group]Compiling",
		"cc a.c",
		"##[error]a.c:1: syntax error",
		"##[endgroup]",
		"##[group]Linking",
		"ld a.o",
		"Killed",
		"Post job cleanup.",
	), 0)
	// An error in a group is kept, and so is a group left open.
	want := []string{"building", "##[error]a.c:1: syntax error", "ld a.o", "Killed"}
	if len(got) != 1 || !slices.Equal(got[0].Lines, want) {
		t.Errorf("got %+v, want lines %q", got, want)
	}
}
