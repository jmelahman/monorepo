package main

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jmelahman/pkglint/internal/rules"
)

// TestFeedErrorRulesExist holds the allowlist to the registry: an ID that was
// renumbered or can never report an error would silently feed nothing.
func TestFeedErrorRulesExist(t *testing.T) {
	byID := map[string]rules.Rule{}
	for _, r := range rules.Registry() {
		byID[r.ID] = r
	}
	for id := range feedErrorRules {
		r, ok := byID[id]
		if !ok {
			t.Errorf("feedErrorRules names %s, which is not in the registry", id)
			continue
		}
		if s := r.Severities(); s.Low > rules.Error || s.High < rules.Error {
			t.Errorf("feedErrorRules names %s, which never reports an error (%v–%v)", id, s.Low, s.High)
		}
	}
}

func TestFeedEntries(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	recent := now.Add(-time.Hour).Unix()
	old := now.Add(-30 * 24 * time.Hour).Unix()
	crit := rules.Finding{RuleID: "PB502", Severity: rules.Critical, Path: "demo.install", Line: 3, Message: "scriptlet installs a crontab"}
	moved := crit
	moved.Line = 9
	secErr := rules.Finding{RuleID: "PB310", Severity: rules.Error, Path: "PKGBUILD", Line: 4, Message: "fetch through tor"}
	plainErr := rules.Finding{RuleID: "PB701", Severity: rules.Error, Path: "PKGBUILD", Line: 1, Message: "invalid pkgname"}
	warn := rules.Finding{RuleID: "PB104", Severity: rules.Warn, Path: "PKGBUILD", Line: 2, Message: "http source"}
	quiet, loud := secErr, secErr
	quiet.Severity, loud.Severity = rules.Warn, rules.Critical
	fp := rulesFingerprint()

	rec := func(lm int64, rulesFP string, drift []string, fs ...rules.Finding) stateRecord {
		return stateRecord{Base: "demo", LastModified: lm, Findings: fs, Drift: drift, Rules: rulesFP}
	}
	for _, tc := range []struct {
		name       string
		prev, next *stateRecord
		repo       string
		want       []string // rule IDs and "drift", in order
	}{
		{name: "new critical on a changed base", prev: ptr(rec(old, fp, nil)), next: ptr(rec(recent, fp, nil, crit)), want: []string{"PB502"}},
		{name: "same finding moved line", prev: ptr(rec(old, fp, nil, crit)), next: ptr(rec(recent, fp, nil, moved))},
		{name: "second identical finding", prev: ptr(rec(old, fp, nil, crit)), next: ptr(rec(recent, fp, nil, crit, moved)), want: []string{"PB502"}},
		{name: "rule-only re-lint", prev: ptr(rec(old, "a", nil)), next: ptr(rec(old, "b", nil, crit))},
		{name: "first seen, backlog", next: ptr(rec(old, fp, nil, crit))},
		{name: "first seen, fresh upload", next: ptr(rec(recent, fp, nil, crit)), want: []string{"PB502"}},
		{name: "drift alone", prev: ptr(rec(old, fp, nil)), next: ptr(rec(recent, fp, []string{"sha256sums changed without a version bump"})), want: []string{"drift"}},
		{name: "allowlisted error", prev: ptr(rec(old, fp, nil)), next: ptr(rec(recent, fp, nil, secErr, plainErr, warn)), want: []string{"PB310"}},
		{name: "correctness error only", prev: ptr(rec(old, fp, nil)), next: ptr(rec(recent, fp, nil, plainErr))},
		{name: "not rescanned", prev: ptr(rec(old, fp, nil, crit)), next: ptr(rec(old, fp, nil, crit))},
		{name: "prior from another repo", prev: &stateRecord{Base: "demo", Repo: "extra", LastModified: recent}, next: ptr(rec(recent, fp, nil, crit)), want: []string{"PB502"}},
		// The baseline predates the registry: only criticals get through.
		{name: "baseline from another registry", prev: ptr(rec(old, "older", nil)), next: ptr(rec(recent, fp, nil, secErr, crit)), want: []string{"PB502"}},
		{name: "same words, worse severity", prev: ptr(rec(old, fp, nil, quiet)), next: ptr(rec(recent, fp, nil, secErr)), want: []string{"PB310"}},
		{name: "same words, milder severity", prev: ptr(rec(old, fp, nil, loud)), next: ptr(rec(recent, fp, nil, secErr))},
		// The milder prior is used up by the milder hit, leaving the critical
		// one new.
		{name: "two levels, one climbs", prev: ptr(rec(old, fp, nil, quiet, secErr)), next: ptr(rec(recent, fp, nil, secErr, loud)), want: []string{"PB310"}},
		{name: "refused", prev: ptr(rec(old, fp, nil, crit)), next: &stateRecord{Base: "demo", LastModified: recent, Rules: fp, Err: "parse PKGBUILD: unexpected EOF"}, want: []string{"refused"}},
		{name: "record from another repo", next: &stateRecord{Base: "demo", Repo: "extra", LastModified: recent, Findings: []rules.Finding{crit}}},
		{name: "no record", repo: aurRepo},
	} {
		t.Run(tc.name, func(t *testing.T) {
			prev, next := map[string]stateRecord{}, map[string]stateRecord{}
			if tc.prev != nil {
				prev["demo"] = *tc.prev
			}
			if tc.next != nil {
				next["demo"] = *tc.next
			}
			res := []siteResult{{Name: "demo", Base: "demo", Repo: aurRepo, Version: "1.0-1"}}
			got := feedEntries(res, prev, next, now)
			var ids []string
			for _, e := range got {
				if e.Err != "" {
					ids = append(ids, "refused")
				}
				for _, f := range e.Findings {
					ids = append(ids, f.RuleID)
				}
				for range e.Drift {
					ids = append(ids, "drift")
				}
			}
			if fmt.Sprint(ids) != fmt.Sprint(tc.want) {
				t.Fatalf("entries = %v, want %v", ids, tc.want)
			}
			if len(got) == 1 {
				e := got[0]
				if want := fmt.Sprintf("tag:jamison.lahman.dev,2026:pkglint/aur/demo/%d", tc.next.LastModified); e.ID != want {
					t.Errorf("ID = %q, want %q", e.ID, want)
				}
				if !e.Seen.Equal(now) || e.Version != "1.0-1" {
					t.Errorf("entry = %+v", e)
				}
			}
		})
	}
}

