// Package cli implements the pkglint command: it statically analyzes
// PKGBUILDs and install scriptlets — never sourcing them — and reports
// integrity, hermeticity, and code-execution findings with an overall letter
// grade per package.
//
// The command lives here rather than in the root main package so that root
// stays a one-line shim: the entry point users install is still
// `go install github.com/jmelahman/pkglint@latest`, and the ~1600 lines behind
// it sit beside the packages they drive.
package cli

import (
	"context"
	// md5 and sha1 are here to *check* the weak digests a PKGBUILD already
	// carries, never to produce a new one — see localDigest.
	"crypto/md5"  //nolint:gosec
	"crypto/sha1" //nolint:gosec
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/jmelahman/pkglint/internal/alpmdb"
	"github.com/jmelahman/pkglint/internal/pkgbuild"
	"github.com/jmelahman/pkglint/internal/pkgfile"
	"github.com/jmelahman/pkglint/internal/report"
	"github.com/jmelahman/pkglint/internal/rules"
)

var version = "dev"

// Main runs pkglint against the process arguments and returns the exit code
// its callers pass to os.Exit.
func Main() int {
	return run(os.Args[1:], os.Stdout)
}

func run(args []string, stdout io.Writer) int {
	var (
		opts       reportOpts
		listRules  bool
		doFix      bool
		unsafeFix  bool
		diff       bool
		offline    bool
		noInline   bool
		addIgnores bool
	)

	code := 0
	cmd := &cobra.Command{
		Use:   "pkglint [flags] [path ...]",
		Short: "security-focused linter for Arch Linux packages",
		Long: `pkglint statically analyzes PKGBUILDs and install scriptlets — never
sourcing them — and reports integrity, hermeticity, and code-execution
findings with an overall letter grade per package. Built packages
(*.pkg.tar.*) are analyzed too: ELF hardening and placement, dependencies
inferred from linked libraries and script interpreters, and filesystem
hygiene — never executing anything from the package.

paths are package directories, PKGBUILD files, or built package
archives (default: .)`,
		Version:       version,
		Args:          cobra.ArbitraryArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(_ *cobra.Command, paths []string) error {
			if listRules {
				colorize, err := colorEnabled(opts.color, stdout)
				if err != nil {
					return err
				}
				report.RenderRules(stdout, rules.Registry(), colorize)
				return nil
			}
			if err := opts.checkSelect(); err != nil {
				return err
			}
			if addIgnores {
				if doFix || unsafeFix || noInline {
					return fmt.Errorf("--add-ignores cannot be combined with --fix, --unsafe-fix, or --no-inline-ignores")
				}
				code = runAddIgnores(paths, opts.disabled(), diff, stdout)
				return nil
			}
			if doFix || unsafeFix {
				level := rules.FixSafe
				if unsafeFix {
					level = rules.FixUnsafe
				}
				code = runFix(paths, opts.disabled(), level, diff, offline, noInline, stdout)
				return nil
			}
			code = lint(paths, opts, noInline, stdout)
			return nil
		},
	}
	addReportFlags(cmd.Flags(), &opts)
	cmd.Flags().BoolVar(&listRules, "rules", false, "print all rules and exit")
	cmd.Flags().BoolVar(&doFix, "fix", false, "apply safe auto-fixes in place")
	cmd.Flags().BoolVar(&unsafeFix, "unsafe-fix", false, "apply safe and behavior-changing auto-fixes in place (implies --fix)")
	cmd.Flags().BoolVar(&diff, "diff", false, "with --fix/--unsafe-fix/--add-ignores: show changes instead of writing them")
	cmd.Flags().BoolVar(&offline, "offline", false, "with --fix: skip fixes needing network access (resolving VCS refs, verifying an https URL answers before rewriting a source's transport)")
	cmd.Flags().BoolVar(&noInline, "no-inline-ignores", false, "disregard '# pkglint: ignore=' directives, reporting the findings they suppress (audit a package without trusting its annotations)")
	cmd.Flags().BoolVar(&addIgnores, "add-ignores", false, "insert '# pkglint: ignore=' directives suppressing every current finding")
	cmd.AddCommand(newBuildCommand(stdout, &code))
	cmd.AddCommand(newExplainCommand(stdout))
	// `build`, `explain` and `help` are the only words the root command can no
	// longer treat as a path; cobra's generated `completion` subcommand would
	// widen that set for a feature this CLI does not otherwise offer.
	cmd.CompletionOptions.DisableDefaultCmd = true
	cmd.SetVersionTemplate("pkglint {{.Version}}\n")
	cmd.SetArgs(args)
	cmd.SetOut(stdout)
	cmd.SetErr(os.Stderr)

	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "pkglint:", err)
		fmt.Fprintln(os.Stderr, "run 'pkglint --help' for usage")
		return 2
	}
	return code
}

