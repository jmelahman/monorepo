// Package eventbus defines the minimal publisher interface the background
// pollers (internal/github, internal/buildcop) use to push board updates onto
// the SSE bus. It is a dependency-free leaf package so both pollers can share
// one interface without importing internal/api (which would form a cycle).
// The concrete *api.EventBus satisfies Publisher structurally.
package eventbus

// Publisher publishes a board event onto the SSE bus.
type Publisher interface {
	Publish(boardID int64, typ string, data any)
}
