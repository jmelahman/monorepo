package sshsig

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/jmelahman/kanban/internal/gittest"
)

// TestMain lets the test binary stand in for kanban as gpg.ssh.program, the
// way main.go dispatches a leading -Y, so TestGitCommit exercises the path git
// really takes.
//
// It also isolates git from the caller's environment and gitconfig (see
// gittest.IsolateEnv). Under a pre-commit hook, TestGitCommit's `git commit`
// would otherwise build its tree from the index of whatever repo is being
// committed to ("invalid object ... Error building trees").
func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == "-Y" {
		os.Exit(Main(os.Args[1:]))
	}
	gittest.IsolateEnv()
	os.Exit(m.Run())
}

// testAgent serves a keyring holding one fresh key of each type over a unix
// socket and points SSH_AUTH_SOCK at it.
func testAgent(t *testing.T) (ed, rs ssh.PublicKey) {
	t.Helper()
	_, edPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	rsaPriv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	kr := agent.NewKeyring()
	for _, k := range []any{edPriv, rsaPriv} {
		if err := kr.Add(agent.AddedKey{PrivateKey: k}); err != nil {
			t.Fatal(err)
		}
	}
	// A short path: t.TempDir() can exceed the 108-byte sun_path limit.
	dir, err := os.MkdirTemp("", "sshsig")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	sock := filepath.Join(dir, "agent.sock")
	l, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			go func() { defer c.Close(); _ = agent.ServeAgent(kr, c) }()
		}
	}()
	t.Setenv("SSH_AUTH_SOCK", sock)

	edPub, _ := ssh.NewPublicKey(edPriv.Public())
	rsPub, _ := ssh.NewPublicKey(&rsaPriv.PublicKey)
	return edPub, rsPub
}

// verify checks sig with the real ssh-keygen, which is the reader that
// matters: it is what `git log --show-signature` and GitHub's verifier agree
// with.
func verify(t *testing.T, pub ssh.PublicKey, message, sig []byte) {
	t.Helper()
	if _, err := exec.LookPath("ssh-keygen"); err != nil {
		t.Skip("no ssh-keygen to verify against")
	}
	dir := t.TempDir()
	signers := filepath.Join(dir, "allowed_signers")
	line := "kanban@test " + strings.TrimSpace(string(ssh.MarshalAuthorizedKey(pub))) + "\n"
	if err := os.WriteFile(signers, []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}
	sigFile := filepath.Join(dir, "msg.sig")
	if err := os.WriteFile(sigFile, sig, 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("ssh-keygen", "-Y", "verify", "-f", signers, "-I", "kanban@test", "-n", "git", "-s", sigFile)
	cmd.Stdin = strings.NewReader(string(message))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("ssh-keygen -Y verify: %v\n%s\n%s", err, out, sig)
	}
}

func TestSign_VerifiesWithSSHKeygen(t *testing.T) {
	ed, rs := testAgent(t)
	conn, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	a := agent.NewClient(conn)
	message := []byte("tree 4b825dc642cb6eb9a060e54bf8d69288fbee4904\n\nhello\n")
	for name, pub := range map[string]ssh.PublicKey{"ed25519": ed, "rsa": rs} {
		t.Run(name, func(t *testing.T) {
			sig, err := Sign(a, pub, "git", message)
			if err != nil {
				t.Fatal(err)
			}
			verify(t, pub, message, sig)
		})
	}
}

func TestParseSign(t *testing.T) {
	cases := []struct {
		args []string
		ok   bool
	}{
		{[]string{"-Y", "sign", "-n", "git", "-f", "/k", "-U", "/b"}, true},
		{[]string{"-Y", "sign", "-n", "git", "-f", "/k", "/b"}, true},
		{[]string{"-Y", "verify", "-n", "git", "-f", "/k", "-I", "x", "-s", "/s"}, false},
		{[]string{"-Y", "sign", "-n", "git", "/b"}, false},       // no key
		{[]string{"-Y", "sign", "-n", "git", "-f", "/k"}, false}, // no buffer
		{[]string{"-Y", "sign", "-O", "x", "-n", "git", "-f", "/k", "/b"}, false},
	}
	for _, c := range cases {
		r, ok := parseSign(c.args)
		if ok != c.ok {
			t.Errorf("parseSign(%q) ok = %v, want %v", c.args, ok, c.ok)
		}
		if ok && (r.namespace != "git" || r.keyFile != "/k" || r.buffer != "/b") {
			t.Errorf("parseSign(%q) = %+v", c.args, r)
		}
	}
}

// TestGitCommit drives a real `git commit` with gpg.ssh.program pointed at
// this binary and ssh-keygen hidden from PATH, so a signed commit can only
// have come from the agent path and not from the fallback.
func TestGitCommit(t *testing.T) {
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Skip("no git")
	}
	_, rs := testAgent(t)
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	if err := os.Symlink(gitPath, filepath.Join(bin, "git")); err != nil {
		t.Fatal(err)
	}
	repo := t.TempDir()
	key := "key::" + strings.TrimSpace(string(ssh.MarshalAuthorizedKey(rs)))
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command(gitPath, append([]string{
			"-C", repo,
			"-c", "user.name=Ada", "-c", "user.email=ada@example.com",
			"-c", "gpg.format=ssh", "-c", "user.signingkey=" + key,
			"-c", "gpg.ssh.program=" + self,
			"-c", "core.hooksPath=/dev/null",
		}, args...)...)
		cmd.Env = append(os.Environ(), "PATH="+bin, "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return string(out)
	}
	git("init", "-q")
	git("commit", "-q", "-S", "--allow-empty", "-m", "signed")
	raw := git("cat-file", "commit", "HEAD")
	if !strings.Contains(raw, "-----BEGIN SSH SIGNATURE-----") {
		t.Fatalf("commit carries no SSH signature:\n%s", raw)
	}
	// Split the gpgsig header back out and check it against the payload git
	// signed, which is the commit object minus that header.
	var payload, sig strings.Builder
	inSig := false
	for _, line := range strings.SplitAfter(raw, "\n") {
		switch {
		case strings.HasPrefix(line, "gpgsig "):
			inSig = true
			sig.WriteString(strings.TrimPrefix(line, "gpgsig "))
		case inSig && strings.HasPrefix(line, " "):
			sig.WriteString(line[1:])
		default:
			inSig = false
			payload.WriteString(line)
		}
	}
	verify(t, rs, []byte(payload.String()), []byte(sig.String()))
}
