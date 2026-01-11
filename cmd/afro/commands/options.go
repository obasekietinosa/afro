package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var optionsCmd = &cobra.Command{
	Use:   "options [url]",
	Short: "Make a OPTIONS request",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		opts := buildRequestOptions("OPTIONS", args, cmd)
		if err := makeRequest(cmd.Context(), opts); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(optionsCmd)
    addRequestFlags(optionsCmd)
}
