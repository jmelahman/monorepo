package cli

// `pkglint build` is the one place pkglint runs a program on the user's
// behalf. Every analysis path in this repo — lint, --fix, --add-ignores, all
// of internal/ — still parses PKGBUILDs and package archives without sourcing
// or executing anything. This command deliberately does execute the PKGBUILD,
// via makepkg, because that is the only way to obtain the artifact the PB8xx
// rules inspect; it is a separately named verb, never reached by
// `pkglint <path>`, and it refuses to build a PKGBUILD whose static findings
// already reach the --fail-on threshold (or are critical, whatever the
// threshold says).
//
// Everything that decides whether to execute is therefore adversarial input,
// and is treated as such: the gate ignores the file's own inline-ignore
// directives, a file argument must be the PKGBUILD makepkg will actually
// build, and makepkg options that repoint the build at some other file are
// refused.

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/jmelahman/pkglint/internal/alpmdb"
	"github.com/jmelahman/pkglint/internal/pkgbuild"
	"github.com/jmelahman/pkglint/internal/pkgfile"
	"github.com/jmelahman/pkglint/internal/report"
	"github.com/jmelahman/pkglint/internal/rules"
)

// runCmd and lookPath are the process seams: the only two ways this package
// reaches outside itself. Tests replace them to exercise the build path on a
// host with neither makepkg nor a container runtime installed.
var (
	runCmd   = func(cmd *exec.Cmd) error { return cmd.Run() }
	lookPath = exec.LookPath
)

// localDBRoot is where the pacman local database is read from. It is a
// variable for the same reason runCmd is: the build path's dependency rules
// (and the fixes they now carry) can only be exercised against a database, and
// a test host is not required to have one.
var localDBRoot = alpmdb.DefaultRoot

// buildOpts are the flags specific to `build`.
type buildOpts struct {
	keep        string
	image       string
	makepkgArgs []string
	force       bool
	docker      bool
	imageSet    bool
	fix         bool
	unsafeFix   bool
	diff        bool
}

// fixLevel is the auto-fix level the flags ask for, or FixNone when neither
// --fix nor --unsafe-fix was given.
func (o buildOpts) fixLevel() rules.FixLevel {
	switch {
	case o.unsafeFix:
		return rules.FixUnsafe
	case o.fix:
		return rules.FixSafe
	default:
		return rules.FixNone
	}
}

// buildDirs are the locations makepkg is redirected to write into, so that
// nothing lands in the package directory itself. A container build fills in
// only pkgdest: everything else it writes stays inside the container.
type buildDirs struct {
	pkgdest    string
	builddir   string
	logdest    string
	srcdest    string
	srcpkgdest string
}

