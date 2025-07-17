package cmd

import (
	"github.com/jmelahman/runtainer/internal/image"
	"github.com/jmelahman/runtainer/internal/runtime"

	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run <image-ref> -- <cmd> [args...]",
	Short: "Run a command inside a container image",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		ref := args[0]
		cmdArgs := args[1:]

		imageID, err := image.PullImage(ref)
		if err != nil {
			return err
		}

		return runtime.RunCommand(imageID, cmdArgs)
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
}
