package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/jmelahman/local-preview/internal/build"
	"github.com/jmelahman/local-preview/internal/db"
	"github.com/jmelahman/local-preview/internal/watch"
)

// maxWebhookBody bounds webhook payload reads; GitHub caps deliveries at
// 25 MB.
const maxWebhookBody = 25 << 20

// zeroSHA is the "after" value of a push that deletes its ref.
const zeroSHA = "0000000000000000000000000000000000000000"

// handleGitHubWebhook turns GitHub push events into deploy requests. The
// endpoint only exists once a shared secret is configured; every delivery
// must carry a valid X-Hub-Signature-256 over the raw body. Pushed commits
// deploy by sha — exactly what the event named, even if the branch has
// moved on by the time we fetch.
func (d Deps) handleGitHubWebhook(w http.ResponseWriter, r *http.Request) {
	if d.GitHubWebhookSecret == "" {
		httpError(w, http.StatusServiceUnavailable,
			"github webhooks are disabled: start the server with a webhook secret")
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxWebhookBody))
	if err != nil {
		httpError(w, http.StatusRequestEntityTooLarge, "payload too large")
		return
	}
	if !validSignature(d.GitHubWebhookSecret, r.Header.Get("X-Hub-Signature-256"), body) {
		httpError(w, http.StatusForbidden, "invalid or missing X-Hub-Signature-256")
		return
	}

	switch event := r.Header.Get("X-GitHub-Event"); event {
	case "ping":
		writeJSON(w, http.StatusOK, map[string]string{"status": "pong"})
		return
	case "push":
	default:
		ignore(w, "unhandled event "+event)
		return
	}

	var ev struct {
		Ref     string `json:"ref"`
		After   string `json:"after"`
		Deleted bool   `json:"deleted"`
		Sender  struct {
			Login string `json:"login"`
		} `json:"sender"`
		Repository struct {
			FullName string `json:"full_name"`
			CloneURL string `json:"clone_url"`
			GitURL   string `json:"git_url"`
			SSHURL   string `json:"ssh_url"`
			HTMLURL  string `json:"html_url"`
		} `json:"repository"`
	}
	if err := json.Unmarshal(body, &ev); err != nil {
		httpError(w, http.StatusBadRequest, "invalid JSON payload")
		return
	}
	branch, isBranch := strings.CutPrefix(ev.Ref, "refs/heads/")
	if !isBranch {
		ignore(w, "not a branch push: "+ev.Ref)
		return
	}
	if ev.Deleted || ev.After == zeroSHA || ev.After == "" {
		ignore(w, "branch "+branch+" deleted")
		return
	}

	repo, ok, err := d.repoForPush(
		ev.Repository.CloneURL, ev.Repository.GitURL, ev.Repository.SSHURL, ev.Repository.HTMLURL)
	if err != nil {
		internalError(w, "match webhook repo", err)
		return
	}
	if !ok {
		httpError(w, http.StatusNotFound,
			"no registered repo matches "+ev.Repository.FullName+" — register it with its clone URL as source")
		return
	}
	if !watch.MatchBranch(watch.SplitPatterns(repo.WatchBranches), branch) {
		ignore(w, "branch "+branch+" excluded by filter")
		return
	}
	row, err := d.Queue.RequestDeploy(r.Context(), repo.Name, ev.After, false, ev.Sender.Login)
	if errors.Is(err, build.ErrRepoNotReady) {
		httpError(w, http.StatusConflict, err.Error())
		return
	}
	if err != nil {
		httpError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, d.deployJSON(row))
}

// ignore acknowledges a delivery without acting on it. Always 200: the
// reason lands in GitHub's delivery log, and a non-error status keeps the
// webhook from showing as failing for events we deliberately skip.
func ignore(w http.ResponseWriter, reason string) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ignored", "reason": reason})
}

// validSignature checks GitHub's HMAC-SHA256 signature header
// ("sha256=<hex>") over the raw body.
func validSignature(secret, header string, body []byte) bool {
	hexSig, ok := strings.CutPrefix(header, "sha256=")
	if !ok {
		return false
	}
	got, err := hex.DecodeString(hexSig)
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hmac.Equal(mac.Sum(nil), got)
}

// repoForPush finds the registered repo whose source names the same
// repository as any of the payload's URLs.
func (d Deps) repoForPush(urls ...string) (db.Repo, bool, error) {
	repos, err := d.Store.ListRepos()
	if err != nil {
		return db.Repo{}, false, err
	}
	for _, repo := range repos {
		src := normalizeGitURL(repo.Source)
		for _, u := range urls {
			if u != "" && normalizeGitURL(u) == src {
				return repo, true, nil
			}
		}
	}
	return db.Repo{}, false, nil
}

// normalizeGitURL reduces a clone URL to "host/path" so the same repository
// written as https, git, ssh, or scp-like syntax compares equal:
// "git@github.com:acme/app.git" == "https://github.com/acme/app".
// Non-URL sources (local paths) pass through mostly untouched.
func normalizeGitURL(u string) string {
	u = strings.TrimSuffix(strings.TrimSpace(u), "/")
	u = strings.TrimSuffix(u, ".git")
	scheme := false
	for _, p := range []string{"https://", "http://", "git://", "ssh://"} {
		if rest, ok := strings.CutPrefix(u, p); ok {
			u, scheme = rest, true
			break
		}
	}
	// user@host[:path] — drop the user; an scp-like colon separates the path.
	if at := strings.Index(u, "@"); at >= 0 {
		u = u[at+1:]
		if !scheme {
			u = strings.Replace(u, ":", "/", 1)
		}
	}
	return strings.ToLower(u)
}
