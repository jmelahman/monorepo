// Package mem mirrors solod.dev/so/mem. Under Go the garbage collector owns
// all memory, so allocators are accepted and ignored and the explicit frees
// are no-ops.
package mem

// Allocator stands in for solod's allocator interface. Nothing in
// check-symlinks calls methods on it; it only travels through signatures.
type Allocator any

// System is solod's malloc-backed allocator.
var System Allocator

// FreeString releases a string allocated by, e.g., strings.Clone.
func FreeString(a Allocator, s string) {
	_, _ = a, s
}

// Free releases a single value allocated by Alloc.
func Free[T any](a Allocator, ptr *T) {
	_, _ = a, ptr
}

// FreeSlice releases a slice allocated by AllocSlice.
func FreeSlice[T any](a Allocator, slice []T) {
	_, _ = a, slice
}
