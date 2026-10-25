// Package netprobe is the one place pkglint reaches the network on the
// strength of a URL taken from a PKGBUILD: `git ls-remote` to resolve a VCS
// ref, and an https HEAD to check that a rewritten source is served. Both are
// confined here so the guards — the git transport allowlist, the public-address
// dial check, the https-only redirect rule — are the only way in, and so they
// can be read without the rest of the command around them.
package netprobe

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

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

// lsRemoteArgs is the ls-remote invocation for ref. The second pattern asks
// for the peeled line: for an annotated tag, refs/tags/v1 is the tag object
// and only refs/tags/v1^{} is the commit makepkg checks out, and ls-remote
// lists it only when a pattern matches it. The ref stays last, and "--" ends
// the options so neither the URL nor a ref can be read as one.
func lsRemoteArgs(url, ref string) []string {
	return []string{"ls-remote", "--", url, ref + "^{}", ref}
}

// ResolveGitRef resolves a git tag or branch name on a remote to its commit
// hash via `git ls-remote` — the only network access the fix path performs.
func ResolveGitRef(rawurl, ref string) (string, error) {
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
	cmd := exec.CommandContext(ctx, "git", lsRemoteArgs(url, ref)...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0") // never block on credentials
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	sha, err := pickRef(string(out), ref)
	if err != nil {
		if strings.HasSuffix(err.Error(), "not found") {
			return "", fmt.Errorf("ref %q not found on %s", ref, url)
		}
		return "", err
	}
	return sha, nil
}

// pickRef chooses the commit for ref out of `git ls-remote` output. ls-remote
// matches its pattern against the tail of every ref, so a request for v1 also
// lists refs/heads/feature/v1 and refs/tags/release/v1; taking the first line
// pinned whichever of them sorted first, and anyone able to push a branch
// could choose which. Only the ref itself counts: the tag (its peeled commit
// when annotated), the branch, HEAD, or a fully qualified name. A tag and a
// branch that share the name but not the commit is refused rather than
// guessed, since the fragment kind (#tag= or #branch=) is not known here.
func pickRef(out, ref string) (string, error) {
	var exact, exactPeeled, tag, tagPeeled, head string
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(line)
		if len(f) < 2 {
			continue
		}
		switch f[1] {
		case "refs/tags/" + ref + "^{}":
			tagPeeled = f[0]
		case "refs/tags/" + ref:
			tag = f[0]
		case "refs/heads/" + ref:
			head = f[0]
		case ref + "^{}":
			exactPeeled = f[0]
		case ref:
			exact = f[0]
		}
	}
	if exactPeeled != "" {
		return exactPeeled, nil
	}
	if exact != "" {
		return exact, nil
	}
	tagSHA := tag
	if tagPeeled != "" {
		tagSHA = tagPeeled
	}
	if tagSHA != "" && head != "" && tagSHA != head {
		return "", fmt.Errorf("ref %q is ambiguous: both a tag and a branch, at different commits", ref)
	}
	if tagSHA != "" {
		return tagSHA, nil
	}
	if head != "" {
		return head, nil
	}
	return "", fmt.Errorf("ref %q not found", ref)
}

// probeTimeout bounds one https probe end to end, redirects included. A host
// that has not answered by then is one the fix should not bet a build on.
const probeTimeout = 15 * time.Second

// ProbeHTTPS reports whether an https URL is served, by asking for headers and
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
func ProbeHTTPS(rawurl, userAgent string) error {
	return probeWith(newProbeClient(), userAgent, rawurl)
}

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

func probeWith(client *http.Client, userAgent, rawurl string) error {
	u, err := url.Parse(rawurl)
	if err != nil {
		return fmt.Errorf("unparseable URL %q: %w", rawurl, err)
	}
	if !strings.EqualFold(u.Scheme, "https") || u.Host == "" {
		return fmt.Errorf("refusing to probe non-https URL %q", rawurl)
	}
	u.User = nil // never replay credentials a PKGBUILD embedded in a URL

	status, err := probeOnce(client, userAgent, http.MethodHead, u.String(), nil)
	if err != nil {
		return err
	}
	if status == http.StatusMethodNotAllowed || status == http.StatusNotImplemented || status == http.StatusForbidden {
		// The server dislikes HEAD, not the URL. Ask for one byte instead.
		status, err = probeOnce(client, userAgent, http.MethodGet, u.String(), map[string]string{"Range": "bytes=0-0"})
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
func probeOnce(client *http.Client, userAgent, method, url string, headers map[string]string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", userAgent)
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
