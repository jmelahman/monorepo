package typesafe

import (
	"context"
	"time"
)

// Test-only options replacing the nondeterministic parts of retrying, so retry
// tests assert exact delays and never actually wait.

func withRand(f func() float64) Option {
	return func(c *config) error {
		c.rand = f
		return nil
	}
}

func withSleep(f func(context.Context, time.Duration) error) Option {
	return func(c *config) error {
		c.sleep = f
		return nil
	}
}

// noJitter makes backoff purely exponential.
func noJitter() float64 { return 0 }
