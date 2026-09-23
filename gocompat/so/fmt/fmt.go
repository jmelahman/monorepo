// Package fmt mirrors the subset of solod.dev/so/fmt used by undot.
package fmt

import (
	"fmt"
	"io"
	"strings"
)

func Printf(format string, a ...any) (int, error) {
	return fmt.Printf(format, a...)
}

func Fprintf(w io.Writer, format string, a ...any) (int, error) {
	return fmt.Fprintf(w, format, a...)
}

// Print writes its arguments separated by spaces, matching solod's
// string-only Print (Go's fmt.Print would not separate string operands).
func Print(a ...string) (int, error) {
	return fmt.Print(strings.Join(a, " "))
}