// reportOpts is the flag set shared by the root command and `build`: which
// rules run, how their findings are rendered, and which of them fail the run.
type reportOpts struct {
	format  string
	color   string
	failOn  string
	ignore  string
	only    string // --select; `select` is a keyword
	verbose bool
}

// addReportFlags registers reportOpts on a command's own flag set. The root
// command declares these locally rather than persistently, so `build` gets its
// own copies with identical names and defaults instead of inheriting them.
func addReportFlags(fs *pflag.FlagSet, o *reportOpts) {
	fs.StringVar(&o.format, "format", "text", "output format: text, json, or sarif")
	fs.StringVar(&o.color, "color", "auto", "colorize text output: auto, always, or never")
	fs.StringVar(&o.failOn, "fail-on", "warn", "exit non-zero when a finding is at or above this severity (info, warn, error, critical, or never)")
	fs.StringVar(&o.ignore, "ignore", "", "comma-separated rule IDs to disable, e.g. PB105,PB206")
	fs.StringVar(&o.only, "select", "", "comma-separated rule IDs to check, running no others (--ignore still subtracts from them), e.g. PB101,PB304")
	fs.BoolVar(&o.verbose, "verbose", false, "text output: list packages with no findings individually instead of only in the summary")
}

// validate checks the rendering flags up front. Linting reports these errors
// at render time, which is late but costs nothing; a build must not spend a
// full makepkg run to discover a typo in --fail-on.
func (o reportOpts) validate(w io.Writer) error {
	switch o.format {
	case "text", "json", "sarif":
	default:
		return fmt.Errorf("unknown format %q (want text, json, or sarif)", o.format)
	}
	if _, err := colorEnabled(o.color, w); err != nil {
		return err
	}
	if o.failOn != "never" {
		if _, err := rules.ParseSeverity(o.failOn); err != nil {
			return err
		}
	}
	return o.checkSelect()
}

// checkSelect rejects a --select that names a rule the registry does not have.
// --ignore lets an unknown ID through, since ignoring a rule that does not
// exist changes nothing; selecting one would run nothing and report every
// package clean, which is the wrong way for a typo to fail. Every command
// checks this before it does anything, so a build never spends a makepkg run
// on it.
func (o reportOpts) checkSelect() error {
	if o.only == "" {
		return nil
	}
	ids := splitIDs(o.only)
	if len(ids) == 0 {
		return fmt.Errorf("--select names no rules")
	}
	for _, id := range ids {
		if _, ok := rules.RuleByID(id); !ok {
			return fmt.Errorf("--select: unknown rule %q (run 'pkglint --rules' to list them)", id)
		}
	}
	return nil
}

// disabled is the set of rule IDs that do not run: every rule outside --select
// when it is given, plus everything in --ignore. The rules package takes it as
// its ignore set, so downstream a rule left out of --select is exactly a rule
// named in --ignore: lint, --fix, --add-ignores, and the build gate all narrow
// with it. That last one is fine for the same reason --ignore is: the flag is
// the operator's choice, not the analyzed file's.
func (o reportOpts) disabled() map[string]bool {
	disabled := map[string]bool{}
	for _, id := range splitIDs(o.ignore) {
		disabled[id] = true
	}
	selected := splitIDs(o.only)
	if len(selected) == 0 {
		return disabled
	}
	for _, r := range rules.Registry() {
		if !slices.Contains(selected, r.ID) {
			disabled[r.ID] = true
		}
	}
	return disabled
}