func newBuildCommand(stdout io.Writer, code *int) *cobra.Command {
	var (
		ro reportOpts
		bo buildOpts
	)
	cmd := &cobra.Command{
		Use:   "build [flags] [path ...] [-- makepkg-arg ...]",
		Short: "build each PKGBUILD with makepkg and lint the package it produces",
		Long: `build runs the static PKGBUILD rules, hands the package to makepkg, and
then lints the archive that comes out with the built-package rules (ELF
hardening, dependencies inferred from linked libraries, filesystem hygiene) —
checks no amount of reading the PKGBUILD can answer.

Unlike every other pkglint command, this one executes the PKGBUILD: that is
what makepkg does. It therefore refuses to build a package whose static
findings reach --fail-on, and always refuses on a critical finding no matter
what --fail-on says. --force overrides that. The refusal deliberately does not
honour the PKGBUILD's own '# pkglint: ignore=' directives — a file does not get
to vote on whether it is safe to run — though the report beside it still does.

makepkg is redirected to write into a temporary PKGDEST/BUILDDIR/LOGDEST, and
the PKGBUILD is held read-only while it runs (a pkgver() package's buildfile is
otherwise rewritten in place), so the package directory is left exactly as it
was found; sources are cached in ${XDG_CACHE_HOME:-~/.cache}/pkglint/sources
between runs, and $SRCDEST or $BUILDDIR moves either elsewhere (a container
build is a clean room and downloads its own). Dependencies are not synced by
default, since that needs root — pass '-- -s' (or '--makepkg-arg=-s') to opt in.

--fix writes back what the build taught it: the built-package rules see things
no reading of the PKGBUILD can (which libraries the binaries actually link),
and the fixes for those findings are only computable here. It rewrites the
PKGBUILD that was just built, never another one, and only for a package that
is the single one its PKGBUILD declares; --diff previews without writing. The
PKGBUILD-scope fixes stay with 'pkglint --fix'.

paths are package directories or PKGBUILD files (default: .). Note that
'pkglint build' names this command even when ./build is a package directory;
spell that one 'pkglint ./build'.`,
		Args:          cobra.ArbitraryArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(c *cobra.Command, args []string) error {
			// Arguments after `--` are makepkg's. pre-commit appends the
			// matched filenames after the hook's `args:`, which a `--` would
			// swallow, hence --makepkg-arg as the scriptable equivalent.
			paths := args
			if n := c.ArgsLenAtDash(); n >= 0 {
				paths = args[:n]
				bo.makepkgArgs = append(bo.makepkgArgs, args[n:]...)
			}
			// An explicit --image is a request to build in a container; only
			// $PKGLINT_BUILD_IMAGE is the passive default that an installed
			// makepkg still wins over.
			bo.imageSet = c.Flags().Changed("image")
			if err := ro.validate(stdout); err != nil {
				return err
			}
			if err := checkMakepkgArgs(bo.makepkgArgs); err != nil {
				return err
			}
			if err := checkBuildAmbiguity(paths); err != nil {
				return err
			}
			*code = runBuild(paths, ro, bo, stdout)
			return nil
		},
	}
	addReportFlags(cmd.Flags(), &ro)
	cmd.Flags().BoolVar(&bo.force, "force", false, "build even when the static findings would otherwise refuse it (they still count toward the exit code)")
	cmd.Flags().StringVar(&bo.keep, "keep", "", "copy the built package archives into this directory instead of discarding them")
	cmd.Flags().BoolVar(&bo.docker, "docker", false, "build inside a container instead of on the host, even when makepkg is installed")
	cmd.Flags().StringVar(&bo.image, "image", "", "container image to build in, e.g. archlinux:base-devel; passing it implies --docker (default $PKGLINT_BUILD_IMAGE, which does not)")
	cmd.Flags().StringArrayVar(&bo.makepkgArgs, "makepkg-arg", nil, "extra argument to pass to makepkg; repeatable (the scriptable form of '-- <args>')")
	cmd.Flags().BoolVar(&bo.fix, "fix", false, "write the safe auto-fixes for the built-package findings back into the PKGBUILD (the PB8xx fixes, which only a build can compute; 'pkglint --fix' applies the rest)")
	cmd.Flags().BoolVar(&bo.unsafeFix, "unsafe-fix", false, "also apply the behavior-changing built-package auto-fixes (implies --fix)")
	cmd.Flags().BoolVar(&bo.diff, "diff", false, "with --fix/--unsafe-fix: show the changes instead of writing them")
	return cmd
}

// checkMakepkgArgs refuses the makepkg options that would point the build at
// something pkglint never linted: -p names a different buildfile and -D/--dir
// a different directory, either of which turns a gated build of one PKGBUILD
// into an ungated build of another. They are the only two options in makepkg's
// short set that take a path, so a bundled cluster like -sp is caught too.
func checkMakepkgArgs(args []string) error {
	for _, a := range args {
		bad := ""
		switch {
		case a == "--dir" || strings.HasPrefix(a, "--dir="):
			bad = "--dir"
		case len(a) > 1 && a[0] == '-' && !strings.HasPrefix(a, "--") && strings.ContainsAny(a, "pD"):
			bad = a
		}
		if bad != "" {
			return fmt.Errorf(
				"makepkg argument %s selects a buildfile or directory other than the one pkglint linted, "+
					"which would run an unreviewed PKGBUILD; name that path to `pkglint build` instead", bad)
		}
	}
	return nil
}

