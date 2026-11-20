package bridge

import (
	"time"

	"github.com/jmelahman/dircon-bridge/internal/dircon"
)

// ServiceUUIDs exposes serviceUUIDs to the tests.
func ServiceUUIDs(services []dircon.UUID) (string, []dircon.UUID) { return serviceUUIDs(services) }

// BackoffDelays returns the first n delays of a fresh backoff.
func BackoffDelays(n int) []time.Duration {
	var b backoff
	out := make([]time.Duration, n)
	for i := range out {
		out[i] = b.next()
	}
	return out
}