// splitIDs parses a comma-separated rule-ID list, in order, blanks dropped.
func splitIDs(csv string) []string {
	var ids []string
	for _, id := range strings.Split(csv, ",") {
		if id = strings.TrimSpace(id); id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

// lint runs the rules over each path and renders the reports, returning the
// process exit code. A path may be a package directory / PKGBUILD (static
// PKGBUILD analysis) or a built .pkg.tar.* archive (package analysis).
// noInline disregards the packages' own inline-ignore directives, so findings
// they suppress are reported anyway. verbose lists findings-free packages in
// text output, which otherwise only counts them in the closing summary.
func lint(paths []string, o reportOpts, noInline bool, stdout io.Writer) int {
	ignored := o.disabled()

	if len(paths) == 0 {
		paths = []string{"."}
	}

	localDB := newLocalDB(alpmdb.DefaultRoot, os.Stderr)

	var reports []report.PackageReport
	for _, path := range paths {
		if pkgfile.IsPackagePath(path) {
			pkg, err := pkgfile.Load(path)
			if err != nil {
				reports = append(reports, report.NewError(path, err))
				continue
			}
			reports = append(reports, report.New(path, rules.RunPackage(pkg, localDB(), ignored)))
			continue
		}
		pkg, err := pkgbuild.Load(path)
		if err != nil {
			reports = append(reports, report.NewError(path, err))
			continue
		}
		if noInline {
			pkg.Suppressions = nil
		}
		findings := rules.Run(pkg, ignored)
		reports = append(reports, report.New(path, findings))
	}

	return renderReports(stdout, reports, o)
}

// newLocalDB defers loading the pacman local database at root until a rule
// that needs it actually runs — it backs the dependency-inference rules,
// which only fire over a built package archive. A missing database (a
// non-Arch host) yields nil, which disables just those rules, silently: that
// is the expected state of most machines. A database that is there but
// cannot be read is a different thing — the rules would be disabled on a
// host where they should work — so that is said once, on warn, and the
// rules are disabled visibly rather than quietly.
func newLocalDB(root string, warn io.Writer) func() *alpmdb.DB {
	var db *alpmdb.DB
	loaded := false
	return func() *alpmdb.DB {
		if !loaded {
			loaded = true
			var err error
			db, err = alpmdb.Load(root)
			if err != nil {
				fmt.Fprintf(warn, "pkglint: %v; the dependency rules (PB8xx) are disabled\n", err)
			}
		}
		return db
	}
}

// renderReports writes the reports in the requested format and returns the
// process exit code: 1 when a finding reaches the --fail-on threshold (a
// package that failed to load always does), 2 when rendering itself failed.
func renderReports(stdout io.Writer, reports []report.PackageReport, o reportOpts) int {
	switch o.format {
	case "json":
		if err := report.RenderJSON(stdout, reports); err != nil {
			fmt.Fprintln(os.Stderr, "pkglint:", err)
			return 2
		}
	case "sarif":
		if err := report.RenderSARIF(stdout, reports); err != nil {
			fmt.Fprintln(os.Stderr, "pkglint:", err)
			return 2
		}
	case "text":
		colorize, err := colorEnabled(o.color, stdout)
		if err != nil {
			fmt.Fprintln(os.Stderr, "pkglint:", err)
			return 2
		}
		report.RenderText(stdout, reports, colorize, o.verbose)
	default:
		fmt.Fprintf(os.Stderr, "pkglint: unknown format %q\n", o.format)
		return 2
	}

	if o.failOn == "never" {
		return 0
	}
	sev, err := rules.ParseSeverity(o.failOn)
	if err != nil {
		fmt.Fprintln(os.Stderr, "pkglint:", err)
		return 2
	}
	if report.ExceedsThreshold(reports, sev) {
		return 1
	}
	return 0
}

// colorEnabled decides whether text output gets ANSI colors. "auto" enables
// them only when writing to a terminal, and defers to the NO_COLOR convention
// (https://no-color.org) and TERM=dumb; an explicit --color=always overrides
// both.
func colorEnabled(mode string, w io.Writer) (bool, error) {
	switch mode {
	case "always":
		return true, nil
	case "never":
		return false, nil
	case "auto":
		if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
			return false, nil
		}
		f, ok := w.(*os.File)
		if !ok {
			return false, nil
		}
		// A terminal presents as a character device; a pipe or regular file
		// does not. This keeps colors out of redirected output without a
		// platform-specific isatty call.
		fi, err := f.Stat()
		return err == nil && fi.Mode()&os.ModeCharDevice != 0, nil
	default:
		return false, fmt.Errorf("unknown color mode %q (want auto, always, or never)", mode)
	}
}

// rewriteMode is one of the modes that rewrite a PKGBUILD on disk: --fix and
// --add-ignores. Both load each path, run a transform, print every edit,
// write the result unless --diff, and summarise per path. Only the transform
// and the wording differ, and this keeps them from drifting apart.
type rewriteMode struct {
	// refuse is printed for a built package archive, which has no PKGBUILD
	// to rewrite. It takes the display path.
	refuse string
	// transform computes the rewrites for one loaded package.
	transform func(pkg *pkgbuild.Package) []rules.FixResult
	// after runs once per path after the edits are printed and written and
	// before the summary, with the loaded package and the number of edits
	// applied; nil to skip. It is how --fix prints its manual nudges.
	after func(path string, pkg *pkgbuild.Package, applied int)
	// none, dryRun and applied are the per-path summaries. none takes the
	// display path; dryRun and applied take the display path then the count.
	none, dryRun, applied string
}

// run applies the mode to every path (the working directory when there are
// none) and returns the exit code: 0, or 2 when a package failed to load or
// a rewritten file could not be written.
func (m rewriteMode) run(paths []string, diff bool, stdout io.Writer) int {
	if len(paths) == 0 {
		paths = []string{"."}
	}
	rc := 0
	for _, path := range paths {
		if pkgfile.IsPackagePath(path) {
			fmt.Fprintf(stdout, m.refuse, rel(path))
			continue
		}
		pkg, err := pkgbuild.Load(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "pkglint: %s: %v\n", path, err)
			rc = 2
			continue
		}
		applied, ok := applyFixResults(stdout, m.transform(pkg), diff)
		if !ok {
			rc = 2
		}
		if m.after != nil {
			m.after(path, pkg, applied)
		}
		switch {
		case applied == 0:
			fmt.Fprintf(stdout, m.none, rel(path))
		case diff:
			fmt.Fprintf(stdout, m.dryRun, rel(path), applied)
		default:
			fmt.Fprintf(stdout, m.applied, rel(path), applied)
		}
	}
	return rc
}

