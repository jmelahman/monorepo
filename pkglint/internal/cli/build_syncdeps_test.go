package cli

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/jmelahman/pkglint/internal/pkgbuild"
)

func TestSplitSyncdeps(t *testing.T) {
	for _, tc := range []struct {
		in, want []string
		sync     bool
		run      pkgbuild.RunCheck
	}{
		{in: []string{"-f", "--noconfirm"}, want: []string{"-f", "--noconfirm"}},
		{in: []string{"-f", "-s"}, want: []string{"-f"}, sync: true},
		{in: []string{"--syncdeps"}, want: nil, sync: true},
		{in: []string{"--sync"}, want: nil, sync: true},
		{in: []string{"--sy", "-C"}, want: []string{"-C"}, sync: true},
		// --s is ambiguous to makepkg (sign, skip*, source, syncdeps): it
		// is left for makepkg to refuse.
		{in: []string{"--s"}, want: []string{"--s"}},
		{in: []string{"-sf"}, want: []string{"-f"}, sync: true},
		{in: []string{"-fsc"}, want: []string{"-fc"}, sync: true},
		{in: []string{"-ss"}, want: nil, sync: true},
		{in: []string{"--key", "-s"}, want: []string{"--key", "-s"}},
		{in: []string{"--ke", "-s", "-s"}, want: []string{"--ke", "-s"}, sync: true},
		{in: []string{"--key=-s"}, want: []string{"--key=-s"}},
		{in: []string{"--", "-s"}, want: []string{"--", "-s"}},
		{in: []string{"-s", "--", "-s"}, want: []string{"--", "-s"}, sync: true},
		{in: []string{"-", "s"}, want: []string{"-", "s"}},
		{in: []string{"--syncdeps=x"}, want: []string{"--syncdeps=x"}},
		// The last of --check and --nocheck, in any unambiguous spelling,
		// decides whether checkdepends are wanted; both stay for makepkg.
		{in: []string{"-s", "--check"}, want: []string{"--check"}, sync: true, run: pkgbuild.CheckForced},
		{in: []string{"-s", "--ch"}, want: []string{"--ch"}, sync: true, run: pkgbuild.CheckForced},
		{in: []string{"-s", "--nocheck"}, want: []string{"--nocheck"}, sync: true, run: pkgbuild.CheckSkipped},
		{in: []string{"--noch", "-s"}, want: []string{"--noch"}, sync: true, run: pkgbuild.CheckSkipped},
		{in: []string{"--nocheck", "--check"}, want: []string{"--nocheck", "--check"}, run: pkgbuild.CheckForced},
		{in: []string{"--check", "--nocheck"}, want: []string{"--check", "--nocheck"}, run: pkgbuild.CheckSkipped},
		// --c and --no are ambiguous to makepkg; nothing is read off them.
		{in: []string{"--c", "--no"}, want: []string{"--c", "--no"}},
	} {
		got, err := splitSyncdeps(tc.in)
		if err != nil {
			t.Errorf("splitSyncdeps(%q): %v", tc.in, err)
			continue
		}
		if !slices.Equal(got.rest, tc.want) || got.sync != tc.sync || got.run != tc.run {
			t.Errorf("splitSyncdeps(%q) = %q, %v, %v; want %q, %v, %v", tc.in, got.rest, got.sync, got.run, tc.want, tc.sync, tc.run)
		}
	}
}

// TestSplitSyncdepsRefusesUnknownShorts: a single-dash word that is not a
// cluster of makepkg's no-value short options cannot be told to hold -s or
// not, so it fails closed instead of being passed on.
func TestSplitSyncdepsRefusesUnknownShorts(t *testing.T) {
	for _, in := range []string{"-sx", "-x", "-fz", "-s=1", "-ñ"} {
		if got, err := splitSyncdeps([]string{"-f", in}); err == nil || !strings.Contains(err.Error(), in) {
			t.Errorf("splitSyncdeps(%q) = %q, %v; want an error naming it", in, got.rest, err)
		}
	}
}

func TestValidDepName(t *testing.T) {
	for _, name := range []string{"go", "jq", "lib32-glibc", "libpng16.so", "gtk+", "python-foo_bar", "qt6-base", "foo@bar", "0ad"} {
		if !validDepName(name) {
			t.Errorf("validDepName(%q) = false, want true", name)
		}
	}
	for _, name := range []string{"", "-oops", "--overwrite=*", "--overwrite", ".hidden", "$(evil)", "a b", "a\tb", `x"y`, "x'y", "x;y", "ñ", "foo/bar", "*"} {
		if validDepName(name) {
			t.Errorf("validDepName(%q) = true, want false", name)
		}
	}
}

