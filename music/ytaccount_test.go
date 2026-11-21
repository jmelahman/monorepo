package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testCookies = "# Netscape HTTP Cookie File\n" +
	".youtube.com\tTRUE\t/\tTRUE\t1893456000\tSID\tsecret\n" +
	"#HttpOnly_.youtube.com\tTRUE\t/\tTRUE\t1893456000\tHSID\tsecret\n"

func TestParsePlaylist(t *testing.T) {
	const id = "RDTMAK5uy_kset8DisdE7LSD4TNjEVvrKRTmG7a56sY"
	for _, in := range []string{
		id,
		" " + id + "\n",
		"https://music.youtube.com/playlist?list=" + id,
		"https://music.youtube.com/playlist?list=" + id + "&si=abc",
		"https://music.youtube.com/watch?v=dQw4w9WgXcQ&list=" + id,
	} {
		if got, err := parsePlaylist(in); err != nil || got != id {
			t.Errorf("%q: got %q, %v", in, got, err)
		}
	}
	for _, in := range []string{"", "short", "https://music.youtube.com/watch?v=dQw4w9WgXcQ", "not an id; rm -rf"} {
		if _, err := parsePlaylist(in); err == nil {
			t.Errorf("%q: want an error", in)
		}
	}
}

func TestCheckCookies(t *testing.T) {
	if err := checkCookies([]byte(testCookies)); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{
		"",
		"# Netscape HTTP Cookie File\n.example.com\tTRUE\t/\tTRUE\t0\tA\tb\n",
		`{"cookies": []}`,
	} {
		if checkCookies([]byte(bad)) == nil {
			t.Errorf("%q: want an error", bad)
		}
	}
}

// What the panel saves wins over the environment, and taking it away
// goes back to the environment's.
func TestYTAccountRoutes(t *testing.T) {
	state := t.TempDir()
	envCookies := filepath.Join(t.TempDir(), "env-cookies.txt")
	if err := os.WriteFile(envCookies, nil, 0o644); err != nil { // the compose file's empty mount
		t.Fatal(err)
	}
	m := &mixer{state: state, cookies: envCookies, supermix: "PLenvironment0"}
	mux := http.NewServeMux()
	routeMixes(mux, m)
	do := func(method, path, body string) (int, ytAccount) {
		t.Helper()
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)))
		var a ytAccount
		if rec.Code == http.StatusOK {
			if err := json.Unmarshal(rec.Body.Bytes(), &a); err != nil {
				t.Fatal(err)
			}
		}
		return rec.Code, a
	}

	if _, a := do(http.MethodGet, "/api/yt/account", ""); a.Ready || a.Cookies != "" || a.SupermixFrom != "env" {
		t.Fatalf("before: %+v", a)
	}
	if code, _ := do(http.MethodPut, "/api/yt/account/cookies", "not cookies"); code != http.StatusBadRequest {
		t.Fatalf("want 400 for a file that isn't a cookies.txt, got %d", code)
	}
	_, a := do(http.MethodPut, "/api/yt/account/cookies", testCookies)
	if !a.Ready || a.Cookies != "saved" || a.Supermix != "PLenvironment0" {
		t.Fatalf("after the upload: %+v", a)
	}
	if fi, err := os.Stat(filepath.Join(state, ytCookiesFile)); err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("the cookies should be readable by the server alone: %v %v", fi.Mode(), err)
	}
	if !m.accountSet() {
		t.Fatal("the account isn't set up after the upload")
	}

	link := `{"supermix": "https://music.youtube.com/playlist?list=RDTMAK5uy_saved123456"}`
	if _, a = do(http.MethodPut, "/api/yt/account/supermix", link); a.Supermix != "RDTMAK5uy_saved123456" || a.SupermixFrom != "saved" {
		t.Fatalf("after the link: %+v", a)
	}
	if code, _ := do(http.MethodPut, "/api/yt/account/supermix", `{"supermix": "nope"}`); code != http.StatusBadRequest {
		t.Fatalf("want 400 for a bad link, got %d", code)
	}
	if _, a = do(http.MethodPut, "/api/yt/account/supermix", `{"supermix": ""}`); a.Supermix != "PLenvironment0" || a.SupermixFrom != "env" {
		t.Fatalf("after clearing the link: %+v", a)
	}
	if _, a = do(http.MethodDelete, "/api/yt/account/cookies", ""); a.Ready || a.Cookies != "" {
		t.Fatalf("after removing the cookies: %+v", a)
	}
}
