package ghrun

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// Workflow is a workflow, as gh workflow list --json describes it.
type Workflow struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	State string `json:"state"`
}

// Latest returns the latest run of each active workflow on branch in repo,
// by workflow name. Deleted and disabled workflows are left out, since their
// last runs are history, and so are pull requests' runs: they test the pull
// request, not the branch.
//
// One page of the branch's runs usually has every workflow's latest, but a
// frequent schedule can crowd a workflow out, so those missing are listed
// on their own, a few at a time.
func Latest(ctx context.Context, repo, branch string) ([]Run, error) {
	out, err := gh(ctx, "workflow", "list", "--repo", repo, "--limit", "100", "--json", "id,name,state")
	if err != nil {
		return nil, err
	}
	// A repository without workflows gets no output, rather than [].
	if len(bytes.TrimSpace(out)) == 0 {
		return nil, nil
	}
	var workflows []Workflow
	if err := json.Unmarshal(out, &workflows); err != nil {
		return nil, fmt.Errorf("gh workflow list --repo %s: %w", repo, err)
	}
	runs, err := listRuns(ctx, repo, "--branch", branch, "--limit", "100")
	if err != nil {
		return nil, err
	}
	latest := LatestPerWorkflow(workflows, runs)
	seen := map[int64]bool{}
	for _, r := range latest {
		seen[r.WorkflowID] = true
	}
	var (
		mu       sync.Mutex
		firstErr error
		wg       sync.WaitGroup
		sem      = make(chan struct{}, 4)
	)
	for _, w := range workflows {
		if w.State != "active" || seen[w.ID] {
			continue
		}
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			runs, err := listRuns(ctx, repo, "--branch", branch, "--workflow", strconv.FormatInt(w.ID, 10), "--limit", "20")
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				firstErr = cmp.Or(firstErr, err)
				return
			}
			latest = append(latest, LatestPerWorkflow(workflows, runs)...)
		})
	}
	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	sort.Slice(latest, func(i, j int) bool { return latest[i].Workflow < latest[j].Workflow })
	return latest, nil
}

// LatestPerWorkflow returns the newest of runs for each of the active
// workflows, by workflow name, leaving out pull requests' runs.
func LatestPerWorkflow(workflows []Workflow, runs []Run) []Run {
	active := map[int64]bool{}
	for _, w := range workflows {
		active[w.ID] = w.State == "active"
	}
	latest := map[int64]Run{}
	for _, r := range runs {
		switch r.Event {
		case "pull_request", "pull_request_target":
			continue
		}
		if !active[r.WorkflowID] {
			continue
		}
		if l, ok := latest[r.WorkflowID]; !ok || r.CreatedAt.After(l.CreatedAt) {
			latest[r.WorkflowID] = r
		}
	}
	out := make([]Run, 0, len(latest))
	for _, r := range latest {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Workflow < out[j].Workflow })
	return out
}

// FailedLog returns the logs of the failed jobs of run id in repo, as gh run
// view --log-failed prints them: a line per log line, of the job, the step
// and the timestamped text, separated by tabs.
func FailedLog(ctx context.Context, repo string, id int64) (string, error) {
	out, err := gh(ctx, "run", "view", strconv.FormatInt(id, 10), "--repo", repo, "--log-failed")
	return string(out), err
}

// Excerpt is the failing part of one job's log.
type Excerpt struct {
	Job string `json:"job"`
	// Step is the failing step, as its log names it, e.g. "Run go test ./...".
	Step  string   `json:"step,omitempty"`
	Lines []string `json:"lines"`
	// Omitted is the number of lines left out from the start of Lines.
	Omitted int `json:"omitted,omitempty"`
}

var (
	timestamp = regexp.MustCompile(`^\d{4}-\d\d-\d\dT[\d:.]+Z ?`)
	// Logs keep their escape sequences, sometimes as a literal "^[".
	escape = regexp.MustCompile(`(?:\x1b|\^\[)\[[0-9;?]*[A-Za-z]`)
)

// Excerpts picks out, for each job in a FailedLog, the output of the step
// that failed, keeping at most maxLines of it, the last ones (all of them if
// maxLines < 1).
//
// The logs don't name their steps, but each starts at a "##[group]Run ..."
// line, and a failed job stops at the failing step: so it's the step that
// logged the job's first error, or failing that the last before post-job
// cleanup. Groups within it, which collapse in GitHub's log viewer, are
// usually setup or verbose logging, so they're left out, all but their
// errors; except a group the step never closed, likely where it died.
func Excerpts(log string, maxLines int) []Excerpt {
	var jobs []string
	lines := map[string][]string{}
	for line := range strings.Lines(log) {
		line = strings.TrimPrefix(strings.TrimRight(line, "\r\n"), "\ufeff")
		job, text := "", line
		if fields := strings.SplitN(line, "\t", 3); len(fields) == 3 {
			job, text = fields[0], fields[2]
		}
		text = strings.TrimPrefix(text, "\ufeff")
		text = timestamp.ReplaceAllString(text, "")
		text = Printable(escape.ReplaceAllString(text, ""))
		if _, ok := lines[job]; !ok {
			jobs = append(jobs, job)
		}
		lines[job] = append(lines[job], text)
	}

	excerpts := make([]Excerpt, 0, len(jobs))
	for _, job := range jobs {
		excerpts = append(excerpts, excerpt(job, lines[job], maxLines))
	}
	return excerpts
}

func excerpt(job string, lines []string, maxLines int) Excerpt {
	end := len(lines)
	for i, l := range lines {
		if strings.HasPrefix(l, "Post job cleanup.") {
			end = i
			break
		}
	}
	// Steps can run after the failing one, with if: always(), so where the
	// job logged an error, it's in the step that did.
	failed := end - 1
	for i, l := range lines[:end] {
		if strings.HasPrefix(l, "##[error]") {
			failed = i
			break
		}
	}
	start := 0
	for i := failed; i >= 0; i-- {
		if strings.HasPrefix(lines[i], "##[group]Run ") {
			start = i
			break
		}
	}
	for i := failed + 1; i < end; i++ {
		if strings.HasPrefix(lines[i], "##[group]Run ") {
			end = i
			break
		}
	}

	e := Excerpt{Job: job, Lines: []string{}}
	depth := 0
	var grouped []string // the lines of the open groups
	for i, l := range lines[start:end] {
		switch {
		case strings.HasPrefix(l, "##[group]"):
			if i == 0 {
				e.Step = strings.TrimPrefix(l, "##[group]")
			}
			depth++
		case strings.HasPrefix(l, "##[endgroup]"):
			depth = max(depth-1, 0)
			if depth == 0 {
				for _, g := range grouped {
					if strings.HasPrefix(g, "##[error]") {
						e.Lines = append(e.Lines, g)
					}
				}
				grouped = nil
			}
		case strings.HasPrefix(l, "##[start-action"), strings.HasPrefix(l, "##[end-action"):
		case depth == 0:
			e.Lines = append(e.Lines, l)
		default:
			grouped = append(grouped, l)
		}
	}
	e.Lines = append(e.Lines, grouped...)
	for len(e.Lines) > 0 && strings.TrimSpace(e.Lines[len(e.Lines)-1]) == "" {
		e.Lines = e.Lines[:len(e.Lines)-1]
	}
	if maxLines > 0 && len(e.Lines) > maxLines {
		e.Omitted = len(e.Lines) - maxLines
		e.Lines = e.Lines[e.Omitted:]
	}
	return e
}