// TestBuildContainerSyncdeps pins the whole sequence: -s is taken off
// makepkg's arguments, wherever in a cluster it was, and pacman installs the
// PKGBUILD's statically read dependencies as root before the package
// directory is copied in. makepkg itself still runs as the container's user.
func TestBuildContainerSyncdeps(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("the container path refuses to run as root: makepkg would too")
	}
	t.Setenv("PKGLINT_BUILD_IMAGE", "archlinux:base-devel")
	seen, _ := stubContainer(t, []string{"docker"})

	var buf bytes.Buffer
	if code := run([]string{"build", "--fail-on=never", "--docker", "testdata/clean", "--", "-sC"}, &buf); code != 0 {
		t.Fatalf("got exit %d, want 0\n%s", code, buf.String())
	}
	create := step(t, *seen, "create")
	if strings.Contains(strings.Join(create.Args, " "), "0:0") {
		t.Errorf("the container itself runs as root: %q", create.Args)
	}
	name := flagValue(t, create, "--name")
	want := [][]string{
		create.Args,
		{"docker", "start", name},
		{"docker", "exec", name, "uname", "-m"},
		{"docker", "exec", "--user", "0:0", name, "pacman", "-Syu", "--needed", "--noconfirm", "--", "go"},
		{"docker", "cp", "-", name + ":/"},
		{"docker", "exec", "-w", "/build", name, "makepkg", "-f", "--noconfirm", "--cleanbuild", "--nosign", "-C"},
		{"docker", "cp", name + ":/tmp/out/.", ""},
		{"docker", "rm", "--force", name},
	}
	got := args(*seen)
	if len(got) == len(want) {
		want[6][3] = got[6][3]
	}
	if !slices.EqualFunc(got, want, slices.Equal) {
		t.Errorf("commands =\n%q\nwant\n%q", got, want)
	}
}

// TestBuildContainerSyncdepsNothingToInstall: -s with no dependencies is
// still taken off makepkg's arguments, and no root step runs at all.
func TestBuildContainerSyncdepsNothingToInstall(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("the container path refuses to run as root: makepkg would too")
	}
	dir := syncdepsPackage(t, "makedepends=('go')", "")
	seen, _ := stubContainer(t, []string{"docker"})
	var buf bytes.Buffer
	if code := run([]string{"build", "--fail-on=never", "--image", "archlinux:base-devel", dir, "--", "-s"}, &buf); code != 0 {
		t.Fatalf("got exit %d, want 0\n%s", code, buf.String())
	}
	for _, cmd := range *seen {
		if slices.Contains(cmd.Args, "pacman") || slices.Contains(cmd.Args, "-s") {
			t.Errorf("ran %q", cmd.Args)
		}
	}
	step(t, *seen, "exec")
}

// TestBuildHostSyncdepsUnchanged: on the host, makepkg -s is makepkg's own
// business (it asks sudo itself), and pkglint passes it through untouched.
func TestBuildHostSyncdepsUnchanged(t *testing.T) {
	seen := stubExec(t, []string{"makepkg"}, dropArchive("goodpkg-2.1.0-1-any.pkg.tar"))
	var buf bytes.Buffer
	if code := run([]string{"build", "--fail-on=never", "testdata/clean", "--", "-sf"}, &buf); code != 0 {
		t.Fatalf("got exit %d, want 0\n%s", code, buf.String())
	}
	cmd := only(t, *seen)
	if want := []string{"makepkg", "-f", "--noconfirm", "--cleanbuild", "--nosign", "-sf"}; !slices.Equal(cmd.Args, want) {
		t.Errorf("args = %q, want %q", cmd.Args, want)
	}
}

