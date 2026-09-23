package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

// PRDetail is an aggregated snapshot of a single PR: diff totals, the
// effective review decision, and a summary of CI checks. It's fetched on
// demand from a few GitHub endpoints and not stored.
type PRDetail struct {
	Additions      int64    `json:"additions"`
	Deletions      int64    `json:"deletions"`
	ReviewDecision string   `json:"review_decision"`
	Checks         PRChecks `json:"checks"`
}

// ReviewDecision values mirror GitHub's GraphQL enum, lowercased.
const (
	ReviewApproved         = "approved"
	ReviewChangesRequested = "changes_requested"
	ReviewRequired         = "review_required"
)

// PRChecks is the rolled-up CI status for the PR's head commit, combining
// modern check-runs (GitHub Actions / Apps) and legacy commit statuses.
type PRChecks struct {
	Total   int            `json:"total"`
	Success int            `json:"success"`
	Failure int            `json:"failure"`
	Pending int            `json:"pending"`
	Failing []PRCheckEntry `json:"failing"`
}

type PRCheckEntry struct {
	Name string `json:"name"`
	URL  string `json:"url,omitempty"`
}

// FetchPRDetail looks up the per-PR detail endpoint plus reviews and
// check-runs/statuses for the head commit, fanning the calls out in
// parallel. Sub-fetch failures are logged into the returned struct as
// zero-value sections rather than aborting the whole request.
func FetchPRDetail(ctx context.Context, repoPath string, prNumber int64) (PRDetail, error) {
	owner, repo, host, err := ParseGitHubRepo(repoPath)
	if err != nil {
		return PRDetail{}, err
	}
	apiBase := APIBaseFor(host)
	tok := Token(host)

	cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	client := &http.Client{Timeout: 20 * time.Second}

	pull, err := fetchPRBasic(cctx, client, apiBase, tok, owner, repo, prNumber)
	if err != nil {
		return PRDetail{}, err
	}

	out := PRDetail{Additions: pull.Additions, Deletions: pull.Deletions}

	var wg sync.WaitGroup
	var reviewDecision string
	var checks PRChecks

	wg.Add(2)
	go func() {
		defer wg.Done()
		decision, err := fetchReviewDecision(cctx, client, apiBase, tok, owner, repo, prNumber)
		if err == nil {
			reviewDecision = decision
		}
	}()
	go func() {
		defer wg.Done()
		c, err := fetchChecks(cctx, client, apiBase, tok, owner, repo, pull.Head.SHA)
		if err == nil {
			checks = c
		}
	}()
	wg.Wait()

	out.ReviewDecision = reviewDecision
	out.Checks = checks
	return out, nil
}

type ghPRBasic struct {
	Additions int64 `json:"additions"`
	Deletions int64 `json:"deletions"`
	Head      struct {
		SHA string `json:"sha"`
	} `json:"head"`
}

func fetchPRBasic(ctx context.Context, client *http.Client, apiBase, tok, owner, repo string, num int64) (ghPRBasic, error) {
	u := fmt.Sprintf("%s/repos/%s/%s/pulls/%d",
		apiBase, url.PathEscape(owner), url.PathEscape(repo), num)
	var out ghPRBasic
	if err := GetJSON(ctx, client, tok, u, &out); err != nil {
		return ghPRBasic{}, err
	}
	return out, nil
}

type ghReview struct {
	State       string `json:"state"`        // APPROVED, CHANGES_REQUESTED, COMMENTED, DISMISSED, PENDING
	SubmittedAt string `json:"submitted_at"` // ISO8601; empty for pending
	User        struct {
		Login string `json:"login"`
	} `json:"user"`
}

func fetchReviewDecision(ctx context.Context, client *http.Client, apiBase, tok, owner, repo string, num int64) (string, error) {
	u := fmt.Sprintf("%s/repos/%s/%s/pulls/%d/reviews?per_page=100",
		apiBase, url.PathEscape(owner), url.PathEscape(repo), num)
	var reviews []ghReview
	if err := GetJSON(ctx, client, tok, u, &reviews); err != nil {
		return "", err
	}
	// Pick the latest non-pending, non-comment review per reviewer; the
	// rolled-up decision is "changes requested" if anyone is blocking,
	// else "approved" if anyone has approved, else "review required".
	type latest struct {
		state string
		when  string
	}
	byUser := map[string]latest{}
	for _, r := range reviews {
		s := strings.ToUpper(r.State)
		if s != "APPROVED" && s != "CHANGES_REQUESTED" && s != "DISMISSED" {
			continue
		}
		if r.User.Login == "" {
			continue
		}
		cur, ok := byUser[r.User.Login]
		if !ok || r.SubmittedAt > cur.when {
			byUser[r.User.Login] = latest{state: s, when: r.SubmittedAt}
		}
	}
	hasApproved := false
	for _, l := range byUser {
		if l.state == "CHANGES_REQUESTED" {
			return ReviewChangesRequested, nil
		}
		if l.state == "APPROVED" {
			hasApproved = true
		}
	}
	if hasApproved {
		return ReviewApproved, nil
	}
	return ReviewRequired, nil
}

