package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/jmelahman/kanban/internal/client"
	"github.com/jmelahman/kanban/internal/kanbantoml"
)

// configViewEntry mirrors one entry of the GET /api/config response.
type configViewEntry struct {
	Key      string `json:"key"`
	Value    any    `json:"value"`
	Source   string `json:"source"`
	Writable bool   `json:"writable"`
}

type configView struct {
	Scope   string            `json:"scope"`
	Board   string            `json:"board"`
	Entries []configViewEntry `json:"entries"`
}

// ---------- config ----------

func configCmd() *cobra.Command {
	var serverURL string
	parent := &cobra.Command{
		Use:   "config",
		Short: "Read and write kanban configuration",
		Long: "Manage the layered kanban config. --global targets the user config " +
			"(~/.config/kanban/config.toml); --local targets a board's project " +
			".kanban.toml (selected with --board). Keys are dotted, e.g. " +
			"sync.allow_rebase or github.draft_column.",
	}
	addServerFlag(parent, &serverURL)

	var (
		clLocal, clGlobal, clJSON bool
		clBoard                   string
	)
	list := &cobra.Command{
		Use:   "list",
		Short: "List config keys with their values and source",
		RunE: func(cmd *cobra.Command, args []string) error {
			scope, err := resolveConfigScope(clLocal, clGlobal, false)
			if err != nil {
				return err
			}
			return runConfigList(cmd.Context(), resolveURL(cmd, serverURL), cmd.OutOrStdout(), scope, clBoard, clJSON)
		},
	}
	list.Flags().BoolVar(&clLocal, "local", false, "Only the board's project .kanban.toml (needs --board)")
	list.Flags().BoolVar(&clGlobal, "global", false, "Only the user config (~/.config/kanban/config.toml)")
	list.Flags().StringVar(&clBoard, "board", "", "Board id or slug for the local layer")
	list.Flags().BoolVar(&clJSON, "json", false, "Print the raw JSON response")

	var (
		cgLocal, cgGlobal, cgJSON bool
		cgBoard                   string
	)
	get := &cobra.Command{
		Use:   "get <key>",
		Short: "Print the value of one config key",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			scope, err := resolveConfigScope(cgLocal, cgGlobal, false)
			if err != nil {
				return err
			}
			return runConfigGet(cmd.Context(), resolveURL(cmd, serverURL), cmd.OutOrStdout(), args[0], scope, cgBoard, cgJSON)
		},
	}
	get.Flags().BoolVar(&cgLocal, "local", false, "Read from the board's project .kanban.toml (needs --board)")
	get.Flags().BoolVar(&cgGlobal, "global", false, "Read from the user config")
	get.Flags().StringVar(&cgBoard, "board", "", "Board id or slug for the local layer")
	get.Flags().BoolVar(&cgJSON, "json", false, "Print the full entry JSON instead of the bare value")

	var (
		csLocal, csGlobal bool
		csBoard           string
	)
	set := &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a config key (collections take a JSON value)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			scope, err := resolveConfigScope(csLocal, csGlobal, true)
			if err != nil {
				return err
			}
			return runConfigSet(cmd.Context(), resolveURL(cmd, serverURL), cmd.OutOrStdout(), args[0], args[1], scope, csBoard)
		},
	}
	set.Flags().BoolVar(&csLocal, "local", false, "Write to the board's project .kanban.toml (needs --board)")
	set.Flags().BoolVar(&csGlobal, "global", false, "Write to the user config")
	set.Flags().StringVar(&csBoard, "board", "", "Board id or slug for the local layer")

	var (
		cuLocal, cuGlobal bool
		cuBoard           string
	)
	unset := &cobra.Command{
		Use:   "unset <key>",
		Short: "Remove a config key",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			scope, err := resolveConfigScope(cuLocal, cuGlobal, true)
			if err != nil {
				return err
			}
			return runConfigUnset(cmd.Context(), resolveURL(cmd, serverURL), cmd.OutOrStdout(), args[0], scope, cuBoard)
		},
	}
	unset.Flags().BoolVar(&cuLocal, "local", false, "Remove from the board's project .kanban.toml (needs --board)")
	unset.Flags().BoolVar(&cuGlobal, "global", false, "Remove from the user config")
	unset.Flags().StringVar(&cuBoard, "board", "", "Board id or slug for the local layer")

	parent.AddCommand(list, get, set, unset)
	return parent
}

