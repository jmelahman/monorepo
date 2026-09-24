package githubapp

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"testing"
	"time"
)

// fakeGitHub answers manifest conversions for code "good".
func fakeGitHub(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/app-manifests/good/conversions" {
			http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(Credentials{
			ID: 1, Slug: "me-git-orchard", ClientID: "Iv1.abc", PEM: "fake key",
			HTMLURL: "https://github.com/apps/me-git-orchard",
		})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestConvert(t *testing.T) {
	api := fakeGitHub(t)
	creds, err := Convert(context.Background(), api.Client(), api.URL, "good")
	if err != nil {
		t.Fatal(err)
	}
	if creds.ClientID != "Iv1.abc" || creds.InstallURL() != "https://github.com/apps/me-git-orchard/installations/new" {
		t.Errorf("got %+v", creds)
	}
	if _, err := Convert(context.Background(), api.Client(), api.URL, "expired"); err == nil {
		t.Error("expected an error for a rejected code")
	}
}

func TestNewAppURL(t *testing.T) {
	o := Options{}
	o.defaults()
	if got := o.NewAppURL(); got != "https://github.com/settings/apps/new" {
		t.Error(got)
	}
	o.Org = "acme"
	if got := o.NewAppURL(); got != "https://github.com/organizations/acme/settings/apps/new" {
		t.Error(got)
	}
}

var (
	actionAttr   = regexp.MustCompile(`action="([^"]+)"`)
	manifestAttr = regexp.MustCompile(`name="manifest" value="([^"]+)"`)
)

// TestCreate plays the browser and GitHub: it loads the form, checks the
// manifest, and follows GitHub's redirect back.
func TestCreate(t *testing.T) {
	api := fakeGitHub(t)
	browser := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

	redirects := make(chan *http.Response, 1)
	open := func(page string) error {
		resp, err := browser.Get(page)
		if err != nil {
			return err
		}
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()

		action, err := url.Parse(html.UnescapeString(actionAttr.FindStringSubmatch(string(body))[1]))
		if err != nil {
			return err
		}
		if action.Path != "/organizations/acme/settings/apps/new" {
			return fmt.Errorf("form posts to %s", action)
		}
		var m Manifest
		if err := json.Unmarshal([]byte(html.UnescapeString(manifestAttr.FindStringSubmatch(string(body))[1])), &m); err != nil {
			return err
		}
		if m.Name != "acme-git-orchard" || m.Public || m.DefaultPermissions["workflows"] != "write" || m.HookAttributes.Active {
			return fmt.Errorf("unexpected manifest %+v", m)
		}

		// A forged callback is refused.
		resp, err = browser.Get(m.RedirectURL + "?code=good&state=forged")
		if err != nil {
			return err
		}
		_ = resp.Body.Close()
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
			_ = resp.Body.Close()
			redirects <- resp
		}()
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	creds, err := Create(ctx, Options{Name: "acme-git-orchard", Org: "acme", APIURL: api.URL, Open: open, HTTPClient: api.Client()})
	if err != nil {
		t.Fatal(err)
	}
	if creds.ClientID != "Iv1.abc" {
		t.Errorf("got %+v", creds)
	}
	if redirect := <-redirects; redirect == nil || redirect.Header.Get("Location") != creds.InstallURL() {
		t.Errorf("browser should be sent to the install page, got %+v", redirect)
	}
}
