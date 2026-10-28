// Command local-ai-detection scores how likely a text is to be machine
// generated, entirely on this machine: llama.cpp and the models it runs are
// downloaded on first use, pinned by digest, and nothing is sent anywhere.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"text/tabwriter"

	"github.com/jmelahman/local-ai-detection/binoculars"
	"github.com/jmelahman/local-ai-detection/internal/version"
	"github.com/jmelahman/local-ai-detection/llm"
)

// Named after whatever the binary is actually called, so usage text and
// error messages stay right when it is renamed.
var programName = filepath.Base(os.Args[0])

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	err := run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	stop()
	if err != nil {
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
func run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet(programName, flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		// Nothing to do about a failed write to stderr.
		_, _ = fmt.Fprintln(stderr, "usage: "+programName+" [flags] [file]\n\nScores file, or standard input, for machine-generated text.")
		fs.PrintDefaults()
	}
	defaultCache, _ := llm.CacheDir()
	var (
		showVersion = fs.Bool("version", false, "print the version and exit")
		showTokens  = fs.Bool("tokens", false, "print every token's surprise and expected surprise")
		cache       = fs.String("cache", defaultCache, "directory for llama.cpp and the models")
		processor   = fs.String("processor", "cpu", "llama.cpp build to use: cpu, cuda, vulkan, metal, rocm")
		threshold   = fs.Float64("threshold", binoculars.Threshold, "scores below this are called machine-generated")
	)
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *showVersion {
		_, err := fmt.Fprintln(stdout, version.Version())
		return err
	}
	if fs.NArg() > 1 {
		fs.Usage()
		return errors.New("at most one file")
	}
	if *cache == "" {
		return errors.New("no cache directory: set -cache")
	}

	text, err := readInput(ctx, fs.Arg(0), stdin)
	if err != nil {
		return err
	}

	d, err := binoculars.Open(ctx, *cache, *processor, stderr)
	if err != nil {
		return err
	}
	defer func() { _ = d.Close() }()

	r, err := d.Score(ctx, text)
	if err != nil {
		return err
	}
	return report(stdout, r, *threshold, *showTokens)
}

// readInput reads the file at path, or stdin for "" or "-". Reading stdin
// gives up when ctx is done, since a terminal may never send EOF.
func readInput(ctx context.Context, path string, stdin io.Reader) (string, error) {
	if path != "" && path != "-" {
		b, err := os.ReadFile(path)
		return string(b), err
	}
	type result struct {
		err error
		b   []byte
	}
	done := make(chan result, 1)
	go func() {
		b, err := io.ReadAll(stdin)
		done <- result{err, b}
	}()
	select {
	case r := <-done:
		return string(r.b), r.err
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func report(w io.Writer, r binoculars.Result, threshold float64, tokens bool) error {
	verdict := "human"
	if r.Machine(threshold) {
		verdict = "machine"
	}
	if tokens {
		tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', tabwriter.AlignRight)
		_, _ = fmt.Fprintln(tw, "surprise\texpected\ttoken\t")
		for _, t := range r.Tokens {
			if t.Scored {
				_, _ = fmt.Fprintf(tw, "%.2f\t%.2f\t%q\t\n", t.Surprise, t.Expected, t.Text)
			} else {
				_, _ = fmt.Fprintf(tw, "\t\t%q\t\n", t.Text)
			}
		}
		if err := tw.Flush(); err != nil {
			return err
		}
	}
	// The default threshold was not calibrated for these models, so a verdict
	// against it is only a guess; say so until the eval harness sets one.
	calibrated := ""
	if threshold == binoculars.Threshold {
		calibrated = ", uncalibrated"
	}
	_, err := fmt.Fprintf(w, "%s (score %.4f, threshold %.4f%s; log-ppl %.3f, log-x-ppl %.3f)\n",
		verdict, r.Score, threshold, calibrated, r.LogPPL, r.LogXPPL)
	return err
}