// syncdepsPackage writes testdata/clean's PKGBUILD with its makedepends line
// replaced, so each case is a package the gate would otherwise build.
func syncdepsPackage(t *testing.T, old, line string) string {
	t.Helper()
	src, err := os.ReadFile("testdata/clean/PKGBUILD")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(src, []byte(old)) {
		t.Fatalf("testdata/clean/PKGBUILD has no %q", old)
	}
	dir := t.TempDir()
	out := bytes.Replace(src, []byte(old), []byte(line), 1)
	if err := os.WriteFile(filepath.Join(dir, "PKGBUILD"), out, 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// TestBuildContainerSyncdepsRefuses covers the dependency lists pkglint will
// not hand to pacman: names makepkg itself would reject, which include every
// way to smuggle an option, and lists that cannot be known without running
// the PKGBUILD. --force takes the static gate out of the way, so what refuses
// is this step. It runs once the container says its CARCH, and the container
// is still removed; nothing runs as root, nothing is copied in, and makepkg
// never starts.
func TestBuildContainerSyncdepsRefuses(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("the container path refuses to run as root: makepkg would too")
	}
	for _, tc := range []struct{ line, want string }{
		{`makedepends=('-oops')`, `"-oops" is not a valid package name`},
		{`makedepends=('--overwrite=*')`, `"--overwrite" is not a valid package name`},
		{`makedepends=('$(evil)')`, "not statically known"},
		{"makedepends=(go)\neval 'makedepends+=(jq)'", "runs eval"},
		{"_py=python\n[[ -e /x ]] && _py=python2\nmakedepends=(\"$_py\")", "_py is assigned under top-level control flow"},
		{`makedepends=("$(curl -s example.com/deps)")`, "not statically known"},
		{`makedepends=('go jq')`, "contains whitespace"},
		{`makedepends=('x"y')`, `"x\"y" is not a valid package name`},
		{`makedepends=("x'y")`, `"x'y" is not a valid package name`},
		{"makedepends=(go)\n[[ -e /etc/foo ]] && makedepends+=(jq)", "cannot sync dependencies"},
		{"makedepends_x86_64=(\"$(uname -r)\")", "makedepends_x86_64 entry"},
	} {
		t.Run(tc.line, func(t *testing.T) {
			dir := syncdepsPackage(t, "makedepends=('go')", tc.line)
			seen, _ := stubContainer(t, []string{"docker"})
			var buf bytes.Buffer
			if code := run([]string{"build", "--force", "--fail-on=never", "--image", "archlinux:base-devel", dir, "--", "-s"}, &buf); code != 1 {
				t.Fatalf("got exit %d, want 1\n%s", code, buf.String())
			}
			if !strings.Contains(buf.String(), tc.want) {
				t.Errorf("output lacks %q:\n%s", tc.want, buf.String())
			}
			assertNoSync(t, *seen)
		})
	}
}

// TestBuildContainerSyncdepsPacmanFails: a failed install stops the build
// before the package directory is copied in or makepkg runs.
func TestBuildContainerSyncdepsPacmanFails(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("the container path refuses to run as root: makepkg would too")
	}
	var ran [][]string
	stubExec(t, []string{"docker"}, func(cmd *exec.Cmd) error {
		ran = append(ran, cmd.Args)
		answerUname(cmd, "x86_64\n")
		if slices.Contains(cmd.Args, "pacman") {
			return errors.New("exit status 1")
		}
		return nil
	})
	var buf bytes.Buffer
	if code := run([]string{"build", "--fail-on=never", "--image", "archlinux:base-devel", "testdata/clean", "--", "--syncdeps"}, &buf); code != 1 {
		t.Fatalf("got exit %d, want 1\n%s", code, buf.String())
	}
	if !strings.Contains(buf.String(), "pacman: exit status 1") {
		t.Errorf("output lacks the pacman failure:\n%s", buf.String())
	}
	for _, a := range ran {
		if a[1] == "cp" || slices.Contains(a, "makepkg") {
			t.Errorf("ran %q after pacman failed", a)
		}
	}
	if last := ran[len(ran)-1]; last[1] != "rm" {
		t.Errorf("the container was not removed last: %q", ran)
	}
}

// assertNoSync checks that a refused sync stopped the build before anything
// ran as root, the tree was copied in, or makepkg started, and that the
// container was still removed.
func assertNoSync(t *testing.T, seen []*exec.Cmd) {
	t.Helper()
	ran := args(seen)
	for _, a := range ran {
		if slices.Contains(a, "pacman") || slices.Contains(a, "makepkg") || slices.Contains(a, "0:0") || a[1] == "cp" {
			t.Errorf("ran %q", a)
		}
	}
	if len(ran) == 0 || ran[len(ran)-1][1] != "rm" {
		t.Errorf("the container was not removed last: %q", ran)
	}
}

// TestBuildContainerSyncdepsCarch: the CARCH whose depends_$CARCH are
// installed is the container's, as its `uname -m` says, never the host's.
func TestBuildContainerSyncdepsCarch(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("the container path refuses to run as root: makepkg would too")
	}
	dir := syncdepsPackage(t, "makedepends=('go')", "makedepends=(go)\nmakedepends_x86_64=(nasm)\nmakedepends_aarch64=(armstuff)")
	for _, tc := range []struct {
		machine string
		want    []string
	}{
		{"x86_64\n", []string{"go", "nasm"}},
		{"aarch64\n", []string{"go", "armstuff"}},
	} {
		t.Run(tc.machine, func(t *testing.T) {
			seen := stubExec(t, []string{"docker"}, func(cmd *exec.Cmd) error {
				if answerUname(cmd, tc.machine) {
					return nil
				}
				if cmd.Args[1] == "cp" && cmd.Args[2] == "-" {
					_, err := io.Copy(io.Discard, cmd.Stdin)
					return err
				}
				if cmd.Args[1] == "cp" {
					return dropArchive("goodpkg-2.1.0-1-any.pkg.tar")(&exec.Cmd{Env: []string{"PKGDEST=" + cmd.Args[len(cmd.Args)-1]}})
				}
				return nil
			})
			var buf bytes.Buffer
			// --force: the arch=() line names only x86_64, which is not what
			// this case is about.
			if code := run([]string{"build", "--force", "--fail-on=never", "--image", "archlinux:base-devel", dir, "--", "-s"}, &buf); code != 0 {
				t.Fatalf("got exit %d, want 0\n%s", code, buf.String())
			}
			pacman := running(t, *seen, "pacman")
			if got := pacman.Args[len(pacman.Args)-len(tc.want):]; !slices.Equal(got, tc.want) {
				t.Errorf("installed %q, want %q", pacman.Args, tc.want)
			}
		})
	}
}

