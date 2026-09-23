// Package fmt mirrors the subset of solod.dev/so/fmt that check-symlinks
// uses. Solod supports fewer verbs than Go does, so anything that formats
// correctly here also formats correctly there.
package fmt

import (
	gofmt "fmt"
	"io"
	"os"
)

// Print and Println take strings only, as they do in solod.
func Print(a ...string) (int, error) {
	args := make([]any, len(a))
	for i := range a {
		args[i] = a[i]
	}
	return gofmt.Fprint(os.Stdout, args...)
}

func Println(a ...string) (int, error) {
	args := make([]any, len(a))
	for i := range a {
		args[i] = a[i]
	}
	return gofmt.Fprintln(os.Stdout, args...)
}

func Printf(format string, a ...any) (int, error) {
	return gofmt.Fprintf(os.Stdout, format, a...)
}

func Fprintf(w io.Writer, format string, a ...any) (int, error) {
	return gofmt.Fprintf(w, format, a...)
}