// applyFixResults prints every edit in results — with its hunk under --diff —
// and writes the rewritten units unless --diff. It returns how many edits were
// printed and whether every write succeeded; a failed write is reported on
// stderr and leaves the rest of the results to be applied.
//
// Both rewriting paths go through it: `--fix`/`--add-ignores`, which transform
// a PKGBUILD in one pass, and `build --fix`, which transforms it from what the
// build produced. Keeping the printing and the writing in one place is what
// makes the two read identically.
func applyFixResults(stdout io.Writer, results []rules.FixResult, diff bool) (applied int, ok bool) {
	ok = true
	for _, r := range results {
		if !r.Changed() {
			continue
		}
		for _, e := range r.Applied {
			applied++
			fmt.Fprintf(stdout, "%s:%d: [%s] %s\n", rel(e.Path), e.Line, e.RuleID, e.Desc)
			if diff {
				fmt.Fprint(stdout, editHunk(r.Original, e))
			}
		}
		if diff {
			continue
		}
		if err := writeFixed(r.Path, r.Fixed); err != nil {
			fmt.Fprintf(os.Stderr, "pkglint: writing %s: %v\n", r.Path, err)
			ok = false
		}
	}
	return applied, ok
}

// runFix applies auto-fixes at the given level, writing files in place (or,
// with diff, printing the changes it would make). It also nudges toward the
// manual follow-ups for findings that can't be rewritten mechanically.
// noInline disregards inline-ignore directives, so suppressed fixes apply too.
func runFix(paths []string, ignore map[string]bool, level rules.FixLevel, diff, offline, noInline bool, stdout io.Writer) int {
	// LocalDigest is not gated on --offline: it only reads files that are
	// already on disk, so there is no network access for --offline to skip.
	env := &rules.FixEnv{LocalDigest: localDigest}
	if !offline {
		env.ResolveRef = resolveGitRef
		env.ProbeHTTPS = probeHTTPS
	}
	return rewriteMode{
		refuse: "%s: built packages have no auto-fixable findings; fix the PKGBUILD and rebuild\n",
		transform: func(pkg *pkgbuild.Package) []rules.FixResult {
			if noInline {
				pkg.Suppressions = nil
			}
			return rules.Fix(pkg, ignore, level, env)
		},
		after: func(path string, pkg *pkgbuild.Package, applied int) {
			// Nudge on what is *left*. Some fixes only sometimes apply — PB102's
			// needs the sources on disk — so suggestions read from the pre-fix
			// package would keep telling the user to run updpkgsums for a digest
			// that has just been written. Re-reading what was actually saved is
			// the only account of the remaining findings that cannot drift from
			// the file; with --diff nothing was written, so the original stands.
			residual := pkg
			if !diff && applied > 0 {
				if reloaded, err := pkgbuild.Load(path); err == nil {
					if noInline {
						reloaded.Suppressions = nil
					}
					residual = reloaded
				}
			}
			for _, s := range manualSuggestions(residual, ignore) {
				fmt.Fprintf(stdout, "%s: %s\n", rel(path), s)
			}
		},
		none:    "%s: no auto-fixable findings\n",
		dryRun:  "%s: %d fix(es) would be applied (dry run)\n",
		applied: "%s: applied %d fix(es)\n",
	}.run(paths, diff, stdout)
}

