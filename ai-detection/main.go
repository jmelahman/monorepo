// Command local-ai-detection scores how likely a text is to be machine
// generated, entirely on this machine: llama.cpp and the models it runs are
// downloaded on first use, pinned by digest, and nothing is sent anywhere.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"text/tabwriter"

	"github.com/spf13/cobra"

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
		fmt.Fprintln(os.Stderr, programName+":", err)
		os.Exit(1)
	}
}

// run holds everything main does apart from process plumbing, which is what
// makes it testable — see main_test.go.
func run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	var (
		showTokens bool
		cache      string
		processor  string
		threshold  float64
	)
	cmd := &cobra.Command{
		Use:     programName + " [file]",
		Short:   "Score text for machine generation, locally",
		Long:    "Scores file, or standard input, for machine-generated text.",
		Version: version.Version(),
		Args:    cobra.MaximumNArgs(1),
		// Usage is for mistakes in the command line, not for failures
		// partway through scoring; main prints the error itself.
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if cache == "" {
				return errors.New("no cache directory: set --cache")
			}
			path := ""
			if len(args) == 1 {
				path = args[0]
			}
			text, err := readInput(cmd.Context(), path, cmd.InOrStdin())
			if err != nil {
				return err
			}

			d, err := binoculars.Open(cmd.Context(), cache, processor, cmd.ErrOrStderr())
			if err != nil {
				return err
			}
			defer func() { _ = d.Close() }()

			r, err := d.Score(cmd.Context(), text)
			if err != nil {
				return err
			}
			return report(cmd.OutOrStdout(), r, threshold, showTokens)
		},
	}
	cmd.SetVersionTemplate("{{.Version}}\n")
	cmd.SetArgs(args)
	cmd.SetIn(stdin)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	defaultCache, _ := llm.CacheDir()
	cmd.Flags().BoolVar(&showTokens, "tokens", false, "print every token's surprise and expected surprise")
	cmd.Flags().StringVar(&cache, "cache", defaultCache, "directory for llama.cpp and the models")
	cmd.Flags().StringVar(&processor, "processor", "cpu", "llama.cpp build to use: cpu, cuda, vulkan, metal, rocm")
	cmd.Flags().Float64Var(&threshold, "threshold", binoculars.Threshold, "scores below this are called machine-generated")

	return cmd.ExecuteContext(ctx)
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
