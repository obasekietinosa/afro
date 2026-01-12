package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/oliveagle/jsonpath"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func addRequestFlags(cmd *cobra.Command) {
	cmd.Flags().StringP("body", "b", "", "Request body (string or file path)")
	cmd.Flags().StringSliceP("header", "H", []string{}, "Request headers (e.g. \"Content-Type: application/json\")")
	cmd.Flags().Bool("no-auth", false, "Do not use configured authentication")
	cmd.Flags().Bool("no-headers", false, "Do not use configured common headers")
	cmd.Flags().String("save", "", "Save the request with the given name")
	cmd.Flags().String("extract-cookie", "", "Extract a cookie from the response and save it to the auth configuration")
	cmd.Flags().StringToString("extract-to-config", nil, "Extract JSON fields to config (key=jsonpath)")
}

// RequestOptions holds the options for making a request
type RequestOptions struct {
	Method          string
	URL             string
	Body            string
	Headers         []string
	NoAuth          bool
	NoHeaders       bool
	SaveName        string
	ExtractCookie   string
	ExtractToConfig map[string]string
}

func buildRequestOptions(method string, args []string, cmd *cobra.Command) RequestOptions {
	body, _ := cmd.Flags().GetString("body")
	headers, _ := cmd.Flags().GetStringSlice("header")
	noAuth, _ := cmd.Flags().GetBool("no-auth")
	noHeaders, _ := cmd.Flags().GetBool("no-headers")
	saveName, _ := cmd.Flags().GetString("save")
	extractCookie, _ := cmd.Flags().GetString("extract-cookie")
	extractToConfig, _ := cmd.Flags().GetStringToString("extract-to-config")

	return RequestOptions{
		Method:          method,
		URL:             args[0],
		Body:            body,
		Headers:         headers,
		NoAuth:          noAuth,
		NoHeaders:       noHeaders,
		SaveName:        saveName,
		ExtractCookie:   extractCookie,
		ExtractToConfig: extractToConfig,
	}
}

func saveRequest(opts RequestOptions, name string) {
	// Structure: requests.<name>
	key := fmt.Sprintf("requests.%s", name)
	viper.Set(key+".method", opts.Method)
	viper.Set(key+".url", opts.URL)
	if opts.Body != "" {
		viper.Set(key+".body", opts.Body)
	}
	if len(opts.Headers) > 0 {
		viper.Set(key+".headers", opts.Headers)
	}
	viper.Set(key+".no_auth", opts.NoAuth)
	viper.Set(key+".no_headers", opts.NoHeaders)
	if opts.ExtractCookie != "" {
		viper.Set(key+".extract_cookie", opts.ExtractCookie)
	}
	if len(opts.ExtractToConfig) > 0 {
		viper.Set(key+".extract_to_config", opts.ExtractToConfig)
	}

	// Save the config
	if err := viper.WriteConfig(); err != nil {
		// Fallback or explicit write?
		if viper.ConfigFileUsed() == "" {
			err = viper.WriteConfigAs("afro.yaml")
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to save request: %v\n", err)
		} else {
			fmt.Printf("Request saved as '%s' to afro.yaml\n", name)
		}
	} else {
		fmt.Printf("Request saved as '%s' to %s\n", name, viper.ConfigFileUsed())
	}
}

func makeRequest(ctx context.Context, opts RequestOptions, out io.Writer) error {
	// Determine URL
	url := opts.URL
	baseURL := viper.GetString("base_url")

	// If URL does not start with http(s), prepend base URL
	if !strings.HasPrefix(url, "http") && baseURL != "" {
		if !strings.HasSuffix(baseURL, "/") && !strings.HasPrefix(url, "/") {
			url = baseURL + "/" + url
		} else if strings.HasSuffix(baseURL, "/") && strings.HasPrefix(url, "/") {
			url = baseURL + strings.TrimPrefix(url, "/")
		} else {
			url = baseURL + url
		}
	} else if !strings.HasPrefix(url, "http") {
	}

	var reqBody io.Reader
	if opts.Body != "" {
		if strings.HasPrefix(opts.Body, "@") {
			// Explicit file path
			filePath := strings.TrimPrefix(opts.Body, "@")
			f, err := os.Open(filePath)
			if err != nil {
				return fmt.Errorf("failed to open body file: %w", err)
			}
			defer f.Close()
			reqBody = f
		} else if _, err := os.Stat(opts.Body); err == nil {
			f, err := os.Open(opts.Body)
			if err != nil {
				return fmt.Errorf("failed to open body file: %w", err)
			}
			defer f.Close()
			reqBody = f
		} else {
			// It's a string
			reqBody = strings.NewReader(opts.Body)
		}
	}

	req, err := http.NewRequestWithContext(ctx, opts.Method, url, reqBody)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Save request if requested
	if opts.SaveName != "" {
		saveRequest(opts, opts.SaveName)
	}

	// Add Headers
	if !opts.NoHeaders {
		headers := viper.GetStringMapString("headers")
		for k, v := range headers {
			req.Header.Add(k, v)
		}
	}

	if !opts.NoAuth {
		username := viper.GetString("auth.username")
		password := viper.GetString("auth.password")
		if username != "" {
			req.SetBasicAuth(username, password)
		}
		cookie := viper.GetString("auth.cookie")
		if cookie != "" {
			req.Header.Set("Cookie", cookie)
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
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if out == nil {
		out = os.Stdout
	}

	var bodyReader io.Reader
	var captureBuf bytes.Buffer

	if len(opts.ExtractToConfig) > 0 {
		bodyReader = io.TeeReader(resp.Body, &captureBuf)
	} else {
		bodyReader = resp.Body
	}

	if _, err := io.Copy(out, bodyReader); err != nil {
		return fmt.Errorf("failed to read body: %w", err)
	}
	// Ensure newline at the end for friendliness in CLI if writing to stdout
	if out == os.Stdout {
		fmt.Println()
	}

	if len(opts.ExtractToConfig) > 0 {
		// Parse JSON
		var jsonData interface{}
		if err := json.Unmarshal(captureBuf.Bytes(), &jsonData); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to parse response as JSON for extraction: %v\n", err)
		} else {
			updated := false
			for configKey, path := range opts.ExtractToConfig {
				res, err := jsonpath.JsonPathLookup(jsonData, path)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to extract '%s' using path '%s': %v\n", configKey, path, err)
					continue
				}
				// Save to viper config
				// We support nested keys like "auth.token"
				viper.Set(configKey, res)
				updated = true
				fmt.Fprintf(os.Stderr, "Extracted '%s' to config key '%s'.\n", res, configKey)
			}
			if updated {
				if err := viper.WriteConfig(); err != nil {
					if viper.ConfigFileUsed() == "" {
						_ = viper.WriteConfigAs("afro.yaml")
					}
				}
			}
		}
	}

	if opts.ExtractCookie != "" {
		found := false
		for _, c := range resp.Cookies() {
			if c.Name == opts.ExtractCookie {
				// We want to save name=value
				val := fmt.Sprintf("%s=%s", c.Name, c.Value)
				viper.Set("auth.cookie", val)
				found = true
				if err := viper.WriteConfig(); err != nil {
					// Fallback
					if viper.ConfigFileUsed() == "" {
						_ = viper.WriteConfigAs("afro.yaml")
					}
				}
				fmt.Fprintf(os.Stderr, "Extracted cookie '%s' and saved to auth configuration.\n", c.Name)
				break
			}
		}
		if !found {
			fmt.Fprintf(os.Stderr, "Warning: cookie '%s' not found in response.\n", opts.ExtractCookie)
		}
	}

	return nil
}
