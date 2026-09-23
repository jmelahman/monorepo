package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/jmelahman/fullstack-template/internal/client"
)

// addClientCommands attaches the user-facing CLI subcommand groups (`item`)
// to root. They're thin wrappers over the HTTP API, useful for scripting and
// remote control of a running `app serve`.
func addClientCommands(root *cobra.Command) {
	root.AddCommand(itemCmd())
}

// resolveURL returns the effective server URL for a leaf command. APP_URL
// wins only when the user didn't explicitly pass --server.
func resolveURL(cmd *cobra.Command, serverURL string) string {
	if env := os.Getenv("APP_URL"); env != "" && !cmd.Flags().Changed("server") {
		return env
	}
	return serverURL
}

// addServerFlag registers --server as a persistent flag on the parent group
// so every leaf inherits it. The same string variable is reused across the
// group's subcommands.
func addServerFlag(parent *cobra.Command, dst *string) {
	parent.PersistentFlags().StringVar(dst, "server", "http://localhost:8080",
		"Base URL of the HTTP server")
}

func itemCmd() *cobra.Command {
	var serverURL string
	parent := &cobra.Command{
		Use:   "item",
		Short: "Manage items",
	}
	addServerFlag(parent, &serverURL)

	list := &cobra.Command{
		Use:   "list",
		Short: "List items as a table",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runItemList(cmd.Context(), resolveURL(cmd, serverURL), cmd.OutOrStdout())
		},
	}

	var (
		icTitle string
		icJSON  bool
	)
	create := &cobra.Command{
		Use:   "create",
		Short: "Create a new item",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runItemCreate(cmd.Context(), resolveURL(cmd, serverURL), cmd.OutOrStdout(), icTitle, icJSON)
		},
	}
	create.Flags().StringVar(&icTitle, "title", "", "Item title (required)")
	create.Flags().BoolVar(&icJSON, "json", false, "Print the full item JSON instead of a one-line summary")
	_ = create.MarkFlagRequired("title")

	del := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete an item",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseInt64(args[0], "item id")
			if err != nil {
				return err
			}
			return runItemDelete(cmd.Context(), resolveURL(cmd, serverURL), cmd.OutOrStdout(), id)
		},
	}

	parent.AddCommand(list, create, del)
	return parent
}

func runItemList(ctx context.Context, url string, out io.Writer) error {
	items, err := client.New(url, nil).ListItems(ctx)
	if err != nil {
		return err
	}
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tTITLE\tCREATED")
	for _, it := range items {
		fmt.Fprintf(tw, "%d\t%s\t%s\n", it.ID, it.Title, it.CreatedAt)
	}
	return tw.Flush()
}

func runItemCreate(ctx context.Context, url string, out io.Writer, title string, asJSON bool) error {
	raw, err := client.New(url, nil).CreateItem(ctx, title)
	if err != nil {
		return err
	}
	if asJSON {
		_, err := fmt.Fprintln(out, string(raw))
		return err
	}
	var it client.Item
	if err := json.Unmarshal(raw, &it); err != nil {
		_, perr := fmt.Fprintln(out, string(raw))
		return perr
	}
	fmt.Fprintf(out, "#%d %s\n", it.ID, it.Title)
	return nil
}

func runItemDelete(ctx context.Context, url string, out io.Writer, id int64) error {
	if err := client.New(url, nil).DeleteItem(ctx, id); err != nil {
		return err
	}
	fmt.Fprintf(out, "deleted item %d\n", id)
	return nil
}

// parseInt64 wraps strconv.ParseInt with a descriptive error.
func parseInt64(s, label string) (int64, error) {
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q: %w", label, s, err)
	}
	return id, nil
}