// runAddIgnores inserts inline-ignore directives suppressing every finding the
// packages currently report (or, with diff, prints the insertions it would
// make). It is the reviewed-and-accepted counterpart to --fix: the findings
// stay in the file as annotations instead of being repaired.
func runAddIgnores(paths []string, ignore map[string]bool, diff bool, stdout io.Writer) int {
	return rewriteMode{
		refuse:    "%s: built packages cannot carry ignore directives; annotate the PKGBUILD instead\n",
		transform: func(pkg *pkgbuild.Package) []rules.FixResult { return rules.AddIgnores(pkg, ignore) },
		none:      "%s: no findings to suppress\n",
		dryRun:    "%s: %d ignore directive(s) would be added (dry run)\n",
		applied:   "%s: added %d ignore directive(s)\n",
	}.run(paths, diff, stdout)
}

// manualSuggestions returns one-line nudges for findings whose remediation is
// real but not a mechanical rewrite pkglint should perform for the user.
func manualSuggestions(pkg *pkgbuild.Package, ignore map[string]bool) []string {
	byRule := map[string]bool{}
	for _, f := range rules.Run(pkg, ignore) {
		byRule[f.RuleID] = true
	}
	var out []string
	if byRule["PB101"] || byRule["PB102"] || byRule["PB104"] {
		out = append(out, "run `updpkgsums` to (re)generate strong checksums for the sources")
	}
	if byRule["PB601"] {
		out = append(out, "run `makepkg --printsrcinfo > .SRCINFO` to regenerate .SRCINFO from the PKGBUILD")
	}
	return out
}

// localDigest hashes an already-downloaded source and is the only file access
// the fix path performs outside the package directory. It does no network I/O:
// a source that has not been fetched has no digest here, and PB102's fixer
// leaves the finding alone rather than going and getting one.
//
// filename comes from a source=() entry — including the fully maintainer-
// controlled name before a `::` — so it is required to be a bare filename.
// Without that check a crafted entry like `../../.ssh/id_ed25519::https://…`
// would walk the fixer out of the directories it is meant to read, and the
// digest of whatever it found would be written into the PKGBUILD.
func localDigest(dir, filename string) (rules.Digests, error) {
	if filename == "" || filename == "." || filename == ".." || filename != filepath.Base(filename) {
		return rules.Digests{}, fmt.Errorf("refusing to hash non-filename source %q", filename)
	}
	// The package directory first — local sources and default makepkg downloads
	// land there — then $SRCDEST, where makepkg parks shared downloads when the
	// user configures one (makepkg.conf ships it commented out).
	dirs := []string{dir}
	if d := os.Getenv("SRCDEST"); d != "" {
		dirs = append(dirs, d)
	}
	for _, d := range dirs {
		f, err := os.Open(filepath.Join(d, filename))
		if err != nil {
			continue
		}
		digests, err := hashAll(f)
		_ = f.Close()
		if err != nil {
			return rules.Digests{}, err
		}
		return digests, nil
	}
	return rules.Digests{}, fmt.Errorf("%s: not downloaded", filename)
}

