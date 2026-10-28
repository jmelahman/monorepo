// Command golang-template is the application half of the template: a thin
// CLI over the greeter package. Living at the module root is what makes
// `go install github.com/jmelahman/golang-template@latest` work and names
// the binary after the repo; a second binary would go in cmd/<name>/.
//
// Delete this file and main_test.go for a library-only SDK repo (and drop
// the builds block from .goreleaser.yaml).
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/jmelahman/golang-template/greeter"
	"github.com/jmelahman/golang-template/internal/version"
)

// Named after whatever the binary is actually called, so usage text and
// error messages stay right when this template is renamed.
var programName = filepath.Base(os.Args[0])

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		// -h is a request that was satisfied: flag has already printed the
		// usage, and exiting non-zero would fail any caller that asks for
		// help. Real parse errors are reported by flag too, so they only
		// need the exit status.
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		fmt.Fprintln(os.Stderr, programName+":", err)
		os.Exit(1)
	}
}

// run holds everything main does apart from process plumbing, which is what
// makes it testable — see main_test.go.
func run(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet(programName, flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		// Nothing to do about a failed write to stderr.
		_, _ = fmt.Fprintln(stderr, "usage: "+programName+" [flags] [name]")
		fs.PrintDefaults()
	}
	var (
		showVersion = fs.Bool("version", false, "print the version and exit")
		shout       = fs.Bool("shout", false, "render the greeting in upper case")
		greeting    = fs.String("greeting", "Hello", "salutation to use")
	)
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *showVersion {
		_, err := fmt.Fprintln(stdout, version.Version())
		return err
	}

	name := "world"
	if fs.NArg() > 0 {
		name = strings.Join(fs.Args(), " ")
	}

	opts := []greeter.Option{greeter.WithGreeting(*greeting)}
	if *shout {
		opts = append(opts, greeter.WithShout())
	}

	msg, err := greeter.New(opts...).Greet(name)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(stdout, msg)
	return err
}
