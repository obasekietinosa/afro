package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var patchCmd = &cobra.Command{
	Use:   "patch [url]",
	Short: "Make a PATCH request",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		opts := buildRequestOptions("PATCH", args, cmd)
		if err := makeRequest(cmd.Context(), opts); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(patchCmd)
    addRequestFlags(patchCmd)
}