// checkBuildAmbiguity catches `pkglint *` in a tree that holds a ./build
// package directory: the glob expands to `pkglint build <the rest>`, which
// cobra routes to this command — silently turning a lint of every package into
// a makepkg run of all but one of them.
//
// The signature of that mistake is what is matched, so nothing deliberate is
// caught in it: ./build has to be a package directory the arguments never
// mention, and every argument has to be a bare sibling name, which is what a
// glob expands to and what an explicit `./mypkg` or `pkgs/mypkg` is not.
func checkBuildAmbiguity(paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	if _, err := os.Stat(filepath.Join("build", "PKGBUILD")); err != nil {
		return nil
	}
	for _, p := range paths {
		if p == "build" || strings.ContainsRune(p, filepath.Separator) || p == "." || p == ".." {
			return nil
		}
	}
	rest := strings.Join(paths, " ")
	return fmt.Errorf(
		"ambiguous: ./build is a package directory and `build` also names this subcommand, so this would run makepkg on %s and never lint ./build "+
			"(which is what `pkglint *` expands to); to lint, put a path first: `pkglint ./build %s`; to build, say so: `pkglint build ./%s`",
		rest, rest, strings.Join(paths, " ./"))
}

// builder is the state one `build` invocation shares across its paths: the
// refusal gate, the runtime decision, and the lazily loaded pacman database,
// each resolved once rather than once per package.
type builder struct {
	ignored   map[string]bool
	gate      rules.Severity
	bo        buildOpts
	runner    string // "" for the host's own makepkg
	image     string
	runnerErr error
	localDB   func() *alpmdb.DB

	// fixes are the PKGBUILD rewrites the built archives earned, held until
	// every report has been rendered. They cannot be printed as they are
	// computed: the reports go to the same stdout, and under --format=json
	// interleaved prose would not be JSON any more.
	fixes []rules.FixResult
}

// runBuild builds and lints each path, returning the process exit code.
func runBuild(paths []string, ro reportOpts, bo buildOpts, stdout io.Writer) int {
	if len(paths) == 0 {
		paths = []string{"."}
	}
	b := &builder{
		ignored: ro.disabled(),
		gate:    refusalGate(ro.failOn),
		bo:      bo,
		localDB: newLocalDB(localDBRoot, os.Stderr),
	}
	// Resolved once: which runtime to use cannot differ between paths, and
	// finding out costs a $PATH search apiece otherwise. The error is deferred
	// to each package's report so it lands after that package's findings.
	b.runner, b.image, b.runnerErr = buildRunner(bo)

	// An interrupted build still has to unwind: the context cancels makepkg,
	// and build's deferred cleanup removes the temporary tree.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var reports []report.PackageReport
	for _, path := range paths {
		// Ctrl-C stops the run rather than starting the next build with a
		// context that will cancel it immediately.
		if err := ctx.Err(); err != nil {
			reports = append(reports, report.NewError(path, fmt.Errorf("interrupted before building: %w", err)))
			break
		}
		reports = append(reports, b.build(ctx, path)...)
	}

	code := renderReports(stdout, reports, ro)
	// After the reports, so nothing lands in the middle of a JSON or SARIF
	// document, and so the findings a fix answers are on screen above it.
	if applied, ok := applyFixResults(stdout, b.fixes, bo.diff); !ok {
		return 2
	} else if applied > 0 {
		verb := "applied"
		if bo.diff {
			verb = "would apply"
		}
		fmt.Fprintf(stdout, "%s %d fix(es) to the PKGBUILD from what the build produced\n", verb, applied)
	}
	// --fail-on grades findings; it says nothing about whether the command
	// did its job. A refusal, a makepkg failure, or a missing runtime is an
	// operational failure and fails the run even under --fail-on=never, which
	// would otherwise report "makepkg exited 4" and exit 0.
	if code == 0 {
		for _, r := range reports {
			if r.Err != "" {
				return 1
			}
		}
	}
	return code
}

// refusalGate is the severity at which a static finding refuses to build. It
// tracks --fail-on, except that "never" — a statement about exit codes, not
// about what is safe to execute — still stops at a critical finding.
func refusalGate(failOn string) rules.Severity {
	if failOn == "never" {
		return rules.Critical
	}
	sev, err := rules.ParseSeverity(failOn)
	if err != nil {
		return rules.Critical
	}
	return sev
}

