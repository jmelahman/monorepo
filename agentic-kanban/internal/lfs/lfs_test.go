package lfs

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/jmelahman/kanban/internal/gittest"
)

// TestMain answers for kanban as the filter, as main.go does: configure points
// git at os.Executable(), which under `go test` is this binary.
func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == Command {
		os.Exit(Main())
	}
	gittest.IsolateEnv()
	os.Exit(m.Run())
}

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, stderr.String())
	}
	return string(out)
}

func write(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
}

// useFilter has git use the filter, whether or not git-lfs is installed here,
// and required, as `git lfs install` has it.
func useFilter(t *testing.T) {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	// IsolateEnv's config stays, and t.Setenv restores what's added to it.
	count := os.Getenv("GIT_CONFIG_COUNT")
	n, _ := strconv.Atoi(count)
	t.Setenv("GIT_CONFIG_COUNT", strconv.Itoa(n+1))
	t.Setenv("GIT_CONFIG_KEY_"+strconv.Itoa(n), "filter.lfs.required")
	t.Setenv("GIT_CONFIG_VALUE_"+strconv.Itoa(n), "true")
	t.Setenv("GIT_CONFIG_KEY_"+strconv.Itoa(n+1), "")
	t.Setenv("GIT_CONFIG_VALUE_"+strconv.Itoa(n+1), "")
	configure(self)
}

// newRepo returns a repository tracking *.bin with LFS.
func newRepo(t *testing.T) string {
	t.Helper()
	useFilter(t)
	repo := t.TempDir()
	git(t, repo, "init", "-q", "-b", "main")
	git(t, repo, "config", "user.name", "Test")
	git(t, repo, "config", "user.email", "test@example.com")
	write(t, filepath.Join(repo, ".gitattributes"), []byte("*.bin filter=lfs diff=lfs merge=lfs -text\n"))
	git(t, repo, "add", ".gitattributes")
	git(t, repo, "commit", "-qm", "track")
	return repo
}

func randomBytes(t *testing.T, n int) []byte {
	t.Helper()
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return b
}

func oidOf(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func objectPath(repo, oid string) string {
	return filepath.Join(repo, ".git", "lfs", "objects", oid[:2], oid[2:4], oid)
}

func assertFile(t *testing.T, path string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s: got %d bytes (%.80q), want %d", path, len(got), got, len(want))
	}
}

func assertClean(t *testing.T, dir string) {
	t.Helper()
	if status := git(t, dir, "status", "--porcelain"); status != "" {
		t.Errorf("status in %s:\n%s", dir, status)
	}
}

// commitBlob commits content at name as it is, without the filter.
func commitBlob(t *testing.T, repo, name string, content []byte) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "blob")
	write(t, path, content)
	oid := strings.TrimSpace(git(t, repo, "hash-object", "-w", "--no-filters", path))
	git(t, repo, "update-index", "--add", "--cacheinfo", "100644,"+oid+","+name)
	git(t, repo, "commit", "-qm", "add "+name)
}

func TestRoundTrip(t *testing.T) {
	repo := newRepo(t)
	// Wider than a packet, so both directions split it.
	big := randomBytes(t, 3*maxPayload+17)
	write(t, filepath.Join(repo, "big.bin"), big)
	write(t, filepath.Join(repo, "empty.bin"), nil)
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-qm", "add")

	oid := oidOf(big)
	if got, want := git(t, repo, "cat-file", "blob", "HEAD:big.bin"), formatPointer(oid, int64(len(big))); got != want {
		t.Errorf("blob = %q, want %q", got, want)
	}
	if got := git(t, repo, "cat-file", "blob", "HEAD:empty.bin"); got != "" {
		t.Errorf("empty blob = %q", got)
	}
	assertFile(t, objectPath(repo, oid), big)
	assertClean(t, repo)

	worktree := filepath.Join(t.TempDir(), "wt")
	git(t, repo, "worktree", "add", "-q", "-b", "other", worktree, "main")
	assertFile(t, filepath.Join(worktree, "big.bin"), big)
	assertFile(t, filepath.Join(worktree, "empty.bin"), nil)
	assertClean(t, worktree)

	// A worktree stores to the repository's objects, not one of its own.
	changed := randomBytes(t, 2000)
	write(t, filepath.Join(worktree, "big.bin"), changed)
	git(t, worktree, "add", "-A")
	if got, want := git(t, worktree, "cat-file", "blob", ":big.bin"), formatPointer(oidOf(changed), 2000); got != want {
		t.Errorf("staged blob = %q, want %q", got, want)
	}
	assertFile(t, objectPath(repo, oidOf(changed)), changed)
}

// A session must start without the object, and the pointer it gets isn't a
// change to commit.
func TestMissingObjectLeavesPointer(t *testing.T) {
	repo := newRepo(t)
	content := randomBytes(t, 100)
	write(t, filepath.Join(repo, "a.bin"), content)
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-qm", "add")
	if err := os.Remove(objectPath(repo, oidOf(content))); err != nil {
		t.Fatal(err)
	}

	worktree := filepath.Join(t.TempDir(), "wt")
	git(t, repo, "worktree", "add", "-q", "-b", "other", worktree, "main")
	assertFile(t, filepath.Join(worktree, "a.bin"), []byte(formatPointer(oidOf(content), 100)))
	assertClean(t, worktree)
}

func TestTruncatedObjectLeavesPointer(t *testing.T) {
	repo := newRepo(t)
	content := randomBytes(t, 100)
	write(t, filepath.Join(repo, "a.bin"), content)
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-qm", "add")
	write(t, objectPath(repo, oidOf(content)), content[:50])

	worktree := filepath.Join(t.TempDir(), "wt")
	git(t, repo, "worktree", "add", "-q", "-b", "other", worktree, "main")
	assertFile(t, filepath.Join(worktree, "a.bin"), []byte(formatPointer(oidOf(content), 100)))
}

