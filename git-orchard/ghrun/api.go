package ghrun

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// perPage is the most the API returns in a page.
const perPage = 100

// client bounds each request, for callers whose context has no deadline. It's
// long enough for a job's whole log.
var client = &http.Client{Timeout: 2 * time.Minute}

// SetTransport sets how requests are sent, e.g. to count them. Call it before
// making any.
func SetTransport(rt http.RoundTripper) {
	client.Transport = rt
}

// ghAuthToken asks gh, for those signed in with it. It's only asked once, since
// it costs a process.
var ghAuthToken = sync.OnceValue(func() string {
	out, err := exec.Command("gh", "auth", "token", "--hostname", "github.com").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
})

// Token returns the token for github.com: $GH_TOKEN, $GITHUB_TOKEN, or gh auth
// token where gh is installed and signed in; "" if there's none.
func Token() string {
	if token := cmp.Or(os.Getenv("GH_TOKEN"), os.Getenv("GITHUB_TOKEN")); token != "" {
		return token
	}
	return ghAuthToken()
}

// apiBase is $GITHUB_API_URL, which GitHub Actions also sets, or github.com's.
func apiBase() string {
	return strings.TrimRight(cmp.Or(os.Getenv("GITHUB_API_URL"), "https://api.github.com"), "/")
}

// errNotFound is the API's 404.
var errNotFound = errors.New("not found")

// apiError is a response the API refused or failed.
type apiError struct {
	method, path string
	status       int
	message      string
}

func (e *apiError) Error() string {
	return fmt.Sprintf("GitHub API %s %s: HTTP %d: %s", e.method, e.path, e.status, e.message)
}

// call makes a request of the API, at a path or a URL from a Link header, and
// returns the body and the next page's URL. The requests only read, or delete
// (where a second try finds nothing to delete), so a transient failure is
// tried again once.
func call(ctx context.Context, method, target string) (body []byte, next string, err error) {
	for attempt := range 2 {
		if attempt > 0 {
			if err := Sleep(ctx, time.Second); err != nil {
				return nil, "", err
			}
		}
		var retry bool
		body, next, retry, err = callOnce(ctx, method, target)
		if err == nil {
			return body, next, nil
		}
		if ctx.Err() != nil {
			return nil, "", ctx.Err()
		}
		if !retry {
			break
		}
	}
	return nil, "", err
}

func callOnce(ctx context.Context, method, target string) (body []byte, next string, retry bool, err error) {
	u := target
	if strings.HasPrefix(target, "/") {
		u = apiBase() + target
	}
	req, err := http.NewRequestWithContext(ctx, method, u, nil)
	if err != nil {
		return nil, "", false, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	// A redirect to another host, as logs get, drops this.
	if token := Token(); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", true, fmt.Errorf("GitHub API %s %s: %w", method, req.URL.Path, err)
	}
	defer resp.Body.Close()
	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", true, fmt.Errorf("GitHub API %s %s: %w", method, req.URL.Path, err)
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, "", false, fmt.Errorf("GitHub API %s %s: %w", method, req.URL.Path, errNotFound)
	}
	if resp.StatusCode/100 != 2 {
		e := &apiError{method: method, path: req.URL.Path, status: resp.StatusCode, message: errorMessage(body)}
		return nil, "", resp.StatusCode/100 == 5, e
	}
	return body, nextLink(resp.Header.Get("Link")), false, nil
}

// errorMessage is the message of an error response, which is usually JSON.
func errorMessage(body []byte) string {
	var e struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(body, &e) == nil && e.Message != "" {
		return e.Message
	}
	return strings.TrimSpace(string(body))
}

// nextLink returns the rel="next" URL of a Link header, or "".
func nextLink(header string) string {
	for link := range strings.SplitSeq(header, ",") {
		target, params, ok := strings.Cut(strings.TrimSpace(link), ";")
		if ok && strings.Contains(params, `rel="next"`) {
			return strings.Trim(strings.TrimSpace(target), "<>")
		}
	}
	return ""
}

// getJSON decodes a response into out, and returns the next page's URL.
func getJSON(ctx context.Context, target string, out any) (next string, err error) {
	body, next, err := call(ctx, http.MethodGet, target)
	if err != nil {
		return "", err
	}
	if err := json.Unmarshal(body, out); err != nil {
		return "", fmt.Errorf("GitHub API GET %s: %w", target, err)
	}
	return next, nil
}

// apiRun is a workflow run, as the API describes it.
type apiRun struct {
	ID         int64     `json:"id"`
	WorkflowID int64     `json:"workflow_id"`
	Name       string    `json:"name"`
	HeadBranch string    `json:"head_branch"`
	HeadSHA    string    `json:"head_sha"`
	Status     string    `json:"status"`
	Conclusion string    `json:"conclusion"`
	URL        string    `json:"html_url"`
	Event      string    `json:"event"`
	Title      string    `json:"display_title"`
	CreatedAt  time.Time `json:"created_at"`
}

func (r apiRun) run() Run {
	return Run{
		ID:         r.ID,
		WorkflowID: r.WorkflowID,
		Workflow:   r.Name,
		HeadBranch: r.HeadBranch,
		HeadSHA:    r.HeadSHA,
		Status:     r.Status,
		Conclusion: r.Conclusion,
		URL:        r.URL,
		Event:      r.Event,
		Title:      r.Title,
		CreatedAt:  r.CreatedAt,
	}
}

// listRuns returns repo's newest runs matching query, up to limit, of workflow
// unless that's 0.
func listRuns(ctx context.Context, repo string, workflow int64, query url.Values, limit int) ([]Run, error) {
	path := "/repos/" + repo + "/actions/runs"
	if workflow != 0 {
		path = "/repos/" + repo + "/actions/workflows/" + strconv.FormatInt(workflow, 10) + "/runs"
	}
	query.Set("per_page", strconv.Itoa(perPage))
	runs := []Run{}
	for target := path + "?" + query.Encode(); target != "" && len(runs) < limit; {
		var page struct {
			Runs []apiRun `json:"workflow_runs"`
		}
		next, err := getJSON(ctx, target, &page)
		if err != nil {
			return nil, err
		}
		for _, r := range page.Runs {
			if len(runs) < limit {
				runs = append(runs, r.run())
			}
		}
		target = next
	}
	return runs, nil
}

// listWorkflows returns repo's first page of workflows.
func listWorkflows(ctx context.Context, repo string) ([]Workflow, error) {
	var page struct {
		Workflows []Workflow `json:"workflows"`
	}
	_, err := getJSON(ctx, "/repos/"+repo+"/actions/workflows?per_page="+strconv.Itoa(perPage), &page)
	return page.Workflows, err
}

// failedJob is a job of a run that failed.
type failedJob struct {
	ID   int64
	Name string
}

// failedJobs returns the failed jobs of run id, from its latest attempt.
func failedJobs(ctx context.Context, repo string, id int64) ([]failedJob, error) {
	var jobs []failedJob
	target := "/repos/" + repo + "/actions/runs/" + strconv.FormatInt(id, 10) + "/jobs?filter=latest&per_page=" + strconv.Itoa(perPage)
	for target != "" {
		var page struct {
			Jobs []struct {
				ID         int64  `json:"id"`
				Name       string `json:"name"`
				Conclusion string `json:"conclusion"`
			} `json:"jobs"`
		}
		next, err := getJSON(ctx, target, &page)
		if err != nil {
			return nil, err
		}
		for _, j := range page.Jobs {
			if failedConclusion(j.Conclusion) {
				jobs = append(jobs, failedJob{ID: j.ID, Name: j.Name})
			}
		}
		target = next
	}
	return jobs, nil
}