// build lints one PKGBUILD, builds it, and lints what it produced. The static
// report is always part of the result, so the findings that decided the gate
// are visible in every output format.
func (b *builder) build(ctx context.Context, path string) []report.PackageReport {
	if pkgfile.IsPackagePath(path) {
		return []report.PackageReport{report.NewError(path, errors.New("already a built package: lint it directly with `pkglint <archive>`"))}
	}
	dir, err := buildDir(path)
	if err != nil {
		return []report.PackageReport{report.NewError(path, err)}
	}
	pkg, err := pkgbuild.Load(path)
	if err != nil {
		return []report.PackageReport{report.NewError(path, err)}
	}

	// The gate decides whether to execute this file, so the file gets no vote
	// in it: a `# pkglint: ignore=PB304` above a `curl | bash` would otherwise
	// switch off the very finding that refuses to run it, and `pkglint
	// --add-ignores` would turn any PKGBUILD into a buildable one. Inline
	// directives are a convenience for auditing a package you already trust.
	// The report still honours them, as every other command does — only the
	// decision to execute is taken on the unsuppressed set.
	suppressions := pkg.Suppressions
	pkg.Suppressions = nil
	gating := rules.Run(pkg, b.ignored)
	pkg.Suppressions = suppressions
	// A second pass, not a filter of the first: PB913 (stale ignore) only
	// exists when directives are honoured, so filtering the unsuppressed set
	// would never show it. Run is deterministic, so every other finding is the
	// same one the gate saw, minus what the directives waive.
	shown := rules.Run(pkg, b.ignored)
	out := []report.PackageReport{report.New(path, shown)}

	if blocked, worst := blocking(gating, b.gate); blocked > 0 && !b.bo.force {
		// Without this note, a package whose directives suppress everything
		// prints a clean report and an unexplained refusal beside it.
		note := ""
		if visible, _ := blocking(shown, b.gate); visible < blocked {
			note = fmt.Sprintf(" (%d of them hidden by inline '# pkglint: ignore=' directives, which do not apply to this decision)", blocked-visible)
		}
		return append(out, report.NewError(dir, fmt.Errorf(
			"build refused: %d static finding(s) at or above %s (worst: %s)%s; fix them, raise --fail-on, or pass --force to build anyway",
			blocked, b.gate, worst, note)))
	}
	if b.runnerErr != nil {
		return append(out, report.NewError(dir, b.runnerErr))
	}

	root, err := os.MkdirTemp("", "pkglint-build-")
	if err != nil {
		return append(out, report.NewError(dir, err))
	}
	defer os.RemoveAll(root)

	dirs, err := prepareDirs(root, b.runner == "")
	if err != nil {
		return append(out, report.NewError(dir, err))
	}
	if err := b.makepkg(ctx, dir, dirs); err != nil {
		return append(out, report.NewError(dir, err))
	}

	archives, err := collectArchives(dirs.pkgdest, b.bo.keep)
	if err != nil {
		return append(out, report.NewError(dir, err))
	}
	if len(archives) == 0 {
		return append(out, report.NewError(dir, errors.New("makepkg succeeded but produced no package archives")))
	}
	for _, archive := range archives {
		built, err := pkgfile.Load(archive)
		if err != nil {
			out = append(out, report.NewError(archive, err))
			continue
		}
		out = append(out, report.New(archive, rules.RunPackage(built, b.localDB(), b.ignored)))
		if level := b.bo.fixLevel(); level != rules.FixNone {
			// pkg is the PKGBUILD this very invocation gated and handed to
			// makepkg, and built is what came back, so the pairing the
			// package-scope fixers need is this loop's own. It is passed
			// nowhere else. Held read-only across the build, the file on disk
			// is still the bytes pkg was parsed from, so the edits apply to
			// it; a split build's other members are turned away by the fixers
			// themselves rather than filtered here.
			b.fixes = append(b.fixes, rules.FixPackage(pkg, built, b.localDB(), b.ignored, level, nil)...)
		}
	}
	return out
}

// blocking counts the findings at or above gate and names the worst of them.
// The seed is gate itself: nothing below it is counted, so nothing below it
// can be the worst either.
func blocking(findings []rules.Finding, gate rules.Severity) (int, rules.Severity) {
	n := 0
	worst := gate
	for _, f := range findings {
		if f.Severity < gate {
			continue
		}
		n++
		if f.Severity > worst {
			worst = f.Severity
		}
	}
	return n, worst
}

// buildDir is the directory makepkg runs in: path itself when it names a
// directory, otherwise the directory holding the PKGBUILD. A file argument
// must be named PKGBUILD, because makepkg builds the PKGBUILD in its working
// directory and nothing else — accepting `pkglint build pkg/PKGBUILD.reviewed`
// would gate one file and then execute another.
func buildDir(path string) (string, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if fi.IsDir() {
		return path, nil
	}
	if filepath.Base(path) != "PKGBUILD" {
		return "", fmt.Errorf(
			"refusing to build %s: makepkg would build %s instead, which linting this file says nothing about",
			path, filepath.Join(filepath.Dir(path), "PKGBUILD"))
	}
	return filepath.Dir(path), nil
}

