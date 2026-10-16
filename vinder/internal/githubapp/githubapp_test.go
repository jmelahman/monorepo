package githubapp_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"testing"
	"time"

	"github.com/jmelahman/vinder/internal/githubapp"
)

// githubAPI answers manifest conversions for code "good".
func githubAPI(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/app-manifests/good/conversions" {
			http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(githubapp.Credentials{
			ID: 1, Slug: "me-vinder", ClientID: "Iv1.abc", PEM: "key",
			HTMLURL: "https://github.com/apps/me-vinder",
		})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestConvert(t *testing.T) {
	t.Parallel()

	api := githubAPI(t)
	tests := []struct {
		wantIs  error
		name    string
		code    string
		want    string
		wantErr bool
	}{
		{name: "good", code: "good", want: "Iv1.abc"},
		{name: "rejected", code: "expired", wantErr: true},
		{name: "no code", code: "", wantErr: true, wantIs: githubapp.ErrNoCode},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			creds, err := githubapp.Convert(context.Background(), api.Client(), api.URL, tt.code)
			if (err != nil) != tt.wantErr || tt.wantIs != nil && !errors.Is(err, tt.wantIs) {
				t.Fatalf("Convert(%q) error = %v, wantErr %v (%v)", tt.code, err, tt.wantErr, tt.wantIs)
			}
			if creds.ClientID != tt.want {
				t.Errorf("Convert(%q) client ID = %q, want %q", tt.code, creds.ClientID, tt.want)
			}
		})
	}
}

func TestNewAppURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		org  string
		want string
	}{
		{org: "", want: "https://github.com/settings/apps/new"},
		{org: "acme", want: "https://github.com/organizations/acme/settings/apps/new"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()

			if got := (githubapp.Options{Org: tt.org}).NewAppURL(); got != tt.want {
				t.Errorf("NewAppURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseRepo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		remote string
		owner  string
		repo   string
		ok     bool
	}{
		{remote: "git@github.com:jmelahman/monorepo.git", owner: "jmelahman", repo: "monorepo", ok: true},
		{remote: "https://github.com/jmelahman/monorepo", owner: "jmelahman", repo: "monorepo", ok: true},
		{remote: "ssh://git@github.com/jmelahman/monorepo.git", owner: "jmelahman", repo: "monorepo", ok: true},
		{remote: "git@gitlab.com:jmelahman/monorepo.git"},
	}
	for _, tt := range tests {
		t.Run(tt.remote, func(t *testing.T) {
			t.Parallel()

			owner, repo, ok := githubapp.ParseRepo(tt.remote)
			if owner != tt.owner || repo != tt.repo || ok != tt.ok {
				t.Errorf("ParseRepo(%q) = %q, %q, %v; want %q, %q, %v", tt.remote, owner, repo, ok, tt.owner, tt.repo, tt.ok)
			}
		})
	}
}

var (
	actionAttr   = regexp.MustCompile(`action="([^"]+)"`)
	manifestAttr = regexp.MustCompile(`name="manifest" value="([^"]+)"`)
)

// TestCreate plays the browser and GitHub: it loads the form, checks the
// manifest, and follows GitHub's redirect back.
func TestCreate(t *testing.T) {
	t.Parallel()

	api := githubAPI(t)
	browser := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

	redirects := make(chan *http.Response, 1)
	open := func(page string) error {
		resp, err := browser.Get(page)
		if err != nil {
			return err
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		action, err := url.Parse(html.UnescapeString(actionAttr.FindStringSubmatch(string(body))[1]))
		if err != nil {
			return err
		}
		if action.Path != "/organizations/acme/settings/apps/new" {
			return fmt.Errorf("form posts to %s", action)
		}
		var m githubapp.Manifest
		if err := json.Unmarshal([]byte(html.UnescapeString(manifestAttr.FindStringSubmatch(string(body))[1])), &m); err != nil {
			return err
		}
		if m.Name != "acme-vinder" || m.Public || m.DefaultPermissions["pull_requests"] != "write" || m.DefaultPermissions["contents"] != "write" || m.HookAttributes.Active {
			return fmt.Errorf("unexpected manifest %+v", m)
		}

		// A forged callback is refused.
		resp, err = browser.Get(m.RedirectURL + "?code=good&state=forged")
		if err != nil {
			return err
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			return fmt.Errorf("forged state got %s", resp.Status)
		}

		// GitHub runs the callback in the background of the user's browser.
		go func() {
			q := url.Values{"code": {"good"}, "state": {action.Query().Get("state")}}
			resp, err := browser.Get(m.RedirectURL + "?" + q.Encode())
			if err != nil {
				t.Error(err)
				redirects <- nil
				return
			}
			resp.Body.Close()
			redirects <- resp
		}()
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	creds, err := githubapp.Create(ctx, githubapp.Options{Name: "acme-vinder", Org: "acme", APIURL: api.URL, Open: open, HTTPClient: api.Client()})
	if err != nil {
		t.Fatal(err)
	}
	if creds.ClientID != "Iv1.abc" {
		t.Errorf("Create() = %+v", creds)
	}
	if redirect := <-redirects; redirect == nil || redirect.Header.Get("Location") != creds.InstallURL() {
		t.Errorf("browser should be sent to the install page, got %+v", redirect)
	}
}
