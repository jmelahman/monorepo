package netprobe

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// fakeGit puts a stub `git` first on PATH that records every invocation (and
// the GIT_TERMINAL_PROMPT it was handed) in a sentinel file, then prints a
// plausible ls-remote line. It returns a func reporting the recorded
// invocations, so a test can assert git was never reached — and so no test
// here performs real network I/O.
func fakeGit(t *testing.T) func() []string {
	t.Helper()
	dir := t.TempDir()
	sentinel := filepath.Join(dir, "invocations")
	script := "#!/bin/sh\n" +
		`echo "prompt=$GIT_TERMINAL_PROMPT $*" >> "$PKGLINT_TEST_GIT_SENTINEL"` + "\n" +
		"for a; do ref=$a; done\n" +
		`if [ -n "$PKGLINT_TEST_GIT_OUTPUT" ]; then` + "\n" +
		`  printf '%s' "$PKGLINT_TEST_GIT_OUTPUT"` + "\n" +
		`elif [ "$ref" = HEAD ]; then` + "\n" +
		`  printf '%s\tHEAD\n' 0123456789abcdef0123456789abcdef01234567` + "\n" +
		"else\n" +
		`  printf '%s\trefs/tags/%s\n' 0123456789abcdef0123456789abcdef01234567 "$ref"` + "\n" +
		"fi\n"
	if err := os.WriteFile(filepath.Join(dir, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PKGLINT_TEST_GIT_SENTINEL", sentinel)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return func() []string {
		data, err := os.ReadFile(sentinel)
		if err != nil {
			return nil
		}
		return strings.Split(strings.TrimSpace(string(data)), "\n")
	}
}

// TestResolveGitRefSchemeGuard pins the transport allow-list on the one place
// pkglint shells out with a URL taken from an untrusted PKGBUILD. The rejected
// URLs must never reach git: ext:: and file:// let git run a local command or
// read the local filesystem, and a leading dash is an option, not a remote.
func TestResolveGitRefSchemeGuard(t *testing.T) {
	for _, url := range []string{
		"ext::sh -c whoami",
		"file:///tmp",
		"git+file:///tmp",
		"-oProxyCommand=id",
		"--upload-pack=id",
		"/tmp/local-repo",
		"git@github.com:example/demo.git",
		"ftp://example.com/demo.git",
		"",
	} {
		t.Run("reject "+url, func(t *testing.T) {
			invocations := fakeGit(t)
			if _, err := ResolveGitRef(url, "v1"); err == nil || !strings.Contains(err.Error(), "unsupported URL scheme") {
				t.Errorf("ResolveGitRef(%q) error = %v, want an unsupported-scheme rejection", url, err)
			}
			if got := invocations(); len(got) != 0 {
				t.Errorf("ResolveGitRef(%q) invoked git %v, want no invocation", url, got)
			}
		})
	}

	// The allowed transports must still resolve, and must hand git
	// GIT_TERMINAL_PROMPT=0 so a private remote errors out instead of
	// blocking the fix path on a credential prompt.
	for _, url := range []string{
		"git+https://example.com/demo.git",
		"git+http://example.com/demo.git",
		"git+ssh://git@example.com/demo.git",
		"git://example.com/demo.git",
		"https://example.com/demo.git",
	} {
		t.Run("allow "+url, func(t *testing.T) {
			invocations := fakeGit(t)
			sha, err := ResolveGitRef(url, "v1")
			if err != nil {
				t.Fatalf("ResolveGitRef(%q) = %v, want success", url, err)
			}
			if sha != "0123456789abcdef0123456789abcdef01234567" {
				t.Errorf("ResolveGitRef(%q) sha = %q", url, sha)
			}
			got := invocations()
			if len(got) != 1 {
				t.Fatalf("ResolveGitRef(%q) invoked git %d times, want 1: %v", url, len(got), got)
			}
			if !strings.HasPrefix(got[0], "prompt=0 ") {
				t.Errorf("git invoked as %q, want GIT_TERMINAL_PROMPT=0", got[0])
			}
			if strings.Contains(got[0], "git+") {
				t.Errorf("git invoked as %q, want the git+ prefix stripped", got[0])
			}
		})
	}
}

// TestProbeHTTPS covers the check that stands between PB104's finding and its
// rewrite. The fix is only as good as this probe: a false "reachable" writes a
// URL nobody has confirmed, and a false "unreachable" costs nothing but the
// fix, which is the direction to err in.
func TestProbeHTTPS(t *testing.T) {
	// A TLS test server plus its own trusted client, with the production
	// redirect policy layered back on. The dial guard below rejects loopback,
	// so the transport is the one part swapped out here — and it is exercised
	// on its own in TestProbeHTTPSRefusesInternalAddresses.
	newServer := func(t *testing.T, h http.HandlerFunc) (string, *http.Client) {
		t.Helper()
		srv := httptest.NewTLSServer(h)
		t.Cleanup(srv.Close)
		client := newProbeClient()
		client.Transport = srv.Client().Transport
		return srv.URL, client
	}

	t.Run("a served URL passes", func(t *testing.T) {
		url, client := newServer(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodHead {
				t.Errorf("method = %s, want HEAD", r.Method)
			}
			w.WriteHeader(http.StatusOK)
		})
		if err := probeWith(client, "pkglint-test", url+"/demo.tar.gz"); err != nil {
			t.Errorf("probe = %v, want success", err)
		}
	})

	// The common real failure: the host speaks https but does not have this
	// path, which is exactly the build the unverified rewrite would have broken.
	t.Run("a 404 fails", func(t *testing.T) {
		url, client := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})
		err := probeWith(client, "pkglint-test", url+"/demo.tar.gz")
		if err == nil || !strings.Contains(err.Error(), "404") {
			t.Errorf("probe = %v, want a 404 refusal", err)
		}
	})

	// Plenty of servers reject HEAD while serving the file happily. That is a
	// statement about the method, not the URL, so the probe asks again for a
	// single byte rather than reporting the source unreachable.
	for name, status := range map[string]int{
		"405": http.StatusMethodNotAllowed,
		"403": http.StatusForbidden,
		"501": http.StatusNotImplemented,
	} {
		t.Run("HEAD "+name+" falls back to a ranged GET", func(t *testing.T) {
			var gets int
			url, client := newServer(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodHead {
					w.WriteHeader(status)
					return
				}
				gets++
				if r.Header.Get("Range") != "bytes=0-0" {
					t.Errorf("Range = %q, want a single byte", r.Header.Get("Range"))
				}
				w.WriteHeader(http.StatusPartialContent)
			})
			if err := probeWith(client, "pkglint-test", url+"/demo.tar.gz"); err != nil {
				t.Errorf("probe = %v, want success via the GET fallback", err)
			}
			if gets != 1 {
				t.Errorf("issued %d GETs, want 1", gets)
			}
		})
	}

	// A redirect landing on http means the https URL does not actually serve
	// the file — the opposite of what the fix would be claiming.
	t.Run("a redirect off https fails", func(t *testing.T) {
		url, client := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
			http.Redirect(w, &http.Request{}, "http://example.com/demo.tar.gz", http.StatusFound)
		})
		err := probeWith(client, "pkglint-test", url+"/demo.tar.gz")
		if err == nil || !strings.Contains(err.Error(), "redirected off https") {
			t.Errorf("probe = %v, want a downgrade refusal", err)
		}
	})

	t.Run("a non-https URL is refused outright", func(t *testing.T) {
		for _, u := range []string{"http://example.com/x", "ftp://example.com/x", "://nonsense"} {
			if err := probeWith(newProbeClient(), "pkglint-test", u); err == nil {
				t.Errorf("probe(%q) = nil, want a refusal", u)
			}
		}
	})
}

