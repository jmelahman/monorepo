package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jmelahman/pkglint/internal/rules"
)

// TestSafeBase pins the filter that keeps untrusted AUR package bases from
// becoming path components in the published site.
func TestSafeBase(t *testing.T) {
	for _, tt := range []struct {
		name string
		want bool
	}{
		// Real-world shapes the AUR allows.
		{"python-foo", true},
		{"a.b+c@1", true},
		{"X", true},
		{"0ad", true},
		{"lib32-gcc-libs", true},
		// Hostile or otherwise unusable as a filename.
		{"../../evil", false},
		{"a/b", false},
		{"..", false},
		{".", false},
		{".hidden", false},
		{"", false},
		{"a..b", false},
		{"-leading-dash", false},
		{"back\\slash", false},
		{"space name", false},
		{"nul\x00byte", false},
		{"ctrl\x01char", false},
		{"new\nline", false},
		{"tilde~", false},
		{"semi;colon", false},
	} {
		if got := safeBase(tt.name); got != tt.want {
			t.Errorf("safeBase(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

// TestSelectSeedDropsUnsafeBases proves the filter runs at the choke point, so
// no unsafe name reaches results, links, or output filenames.
func TestSelectSeedDropsUnsafeBases(t *testing.T) {
	who := "jmelahman"
	meta := []metaPackage{
		{PackageBase: "good-pkg", NumVotes: 10},
		{PackageBase: "../../evil", NumVotes: 9999},
		{PackageBase: "a/b", NumVotes: 9999},
		{PackageBase: "..", NumVotes: 9999},
		{PackageBase: "", NumVotes: 9999},
		// Unsafe names must be dropped even via the maintainer path, which
		// bypasses the top-N cutoff.
		{PackageBase: ".hidden", NumVotes: 1, Maintainer: &who},
		{PackageBase: "mine", NumVotes: 1, Maintainer: &who},
	}
	seed := selectSeed(meta, nil, who, 100, 0, time.Now())
	got := map[string]bool{}
	for _, m := range seed {
		got[m.PackageBase] = true
		if !safeBase(m.PackageBase) {
			t.Errorf("selectSeed returned unsafe base %q", m.PackageBase)
		}
	}
	if len(seed) != 2 || !got["good-pkg"] || !got["mine"] {
		t.Errorf("selectSeed = %v, want exactly [good-pkg mine]", got)
	}
}

// TestSelectSeedIncludesCoMaintained pins the -maintainer flag's reach: the
// flag exists so somebody can keep every package they answer for on the site,
// and co-maintainership is answering for a package under another title.
func TestSelectSeedIncludesCoMaintained(t *testing.T) {
	who, other := "jmelahman", "somebody-else"
	meta := []metaPackage{
		{PackageBase: "theirs", NumVotes: 50, Maintainer: &other},
		{PackageBase: "shared", NumVotes: 1, Maintainer: &other, CoMaintainers: []string{"helper", who}},
		{PackageBase: "mine", NumVotes: 1, Maintainer: &who},
	}
	got := map[string]bool{}
	for _, m := range selectSeed(meta, nil, who, 0, 0, time.Now()) {
		got[m.PackageBase] = true
	}
	if len(got) != 2 || !got["mine"] || !got["shared"] {
		t.Errorf("selectSeed = %v, want exactly [mine shared]", got)
	}
}

// TestMaintains pins who the -maintainer flag reaches: the AUR maintainer, any
// AUR co-maintainer, and any of an official package's maintainers on
// archlinux.org, each a way of answering for the package.
func TestMaintains(t *testing.T) {
	who, other := "jmelahman", "somebody-else"
	for _, tc := range []struct {
		name string
		m    metaPackage
		want bool
	}{
		{"maintainer", metaPackage{Maintainer: &who}, true},
		{"co-maintainer", metaPackage{Maintainer: &other, CoMaintainers: []string{"helper", who}}, true},
		{"official maintainer", metaPackage{Repo: "extra", Packager: "Somebody Else", Maintainers: []string{"helper", who}}, true},
		{"packager only", metaPackage{Repo: "extra", Packager: who, Maintainers: []string{other}}, false},
		{"orphan", metaPackage{}, false},
	} {
		if got := maintains(tc.m, who); got != tc.want {
			t.Errorf("%s: maintains = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// TestSelectSeedIsDeterministic pins the tie-break at the top-N cutoff. The
// bases come out of a map and sort.Slice is not stable, so without one the
// cutoff falls differently on every run and packages appear on the site, 404,
// and come back over input that never changed.
func TestSelectSeedIsDeterministic(t *testing.T) {
	// Every base carries the same vote count, so the cutoff is decided purely
	// by the tie-break, and the input order is not the answer.
	var meta []metaPackage
	for _, n := range []string{"delta", "alpha", "echo", "charlie", "bravo"} {
		meta = append(meta, metaPackage{PackageBase: n, NumVotes: 42})
	}
	want := []string{"alpha", "bravo", "charlie"}
	for i := 0; i < 32; i++ {
		var got []string
		for _, m := range selectSeed(meta, nil, "", 3, 0, time.Now()) {
			got = append(got, m.PackageBase)
		}
		if !slices.Equal(got, want) {
			t.Fatalf("run %d: selectSeed = %v, want %v", i, got, want)
		}
	}
}

// TestSelectSeedWindow pins the recency window: the corpus is what the AUR has
// seen an update to lately, and -top is what keeps a heavily-installed but
// long-stable package on the site anyway.
func TestSelectSeedWindow(t *testing.T) {
	now := time.Date(2026, 8, 27, 0, 0, 0, 0, time.UTC)
	ago := func(days int) int64 { return now.AddDate(0, 0, -days).Unix() }
	meta := []metaPackage{
		{PackageBase: "fresh", NumVotes: 1, LastModified: ago(30)},
		{PackageBase: "edge", NumVotes: 2, LastModified: ago(365) + 60},
		{PackageBase: "stale", NumVotes: 3, LastModified: ago(400)},
		{PackageBase: "ancient-but-loved", NumVotes: 9999, LastModified: ago(2000)},
	}

	got := map[string]bool{}
	for _, m := range selectSeed(meta, nil, "", 0, 365, now) {
		got[m.PackageBase] = true
	}
	if !got["fresh"] || !got["edge"] {
		t.Errorf("window dropped a package inside it: %v", got)
	}
	if got["stale"] || got["ancient-but-loved"] {
		t.Errorf("window kept a package outside it: %v", got)
	}

	// -top is additive: the popular package returns even though nothing about
	// it has changed inside the window.
	got = map[string]bool{}
	for _, m := range selectSeed(meta, nil, "", 1, 365, now) {
		got[m.PackageBase] = true
	}
	if !got["ancient-but-loved"] {
		t.Errorf("-top did not re-add the most-voted base: %v", got)
	}
}

// TestSelectSeedWindowIsVotesOrdered pins the ordering the fetch budget relies
// on. -budget spends on the front of the seed, so if this is not votes-first a
// bounded run scans an arbitrary sample instead of the packages people install.
func TestSelectSeedWindowIsVotesOrdered(t *testing.T) {
	now := time.Now()
	var meta []metaPackage
	for i, n := range []string{"few", "many", "some"} {
		meta = append(meta, metaPackage{
			PackageBase: n, NumVotes: []int{1, 100, 10}[i], LastModified: now.Unix(),
		})
	}
	var got []string
	for _, m := range selectSeed(meta, nil, "", 0, 365, now) {
		got = append(got, m.PackageBase)
	}
	if want := []string{"many", "some", "few"}; !slices.Equal(got, want) {
		t.Errorf("selectSeed = %v, want %v", got, want)
	}
}

// TestScanAllReusesUnchanged is the property the whole corpus rests on: a base
// whose LastModified has not moved is served from state, without a fetch. If
// this regresses the nightly run tries to download 47,000 snapshots.
func TestScanAllReusesUnchanged(t *testing.T) {
	seed := []metaPackage{{
		PackageBase: "demo", Version: "2.0-1", Description: "now with more votes",
		NumVotes: 99, LastModified: 1000,
	}}
	prev := map[string]stateRecord{"demo": {
		Base: "demo", LastModified: 1000, Grade: "B",
		Findings: []rules.Finding{{RuleID: "PB101"}},
		Drift:    []string{"a note from the run that saw it change"},
		Rules:    rulesFingerprint(),
	}}

	// cache points nowhere: a fetch would fail, so a passing test is proof
	// none was attempted.
	results, next := scanAll(seed, filepath.Join(t.TempDir(), "absent"), 1, 0, prev, time.Time{}, nil)
	if len(results) != 1 {
		t.Fatalf("expected one result, got %d", len(results))
	}
	r := results[0]
	if r.Grade != "B" || len(r.Findings) != 1 {
		t.Errorf("reused result lost its lint output: %+v", r)
	}
	if len(r.Drift) != 1 {
		t.Errorf("reused result lost its drift note: %+v", r.Drift)
	}
	// Metadata is free every run, so it refreshes even when the lint does not.
	if r.Votes != 99 || r.Version != "2.0-1" || r.Description != "now with more votes" {
		t.Errorf("reused result kept stale metadata: %+v", r)
	}
	if next["demo"].LastModified != 1000 || next["demo"].Grade != "B" {
		t.Errorf("state lost the reused record: %+v", next["demo"])
	}
}

// TestScanAllRescansOnRuleChange pins the other half of the reuse bargain: a
// record produced under a different rule registry is stale even though the
// PKGBUILD has not changed. The snapshot is planted in the cache, so the
// re-lint needs no network — which is also how a real registry bump refreshes
// most of the corpus for free.
func TestScanAllRescansOnRuleChange(t *testing.T) {
	cache := t.TempDir()
	dir := filepath.Join(cache, "snapshots", "demo@1000")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	pkgbuild := "pkgname=demo\npkgver=1.0\npkgrel=1\npkgdesc='A demonstration tool'\n" +
		"arch=('any')\nurl='https://example.com'\nlicense=('MIT')\n"
	if err := os.WriteFile(filepath.Join(dir, "PKGBUILD"), []byte(pkgbuild), 0o644); err != nil {
		t.Fatal(err)
	}

	seed := []metaPackage{{PackageBase: "demo", LastModified: 1000}}
	prev := map[string]stateRecord{"demo": {
		Base: "demo", LastModified: 1000, Grade: "F",
		Findings: []rules.Finding{{RuleID: "PB000", Message: "from a previous ruleset"}},
		Rules:    "0000000000000000", // not the current registry
	}}

	results, next := scanAll(seed, cache, 1, 0, prev, time.Time{}, nil)
	if len(results) != 1 {
		t.Fatalf("expected one result, got %d", len(results))
	}
	for _, f := range results[0].Findings {
		if f.RuleID == "PB000" {
			t.Errorf("stale findings survived the registry change: %+v", results[0].Findings)
		}
	}
	if results[0].Grade == "F" && len(results[0].Findings) == 1 {
		t.Errorf("result was reused despite the registry change: %+v", results[0])
	}
	if got := next["demo"].Rules; got != rulesFingerprint() {
		t.Errorf("refreshed record carries fingerprint %q, want the current registry's", got)
	}
}

// TestScanAllBudget pins how a bounded run divides the corpus. Bases past the
// budget must not be fetched, must keep their last known grade if they have
// one, and must keep their old LastModified so a later run picks them up.
func TestScanAllBudget(t *testing.T) {
	// The one fetch the budget allows is served here, not by the AUR: a 404
	// so get returns without retrying, and a counter so the budget's "one"
	// is an observed number rather than an inference.
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	defer restoreSnapshotURL(t, srv.URL+"/%s.tar.gz")()

	seed := []metaPackage{
		{PackageBase: "unchanged", NumVotes: 30, LastModified: 1000},
		{PackageBase: "known", NumVotes: 20, LastModified: 2000},
		{PackageBase: "new", NumVotes: 10, LastModified: 3000},
	}
	prev := map[string]stateRecord{
		"unchanged": {Base: "unchanged", LastModified: 1000, Grade: "A", Rules: rulesFingerprint()},
		// Changed since it was last scanned, so it is work — but the budget is
		// zero, so it renders at its old grade instead.
		"known": {Base: "known", LastModified: 1, Grade: "D", Rules: rulesFingerprint()},
	}

	// One fetch allowed against two candidates. "unchanged" is not a candidate
	// at all — it is reused — so the slot goes to "known", the more-voted of
	// the two, and "new" waits for a later run.
	results, next := scanAll(seed, t.TempDir(), 1, 1, prev, time.Time{}, nil)
	got := map[string]string{}
	for _, r := range results {
		got[r.Name] = r.Grade
	}
	if got["unchanged"] != "A" {
		t.Errorf("unchanged base was not reused: %v", got)
	}
	// "known" was the one scan the budget allowed; the fetch 404s, which is
	// the recorded outcome.
	if _, ok := got["known"]; !ok {
		t.Errorf("budgeted base was dropped: %v", got)
	}
	if _, ok := got["new"]; ok {
		t.Errorf("never-scanned base past the budget should not render: %v", got)
	}
	// A failed scan must not be persisted, or the next run would treat the
	// failure as this snapshot's answer and never retry it.
	if next["known"].LastModified != 1 {
		t.Errorf("failed scan overwrote the record: %+v", next["known"])
	}
	if n := hits.Load(); n != 1 {
		t.Errorf("budget of one allowed %d fetches, want exactly 1", n)
	}
}

// TestScanAllDeadline pins the deadline's semantics: a base whose fetch never
// started behaves exactly as if it were past the budget — last known grade if
// it has one, off the site if not, record untouched so a later run picks it
// up. The deadline is already in the past and the cache points nowhere, so a
// passing test is also proof no fetch was attempted.
func TestScanAllDeadline(t *testing.T) {
	seed := []metaPackage{
		{PackageBase: "known", NumVotes: 20, LastModified: 2000},
		{PackageBase: "new", NumVotes: 10, LastModified: 3000},
	}
	prev := map[string]stateRecord{
		"known": {Base: "known", LastModified: 1, Grade: "D", Rules: rulesFingerprint()},
	}

	results, next := scanAll(seed, filepath.Join(t.TempDir(), "absent"), 1, 0, prev,
		time.Now().Add(-time.Minute), nil)
	got := map[string]string{}
	for _, r := range results {
		got[r.Name] = r.Grade
	}
	if got["known"] != "D" {
		t.Errorf("base with history should render its last grade: %v", got)
	}
	if _, ok := got["new"]; ok {
		t.Errorf("never-scanned base past the deadline should not render: %v", got)
	}
	if next["known"].LastModified != 1 {
		t.Errorf("deadline overwrote the record: %+v", next["known"])
	}
}

// TestScanAllCheckpoint pins when the checkpoint runs: once per interior
// batch, carrying the records of every base finished so far, and not after
// the final batch, whose records the caller's own save covers.
func TestScanAllCheckpoint(t *testing.T) {
	prevBatch := scanBatch
	scanBatch = 1
	defer func() { scanBatch = prevBatch }()

	cache := t.TempDir()
	for _, base := range []string{"one", "two"} {
		dir := filepath.Join(cache, "snapshots", base+"@1000")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		pkgbuild := "pkgname=" + base + "\npkgver=1.0\npkgrel=1\npkgdesc='A demonstration tool'\n" +
			"arch=('any')\nurl='https://example.com'\nlicense=('MIT')\n"
		if err := os.WriteFile(filepath.Join(dir, "PKGBUILD"), []byte(pkgbuild), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	seed := []metaPackage{
		{PackageBase: "one", LastModified: 1000},
		{PackageBase: "two", LastModified: 1000},
	}

	var calls int
	scanAll(seed, cache, 1, 0, map[string]stateRecord{}, time.Time{}, func(st map[string]stateRecord) {
		calls++
		if _, ok := st["one"]; !ok {
			t.Errorf("checkpoint %d is missing the finished base: %v", calls, st)
		}
	})
	if calls != 1 {
		t.Errorf("checkpoint ran %d times, want once (interior batches only)", calls)
	}
}

// TestDownloadRefusesOversizedBody feeds download a body past the ceiling from
// a loopback test server and asserts it errors and leaves nothing behind.
func TestDownloadRefusesOversizedBody(t *testing.T) {
	defer restoreDownloadCeiling(t, 1<<10)()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write(bytes.Repeat([]byte("x"), 8<<10))
	}))
	defer srv.Close()

	path := filepath.Join(t.TempDir(), "sub", "big.bin")
	err := download(srv.URL, path)
	if err == nil {
		t.Fatal("download of an oversized body returned nil error")
	}
	if !strings.Contains(err.Error(), "exceeded") {
		t.Errorf("error = %v, want a size-ceiling error", err)
	}
	for _, p := range []string{path, path + ".tmp"} {
		if _, statErr := os.Stat(p); !os.IsNotExist(statErr) {
			t.Errorf("%s should not exist after a refused download (stat: %v)", p, statErr)
		}
	}
}

// TestDownloadWithinCeiling guards the happy path: bounding the copy must not
// truncate or otherwise corrupt a legitimate body.
func TestDownloadWithinCeiling(t *testing.T) {
	defer restoreDownloadCeiling(t, 1<<10)()

	body := bytes.Repeat([]byte("y"), 1<<10) // exactly at the ceiling
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write(body)
	}))
	defer srv.Close()

	path := filepath.Join(t.TempDir(), "ok.bin")
	if err := download(srv.URL, path); err != nil {
		t.Fatalf("download: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, body) {
		t.Errorf("downloaded %d bytes, want %d", len(got), len(body))
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Errorf("temp file left behind (stat: %v)", err)
	}
}

func restoreDownloadCeiling(t *testing.T, limit int64) func() {
	t.Helper()
	prev := maxDownloadBytes
	maxDownloadBytes = limit
	return func() { maxDownloadBytes = prev }
}

func restoreEmptyBodyBackoff(t *testing.T, backoff []time.Duration) func() {
	t.Helper()
	prev := emptyBodyBackoff
	emptyBodyBackoff = backoff
	return func() { emptyBodyBackoff = prev }
}

func restoreGetBackoff(t *testing.T, backoff []time.Duration) func() {
	t.Helper()
	prev := getBackoff
	getBackoff = backoff
	return func() { getBackoff = prev }
}

func restoreSnapshotURL(t *testing.T, url string) func() {
	t.Helper()
	prev := snapshotURL
	snapshotURL = url
	return func() { snapshotURL = prev }
}

// TestDownloadRetriesEmptyBody pins the cgit hiccup that used to surface as a
// bare "scan <base>: EOF": a 200 with no bytes behind it is a transient
// server-side failure, and one retry rides it out.
func TestDownloadRetriesEmptyBody(t *testing.T) {
	defer restoreEmptyBodyBackoff(t, []time.Duration{time.Millisecond})()

	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if hits.Add(1) == 1 {
			return // 200, zero bytes
		}
		w.Write([]byte("tarball"))
	}))
	defer srv.Close()

	path := filepath.Join(t.TempDir(), "snap.tar.gz")
	if err := download(srv.URL, path); err != nil {
		t.Fatalf("download: %v", err)
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != "tarball" {
		t.Errorf("file = %q (%v), want the retried body", got, err)
	}
	if n := hits.Load(); n != 2 {
		t.Errorf("server saw %d requests, want 2", n)
	}
}

func TestDownloadGivesUpOnPersistentEmptyBody(t *testing.T) {
	defer restoreEmptyBodyBackoff(t, []time.Duration{time.Millisecond})()

	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
	}))
	defer srv.Close()

	path := filepath.Join(t.TempDir(), "snap.tar.gz")
	err := download(srv.URL, path)
	if err == nil || !errors.Is(err, errEmptyBody) {
		t.Fatalf("download = %v, want an empty-body error", err)
	}
	if n := hits.Load(); n != 2 {
		t.Errorf("server saw %d requests, want one per backoff step plus one", n)
	}
	for _, p := range []string{path, path + ".tmp"} {
		if _, statErr := os.Stat(p); !os.IsNotExist(statErr) {
			t.Errorf("%s should not exist after a failed download (stat: %v)", p, statErr)
		}
	}
}

