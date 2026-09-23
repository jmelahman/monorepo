package server

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/jmelahman/local-preview/internal/client"
	"github.com/jmelahman/local-preview/internal/config"
)

// verifyTimeout bounds the post-write health check. It's a courtesy ping,
// not a gate — the URL is already saved by then.
const verifyTimeout = 5 * time.Second

func configureCmd() *cobra.Command {
	var show, unset bool
	var token string

	cmd := &cobra.Command{
		Use:   "configure [url]",
		Short: "Set the server CLI subcommands talk to",
		Long: "Store the base URL of the `preview serve` instance the other\n" +
			"subcommands talk to, so `preview open`, `preview deploy`, and friends\n" +
			"reach it without --server or $PREVIEW_URL. The setting lives in the\n" +
			"user config file and applies to every shell, including the git hooks\n" +
			"installed by `preview install-hook`.\n\n" +
			"With no URL, prompts for one. $PREVIEW_URL and an explicit --server\n" +
			"still win over what's stored here.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			if show && unset {
				return fmt.Errorf("--show and --unset are mutually exclusive")
			}
			if show {
				if len(args) == 1 {
					return fmt.Errorf("--show takes no URL")
				}
				return runConfigureShow(out)
			}
			if unset {
				if len(args) == 1 {
					return fmt.Errorf("--unset takes no URL")
				}
				return runConfigureUnset(out)
			}
			tokenChanged := cmd.Flags().Changed("token")
			url := ""
			if len(args) == 1 {
				url = strings.TrimSpace(args[0])
			}
			// Setting only the token shouldn't prompt for a server URL.
			if tokenChanged && url == "" {
				return runConfigureToken(out, token)
			}
			return runConfigure(cmd.Context(), cmd.InOrStdin(), out, url, token, tokenChanged)
		},
	}
	cmd.Flags().BoolVar(&show, "show", false, "Print the current configuration instead of changing it")
	cmd.Flags().BoolVar(&unset, "unset", false, "Remove the configured server, falling back to "+defaultServerURL)
	cmd.Flags().StringVar(&token, "token", "", "Store a bearer token (GitHub personal-access token) presented to a server that requires authentication; empty clears it")
	return cmd
}

// runConfigureShow reports what's stored and what actually takes effect —
// the two differ whenever $PREVIEW_URL is set.
func runConfigureShow(out io.Writer) error {
	path := config.ClientConfigPath()
	if path == "" {
		fmt.Fprintln(out, "config file:       (none: no config directory available)")
	} else if _, err := os.Stat(path); err != nil {
		fmt.Fprintf(out, "config file:       %s (not created yet)\n", path)
	} else {
		fmt.Fprintf(out, "config file:       %s\n", path)
	}
	cfg, err := config.LoadClientConfig()
	if err != nil {
		return err
	}
	stored := cfg.Server
	if stored == "" {
		stored = "(unset)"
	}
	fmt.Fprintf(out, "configured server: %s\n", stored)
	tokenState := "(unset)"
	if cfg.Token != "" {
		tokenState = "(set)"
	}
	if env := os.Getenv("PREVIEW_TOKEN"); env != "" {
		tokenState = "(set via $PREVIEW_TOKEN)"
	}
	fmt.Fprintf(out, "configured token:  %s\n", tokenState)
	url, source, err := effectiveServer()
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "effective server:  %s (from %s)\n", url, source)
	return nil
}

func runConfigureUnset(out io.Writer) error {
	cfg, err := config.LoadClientConfig()
	if err != nil {
		return err
	}
	if cfg.Server == "" {
		fmt.Fprintf(out, "no server configured; subcommands use %s\n", defaultServerURL)
		return nil
	}
	cfg.Server = ""
	path, err := config.SaveClientConfig(cfg)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "unset server in %s; subcommands use %s\n", path, defaultServerURL)
	return nil
}

// runConfigureToken stores (or, when empty, clears) the CLI's bearer token
// without touching the server URL.
func runConfigureToken(out io.Writer, token string) error {
	cfg, err := config.LoadClientConfig()
	if err != nil {
		return err
	}
	cfg.Token = token
	path, err := config.SaveClientConfig(cfg)
	if err != nil {
		return err
	}
	if token == "" {
		fmt.Fprintf(out, "cleared token in %s\n", path)
	} else {
		fmt.Fprintf(out, "stored token in %s\n", path)
	}
	return nil
}

// runConfigure stores url as the server, prompting for it when empty, then
// pings it so a typo surfaces here rather than on the next command. It also
// updates the stored token when setToken is set.
func runConfigure(ctx context.Context, in io.Reader, out io.Writer, url, token string, setToken bool) error {
	cfg, err := config.LoadClientConfig()
	if err != nil {
		return err
	}
	if url == "" {
		current := cfg.Server
		if current == "" {
			current = defaultServerURL
		}
		if url, err = promptServer(in, out, current); err != nil {
			return err
		}
	}
	if err := config.ValidateServerURL(url); err != nil {
		return err
	}
	cfg.Server = strings.TrimRight(url, "/")
	if setToken {
		cfg.Token = token
	}
	path, err := config.SaveClientConfig(cfg)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "server set to %s in %s\n", cfg.Server, path)
	if setToken {
		if token == "" {
			fmt.Fprintf(out, "cleared token\n")
		} else {
			fmt.Fprintf(out, "stored token\n")
		}
	}
	if env := os.Getenv("PREVIEW_URL"); env != "" && env != cfg.Server {
		fmt.Fprintf(out, "note: $PREVIEW_URL is set to %s and overrides this in the current shell\n", env)
	}
	verifyServer(ctx, out, cfg.Server)
	return nil
}

// promptServer asks for a server URL, offering current as the default.
func promptServer(in io.Reader, out io.Writer, current string) (string, error) {
	fmt.Fprintf(out, "Server URL [%s]: ", current)
	line, err := bufio.NewReader(in).ReadString('\n')
	if line == "" && err != nil {
		return "", fmt.Errorf("no server URL given; pass one as an argument: preview configure <url>")
	}
	if answer := strings.TrimSpace(line); answer != "" {
		return answer, nil
	}
	return current, nil
}

// verifyServer reports what answers at url. An unreachable server is a
// warning, not a failure: configuring the CLI before starting the server is
// a legitimate order to do things in.
func verifyServer(ctx context.Context, out io.Writer, url string) {
	ctx, cancel := context.WithTimeout(ctx, verifyTimeout)
	defer cancel()
	h, err := client.New(url, nil).GetHealth(ctx)
	if err != nil {
		fmt.Fprintf(out, "warning: could not reach %s: %v\n", url, err)
		return
	}
	fmt.Fprintf(out, "reached %s (version %s, previews at *.%s)\n", url, h.Version, h.PreviewDomain)
}