// TestProbeHTTPSRefusesInternalAddresses pins the guard that keeps a probe on
// the public internet. The URL being probed comes out of a file pkglint is
// analyzing *because* nobody has vetted it, so a PKGBUILD naming the cloud
// metadata service or a host resolving to one must not turn linting into a
// request to it.
func TestProbeHTTPSRefusesInternalAddresses(t *testing.T) {
	for _, u := range []string{
		"https://127.0.0.1/latest/meta-data/",
		"https://169.254.169.254/latest/meta-data/",
		"https://10.0.0.1/internal.tar.gz",
		"https://192.168.1.1/internal.tar.gz",
		"https://[::1]/internal.tar.gz",
	} {
		t.Run(u, func(t *testing.T) {
			err := ProbeHTTPS(u, "pkglint-test")
			if err == nil || !strings.Contains(err.Error(), "non-public address") {
				t.Errorf("probeHTTPS(%q) = %v, want a non-public-address refusal", u, err)
			}
		})
	}
}

func TestRefuseInternalAddr(t *testing.T) {
	for addr, want := range map[string]bool{
		"93.184.216.34:443":  true, // public
		"[2606:2800::1]:443": true,
		"127.0.0.1:443":      false,
		"[::1]:443":          false,
		"10.1.2.3:443":       false,
		"172.16.0.1:443":     false,
		"192.168.0.5:443":    false,
		"169.254.169.254:80": false,
		"0.0.0.0:443":        false,
		"224.0.0.1:443":      false,
		"[fd00::1]:443":      false, // unique-local
		"not-an-address":     false,
	} {
		err := refuseInternalAddr("tcp", addr, nil)
		if got := err == nil; got != want {
			t.Errorf("refuseInternalAddr(%q) allowed = %v (err %v), want %v", addr, got, err, want)
		}
	}
}

