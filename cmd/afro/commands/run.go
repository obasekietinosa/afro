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

	"math/rand"
	"strconv"
	"time"

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

type Assertion struct {
	Left  string `mapstructure:"left"`
	Op    string `mapstructure:"op"`
	Right string `mapstructure:"right"`
}

type ChainStep struct {
	Request   string              `mapstructure:"request"`
	Extract   map[string]string   `mapstructure:"extract"`
	OnStatus  map[int][]ChainStep `mapstructure:"on_status"`
	Assert    []Assertion         `mapstructure:"assert"`
	Variables map[string]string   `mapstructure:"variables"`
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

		// Merge step-level variables (mapping/overrides)
		// We create a copy of variables for this step to avoid polluting the global scope if that's desired?
		// Actually, the user wants to map "token_b" to "token".
		// variables={"token": "{{token_b}}"}
		// So we need to process these.
		stepVars := make(map[string]interface{})
		for k, v := range variables {
			stepVars[k] = v
		}
		for k, v := range step.Variables {
			// Substitute values from global variables
			// e.g. v="{{token_b}}", we look up token_b in variables
			stepVars[k] = substitute(v, variables)
		}

		respStatusCode, err := runSavedRequest(ctx, step.Request, stepVars, outputWriter)
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

		// Assertions
		if len(step.Assert) > 0 {
			if err := executeAssertions(step.Assert, variables); err != nil {
				return fmt.Errorf("assertion failed in step '%s': %w", step.Request, err)
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

func executeAssertions(assertions []Assertion, vars map[string]interface{}) error {
	for i, a := range assertions {
		left := substitute(a.Left, vars)
		right := substitute(a.Right, vars)

		// Simple integer comparison if possible
		leftInt, errL := strconv.Atoi(left)
		rightInt, errR := strconv.Atoi(right)
		isNumeric := errL == nil && errR == nil

		pass := false
		switch a.Op {
		case "==":
			pass = left == right
		case "!=":
			pass = left != right
		case ">":
			if isNumeric {
				pass = leftInt > rightInt
			} else {
				pass = left > right
			}
		case ">=":
			if isNumeric {
				pass = leftInt >= rightInt
			} else {
				pass = left >= right
			}
		case "<":
			if isNumeric {
				pass = leftInt < rightInt
			} else {
				pass = left < right
			}
		case "<=":
			if isNumeric {
				pass = leftInt <= rightInt
			} else {
				pass = left <= right
			}
		default:
			return fmt.Errorf("unknown operator '%s'", a.Op)
		}

		if !pass {
			return fmt.Errorf("assertion %d failed: '%s' %s '%s'", i+1, left, a.Op, right)
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
	var headers []string
	if headerMap := viper.GetStringMapString(key + ".headers"); len(headerMap) > 0 {
		for k, v := range headerMap {
			headers = append(headers, fmt.Sprintf("%s: %s", k, v))
		}
	} else {
		headers = viper.GetStringSlice(key + ".headers")
	}
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
	// makeRequest has been updated to return status code.
	return makeRequest(ctx, opts, out)
}

func substitute(tmpl string, vars map[string]interface{}) string {
	// Built-in dynamic variables
	if strings.Contains(tmpl, "{{$timestamp}}") {
		tmpl = strings.ReplaceAll(tmpl, "{{$timestamp}}", fmt.Sprintf("%d", time.Now().Unix()))
	}
	if strings.Contains(tmpl, "{{$uuid}}") {
		// Simple random "UUID" - good enough for now
		uuid := fmt.Sprintf("%x", rand.Int63())
		tmpl = strings.ReplaceAll(tmpl, "{{$uuid}}", uuid)
	}

	for k, v := range vars {
		placeholder := fmt.Sprintf("{{%s}}", k)
		valStr := fmt.Sprintf("%v", v)
		tmpl = strings.ReplaceAll(tmpl, placeholder, valStr)
	}
	return tmpl
}

func substituteURL(tmpl string, vars map[string]interface{}) string {
	// Built-in dynamic variables
	if strings.Contains(tmpl, "{{$timestamp}}") {
		tmpl = strings.ReplaceAll(tmpl, "{{$timestamp}}", fmt.Sprintf("%d", time.Now().Unix()))
	}
	if strings.Contains(tmpl, "{{$uuid}}") {
		uuid := fmt.Sprintf("%x", rand.Int63())
		tmpl = strings.ReplaceAll(tmpl, "{{$uuid}}", uuid)
	}

	for k, v := range vars {
		placeholder := fmt.Sprintf("{{%s}}", k)
		valStr := fmt.Sprintf("%v", v)
		tmpl = strings.ReplaceAll(tmpl, placeholder, url.QueryEscape(valStr))
	}
	return tmpl
}
