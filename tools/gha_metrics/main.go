
package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"
)

type WorkflowRun struct {
	ID           int64  `json:"id"`
	RunID        int64  `json:"run_id"`
	WorkflowID   int64  `json:"workflow_id"`
	WorkflowName string `json:"name"`
	RunStartedAt string `json:"run_started_at"`
	UpdatedAt    string `json:"updated_at"`
	CreatedAt    string `json:"created_at"`
	Conclusion   string `json:"conclusion"`
	Event        string `json:"event"`
	HeadBranch   string `json:"head_branch"`
}

type Job struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	StartedAt string `json:"started_at"`
	CompletedAt string `json:"completed_at"`
	CreatedAt string `json:"created_at"`
	Conclusion string `json:"conclusion"`
	Status    string `json:"status"`
}

type JobsResponse struct {
	Jobs []Job `json:"jobs"`
}

type Response struct {
	Runs []WorkflowRun `json:"workflow_runs"`
}

type JobMetrics struct {
	Name        string
	BuildTimes  []float64
	WaitTimes   []float64
	Success     int
	Failure     int
	Cancelled   int
	Total       int
}

func main() {
	var owner, repo, token, csvPath string
	flag.StringVar(&owner, "owner", "", "GitHub org or user")
	flag.StringVar(&repo, "repo", "", "Repository name")
	flag.StringVar(&token, "token", os.Getenv("GITHUB_TOKEN"), "GitHub token (env GITHUB_TOKEN)")
	flag.StringVar(&csvPath, "csv", "", "Optional CSV output file path")
	flag.Parse()

	if owner == "" || repo == "" {
		fmt.Fprintln(os.Stderr, "Usage: gha_metrics -owner <org> -repo <repo> [-csv path]")
		os.Exit(1)
	}
	if token == "" {
		var err error
		token, err = getGHToken()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: GitHub token required (failed to get from gh auth token: %v)\n", err)
			os.Exit(1)
		}
	}

	runs, err := fetchRuns(owner, repo, token)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching runs: %v\n", err)
		os.Exit(1)
	}
	if len(runs) == 0 {
		fmt.Println("No workflow runs found.")
		return
	}

	// Fetch jobs for all runs
	fmt.Fprintf(os.Stderr, "Fetching jobs for %d runs...\n", len(runs))
	allJobs := make(map[int64][]Job) // run_id -> jobs
	for _, r := range runs {
		jobs, err := fetchJobs(owner, repo, r.ID, token)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to fetch jobs for run %d: %v\n", r.ID, err)
			continue
		}
		allJobs[r.ID] = jobs
	}

	// Compute overall metrics
	var durations, queueTimes []float64
	var success, failure, cancelled int
	retryCount := 0
	workflowRunCounts := make(map[int64]int) // workflow_id -> count

	// Per-job metrics
	jobMetricsMap := make(map[string]*JobMetrics)

	// Weekly throughput
	weeklyCounts := make(map[string]int)

	for _, r := range runs {
		start, end, created := parseTime(r.RunStartedAt), parseTime(r.UpdatedAt), parseTime(r.CreatedAt)

		// Track retries (multiple runs with same workflow_id)
		workflowRunCounts[r.WorkflowID]++

		// Track weekly throughput
		week := getWeek(created)
		weeklyCounts[week]++

		if !start.IsZero() && !end.IsZero() {
			durations = append(durations, end.Sub(start).Seconds())
			queueTimes = append(queueTimes, start.Sub(created).Seconds())
		}

		switch r.Conclusion {
		case "success":
			success++
		case "failure":
			failure++
		case "cancelled":
			cancelled++
		}

		// Process jobs for this run
		jobs := allJobs[r.ID]
		for _, job := range jobs {
			jobStart := parseTime(job.StartedAt)
			jobEnd := parseTime(job.CompletedAt)
			jobCreated := parseTime(job.CreatedAt)

			if jobMetricsMap[job.Name] == nil {
				jobMetricsMap[job.Name] = &JobMetrics{Name: job.Name}
			}
			jm := jobMetricsMap[job.Name]

			if !jobStart.IsZero() && !jobEnd.IsZero() {
				jm.BuildTimes = append(jm.BuildTimes, jobEnd.Sub(jobStart).Seconds())
			}
			if !jobStart.IsZero() && !jobCreated.IsZero() {
				jm.WaitTimes = append(jm.WaitTimes, jobStart.Sub(jobCreated).Seconds())
			}

			jm.Total++
			switch job.Conclusion {
			case "success":
				jm.Success++
			case "failure":
				jm.Failure++
			case "cancelled":
				jm.Cancelled++
			}
		}
	}

	// Count retries (workflows with more than 1 run)
	for _, count := range workflowRunCounts {
		if count > 1 {
			retryCount += count - 1
		}
	}

	// Print overall metrics
	total := success + failure + cancelled
	fmt.Printf("\n=== OVERALL METRICS ===\n")
	fmt.Printf("Analyzed %d workflow runs\n", total)
	fmt.Printf("Success: %.1f%% | Failure: %.1f%% | Cancelled: %.1f%%\n",
		float64(success)/float64(total)*100,
		float64(failure)/float64(total)*100,
		float64(cancelled)/float64(total)*100)
	fmt.Printf("Failure Rate: %.1f%%\n", float64(failure)/float64(total)*100)
	fmt.Printf("Manual Retries: %d\n", retryCount)

	if len(durations) > 0 {
		fmt.Printf("\nBuild Duration (s): p50=%.2f  p75=%.2f  p90=%.2f\n",
			percentile(durations, 0.50), percentile(durations, 0.75), percentile(durations, 0.90))
	}
	if len(queueTimes) > 0 {
		fmt.Printf("Wait Time (s):     p50=%.2f  p75=%.2f  p90=%.2f\n",
			percentile(queueTimes, 0.50), percentile(queueTimes, 0.75), percentile(queueTimes, 0.90))
	}

	// Print per-job metrics
	fmt.Printf("\n=== PER-JOB METRICS ===\n")
	jobNames := make([]string, 0, len(jobMetricsMap))
	for name := range jobMetricsMap {
		jobNames = append(jobNames, name)
	}
	sort.Strings(jobNames)

	for _, name := range jobNames {
		jm := jobMetricsMap[name]
		if jm.Total == 0 {
			continue
		}
		fmt.Printf("\nJob: %s\n", jm.Name)
		fmt.Printf("  Total: %d | Success: %.1f%% | Failure: %.1f%%\n",
			jm.Total,
			float64(jm.Success)/float64(jm.Total)*100,
			float64(jm.Failure)/float64(jm.Total)*100)
		if len(jm.BuildTimes) > 0 {
			fmt.Printf("  Build Duration (s): p50=%.2f  p75=%.2f  p90=%.2f\n",
				percentile(jm.BuildTimes, 0.50), percentile(jm.BuildTimes, 0.75), percentile(jm.BuildTimes, 0.90))
		}
		if len(jm.WaitTimes) > 0 {
			fmt.Printf("  Wait Time (s):     p50=%.2f  p75=%.2f  p90=%.2f\n",
				percentile(jm.WaitTimes, 0.50), percentile(jm.WaitTimes, 0.75), percentile(jm.WaitTimes, 0.90))
		}
	}

	// Print weekly throughput
	fmt.Printf("\n=== WEEKLY THROUGHPUT ===\n")
	weeks := make([]string, 0, len(weeklyCounts))
	for week := range weeklyCounts {
		weeks = append(weeks, week)
	}
	sort.Strings(weeks)
	for _, week := range weeks {
		fmt.Printf("%s: %d builds\n", week, weeklyCounts[week])
	}

	if csvPath != "" {
		if err := exportCSV(csvPath, runs, allJobs); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing CSV: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("\nCSV exported to %s\n", csvPath)
	}
}

