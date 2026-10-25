// Package githubapp creates the GitHub App the upgrade workflow opens pull
// requests as, with GitHub's manifest flow
// (https://docs.github.com/en/apps/sharing-github-apps/registering-a-github-app-from-a-manifest).
//
// A local server hands the browser a form that posts the manifest to GitHub.
// Once the user confirms, GitHub redirects back with a code, which is
// exchanged for the new App's credentials, and the browser is sent on to
// install it.
package githubapp

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var (
	// ErrNoCode is returned when GitHub redirects back without a code,
	// as when the user cancels.
	ErrNoCode = errors.New("GitHub redirected back without a code")
	// ErrNoCredentials is returned when GitHub's conversion response lacks
	// the client ID or private key.
	ErrNoCredentials = errors.New("GitHub's response had no client ID or private key")
)

// Permissions are what the upgrade workflow needs: pushing its branch and
// opening a pull request from it. A PR opened as an App, unlike one opened
// with GITHUB_TOKEN, triggers the repository's other workflows.
var Permissions = map[string]string{
	"contents":      "write",
	"metadata":      "read",
	"pull_requests": "write",
}

const home = "https://github.com/jmelahman/vinder"

// Manifest is a GitHub App manifest.
type Manifest struct {
	DefaultPermissions map[string]string `json:"default_permissions"`
	DefaultEvents      []string          `json:"default_events"`
	HookAttributes     HookAttributes    `json:"hook_attributes"`
	Name               string            `json:"name"`
	URL                string            `json:"url"`
	Description        string            `json:"description,omitempty"`
	RedirectURL        string            `json:"redirect_url"`
	Public             bool              `json:"public"`
}

// HookAttributes configures the App's webhook, which vinder doesn't use.
type HookAttributes struct {
	URL    string `json:"url"`
	Active bool   `json:"active"`
}

// NewManifest describes a private App named name that can open upgrade
// pull requests.
func NewManifest(name, redirectURL string) Manifest {
	return Manifest{
		Name:               name,
		URL:                home,
		Description:        "Opens pull requests that upgrade vendored npm dependencies with vinder.",
		HookAttributes:     HookAttributes{URL: home},
		RedirectURL:        redirectURL,
		DefaultPermissions: Permissions,
		DefaultEvents:      []string{},
	}
}

// Credentials are the new App's identity and private key.
type Credentials struct {
	Slug     string `json:"slug"`
	ClientID string `json:"client_id"`
	PEM      string `json:"pem"`
	HTMLURL  string `json:"html_url"`
	ID       int64  `json:"id"`
}

// InstallURL is where the App is installed on repositories.
func (c Credentials) InstallURL() string {
	return c.HTMLURL + "/installations/new"
}

// Options configure Create.
type Options struct {
	// Open shows the user a URL, e.g. by starting a browser.
	Open func(url string) error
	// HTTPClient exchanges the code; it defaults to http.DefaultClient.
	HTTPClient *http.Client
	// Name is the App's name, which must be unique on GitHub. The user can
	// still change it before confirming.
	Name string
	// Org creates the App under an organization instead of the user.
	Org string
	// WebURL and APIURL default to github.com's.
	WebURL string
	APIURL string
}

func (o Options) withDefaults() Options {
	if o.WebURL == "" {
		o.WebURL = "https://github.com"
	}
	if o.APIURL == "" {
		o.APIURL = "https://api.github.com"
	}
	if o.HTTPClient == nil {
		o.HTTPClient = http.DefaultClient
	}
	return o
}

// NewAppURL is where the manifest is posted.
func (o Options) NewAppURL() string {
	o = o.withDefaults()
	if o.Org != "" {
		return fmt.Sprintf("%s/organizations/%s/settings/apps/new", o.WebURL, url.PathEscape(o.Org))
	}
	return o.WebURL + "/settings/apps/new"
}

var formPage = template.Must(template.New("form").Parse(`<!doctype html>
<title>vinder</title>
<p>Sending you to GitHub to create the App…</p>
<form id="f" method="post" action="{{.Action}}">
<input type="hidden" name="manifest" value="{{.Manifest}}">
<noscript><button>Continue to GitHub</button></noscript>
</form>
<script>document.getElementById("f").submit()</script>
`))

type result struct {
	err   error
	creds Credentials
}

// Create runs the manifest flow and returns the new App's credentials. It
// returns once GitHub redirects back, or when ctx is done.
func Create(ctx context.Context, opts Options) (Credentials, error) {
	opts = opts.withDefaults()

	ln, err := (&net.ListenConfig{}).Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		return Credentials{}, err
	}
	base := "http://" + ln.Addr().String()
	state := rand.Text()
	manifest, err := json.Marshal(NewManifest(opts.Name, base+"/callback"))
	if err != nil {
		return Credentials{}, err
	}

	done := make(chan result, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = formPage.Execute(w, map[string]string{
			"Action":   opts.NewAppURL() + "?state=" + url.QueryEscape(state),
			"Manifest": string(manifest),
		})
	})
	mux.HandleFunc("GET /callback", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("state") != state {
			http.Error(w, "state mismatch; start over with vinder github-app", http.StatusBadRequest)
			return
		}
		creds, err := Convert(r.Context(), opts.HTTPClient, opts.APIURL, q.Get("code"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
		} else {
			http.Redirect(w, r, creds.InstallURL(), http.StatusFound)
		}
		select {
		case done <- result{creds: creds, err: err}:
		default:
		}
	})

	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = srv.Serve(ln) }()
	defer func() {
		shutdown, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
	}()

	if opts.Open != nil {
		if err := opts.Open(base + "/"); err != nil {
			return Credentials{}, err
		}
	}

	select {
	case res := <-done:
		return res.creds, res.err
	case <-ctx.Done():
		return Credentials{}, ctx.Err()
	}
}

// Convert exchanges the code GitHub redirects back with for the App's
// credentials. The code is single-use and expires after an hour.
func Convert(ctx context.Context, client *http.Client, apiURL, code string) (Credentials, error) {
	if code == "" {
		return Credentials{}, ErrNoCode
	}
	endpoint := fmt.Sprintf("%s/app-manifests/%s/conversions", strings.TrimSuffix(apiURL, "/"), url.PathEscape(code))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return Credentials{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	resp, err := client.Do(req)
	if err != nil {
		return Credentials{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return Credentials{}, err
	}
	if resp.StatusCode != http.StatusCreated {
		return Credentials{}, fmt.Errorf("converting the App manifest: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var creds Credentials
	if err := json.Unmarshal(body, &creds); err != nil {
		return Credentials{}, err
	}
	if creds.PEM == "" || creds.ClientID == "" {
		return Credentials{}, ErrNoCredentials
	}
	return creds, nil
}

var githubRepo = regexp.MustCompile(`^(?:git@github\.com:|ssh://git@github\.com/|https://github\.com/)([^/]+)/([^/]+?)(?:\.git)?/?$`)

// ParseRepo returns the owner and name of a github.com remote URL.
func ParseRepo(remote string) (owner, repo string, ok bool) {
	m := githubRepo.FindStringSubmatch(remote)
	if m == nil {
		return "", "", false
	}
	return m[1], m[2], true
}
