package server

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jmelahman/local-preview/internal/db"
	"github.com/jmelahman/local-preview/internal/githubsso"
)

// envDefault fills *p from the environment variable key when *p is empty,
// giving flags a $PREVIEW_* fallback (the pattern used across serve options).
func envDefault(p *string, key string) {
	if *p == "" {
		*p = os.Getenv(key)
	}
}

// envDefaultInt64 fills *p from the environment variable key when *p is zero,
// the int64 analogue of envDefault. A malformed value is ignored (leaves *p).
func envDefaultInt64(p *int64, key string) {
	if *p != 0 {
		return
	}
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			*p = n
		}
	}
}

// envOverrideInt64 applies key from the environment to *p only while *p still
// holds def (the flag's compiled-in default). This gives a non-zero-default
// flag a $PREVIEW_* fallback while letting an explicit flag — including an
// explicit 0 that disables the limit — win over the environment. A malformed
// value is ignored.
func envOverrideInt64(p *int64, key string, def int64) {
	if *p != def {
		return
	}
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			*p = n
		}
	}
}

// buildSSO validates the SSO flags and constructs the provider, deriving the
// dashboard origin and cookie-security flag from the callback URL. It fails
// closed: any missing secret, callback URL, or allowlist is an error rather
// than a silently-open instance.
func buildSSO(opts serveOptions) (*githubsso.Provider, string, bool, error) {
	if opts.ssoClientSecret == "" {
		return nil, "", false, fmt.Errorf("--sso-github-client-id set without --sso-github-client-secret")
	}
	if opts.ssoCallbackURL == "" {
		return nil, "", false, fmt.Errorf("--sso-github-client-id set without --sso-callback-url")
	}
	u, err := url.Parse(opts.ssoCallbackURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, "", false, fmt.Errorf("invalid --sso-callback-url %q: want e.g. https://host/api/auth/callback", opts.ssoCallbackURL)
	}
	if opts.ssoAllowedTeam != "" && opts.ssoAllowedOrg == "" {
		return nil, "", false, fmt.Errorf("--sso-allowed-team requires --sso-allowed-org")
	}
	allow := githubsso.Allowlist{
		Logins: splitSet(opts.ssoAllowedLogins),
		Emails: splitSet(opts.ssoAllowedEmails),
		Org:    opts.ssoAllowedOrg,
		Team:   opts.ssoAllowedTeam,
	}
	if allow.Empty() {
		return nil, "", false, fmt.Errorf(
			"SSO needs an allowlist: set --sso-allowed-org, --sso-allowed-logins, or --sso-allowed-emails")
	}
	provider := githubsso.New(githubsso.Config{
		ClientID:     opts.ssoClientID,
		ClientSecret: opts.ssoClientSecret,
		CallbackURL:  opts.ssoCallbackURL,
		Allowlist:    allow,
	})
	origin := u.Scheme + "://" + u.Host
	// localhost is a secure context even over plain http, so Secure cookies
	// still work for local testing without HTTPS.
	secure := u.Scheme == "https" || u.Hostname() == "localhost"
	return provider, origin, secure, nil
}

// splitSet turns a comma-separated flag value into a lowercased set, or nil
// when empty.
func splitSet(csv string) map[string]bool {
	out := map[string]bool{}
	for _, p := range strings.Split(csv, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out[strings.ToLower(p)] = true
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// startSessionGC periodically drops expired sessions and preview-grant codes,
// mirroring the other background loops. Best-effort: expired rows are already
// rejected at read time, so a missed sweep only delays cleanup.
func startSessionGC(ctx context.Context, database *db.Store) {
	go func() {
		t := time.NewTicker(time.Hour)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if err := database.DeleteExpiredSessions(); err != nil {
					log.Printf("gc sessions: %v", err)
				}
				if err := database.DeleteExpiredPreviewGrants(); err != nil {
					log.Printf("gc preview grants: %v", err)
				}
			}
		}
	}()
}
