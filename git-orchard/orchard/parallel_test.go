package orchard

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestEach(t *testing.T) {
	for _, tc := range []struct{ n, jobs, wantMax int }{
		{n: 10, jobs: 3, wantMax: 3},
		{n: 4, jobs: 0, wantMax: 4},
		{n: 2, jobs: 8, wantMax: 2},
		{n: 0, jobs: 2, wantMax: 0},
	} {
		var (
			running, peak atomic.Int32
			mu            sync.Mutex
			seen          = map[int]int{}
			// Hold every call until as many as allowed are running, so the
			// peak reaches the limit if Each lets it.
			release = make(chan struct{})
			once    sync.Once
		)
		Each(tc.n, tc.jobs, func(i int) {
			now := running.Add(1)
			for {
				p := peak.Load()
				if now <= p || peak.CompareAndSwap(p, now) {
					break
				}
			}
			if int(now) == tc.wantMax {
				once.Do(func() { close(release) })
			}
			<-release
			running.Add(-1)
			mu.Lock()
			seen[i]++
			mu.Unlock()
		})
		if got := int(peak.Load()); got != tc.wantMax {
			t.Errorf("Each(%d, %d): peak concurrency %d, want %d", tc.n, tc.jobs, got, tc.wantMax)
		}
		if len(seen) != tc.n {
			t.Errorf("Each(%d, %d): called for %d indexes, want %d", tc.n, tc.jobs, len(seen), tc.n)
		}
		for i, c := range seen {
			if c != 1 {
				t.Errorf("Each(%d, %d): called %d times for %d", tc.n, tc.jobs, c, i)
			}
		}
	}
}
