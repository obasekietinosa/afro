package commands

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var runCmd = &cobra.Command{
	Use:   "run [request-name]",
	Short: "Run a saved request",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		runSavedRequest(cmd.Context(), name)
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
}

func runSavedRequest(ctx context.Context, name string) {
    key := fmt.Sprintf("requests.%s", name)
    if !viper.IsSet(key) {
        fmt.Fprintf(os.Stderr, "Error: request '%s' not found in config\n", name)
        os.Exit(1)
    }

    method := viper.GetString(key + ".method")
    url := viper.GetString(key + ".url")
    body := viper.GetString(key + ".body")
    headers := viper.GetStringSlice(key + ".headers")
    noAuth := viper.GetBool(key + ".no_auth")
    noHeaders := viper.GetBool(key + ".no_headers")

    opts := RequestOptions{
        Method:    method,
        URL:       url,
        Body:      body,
        Headers:   headers,
        NoAuth:    noAuth,
        NoHeaders: noHeaders,
        // Don't save again when running
    }

    if err := makeRequest(ctx, opts); err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
}