func ptr[T any](v T) *T { return &v }

func TestMergeFeed(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	entry := func(id string, age time.Duration) feedEntry {
		return feedEntry{ID: id, Base: "demo", Seen: now.Add(-age)}
	}
	history := []feedEntry{entry("b", time.Hour), entry("expired", 31*24*time.Hour), entry("a", 2*time.Hour)}
	added := []feedEntry{entry("new", 0), {ID: "a", Base: "demo", Seen: now}}
	got := mergeFeed(history, added, now)
	var ids []string
	for _, e := range got {
		ids = append(ids, e.ID)
	}
	if fmt.Sprint(ids) != "[new b a]" {
		t.Fatalf("merged = %v, want [new b a]", ids)
	}
	if !got[2].Seen.Equal(now.Add(-2 * time.Hour)) {
		t.Errorf("re-emitted entry was re-dated to %v", got[2].Seen)
	}

	var many []feedEntry
	for i := range feedMaxEntries + 10 {
		many = append(many, entry(fmt.Sprintf("e%04d", i), time.Duration(i)*time.Minute))
	}
	got = mergeFeed(many, nil, now)
	if len(got) != feedMaxEntries || got[0].ID != "e0000" {
		t.Errorf("capped to %d entries starting %s, want %d starting e0000", len(got), got[0].ID, feedMaxEntries)
	}
}

func TestFeedRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "feed.jsonl")
	if got, err := loadFeed(path); err != nil || got != nil {
		t.Fatalf("missing feed = %v, %v; want empty", got, err)
	}
	now := time.Unix(1_800_000_000, 0).UTC()
	want := []feedEntry{{ID: "x", Base: "demo", Repo: aurRepo, Seen: now, Drift: []string{"d"}}}
	if err := saveFeed(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := loadFeed(path)
	if err != nil || len(got) != 1 || got[0].ID != "x" || !got[0].Seen.Equal(now) {
		t.Fatalf("round trip = %+v, %v", got, err)
	}

	for name, body := range map[string]string{
		"malformed":   "{not json\n",
		"unsafe base": `{"id":"x","base":"../etc"}` + "\n",
	} {
		p := filepath.Join(dir, strings.ReplaceAll(name, " ", "_"))
		if err := os.WriteFile(p, []byte("\n"+body), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := loadFeed(p); err == nil {
			t.Errorf("%s feed loaded without error", name)
		}
	}
}

// TestWriteAtomHostileText feeds writeAtom a message built to break out of
// the document or reorder the reader's display, and checks the feed still
// parses and says what the message said, visibly.
func TestWriteAtomHostileText(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	hostile := "]]></content><script>alert(1)</script>\x00 evil\u202etxt.sh"
	entries := []feedEntry{{
		ID: "tag:x", Base: "demo", Repo: aurRepo, Version: "1.0-1", Seen: now,
		Findings: []rules.Finding{
			{RuleID: "PB309", Severity: rules.Error, Path: "PKGBUILD", Line: 7, Message: hostile},
			{RuleID: "PB502", Severity: rules.Critical, Path: "demo.install", Line: 1, Message: "crontab"},
			{RuleID: "PB502", Severity: rules.Critical, Path: "demo.install", Line: 2, Message: "crontab again"},
		},
		Drift: []string{"source changed"},
	}}
	path := filepath.Join(t.TempDir(), "feed.xml")
	if err := writeAtom(path, entries); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var feed atomFeed
	if err := xml.Unmarshal(b, &feed); err != nil {
		t.Fatalf("feed.xml does not parse: %v\n%s", err, b)
	}
	if len(feed.Entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(feed.Entries))
	}
	e := feed.Entries[0]
	if want := "demo 1.0-1 (aur): 2 critical, 1 error, 1 drift"; e.Title.Body != want {
		t.Errorf("title = %q, want %q", e.Title.Body, want)
	}
	if e.Content.Type != "text" || !strings.Contains(e.Content.Body, `<script>`) || !strings.Contains(e.Content.Body, `evil\u202etxt.sh`) {
		t.Errorf("content (%q) = %q", e.Content.Type, e.Content.Body)
	}
	if strings.ContainsRune(e.Content.Body, '\u202e') || strings.ContainsRune(e.Content.Body, 0) {
		t.Errorf("content carries a raw control character: %q", e.Content.Body)
	}
	if strings.Contains(string(b), "<script>") {
		t.Error("feed.xml carries an unescaped <script>")
	}
	var terms []string
	for _, c := range e.Categories {
		terms = append(terms, c.Term)
	}
	if fmt.Sprint(terms) != "[aur PB309 PB502 drift]" {
		t.Errorf("categories = %v", terms)
	}
	if feed.Updated != now.Format(time.RFC3339) {
		t.Errorf("updated = %q", feed.Updated)
	}

	// Rendering the same history again yields the same document: entry IDs
	// and dates come from the history, not the clock.
	if err := writeAtom(path, entries); err != nil {
		t.Fatal(err)
	}
	if again, _ := os.ReadFile(path); string(again) != string(b) {
		t.Error("re-rendering the same history changed feed.xml")
	}

	// An empty history still makes a valid feed, dated the epoch.
	if err := writeAtom(path, nil); err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(path)
	var empty atomFeed
	if err := xml.Unmarshal(b, &empty); err != nil || len(empty.Entries) != 0 || empty.Updated != "1970-01-01T00:00:00Z" {
		t.Errorf("empty feed: %v, %d entries, updated %q", err, len(empty.Entries), empty.Updated)
	}
}

func TestUpdateFeedRejectsBadHistory(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "feed.jsonl")
	if err := os.WriteFile(p, []byte("nope\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := updateFeed(dir, p, nil, nil, nil, time.Now()); err == nil {
		t.Error("updateFeed accepted a malformed history")
	}
}

// TestFeedIgnoreFile parses the checked-in ignorelist and holds each entry to
// the registry, so a typo cannot quietly waive nothing.
func TestFeedIgnoreFile(t *testing.T) {
	m, err := parseFeedIgnore(feedIgnoreText)
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	for _, r := range rules.Registry() {
		ids[r.ID] = true
	}
	for base, byRule := range m {
		for id := range byRule {
			if !ids[id] {
				t.Errorf("feedignore.txt waives %s for %s, which is not in the registry", id, base)
			}
		}
	}
}