// hashAll computes every digest the fixer compares in one pass, so the weak
// digest it verifies and the strong one it writes cannot describe different
// bytes.
func hashAll(r io.Reader) (rules.Digests, error) {
	m, s1, s256 := md5.New(), sha1.New(), sha256.New()
	if _, err := io.Copy(io.MultiWriter(m, s1, s256), r); err != nil {
		return rules.Digests{}, err
	}
	return rules.Digests{
		MD5:    hex.EncodeToString(m.Sum(nil)),
		SHA1:   hex.EncodeToString(s1.Sum(nil)),
		SHA256: hex.EncodeToString(s256.Sum(nil)),
	}, nil
}

// gitTransports are the URL schemes `git ls-remote` may be pointed at. The
// URL comes from an untrusted source=() entry, so everything else is refused:
// notably file:// and ext:: (which git treats as "run this command"), and
// anything without a scheme (which git could read as an option or a local
// path). These cover every transport a real AUR git source uses — makepkg's
// git+ prefix strips to one of them, and scp-style git@host:repo URLs never
// reach here because they parse as local sources.
var gitTransports = map[string]bool{"https": true, "http": true, "git": true, "ssh": true}

// allowedGitURL reports whether url names a remote over an allowed transport.
// Schemes are case-insensitive per RFC 3986; the URL itself is passed to git
// unchanged.
func allowedGitURL(url string) bool {
	scheme, _, ok := strings.Cut(url, "://")
	return ok && gitTransports[strings.ToLower(scheme)]
}

// resolveGitRef resolves a git tag or branch name on a remote to its commit
// hash via `git ls-remote` — the only network access the fix path performs.
func resolveGitRef(rawurl, ref string) (string, error) {
	url := rawurl
	if i := strings.Index(url, "://"); i > 0 {
		if plus := strings.IndexByte(url[:i], '+'); plus >= 0 {
			url = url[plus+1:] // git+https://… → https://…
		}
	}
	if !allowedGitURL(url) {
		return "", fmt.Errorf("refusing to resolve ref over unsupported URL scheme: %q", url)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "ls-remote", url, ref)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0") // never block on credentials
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	var sha string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		f := strings.Fields(line)
		if len(f) < 2 {
			continue
		}
		if strings.HasSuffix(f[1], "^{}") { // peeled commit of an annotated tag
			return f[0], nil
		}
		if sha == "" {
			sha = f[0]
		}
	}
	if sha == "" {
		return "", fmt.Errorf("ref %q not found on %s", ref, url)
	}
	return sha, nil
}

// probeTimeout bounds one https probe end to end, redirects included. A host
// that has not answered by then is one the fix should not bet a build on.
const probeTimeout = 15 * time.Second

// probeHTTPS reports whether an https URL is served, by asking for headers and
// nothing else. It is how PB104's rewrite is checked before it is written: the
// linter would otherwise be asserting, on a maintainer's behalf, that a host it
// never contacted offers the same path over TLS.
//
// The request is deliberately minimal. HEAD first, a single-byte ranged GET
// only where a server rejects HEAD outright (405 and 501 are common on older
// CDNs, and some object stores answer 403 to it), no body is ever read, and
// nothing that comes back is used for anything but the reachable/not decision.
// Credentials in the URL are dropped rather than sent, and a redirect is
// followed only while it stays on https — a hop to http would mean the https
// URL does not really serve the file, which is the opposite of what the fix
// claims.
func probeHTTPS(rawurl string) error { return probeWith(newProbeClient(), rawurl) }

// newProbeClient is the client every probe goes through: no redirect off
// https, a bounded number of hops, and a dialer that refuses to leave the
// public internet.
func newProbeClient() *http.Client {
	return &http.Client{
		Timeout:   probeTimeout,
		Transport: &http.Transport{DialContext: (&net.Dialer{Timeout: 5 * time.Second, Control: refuseInternalAddr}).DialContext},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("too many redirects")
			}
			if !strings.EqualFold(req.URL.Scheme, "https") {
				return fmt.Errorf("redirected off https to %s", req.URL.Scheme)
			}
			return nil
		},
	}
}

