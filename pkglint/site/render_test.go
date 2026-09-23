package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jmelahman/pkglint/internal/rules"
)

// TestRenderFixableBadges renders the site with a mix of fixable and
// non-fixable findings and asserts the auto-fix badges surface on the
// dashboard, the rule reference, and the per-package page.
func TestRenderFixableBadges(t *testing.T) {
	// Sanity-check the fixture rules still classify as expected, so this test
	// fails loudly if a rule's FixLevel changes rather than silently passing.
	for id, want := range map[string]rules.FixLevel{
		"PB101": rules.FixNone,
		"PB205": rules.FixSafe,
		"PB204": rules.FixUnsafe,
	} {
		r, ok := rules.RuleByID(id)
		if !ok {
			t.Fatalf("rule %s missing from registry", id)
		}
		if r.FixLevel != want {
			t.Fatalf("%s FixLevel = %v, want %v", id, r.FixLevel, want)
		}
	}

	results := []siteResult{{
		Name:    "demo",
		Base:    "demo",
		Version: "1.0-1",
		Grade:   "D",
		Findings: []rules.Finding{
			{RuleID: "PB101", Severity: rules.Error, Message: "no checksum", Path: "PKGBUILD", Line: 3},
			{RuleID: "PB205", Severity: rules.Warn, Message: "GOSUMDB disabled", Path: "PKGBUILD", Line: 8},
			{RuleID: "PB204", Severity: rules.Warn, Message: "go downloads modules", Path: "PKGBUILD", Line: 9},
		},
	}}

	out := t.TempDir()
	for _, sub := range []string{"rules", "package", "badge"} {
		if err := os.MkdirAll(filepath.Join(out, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := renderSite(out, results); err != nil {
		t.Fatalf("renderSite: %v", err)
	}

	read := func(rel string) string {
		b, err := os.ReadFile(filepath.Join(out, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		return string(b)
	}

	// Dashboard: two of three findings are fixable.
	index := read("index.html")
	if !strings.Contains(index, ">2<") || !strings.Contains(index, "auto-fixable") {
		t.Errorf("index.html missing fixable tile (want count 2):\n%s", index)
	}

	// Package page: the fixable findings carry a pill, the non-fixable one does not.
	pkg := read(filepath.Join("package", "demo.html"))
	if strings.Count(pkg, `class="fix `) != 2 {
		t.Errorf("package page should show exactly 2 fix pills, got %d:\n%s",
			strings.Count(pkg, `class="fix `), pkg)
	}
	if !strings.Contains(pkg, ">--fix<") || !strings.Contains(pkg, ">--unsafe-fix<") {
		t.Errorf("package page missing --fix/--unsafe-fix pills:\n%s", pkg)
	}

	// Rule pages: fixable rules advertise the flag, PB101 does not.
	if r := read(filepath.Join("rules", "PB205.html")); !strings.Contains(r, "pkglint --fix") {
		t.Errorf("PB205 rule page missing --fix guidance:\n%s", r)
	}
	if r := read(filepath.Join("rules", "PB204.html")); !strings.Contains(r, "pkglint --unsafe-fix") {
		t.Errorf("PB204 rule page missing --unsafe-fix guidance:\n%s", r)
	}
	if r := read(filepath.Join("rules", "PB101.html")); strings.Contains(r, `class="fix `) {
		t.Errorf("PB101 rule page should not show a fix pill:\n%s", r)
	}

	// Rule reference table lists the Fix column, and its header carries the
	// invisible --unsafe-fix sizer that holds the column at one width even in
	// categories with no fixable rules.
	idx := read(filepath.Join("rules", "index.html"))
	if !strings.Contains(idx, `<th>Fix<span class="colsizer">`) {
		t.Errorf("rule reference missing Fix column sizer:\n%s", idx)
	}
}

// TestRenderRuleSeverities covers the Severity column on the rule reference:
// a fixed-severity rule gets one badge, an escalating one gets the range, and
// the header carries the sizer that holds the column at a single width across
// the seven category tables.
func TestRenderRuleSeverities(t *testing.T) {
	// Pin the fixtures' declared severities, so a change to a rule fails here
	// rather than quietly turning the assertions below into no-ops.
	for id, want := range map[string]rules.SeverityRange{
		"PB101": {Low: rules.Error, High: rules.Error},
		"PB302": {Low: rules.Error, High: rules.Critical},
	} {
		r, ok := rules.RuleByID(id)
		if !ok {
			t.Fatalf("rule %s missing from registry", id)
		}
		if got := r.Severities(); got != want {
			t.Fatalf("%s severities = %v, want %v", id, got, want)
		}
	}

	out := t.TempDir()
	for _, sub := range []string{"rules", "package", "badge"} {
		if err := os.MkdirAll(filepath.Join(out, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := renderSite(out, nil); err != nil {
		t.Fatalf("renderSite: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(out, "rules", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	idx := string(b)

	if !strings.Contains(idx, `<th>Severity<span class="colsizer">`) {
		t.Errorf("rule reference missing Severity column sizer:\n%s", idx)
	}
	// PB101 always reports error: one badge, no range.
	const fixed = `<td class="rsev"><span class="sev error">error</span></td>`
	if !strings.Contains(idx, fixed) {
		t.Errorf("PB101 row should carry a single error badge (%s)", fixed)
	}
	// PB302 escalates to critical when the evaluated string is downloaded.
	const ranged = `<td class="rsev"><span class="sev error">error</span>` +
		`<span class="sevto">–</span><span class="sev critical">critical</span></td>`
	if !strings.Contains(idx, ranged) {
		t.Errorf("PB302 row should carry an error–critical range (%s)", ranged)
	}
	// Every rule reports one, so every row has a badge — none may be skipped.
	if got, want := strings.Count(idx, `<td class="rsev">`), len(rules.Registry()); got != want {
		t.Errorf("rule reference has %d severity cells, want %d", got, want)
	}
}

// TestRenderLinkPreview covers the head metadata a link preview reads. The
// failure this guards against is silent: a relative og:image or a page-relative
// og:url renders identically in a browser and produces no card at all when the
// page is posted, and nobody notices until someone shares a link.
func TestRenderLinkPreview(t *testing.T) {
	results := []siteResult{{
		Name: "demo", Base: "demo", Version: "1.0-1", Grade: "A",
		Findings: []rules.Finding{},
	}}

	out := t.TempDir()
	for _, sub := range []string{"rules", "package", "badge"} {
		if err := os.MkdirAll(filepath.Join(out, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := renderSite(out, results); err != nil {
		t.Fatalf("renderSite: %v", err)
	}

	// Every page carries the card image and the card type; the canonical URL is
	// the page's own, which is what pins a preview to one address when the same
	// page is reachable as both /rules/ and /rules/index.html.
	for rel, wantURL := range map[string]string{
		"index.html":                          baseURL,
		filepath.Join("rules", "index.html"):  baseURL + "rules/",
		filepath.Join("rules", "PB101.html"):  baseURL + "rules/PB101.html",
		filepath.Join("package", "demo.html"): baseURL + "package/demo.html",
	} {
		b, err := os.ReadFile(filepath.Join(out, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		page := string(b)
		for _, want := range []string{
			`<meta name="twitter:card" content="summary_large_image">`,
			`<meta property="og:image" content="` + baseURL + `assets/og.png">`,
			`<meta property="og:url" content="` + wantURL + `">`,
			`<link rel="canonical" href="` + wantURL + `">`,
		} {
			if !strings.Contains(page, want) {
				t.Errorf("%s missing %s", rel, want)
			}
		}
		// A card with no title or description is a bare link, so the tags have
		// to be filled in rather than merely present.
		for _, empty := range []string{
			`<meta property="og:title" content="">`,
			`<meta property="og:description" content="">`,
			"<title></title>",
		} {
			if strings.Contains(page, empty) {
				t.Errorf("%s has an empty head tag: %s", rel, empty)
			}
		}
	}

	// The image the tags point at has to be published alongside them.
	if _, err := os.Stat(filepath.Join(out, "assets", "og.png")); err != nil {
		t.Errorf("card image not copied to the site: %v", err)
	}
}

// TestRenderMaintainerFilter covers the roster's maintainer search. The roster
// has no maintainer column, so data attributes are the only thing carrying the
// values into the page — and the script that reads them lives in a separate
// file, where renaming either half leaves a page that builds, ships, and
// quietly matches nothing.
func TestRenderMaintainerFilter(t *testing.T) {
	results := []siteResult{
		{Name: "demo", Base: "demo", Version: "1.0-1", Grade: "A", Maintainer: "dbermond", Findings: []rules.Finding{}},
		// Orphaned packages have no maintainer; the AUR drops the field entirely.
		{Name: "orphaned", Base: "orphaned", Version: "2.0-1", Grade: "A", Findings: []rules.Finding{}},
		// A co-maintained package: everyone who can push to it improves it, so
		// everyone who can push to it has to be findable.
		{Name: "shared", Base: "shared", Version: "3.0-1", Grade: "B", Maintainer: "dbermond",
			CoMaintainers: []string{"alice", "bob"}, Findings: []rules.Finding{}},
	}

	out := t.TempDir()
	for _, sub := range []string{"rules", "package", "badge"} {
		if err := os.MkdirAll(filepath.Join(out, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := renderSite(out, results); err != nil {
		t.Fatalf("renderSite: %v", err)
	}

	read := func(rel string) string {
		b, err := os.ReadFile(filepath.Join(out, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		return string(b)
	}

	index := read("index.html")
	for _, want := range []string{
		// What the filter matches on, and the tooltip that lets a row matched on
		// a maintainer say why, given no column shows it.
		`data-maintainer="dbermond"`,
		`title="maintained by dbermond"`,
		// Co-maintainers ride the same way: space-joined in an attribute of
		// their own, and named in the tooltip after the maintainer.
		`data-comaintainers="alice bob"`,
		`title="maintained by dbermond, co-maintained by alice, bob"`,
		// An orphan still carries the attributes, empty, so the script reads a
		// string on every row rather than an undefined on some of them.
		`data-maintainer=""`,
		`data-comaintainers=""`,
		// Nobody searches a field the box does not admit to having.
		`placeholder="Filter by name, maintainer, or description"`,
	} {
		if !strings.Contains(index, want) {
			t.Errorf("index.html missing %s", want)
		}
	}
	// An orphan has no maintainer to name, so it gets no tooltip either.
	if strings.Contains(index, `title="maintained by "`) {
		t.Error("index.html gives an unmaintained package an empty maintainer tooltip")
	}
	// The @ prefix has no other affordance, so the label is the whole of its
	// discoverability.
	if !strings.Contains(index, "Prefix the query with @") {
		t.Error("index.html does not document the @maintainer prefix")
	}

	// The package page names the co-maintainers too: it is where a search for
	// one of them lands, so it has to corroborate what matched.
	if pkg := read(filepath.Join("package", "shared.html")); !strings.Contains(pkg, "co-maintained by alice, bob") {
		t.Errorf("package page does not name the co-maintainers:\n%s", pkg)
	}

	// The other half of the contract: the script has to read the attributes the
	// template writes.
	js := read(filepath.Join("assets", "site.js"))
	for _, want := range []string{"tr.dataset.maintainer", "tr.dataset.comaintainers", "tr.dataset.maintainers", `charAt(0) === "@"`} {
		if !strings.Contains(js, want) {
			t.Errorf("site.js missing %s: the roster's maintainer filter is not wired up", want)
		}
	}
	// The query is shareable: ?search= seeds the box on load and typing
	// mirrors it back, so a link to a filtered view survives being sent.
	for _, want := range []string{`.get("search")`, "history.replaceState"} {
		if !strings.Contains(js, want) {
			t.Errorf("site.js missing %s: the ?search= deep link is not wired up", want)
		}
	}
}

// TestClip covers the description shortener: previews cut a long description
// wherever they run out of room, so it is cut here on a word instead.
func TestClip(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		n    int
		want string
	}{
		{"short", "PB101: pin it.", 200, "PB101: pin it."},
		{"exact", "abcde", 5, "abcde"},
		{"word boundary", "one two three four", 12, "one two…"},
		{"trailing comma", "one two, three", 9, "one two…"},
		// A dash left stranded by the cut goes with it.
		{"dangling dash", "a — bcdefg", 5, "a…"},
		// Cutting by bytes would end this one mid-rune.
		{"multibyte", "ünïcödé wörd", 7, "ünïcödé…"},
		{"unbroken", "aaaaaaaa", 4, "aaaa…"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := clip(tc.in, tc.n); got != tc.want {
				t.Errorf("clip(%q, %d) = %q, want %q", tc.in, tc.n, got, tc.want)
			}
		})
	}
}

// TestRenderRepositories covers what the official repositories add to the
// pages: a repository column and breakdown the roster filters on, a packager
// where the AUR has a maintainer, and package-page links to the repository
// that carries the base — all wired through data attributes to a script in
// another file, where a rename on either side ships quietly.
func TestRenderRepositories(t *testing.T) {
	results := []siteResult{
		{Name: "demo", Base: "demo", Repo: "aur", Version: "1.0-1", Grade: "A", Votes: 7,
			Maintainer: "dbermond", Findings: []rules.Finding{}},
		{Name: "linux", Base: "linux", Repo: "core", Version: "6.1-2", Grade: "B",
			Maintainers: []string{"heftig", "loqs"}, Packager: "Jan Alexander Steffens (heftig)", Findings: []rules.Finding{}},
	}
	out := t.TempDir()
	for _, sub := range []string{"rules", "package", "badge"} {
		if err := os.MkdirAll(filepath.Join(out, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := renderSite(out, results); err != nil {
		t.Fatalf("renderSite: %v", err)
	}
	read := func(rel string) string {
		b, err := os.ReadFile(filepath.Join(out, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		return string(b)
	}

	index := read("index.html")
	for _, want := range []string{
		// The column, and the attribute the filter reads.
		`<th class="repo-h" data-sort="repo">Repo</th>`,
		`data-repo="core"`,
		`data-repo="aur"`,
		`<span class="repo repo-core">core</span>`,
		// The packager and the archlinux.org maintainers ride the same way the
		// AUR maintainer does — the maintainers space-joined, like the
		// co-maintainers — and the tooltip says which is which.
		`data-packager="Jan Alexander Steffens (heftig)"`,
		`data-maintainers="heftig loqs"`,
		`title="maintained by heftig, loqs, packaged by Jan Alexander Steffens (heftig)"`,
		`data-packager=""`,
		`data-maintainers=""`,
		// Votes are an AUR notion; an official row shows none rather than 0.
		`<span class="none">&ndash;</span>`,
		// The repository toggles beside the disclosure, and the tallies they
		// sum: the page is rendered for the whole corpus, with hooks the
		// script rewrites.
		`class="rkey" data-repo="core"`,
		`class="rkey" data-repo="aur"`,
		`<script type="application/json" id="repostats">`,
		`"core":{"total":1,"findings":0,"fixable":0,"drifted":0,"grades":{"B":1}}`,
		`<dd data-stat="total">2</dd>`,
		`<dd data-stat="findings">0</dd>`,
		` hidden data-stat-row="drifted"`,
	} {
		if !strings.Contains(index, want) {
			t.Errorf("index.html missing %s", want)
		}
	}
	pkg := read(filepath.Join("package", "linux.html"))
	for _, want := range []string{
		`<span class="repo repo-core">core</span>`,
		"maintained by heftig, loqs",
		"packaged by",
		`href="https://archlinux.org/pkgbase/linux/"`,
		`href="` + gitlabPackages + `linux"`,
	} {
		if !strings.Contains(pkg, want) {
			t.Errorf("linux.html missing %s", want)
		}
	}
	for _, stray := range []string{"votes", "AUR page", "co-maintained by"} {
		if strings.Contains(pkg, stray) {
			t.Errorf("linux.html carries an AUR-only fact: %s", stray)
		}
	}
	demo := read(filepath.Join("package", "demo.html"))
	for _, want := range []string{`href="https://aur.archlinux.org/pkgbase/demo"`, "maintained by dbermond", "AUR page"} {
		if !strings.Contains(demo, want) {
			t.Errorf("demo.html missing %s", want)
		}
	}
	if strings.Contains(demo, "packaged by") {
		t.Error("demo.html names a packager for an AUR package")
	}

	js := read(filepath.Join("assets", "site.js"))
	for _, want := range []string{"tr.dataset.repo", "tr.dataset.packager", `.get("repo")`, `"packaged by "`, `".rkey"`,
		`getElementById("repostats")`, `"[data-stat]"`, `"all"`} {
		if !strings.Contains(js, want) {
			t.Errorf("site.js missing %s: the repository filter is not wired up", want)
		}
	}
}

// TestRenderSingleRepositoryHasNoBreakdown: with one source a repository
// toggle would have nothing to toggle, so the controls are left out.
func TestRenderSingleRepositoryHasNoBreakdown(t *testing.T) {
	results := []siteResult{
		{Name: "demo", Base: "demo", Version: "1.0-1", Grade: "A", Findings: []rules.Finding{}},
	}
	out := t.TempDir()
	for _, sub := range []string{"rules", "package", "badge"} {
		if err := os.MkdirAll(filepath.Join(out, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := renderSite(out, results); err != nil {
		t.Fatalf("renderSite: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(out, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), `class="rkey"`) || strings.Contains(string(b), `id="repostats"`) {
		t.Error("index.html offers repository toggles with nothing to toggle")
	}
	// A result with no repository is an AUR one, and says so in the column.
	if !strings.Contains(string(b), `data-repo="aur"`) {
		t.Error("index.html does not default an unlabelled result to the AUR")
	}
}
