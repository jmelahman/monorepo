package ghrun

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
)

// DependabotWorkflow is the workflow GitHub runs Dependabot's updates in. It
// has no file of its own, so it lingers after its configuration is gone.
const DependabotWorkflow = "Dependabot Updates"

// DependabotStale returns why Dependabot no longer updates repo, making its
// runs history, or "" if it still might: branch has a .github/dependabot.yml
// (or .yaml) for version updates, or automated security fixes are on. A
// repository it can't tell about, e.g. for want of admin access to read the
// security fixes setting, gets an error.
func DependabotStale(ctx context.Context, repo, branch string) (string, error) {
	for _, name := range []string{"dependabot.yml", "dependabot.yaml"} {
		_, _, err := call(ctx, http.MethodGet, "/repos/"+repo+"/contents/.github/"+name+"?ref="+url.QueryEscape(branch))
		if err == nil {
			return "", nil
		}
		if !errors.Is(err, errNotFound) {
			return "", err
		}
	}
	var fixes struct {
		Enabled bool `json:"enabled"`
	}
	if _, err := getJSON(ctx, "/repos/"+repo+"/automated-security-fixes", &fixes); err != nil {
		return "", err
	}
	if fixes.Enabled {
		return "", nil
	}
	return "no .github/dependabot.yml and security updates are off", nil
}

// FailedRuns returns the failed runs of workflow in repo, on any branch.
func FailedRuns(ctx context.Context, repo string, workflow int64) ([]Run, error) {
	runs, err := listRuns(ctx, repo, workflow, url.Values{}, 1000)
	if err != nil {
		return nil, err
	}
	var failed []Run
	for _, r := range runs {
		if r.Failed() {
			failed = append(failed, r)
		}
	}
	return failed, nil
}

// DeleteRun deletes run id of repo. One already gone counts as deleted, so
// trying again after a dropped connection is safe.
func DeleteRun(ctx context.Context, repo string, id int64) error {
	_, _, err := call(ctx, http.MethodDelete, "/repos/"+repo+"/actions/runs/"+strconv.FormatInt(id, 10))
	if errors.Is(err, errNotFound) {
		return nil
	}
	return err
}
