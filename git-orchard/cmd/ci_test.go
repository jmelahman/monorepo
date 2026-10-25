package cmd

import (
	"bytes"
	"errors"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/jmelahman/git-orchard/ghrun"
)

var now = time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)

func ciRunOf(workflow, status, conclusion string, ago time.Duration) ciRun {
	return ciRun{
		ID: 1, Workflow: workflow, Status: status, Conclusion: conclusion,
		HeadSHA: "0123456789abcdef", Title: "Fix the thing", URL: "https://example.com/" + workflow,
		CreatedAt: now.Add(-ago),
	}
}

func ciResults() []ciResult {
	return []ciResult{
		{Prefix: ".", Repo: "o/mono", Branch: "master", Runs: []ciRun{
			ciRunOf("Tests", "completed", "failure", 3*time.Hour),
			ciRunOf("Mirror", "in_progress", "", 30*time.Second),
		}},
		{Prefix: "tools/foo", Repo: "o/foo", Branch: "main", Runs: []ciRun{
			ciRunOf("pre-commit", "completed", "success", 72*time.Hour),
		}},
		{Prefix: "bar", Branch: "master", Runs: []ciRun{}},
		{Prefix: "baz", Repo: "o/baz", Branch: "master", Runs: []ciRun{}, Error: "gh run list: EOF"},
	}
}

func TestPrintCI(t *testing.T) {
	var out bytes.Buffer
	printCI(&out, ciResults(), &CIOptions{}, false, now)
	want := `.          ✗ failure      Tests       3h ago    0123456  Fix the thing  https://example.com/Tests
.          ● in_progress  Mirror      just now  0123456  Fix the thing  https://example.com/Mirror
tools/foo  ✓ success      pre-commit  3d ago    0123456  Fix the thing  https://example.com/pre-commit
bar        - not on GitHub
baz        ! error        gh run list: EOF

1 failed, 1 running, 3 run(s) in 4 repositories
`
	if out.String() != want {
		t.Errorf("got\n%s\nwant\n%s", out.String(), want)
	}
}

func TestPrintCIFailed(t *testing.T) {
	var out bytes.Buffer
	printCI(&out, ciResults(), &CIOptions{Failed: true}, false, now)
	want := `.    ✗ failure  Tests  3h ago  0123456  Fix the thing  https://example.com/Tests
baz  ! error    gh run list: EOF

1 failed run(s) in 4 repositories
`
	if out.String() != want {
		t.Errorf("got\n%s\nwant\n%s", out.String(), want)
	}
}

func TestPrintCIColorAligns(t *testing.T) {
	var out bytes.Buffer
	printCI(&out, ciResults()[:2], &CIOptions{}, true, now)
	lines := strings.Split(out.String(), "\n")
	// Workflows link to their runs, so the age column starts after them.
	col := func(line string) int {
		return utf8.RuneCountInString(strings.SplitN(stripEscapes(line), "ago", 2)[0])
	}
	if a, b := col(lines[0]), col(lines[2]); a != b {
		t.Errorf("columns misaligned:\n%s\n%s", stripEscapes(lines[0]), stripEscapes(lines[2]))
	}
	if !strings.Contains(lines[0], "\x1b]8;;https://example.com/Tests\x1b\\Tests") {
		t.Errorf("Tests doesn't link to its run: %q", lines[0])
	}
}

var escapes = regexp.MustCompile(`\x1b\[[0-9;]*m|\x1b\]8;;[^\x1b]*\x1b\\`)

func stripEscapes(s string) string {
	return escapes.ReplaceAllString(s, "")
}

func TestVisibleLen(t *testing.T) {
	for s, want := range map[string]int{
		"plain":                    5,
		"✓ ok":                     4,
		"\x1b[31m✗ failure\x1b[0m": 9,
		"\x1b]8;;https://e.com/x\x1b\\Tests\x1b]8;;\x1b\\": 5,
	} {
		if got := visibleLen(s); got != want {
			t.Errorf("visibleLen(%q) = %d, want %d", s, got, want)
		}
	}
}

