package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var getCmd = &cobra.Command{
	Use:   "get [url]",
	Short: "Make a GET request",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		opts := buildRequestOptions("GET", args, cmd)
		if err := makeRequest(opts); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(getCmd)
    addRequestFlags(getCmd)
}

func buildRequestOptions(method string, args []string, cmd *cobra.Command) RequestOptions {
    body, _ := cmd.Flags().GetString("body")
    headers, _ := cmd.Flags().GetStringSlice("header")
    noAuth, _ := cmd.Flags().GetBool("no-auth")
    noHeaders, _ := cmd.Flags().GetBool("no-headers")
    saveName, _ := cmd.Flags().GetString("save")

    return RequestOptions{
        Method:    method,
        URL:       args[0],
        Body:      body,
        Headers:   headers,
        NoAuth:    noAuth,
        NoHeaders: noHeaders,
        SaveName:  saveName,
    }
}