// prepareDirs creates the destinations makepkg writes to. All but the source
// cache live under the per-build temporary root and are thrown away with it;
// sources persist, because a hook that re-downloads every tarball on every run
// is one nobody keeps enabled. A container build needs none of that: it writes
// to its own /tmp and downloads its own sources, so only the directory the
// archives are copied back into is a host path.
func prepareDirs(root string, host bool) (buildDirs, error) {
	d := buildDirs{pkgdest: filepath.Join(root, "pkg")}
	if !host {
		return d, os.MkdirAll(d.pkgdest, 0o755)
	}
	d.logdest = filepath.Join(root, "log")
	d.srcpkgdest = filepath.Join(root, "srcpkg")
	// $BUILDDIR is honoured for the same reason $SRCDEST is, and one more:
	// the temporary root lives under $TMPDIR, which is tmpfs on a stock Arch
	// install. Unpacking and compiling a kernel or a Rust workspace in RAM
	// fails with ENOSPC or drives the machine into swap, and a maintainer who
	// set BUILDDIR did so precisely to avoid that.
	d.builddir = os.Getenv("BUILDDIR")
	if d.builddir == "" {
		d.builddir = filepath.Join(root, "build")
	}
	d.srcdest = os.Getenv("SRCDEST")
	if d.srcdest == "" {
		cache, err := os.UserCacheDir()
		if err != nil {
			return d, fmt.Errorf("locating the source cache: %w", err)
		}
		d.srcdest = filepath.Join(cache, "pkglint", "sources")
	}
	// makepkg would create these itself; doing it here surfaces a permission
	// problem as an error naming the directory rather than as an abort part
	// way through a build.
	for _, dir := range []string{d.pkgdest, d.builddir, d.logdest, d.srcdest, d.srcpkgdest} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return d, err
		}
	}
	return d, nil
}

// containerPkgdest is where a containerized makepkg leaves its archives.
// Everything makepkg writes is kept under /tmp inside the container, the one
// directory every Arch image has world-writable — so the build works whatever
// uid it ends up running as, without the image needing a builder account.
const containerPkgdest = "/tmp/out"

// makepkg builds the package in dir, leaving the archives in d.pkgdest. -s is
// deliberately absent from the default arguments: syncing dependencies needs
// root, and a commit hook that can prompt for a password is a commit hook that
// hangs.
func (b *builder) makepkg(ctx context.Context, dir string, d buildDirs) error {
	// --nosign because the artifact is inspected and thrown away: signing it
	// is pointless, and a maintainer with `sign` in BUILDENV cannot turn that
	// off from the environment (makepkg.conf marks BUILDENV readonly), so a
	// gpg passphrase prompt would hang the very commit hook this command is
	// meant to be run from.
	args := append([]string{"-f", "--noconfirm", "--cleanbuild", "--nosign"}, b.bo.makepkgArgs...)
	if b.runner != "" {
		return containerBuild(ctx, b.runner, b.image, dir, d.pkgdest, args)
	}
	restore, err := holdBuildFile(dir)
	if err != nil {
		return err
	}
	defer restore()

	cmd := exec.CommandContext(ctx, "makepkg", args...)
	cmd.Dir = dir
	// Appended, so these win over any of the five already exported: last
	// assignment wins in the child, and makepkg re-reads all five from the
	// environment after sourcing makepkg.conf. SRCPKGDEST is redirected too —
	// it defaults to the package directory, which `-- -S` would then write a
	// source tarball into.
	cmd.Env = append(os.Environ(),
		"PKGDEST="+d.pkgdest,
		"BUILDDIR="+d.builddir,
		"LOGDEST="+d.logdest,
		"SRCDEST="+d.srcdest,
		"SRCPKGDEST="+d.srcpkgdest,
	)
	if err := runStreamed(cmd); err != nil {
		return describe(ctx, "makepkg", err)
	}
	return nil
}