// TestDecodeMetaBounded shows the metadata decoder stops at the ceiling
// instead of inflating an arbitrarily large gzip stream into memory.
func TestDecodeMetaBounded(t *testing.T) {
	// A highly compressible payload: ~1 MiB of JSON from a few KiB of gzip.
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	zw.Write([]byte(`[{"PackageBase":"a","Description":"`))
	zw.Write(bytes.Repeat([]byte("A"), 1<<20))
	zw.Write([]byte(`"}]`))
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	bomb := buf.Bytes()

	prev := maxMetaDecompressed
	defer func() { maxMetaDecompressed = prev }()

	maxMetaDecompressed = 4 << 10
	zr, err := gzip.NewReader(bytes.NewReader(bomb))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeMeta(zr); err == nil {
		t.Error("decodeMeta accepted a stream past the ceiling, want a decode error")
	}

	// With headroom the same stream decodes fine, so the bound is what
	// rejected it above and legitimate dumps are unaffected.
	maxMetaDecompressed = 1 << 30
	zr, err = gzip.NewReader(bytes.NewReader(bomb))
	if err != nil {
		t.Fatal(err)
	}
	meta, err := decodeMeta(zr)
	if err != nil {
		t.Fatalf("decodeMeta under an ample ceiling: %v", err)
	}
	if len(meta) != 1 || meta[0].PackageBase != "a" {
		t.Errorf("decodeMeta = %+v, want one package base %q", meta, "a")
	}
}

