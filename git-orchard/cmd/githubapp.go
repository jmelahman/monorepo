package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jmelahman/git-orchard/ghrun"
	"github.com/jmelahman/git-orchard/git"
	"github.com/jmelahman/git-orchard/githubapp"
)

// Where the action reads the App's credentials from by default.
const (
	appClientIDVariable   = "ORCHARD_APP_CLIENT_ID"
	appPrivateKeySecret   = "ORCHARD_APP_PRIVATE_KEY"
	appPrivateKeyFileMode = 0o600
)

// GitHubAppOptions holds options for the github-app command
type GitHubAppOptions struct {
	Name      string
	Org       string
	Repo      string
	NoSecrets bool
	KeyFile   string
}

// NewGitHubAppCommand creates a new github-app command
func NewGitHubAppCommand() *cobra.Command {
	opts := &GitHubAppOptions{}

	cmd := &cobra.Command{
		Use:   "github-app",
		Short: "Create a GitHub App for the mirror action",
		Long: `Create a GitHub App for the mirror action.

Opens a browser to create a private App from a manifest, with contents and
workflows write permission and no webhook, then on to install it: pick the
upstream repositories there. The App belongs to you (or --org), and so does
its private key; git-orchard runs no service.

With the GitHub CLI installed, the App's client ID and private key are stored
on --repo (by default origin) as the ` + appClientIDVariable + ` variable and the
` + appPrivateKeySecret + ` secret, which the action reads as:

  - uses: jmelahman/git-orchard@v1
    with:
      app-client-id: ${{ vars.` + appClientIDVariable + ` }}
      app-private-key: ${{ secrets.` + appPrivateKeySecret + ` }}

Otherwise, or with --no-secrets, the private key is written to --key-file.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGitHubApp(cmd.Context(), opts)
		},
	}

	cmd.Flags().StringVar(&opts.Name, "name", "", `App name, unique on GitHub (default "<owner>-git-orchard")`)
	cmd.Flags().StringVar(&opts.Org, "org", "", "create the App under this organization instead of your account")
	cmd.Flags().StringVar(&opts.Repo, "repo", "", "OWNER/REPO to store the credentials on (default origin)")
	cmd.Flags().BoolVar(&opts.NoSecrets, "no-secrets", false, "write the private key to --key-file instead of storing it with gh")
	cmd.Flags().StringVar(&opts.KeyFile, "key-file", "", `where to write the private key (default "<app>.private-key.pem")`)

	return cmd
}

func runGitHubApp(ctx context.Context, opts *GitHubAppOptions) error {
	repo := opts.Repo
	if repo == "" {
		if r, err := git.Open("."); err == nil {
			if origin, err := r.Output("config", "--get", "remote.origin.url"); err == nil {
				if owner, name, ok := ghrun.ParseGitHubRepo(origin); ok {
					repo = owner + "/" + name
				}
			}
		}
	}

	name := opts.Name
	if name == "" {
		owner := opts.Org
		if owner == "" && repo != "" {
			owner, _, _ = strings.Cut(repo, "/")
		}
		name = "git-orchard"
		if owner != "" {
			name = owner + "-git-orchard"
		}
	}

	storeWithGH := !opts.NoSecrets && repo != ""
	if storeWithGH {
		if _, err := exec.LookPath("gh"); err != nil {
			fmt.Fprintln(os.Stderr, "gh is not installed; the private key will be written to a file instead.")
			storeWithGH = false
		}
	}

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()
	creds, err := githubapp.Create(ctx, githubapp.Options{
		Name: name,
		Org:  opts.Org,
		Open: func(url string) error {
			fmt.Fprintf(os.Stderr, "Opening %s to create the App (Ctrl-C to cancel)...\n", url)
			if err := openBrowser(url); err != nil {
				fmt.Fprintln(os.Stderr, "Couldn't start a browser; open that URL yourself.")
			}
			return nil
		},
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Created %s (client ID %s).\n", creds.HTMLURL, creds.ClientID)

	if storeWithGH {
		if err := ghRun("", "variable", "set", appClientIDVariable, "--repo", repo, "--body", creds.ClientID); err != nil {
			return err
		}
		if err := ghRun(creds.PEM, "secret", "set", appPrivateKeySecret, "--repo", repo); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "Stored %s and %s on %s.\n", appClientIDVariable, appPrivateKeySecret, repo)
	} else {
		keyFile := opts.KeyFile
		if keyFile == "" {
			keyFile = creds.Slug + ".private-key.pem"
		}
		if err := os.WriteFile(keyFile, []byte(creds.PEM), appPrivateKeyFileMode); err != nil {
			return err
		}
		target := "--repo OWNER/REPO"
		if repo != "" {
			target = "--repo " + repo
		}
		fmt.Fprintf(os.Stderr, "Wrote the private key to %s. Store it, then delete the file:\n\n", keyFile)
		fmt.Fprintf(os.Stderr, "  gh variable set %s %s --body %s\n", appClientIDVariable, target, creds.ClientID)
		fmt.Fprintf(os.Stderr, "  gh secret set %s %s < %s\n\n", appPrivateKeySecret, target, keyFile)
	}

	fmt.Fprintf(os.Stderr, "Install the App on the upstream repositories, if you haven't yet: %s\n", creds.InstallURL())
	return nil
}

func ghRun(stdin string, args ...string) error {
	cmd := exec.Command("gh", args...)
	cmd.Stdin = strings.NewReader(stdin)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("gh %s: %w", strings.Join(args[:2], " "), err)
	}
	return nil
}

func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}
