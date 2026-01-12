package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	"github.com/oliveagle/jsonpath"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var runCmd = &cobra.Command{
	Use:   "run [request-name]",
	Short: "Run a saved request or a chain",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		if viper.IsSet("chains." + name) {
			runChain(cmd.Context(), name)
		} else {
			if err := runSavedRequest(cmd.Context(), name, nil, nil); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
}

func runChain(ctx context.Context, name string) {
	key := fmt.Sprintf("chains.%s", name)
	// We expect the chain to be a list of objects
	var steps []map[string]interface{}
	if err := viper.UnmarshalKey(key, &steps); err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to parse chain '%s': %v\n", name, err)
		os.Exit(1)
	}

	variables := make(map[string]interface{})

	for i, step := range steps {
		reqName, ok := step["request"].(string)
		if !ok {
			fmt.Fprintf(os.Stderr, "Error: step %d in chain '%s' missing 'request' field\n", i+1, name)
			os.Exit(1)
		}

		// Run request, capturing output
		fmt.Fprintf(os.Stderr, "Running step %d: %s\n", i+1, reqName)

		extract, hasExtract := step["extract"].(map[string]interface{})

		var outputWriter io.Writer
		var captureBuf bytes.Buffer

		// If we are extracting, we need to capture the output.
		// We also want to print to stdout usually? Or maybe silence it if it's an intermediate step?
		// The requirement doesn't specify. I'll print to stdout as well.
		if hasExtract {
			outputWriter = io.MultiWriter(os.Stdout, &captureBuf)
		} else {
			outputWriter = os.Stdout
		}

		if err := runSavedRequest(ctx, reqName, variables, outputWriter); err != nil {
			fmt.Fprintf(os.Stderr, "Error running step '%s': %v\n", reqName, err)
			os.Exit(1)
		}

		if hasExtract {
			// Parse JSON
			var jsonData interface{}
			if err := json.Unmarshal(captureBuf.Bytes(), &jsonData); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to parse response as JSON for extraction in step '%s': %v\n", reqName, err)
				continue
			}

			for varName, path := range extract {
				pathStr, ok := path.(string)
				if !ok {
					continue
				}
				res, err := jsonpath.JsonPathLookup(jsonData, pathStr)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to extract '%s' using path '%s': %v\n", varName, pathStr, err)
					continue
				}
				variables[varName] = res
				// Debug log
				// fmt.Fprintf(os.Stderr, "Extracted %s = %v\n", varName, res)
			}
		}
	}
}

// runSavedRequest runs a request. If variables are provided, they are substituted.
// If out is nil, it defaults to os.Stdout (inside makeRequest)
func runSavedRequest(ctx context.Context, name string, vars map[string]interface{}, out io.Writer) error {
	key := fmt.Sprintf("requests.%s", name)
	if !viper.IsSet(key) {
		return fmt.Errorf("request '%s' not found in config", name)
	}

	method := viper.GetString(key + ".method")
	url := viper.GetString(key + ".url")
	body := viper.GetString(key + ".body")
	headers := viper.GetStringSlice(key + ".headers")
	noAuth := viper.GetBool(key + ".no_auth")
	noHeaders := viper.GetBool(key + ".no_headers")
	extractCookie := viper.GetString(key + ".extract_cookie")

	// Variable Substitution
	if vars != nil {
		url = substituteURL(url, vars)
		body = substitute(body, vars)
		for i, h := range headers {
			headers[i] = substitute(h, vars)
		}
	}

	opts := RequestOptions{
		Method:        method,
		URL:           url,
		Body:          body,
		Headers:       headers,
		NoAuth:        noAuth,
		NoHeaders:     noHeaders,
		ExtractCookie: extractCookie,
	}

	return makeRequest(ctx, opts, out)
}

func substitute(tmpl string, vars map[string]interface{}) string {
	// Replace {{var}} with value
	for k, v := range vars {
		placeholder := fmt.Sprintf("{{%s}}", k)
		valStr := fmt.Sprintf("%v", v)
		tmpl = strings.ReplaceAll(tmpl, placeholder, valStr)
	}
	return tmpl
}

func substituteURL(tmpl string, vars map[string]interface{}) string {
	// Replace {{var}} with URL encoded value
	for k, v := range vars {
		placeholder := fmt.Sprintf("{{%s}}", k)
		valStr := fmt.Sprintf("%v", v)
		tmpl = strings.ReplaceAll(tmpl, placeholder, url.QueryEscape(valStr))
	}
	return tmpl
}