func TestResolveGitRefMatchesExactRef(t *testing.T) {
	A, B, C := strings.Repeat("a", 40), strings.Repeat("b", 40), strings.Repeat("c", 40)
	const url = "https://example.com/demo.git"
	lines := func(l ...string) string { return strings.Join(l, "\n") + "\n" }
	tests := []struct {
		name, out, ref, want, wantErr string
	}{
		{"branch with tag name sorts first", lines(A+"\trefs/heads/feature/v1", B+"\trefs/tags/v1"), "v1", B, ""},
		{"only a lookalike", lines(A + "\trefs/heads/feature/v1"), "v1", "", "not found"},
		{"peeled tag", lines(A+"\trefs/tags/release/v1", B+"\trefs/tags/v1", C+"\trefs/tags/v1^{}"), "v1", C, ""},
		{"peeled other tag ignored", lines(A+"\trefs/tags/release/v1^{}", B+"\trefs/tags/v1"), "v1", B, ""},
		{"branch", lines(A + "\trefs/heads/main"), "main", A, ""},
		{"ambiguous", lines(A+"\trefs/heads/v1", B+"\trefs/tags/v1"), "v1", "", "ambiguous"},
		{"same commit", lines(A+"\trefs/heads/v1", A+"\trefs/tags/v1"), "v1", A, ""},
		{"HEAD", lines(A+"\tHEAD", B+"\trefs/heads/HEAD"), "HEAD", A, ""},
		{"fully qualified", lines(A + "\trefs/tags/v1"), "refs/tags/v1", A, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeGit(t)
			t.Setenv("PKGLINT_TEST_GIT_OUTPUT", tt.out)
			got, err := ResolveGitRef(url, tt.ref)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("resolveGitRef = %q, %v, want error containing %q", got, err, tt.wantErr)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("resolveGitRef = %q, %v, want %q", got, err, tt.want)
			}
		})
	}
}

func TestLsRemoteArgs(t *testing.T) {
	got := lsRemoteArgs("https://example.com/d.git", "v1")
	want := []string{"ls-remote", "--", "https://example.com/d.git", "v1^{}", "v1"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("lsRemoteArgs = %q, want %q", got, want)
	}
}

func TestResolveGitRefAsksForPeeledTag(t *testing.T) {
	calls := fakeGit(t)
	if _, err := ResolveGitRef("https://example.com/d.git", "v1"); err != nil {
		t.Fatal(err)
	}
	got := calls()
	if len(got) != 1 || !strings.HasSuffix(got[0], " ls-remote -- https://example.com/d.git v1^{} v1") {
		t.Errorf("invocations = %q", got)
	}
}

func TestResolveGitRefAnnotatedTagRealGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	// Under a git hook the environment carries GIT_DIR, GIT_INDEX_FILE and
	// friends pointing at the repository being committed to; inherited, they
	// would aim this scratch repository's commit and tag at the real one.
	var env []string
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "GIT_") {
			env = append(env, kv)
		}
	}
	env = append(env,
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@e",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@e")
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = env
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("git %v: %v", args, err)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q", ".")
	git("commit", "-q", "--allow-empty", "-m", "m")
	git("tag", "-a", "v1", "-m", "t")
	out := git(lsRemoteArgs(dir, "v1")...)
	got, err := pickRef(out, "v1")
	if err != nil {
		t.Fatal(err)
	}
	if want := git("rev-parse", "v1^{commit}"); got != want {
		t.Errorf("pickRef = %s, want commit %s", got, want)
	}
	if tag := git("rev-parse", "v1"); got == tag {
		t.Errorf("pickRef returned the tag object %s", tag)
	}
}
