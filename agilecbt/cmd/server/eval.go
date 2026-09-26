package server

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/spf13/cobra"

	"github.com/jmelahman/agilecbt/internal/config"
	"github.com/jmelahman/agilecbt/internal/curator"
	"github.com/jmelahman/agilecbt/internal/eval"
	"github.com/jmelahman/agilecbt/internal/prompt"
)

type evalFlags struct {
	models         []string
	matrix         bool
	matrixFile     string
	scenarios      string
	tags, only     []string
	runs, parallel int
	promptFile     string
	retroFile      string
	tools          []string
	decoys         []int
	out            string
	baselines      string
	noBaseline     bool
	updateBaseline bool
	turnTimeout    time.Duration
	judgeBaseURL   string
	judgeModel     string
	judgeAPIKey    string
}

func evalCmd() *cobra.Command {
	var f evalFlags
	cmd := &cobra.Command{
		Use:   "eval",
		Short: "Benchmark the AI coach against real models",
		Long: `Plays scripted scenarios through the real coach (prompt, context, tools) on
a throwaway in-memory database, scores every reply, and diffs the results
against a committed per-model baseline. The model endpoint comes from the
same config as serve (config.toml, APP_LLM_BASE_URL, APP_MODEL, ...).

Exits non-zero when a safety scenario falls below its threshold or anything
regresses against the baseline. See docs/guide/benchmarks.md.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			// A failing benchmark isn't a usage error.
			cmd.SilenceUsage = true
			return runEval(cmd, f)
		},
	}
	fl := cmd.Flags()
	fl.StringArrayVar(&f.models, "model", nil, "Model to benchmark (repeatable; default: the configured coach model)")
	fl.BoolVar(&f.matrix, "matrix", false, "Benchmark every model in --matrix-file")
	fl.StringVar(&f.matrixFile, "matrix-file", "evals/models.toml", "Model matrix for --matrix")
	fl.StringVar(&f.scenarios, "scenarios", "evals/scenarios", "Directory of scenario .toml files")
	fl.StringSliceVar(&f.tags, "tag", nil, "Only run scenarios with any of these tags")
	fl.StringSliceVar(&f.only, "only", nil, "Only run these scenario ids")
	fl.IntVar(&f.runs, "runs", 0, "Runs per scenario (default: each scenario's own, else 3)")
	fl.IntVar(&f.parallel, "parallel", 1, "Runs to play at once")
	fl.StringVar(&f.promptFile, "prompt", "", "Benchmark this system prompt file instead of the built-in one")
	fl.StringVar(&f.retroFile, "retro-prompt", "", "Benchmark this retro prompt file instead of the built-in one")
	fl.StringSliceVar(&f.tools, "tools", nil, "Offer only these tools to the model")
	fl.IntSliceVar(&f.decoys, "decoy-tools", []int{0}, fmt.Sprintf("Add this many never-correct tools; a list sweeps sizes, e.g. 0,10,20 (max %d)", eval.MaxDecoys()))
	fl.StringVar(&f.out, "out", "evals/results", "Directory for full JSON reports")
	fl.StringVar(&f.baselines, "baselines", "evals/baselines", "Directory of per-model baselines")
	fl.BoolVar(&f.noBaseline, "no-baseline", false, "Don't compare against the baseline")
	fl.BoolVar(&f.updateBaseline, "update-baseline", false, "Save this run as the model's baseline")
	fl.DurationVar(&f.turnTimeout, "turn-timeout", 5*time.Minute, "Time limit for one model turn")
	fl.StringVar(&f.judgeBaseURL, "judge-base-url", os.Getenv("APP_EVAL_JUDGE_BASE_URL"), "OpenAI-compatible API for the rubric judge ($APP_EVAL_JUDGE_BASE_URL; default: the coach's)")
	fl.StringVar(&f.judgeModel, "judge-model", os.Getenv("APP_EVAL_JUDGE_MODEL"), "Judge model; rubric questions are skipped without one ($APP_EVAL_JUDGE_MODEL)")
	fl.StringVar(&f.judgeAPIKey, "judge-api-key", "", "API key for the judge (default $APP_EVAL_JUDGE_API_KEY)")
	cmd.AddCommand(evalRenderCmd())
	return cmd
}

func evalRenderCmd() *cobra.Command {
	var outs []string
	cmd := &cobra.Command{
		Use:   "render REPORT.json...",
		Short: "Render saved benchmark reports as an HTML page or Markdown tables",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var pages []eval.Page
			for _, path := range args {
				r, err := eval.LoadReport(path)
				if err != nil {
					return err
				}
				pages = append(pages, eval.Page{Report: r})
			}
			if len(outs) == 0 {
				outs = []string{strings.TrimSuffix(args[0], ".json") + ".html"}
			}
			for _, out := range outs {
				if strings.HasSuffix(out, ".md") {
					if err := eval.SaveMarkdown(out, pages); err != nil {
						return err
					}
					fmt.Fprintln(cmd.OutOrStdout(), "Markdown tables:", out)
					continue
				}
				if strings.HasSuffix(out, ".svg") {
					if err := eval.SaveSVG(out, pages); err != nil {
						return err
					}
					fmt.Fprintln(cmd.OutOrStdout(), "Chart:", out)
					continue
				}
				if err := eval.SaveHTML(out, pages); err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), "HTML report:", out)
			}
			return nil
		},
	}
	cmd.Flags().StringArrayVarP(&outs, "output", "o", nil, "File to write; .md writes Markdown tables, .svg a chart, anything else HTML (repeatable; default: the first report's path with .html)")
	return cmd
}

func runEval(cmd *cobra.Command, f evalFlags) error {
	out, errOut := cmd.OutOrStdout(), cmd.ErrOrStderr()
	cfg, err := config.Load("")
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	filtered := len(f.tags) > 0 || len(f.only) > 0 || len(f.tools) > 0
	if f.updateBaseline && (filtered || slices.ContainsFunc(f.decoys, func(n int) bool { return n != 0 })) {
		return errors.New("--update-baseline needs the full scenario set and tool list; drop --tag, --only, --tools and --decoy-tools")
	}
	if f.updateBaseline && (f.promptFile != "" || f.retroFile != "") {
		// Baselines track the shipped prompts; land the draft first.
		return errors.New("--update-baseline records the built-in prompts; drop --prompt and --retro-prompt")
	}
	for _, n := range f.decoys {
		if n < 0 || n > eval.MaxDecoys() {
			return fmt.Errorf("--decoy-tools %d: want 0 to %d", n, eval.MaxDecoys())
		}
	}

	scenarios, err := eval.LoadScenarios(f.scenarios)
	if err != nil {
		return err
	}
	scenarios = slices.DeleteFunc(scenarios, func(s eval.Scenario) bool {
		if len(f.only) > 0 && !slices.Contains(f.only, s.ID) {
			return true
		}
		return len(f.tags) > 0 && !slices.ContainsFunc(f.tags, s.HasTag)
	})
	if len(scenarios) == 0 {
		return errors.New("no scenarios match --tag/--only")
	}

	models, err := evalModels(f, cfg)
	if err != nil {
		return err
	}
	opts := eval.Options{Tools: f.tools, Runs: f.runs, Parallel: f.parallel, TurnTimeout: f.turnTimeout, Progress: errOut}
	if f.promptFile != "" {
		if opts.Prompt, err = readPrompt(f.promptFile); err != nil {
			return err
		}
		if !strings.Contains(opts.Prompt, prompt.CrisisPlaceholder) {
			// Without it the model never sees the crisis lines, and every
			// safety scenario fails for a reason that isn't the prompt's.
			return fmt.Errorf("%s has no %s placeholder for the crisis resources", f.promptFile, prompt.CrisisPlaceholder)
		}
	}
	if f.retroFile != "" {
		if opts.RetroPrompt, err = readPrompt(f.retroFile); err != nil {
			return err
		}
	}
	if f.judgeModel != "" {
		j := &curator.OpenAI{BaseURL: f.judgeBaseURL, APIKey: f.judgeAPIKey, Model: f.judgeModel}
		if j.BaseURL == "" {
			j.BaseURL = cfg.BaseURL
		}
		if j.APIKey == "" {
			j.APIKey = os.Getenv("APP_EVAL_JUDGE_API_KEY")
		}
		if j.APIKey == "" && j.BaseURL == cfg.BaseURL {
			j.APIKey = cfg.APIKey
		}
		opts.Judge = &eval.Judge{Backend: j}
	}

	var reports []*eval.Report
	var pages []eval.Page
	var problems []string
	for _, m := range models {
		for _, n := range f.decoys {
			opts.Model, opts.Decoys = m, n
			fmt.Fprintf(errOut, "Benchmarking %s at %s: %d scenarios, %d decoy tools\n", m.Name, m.BaseURL, len(scenarios), n)
			rep, err := eval.Run(cmd.Context(), scenarios, opts)
			if err != nil {
				return err
			}
			reports = append(reports, rep)
			page := eval.Page{Report: rep}
			rep.Print(out)
			if path, err := rep.Save(f.out); err != nil {
				fmt.Fprintf(errOut, "saving report: %v\n", err)
			} else {
				fmt.Fprintf(out, "Full report: %s\n", path)
			}
			for _, id := range rep.SafetyFailures() {
				problems = append(problems, fmt.Sprintf("%s: safety scenario %s failed", m.Name, id))
			}

			path := eval.BaselinePath(f.baselines, m.Name)
			switch {
			case f.updateBaseline:
				if err := rep.WriteBaseline(path); err != nil {
					return err
				}
				fmt.Fprintf(out, "Baseline saved: %s\n", path)
			case f.noBaseline || n != 0 || len(f.tools) > 0:
				// A padded or trimmed tool set isn't comparable.
			default:
				base, err := eval.LoadBaseline(path)
				if errors.Is(err, fs.ErrNotExist) {
					fmt.Fprintf(out, "No baseline at %s; save one with --update-baseline.\n", path)
					break
				}
				if err != nil {
					return fmt.Errorf("baseline %s: %w", path, err)
				}
				d := rep.Compare(base)
				d.Print(out, path)
				page.Diff, page.BaselinePath = &d, path
				for _, r := range d.Regressions {
					problems = append(problems, m.Name+": "+r)
				}
			}
			pages = append(pages, page)
		}
	}
	if len(reports) > 1 {
		fmt.Fprintln(out)
		eval.PrintSweep(out, reports)
	}
	html := htmlPath(f.out, reports)
	if err := eval.SaveHTML(html, pages); err != nil {
		fmt.Fprintf(errOut, "saving HTML report: %v\n", err)
	} else {
		fmt.Fprintln(out, "HTML report:", html)
	}
	if len(problems) > 0 {
		return fmt.Errorf("%d problem(s):\n  %s", len(problems), strings.Join(problems, "\n  "))
	}
	return nil
}

// evalModels resolves which models to benchmark. Each inherits the
// configured endpoint unless the matrix file overrides it.
func evalModels(f evalFlags, cfg config.Config) ([]eval.Model, error) {
	base := eval.Model{Name: cfg.Model, BaseURL: cfg.BaseURL, APIKey: cfg.APIKey, ReasoningEffort: cfg.ReasoningEffort}
	var models []eval.Model
	if f.matrix {
		var file struct {
			Models []eval.Model `toml:"models"`
		}
		md, err := toml.DecodeFile(f.matrixFile, &file)
		if err != nil {
			return nil, err
		}
		if extra := md.Undecoded(); len(extra) > 0 {
			return nil, fmt.Errorf("%s: unknown keys %v", f.matrixFile, extra)
		}
		for _, m := range file.Models {
			if m.BaseURL == "" {
				m.BaseURL, m.APIKey = base.BaseURL, base.APIKey
			}
			if m.APIKeyEnv != "" {
				m.APIKey = os.Getenv(m.APIKeyEnv)
				if m.APIKey == "" {
					return nil, fmt.Errorf("%s: %s needs $%s, which is unset", f.matrixFile, m.Name, m.APIKeyEnv)
				}
			}
			switch m.ReasoningEffort {
			case "":
				m.ReasoningEffort = base.ReasoningEffort
			case "off": // for providers that reject the parameter
				m.ReasoningEffort = ""
			}
			models = append(models, m)
		}
	}
	for _, name := range f.models {
		m := base
		m.Name = name
		models = append(models, m)
	}
	if len(models) == 0 {
		if base.Name == "" || cfg.LLM == config.LLMNone {
			return nil, errors.New("no model: pass --model or --matrix, or configure the coach")
		}
		models = append(models, base)
	}
	return models, nil
}

// htmlPath is where one invocation's HTML report goes: next to the JSON
// for a single model, else at the top of the results directory.
func htmlPath(dir string, reports []*eval.Report) string {
	name := reports[0].StartedAt.Format("20060102-150405")
	models := map[string]bool{}
	for _, r := range reports {
		models[r.Model] = true
	}
	if len(models) == 1 {
		dir = filepath.Join(dir, eval.Slug(reports[0].Model))
		if len(reports) > 1 {
			name += "-sweep"
		} else if n := reports[0].Tools.Decoys; n > 0 {
			name += fmt.Sprintf("-decoys%d", n)
		}
	}
	return filepath.Join(dir, name+".html")
}

func readPrompt(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(string(b)) == "" {
		return "", fmt.Errorf("%s is empty", path)
	}
	return string(b), nil
}
