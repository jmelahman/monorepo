package orchestrator

import (
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/jmelahman/local-preview/internal/db"
)

// sha builds a 40-char hex sha whose every digit is n, so no two fixtures
// share a prefix.
func sha(n int) string { return strings.Repeat(strconv.FormatInt(int64(n), 16), 40) }

// The SQL behind paging is covered in internal/db; this checks the facade's
// own contract — that DeployQuery maps onto the filter and that Total counts
// the whole match, not the page.
func TestDeploysPage(t *testing.T) {
	o, err := New(Options{
		DataDir:           filepath.Join(t.TempDir(), "previews"),
		Addr:              ":8080",
		Runner:            &recordingRunner{},
		RetentionInterval: -1,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { o.Close() })

	// Rows are written straight to the store: this exercises listing, and
	// building six real deploys would cost seconds for no added coverage.
	repo, err := o.database.CreateRepo("demo", "/src/demo", "/bare/demo", db.RepoReady)
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]int64, 6)
	for i := range ids {
		// Distinct in every position, so a prefix search picks out exactly one.
		d, err := o.database.CreateDeploy(repo.ID, sha(i+1), db.DeployMeta{})
		if err != nil {
			t.Fatal(err)
		}
		ids[i] = d.ID
		if i < 4 {
			if err := o.database.SetDeployEvicted(d.ID); err != nil {
				t.Fatal(err)
			}
		}
	}

	page, err := o.DeploysPage(DeployQuery{Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 6 {
		t.Errorf("Total = %d, want 6 (the whole match, not the page)", page.Total)
	}
	if len(page.Deploys) != 2 || page.Deploys[0].ID != ids[5] {
		t.Fatalf("first page = %d rows starting at %d, want 2 starting at %d",
			len(page.Deploys), page.Deploys[0].ID, ids[5])
	}

	second, err := o.DeploysPage(DeployQuery{Limit: 2, Offset: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Deploys) != 2 || second.Deploys[0].ID != ids[3] {
		t.Fatalf("second page starts at %d, want %d", second.Deploys[0].ID, ids[3])
	}

	// Filtering to live deploys is the point of the status knob: evicted rows
	// linger as history and would otherwise dominate the listing.
	live, err := o.DeploysPage(DeployQuery{Status: StatusQueued})
	if err != nil {
		t.Fatal(err)
	}
	if live.Total != 2 || len(live.Deploys) != 2 {
		t.Fatalf("status filter = %d rows / total %d, want 2 / 2", len(live.Deploys), live.Total)
	}

	evicted, err := o.DeploysPage(DeployQuery{Status: StatusEvicted, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if evicted.Total != 4 || len(evicted.Deploys) != 1 {
		t.Fatalf("evicted page = %d rows / total %d, want 1 / 4",
			len(evicted.Deploys), evicted.Total)
	}

	// Query reaches the free-text predicate: shas match by prefix, so a
	// short sha off a listing row finds the deploy it came from.
	short := sha(3)[:7]
	hit, err := o.DeploysPage(DeployQuery{Query: short})
	if err != nil {
		t.Fatal(err)
	}
	if hit.Total != 1 || len(hit.Deploys) != 1 || hit.Deploys[0].ID != ids[2] {
		t.Fatalf("query %q = %+v, want the one deploy %d", short, hit, ids[2])
	}

	// Query narrows the same result set the other knobs page through, rather
	// than replacing them.
	narrowed, err := o.DeploysPage(DeployQuery{Query: "demo", Status: StatusEvicted})
	if err != nil {
		t.Fatal(err)
	}
	if narrowed.Total != 4 {
		t.Errorf("query+status total = %d, want 4", narrowed.Total)
	}

	// Unknown repo: an empty page, not an error.
	none, err := o.DeploysPage(DeployQuery{Repo: "nope"})
	if err != nil || none.Total != 0 || len(none.Deploys) != 0 {
		t.Fatalf("unknown repo = %+v (%v), want an empty page", none, err)
	}
}
