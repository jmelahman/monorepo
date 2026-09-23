// Package strings mirrors the subset of solod.dev/so/strings used by
// undot. Functions that take an allocator in solod ignore it here.
package strings

import (
	"strings"

	"solod.dev/so/mem"
)

func Contains(s, substr string) bool        { return strings.Contains(s, substr) }
func Count(s, substr string) int            { return strings.Count(s, substr) }
func HasPrefix(s, prefix string) bool       { return strings.HasPrefix(s, prefix) }
func HasSuffix(s, suffix string) bool       { return strings.HasSuffix(s, suffix) }
func Index(s, substr string) int            { return strings.Index(s, substr) }
func IndexByte(s string, c byte) int        { return strings.IndexByte(s, c) }
func TrimPrefix(s, prefix string) string    { return strings.TrimPrefix(s, prefix) }
func TrimSpace(s string) string             { return strings.TrimSpace(s) }
func TrimSuffix(s, suffix string) string    { return strings.TrimSuffix(s, suffix) }

func Split(a mem.Allocator, s, sep string) []string {
	_ = a
	return strings.Split(s, sep)
}