// holdBuildFile drops the PKGBUILD's write bits for the duration of a host
// build and returns the undo. A PKGBUILD with a pkgver() function — every -git
// package in the AUR has one — otherwise has makepkg rewrite its pkgver= and
// reset its pkgrel= in place: it would edit the file the user is about to
// commit, and build a version other than the one the gate reviewed. makepkg
// offers no way to decline (--holdver is parsed and unused, and update_pkgver
// is reached whenever the function exists); it does check whether the buildfile
// is writable and warns instead when it is not. That is exactly what the
// container path already gets from its read-only mount, so this gives the host
// path the same answer.
func holdBuildFile(dir string) (func(), error) {
	path := filepath.Join(dir, "PKGBUILD")
	fi, err := os.Stat(path)
	if err != nil {
		// Not this function's error to report: makepkg is about to say so
		// far more precisely than "could not chmod".
		return func() {}, nil
	}
	mode := fi.Mode().Perm()
	if mode&0o222 == 0 {
		return func() {}, nil
	}
	if err := os.Chmod(path, mode&^0o222); err != nil {
		return func() {}, fmt.Errorf("holding %s read-only for the build: %w", path, err)
	}
	return func() { _ = os.Chmod(path, mode) }, nil
}

// containerBuild runs makepkg inside a container and copies the archives back
// out, rather than bind-mounting a destination for it to write.
//
// The copy is what makes this work everywhere: `<runner> cp` extracts on the
// client side, so the archives land owned by whoever ran pkglint. A writable
// bind mount cannot manage that under rootless docker, where the caller's uid
// maps to the container's root and every other container uid maps to a
// subordinate uid that owns nothing on the host — and makepkg, which refuses
// to run as root, can only be the latter.
func containerBuild(ctx context.Context, runner, image, dir, pkgdest string, makepkgArgs []string) error {
	// --user hands the container the caller's uid, and makepkg refuses to run
	// as root; as root there is no uid to pass that would work. Say so, rather
	// than spending a container start to arrive at "makepkg exited 1".
	if os.Getuid() == 0 {
		return errors.New("a container build cannot run as root: makepkg refuses to run as root, and the container runs as the calling uid; rerun as an unprivileged user")
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	// --mount takes a comma-separated key=value list, so a comma in the path
	// would be read as the start of another option.
	if strings.ContainsRune(abs, ',') {
		return fmt.Errorf("cannot bind-mount %q into a container: the path contains a comma", abs)
	}
	// The container is addressed by a name chosen up front rather than by the
	// id `create` prints: the name is known before anything runs, so the
	// removal below is registered first and holds even if create fails part
	// way or its output cannot be read.
	name := fmt.Sprintf("pkglint-build-%d-%d", os.Getpid(), time.Now().UnixNano())
	defer func() {
		// Not CommandContext: on an interrupted build the context is already
		// cancelled, and the container still has to go.
		rm := exec.Command(runner, "rm", "--force", name)
		rm.Stdout, rm.Stderr = io.Discard, io.Discard
		_ = runCmd(rm)
	}()

	// The package directory is mounted read-only: with everything makepkg
	// writes redirected, it needs nothing writable there, and the mount
	// enforces "the tree is left as it was found" instead of asserting it.
	create := exec.CommandContext(ctx, runner, append([]string{
		"create",
		"--name", name,
		"--user", fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()),
		"-e", "HOME=/tmp",
		"--mount", "type=bind,source=" + abs + ",target=/build,readonly",
		"-w", "/build",
		"-e", "PKGDEST=" + containerPkgdest,
		"-e", "BUILDDIR=/tmp/build",
		"-e", "SRCDEST=/tmp/src",
		"-e", "LOGDEST=/tmp/log",
		"-e", "SRCPKGDEST=/tmp/srcpkg",
		image, "makepkg",
	}, makepkgArgs...)...)
	// The id on stdout is not needed and must not reach pkglint's own stdout,
	// which carries the --format json/sarif document.
	create.Stdout, create.Stderr = io.Discard, os.Stderr
	if err := runCmd(create); err != nil {
		return describe(ctx, runner+" create", err)
	}

	if err := runStreamed(exec.CommandContext(ctx, runner, "start", "--attach", name)); err != nil {
		return describe(ctx, "makepkg", err)
	}
	cp := exec.CommandContext(ctx, runner, "cp", name+":"+containerPkgdest+"/.", pkgdest)
	if err := runStreamed(cp); err != nil {
		return describe(ctx, runner+" cp", err)
	}
	return nil
}

// runStreamed runs a command with its output on pkglint's stderr, never
// stdout: stdout carries the --format json/sarif document and has to stay
// parseable.
func runStreamed(cmd *exec.Cmd) error {
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return runCmd(cmd)
}