func fetchRuns(owner, repo, token string) ([]WorkflowRun, error) {
	var allRuns []WorkflowRun
	page := 1
	perPage := 100

	for {
		url := fmt.Sprintf("https://api.github.com/repos/%s/%s/actions/runs?status=completed&per_page=%d&page=%d", owner, repo, perPage, page)
		req, _ := http.NewRequest("GET", url, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/vnd.github+json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
		}

		var data Response
		if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
			return nil, err
		}

		if len(data.Runs) == 0 {
			break
		}

		allRuns = append(allRuns, data.Runs...)

		// Check if there are more pages
		if len(data.Runs) < perPage {
			break
		}
		page++
	}

	return allRuns, nil
}

func fetchJobs(owner, repo string, runID int64, token string) ([]Job, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/actions/runs/%d/jobs?per_page=100", owner, repo, runID)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var data JobsResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	return data.Jobs, nil
}

func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

func percentile(data []float64, p float64) float64 {
	if len(data) == 0 {
		return 0
	}
	sort.Float64s(data)
	k := int(float64(len(data))*p + 0.5)
	if k <= 0 {
		return data[0]
	}
	if k >= len(data) {
		return data[len(data)-1]
	}
	return data[k-1]
}

func exportCSV(path string, runs []WorkflowRun, allJobs map[int64][]Job) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	w := csv.NewWriter(file)
	defer w.Flush()

	// Write header
	w.Write([]string{
		"workflow_name", "workflow_id", "run_id", "job_name", "job_id",
		"created_at", "started_at", "completed_at",
		"duration_s", "wait_time_s", "conclusion", "status", "event", "head_branch",
	})

	// Write data
	for _, r := range runs {
		jobs := allJobs[r.ID]
		if len(jobs) == 0 {
			// Write run-level data even if no jobs
			start, end, created := parseTime(r.RunStartedAt), parseTime(r.UpdatedAt), parseTime(r.CreatedAt)
			var duration, waitTime float64
			if !start.IsZero() && !end.IsZero() {
				duration = end.Sub(start).Seconds()
			}
			if !start.IsZero() && !created.IsZero() {
				waitTime = start.Sub(created).Seconds()
			}
			w.Write([]string{
				r.WorkflowName,
				fmt.Sprintf("%d", r.WorkflowID),
				fmt.Sprintf("%d", r.ID),
				"", "", // job name and ID
				r.CreatedAt,
				r.RunStartedAt,
				r.UpdatedAt,
				fmt.Sprintf("%.2f", duration),
				fmt.Sprintf("%.2f", waitTime),
				r.Conclusion,
				"completed",
				r.Event,
				r.HeadBranch,
			})
		} else {
			// Write job-level data
			for _, job := range jobs {
				jobStart := parseTime(job.StartedAt)
				jobEnd := parseTime(job.CompletedAt)
				jobCreated := parseTime(job.CreatedAt)

				var duration, waitTime float64
				if !jobStart.IsZero() && !jobEnd.IsZero() {
					duration = jobEnd.Sub(jobStart).Seconds()
				}
				if !jobStart.IsZero() && !jobCreated.IsZero() {
					waitTime = jobStart.Sub(jobCreated).Seconds()
				}

				w.Write([]string{
					r.WorkflowName,
					fmt.Sprintf("%d", r.WorkflowID),
					fmt.Sprintf("%d", r.ID),
					job.Name,
					fmt.Sprintf("%d", job.ID),
					job.CreatedAt,
					job.StartedAt,
					job.CompletedAt,
					fmt.Sprintf("%.2f", duration),
					fmt.Sprintf("%.2f", waitTime),
					job.Conclusion,
					job.Status,
					r.Event,
					r.HeadBranch,
				})
			}
		}
	}
	return nil
}

func getWeek(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}
	// Get Monday of the week
	weekday := int(t.Weekday())
	if weekday == 0 {
		weekday = 7 // Sunday is 7
	}
	monday := t.AddDate(0, 0, -weekday+1)
	return monday.Format("2006-01-02")
}

func getGHToken() (string, error) {
	cmd := exec.Command("gh", "auth", "token")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}
