package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var putCmd = &cobra.Command{
	Use:   "put [url]",
	Short: "Make a PUT request",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		opts := buildRequestOptions("PUT", args, cmd)
		if _, err := makeRequest(cmd.Context(), opts, os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(putCmd)
	addRequestFlags(putCmd)
}
