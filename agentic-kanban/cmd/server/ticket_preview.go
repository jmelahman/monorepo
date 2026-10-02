package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/jmelahman/kanban/internal/client"
)

// previewPollInterval matches the web UI's Previews tab while a deploy is
// in flight. A var so tests can shorten it.
var previewPollInterval = 1500 * time.Millisecond

// previewLogTail is how many trailing build-log lines a failed deploy
// prints: enough to show the error without flooding the terminal.
const previewLogTail = 20

func ticketPreviewCmd(serverURL, boardIdent *string) *cobra.Command {
	var noWait, asJSON bool
	cmd := &cobra.Command{
		Use:   "preview [id]",
		Short: "Deploy a preview of a ticket's branch and print its URL",
		Long: `Deploy the tip of a ticket's branch as a live preview, the same as
"deploy tip" on the web UI's Previews tab, then wait for the build and
print the preview URL along with any downloadable artifacts.

Deploys are idempotent per commit: if the tip is already deployed, its
existing preview is reported right away. A failed build prints the end of
its log and exits non-zero. With --no-wait the deploy is only requested.

The ticket needs a session (its branch); start one with
"kanban ticket attach". The repo needs a preview manifest; see the
previews guide.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			url := resolveURL(cmd, *serverURL)
			id, err := ticketArg(ctx, url, args, *boardIdent, pickerAction{"Preview ticket branch", "preview"}, false)
			if err != nil {
				return err
			}
			return runTicketPreview(ctx, url, cmd.OutOrStdout(), id, !noWait, asJSON)
		},
	}
	cmd.Flags().BoolVar(&noWait, "no-wait", false, "Request the deploy and exit without waiting for the build")
	cmd.Flags().BoolVar(&asJSON, "json", false, "Print the deploy JSON instead of a summary")
	return cmd
}

// runTicketPreview requests a deploy of the ticket session's branch and,
// with wait, follows it until it settles.
func runTicketPreview(ctx context.Context, url string, out io.Writer, id int64, wait, asJSON bool) error {
	info, err := loadTicketInfo(ctx, url, id)
	if err != nil {
		return err
	}
	if info.Session == nil {
		return fmt.Errorf("ticket %d has no session (and so no branch) to preview; start one with `kanban ticket attach %d`", id, id)
	}
	c := client.New(url, nil)
	raw, err := c.CreateSessionPreview(ctx, info.Session.ID)
	if err != nil {
		return err
	}
	p, err := decodePreview(raw)
	if err != nil {
		return err
	}
	if !asJSON {
		fmt.Fprintf(out, "preview #%d of %s (%s): %s\n", p.ID, info.Session.BranchName, p.ShortSHA, p.Status)
	}
	last := p.Status
	for wait && previewInFlight(p.Status) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(previewPollInterval):
		}
		if raw, err = c.GetPreview(ctx, p.ID); err != nil {
			return err
		}
		if p, err = decodePreview(raw); err != nil {
			return err
		}
		if !asJSON && p.Status != last {
			fmt.Fprintf(out, "preview #%d: %s\n", p.ID, p.Status)
			last = p.Status
		}
	}

	if asJSON {
		if _, err := fmt.Fprintln(out, string(raw)); err != nil {
			return err
		}
	} else if p.Status == "ready" {
		printPreviewReady(out, c, p)
	}
	switch p.Status {
	case "failed":
		if !asJSON {
			printPreviewLogTail(ctx, out, c, p.ID)
		}
		if p.Error != "" {
			return fmt.Errorf("preview #%d failed: %s", p.ID, p.Error)
		}
		return fmt.Errorf("preview #%d failed", p.ID)
	case "evicted":
		return fmt.Errorf("preview #%d was evicted; redeploy it from the web UI's Previews tab", p.ID)
	}
	return nil
}

func decodePreview(raw json.RawMessage) (client.Preview, error) {
	var p client.Preview
	if err := json.Unmarshal(raw, &p); err != nil {
		return p, fmt.Errorf("decode preview: %w", err)
	}
	return p, nil
}

func previewInFlight(status string) bool {
	return status == "queued" || status == "building"
}

// printPreviewReady prints the preview URL and each artifact file's
// download URL. A downloads-only deploy (no frontend or backend) has a URL
// that only lists its downloads, so it's left out, as in the web UI.
func printPreviewReady(out io.Writer, c *client.Client, p client.Preview) {
	if p.PreviewURL != "" && (p.FeHash != "" || p.BeHash != "") {
		fmt.Fprintln(out, p.PreviewURL)
	}
	for _, a := range p.Artifacts {
		for _, f := range a.Files {
			fmt.Fprintf(out, "%s/%s: %s\n", a.Name, f.Name, c.PreviewArtifactURL(p.ID, a.Name, f.Name))
		}
	}
}

// printPreviewLogTail prints the last lines of a failed deploy's build log.
// The log is a diagnostic extra, so failing to fetch it is not an error.
func printPreviewLogTail(ctx context.Context, out io.Writer, c *client.Client, id int64) {
	logs, err := c.PreviewLogs(ctx, id)
	if err != nil {
		return
	}
	lines := strings.Split(strings.TrimRight(logs, "\n"), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return
	}
	if len(lines) > previewLogTail {
		lines = lines[len(lines)-previewLogTail:]
	}
	fmt.Fprintf(out, "--- build log (last %d lines) ---\n%s\n", len(lines), strings.Join(lines, "\n"))
}