type ghCheckRun struct {
	Name       string `json:"name"`
	Status     string `json:"status"`     // queued, in_progress, completed
	Conclusion string `json:"conclusion"` // success, failure, neutral, cancelled, skipped, timed_out, action_required, ""
	HTMLURL    string `json:"html_url"`
	DetailsURL string `json:"details_url"`
	StartedAt  string `json:"started_at"`
}

type ghCheckRunsResp struct {
	TotalCount int          `json:"total_count"`
	CheckRuns  []ghCheckRun `json:"check_runs"`
}

type ghStatus struct {
	State     string `json:"state"`   // success, failure, error, pending
	Context   string `json:"context"` // status name
	TargetURL string `json:"target_url"`
	UpdatedAt string `json:"updated_at"`
}

type ghCombinedStatus struct {
	State    string     `json:"state"`
	Statuses []ghStatus `json:"statuses"`
}

func fetchChecks(ctx context.Context, client *http.Client, apiBase, tok, owner, repo, sha string) (PRChecks, error) {
	if sha == "" {
		return PRChecks{}, nil
	}
	runsURL := fmt.Sprintf("%s/repos/%s/%s/commits/%s/check-runs?per_page=100",
		apiBase, url.PathEscape(owner), url.PathEscape(repo), url.PathEscape(sha))
	var runs ghCheckRunsResp
	_ = GetJSON(ctx, client, tok, runsURL, &runs)

	statusURL := fmt.Sprintf("%s/repos/%s/%s/commits/%s/status",
		apiBase, url.PathEscape(owner), url.PathEscape(repo), url.PathEscape(sha))
	var combined ghCombinedStatus
	_ = GetJSON(ctx, client, tok, statusURL, &combined)

	out := PRChecks{}
	// Latest run per name wins — re-runs of the same check land as new rows.
	latestRun := map[string]ghCheckRun{}
	for _, r := range runs.CheckRuns {
		if r.Name == "" {
			continue
		}
		cur, ok := latestRun[r.Name]
		if !ok || r.StartedAt > cur.StartedAt {
			latestRun[r.Name] = r
		}
	}
	for _, r := range latestRun {
		switch classifyCheckRun(r) {
		case "success":
			out.Success++
		case "failure":
			out.Failure++
			out.Failing = append(out.Failing, PRCheckEntry{Name: r.Name, URL: firstNonEmpty(r.HTMLURL, r.DetailsURL)})
		case "pending":
			out.Pending++
		}
	}
	// Legacy status contexts: combined.statuses is already deduped to
	// the latest per context by GitHub.
	for _, s := range combined.Statuses {
		if s.Context == "" {
			continue
		}
		// Skip if a check-run with the same name covered it.
		if _, dup := latestRun[s.Context]; dup {
			continue
		}
		switch strings.ToLower(s.State) {
		case "success":
			out.Success++
		case "failure", "error":
			out.Failure++
			out.Failing = append(out.Failing, PRCheckEntry{Name: s.Context, URL: s.TargetURL})
		case "pending":
			out.Pending++
		}
	}
	out.Total = out.Success + out.Failure + out.Pending
	sort.Slice(out.Failing, func(i, j int) bool { return out.Failing[i].Name < out.Failing[j].Name })
	return out, nil
}

func classifyCheckRun(r ghCheckRun) string {
	if r.Status != "completed" {
		return "pending"
	}
	switch strings.ToLower(r.Conclusion) {
	case "success", "neutral", "skipped":
		return "success"
	case "failure", "timed_out", "cancelled", "action_required":
		return "failure"
	}
	return "pending"
}

func firstNonEmpty(s ...string) string {
	for _, v := range s {
		if v != "" {
			return v
		}
	}
	return ""
}

// GetJSONPage issues a GET against u and decodes the JSON body into out, like
// GetJSON, and additionally returns the URL of the next page parsed from the
// Link header (or "" when there is none). Sets the standard GitHub API headers
// and bearer auth when tok is non-empty.
func GetJSONPage(ctx context.Context, client *http.Client, tok, u string, out any) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("github api: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("github api: read body: %w", err)
	}
	if resp.StatusCode/100 != 2 {
		return "", fmt.Errorf("github api %s: %s: %s",
			u, resp.Status, strings.TrimSpace(string(body)))
	}
	if err := json.Unmarshal(body, out); err != nil {
		return "", fmt.Errorf("github api: parse: %w", err)
	}
	return ParseNextLink(resp.Header.Get("Link")), nil
}

// GetJSON issues a GET against u and decodes the JSON body into out. Sets the
// standard GitHub API headers and bearer auth when tok is non-empty. Shared
// between the PR poller, PR-detail aggregator, and the build-cop poller.
func GetJSON(ctx context.Context, client *http.Client, tok, u string, out any) error {
	_, err := GetJSONPage(ctx, client, tok, u, out)
	return err
}
