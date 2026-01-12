package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var headCmd = &cobra.Command{
	Use:   "head [url]",
	Short: "Make a HEAD request",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		opts := buildRequestOptions("HEAD", args, cmd)
		if err := makeRequest(cmd.Context(), opts, os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(headCmd)
    addRequestFlags(headCmd)
}
