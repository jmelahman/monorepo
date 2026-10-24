package cli

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestSplitSyncdeps(t *testing.T) {
	for _, tc := range []struct {
		in, want []string
		sync     bool
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
		// A word with a letter outside the no-value short set is not a
		// cluster makepkg would read as holding -s.
		{in: []string{"-sx"}, want: []string{"-sx"}},
		{in: []string{"--key", "-s"}, want: []string{"--key", "-s"}},
		{in: []string{"--ke", "-s", "-s"}, want: []string{"--ke", "-s"}, sync: true},
		{in: []string{"--key=-s"}, want: []string{"--key=-s"}},
		{in: []string{"--", "-s"}, want: []string{"--", "-s"}},
		{in: []string{"-s", "--", "-s"}, want: []string{"--", "-s"}, sync: true},
		{in: []string{"-", "s"}, want: []string{"-", "s"}},
		{in: []string{"--syncdeps=x"}, want: []string{"--syncdeps=x"}},
	} {
		got, sync := splitSyncdeps(tc.in)
		if !slices.Equal(got, tc.want) || sync != tc.sync {
			t.Errorf("splitSyncdeps(%q) = %q, %v; want %q, %v", tc.in, got, sync, tc.want, tc.sync)
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
		{"docker", "exec", "--user", "0:0", name, "pacman", "-Syu", "--needed", "--noconfirm", "--", "go"},
		{"docker", "cp", "-", name + ":/"},
		{"docker", "exec", "-w", "/build", name, "makepkg", "-f", "--noconfirm", "--cleanbuild", "--nosign", "-C"},
		{"docker", "cp", name + ":/tmp/out/.", ""},
		{"docker", "rm", "--force", name},
	}
	got := args(*seen)
	if len(got) == len(want) {
		want[5][3] = got[5][3]
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
// is this step; either way nothing runs as root and makepkg never starts.
func TestBuildContainerSyncdepsRefuses(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("the container path refuses to run as root: makepkg would too")
	}
	for _, tc := range []struct{ line, want string }{
		{`makedepends=('-oops')`, `"-oops" is not a valid package name`},
		{`makedepends=('--overwrite=*')`, `"--overwrite" is not a valid package name`},
		{`makedepends=('$(evil)')`, "not statically known"},
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
			if len(*seen) != 0 {
				t.Errorf("ran %q", args(*seen))
			}
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
