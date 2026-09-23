package typesafe

import (
	"context"
	"sync"
)

// A BatchResult is the outcome of one request in [Client.SystemOneBatch].
// Exactly one of Response and Err is set.
type BatchResult struct {
	Response *SystemOneResponse
	Err      error
}

// SystemOneBatch sends each request with [Client.SystemOne], at most
// concurrency at a time, and returns their results in the order of reqs. A
// concurrency below 1 is treated as 1.
//
// Requests fail independently: one error does not stop the others. When ctx
// ends, requests that have not started fail with its error without being sent.
// The opts apply to every request.
//
// Each request is retried on its own, so a high concurrency multiplies the load
// on a rate-limited API. Start low and raise it while 429s stay rare.
func (c *Client) SystemOneBatch(ctx context.Context, reqs []SystemOneRequest, concurrency int, opts ...RequestOption) []BatchResult {
	results := make([]BatchResult, len(reqs))
	sem := make(chan struct{}, max(concurrency, 1))
	var wg sync.WaitGroup
	for i, req := range reqs {
		// Checked first because select picks at random between ready cases.
		if err := ctx.Err(); err != nil {
			results[i].Err = err
			continue
		}
		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			results[i].Err = ctx.Err()
			continue
		}
		wg.Go(func() {
			defer func() { <-sem }()
			results[i].Response, results[i].Err = c.SystemOne(ctx, req, opts...)
		})
	}
	wg.Wait()
	return results
}