// TestBuildContainerSyncdepsUnknownArch: a machine pkglint does not know the
// CARCH of, or a uname that fails, refuses the sync rather than guessing.
func TestBuildContainerSyncdepsUnknownArch(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("the container path refuses to run as root: makepkg would too")
	}
	for _, tc := range []struct {
		name, machine string
		fail          bool
		want          string
	}{
		{name: "armv7l", machine: "armv7l\n", want: `machine "armv7l" is not one pkglint knows`},
		{name: "empty", machine: "", want: `machine "" is not one`},
		{name: "two lines", machine: "x86_64\nx86_64\n", want: "is not one pkglint knows"},
		{name: "uname fails", fail: true, want: "docker exec uname: exit status 1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			seen := stubExec(t, []string{"docker"}, func(cmd *exec.Cmd) error {
				if slices.Contains(cmd.Args, "uname") {
					if tc.fail {
						return errors.New("exit status 1")
					}
					answerUname(cmd, tc.machine)
				}
				return nil
			})
			var buf bytes.Buffer
			if code := run([]string{"build", "--fail-on=never", "--image", "archlinux:base-devel", "testdata/clean", "--", "-s"}, &buf); code != 1 {
				t.Fatalf("got exit %d, want 1\n%s", code, buf.String())
			}
			if !strings.Contains(buf.String(), tc.want) {
				t.Errorf("output lacks %q:\n%s", tc.want, buf.String())
			}
			assertNoSync(t, *seen)
		})
	}
}

// TestBuildContainerSyncdepsRefusesArgs: makepkg arguments whose -s cannot be
// told are refused before any container exists.
func TestBuildContainerSyncdepsRefusesArgs(t *testing.T) {
	seen, _ := stubContainer(t, []string{"docker"})
	var buf bytes.Buffer
	if code := run([]string{"build", "--fail-on=never", "--image", "archlinux:base-devel", "testdata/clean", "--", "-sx"}, &buf); code != 1 {
		t.Fatalf("got exit %d, want 1\n%s", code, buf.String())
	}
	if !strings.Contains(buf.String(), "-sx is not a cluster") {
		t.Errorf("output lacks the refusal:\n%s", buf.String())
	}
	if len(*seen) != 0 {
		t.Errorf("ran %q", args(*seen))
	}
}

// TestBuildContainerSyncdepsNocheck: --nocheck leaves checkdepends out of
// what pacman installs, as makepkg itself would.
func TestBuildContainerSyncdepsNocheck(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("the container path refuses to run as root: makepkg would too")
	}
	dir := syncdepsPackage(t, "makedepends=('go')", "makedepends=(go)\ncheckdepends=(python-pytest)\ncheck() { :; }")
	for _, tc := range []struct {
		arg  string
		want []string
	}{
		{"--check", []string{"go", "python-pytest"}},
		{"--nocheck", []string{"go"}},
	} {
		t.Run(tc.arg, func(t *testing.T) {
			seen, _ := stubContainer(t, []string{"docker"})
			var buf bytes.Buffer
			if code := run([]string{"build", "--force", "--fail-on=never", "--image", "archlinux:base-devel", dir, "--", "-s", tc.arg}, &buf); code != 0 {
				t.Fatalf("got exit %d, want 0\n%s", code, buf.String())
			}
			pacman := running(t, *seen, "pacman")
			if got := pacman.Args[len(pacman.Args)-len(tc.want):]; !slices.Equal(got, tc.want) || pacman.Args[len(pacman.Args)-len(tc.want)-1] != "--" {
				t.Errorf("installed %q, want %q", pacman.Args, tc.want)
			}
			if makepkg := running(t, *seen, "makepkg"); !slices.Contains(makepkg.Args, tc.arg) {
				t.Errorf("makepkg lost %s: %q", tc.arg, makepkg.Args)
			}
		})
	}
}

// running returns the first command that runs program, wherever in its
// arguments the container runner put it.
func running(t *testing.T, seen []*exec.Cmd, program string) *exec.Cmd {
	t.Helper()
	for _, cmd := range seen {
		if slices.Contains(cmd.Args, program) {
			return cmd
		}
	}
	t.Fatalf("no %s in %q", program, args(seen))
	return nil
}
