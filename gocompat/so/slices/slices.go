// Package slices mirrors the subset of solod.dev/so/slices that
// check-symlinks uses. In solod these functions grow and release explicit
// allocations; under Go they are append and a no-op.
package slices

import (
	"solod.dev/so/mem"
)

func Append[T any](a mem.Allocator, s []T, elems ...T) []T {
	_ = a
	return append(s, elems...)
}

func Extend[T any](a mem.Allocator, s []T, other []T) []T {
	_ = a
	return append(s, other...)
}

func Make[T any](a mem.Allocator, length int) []T {
	_ = a
	return make([]T, length)
}

func MakeCap[T any](a mem.Allocator, length int, capacity int) []T {
	_ = a
	return make([]T, length, capacity)
}

func Clone[T any](a mem.Allocator, s []T) []T {
	_ = a
	out := make([]T, len(s))
	copy(out, s)
	return out
}

func Free[T any](a mem.Allocator, s []T) {
	_, _ = a, s
}

func Equal[T comparable](s1, s2 []T) bool {
	if len(s1) != len(s2) {
		return false
	}
	for i := range s1 {
		if s1[i] != s2[i] {
			return false
		}
	}
	return true
}

func Index[T comparable](s []T, v T) int {
	for i := range s {
		if s[i] == v {
			return i
		}
	}
	return -1
}

func Contains[T comparable](s []T, v T) bool {
	return Index(s, v) >= 0
}
