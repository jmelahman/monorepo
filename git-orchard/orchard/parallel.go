package orchard

import "sync"

// Each calls f(i) for i in [0, n), at most jobs at a time (n at a time if
// jobs < 1), and returns once every call has. The operations here are safe
// to run at once for different subtrees: each fetches to its own ref, and
// git subtree split keeps its cache per process.
func Each(n, jobs int, f func(i int)) {
	if jobs < 1 || jobs > n {
		jobs = n
	}
	sem := make(chan struct{}, jobs)
	var wg sync.WaitGroup
	for i := range n {
		sem <- struct{}{}
		wg.Add(1)
		go func() {
			defer func() {
				<-sem
				wg.Done()
			}()
			f(i)
		}()
	}
	wg.Wait()
}