func probeWith(client *http.Client, rawurl string) error {
	u, err := url.Parse(rawurl)
	if err != nil {
		return fmt.Errorf("unparseable URL %q: %w", rawurl, err)
	}
	if !strings.EqualFold(u.Scheme, "https") || u.Host == "" {
		return fmt.Errorf("refusing to probe non-https URL %q", rawurl)
	}
	u.User = nil // never replay credentials a PKGBUILD embedded in a URL

	status, err := probeOnce(client, http.MethodHead, u.String(), nil)
	if err != nil {
		return err
	}
	if status == http.StatusMethodNotAllowed || status == http.StatusNotImplemented || status == http.StatusForbidden {
		// The server dislikes HEAD, not the URL. Ask for one byte instead.
		status, err = probeOnce(client, http.MethodGet, u.String(), map[string]string{"Range": "bytes=0-0"})
		if err != nil {
			return err
		}
	}
	if status >= 400 {
		return fmt.Errorf("%s returned HTTP %d", u.Redacted(), status)
	}
	return nil
}

// probeOnce issues one request and returns its status code, discarding the
// response body unread.
func probeOnce(client *http.Client, method, url string, headers map[string]string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", "pkglint/"+version)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	resp.Body.Close()
	return resp.StatusCode, nil
}

// refuseInternalAddr blocks a probe that would leave the public internet. The
// URL comes out of a file pkglint is analyzing precisely because nobody has
// vetted it yet, and "linting this PKGBUILD made my machine knock on the cloud
// metadata service" is not a thing a linter should be able to do. Checking the
// address at dial time rather than the hostname up front is what makes it hold:
// the name is resolved by then, so a hostname pointed at 127.0.0.1 or
// 169.254.169.254 is caught along with the literal.
//
// The cost is that a source on an internal mirror cannot be verified and so
// keeps its finding. That is the right way round: pkglint declining to fix is
// recoverable, and reaching into a private network on a hostile file's say-so
// is not.
func refuseInternalAddr(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("unroutable probe address %q", address)
	}
	ip := net.ParseIP(host)
	switch {
	case ip == nil:
		return fmt.Errorf("unresolvable probe address %q", host)
	case ip.IsLoopback(), ip.IsPrivate(), ip.IsLinkLocalUnicast(), ip.IsLinkLocalMulticast(),
		ip.IsInterfaceLocalMulticast(), ip.IsMulticast(), ip.IsUnspecified(), !ip.IsGlobalUnicast():
		return fmt.Errorf("refusing to probe non-public address %s", ip)
	}
	return nil
}

// writeFixed replaces the file at path with data, preserving the file's
// existing permissions. The bytes go to a temp file in the same directory
// first and land by rename, so a crash or a full disk mid-write leaves the
// PKGBUILD the user had rather than a truncated one. A symlinked PKGBUILD is
// written through to its target, as the plain write did.
func writeFixed(path string, data []byte) error {
	if real, err := filepath.EvalSymlinks(path); err == nil {
		path = real
	}
	mode := os.FileMode(0o644)
	if fi, err := os.Stat(path); err == nil {
		mode = fi.Mode().Perm()
	}
	f, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	fail := func(err error) error {
		f.Close()
		os.Remove(tmp)
		return err
	}
	// CreateTemp opens 0600 regardless of umask; the chmod is what carries
	// the original mode across the rename.
	if err := f.Chmod(mode); err != nil {
		return fail(err)
	}
	if _, err := f.Write(data); err != nil {
		return fail(err)
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// rel shortens path relative to the working directory for display.
func rel(path string) string {
	if wd, err := os.Getwd(); err == nil {
		if r, err := filepath.Rel(wd, path); err == nil && !strings.HasPrefix(r, "..") {
			return r
		}
	}
	return path
}

// editHunk renders a git-style before/after fragment for one edit, computed
// against the original bytes so line numbers stay stable across edits.
func editHunk(orig []byte, e rules.Edit) string {
	ls := e.Start
	for ls > 0 && orig[ls-1] != '\n' {
		ls--
	}
	le := e.End
	if !(le > 0 && le <= len(orig) && orig[le-1] == '\n') {
		for le < len(orig) && orig[le] != '\n' {
			le++
		}
	}
	before := strings.TrimRight(string(orig[ls:le]), "\n")
	after := strings.TrimRight(string(orig[ls:e.Start])+e.New+string(orig[e.End:le]), "\n")
	var b strings.Builder
	for _, l := range strings.Split(before, "\n") {
		if l != "" {
			fmt.Fprintf(&b, "    - %s\n", l)
		}
	}
	for _, l := range strings.Split(after, "\n") {
		if l != "" {
			fmt.Fprintf(&b, "    + %s\n", l)
		}
	}
	return b.String()
}
