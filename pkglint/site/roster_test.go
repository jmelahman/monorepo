package main

import (
	"encoding/json"
	"encoding/xml"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/jmelahman/pkglint/internal/rules"
)

func TestShardKey(t *testing.T) {
	for name, want := range map[string]string{
		"yay":       "y",
		"Zoom":      "z", // the AUR lowercases bases, but the key must not depend on it
		"0ad":       "0-9",
		"7zip":      "0-9",
		"visual-fp": "v",
	} {
		if got := shardKey(name); got != want {
			t.Errorf("shardKey(%q) = %q, want %q", name, got, want)
		}
	}
}

// TestGroupShardsCoversEveryPackage is the reachability property: the shards
// exist so nothing falls off the site, so every safe base must land in exactly
// one of them, and unsafe ones in none.
func TestGroupShardsCoversEveryPackage(t *testing.T) {
	results := []siteResult{
		{Name: "zsh"}, {Name: "yay"}, {Name: "0ad"}, {Name: "aardvark"},
		{Name: "abiword"}, {Name: "../../evil"},
	}
	shards := groupShards(results)

	seen := map[string]int{}
	for _, s := range shards {
		for _, r := range s.Results {
			seen[r.Name]++
			if shardKey(r.Name) != s.Key {
				t.Errorf("%q filed under %q", r.Name, s.Key)
			}
		}
	}
	for _, r := range results[:5] {
		if seen[r.Name] != 1 {
			t.Errorf("%q appears in %d shards, want 1", r.Name, seen[r.Name])
		}
	}
	if seen["../../evil"] != 0 {
		t.Error("an unsafe base reached a shard page")
	}

	var keys []string
	for _, s := range shards {
		keys = append(keys, s.Key)
	}
	if want := []string{"0-9", "a", "y", "z"}; !slices.Equal(keys, want) {
		t.Errorf("shard keys = %v, want %v", keys, want)
	}
	// Alphabetical within a shard, not by grade or votes: those reshuffle every
	// night, which would churn the committed diff and move packages between
	// pages, where a name is fixed for the life of the package.
	a := shards[1].Results
	if a[0].Name != "aardvark" || a[1].Name != "abiword" {
		t.Errorf("shard %q is not alphabetical: %v", shards[1].Key, a)
	}
}

// TestWriteRosterRowShape pins the payload site.js decodes positionally. A
// field moving here silently mislabels every row in the browser, and nothing
// else in the build would notice.
func TestWriteRosterRowShape(t *testing.T) {
	out := t.TempDir()
	results := []siteResult{{
		Name: "demo", Repo: "aur", Grade: "C", Votes: 42, Description: "a demo",
		Maintainer: "someone", CoMaintainers: []string{"alice", "bob"},
		Findings: []rules.Finding{{RuleID: "PB101"}, {RuleID: "PB102"}},
		Drift:    []string{"a note"},
	}, {
		Name: "plain", Repo: "extra", Grade: "A", Packager: "Somebody Else", Maintainers: []string{"anthraxx", "dvzrv"},
	}}
	if err := writeRoster(out, results); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(out, "roster.json"))
	if err != nil {
		t.Fatal(err)
	}
	var rows [][]any
	if err := json.Unmarshal(b, &rows); err != nil {
		t.Fatalf("roster.json is not decodable: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	// Co-maintainers are one space-joined string, the shape the row's
	// data-comaintainers attribute uses, so site.js folds one value into its
	// haystacks whichever way a row arrived.
	// The repository, packager and archlinux.org maintainers ride at the end,
	// where an AUR row's packager and maintainers are "" and an official row's
	// maintainer is — the maintainers space-joined like the co-maintainers.
	want := []any{"demo", "C", 2.0, 42.0, "a demo", "someone", 1.0, "alice bob", "aur", "", ""}
	if !slices.Equal(rows[0], want) {
		t.Errorf("row = %v, want %v", rows[0], want)
	}
	// Drift is 0 and co-maintainers "", not absent: the rows are positional,
	// so an omitted field would shift or drop every later one.
	want = []any{"plain", "A", 0.0, 0.0, "", "", 0.0, "", "extra", "Somebody Else", "anthraxx dvzrv"}
	if !slices.Equal(rows[1], want) {
		t.Errorf("official row = %v, want %v", rows[1], want)
	}
}

// TestWriteSitemapChunks drives the sitemap past one file. Under 50,000 URLs
// a single file is legal, so nothing would exercise the split until the corpus
// had already outgrown it in production.
func TestWriteSitemapChunks(t *testing.T) {
	out := t.TempDir()
	var results []siteResult
	for i := range sitemapChunk + 10 {
		results = append(results, siteResult{Name: "pkg" + itoa(i)})
	}
	if err := writeSitemap(out, results, nil); err != nil {
		t.Fatal(err)
	}

	var index struct {
		Sitemaps []struct {
			Loc string `xml:"loc"`
		} `xml:"sitemap"`
	}
	read(t, filepath.Join(out, "sitemap.xml"), &index)
	if len(index.Sitemaps) != 2 {
		t.Fatalf("expected 2 sitemap files, got %d", len(index.Sitemaps))
	}
	if !strings.HasPrefix(index.Sitemaps[0].Loc, baseURL) {
		t.Errorf("sitemap index holds a relative URL: %q", index.Sitemaps[0].Loc)
	}

	var total int
	for i, s := range index.Sitemaps {
		var set struct {
			URLs []struct {
				Loc string `xml:"loc"`
			} `xml:"url"`
		}
		read(t, filepath.Join(out, filepath.Base(s.Loc)), &set)
		if len(set.URLs) > sitemapChunk {
			t.Errorf("file %d holds %d URLs, over the %d cap", i, len(set.URLs), sitemapChunk)
		}
		total += len(set.URLs)
	}
	// Every package, plus the index, the roster index, the rules index, and one
	// page per shard — here just "p", since every name starts with it.
	if want := len(results) + 4; total != want {
		t.Errorf("sitemap lists %d URLs, want %d", total, want)
	}
}

// TestWriteSitemapSkipsUnsafeBases keeps a hostile name out of the one file
// that exists to hand URLs to crawlers.
func TestWriteSitemapSkipsUnsafeBases(t *testing.T) {
	out := t.TempDir()
	if err := writeSitemap(out, []siteResult{{Name: "ok"}, {Name: "../../evil"}}, nil); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(out, "sitemap-1.xml"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "evil") {
		t.Errorf("sitemap lists an unsafe base:\n%s", b)
	}
}

func read(t *testing.T, path string, v any) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := xml.Unmarshal(b, v); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
}

// itoa keeps the fixture names ASCII without pulling strconv in for one call.
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for ; i > 0; i /= 10 {
		b = append([]byte{byte('0' + i%10)}, b...)
	}
	return string(b)
}
