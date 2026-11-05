// Command eval scores a labelled CSV with the Binoculars detector and reports
// how well it separates human from machine-written text.
//
// It has two steps so that a long run is resumable and can be split across
// processes:
//
//	eval score --data mage_test.csv --limit 1000 --out scores.csv
//	eval report --scores scores.csv
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/jmelahman/local-ai-detection/binoculars"
	"github.com/jmelahman/local-ai-detection/eval"
	"github.com/jmelahman/local-ai-detection/llm"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	err := run(ctx, os.Args[1:], os.Stdout, os.Stderr)
	stop()
	if err != nil {
		fmt.Fprintln(os.Stderr, "eval:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return errors.New("no command")
	}
	switch args[0] {
	case "score":
		return runScore(ctx, args[1:], stderr)
	case "report":
		return runReport(args[1:], stdout, stderr)
	case "help", "-h", "--help":
		fmt.Fprint(stdout, usage)
		return nil
	default:
		fmt.Fprint(stderr, usage)
		return fmt.Errorf("unknown command %q", args[0])
	}
}

const usage = `usage:
  eval score  --data FILE --out FILE [--limit N] [--seed N] [--max-words N]
              [--shard-index I --shard-count N] [--cache DIR] [--processor P]
  eval report --scores FILE[,FILE...] [--folds N] [--fpr F] [--threshold F]
              [--bootstrap N] [--json FILE] [--name NAME] [--min-group N]

score  load MAGE-style text,label,src CSV, score a sample, and append results.
report  compute ROC-AUC, a cross-fitted threshold, and per-source breakdowns.
`

func runScore(ctx context.Context, args []string, stderr io.Writer) error {
	fs := flag.NewFlagSet("score", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		data       = fs.String("data", "", "CSV with text,label,src columns")
		out        = fs.String("out", "", "append scored results to this CSV")
		cache      = fs.String("cache", "", "directory for llama.cpp and the models")
		processor  = fs.String("processor", "cpu", "llama.cpp build to use")
		limit      = fs.Int("limit", 0, "score a random sample of this size; 0 for all")
		seed       = fs.Uint64("seed", 1, "seed for the sample")
		maxWords   = fs.Int("max-words", 0, "score only the first N words of each text; 0 for all")
		shardIndex = fs.Int("shard-index", 0, "index of this shard, from 0")
		shardCount = fs.Int("shard-count", 1, "number of shards run in parallel")
		resume     = fs.Bool("resume", true, "skip examples already in --out")
		quiet      = fs.Bool("quiet", false, "print only the summary")
	)
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if *data == "" {
		return errors.New("score: --data is required")
	}
	if *out == "" {
		return errors.New("score: --out is required")
	}
	if *shardCount < 1 || *shardIndex < 0 || *shardIndex >= *shardCount {
		return fmt.Errorf("score: shard %d of %d", *shardIndex, *shardCount)
	}
	if *cache == "" {
		dir, err := llm.CacheDir()
		if err != nil {
			return err
		}
		*cache = dir
	}

	file, err := os.Open(*data)
	if err != nil {
		return err
	}
	examples, err := eval.Load(file)
	_ = file.Close()
	if err != nil {
		return fmt.Errorf("%s: %w", *data, err)
	}
	sample := eval.Shard(eval.Sample(examples, *limit, *seed), *shardIndex, *shardCount)

	done := make(map[int]bool)
	if *resume {
		if _, err := os.Stat(*out); err == nil {
			previous, err := eval.ReadObservations(*out)
			if err != nil {
				return fmt.Errorf("%s: %w", *out, err)
			}
			for _, o := range previous {
				done[o.ID] = true
			}
		}
	}

	var results *eval.Observations
	if *resume {
		results, err = eval.AppendObservations(*out)
	} else {
		results, err = eval.CreateObservations(*out)
	}
	if err != nil {
		return err
	}
	defer func() { _ = results.Close() }()

	remaining := 0
	for _, example := range sample {
		if !done[example.ID] {
			remaining++
		}
	}
	if !*quiet {
		fmt.Fprintf(stderr, "%s: %d of %d examples, %d already scored, %d to go\n",
			*data, len(sample), len(examples), len(done), remaining)
	}
	if remaining == 0 {
		return nil
	}

	d, err := binoculars.Open(ctx, *cache, *processor, stderr)
	if err != nil {
		return err
	}
	defer func() { _ = d.Close() }()

	started := time.Now()
	scored, failed := 0, 0
	for _, example := range sample {
		if done[example.ID] {
			continue
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		text := eval.TruncateWords(example.Text, *maxWords)
		begin := time.Now()
		result, err := d.Score(ctx, text)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			failed++
			fmt.Fprintf(stderr, "id %d: %v\n", example.ID, err)
			continue
		}
		observation := eval.Observation{
			Example: example,
			Score:   result.Score,
			LogPPL:  result.LogPPL,
			LogXPPL: result.LogXPPL,
			Tokens:  len(result.Tokens),
			Words:   len(strings.Fields(text)),
		}
		if err := results.Write(observation); err != nil {
			return err
		}
		scored++
		if !*quiet {
			label := "human"
			if example.Machine {
				label = "machine"
			}
			eta := time.Duration(0)
			if scored > 0 {
				eta = time.Since(started) / time.Duration(scored) * time.Duration(remaining-scored-failed)
			}
			fmt.Fprintf(stderr, "[%d/%d] id=%d %-7s %dt %0.3fs score=%.4f eta=%s\n",
				scored+failed, remaining, example.ID, label, observation.Tokens,
				time.Since(begin).Seconds(), result.Score, eta.Round(time.Second))
		}
	}
	if !*quiet {
		fmt.Fprintf(stderr, "scored %d in %s (%d failed)\n", scored, time.Since(started).Round(time.Second), failed)
	}
	if failed > 0 {
		return fmt.Errorf("%d examples failed to score", failed)
	}
	return nil
}
