package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jmelahman/pkglint/internal/rules"
	"github.com/jmelahman/typesafe-sdk-go"
)

// flagged draws PB401 (an install to /usr/bin, line 9) among others.
const flagged = `pkgname=foo
pkgver=1
pkgrel=1
arch=(x86_64)
source=("git+https://example.com/foo.git")
sha256sums=(SKIP)
package() {
  cd "$srcdir"
  install -Dm755 foo /usr/bin/foo
}
`

func writeTree(t *testing.T, cache, rel, pkgbuild string) {
	t.Helper()
	dir := filepath.Join(cache, "snapshots", rel)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if pkgbuild == "" {
		return
	}
	if err := os.WriteFile(filepath.Join(dir, "PKGBUILD"), []byte(pkgbuild), 0o644); err != nil {
		t.Fatal(err)
	}
}

// corpus is a cache with one AUR base and one official base, both drawing
// PB401 once.
func corpus(t *testing.T) string {
	t.Helper()
	cache := t.TempDir()
	writeTree(t, cache, "foo@100", flagged)
	writeTree(t, cache, "extra/bar@200", strings.Replace(flagged, "pkgname=foo", "pkgname=bar", 1))
	return cache
}

// fakeAPI answers every SystemOne request with fp and a rule-too-broad cause,
// or with status when it is not 200, and counts the requests.
func fakeAPI(t *testing.T, status int, fp float64) *atomic.Int32 {
	t.Helper()
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		var req struct {
			State     state          `json:"state"`
			Questions map[string]any `json:"questions"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || r.URL.Path != "/v1/systemone" {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if !strings.Contains(req.State.Excerpt, ">   9    install") {
			t.Errorf("excerpt does not mark the flagged line:\n%s", req.State.Excerpt)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("x-typesafe-request-id", "req-1")
		if status != http.StatusOK {
			w.WriteHeader(status)
			fmt.Fprint(w, `{"error":{"message":"no"}}`)
			return
		}
		fmt.Fprintf(w, `{"model":"jev-1","answers":{
			"fp":{"type":"noul","noul":%v},
			"cause":{"type":"choice","choice":"rule-too-broad","confidence":0.6,
				"probabilities":{"correct":0.1,"parser-limitation":0.1,"rule-too-broad":0.6,"intentional":0.2}}},
			"usage":{"input_tokens":1,"output_tokens":1}}`, fp)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("TYPESAFE_API_KEY", "test-key")
	t.Setenv("TYPESAFE_BASE_URL", srv.URL)
	return &hits
}

func runArgs(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	err := run(context.Background(), args, &stdout, &stderr)
	return stdout.String(), stderr.String(), err
}

func TestRunJudgesThenReusesVerdicts(t *testing.T) {
	cache := corpus(t)
	out := filepath.Join(t.TempDir(), "verdicts.jsonl")
	hits := fakeAPI(t, http.StatusOK, 0.9)

	report, _, err := runArgs(t, "-cache", cache, "-rules", "PB401", "-out", out)
	if err != nil {
		t.Fatal(err)
	}
	if got := hits.Load(); got != 2 {
		t.Fatalf("requests = %d, want 2", got)
	}
	for _, want := range []string{
		"| PB401 | 2 | 0.90 | 2 | rule-too-broad |",
		"- 0.90 rule-too-broad — aur/foo PKGBUILD:9 — ",
		"- 0.90 rule-too-broad — extra/bar PKGBUILD:9 — ",
	} {
		if !strings.Contains(report, want) {
			t.Errorf("report lacks %q:\n%s", want, report)
		}
	}
	verdicts, err := loadVerdicts(out)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range verdicts {
		if v.RequestID != "req-1" || v.Cause != "rule-too-broad" || v.CauseConfidence != 0.6 || v.Prompt != promptVersion {
			t.Errorf("verdict = %+v", v)
		}
	}

	again, _, err := runArgs(t, "-cache", cache, "-rules", "PB401", "-out", out)
	if err != nil {
		t.Fatal(err)
	}
	if got := hits.Load(); got != 2 {
		t.Errorf("rerun sent %d more requests, want 0", got-2)
	}
	if again != report {
		t.Errorf("rerun report differs:\n%s\nvs\n%s", again, report)
	}
}

func TestRunRejudgesOldPrompt(t *testing.T) {
	cache := corpus(t)
	out := filepath.Join(t.TempDir(), "verdicts.jsonl")
	hits := fakeAPI(t, http.StatusOK, 0.9)
	if _, _, err := runArgs(t, "-cache", cache, "-rules", "PB401", "-out", out); err != nil {
		t.Fatal(err)
	}
	// Rewrite the verdicts as answers to an earlier wording of the question.
	verdicts, err := loadVerdicts(out)
	if err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	for _, v := range verdicts {
		v.Prompt = promptVersion - 1
		line, _ := json.Marshal(v)
		b.Write(append(line, '\n'))
	}
	if err := os.WriteFile(out, b.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := runArgs(t, "-cache", cache, "-rules", "PB401", "-out", out); err != nil {
		t.Fatal(err)
	}
	if got := hits.Load(); got != 4 {
		t.Errorf("requests = %d, want 4: old-prompt verdicts must not satisfy the cache", got)
	}
}

func TestRunBudget(t *testing.T) {
	out := filepath.Join(t.TempDir(), "verdicts.jsonl")
	hits := fakeAPI(t, http.StatusOK, 0.2)
	report, stderr, err := runArgs(t, "-cache", corpus(t), "-rules", "PB401", "-out", out, "-max-requests", "1")
	if err != nil {
		t.Fatal(err)
	}
	if hits.Load() != 1 {
		t.Errorf("requests = %d, want 1", hits.Load())
	}
	if !strings.Contains(stderr, "2 findings to judge; this run's budget covers 1") {
		t.Errorf("stderr = %q", stderr)
	}
	if !strings.Contains(report, "| PB401 | 1 | 0.20 | 0 |") || !strings.Contains(report, "1 sampled findings are not judged yet") {
		t.Errorf("report:\n%s", report)
	}
}

func TestRunDryRunNeedsNoKey(t *testing.T) {
	t.Setenv("TYPESAFE_API_KEY", "")
	out, _, err := runArgs(t, "-cache", corpus(t), "-rules", "PB401", "-dry-run", "-out", filepath.Join(t.TempDir(), "v.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"# PB401 aur/foo PKGBUILD:9", "# PB401 extra/bar PKGBUILD:9", `"type": "noul"`, ">   9    install", "# 2 requests"} {
		if !strings.Contains(out, want) {
			t.Errorf("dry run lacks %q:\n%s", want, out)
		}
	}
}

func TestRunWithoutKey(t *testing.T) {
	t.Setenv("TYPESAFE_API_KEY", "")
	if _, _, err := runArgs(t, "-cache", corpus(t), "-rules", "PB401", "-out", filepath.Join(t.TempDir(), "v.jsonl")); err == nil {
		t.Fatal("ran without an API key")
	}
}

func TestRunStopsOnRejectedKey(t *testing.T) {
	out := filepath.Join(t.TempDir(), "verdicts.jsonl")
	fakeAPI(t, http.StatusUnauthorized, 0)
	report, _, err := runArgs(t, "-cache", corpus(t), "-rules", "PB401", "-out", out, "-jobs", "1")
	var authErr *typesafe.AuthenticationError
	if !errors.As(err, &authErr) {
		t.Fatalf("err = %v, want an AuthenticationError", err)
	}
	if !strings.Contains(report, "No judged findings.") {
		t.Errorf("report:\n%s", report)
	}
}

func TestRunReportsFailedRequests(t *testing.T) {
	out := filepath.Join(t.TempDir(), "verdicts.jsonl")
	fakeAPI(t, http.StatusBadRequest, 0)
	_, stderr, err := runArgs(t, "-cache", corpus(t), "-rules", "PB401", "-out", out)
	if err == nil || !strings.Contains(err.Error(), "2 of 2 requests failed") {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(stderr, "PB401 aur/foo PKGBUILD:9:") {
		t.Errorf("stderr = %q", stderr)
	}
}

func TestRunErrors(t *testing.T) {
	cache := corpus(t)
	badVerdicts := filepath.Join(t.TempDir(), "v.jsonl")
	if err := os.WriteFile(badVerdicts, []byte("{\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"-per-rule", "0"}, "must be positive"},
		{[]string{"stray"}, "unexpected arguments"},
		{[]string{"-no-such-flag"}, "not defined"},
		{[]string{"-rules", "PB000"}, "unknown rule PB000"},
		{[]string{"-severity", "fatal"}, `unknown severity "fatal"`},
		{[]string{"-cache", t.TempDir()}, "run the site generator first"},
		{[]string{"-cache", cache, "-out", badVerdicts}, "v.jsonl:1"},
	} {
		if _, _, err := runArgs(t, tc.args...); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%v: err = %v, want %q", tc.args, err, tc.want)
		}
	}
}

func TestRunSkipsUnloadableTrees(t *testing.T) {
	cache := corpus(t)
	writeTree(t, cache, "empty@1", "") // no PKGBUILD
	_, stderr, err := runArgs(t, "-cache", cache, "-rules", "PB401", "-dry-run", "-out", filepath.Join(t.TempDir(), "v.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stderr, "1 of 3 snapshots failed to load") {
		t.Errorf("stderr = %q", stderr)
	}
}

func TestFindSnapshots(t *testing.T) {
	cache := t.TempDir()
	writeTree(t, cache, "foo@100", flagged)
	writeTree(t, cache, "foo@50", flagged) // older tree of the same base
	writeTree(t, cache, "a@b@7", flagged)  // @ inside the base name
	writeTree(t, cache, "extra/foo@300", flagged)
	writeTree(t, cache, "extra/nested/x@1", flagged) // repos do not nest
	writeTree(t, cache, "bad@stamp", flagged)
	writeTree(t, cache, "@9", flagged)
	if err := os.WriteFile(filepath.Join(cache, "snapshots", "foo@101.tar.gz"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	snaps, err := findSnapshots(cache)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, s := range snaps {
		got = append(got, fmt.Sprintf("%s/%s@%d", s.Repo, s.Base, s.LastModified))
	}
	want := "aur/a@b@7 aur/foo@100 extra/foo@300"
	if strings.Join(got, " ") != want {
		t.Errorf("snapshots = %v, want %s", got, want)
	}

	if snaps, err := findSnapshots(t.TempDir()); err != nil || snaps != nil {
		t.Errorf("missing cache: %v, %v", snaps, err)
	}
	unreadable := t.TempDir()
	if err := os.WriteFile(filepath.Join(unreadable, "snapshots"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := findSnapshots(unreadable); err == nil {
		t.Error("snapshots as a file: no error")
	}
}

func TestExcerpt(t *testing.T) {
	var src strings.Builder
	for i := 1; i <= 100; i++ {
		fmt.Fprintf(&src, "line%d\n", i)
	}
	raw := []byte(src.String())

	got := excerpt(raw, 50, 2)
	want := "   48  line48\n   49  line49\n>  50  line50\n   51  line51\n   52  line52\n"
	if got != want {
		t.Errorf("middle:\n%s\nwant:\n%s", got, want)
	}
	if got := excerpt(raw, 1, 1); got != ">   1  line1\n    2  line2\n" {
		t.Errorf("first line:\n%s", got)
	}
	if got := excerpt(raw, 100, 1); got != "   99  line99\n> 100  line100\n" {
		t.Errorf("last line:\n%s", got)
	}
	for _, line := range []int{0, 101} {
		head := excerpt(raw, line, 2)
		if n := strings.Count(head, "\n"); n != headLines || strings.Contains(head, ">") {
			t.Errorf("line %d: %d lines, want the unmarked first %d", line, n, headLines)
		}
	}
	long := excerpt([]byte(strings.Repeat("é", 400)), 1, 0)
	if !strings.HasSuffix(long, strings.Repeat("é", maxLineRune)+" …\n") {
		t.Errorf("long line not truncated at %d runes: %q", maxLineRune, long)
	}
}

func TestSampleByRule(t *testing.T) {
	var cands []candidate
	for i := range 20 {
		for _, rule := range []string{"PB2", "PB1"} {
			cands = append(cands, candidate{
				snapshot: snapshot{Repo: aurRepo, Base: fmt.Sprintf("b%d", i)},
				Finding:  rules.Finding{RuleID: rule, Line: i},
				File:     "PKGBUILD",
			})
		}
	}
	keys := func(cs []candidate) []string {
		var out []string
		for _, c := range cs {
			out = append(out, c.Finding.RuleID+"/"+c.Base)
		}
		return out
	}

	a := sampleByRule(cands, 3, 1)
	if len(a) != 6 || a[0].Finding.RuleID != "PB1" || a[5].Finding.RuleID != "PB2" {
		t.Fatalf("sample = %v, want 3 per rule, PB1 first", keys(a))
	}
	if b := sampleByRule(append([]candidate(nil), cands...), 3, 1); strings.Join(keys(a), " ") != strings.Join(keys(b), " ") {
		t.Errorf("same seed, different sample: %v vs %v", keys(a), keys(b))
	}
	if c := sampleByRule(cands, 3, 2); strings.Join(keys(a), " ") == strings.Join(keys(c), " ") {
		t.Errorf("seed ignored: %v", keys(c))
	}
	// A finding's rank depends on its own key alone, so a bigger corpus can
	// only push a sampled finding out, never reorder the ones that stay.
	grown := append(append([]candidate(nil), cands...), candidate{
		snapshot: snapshot{Repo: aurRepo, Base: "new"}, Finding: rules.Finding{RuleID: "PB1"}, File: "PKGBUILD",
	})
	full := keys(sampleByRule(grown, 21, 1))
	var filtered []string
	for _, k := range full {
		if k != "PB1/new" {
			filtered = append(filtered, k)
		}
	}
	if want := keys(sampleByRule(cands, 20, 1)); strings.Join(filtered, " ") != strings.Join(want, " ") {
		t.Errorf("growing the corpus reordered the sample")
	}
}

func TestWriteReportTopCauseTie(t *testing.T) {
	c1 := candidate{snapshot: snapshot{Repo: aurRepo, Base: "a"}, Finding: rules.Finding{RuleID: "PB1", Line: 1}, File: "PKGBUILD"}
	c2 := c1
	c2.Base = "b"
	verdicts := map[string]verdict{}
	for c, cause := range map[candidate]string{c1: "rule-too-broad", c2: "correct"} {
		v := verdict{Repo: c.Repo, Base: c.Base, Rule: "PB1", File: c.File, Line: 1, FP: 0.5, Cause: cause, Prompt: promptVersion}
		verdicts[v.key()] = v
	}
	var b bytes.Buffer
	writeReport(&b, []candidate{c1, c2}, verdicts)
	if !strings.Contains(b.String(), "| PB1 | 2 | 0.50 | 0 | correct |") {
		t.Errorf("a tie should break alphabetically:\n%s", b.String())
	}
}

func TestLoadVerdicts(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "v.jsonl")
	v := verdict{Repo: aurRepo, Base: "a", Rule: "PB1", FP: 0.3}
	b, _ := json.Marshal(v)
	if err := os.WriteFile(path, append(append(b, '\n', '\n'), b...), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := loadVerdicts(path)
	if err != nil || len(got) != 1 || got[v.key()] != v {
		t.Errorf("loadVerdicts = %v, %v", got, err)
	}
	if _, err := loadVerdicts(dir); err == nil {
		t.Error("a directory loaded as a verdicts file")
	}
}

func TestParseFlagsRules(t *testing.T) {
	o, err := parseFlags([]string{"-rules", " PB401, ,PB908 "}, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if len(o.rules) != 2 || !o.rules["PB401"] || !o.rules["PB908"] {
		t.Errorf("rules = %v", o.rules)
	}
}

func TestRunSeverityFilter(t *testing.T) {
	t.Setenv("TYPESAFE_API_KEY", "")
	dry := func(sev string) string {
		out, _, err := runArgs(t, "-cache", corpus(t), "-severity", sev, "-dry-run", "-out", filepath.Join(t.TempDir(), "v.jsonl"))
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	// PB401 reports at error; the fixture's other findings are milder.
	if out := dry(" error , critical"); !strings.Contains(out, "# PB401 aur/foo") || strings.Contains(out, "# PB908") {
		t.Errorf("-severity error kept the wrong findings:\n%s", out)
	}
	if out := dry("info"); strings.Contains(out, "# PB401") {
		t.Errorf("-severity info kept PB401:\n%s", out)
	}
}
