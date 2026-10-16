package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jmelahman/vinder/internal/githubapp"
)

// Where the upgrade workflow reads the App's credentials from.
const (
	appClientIDVariable = "VINDER_APP_CLIENT_ID"
	appPrivateKeySecret = "VINDER_APP_PRIVATE_KEY"
)

func newGitHubAppCmd() *cobra.Command {
	var (
		name, org, repo, keyFile string
		noSecrets                bool
	)
	cmd := &cobra.Command{
		Use:   "github-app",
		Short: "Create a GitHub App for the upgrade workflow to open pull requests as",
		Long: `Create a GitHub App for the upgrade workflow to open pull requests as.

A pull request opened with GITHUB_TOKEN doesn't trigger the repository's
other workflows, so CI would never run on the upgrade. One opened as an App
does.

Opens a browser to create a private App from a manifest, with contents and
pull requests write permission and no webhook, then on to install it: pick
the repository the workflow runs in. The App belongs to you (or --org), and
so does its private key; vinder runs no service.

With the GitHub CLI installed, the App's client ID and private key are stored
on --repo (by default origin) as the ` + appClientIDVariable + ` variable and the
` + appPrivateKeySecret + ` secret, which the workflow reads as:

  - uses: actions/create-github-app-token@v3
    with:
      client-id: ${{ vars.` + appClientIDVariable + ` }}
      private-key: ${{ secrets.` + appPrivateKeySecret + ` }}

Otherwise, or with --no-secrets, the private key is written to --key-file.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runGitHubApp(cmd.Context(), cmd.ErrOrStderr(), name, org, repo, keyFile, noSecrets)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", `App name, unique on GitHub (default "<owner>-vinder")`)
	cmd.Flags().StringVar(&org, "org", "", "create the App under this organization instead of your account")
	cmd.Flags().StringVar(&repo, "repo", "", "OWNER/REPO to store the credentials on (default origin)")
	cmd.Flags().BoolVar(&noSecrets, "no-secrets", false, "write the private key to --key-file instead of storing it with gh")
	cmd.Flags().StringVar(&keyFile, "key-file", "", `where to write the private key (default "<app>.private-key.pem")`)
	return cmd
}

func runGitHubApp(ctx context.Context, stderr io.Writer, name, org, repo, keyFile string, noSecrets bool) error {
	if repo == "" {
		if origin, err := exec.CommandContext(ctx, "git", "config", "--get", "remote.origin.url").Output(); err == nil {
			if owner, r, ok := githubapp.ParseRepo(strings.TrimSpace(string(origin))); ok {
				repo = owner + "/" + r
			}
		}
	}
	if name == "" {
		owner := org
		if owner == "" && repo != "" {
			owner, _, _ = strings.Cut(repo, "/")
		}
		name = "vinder"
		if owner != "" {
			name = owner + "-vinder"
		}
	}

	storeWithGH := !noSecrets && repo != ""
	if storeWithGH {
		if _, err := exec.LookPath("gh"); err != nil {
			fmt.Fprintln(stderr, "gh is not installed; the private key will be written to a file instead.")
			storeWithGH = false
		}
	}

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()
	creds, err := githubapp.Create(ctx, githubapp.Options{
		Name: name,
		Org:  org,
		Open: func(url string) error {
			fmt.Fprintf(stderr, "Opening %s to create the App (Ctrl-C to cancel)...\n", url)
			if err := openBrowser(ctx, url); err != nil {
				fmt.Fprintln(stderr, "Couldn't start a browser; open that URL yourself.")
			}
			return nil
		},
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(stderr, "Created %s (client ID %s).\n", creds.HTMLURL, creds.ClientID)

	if storeWithGH {
		if err := gh(ctx, stderr, "", "variable", "set", appClientIDVariable, "--repo", repo, "--body", creds.ClientID); err != nil {
			return err
		}
		if err := gh(ctx, stderr, creds.PEM, "secret", "set", appPrivateKeySecret, "--repo", repo); err != nil {
			return err
		}
		fmt.Fprintf(stderr, "Stored %s and %s on %s.\n", appClientIDVariable, appPrivateKeySecret, repo)
	} else {
		if keyFile == "" {
			keyFile = creds.Slug + ".private-key.pem"
		}
		if err := os.WriteFile(keyFile, []byte(creds.PEM), 0o600); err != nil {
			return err
		}
		target := "--repo OWNER/REPO"
		if repo != "" {
			target = "--repo " + repo
		}
		fmt.Fprintf(stderr, "Wrote the private key to %s. Store it, then delete the file:\n\n", keyFile)
		fmt.Fprintf(stderr, "  gh variable set %s %s --body %s\n", appClientIDVariable, target, creds.ClientID)
		fmt.Fprintf(stderr, "  gh secret set %s %s < %s\n\n", appPrivateKeySecret, target, keyFile)
	}

	fmt.Fprintf(stderr, "Install the App on the repository the workflow runs in, if you haven't yet: %s\n", creds.InstallURL())
	return nil
}

// gh runs the GitHub CLI with stdin as its input.
func gh(ctx context.Context, stderr io.Writer, stdin string, args ...string) error {
	cmd := exec.CommandContext(ctx, "gh", args...)
	cmd.Stdin = strings.NewReader(stdin)
	cmd.Stdout = stderr
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("gh %s: %w", strings.Join(args[:2], " "), err)
	}
	return nil
}

func openBrowser(ctx context.Context, url string) error {
	// The browser outlives the command, so it mustn't be killed with it.
	ctx = context.WithoutCancel(ctx)
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.CommandContext(ctx, "open", url)
	case "windows":
		cmd = exec.CommandContext(ctx, "rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.CommandContext(ctx, "xdg-open", url)
	}
	return cmd.Start()
}