// TestDecodeMetaCoMaintainers pins the field's spelling against the dump's.
// metaPackage carries no json tags, so a rename on either side would not fail
// to decode — it would silently turn every co-maintainer into nobody, and
// nothing else in the build would notice.
func TestDecodeMetaCoMaintainers(t *testing.T) {
	meta, err := decodeMeta(strings.NewReader(
		`[{"PackageBase":"a","Maintainer":"m","CoMaintainers":["b","c"]},{"PackageBase":"lone"}]`))
	if err != nil {
		t.Fatal(err)
	}
	if len(meta) != 2 || !slices.Equal(meta[0].CoMaintainers, []string{"b", "c"}) {
		t.Errorf("decodeMeta = %+v, want co-maintainers [b c]", meta)
	}
	// The dump omits the key entirely when there are none; that must stay the
	// zero value, not an error and not an empty-but-allocated slice the JSON
	// output would then render as "co_maintainers": [].
	if meta[1].CoMaintainers != nil {
		t.Errorf("absent CoMaintainers decoded to %#v, want nil", meta[1].CoMaintainers)
	}
}

// gzJSON gzips a JSON document, in the shape of the cached AUR metadata dump.
func gzJSON(t testing.TB, doc string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write([]byte(doc)); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// TestRunEndToEndOffline drives run() with a warm metadata cache and a state
// record matching the seed, so the whole pipeline — seed selection, state
// reuse, rendering, badges, results.json, pruning — executes without a single
// network request.
func TestRunEndToEndOffline(t *testing.T) {
	tmp := t.TempDir()
	out := filepath.Join(tmp, "out")
	cache := filepath.Join(tmp, "cache")
	statePath := filepath.Join(tmp, "state.jsonl")

	// A same-day cached dump; loadMeta must reuse it instead of downloading.
	if err := os.MkdirAll(cache, 0o755); err != nil {
		t.Fatal(err)
	}
	meta := `[{"Name":"demo","PackageBase":"demo","Version":"1.0-1","Description":"a demo","Maintainer":"jmelahman","NumVotes":5,"LastModified":1000}]`
	if err := os.WriteFile(filepath.Join(cache, "packages-meta-ext-v1.json.gz"), gzJSON(t, meta), 0o644); err != nil {
		t.Fatal(err)
	}

	// A fresh state record for the same LastModified and registry: scanAll
	// reuses it, so no snapshot fetch happens either.
	if err := saveState(statePath, map[string]stateRecord{"demo": {
		Base: "demo", LastModified: 1000, Grade: "B",
		Findings: []rules.Finding{{RuleID: "PB101", Severity: rules.Warn}},
		Rules:    rulesFingerprint(),
	}}); err != nil {
		t.Fatal(err)
	}

	// Stale pages from a package no longer in the corpus: prune must drop them.
	for _, stale := range []string{
		filepath.Join(out, "badge", "gone.svg"),
		filepath.Join(out, "package", "gone.html"),
	} {
		if err := os.MkdirAll(filepath.Dir(stale), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(stale, []byte("stale"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// top=1 pulls demo in regardless of its (ancient) LastModified.
	if err := run(out, cache, statePath, "", nil, 1, 0, 0, 1, 5, 0); err != nil {
		t.Fatalf("run: %v", err)
	}

	results, err := os.ReadFile(filepath.Join(out, "results.json"))
	if err != nil {
		t.Fatalf("results.json: %v", err)
	}
	for _, want := range []string{`"demo"`, `"B"`, `"PB101"`} {
		if !strings.Contains(string(results), want) {
			t.Errorf("results.json missing %s:\n%s", want, results)
		}
	}
	for _, p := range []string{
		filepath.Join(out, ".nojekyll"),
		filepath.Join(out, "badge", "demo.svg"),
	} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("expected output %s: %v", p, err)
		}
	}
	for _, p := range []string{
		filepath.Join(out, "badge", "gone.svg"),
		filepath.Join(out, "package", "gone.html"),
	} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("stale page %s survived prune (stat: %v)", p, err)
		}
	}
}

// tarGz builds a gzipped tarball from (name, body) pairs; a nil body makes a
// non-regular entry (symlink), which extract must skip.
func tarGz(t *testing.T, entries []struct {
	name string
	body []byte
	link bool
}) string {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		hdr := &tar.Header{Name: e.name, Mode: 0o644, Size: int64(len(e.body))}
		if e.link {
			hdr.Typeflag = tar.TypeSymlink
			hdr.Linkname = "/etc/passwd"
			hdr.Size = 0
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if !e.link {
			if _, err := tw.Write(e.body); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "snapshot.tar.gz")
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestExtractSnapshotSafety pins extract's handling of hostile tarballs: path
// traversal flattens to a basename inside the target, symlinks are skipped,
// and nothing is written outside dir.
func TestExtractSnapshotSafety(t *testing.T) {
	type entry = struct {
		name string
		body []byte
		link bool
	}
	parent := t.TempDir()
	dir := filepath.Join(parent, "extracted")
	tarPath := tarGz(t, []entry{
		{name: "demo/PKGBUILD", body: []byte("pkgname=demo\n")},
		{name: "../../escape", body: []byte("evil")},
		{name: "demo/../../../also-escape", body: []byte("evil")},
		{name: "demo/link", link: true},
		{name: "demo/.", body: nil},
	})
	if err := extract(tarPath, dir); err != nil {
		t.Fatalf("extract: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "PKGBUILD"))
	if err != nil || string(got) != "pkgname=demo\n" {
		t.Errorf("PKGBUILD not extracted: %v %q", err, got)
	}
	if _, err := os.Lstat(filepath.Join(dir, "link")); !os.IsNotExist(err) {
		t.Error("symlink entry should be skipped")
	}
	// The traversal names flatten to basenames inside dir — and must never
	// materialize outside it.
	for _, p := range []string{
		filepath.Join(parent, "escape"),
		filepath.Join(parent, "also-escape"),
		filepath.Dir(parent) + "/escape",
	} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("hostile entry escaped to %s (stat: %v)", p, err)
		}
	}
}

// TestFetchSnapshotFailedExtractLeavesNoCache pins the poisoning edge: an
// extraction that dies after writing PKGBUILD must remove the directory, or
// the stat-based cache check would trust the truncated snapshot on every
// later run (.cache persists across CI runs) and pin the base to it until its
// next upstream update.
func TestFetchSnapshotFailedExtractLeavesNoCache(t *testing.T) {
	type entry = struct {
		name string
		body []byte
		link bool
	}
	// Incompressible filler, so cutting the tail of the *compressed* stream
	// lands inside this entry's data — after PKGBUILD is already on disk.
	filler := make([]byte, 64<<10)
	seed := uint32(1)
	for i := range filler {
		seed = seed*1664525 + 1013904223
		filler[i] = byte(seed >> 24)
	}
	good, err := os.ReadFile(tarGz(t, []entry{
		{name: "demo/PKGBUILD", body: []byte("pkgname=demo\n")},
		{name: "demo/big", body: filler},
	}))
	if err != nil {
		t.Fatal(err)
	}
	truncated := good[:len(good)/2]

	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if hits.Add(1) == 1 {
			w.Write(truncated)
			return
		}
		w.Write(good)
	}))
	defer srv.Close()
	defer restoreSnapshotURL(t, srv.URL+"/%s.tar.gz")()

	cache := t.TempDir()
	m := metaPackage{PackageBase: "demo", LastModified: 42}
	_, err = fetchSnapshot(m, cache)
	if err == nil {
		t.Fatal("fetchSnapshot of a truncated tarball returned nil error")
	}
	if !strings.Contains(err.Error(), "extract snapshot") {
		t.Errorf("error = %v, want extract-stage context", err)
	}
	dir := filepath.Join(cache, "snapshots", "demo@42")
	if _, statErr := os.Stat(dir); !os.IsNotExist(statErr) {
		t.Errorf("failed extraction left %s behind (stat: %v)", dir, statErr)
	}
	// With nothing poisoned, the next run's fetch re-downloads and succeeds.
	got, err := fetchSnapshot(m, cache)
	if err != nil {
		t.Fatalf("second fetchSnapshot: %v", err)
	}
	if b, readErr := os.ReadFile(filepath.Join(got, "PKGBUILD")); readErr != nil || string(b) != "pkgname=demo\n" {
		t.Errorf("PKGBUILD after retry = %q (%v)", b, readErr)
	}
	if n := hits.Load(); n != 2 {
		t.Errorf("server saw %d requests, want a re-download on the second call", n)
	}
}

