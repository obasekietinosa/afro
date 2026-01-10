package commands

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func addRequestFlags(cmd *cobra.Command) {
	cmd.Flags().StringP("body", "b", "", "Request body (string or file path)")
	cmd.Flags().StringSliceP("header", "H", []string{}, "Request headers (e.g. \"Content-Type: application/json\")")
	cmd.Flags().Bool("no-auth", false, "Do not use configured authentication")
	cmd.Flags().Bool("no-headers", false, "Do not use configured common headers")
}

// RequestOptions holds the options for making a request
type RequestOptions struct {
	Method    string
	URL       string
	Body      string
	Headers   []string
	NoAuth    bool
	NoHeaders bool
}

func makeRequest(opts RequestOptions) error {
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
		// If no base URL and no scheme, assume http:// for now or error?
		// For now let's assume if the user provides a raw domain, they meant http
		// But strictly speaking, we should probably require scheme if no base_url
		// The requirement says "If you pass in a relative path... prepend the base URL".
		// If no base url is configured, maybe we should just try using it as is, but it will fail if no scheme.
		// Let's prepend https:// if it looks like a domain, or just error.
		// For now, let's just proceed and let http.NewRequest fail or not.
		if !strings.Contains(url, "://") {
             // Maybe error out if no base url?
             // "An example GET request would be afro get https://api.etin.dev"
             // "If you pass in a relative path, ie without a scheme, then Afro will automatically prepend the base URL"
        }
	}

	var reqBody io.Reader
	if opts.Body != "" {
		if _, err := os.Stat(opts.Body); err == nil {
			// It's a file
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

	req, err := http.NewRequest(opts.Method, url, reqBody)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Add Headers
	// 1. Common headers from config (unless --no-headers)
	if !opts.NoHeaders {
		headers := viper.GetStringMapString("headers")
		for k, v := range headers {
			req.Header.Add(k, v)
		}
	}

	// 2. Auth headers from config (unless --no-auth)
    // Supports Basic Auth for now
	if !opts.NoAuth {
		username := viper.GetString("auth.username")
		password := viper.GetString("auth.password")
		if username != "" {
			req.SetBasicAuth(username, password)
		}
	}

	// 3. CLI Headers (precedence)
	for _, h := range opts.Headers {
		parts := strings.SplitN(h, ":", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			// If header exists, we might want to overwrite or append.
            // README says "If the same header is set on the bundle and in the request, the request takes precedence."
            // So Set is better than Add for precedence if it exists.
			req.Header.Set(key, val)
		}
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

    // Output status
    // fmt.Printf("%s %s\n", resp.Proto, resp.Status)

    // Output Headers? Maybe verbose mode?
    // For now just output body as that's typical for curl-like tools, maybe status code too.
    // The README doesn't explicitly say what to output, but usually it's the response body.

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read body: %w", err)
	}

	fmt.Println(string(body))
	return nil
}
