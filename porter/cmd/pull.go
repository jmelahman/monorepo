package cmd

import (
	"fmt"
	"github.com/jmelahman/porter/internal/image"

	"github.com/spf13/cobra"
)

var pullCmd = &cobra.Command{
	Use:   "pull <image-ref>",
	Short: "Pull an OCI image from a remote registry",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ref := args[0]
		imageID, err := image.PullImage(ref)
		if err != nil {
			return err
		}
		fmt.Println("Image pulled with ID:", imageID)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(pullCmd)
}
