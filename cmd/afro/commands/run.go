package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
			if err := runChain(cmd.Context(), name); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		} else {
			if _, err := runSavedRequest(cmd.Context(), name, nil, nil); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
}

type ChainStep struct {
	Request  string              `mapstructure:"request"`
	Extract  map[string]string   `mapstructure:"extract"`
	OnStatus map[int][]ChainStep `mapstructure:"on_status"`
}

func runChain(ctx context.Context, name string) error {
	key := fmt.Sprintf("chains.%s", name)
	var steps []ChainStep
	if err := viper.UnmarshalKey(key, &steps); err != nil {
		return fmt.Errorf("failed to parse chain '%s': %w", name, err)
	}

	variables := make(map[string]interface{})
	if err := executeSteps(ctx, steps, variables); err != nil {
		return fmt.Errorf("chain execution failed: %w", err)
	}
	return nil
}

func executeSteps(ctx context.Context, steps []ChainStep, variables map[string]interface{}) error {
	for i, step := range steps {
		if step.Request == "" {
			return fmt.Errorf("step %d missing 'request' field", i+1)
		}

		fmt.Fprintf(os.Stderr, "Running step: %s\n", step.Request)

		// Capture output for extraction or branching analysis
		var captureBuf bytes.Buffer
		// We always capture to buffer to allow body inspection if needed (future proofing),
		// but specifically for extraction we need it.
		// Use MultiWriter to still show output to user.
		outputWriter := io.MultiWriter(os.Stdout, &captureBuf)

		respStatusCode, err := runSavedRequest(ctx, step.Request, variables, outputWriter)
		if err != nil {
			return fmt.Errorf("step '%s' failed: %w", step.Request, err)
		}

		// Extraction
		if len(step.Extract) > 0 {
			var jsonData interface{}
			if err := json.Unmarshal(captureBuf.Bytes(), &jsonData); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to parse response for extraction in step '%s': %v\n", step.Request, err)
			} else {
				for varName, path := range step.Extract {
					res, err := jsonpath.JsonPathLookup(jsonData, path)
					if err != nil {
						fmt.Fprintf(os.Stderr, "Warning: failed to extract '%s' using path '%s': %v\n", varName, path, err)
						continue
					}
					variables[varName] = res
				}
			}
		}

		// Branching
		if subSteps, ok := step.OnStatus[respStatusCode]; ok {
			fmt.Fprintf(os.Stderr, "Status %d matched, executing branch...\n", respStatusCode)
			if err := executeSteps(ctx, subSteps, variables); err != nil {
				return fmt.Errorf("branch execution failed: %w", err)
			}
		}
	}
	return nil
}

// runSavedRequest runs a request and returns the status code.
func runSavedRequest(ctx context.Context, name string, vars map[string]interface{}, out io.Writer) (int, error) {
	key := fmt.Sprintf("requests.%s", name)
	if !viper.IsSet(key) {
		return 0, fmt.Errorf("request '%s' not found in config", name)
	}

	method := viper.GetString(key + ".method")
	url := viper.GetString(key + ".url")
	body := viper.GetString(key + ".body")
	headers := viper.GetStringSlice(key + ".headers")
	noHeaders := viper.GetBool(key + ".no_headers")

	// Variable Substitution
	if vars != nil {
		url = substituteURL(url, vars)
		body = substitute(body, vars)
		for i, h := range headers {
			headers[i] = substitute(h, vars)
		}
	}

	opts := RequestOptions{
		Method:    method,
		URL:       url,
		Body:      body,
		Headers:   headers,
		NoHeaders: noHeaders,
	}

	// We need to capture the response to get the status code.
	// makeRequest currently doesn't return the response or status code.
	// I need to update makeRequest or duplicate logic slightly?
	// makeRequest is in request.go. I should update it to return status code?
	// Or I can just inline the request making logic here?
	// Better to update request.go to return *http.Response or status code.
	// But I cannot edit request.go in this tool call (limitations).
	// I will assume I can't edit request.go blindly without context.
	// Wait, I just edited request.go.
	// Implementation strategy:
	// Copy a variation of makeRequest logic here for now, OR update request.go in a subsequent step.
	// Since I need the status code for branching, and makeRequest doesn't return it,
	// I definitely need to modify makeRequest or use a local version.
	// Given the instructions, I should probably modify makeRequest.
	// I will implement a local 'makeRequestWithStatus' for now to fit in this file replacement.

	return makeRequestWithStatus(ctx, opts, out)
}

func makeRequestWithStatus(ctx context.Context, opts RequestOptions, out io.Writer) (int, error) {
	// ... Copy of makeRequest logic but returning status ...
	// Since I can't import private things from the same package easily if they aren't exported... everything is in `commands` package.
	// makeRequest is in the same package.

	// Determine URL
	urlVal := opts.URL
	baseURL := viper.GetString("base_url")

	if !strings.HasPrefix(urlVal, "http") && baseURL != "" {
		if !strings.HasSuffix(baseURL, "/") && !strings.HasPrefix(urlVal, "/") {
			urlVal = baseURL + "/" + urlVal
		} else if strings.HasSuffix(baseURL, "/") && strings.HasPrefix(urlVal, "/") {
			urlVal = baseURL + strings.TrimPrefix(urlVal, "/")
		} else {
			urlVal = baseURL + urlVal
		}
	}

	var reqBody io.Reader
	if opts.Body != "" {
		if strings.HasPrefix(opts.Body, "@") {
			filePath := strings.TrimPrefix(opts.Body, "@")
			f, err := os.Open(filePath)
			if err != nil {
				return 0, fmt.Errorf("failed to open body file: %w", err)
			}
			defer f.Close()
			reqBody = f
		} else if _, err := os.Stat(opts.Body); err == nil {
			f, err := os.Open(opts.Body)
			if err != nil {
				return 0, fmt.Errorf("failed to open body file: %w", err)
			}
			defer f.Close()
			reqBody = f
		} else {
			reqBody = strings.NewReader(opts.Body)
		}
	}

	req, err := http.NewRequestWithContext(ctx, opts.Method, urlVal, reqBody)
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}

	if !opts.NoHeaders {
		headers := viper.GetStringMapString("headers")
		for k, v := range headers {
			req.Header.Add(k, v)
		}
	}

	for _, h := range opts.Headers {
		parts := strings.SplitN(h, ":", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			req.Header.Set(key, val)
		}
	}

	client := &http.Client{}
	// Don't follow redirects automatically? Standard client follows.

	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if out == nil {
		out = os.Stdout
	}
	if _, err := io.Copy(out, resp.Body); err != nil {
		return resp.StatusCode, fmt.Errorf("failed to read body: %w", err)
	}
	if out == os.Stdout {
		fmt.Println()
	}

	return resp.StatusCode, nil
}

func substitute(tmpl string, vars map[string]interface{}) string {
	for k, v := range vars {
		placeholder := fmt.Sprintf("{{%s}}", k)
		valStr := fmt.Sprintf("%v", v)
		tmpl = strings.ReplaceAll(tmpl, placeholder, valStr)
	}
	return tmpl
}

func substituteURL(tmpl string, vars map[string]interface{}) string {
	for k, v := range vars {
		placeholder := fmt.Sprintf("{{%s}}", k)
		valStr := fmt.Sprintf("%v", v)
		tmpl = strings.ReplaceAll(tmpl, placeholder, url.QueryEscape(valStr))
	}
	return tmpl
}
