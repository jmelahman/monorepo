package cmd

import (
	"fmt"
	"github.com/jmelahman/porter/internal/image"
	"github.com/spf13/cobra"
)

var extractCmd = &cobra.Command{
	Use:   "extract <image-tar>",
	Short: "Extract an OCI image tarball",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		tarPath := args[0]
		imageID, err := image.ExtractImage(tarPath)
		if err != nil {
			return err
		}
		fmt.Println("Image extracted with ID:", imageID)
		return nil
	},
}