func TestCIError(t *testing.T) {
	results := ciResults()
	// A failure is news even when some repository couldn't be checked.
	if err := ciError(results); !errors.Is(err, ErrCIFailed) {
		t.Errorf("with an error and a failure, got %v; want ErrCIFailed", err)
	}
	if err := ciError(results[1:]); err == nil || errors.Is(err, ErrCIFailed) {
		t.Errorf("with an error and no failure, got %v; want the error", err)
	}
	if err := ciError(results[1:3]); err != nil {
		t.Errorf("with nothing failed, got %v", err)
	}
}

func TestFilterCI(t *testing.T) {
	got := filterCI(ciResults(), true)
	prefixes := make([]string, 0, len(got))
	for _, r := range got {
		prefixes = append(prefixes, r.Prefix)
		// JSON has [], not null, for a repository with nothing failed.
		if r.Runs == nil {
			t.Errorf("%s: Runs is nil", r.Prefix)
		}
	}
	if want := []string{".", "baz"}; !slices.Equal(prefixes, want) {
		t.Errorf("got %v, want %v", prefixes, want)
	}
}

func TestPrintCIEscapes(t *testing.T) {
	results := ciResults()[:1]
	results[0].Runs[0].Title = "Clear \x1b[2J the screen"
	var out bytes.Buffer
	printCI(&out, results, &CIOptions{}, false, now)
	if strings.Contains(out.String(), "\x1b") {
		t.Errorf("a title's escape reached the terminal: %q", out.String())
	}
}

func TestPrintCILogs(t *testing.T) {
	results := ciResults()[:1]
	results[0].Runs[0].Logs = []ghrun.Excerpt{{Job: "test", Step: "Run go test", Lines: []string{"FAIL"}, Omitted: 3}}
	var out bytes.Buffer
	printCILogs(&out, results)
	for _, want := range []string{
		"## .: Tests (failure)",
		"Repository o/mono, branch master, commit 0123456789abcdef: Fix the thing\nhttps://example.com/Tests",
		"### Job test, step Run go test",
		"3 earlier lines left out; gh run view 1 --repo o/mono --log-failed has them all.",
		"```text\nFAIL\n```",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("logs lack %q:\n%s", want, out.String())
		}
	}
	if strings.Contains(out.String(), "Mirror") {
		t.Errorf("logs include a run that didn't fail:\n%s", out.String())
	}
}

func staleResults() []ciResult {
	stale := ciRunOf("Dependabot Updates", "completed", "failure", 48*time.Hour)
	stale.Stale = "no .github/dependabot.yml and security updates are off"
	return []ciResult{{Prefix: "tools/foo", Repo: "o/foo", Branch: "main", Runs: []ciRun{
		stale,
		ciRunOf("pre-commit", "completed", "success", 72*time.Hour),
	}}}
}

func TestPrintCIStale(t *testing.T) {
	var out bytes.Buffer
	printCI(&out, staleResults(), &CIOptions{}, false, now)
	want := `tools/foo  - stale failure  Dependabot Updates  2d ago  0123456  Fix the thing  https://example.com/Dependabot Updates
tools/foo  ✓ success        pre-commit          3d ago  0123456  Fix the thing  https://example.com/pre-commit

0 failed, 0 running, 2 run(s) in 1 repositories; 1 stale failed run(s), which --prune-stale deletes
`
	if out.String() != want {
		t.Errorf("got\n%s\nwant\n%s", out.String(), want)
	}
}

func TestPrintCIFailedHidesStale(t *testing.T) {
	var out bytes.Buffer
	printCI(&out, staleResults(), &CIOptions{Failed: true}, false, now)
	want := "Nothing failed in 1 repositories; 1 stale failed run(s) hidden, which --prune-stale deletes\n"
	if out.String() != want {
		t.Errorf("got\n%s\nwant\n%s", out.String(), want)
	}
}

func TestStaleDoesntFail(t *testing.T) {
	if err := ciError(staleResults()); err != nil {
		t.Errorf("got %v for only a stale failure", err)
	}
	var out bytes.Buffer
	printCILogs(&out, staleResults())
	if out.Len() != 0 {
		t.Errorf("logs include a stale run:\n%s", out.String())
	}
}
