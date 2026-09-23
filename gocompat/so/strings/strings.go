// Package strings mirrors the subset of solod.dev/so/strings that
// check-symlinks uses. The functions that allocate take an allocator in
// solod; here the allocator is ignored.
package strings

import (
	gostrings "strings"

	"solod.dev/so/mem"
)

func Clone(a mem.Allocator, s string) string {
	_ = a
	return gostrings.Clone(s)
}

func Split(a mem.Allocator, s, sep string) []string {
	_ = a
	return gostrings.Split(s, sep)
}

func Join(a mem.Allocator, elems []string, sep string) string {
	_ = a
	return gostrings.Join(elems, sep)
}

func Compare(a, b string) int        { return gostrings.Compare(a, b) }
func Contains(s, substr string) bool { return gostrings.Contains(s, substr) }
func Count(s, substr string) int     { return gostrings.Count(s, substr) }
func Cut(s, sep string) (string, string) {
	before, after, _ := gostrings.Cut(s, sep)
	return before, after
}
func HasPrefix(s, prefix string) bool    { return gostrings.HasPrefix(s, prefix) }
func HasSuffix(s, suffix string) bool    { return gostrings.HasSuffix(s, suffix) }
func Index(s, substr string) int         { return gostrings.Index(s, substr) }
func IndexByte(s string, c byte) int     { return gostrings.IndexByte(s, c) }
func LastIndex(s, substr string) int     { return gostrings.LastIndex(s, substr) }
func LastIndexByte(s string, c byte) int { return gostrings.LastIndexByte(s, c) }
func Trim(s, cutset string) string       { return gostrings.Trim(s, cutset) }
func TrimLeft(s, cutset string) string   { return gostrings.TrimLeft(s, cutset) }
func TrimRight(s, cutset string) string  { return gostrings.TrimRight(s, cutset) }
func TrimPrefix(s, prefix string) string { return gostrings.TrimPrefix(s, prefix) }
func TrimSuffix(s, suffix string) string { return gostrings.TrimSuffix(s, suffix) }
func TrimSpace(s string) string          { return gostrings.TrimSpace(s) }