// describe names the failing step and, for a command that ran and failed, the
// status it exited with. A cancelled context is reported as an interruption:
// the exit status in that case is the signal the command was killed with, not
// anything the build decided.
func describe(ctx context.Context, what string, err error) error {
	if ctx.Err() != nil {
		return fmt.Errorf("%s: interrupted", what)
	}
	if exit, ok := errors.AsType[*exec.ExitError](err); ok {
		return fmt.Errorf("%s exited %d", what, exit.ExitCode())
	}
	return fmt.Errorf("%s: %w", what, err)
}

// buildRunner decides how makepkg is reached. An empty runner means the host's
// own makepkg; otherwise it is the container runtime to run image with.
func buildRunner(bo buildOpts) (runner, image string, err error) {
	image = bo.image
	if image == "" {
		image = os.Getenv("PKGLINT_BUILD_IMAGE")
	}
	_, hostErr := lookPath("makepkg")
	// bo.imageSet, not image != "": $PKGLINT_BUILD_IMAGE is a fallback for
	// hosts without makepkg, while spelling --image on the command line is a
	// request that an installed makepkg must not quietly override.
	if !bo.docker && !bo.imageSet && hostErr == nil {
		return "", "", nil
	}
	if image == "" {
		if bo.docker {
			return "", "", errors.New("--docker needs an image: pass --image or set PKGLINT_BUILD_IMAGE (e.g. archlinux:base-devel)")
		}
		return "", "", errors.New("makepkg not found on $PATH: install it, or set --image / PKGLINT_BUILD_IMAGE to build in a container")
	}
	runner, err = containerRunner()
	return runner, image, err
}

// containerRunner picks the container runtime: whatever PKGLINT_BUILD_RUNNER
// names, else docker, else podman.
func containerRunner() (string, error) {
	if named := os.Getenv("PKGLINT_BUILD_RUNNER"); named != "" {
		if _, err := lookPath(named); err != nil {
			return "", fmt.Errorf("PKGLINT_BUILD_RUNNER=%q: %w", named, err)
		}
		return named, nil
	}
	for _, candidate := range []string{"docker", "podman"} {
		if _, err := lookPath(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", errors.New("no container runtime found: install docker or podman, or set PKGLINT_BUILD_RUNNER")
}

// collectArchives returns the package archives makepkg left in pkgdest. With
// keep set they are moved out first and linted where they landed, so the
// reported paths outlive the run.
func collectArchives(pkgdest, keep string) ([]string, error) {
	entries, err := os.ReadDir(pkgdest)
	if err != nil {
		return nil, err
	}
	dest := pkgdest
	if keep != "" {
		if err := os.MkdirAll(keep, 0o755); err != nil {
			return nil, err
		}
		dest = keep
	}
	var archives []string
	for _, e := range entries {
		name := e.Name()
		// Not a *.pkg.tar.* glob: PKGEXT=.pkg.tar is legal and uncompressed.
		// That makes it a name test, though, which the detached signature
		// `--sign` leaves beside each archive also passes.
		if !e.Type().IsRegular() || !pkgfile.IsPackagePath(name) || isArchiveSidecar(name) {
			continue
		}
		if keep != "" {
			if err := moveFile(filepath.Join(pkgdest, name), filepath.Join(keep, name)); err != nil {
				return nil, err
			}
		}
		archives = append(archives, filepath.Join(dest, name))
	}
	return archives, nil
}

// isArchiveSidecar reports whether name is one of the files makepkg leaves
// next to an archive whose name ends in an archive's name without being one.
func isArchiveSidecar(name string) bool {
	lower := strings.ToLower(name)
	for _, suffix := range []string{".sig", ".part", ".log"} {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return false
}

// moveFile relocates src to dst, falling back to a copy when the two are on
// different filesystems — the temporary tree is often a tmpfs while --keep
// points at the user's disk. The copy lands under a temporary name and is
// renamed into place, so a failure part way cannot leave a truncated file
// wearing a valid package name for the next run to install or lint.
func moveFile(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".pkglint-*")
	if err != nil {
		return err
	}
	if _, err := io.Copy(tmp, in); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	// CreateTemp opens at 0600; a kept archive should be readable like the one
	// makepkg would have written.
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	if err := os.Rename(tmp.Name(), dst); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return nil
}