// resolveConfigScope maps the --local/--global flags to a scope. Reads default
// to the merged effective view; writes require an explicit scope.
func resolveConfigScope(local, global, forWrite bool) (string, error) {
	if local && global {
		return "", fmt.Errorf("--local and --global are mutually exclusive")
	}
	if local {
		return "local", nil
	}
	if global {
		return "global", nil
	}
	if forWrite {
		return "", fmt.Errorf("specify --global or --local (with --board)")
	}
	return "effective", nil
}

func runConfigList(ctx context.Context, url string, out io.Writer, scope, board string, asJSON bool) error {
	raw, err := client.New(url, nil).Config(ctx, scope, board)
	if err != nil {
		return err
	}
	if asJSON {
		_, err := out.Write(raw)
		return err
	}
	var view configView
	if err := json.Unmarshal(raw, &view); err != nil {
		return err
	}
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "KEY\tVALUE\tSOURCE")
	for _, e := range view.Entries {
		fmt.Fprintf(tw, "%s\t%s\t%s\n", e.Key, renderConfigValue(e.Value), e.Source)
	}
	return tw.Flush()
}

func runConfigGet(ctx context.Context, url string, out io.Writer, key, scope, board string, asJSON bool) error {
	raw, err := client.New(url, nil).Config(ctx, scope, board)
	if err != nil {
		return err
	}
	var view configView
	if err := json.Unmarshal(raw, &view); err != nil {
		return err
	}
	value, found := findConfigValue(view.Entries, key)
	if !found {
		return fmt.Errorf("no value set for %q in %s scope", key, view.Scope)
	}
	if asJSON {
		b, err := json.Marshal(map[string]any{"key": key, "value": value})
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(out, string(b))
		return err
	}
	_, err = fmt.Fprintln(out, renderConfigValue(value))
	return err
}

func runConfigSet(ctx context.Context, url string, out io.Writer, key, value, scope, board string) error {
	v, err := kanbantoml.CoerceCLIValue(key, value)
	if err != nil {
		return err
	}
	if _, err := client.New(url, nil).ConfigPatch(ctx, client.ConfigPatchArgs{
		Scope: scope,
		Board: board,
		Set:   map[string]any{key: v},
	}); err != nil {
		return err
	}
	fmt.Fprintf(out, "set %s (%s)\n", key, scope)
	return nil
}

func runConfigUnset(ctx context.Context, url string, out io.Writer, key, scope, board string) error {
	if _, _, ok := kanbantoml.Lookup(key); !ok {
		return fmt.Errorf("unknown config key %q", key)
	}
	if _, err := client.New(url, nil).ConfigPatch(ctx, client.ConfigPatchArgs{
		Scope: scope,
		Board: board,
		Unset: []string{key},
	}); err != nil {
		return err
	}
	fmt.Fprintf(out, "unset %s (%s)\n", key, scope)
	return nil
}

// findConfigValue locates a key in a config view, resolving a single
// StringMap entry address (e.g. devcontainer.container_env.FOO) by indexing
// into the parent map.
func findConfigValue(entries []configViewEntry, key string) (any, bool) {
	for _, e := range entries {
		if e.Key == key {
			return e.Value, true
		}
	}
	if spec, sub, ok := kanbantoml.Lookup(key); ok && sub != "" {
		for _, e := range entries {
			if e.Key == spec.Key {
				if m, ok := e.Value.(map[string]any); ok {
					if v, ok := m[sub]; ok {
						return v, true
					}
				}
				return nil, false
			}
		}
	}
	return nil, false
}

// renderConfigValue formats a config value for the human-readable table /
// `get` output. Scalars print bare; arrays and objects print as compact JSON.
func renderConfigValue(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case bool:
		return strconv.FormatBool(t)
	case float64:
		return strconv.FormatFloat(t, 'g', -1, 64)
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(b)
	}
}
