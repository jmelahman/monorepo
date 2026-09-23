package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jmelahman/pkglint/internal/rules"
)

// TestParseRepos pins the -repos flag's reading. The names become cache
// filenames, URL segments and CSS classes, so what the flag admits is the
// whole of their validation.
func TestParseRepos(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want []string
		err  bool
	}{
		{"core,extra,multilib", []string{"core", "extra", "multilib"}, false},
		{" extra , core ", []string{"extra", "core"}, false},
		{"core,core", []string{"core"}, false},
		{"core-testing", []string{"core-testing"}, false},
		{"", nil, false},
		{",", nil, false},
		// The AUR is not a sync database: it has its own index and its own
		// snapshot source, and is always on.
		{"aur", nil, true},
		{"core,aur", nil, true},
		{"Core", nil, true},
		{"../core", nil, true},
		{"core extra", nil, true},
	} {
		got, err := parseRepos(tc.in)
		if (err != nil) != tc.err {
			t.Errorf("parseRepos(%q) error = %v, want error %v", tc.in, err, tc.err)
			continue
		}
		if !slices.Equal(got, tc.want) {
			t.Errorf("parseRepos(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

// TestRepoRank pins the order repositories sort in: pacman's own, then
// anything unfamiliar, then the AUR — so a name shared across the sets lands
// on the one a user's pacman would actually install from.
func TestRepoRank(t *testing.T) {
	order := []string{"core", "extra", "multilib", "core-testing", aurRepo}
	for i := 1; i < len(order); i++ {
		if repoRank(order[i-1]) > repoRank(order[i]) {
			t.Errorf("repoRank(%q) = %d > repoRank(%q) = %d", order[i-1], repoRank(order[i-1]), order[i], repoRank(order[i]))
		}
	}
	if repoRank("core") == repoRank("extra") {
		t.Error("core and extra rank equal")
	}
	if repoRank(aurRepo) <= repoRank("core-testing") {
		t.Error("the AUR should sort after a repository this program has not heard of")
	}
}

// desc renders a pacman desc entry from key/value pairs.
func desc(kv ...string) string {
	var b strings.Builder
	for i := 0; i+1 < len(kv); i += 2 {
		fmt.Fprintf(&b, "%%%s%%\n%s\n\n", kv[i], kv[i+1])
	}
	return b.String()
}

// syncDB builds a gzipped sync database: one directory per package holding
// its desc file, plus whatever else the caller lists.
func syncDB(t testing.TB, entries map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, name := range slices.Sorted(func(yield func(string) bool) {
		for k := range entries {
			if !yield(k) {
				return
			}
		}
	}) {
		body := entries[name]
		hdr := &tar.Header{Name: name, Mode: 0o644, Size: int64(len(body)), Typeflag: tar.TypeReg}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// TestParseSyncDB pins how a sync database folds into package bases. A split
// package is several desc entries with one PKGBUILD behind them, and when a
// mirror is mid-update they disagree on the version; the report card grades
// the PKGBUILD the newest build came from and describes it in the base's own
// words.
func TestParseSyncDB(t *testing.T) {
	db := syncDB(t, map[string]string{
		// The base package: an older build than its siblings, its own
		// description.
		"linux-6.1-1/desc": desc("NAME", "linux", "BASE", "linux", "VERSION", "6.1-1",
			"DESC", "The Linux kernel", "URL", "https://kernel.org", "BUILDDATE", "200",
			"PACKAGER", "Jan Alexander Steffens (heftig) <heftig@archlinux.org>"),
		"linux-6.1-1/files": "%FILES%\nboot/vmlinuz-linux\n",
		// Two siblings from the same, newer build: the tie breaks on the
		// name, so the answer does not depend on tar order.
		"linux-headers-6.1-2/desc": desc("NAME", "linux-headers", "BASE", "linux", "VERSION", "6.1-2",
			"DESC", "Headers", "BUILDDATE", "300", "PACKAGER", "Someone Else <else@archlinux.org>"),
		"linux-docs-6.1-2/desc": desc("NAME", "linux-docs", "BASE", "linux", "VERSION", "6.1-2",
			"DESC", "Documentation", "BUILDDATE", "300", "PACKAGER", "Someone Else <else@archlinux.org>"),
		// No %BASE%: the package is its own base.
		"zlib-1.3-1/desc": desc("NAME", "zlib", "VERSION", "1.3-1", "DESC", "Compression", "BUILDDATE", "100",
			"PACKAGER", "Unknown Packager"),
		// Nothing to key on; skipped rather than fatal.
		"junk-0-0/desc": desc("VERSION", "0-0"),
	})
	got, err := parseSyncDB(bytes.NewReader(db), "core")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("parseSyncDB returned %d bases, want 2: %+v", len(got), got)
	}
	linux, zlib := got[0], got[1]
	if linux.PackageBase != "linux" || zlib.PackageBase != "zlib" {
		t.Fatalf("bases = %q, %q; want linux, zlib in that order", linux.PackageBase, zlib.PackageBase)
	}
	want := metaPackage{
		Name: "linux", PackageBase: "linux", Version: "6.1-2", Description: "The Linux kernel",
		URL: "https://kernel.org", Packager: "Someone Else", LastModified: 300, Repo: "core",
	}
	// The newest build names the tag; the base's own description reads as
	// the base's; the address is dropped from the packager.
	if linux.Version != want.Version || linux.LastModified != want.LastModified ||
		linux.Description != want.Description || linux.Packager != want.Packager ||
		linux.Name != want.Name || linux.Repo != want.Repo {
		t.Errorf("linux = %+v, want %+v", linux, want)
	}
	if zlib.Version != "1.3-1" || zlib.Description != "Compression" || zlib.Packager != "Unknown Packager" || zlib.Repo != "core" {
		t.Errorf("zlib = %+v", zlib)
	}
}

// TestParseSyncDBRejectsGarbage: a mirror that hands back something other
// than a database is an error, not an empty repository — an empty repository
// would prune every one of its pages from the site.
func TestParseSyncDBRejectsGarbage(t *testing.T) {
	if _, err := parseSyncDB(strings.NewReader("<html>not a database</html>"), "core"); err == nil {
		t.Error("parseSyncDB accepted an HTML page")
	}
}

// TestDescFields pins the desc format's corners: a section holds every
// non-empty line under it, blank lines end nothing, and a bare "%" is content.
func TestDescFields(t *testing.T) {
	f := descFields([]byte("%NAME%\r\nfoo\r\n\r\n%DEPENDS%\nglibc\nzlib\n%%\nstray\n%LICENSE%\n\n"))
	if got := f.first("%NAME%"); got != "foo" {
		t.Errorf("%%NAME%% = %q, want foo", got)
	}
	if got := f["%DEPENDS%"]; !slices.Equal(got, []string{"glibc", "zlib", "%%", "stray"}) {
		t.Errorf("%%DEPENDS%% = %q", got)
	}
	if got := f.first("%LICENSE%"); got != "" {
		t.Errorf("empty section = %q, want \"\"", got)
	}
	if got := f.first("%ABSENT%"); got != "" {
		t.Errorf("absent section = %q, want \"\"", got)
	}
}

func TestPackagerName(t *testing.T) {
	for in, want := range map[string]string{
		"Jan Alexander Steffens (heftig) <heftig@archlinux.org>": "Jan Alexander Steffens (heftig)",
		"Unknown Packager":     "Unknown Packager",
		"<nobody@example.org>": "",
		"":                     "",
	} {
		if got := packagerName(in); got != want {
			t.Errorf("packagerName(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestGitlabProject pins devtools' project naming, which is the only thing
// standing between a pkgbase and a 200 sign-in page for a project that does
// not exist.
func TestGitlabProject(t *testing.T) {
	for in, want := range map[string]string{
		"linux":          "linux",
		"python-foo_bar": "python-foo_bar",
		"libsigc++":      "libsigcplusplus",
		"libxml++2.6":    "libxmlplusplus2.6",
		"dvd+rw-tools":   "dvd-rw-tools",
		"gtk+extra":      "gtk-extra",
		"tree":           "unix-tree",
		"foo@bar":        "foo-bar",
		"foo--bar__baz":  "foo-bar-baz",
	} {
		if got := gitlabProject(in); got != want {
			t.Errorf("gitlabProject(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestGitlabTag(t *testing.T) {
	for in, want := range map[string]string{
		"6.1-2":      "6.1-2",
		"1:1.10.0-2": "1-1.10.0-2",
		"1.0~rc1-1":  "1.0.rc1-1",
		"2:1~a~b-1":  "2-1.a.b-1",
		"1.0+r12-1":  "1.0+r12-1",
	} {
		if got := gitlabTag(in); got != want {
			t.Errorf("gitlabTag(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestSnapshotSource pins where each kind of base is fetched from, and that
// a version this program cannot turn into a tag is refused rather than built
// into a URL.
func TestSnapshotSource(t *testing.T) {
	defer restoreSnapshotURL(t, "https://aur.test/%s.tar.gz")()
	defer restoreOfficialSnapshotURL(t, "https://gitlab.test/%s/-/archive/%s/%s-%s.tar.gz")()

	got, err := snapshotSource(metaPackage{PackageBase: "demo", Version: "1.0-1"})
	if err != nil || got != "https://aur.test/demo.tar.gz" {
		t.Errorf("AUR source = %q, %v", got, err)
	}
	got, err = snapshotSource(metaPackage{PackageBase: "libsigc++", Version: "1:2.0-1", Repo: "extra"})
	if want := "https://gitlab.test/libsigcplusplus/-/archive/1-2.0-1/libsigcplusplus-1-2.0-1.tar.gz"; err != nil || got != want {
		t.Errorf("official source = %q, %v; want %s", got, err, want)
	}
	for _, bad := range []string{"", "-1", "1.0/../x-1", "1:2:3-1"} {
		if got, err := snapshotSource(metaPackage{PackageBase: "demo", Version: bad, Repo: "core"}); err == nil {
			t.Errorf("snapshotSource(version %q) = %q, want an error", bad, got)
		}
	}
}

func restoreOfficialSnapshotURL(t *testing.T, url string) func() {
	t.Helper()
	prev := officialSnapshotURL
	officialSnapshotURL = url
	return func() { officialSnapshotURL = prev }
}

func restoreSyncDBURL(t *testing.T, url string) func() {
	t.Helper()
	prev := syncDBURL
	syncDBURL = url
	return func() { syncDBURL = prev }
}

func restoreArchwebSearchURL(t *testing.T, url string) func() {
	t.Helper()
	prev := archwebSearchURL
	archwebSearchURL = url
	return func() { archwebSearchURL = prev }
}

// TestFetchMaintainers pins the sweep of archlinux.org's package search against
// the API's own habits, measured 2026-09-06: a page past the last is answered
// with the last page again rather than an error or an empty one, so the sweep
// has to stop where num_pages says and cannot run until nothing comes back;
// every package of a base repeats the base's maintainers, so they fold to one
// entry; and their order is nobody's, so it is sorted.
func TestFetchMaintainers(t *testing.T) {
	pages := map[string]string{
		"1": `{"valid":true,"num_pages":2,"page":1,"results":[
			{"pkgbase":"linux","maintainers":["heftig"]},
			{"pkgbase":"pacman","maintainers":["morganamilo","anthraxx"]},
			{"pkgbase":"","maintainers":["nobody"]}]}`,
		"2": `{"valid":true,"num_pages":2,"page":2,"results":[
			{"pkgbase":"linux","maintainers":["heftig"]},
			{"pkgbase":"orphan","maintainers":[]}]}`,
	}
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		if body, ok := pages[r.URL.Query().Get("page")]; ok {
			fmt.Fprint(w, body)
			return
		}
		fmt.Fprint(w, pages["2"]) // the last page, over again
	}))
	defer srv.Close()
	defer restoreArchwebSearchURL(t, srv.URL+"/packages/search/json/?page=%d")()
	defer restoreGetBackoff(t, nil)()

	got, err := fetchMaintainers()
	if err != nil {
		t.Fatalf("fetchMaintainers: %v", err)
	}
	want := map[string][]string{"linux": {"heftig"}, "pacman": {"anthraxx", "morganamilo"}, "orphan": nil}
	if len(got) != len(want) {
		t.Errorf("fetchMaintainers = %v, want %v", got, want)
	}
	for base, ms := range want {
		if g, ok := got[base]; !ok || !slices.Equal(g, ms) {
			t.Errorf("fetchMaintainers[%q] = %v (%v), want %v", base, g, ok, ms)
		}
	}
	if n := hits.Load(); n != 2 {
		t.Errorf("server saw %d requests, want exactly num_pages", n)
	}

	// The rejections: a query the form did not accept, and a page count no
	// corpus could have.
	for name, body := range map[string]string{
		"invalid":   `{"valid":false}`,
		"unbounded": `{"valid":true,"num_pages":100000,"results":[]}`,
	} {
		pages["1"] = body
		if _, err := fetchMaintainers(); err == nil {
			t.Errorf("%s page: fetchMaintainers returned no error", name)
		}
	}
}

// TestLoadOfficialCachesDB pins that a sync database is downloaded once a
// day, not once a run: the mirror serves a few megabytes per repository, and
// a same-day file in the cache is what a local re-run reads instead.
func TestLoadOfficialCachesDB(t *testing.T) {
	db := syncDB(t, map[string]string{
		"zlib-1.3-1/desc": desc("NAME", "zlib", "VERSION", "1.3-1", "BUILDDATE", "100", "PACKAGER", "A <a@b>"),
	})
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		switch r.URL.Path {
		case "/core/core.db":
			w.Write(db)
		case "/packages/search/json/":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"valid":true,"num_pages":1,"page":1,"results":[{"pkgbase":"zlib","maintainers":["zed","amy"]}]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	defer restoreSyncDBURL(t, srv.URL+"/%s/%s.db")()
	defer restoreArchwebSearchURL(t, srv.URL+"/packages/search/json/?page=%d")()
	defer restoreGetBackoff(t, nil)()

	cache := t.TempDir()
	for i := 0; i < 2; i++ {
		got, err := loadOfficial(cache, []string{"core"})
		if err != nil {
			t.Fatalf("loadOfficial #%d: %v", i+1, err)
		}
		if len(got) != 1 || got[0].PackageBase != "zlib" || got[0].Repo != "core" || got[0].Packager != "A" ||
			!slices.Equal(got[0].Maintainers, []string{"amy", "zed"}) {
			t.Errorf("loadOfficial #%d = %+v", i+1, got)
		}
	}
	// One request for the database and one for the maintainers, both served
	// from the cache the second time.
	if n := hits.Load(); n != 2 {
		t.Errorf("server saw %d requests, want the second load served from the cache", n)
	}
	// A repository the mirror does not carry is an error with the repository
	// in it, not an empty corpus that would prune every page.
	if _, err := loadOfficial(cache, []string{"core", "nope"}); err == nil || !strings.Contains(err.Error(), "nope") {
		t.Errorf("loadOfficial of a missing repository: %v", err)
	}
	// Nothing to load is nothing loaded, without a request.
	if got, err := loadOfficial(cache, nil); err != nil || len(got) != 0 {
		t.Errorf("loadOfficial(nil) = %v, %v", got, err)
	}
}

// TestDownloadRefusesHTML pins the guard for GitLab's answer to a project that
// does not exist: a 200 carrying its sign-in page. Handed to the tar reader
// it would fail as "gzip: invalid header", which says nothing about the
// project name being wrong.
func TestDownloadRefusesHTML(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte("<!doctype html><title>Sign in</title>"))
	}))
	defer srv.Close()
	path := filepath.Join(t.TempDir(), "x.tar.gz")
	err := download(srv.URL+"/x.tar.gz", path)
	if err == nil || !strings.Contains(err.Error(), "HTML page") {
		t.Fatalf("download of an HTML page: %v, want an error naming it", err)
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Errorf("an HTML page was written to %s (stat: %v)", path, statErr)
	}
}

// TestThrottleFor pins the per-host pacing: GitLab's unauthenticated limit is
// 600 requests per ten minutes, and one global 500ms ticker would spend the
// AUR's slack on getting the runner's address banned.
func TestThrottleFor(t *testing.T) {
	if got := throttleFor("https://gitlab.archlinux.org/archlinux/packaging/packages/linux/-/archive/6.1-2/linux-6.1-2.tar.gz"); got != throttles["gitlab.archlinux.org"] {
		t.Error("a GitLab request does not wait on GitLab's ticker")
	}
	if got := throttleFor("https://aur.archlinux.org/cgit/aur.git/snapshot/demo.tar.gz"); got != throttle {
		t.Error("an AUR request does not wait on the global ticker")
	}
	if got := throttleFor("https://archlinux.org/packages/search/json/?limit=250&page=1"); got != throttles["archlinux.org"] {
		t.Error("an archlinux.org request does not wait on archlinux.org's ticker")
	}
	if got := throttleFor("::not a url"); got != throttle {
		t.Error("an unparseable URL does not fall back to the global ticker")
	}
}

// TestSiteResultLinks pins what a package page links to for each kind of
// base, and that a result with no repository is an AUR one — results.json
// from before the official repositories carries none.
func TestSiteResultLinks(t *testing.T) {
	official := siteResult{Base: "libsigc++", Repo: "extra", Packager: "Somebody", Maintainer: "ignored"}
	if !official.Official() {
		t.Error("an extra package is not official")
	}
	if got := official.PageURL(); got != "https://archlinux.org/pkgbase/libsigc++/" {
		t.Errorf("official PageURL = %s", got)
	}
	if got := official.SourceURL(); got != gitlabPackages+"libsigcplusplus" {
		t.Errorf("official SourceURL = %s", got)
	}
	if got := official.MaintainedBy(); got != "packaged by Somebody" {
		t.Errorf("official MaintainedBy = %q", got)
	}
	if got := (siteResult{Repo: "core"}).MaintainedBy(); got != "" {
		t.Errorf("official package without a packager: MaintainedBy = %q", got)
	}

	for _, aur := range []siteResult{{Base: "demo", Maintainer: "someone"}, {Base: "demo", Repo: aurRepo, Maintainer: "someone"}} {
		if aur.Official() {
			t.Errorf("%+v is reported official", aur)
		}
		if got := aur.PageURL(); got != "https://aur.archlinux.org/pkgbase/demo" {
			t.Errorf("AUR PageURL = %s", got)
		}
		if got := aur.SourceURL(); got != "https://aur.archlinux.org/cgit/aur.git/tree/PKGBUILD?h=demo" {
			t.Errorf("AUR SourceURL = %s", got)
		}
		if got := aur.MaintainedBy(); got != "maintained by someone" {
			t.Errorf("AUR MaintainedBy = %q", got)
		}
	}
}

// TestFetchSnapshotOfficial drives one official fetch end to end: the URL is
// the project's tag archive, and the tree lands in the cache under the
// repository, so an AUR base of the same name cached at the same timestamp
// cannot be mistaken for it.
func TestFetchSnapshotOfficial(t *testing.T) {
	type entry = struct {
		name string
		body []byte
		link bool
	}
	tarball, err := os.ReadFile(tarGz(t, []entry{
		{name: "libsigcplusplus-1-2.0-1/PKGBUILD", body: []byte("pkgname=libsigc++\n")},
		{name: "libsigcplusplus-1-2.0-1/.SRCINFO", body: []byte("pkgbase = libsigc++\n")},
	}))
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var paths []string
	requested := func() []string {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(paths)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths = append(paths, r.URL.Path)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/gzip")
		w.Write(tarball)
	}))
	defer srv.Close()
	defer restoreOfficialSnapshotURL(t, srv.URL+"/%s/-/archive/%s/%s-%s.tar.gz")()

	cache := t.TempDir()
	m := metaPackage{PackageBase: "libsigc++", Version: "1:2.0-1", LastModified: 42, Repo: "extra"}
	dir, err := fetchSnapshot(m, cache)
	if err != nil {
		t.Fatalf("fetchSnapshot: %v", err)
	}
	if want := filepath.Join(cache, "snapshots", "extra", "libsigc++@42"); dir != want {
		t.Errorf("snapshot dir = %s, want %s", dir, want)
	}
	if b, err := os.ReadFile(filepath.Join(dir, "PKGBUILD")); err != nil || string(b) != "pkgname=libsigc++\n" {
		t.Errorf("PKGBUILD = %q (%v)", b, err)
	}
	if want := []string{"/libsigcplusplus/-/archive/1-2.0-1/libsigcplusplus-1-2.0-1.tar.gz"}; !slices.Equal(requested(), want) {
		t.Errorf("requested %v, want %v", requested(), want)
	}
	// Cached: the second call is served from disk.
	if _, err := fetchSnapshot(m, cache); err != nil || len(requested()) != 1 {
		t.Errorf("second fetchSnapshot: %v, %d requests", err, len(requested()))
	}
}

// TestSelectSeedOfficialWins pins the flat namespace: a base carried by both
// an official repository and the AUR has one page, and it grades the
// PKGBUILD pacman would install from. The AUR entries stay ahead of the
// official fill so a bounded night still keeps up with the AUR's churn.
func TestSelectSeedOfficialWins(t *testing.T) {
	who := "someone"
	meta := []metaPackage{
		{PackageBase: "shared", NumVotes: 500, Maintainer: &who},
		{PackageBase: "aur-only", NumVotes: 1, Maintainer: &who},
	}
	official := []metaPackage{
		{PackageBase: "shared", Repo: "core", Version: "1-1"},
		{PackageBase: "zeta", Repo: "extra"},
		// Listed twice across repositories, as a mirror mid-move can: the
		// first wins and the second is dropped, not duplicated.
		{PackageBase: "shared", Repo: "extra", Version: "2-1"},
		{PackageBase: "../evil", Repo: "extra"},
	}
	var got []string
	for _, m := range selectSeed(meta, official, "", 10, 0, time.Now()) {
		got = append(got, m.repo()+"/"+m.PackageBase)
	}
	if want := []string{"aur/aur-only", "core/shared", "extra/zeta"}; !slices.Equal(got, want) {
		t.Errorf("selectSeed = %v, want %v", got, want)
	}
	// The maintainer flag still reaches the AUR set, and the AUR copy of a
	// shadowed base is not what it keeps.
	got = nil
	for _, m := range selectSeed(meta, official, who, 0, 0, time.Now()) {
		got = append(got, m.repo()+"/"+m.PackageBase)
	}
	if want := []string{"aur/aur-only", "core/shared", "extra/zeta"}; !slices.Equal(got, want) {
		t.Errorf("selectSeed with -maintainer = %v, want %v", got, want)
	}
}

// TestScanAllRepoMismatchIsStale pins the reuse check across the namespace
// seam: a record graded from the AUR's PKGBUILD says nothing about the
// official one, even at an equal timestamp — and the AUR's LastModified and
// a build date are not even the same clock.
func TestScanAllRepoMismatchIsStale(t *testing.T) {
	type entry = struct {
		name string
		body []byte
		link bool
	}
	tarball, err := os.ReadFile(tarGz(t, []entry{
		{name: "shared-1-1/PKGBUILD", body: []byte("pkgname=shared\npkgver=1\npkgrel=1\n")},
	}))
	if err != nil {
		t.Fatal(err)
	}
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.Write(tarball)
	}))
	defer srv.Close()
	defer restoreOfficialSnapshotURL(t, srv.URL+"/%s/-/archive/%s/%s-%s.tar.gz")()

	seed := []metaPackage{{PackageBase: "shared", Version: "1-1", LastModified: 1000, Repo: "core", Packager: "Somebody"}}
	prev := map[string]stateRecord{"shared": {
		Base: "shared", LastModified: 1000, Grade: "B",
		Findings: []rules.Finding{{RuleID: "PB101"}},
		Rules:    rulesFingerprint(),
	}}
	results, next := scanAll(seed, t.TempDir(), 1, 0, prev, time.Time{}, nil)
	if n := hits.Load(); n != 1 {
		t.Fatalf("server saw %d fetches, want the AUR record to be ignored and the official tree fetched", n)
	}
	if len(results) != 1 || results[0].Repo != "core" || results[0].Packager != "Somebody" || results[0].Err != "" {
		t.Errorf("result = %+v", results)
	}
	rec := next["shared"]
	if rec.Repo != "core" || rec.LastModified != 1000 || rec.Grade == "" || rec.Grade == "?" {
		t.Errorf("state record = %+v, want a fresh core record", rec)
	}
	// The record just written is what the next run reuses: same repository,
	// same timestamp, no fetch.
	if _, next2 := scanAll(seed, filepath.Join(t.TempDir(), "absent"), 1, 0, next, time.Time{}, nil); next2["shared"].Repo != "core" || hits.Load() != 1 {
		t.Errorf("a matching official record was not reused: %+v, %d fetches", next2["shared"], hits.Load())
	}
}

// TestStateRepoRoundTrip pins the state file's repository field and what its
// absence means: every record written before the official repositories is an
// AUR record, and stays one.
func TestStateRepoRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.jsonl")
	if err := saveState(path, map[string]stateRecord{
		"legacy": {Base: "legacy", LastModified: 1, Grade: "A"},
		"linux":  {Base: "linux", Repo: "core", LastModified: 2, Grade: "B"},
	}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(raw), `"repo"`) != 1 {
		t.Errorf("state file should carry a repo only for the official record:\n%s", raw)
	}
	got, err := loadState(path)
	if err != nil {
		t.Fatal(err)
	}
	if r := got["legacy"].repo(); r != aurRepo {
		t.Errorf("legacy record repo() = %q, want %q", r, aurRepo)
	}
	if r := got["linux"].repo(); r != "core" {
		t.Errorf("official record repo() = %q, want core", r)
	}
}

// TestRunEndToEndOfficialOffline drives run with both sources cached — the
// AUR dump and a sync database — and state records fresh for both, so no
// fetch happens and the site still comes out with both kinds of page.
func TestRunEndToEndOfficialOffline(t *testing.T) {
	tmp := t.TempDir()
	out := filepath.Join(tmp, "out")
	cache := filepath.Join(tmp, "cache")
	statePath := filepath.Join(tmp, "state.jsonl")
	if err := os.MkdirAll(cache, 0o755); err != nil {
		t.Fatal(err)
	}
	meta := `[{"Name":"demo","PackageBase":"demo","Version":"1.0-1","Description":"a demo","Maintainer":"jmelahman","NumVotes":5,"LastModified":1000}]`
	if err := os.WriteFile(filepath.Join(cache, "packages-meta-ext-v1.json.gz"), gzJSON(t, meta), 0o644); err != nil {
		t.Fatal(err)
	}
	db := syncDB(t, map[string]string{
		"linux-6.1-2/desc": desc("NAME", "linux", "BASE", "linux", "VERSION", "6.1-2",
			"DESC", "The Linux kernel", "BUILDDATE", "2000", "PACKAGER", "Somebody <s@archlinux.org>"),
	})
	if err := os.WriteFile(filepath.Join(cache, "core.db"), db, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cache, "maintainers.json"), []byte(`{"linux":["heftig","someone"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := saveState(statePath, map[string]stateRecord{
		"demo": {Base: "demo", LastModified: 1000, Grade: "B",
			Findings: []rules.Finding{{RuleID: "PB101", Severity: rules.Warn}}, Rules: rulesFingerprint()},
		"linux": {Base: "linux", Repo: "core", LastModified: 2000, Grade: "A",
			Findings: []rules.Finding{}, Rules: rulesFingerprint()},
	}); err != nil {
		t.Fatal(err)
	}
	// syncDBURL and archwebSearchURL point nowhere reachable: a passing run
	// is proof the cached database and maintainers were used.
	defer restoreSyncDBURL(t, "http://127.0.0.1:1/%s/%s.db")()
	defer restoreArchwebSearchURL(t, "http://127.0.0.1:1/search?page=%d")()
	defer restoreGetBackoff(t, nil)()

	if err := run(out, cache, statePath, "", []string{"core"}, 1, 0, 0, 1, 5, 0); err != nil {
		t.Fatalf("run: %v", err)
	}
	results, err := os.ReadFile(filepath.Join(out, "results.json"))
	if err != nil {
		t.Fatalf("results.json: %v", err)
	}
	for _, want := range []string{`"repo": "aur"`, `"repo": "core"`, `"packager": "Somebody"`, `"name": "linux"`,
		"\"maintainers\": [\n      \"heftig\",\n      \"someone\"\n    ]"} {
		if !strings.Contains(string(results), want) {
			t.Errorf("results.json missing %s:\n%s", want, results)
		}
	}
	pkg, err := os.ReadFile(filepath.Join(out, "package", "linux.html"))
	if err != nil {
		t.Fatalf("package page: %v", err)
	}
	for _, want := range []string{"maintained by heftig, someone", "packaged by Somebody", "https://archlinux.org/pkgbase/linux/", gitlabPackages + "linux"} {
		if !strings.Contains(string(pkg), want) {
			t.Errorf("linux.html missing %s", want)
		}
	}
	if _, err := os.Stat(filepath.Join(out, "badge", "linux.svg")); err != nil {
		t.Errorf("badge: %v", err)
	}
	// The state written back still says which repository each record is for.
	st, err := loadState(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if st["linux"].Repo != "core" || st["demo"].repo() != aurRepo {
		t.Errorf("state after run: linux=%+v demo=%+v", st["linux"], st["demo"])
	}
}

// FuzzParseSyncDB explores the one parser here that reads what a mirror
// serves. A hostile or merely corrupt database must come back as an error,
// never a panic: the nightly run would otherwise die before writing anything.
func FuzzParseSyncDB(f *testing.F) {
	f.Add(syncDB(f, map[string]string{
		"linux-6.1-2/desc": desc("NAME", "linux", "BASE", "linux", "VERSION", "6.1-2",
			"DESC", "The Linux kernel", "BUILDDATE", "300", "PACKAGER", "Someone <s@archlinux.org>"),
		"linux-6.1-2/files": "%FILES%\nboot/vmlinuz-linux\n",
		"zlib-1.3-1/desc":   desc("NAME", "zlib", "VERSION", "1.3-1", "BUILDDATE", "100"),
	}))
	f.Add([]byte("not gzip"))
	f.Add(gzJSON(f, "not a tar"))
	f.Fuzz(func(t *testing.T, data []byte) {
		pkgs, err := parseSyncDB(bytes.NewReader(data), "core")
		if err != nil {
			return
		}
		for _, m := range pkgs {
			if m.PackageBase == "" || m.Name != m.PackageBase || m.Repo != "core" {
				t.Errorf("parseSyncDB produced an ill-formed base: %+v", m)
			}
		}
	})
}