func TestParseFeedIgnore(t *testing.T) {
	m, err := parseFeedIgnore("# comment\n\ndemo PB302 error split-package eval  # trailing\n")
	if err != nil {
		t.Fatal(err)
	}
	if m["demo"]["PB302"] != rules.Error {
		t.Fatalf("parsed %v", m)
	}
	for _, bad := range []string{
		"demo PB302 error",             // no reason
		"linux-* PB302 error template", // a pattern
		"demo PB302 severe reason",     // unknown severity
	} {
		if _, err := parseFeedIgnore(bad); err == nil {
			t.Errorf("parseFeedIgnore(%q) accepted it", bad)
		}
	}
	defer func() {
		if recover() == nil {
			t.Error("mustParseFeedIgnore did not panic on a bad line")
		}
	}()
	mustParseFeedIgnore("demo")
}

// TestFeedEntriesIgnored checks a waiver quiets exactly what was triaged:
// that base, that rule, up to that severity.
func TestFeedEntriesIgnored(t *testing.T) {
	prevIgnore := feedIgnore
	feedIgnore = mustParseFeedIgnore("demo PB302 error split-package eval\n")
	defer func() { feedIgnore = prevIgnore }()

	now := time.Unix(1_800_000_000, 0)
	evalErr := rules.Finding{RuleID: "PB302", Severity: rules.Error, Path: "PKGBUILD", Message: "eval"}
	evalCrit := rules.Finding{RuleID: "PB302", Severity: rules.Critical, Path: "PKGBUILD", Message: "eval of a download"}
	for _, tc := range []struct {
		base string
		f    rules.Finding
		want int
	}{
		{"demo", evalErr, 0},
		{"demo", evalCrit, 1},
		{"other", evalErr, 1},
	} {
		next := map[string]stateRecord{tc.base: {Base: tc.base, LastModified: now.Unix(), Findings: []rules.Finding{tc.f}}}
		res := []siteResult{{Name: tc.base, Base: tc.base, Repo: aurRepo}}
		if got := feedEntries(res, nil, next, now); len(got) != tc.want {
			t.Errorf("%s %v: %d entries, want %d", tc.base, tc.f.Severity, len(got), tc.want)
		}
	}
}

// TestWriteAtomRefusedSplitPackage pins the links for a split package — its
// report card is filed under the package's name, not the base's — and how a
// refusal reads.
func TestWriteAtomRefusedSplitPackage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "feed.xml")
	e := feedEntry{
		ID: "tag:x", Base: "demo", Name: "demo-bin", Repo: aurRepo, Version: "1.0-1",
		Seen: time.Date(2026, 10, 23, 0, 0, 0, 0, time.UTC), Err: "parse PKGBUILD: bad \u202e token",
	}
	if err := writeAtom(path, []feedEntry{e}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var feed atomFeed
	if err := xml.Unmarshal(b, &feed); err != nil {
		t.Fatal(err)
	}
	got := feed.Entries[0]
	links := map[string]string{}
	for _, l := range got.Links {
		links[l.Title] = l.Rel + " " + l.Href
	}
	for title, want := range map[string]string{
		"package":     "alternate https://aur.archlinux.org/pkgbase/demo",
		"report card": "related " + baseURL + "package/demo-bin.html",
	} {
		if links[title] != want {
			t.Errorf("%s link = %q, want %q", title, links[title], want)
		}
	}
	if want := `refused parse PKGBUILD: bad \u202e token`; !strings.Contains(got.Content.Body, want) {
		t.Errorf("content = %q, want it to hold %q", got.Content.Body, want)
	}
	if want := "demo 1.0-1 (aur): refused"; got.Title.Body != want {
		t.Errorf("title = %q, want %q", got.Title.Body, want)
	}
	if n := len(got.Categories); n != 2 || got.Categories[1].Term != "refused" {
		t.Errorf("categories = %+v", got.Categories)
	}

	// An entry from before names were recorded falls back on the base.
	if r := (feedEntry{Base: "demo"}).result(); r.ReportURL() != baseURL+"package/demo.html" {
		t.Errorf("fallback report card = %s", r.ReportURL())
	}
}