// TestExtractRefusesTooManyFiles pins the file-count DoS backstop.
func TestExtractRefusesTooManyFiles(t *testing.T) {
	type entry = struct {
		name string
		body []byte
		link bool
	}
	var entries []entry
	for i := 0; i <= maxSnapshotFiles; i++ {
		entries = append(entries, entry{name: fmt.Sprintf("demo/f%d", i), body: []byte("x")})
	}
	err := extract(tarGz(t, entries), filepath.Join(t.TempDir(), "out"))
	if err == nil || !strings.Contains(err.Error(), "too many files") {
		t.Errorf("extract of %d files: err = %v, want too-many-files", maxSnapshotFiles+1, err)
	}
}

// TestScanAllKeepsPriorGradeOnFetchFailure pins the policy the over-budget
// path already follows: a snapshot fetch that fails tonight says nothing about
// the PKGBUILD the last run graded, so the base keeps its real grade rather
// than flipping its public badge to "?" for a day. The record is still left
// unwritten, which is what makes the next run retry.
func TestScanAllKeepsPriorGradeOnFetchFailure(t *testing.T) {
	// 404 so get returns without retrying: a 429 or 5xx would sleep ~52s.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	defer restoreSnapshotURL(t, srv.URL+"/%s.tar.gz")()

	// LastModified differs from the record's, so the base is scanned, not reused.
	seed := []metaPackage{{PackageBase: "demo", Version: "2.0-1", NumVotes: 7, LastModified: 2000}}

	t.Run("prior record", func(t *testing.T) {
		prev := map[string]stateRecord{"demo": {
			Base: "demo", LastModified: 1000, Grade: "B",
			Findings: []rules.Finding{{RuleID: "PB101"}},
			Rules:    rulesFingerprint(),
		}}
		results, next := scanAll(seed, t.TempDir(), 1, 0, prev, time.Time{}, nil)
		if len(results) != 1 {
			t.Fatalf("expected one result, got %d", len(results))
		}
		r := results[0]
		if r.Grade != "B" || len(r.Findings) != 1 {
			t.Errorf("failed fetch blanked the prior grade: %+v", r)
		}
		if r.Err != "" {
			t.Errorf("a result carrying a real grade must not also carry an error: %q", r.Err)
		}
		// Metadata is free every run, so it refreshes even when the lint does not.
		if r.Votes != 7 || r.Version != "2.0-1" {
			t.Errorf("kept result has stale metadata: %+v", r)
		}
		if next["demo"].LastModified != 1000 {
			t.Errorf("failed scan was persisted, so the next run will not retry: %+v", next["demo"])
		}
	})

	t.Run("never scanned", func(t *testing.T) {
		results, _ := scanAll(seed, t.TempDir(), 1, 0, map[string]stateRecord{}, time.Time{}, nil)
		if len(results) != 1 {
			t.Fatalf("expected one result, got %d", len(results))
		}
		if results[0].Grade != "?" || results[0].Err == "" {
			t.Errorf("a base with nothing to fall back on must show the failure: %+v", results[0])
		}
	})
}

// TestGetRetriesTransientStatus pins that a 5xx is a hiccup, not a verdict:
// cgit builds snapshots on request and sheds load under a nightly sweep, so a
// run that gave up on the first 503 would blank grades for no reason.
func TestGetRetriesTransientStatus(t *testing.T) {
	defer restoreGetBackoff(t, []time.Duration{0, 0, 0, 0})()

	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if hits.Add(1) <= 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte("body"))
	}))
	defer srv.Close()

	resp, err := get(srv.URL)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %s, want 200", resp.Status)
	}
	if n := hits.Load(); n != 3 {
		t.Errorf("server saw %d requests, want 3", n)
	}
}
