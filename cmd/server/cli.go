package server

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/jmelahman/local-preview/internal/client"
	"github.com/jmelahman/local-preview/internal/config"
)

// defaultServerURL is where the CLI looks when nothing else says otherwise:
// a `preview serve` on this machine.
const defaultServerURL = "http://localhost:8080"

// addClientCommands attaches the user-facing CLI subcommand groups to root.
// They're thin wrappers over the HTTP API, useful for scripting and remote
// control of a running `preview serve`.
func addClientCommands(root *cobra.Command) {
	root.AddCommand(repoCmd(), deployCmd(), uploadCmd(), openCmd(), logsCmd(), statsCmd(), execCmd(), installHookCmd(), configureCmd())
}

// resolveURL returns the effective server URL for a leaf command: an
// explicit --server wins, otherwise the ambient configuration decides.
func resolveURL(cmd *cobra.Command, serverURL string) (string, error) {
	if cmd.Flags().Changed("server") {
		return serverURL, nil
	}
	url, _, err := effectiveServer()
	return url, err
}

// effectiveServer resolves the server URL used when no --server is passed,
// in precedence order: $PREVIEW_URL, the config file, then the built-in
// default. It also reports which of those won, for `preview configure`.
// A malformed config file is an error rather than a fall-through — silently
// talking to localhost when the file names a remote server would look like
// success.
func effectiveServer() (url, source string, err error) {
	if env := os.Getenv("PREVIEW_URL"); env != "" {
		return env, "$PREVIEW_URL", nil
	}
	cfg, err := config.LoadClientConfig()
	if err != nil {
		return "", "", err
	}
	if cfg.Server != "" {
		return cfg.Server, "config file", nil
	}
	return defaultServerURL, "default", nil
}

// effectiveToken resolves the bearer token CLI subcommands present, in
// precedence order: $PREVIEW_TOKEN, the config file's token, then the GitHub
// CLI's stored credential (`gh auth token`) when gh is installed and signed
// in. Empty means send none, which is correct against a server without SSO.
//
// The gh fallback is what makes an SSO-gated server work with zero client
// setup: gh's OAuth token carries read:org — enough for the server's
// org-allowlist check — so anyone already using gh authenticates without
// minting a PAT or running preview configure --token.
func effectiveToken() (string, error) {
	if env := os.Getenv("PREVIEW_TOKEN"); env != "" {
		return env, nil
	}
	cfg, err := config.LoadClientConfig()
	if err != nil {
		return "", err
	}
	if cfg.Token != "" {
		return cfg.Token, nil
	}
	return ghAuthToken(), nil
}

// ghAuthToken returns the GitHub CLI's stored token, or "" when gh is
// missing, signed out, or slow — the fallback must never break a CLI that
// worked without it.
func ghAuthToken() string {
	gh, err := exec.LookPath("gh")
	if err != nil {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, gh, "auth", "token").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// newClient builds an HTTP client for the server at url, attaching the
// configured bearer token (a GitHub personal-access token) when one is set.
// Use it instead of client.New so every subcommand authenticates uniformly.
func newClient(url string) (*client.Client, error) {
	tok, err := effectiveToken()
	if err != nil {
		return nil, err
	}
	c := client.New(url, nil)
	if tok != "" {
		c.SetToken(tok)
	}
	return c, nil
}

// addServerFlag registers --server as a persistent flag on the parent group
// so every leaf inherits it. The same string variable is reused across the
// group's subcommands.
func addServerFlag(parent *cobra.Command, dst *string) {
	// No backticks in the usage string: cobra reads a back-quoted word as
	// the flag's value placeholder.
	parent.PersistentFlags().StringVar(dst, "server", defaultServerURL,
		"Base URL of the HTTP server (default: $PREVIEW_URL, else the server set by 'preview configure')")
}
