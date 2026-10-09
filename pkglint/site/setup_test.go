package main

import (
	"os"
	"testing"
	"time"
)

// TestMain shortens the default request throttle. The test servers are on
// 127.0.0.1, so every request to them waits on it; at its real period each
// one would cost half a second and prove nothing. Reset keeps the ticker's
// identity, which TestThrottleFor compares.
func TestMain(m *testing.M) {
	throttle.Reset(time.Millisecond)
	os.Exit(m.Run())
}
