// Package sshsig lets kanban sign its own commits without ssh-keygen.
//
// With gpg.format=ssh, git does not sign anything itself: it execs
// gpg.ssh.program (default `ssh-keygen`) as
//
//	<program> -Y sign -n git -f <keyfile> [-U] <buffer>
//
// and reads the armored signature back from <buffer>.sig. The kanban image
// ships no ssh-keygen, and a binary install can't count on the host having
// one either, so when [git] sign_commits is on, kanban points
// gpg.ssh.program at its own executable and main dispatches a leading `-Y`
// here (see Main). The private key never enters the process: the signature
// comes from the ssh-agent at SSH_AUTH_SOCK, so the agent socket is still
// the one thing a container has to be given.
//
// Anything this can't do with the agent — any `-Y` verb but sign (git calls
// find-principals and verify for merge.verifySignatures), no agent, a key the
// agent doesn't hold, a keyfile that is a private key rather than a public
// one — is handed to a real ssh-keygen on PATH with the same arguments, so
// setting gpg.ssh.program never does worse than leaving it alone.
package sshsig

import (
	"bytes"
	"crypto/sha512"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// Signature format, per OpenSSH's PROTOCOL.sshsig. Only what git needs is
// implemented: version 1, sha512 (what ssh-keygen itself picks), armored.
const (
	magic      = "SSHSIG"
	sigVersion = 1
	hashAlg    = "sha512"
	armorBegin = "-----BEGIN SSH SIGNATURE-----\n"
	armorEnd   = "-----END SSH SIGNATURE-----\n"
	armorWidth = 70 // ssh-keygen's line length; verify accepts any.
)

// Sign returns the armored SSHSIG signature of message under namespace, made
// by the agent with the private half of pub.
func Sign(a agent.ExtendedAgent, pub ssh.PublicKey, namespace string, message []byte) ([]byte, error) {
	h := sha512.Sum512(message)
	signed := ssh.Marshal(struct {
		Namespace string
		Reserved  string
		HashAlg   string
		Hash      []byte
	}{namespace, "", hashAlg, h[:]})
	signed = append([]byte(magic), signed...)

	// An RSA key must sign with rsa-sha2-512: the agent's default is the
	// SHA-1 ssh-rsa scheme, which `ssh-keygen -Y verify` refuses. The flag is
	// ignored for every other key type.
	var flags agent.SignatureFlags
	if pub.Type() == ssh.KeyAlgoRSA {
		flags = agent.SignatureFlagRsaSha512
	}
	sig, err := a.SignWithFlags(pub, signed, flags)
	if err != nil {
		return nil, fmt.Errorf("agent sign: %w", err)
	}

	blob := ssh.Marshal(struct {
		Version   uint32
		PublicKey []byte
		Namespace string
		Reserved  string
		HashAlg   string
		Signature []byte
	}{sigVersion, pub.Marshal(), namespace, "", hashAlg, ssh.Marshal(sig)})
	blob = append([]byte(magic), blob...)
	return armor(blob), nil
}

func armor(blob []byte) []byte {
	enc := base64.StdEncoding.EncodeToString(blob)
	var b bytes.Buffer
	b.WriteString(armorBegin)
	for len(enc) > armorWidth {
		b.WriteString(enc[:armorWidth])
		b.WriteByte('\n')
		enc = enc[armorWidth:]
	}
	b.WriteString(enc)
	b.WriteByte('\n')
	b.WriteString(armorEnd)
	return b.Bytes()
}

// signRequest is a parsed `-Y sign -n <ns> -f <keyfile> [-U] <buffer>`.
type signRequest struct {
	namespace, keyFile, buffer string
}

// parseSign accepts exactly the argument shape git produces and nothing
// else; anything unexpected is ssh-keygen's to interpret.
func parseSign(args []string) (signRequest, bool) {
	var r signRequest
	if len(args) < 2 || args[0] != "-Y" || args[1] != "sign" {
		return r, false
	}
	rest := args[2:]
	for len(rest) > 1 {
		switch rest[0] {
		case "-n":
			r.namespace, rest = rest[1], rest[2:]
		case "-f":
			r.keyFile, rest = rest[1], rest[2:]
		case "-U": // key is in the agent — the only way this signs anyway.
			rest = rest[1:]
		default:
			return r, false
		}
	}
	if len(rest) != 1 || r.namespace == "" || r.keyFile == "" {
		return r, false
	}
	r.buffer = rest[0]
	return r, true
}

// Main is the gpg.ssh.program entry point; args excludes the program name.
// It returns the process exit code.
func Main(args []string) int {
	req, ok := parseSign(args)
	if !ok {
		return sshKeygen(args, nil)
	}
	if err := signFile(req); err != nil {
		return sshKeygen(args, err)
	}
	return 0
}

func signFile(req signRequest) error {
	keyText, err := os.ReadFile(req.keyFile)
	if err != nil {
		return err
	}
	// git writes a `key::` literal out as a one-line public key; a path
	// signingkey may name either half, and only the public one is ours.
	pub, _, _, _, err := ssh.ParseAuthorizedKey(keyText)
	if err != nil {
		return fmt.Errorf("%s is not a public key: %w", req.keyFile, err)
	}
	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock == "" {
		return errors.New("SSH_AUTH_SOCK is not set")
	}
	conn, err := net.Dial("unix", sock)
	if err != nil {
		return fmt.Errorf("ssh-agent: %w", err)
	}
	defer conn.Close()
	message, err := os.ReadFile(req.buffer)
	if err != nil {
		return err
	}
	sig, err := Sign(agent.NewClient(conn), pub, req.namespace, message)
	if err != nil {
		return err
	}
	return os.WriteFile(req.buffer+".sig", sig, 0o644)
}

// sshKeygen runs the real ssh-keygen with args. cause, when set, is why the
// agent path gave up, reported only if there is no ssh-keygen to fall back
// on — git shows the program's stderr verbatim, and "ssh-keygen: not found"
// alone would hide the actual problem.
func sshKeygen(args []string, cause error) int {
	path, err := exec.LookPath("ssh-keygen")
	if err != nil {
		if cause != nil {
			fmt.Fprintf(os.Stderr, "kanban: cannot sign with ssh-agent (%v), and no ssh-keygen to fall back on\n", cause)
		} else {
			fmt.Fprintln(os.Stderr, "kanban: no ssh-keygen for", args)
		}
		return 1
	}
	cmd := exec.Command(path, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		if ee, ok := errors.AsType[*exec.ExitError](err); ok {
			return ee.ExitCode()
		}
		fmt.Fprintln(os.Stderr, "kanban:", err)
		return 1
	}
	return 0
}
