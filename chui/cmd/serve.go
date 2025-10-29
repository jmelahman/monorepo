package cmd

import (
	"fmt"
	"net/http"

	"github.com/spf13/cobra"
)

func NewServeCommand() *cobra.Command {
	var port int

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the chat server",
		Long:  "Start a server to handle chat connections",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Starting server on port %d...\n", port)

			http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
				_ = fmt.Fprintf(w, "chui server is running!")
			})

			fmt.Printf("Server listening on :%d\n", port)
			return http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
		},
	}

	cmd.Flags().IntVarP(&port, "port", "p", 8080, "Port to run the server on")

	return cmd
}
