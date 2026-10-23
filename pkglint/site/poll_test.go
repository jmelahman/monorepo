package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// pollServer serves an RSS list of names and an RPC answer, and 404s every
// snapshot.
func pollServer(t *testing.T, rss string, results []map[string]any) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/rss":
			w.Header().Set("Content-Type", "application/rss+xml")
			fmt.Fprint(w, rss)
		case r.URL.Path == "/rpc":
			// Answer only for the names asked about, as the RPC does.
			asked := map[string]bool{}
			for _, n := range r.URL.Query()["arg[]"] {
				asked[n] = true
			}
			var out []map[string]any
			for _, res := range results {
				if asked[res["Name"].(string)] {
					out = append(out, res)
				}
			}
			json.NewEncoder(w).Encode(map[string]any{"type": "multiinfo", "results": out})
		default:
			http.NotFound(w, r)
		}
	}))
	prevRSS, prevInfo, prevSnap := aurModifiedURL, aurInfoURL, snapshotURL
	aurModifiedURL, aurInfoURL, snapshotURL = srv.URL+"/rss", srv.URL+"/rpc", srv.URL+"/snap/%s.tar.gz"
	t.Cleanup(func() {
		srv.Close()
		aurModifiedURL, aurInfoURL, snapshotURL = prevRSS, prevInfo, prevSnap
	})
	return srv
}

func rssOf(names ...string) string {
	var b strings.Builder
	b.WriteString(`<?xml version='1.0' encoding='UTF-8'?><rss version="2.0"><channel><title>AUR</title>`)
	for _, n := range names {
		fmt.Fprintf(&b, "<item><title>%s</title><guid>%s-1</guid></item>", n, n)
	}
	b.WriteString("</channel></rss>")
	return b.String()
}

func TestPollAUR(t *testing.T) {
	tmp := t.TempDir()
	cache, out := filepath.Join(tmp, "cache"), filepath.Join(tmp, "out")
	statePath, feedPath := filepath.Join(tmp, "state.jsonl"), filepath.Join(tmp, "feed.jsonl")
	officialPath := filepath.Join(tmp, "official.txt")
	lm := time.Now().Add(-time.Hour).Unix()

	// evil's snapshot is already in the cache, so it scans without a fetch;
	// broken's is not, and the server 404s it.
	dir := filepath.Join(cache, "snapshots", fmt.Sprintf("evil@%d", lm))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	pkgbuild := "pkgname=(evil evil-docs)\npkgbase=evil\npkgver=1.0\npkgrel=1\narch=('any')\ncurl -s https://example.com/x\n"
	if err := os.WriteFile(filepath.Join(dir, "PKGBUILD"), []byte(pkgbuild), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := saveState(statePath, map[string]stateRecord{
		"same":   {Base: "same", LastModified: 500, Rules: rulesFingerprint()},
		"shadow": {Base: "shadow", Repo: "extra", LastModified: 1},
	}); err != nil {
		t.Fatal(err)
	}

	// linux is official but the state has not reached it: the list alone
	// keeps the poll off its AUR namesake.
	if err := saveOfficialBases(officialPath, []metaPackage{{PackageBase: "linux"}, {PackageBase: "shadow"}}); err != nil {
		t.Fatal(err)
	}

	pkg := func(name, base string, lm int64) map[string]any {
		return map[string]any{"Name": name, "PackageBase": base, "Version": "1.0-1", "LastModified": lm}
	}
	pollServer(t, rssOf("evil", "evil-docs", "same", "shadow", "linux", "broken", "../up", "evil"), []map[string]any{
		pkg("evil", "evil", lm), pkg("evil-docs", "evil", lm),
		pkg("same", "same", 500), pkg("shadow", "shadow", 2), pkg("linux", "linux", lm), pkg("broken", "broken", lm),
	})

	if err := pollAUR(out, cache, statePath, feedPath, officialPath); err != nil {
		t.Fatalf("pollAUR: %v", err)
	}
	state, err := loadState(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if rec, ok := state["evil"]; !ok || rec.LastModified != lm {
		t.Errorf("evil not recorded: %+v", rec)
	}
	if _, ok := state["broken"]; ok {
		t.Error("a failed fetch was persisted")
	}
	if rec, ok := state["linux"]; ok {
		t.Errorf("an official base's AUR copy was scanned: %+v", rec)
	}
	if rec := state["shadow"]; rec.Repo != "extra" {
		t.Errorf("the official record was overwritten: %+v", rec)
	}
	feed, err := os.ReadFile(filepath.Join(out, "feed.xml"))
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(feed), "<entry>"); n != 1 || !strings.Contains(string(feed), "PB301") {
		t.Errorf("feed.xml has %d entries, want one for evil's PB301:\n%s", n, feed)
	}

	// A second poll over the same list finds everything current and leaves
	// both files byte-for-byte alone, which is what lets the workflow skip
	// the push.
	before, _ := os.ReadFile(statePath)
	if err := pollAUR(out, cache, statePath, feedPath, officialPath); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(statePath)
	again, _ := os.ReadFile(filepath.Join(out, "feed.xml"))
	if string(before) != string(after) || string(feed) != string(again) {
		t.Error("an idle poll rewrote the state or the feed")
	}
}

func TestPollAURErrors(t *testing.T) {
	tmp := t.TempDir()
	run := func() error {
		return pollAUR(filepath.Join(tmp, "out"), filepath.Join(tmp, "cache"), filepath.Join(tmp, "state.jsonl"), filepath.Join(tmp, "feed.jsonl"), filepath.Join(tmp, "official.txt"))
	}

	srv := pollServer(t, "<rss><channel><item><title>x", nil)
	if err := run(); err == nil {
		t.Error("a truncated RSS document was accepted")
	}

	srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rss" {
			fmt.Fprint(w, rssOf("a"))
			return
		}
		fmt.Fprint(w, `{"type":"error","error":"Too many package results."}`)
	})
	if err := run(); err == nil || !strings.Contains(err.Error(), "Too many") {
		t.Errorf("an RPC error came back as %v", err)
	}

	// An empty list makes no RPC call and still writes a feed.
	srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rss" {
			t.Errorf("unexpected request %s", r.URL)
		}
		fmt.Fprint(w, rssOf())
	})
	if err := run(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(tmp, "out", "feed.xml")); err != nil {
		t.Error(err)
	}

	if err := os.WriteFile(filepath.Join(tmp, "official.txt"), []byte("core\n../etc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := run(); err == nil || !strings.Contains(err.Error(), "official.txt:2") {
		t.Errorf("an unsafe official base came back as %v", err)
	}
	if err := os.Remove(filepath.Join(tmp, "official.txt")); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(tmp, "state.jsonl"), []byte("nope\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := run(); err == nil {
		t.Error("a malformed state was accepted")
	}
}
