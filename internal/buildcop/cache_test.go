package buildcop

import "testing"

func TestJobsCacheHitMiss(t *testing.T) {
	p := &Poller{jobsCache: make(map[string][]ghJob)}

	if _, ok := p.getCachedJobs("repo-a", 1); ok {
		t.Fatal("empty cache should miss")
	}

	jobs := []ghJob{{ID: 10, Conclusion: "failure"}}
	p.putCachedJobs("repo-a", 1, jobs)

	got, ok := p.getCachedJobs("repo-a", 1)
	if !ok {
		t.Fatal("expected hit after put")
	}
	if len(got) != 1 || got[0].ID != 10 {
		t.Errorf("got %+v; want jobs[0].ID=10", got)
	}

	// Different repo with same run_id must not collide.
	if _, ok := p.getCachedJobs("repo-b", 1); ok {
		t.Error("repo-b should miss; key collides with repo-a")
	}
}

func TestJobsCacheRetainEvictsStale(t *testing.T) {
	p := &Poller{jobsCache: make(map[string][]ghJob)}
	p.putCachedJobs("repo-a", 1, nil)
	p.putCachedJobs("repo-a", 2, nil)
	p.putCachedJobs("repo-a", 3, nil)
	p.putCachedJobs("repo-b", 1, nil) // different repo, must survive

	p.retainJobs("repo-a", map[int64]struct{}{1: {}, 3: {}})

	if _, ok := p.getCachedJobs("repo-a", 1); !ok {
		t.Error("repo-a/1 evicted; should be retained")
	}
	if _, ok := p.getCachedJobs("repo-a", 2); ok {
		t.Error("repo-a/2 retained; should have been evicted")
	}
	if _, ok := p.getCachedJobs("repo-a", 3); !ok {
		t.Error("repo-a/3 evicted; should be retained")
	}
	if _, ok := p.getCachedJobs("repo-b", 1); !ok {
		t.Error("repo-b/1 evicted by repo-a's retainJobs; scopes leaked")
	}
}

// TestRetainJobsForBoardsUnionsSameRepo is the rate-limit regression guard:
// two Build Cop boards on the same repo (e.g. an all-branches board plus a
// main-only board) must retain the UNION of their windows. Retaining against
// either board's window alone would evict the other's freshly cached runs and
// force a full refetch of the (budget-consuming) jobs endpoint every tick.
func TestRetainJobsForBoardsUnionsSameRepo(t *testing.T) {
	p := &Poller{jobsCache: make(map[string][]ghJob)}
	const repo = "/repo/onyx"

	// Board A ("all branches") cached runs 1,2,3; board B ("main") shares run 2.
	for _, id := range []int64{1, 2, 3} {
		p.putCachedJobs(repo, id, nil)
	}
	p.putCachedJobs(repo, 4, nil)          // out of every board's window -> evict
	p.putCachedJobs("/repo/other", 1, nil) // different repo -> must survive

	synced := []boardKeep{
		{repo: repo, keep: map[int64]struct{}{1: {}, 2: {}, 3: {}}},
		{repo: repo, keep: map[int64]struct{}{2: {}}},
	}
	p.retainJobsForBoards(synced, nil)

	for _, id := range []int64{1, 2, 3} {
		if _, ok := p.getCachedJobs(repo, id); !ok {
			t.Errorf("run %d evicted; boards on the same repo must not evict each other", id)
		}
	}
	if _, ok := p.getCachedJobs(repo, 4); ok {
		t.Error("run 4 retained; should be evicted (outside every board's window)")
	}
	if _, ok := p.getCachedJobs("/repo/other", 1); !ok {
		t.Error("other repo evicted; per-repo scoping leaked")
	}
}

// TestRetainJobsForBoardsSkipsFailedRepo: when a board on a repo fails to sync,
// retention is skipped for that whole repo so a transient error can't evict
// cache entries the failed board would have kept (causing a needless refetch).
func TestRetainJobsForBoardsSkipsFailedRepo(t *testing.T) {
	p := &Poller{jobsCache: make(map[string][]ghJob)}
	const repo = "/repo/onyx"
	p.putCachedJobs(repo, 1, nil)
	p.putCachedJobs(repo, 2, nil)

	// Board A synced (keep {1}); a sibling board on the same repo errored, so
	// repo is in skip. Run 2 (only the errored board's) must NOT be evicted.
	synced := []boardKeep{{repo: repo, keep: map[int64]struct{}{1: {}}}}
	p.retainJobsForBoards(synced, map[string]struct{}{repo: {}})

	for _, id := range []int64{1, 2} {
		if _, ok := p.getCachedJobs(repo, id); !ok {
			t.Errorf("run %d evicted; retention must be skipped for a repo with a failed board", id)
		}
	}
}
