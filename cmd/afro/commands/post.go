package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var postCmd = &cobra.Command{
	Use:   "post [url]",
	Short: "Make a POST request",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		opts := buildRequestOptions("POST", args, cmd)
		if err := makeRequest(cmd.Context(), opts); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(postCmd)
    addRequestFlags(postCmd)
}