// Blobs that aren't pointers, committed around the filter, pass through; that
// includes one whose oid would climb out of the object store.
func TestNonPointersPassThrough(t *testing.T) {
	repo := newRepo(t)
	secret := filepath.Join(repo, ".git", "lfs", "secret")
	if err := os.MkdirAll(filepath.Dir(secret), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, secret, []byte("secret"))
	for name, content := range map[string][]byte{
		"raw.bin":        randomBytes(t, 5000),
		"traversal.bin":  []byte("version https://git-lfs.github.com/spec/v1\noid sha256:../../secret\nsize 6\n"),
		"extensions.bin": []byte("version https://git-lfs.github.com/spec/v1\next-0-foo sha256:" + strings.Repeat("a", 64) + "\noid sha256:" + strings.Repeat("b", 64) + "\nsize 6\n"),
	} {
		commitBlob(t, repo, name, content)
		worktree := filepath.Join(t.TempDir(), "wt")
		git(t, repo, "worktree", "add", "-q", "-b", "wt-"+name, worktree, "main")
		assertFile(t, filepath.Join(worktree, name), content)
	}
}

func TestStorageConfig(t *testing.T) {
	absolute := t.TempDir()
	for name, tc := range map[string]struct{ value, dir string }{
		"absolute": {absolute, absolute},
		"relative": {"elsewhere", filepath.Join(".git", "elsewhere")},
	} {
		t.Run(name, func(t *testing.T) {
			repo := newRepo(t)
			git(t, repo, "config", "lfs.storage", tc.value)
			content := randomBytes(t, 2000)
			write(t, filepath.Join(repo, "a.bin"), content)
			git(t, repo, "add", "-A")
			oid := oidOf(content)
			dir := tc.dir
			if !filepath.IsAbs(dir) {
				dir = filepath.Join(repo, dir)
			}
			assertFile(t, filepath.Join(dir, "objects", oid[:2], oid[2:4], oid), content)
		})
	}
}

// What it stores, the real git-lfs reads, and the other way around.
func TestAgreesWithGitLFS(t *testing.T) {
	if _, err := exec.LookPath("git-lfs"); err != nil {
		t.Skip("git-lfs isn't installed")
	}
	repo := newRepo(t)
	content := randomBytes(t, 2*maxPayload)
	path := filepath.Join(repo, "a.bin")
	write(t, path, content)
	git(t, repo, "add", "-A")
	pointer := git(t, repo, "cat-file", "blob", ":a.bin")
	if got := git(t, repo, "lfs", "pointer", "--file=a.bin"); got != pointer {
		t.Errorf("git-lfs makes %q, kanban %q", got, pointer)
	}

	cmd := exec.Command("git", "lfs", "smudge")
	cmd.Dir = repo
	cmd.Stdin = strings.NewReader(pointer)
	got, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("git-lfs smudged %d bytes, want %d", len(got), len(content))
	}

	stored := randomBytes(t, 3000)
	cmd = exec.Command("git", "lfs", "clean")
	cmd.Dir = repo
	cmd.Stdin = bytes.NewReader(stored)
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	commitBlob(t, repo, "stored.bin", out)
	worktree := filepath.Join(t.TempDir(), "wt")
	git(t, repo, "worktree", "add", "-q", "-b", "other", worktree, "main")
	assertFile(t, filepath.Join(worktree, "stored.bin"), stored)
}

func TestParsePointer(t *testing.T) {
	oid := strings.Repeat("a", 64)
	ok := "version https://git-lfs.github.com/spec/v1\noid sha256:" + oid + "\nsize 12\n"
	if got, size, valid := parsePointer([]byte(ok)); !valid || got != oid || size != 12 {
		t.Errorf("parsePointer = %q, %d, %t", got, size, valid)
	}
	for name, s := range map[string]string{
		"empty":        "",
		"no newline":   strings.TrimSuffix(ok, "\n"),
		"trailing":     ok + "x",
		"short oid":    strings.Replace(ok, oid, "aa", 1),
		"uppercase":    strings.Replace(ok, oid, strings.ToUpper(oid), 1),
		"slash in oid": strings.Replace(ok, oid[:3], "../", 1),
		"negative":     strings.Replace(ok, "size 12", "size -1", 1),
		"leading zero": strings.Replace(ok, "size 12", "size 012", 1),
		"huge size":    strings.Replace(ok, "size 12", "size 99999999999999999999", 1),
		"other spec":   strings.Replace(ok, "/spec/v1", "/spec/v2", 1),
		"too long":     ok + strings.Repeat("\n", maxPointer),
	} {
		if _, _, valid := parsePointer([]byte(s)); valid {
			t.Errorf("%s: parsed as a pointer", name)
		}
	}
}

func TestConfigureKeepsExistingConfig(t *testing.T) {
	t.Setenv("GIT_CONFIG_COUNT", "2")
	t.Setenv("GIT_CONFIG_KEY_2", "")
	t.Setenv("GIT_CONFIG_VALUE_2", "")
	configure("/opt/it's here/kanban")
	if got := os.Getenv("GIT_CONFIG_COUNT"); got != "3" {
		t.Errorf("count = %q", got)
	}
	if got := os.Getenv("GIT_CONFIG_KEY_2"); got != "filter.lfs.process" {
		t.Errorf("key = %q", got)
	}
	if got, want := os.Getenv("GIT_CONFIG_VALUE_2"), `'/opt/it'\''s here/kanban' lfs-filter`; got != want {
		t.Errorf("value = %q, want %q", got, want)
	}
}
