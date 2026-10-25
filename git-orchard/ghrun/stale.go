package ghrun

import (
	"context"
	"errors"
	"strconv"
	"strings"
)

// DependabotWorkflow is the workflow GitHub runs Dependabot's updates in. It
// has no file of its own, so it lingers after its configuration is gone.
const DependabotWorkflow = "Dependabot Updates"

// errNotFound is gh api's 404.
var errNotFound = errors.New("not found")

// api runs gh api, mapping a 404 to errNotFound.
func api(ctx context.Context, args ...string) ([]byte, error) {
	out, err := gh(ctx, append([]string{"api"}, args...)...)
	if err != nil && strings.Contains(err.Error(), "(HTTP 404)") {
		return nil, errNotFound
	}
	return out, err
}

// DependabotStale returns why Dependabot no longer updates repo, making its
// runs history, or "" if it still might: branch has a .github/dependabot.yml
// (or .yaml) for version updates, or automated security fixes are on. A
// repository it can't tell about, e.g. for want of admin access to read the
// security fixes setting, gets an error.
func DependabotStale(ctx context.Context, repo, branch string) (string, error) {
	for _, name := range []string{"dependabot.yml", "dependabot.yaml"} {
		_, err := api(ctx, "repos/"+repo+"/contents/.github/"+name+"?ref="+branch, "--silent")
		if err == nil {
			return "", nil
		}
		if !errors.Is(err, errNotFound) {
			return "", err
		}
	}
	out, err := api(ctx, "repos/"+repo+"/automated-security-fixes", "--jq", ".enabled")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(string(out)) != "false" {
		return "", nil
	}
	return "no .github/dependabot.yml and security updates are off", nil
}

// FailedRuns returns the failed runs of workflow in repo, on any branch.
func FailedRuns(ctx context.Context, repo string, workflow int64) ([]Run, error) {
	runs, err := listRuns(ctx, repo, "--workflow", strconv.FormatInt(workflow, 10), "--limit", "1000")
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
	_, err := api(ctx, "repos/"+repo+"/actions/runs/"+strconv.FormatInt(id, 10), "-X", "DELETE", "--silent")
	if errors.Is(err, errNotFound) {
		return nil
	}
	return err
}
