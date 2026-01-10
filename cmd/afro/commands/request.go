package commands

import (
	"context"
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
	cmd.Flags().String("save", "", "Save the request with the given name")
}

// RequestOptions holds the options for making a request
type RequestOptions struct {
	Method    string
	URL       string
	Body      string
	Headers   []string
	NoAuth    bool
	NoHeaders bool
	SaveName  string
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

func makeRequest(ctx context.Context, opts RequestOptions) error {
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

	if _, err := io.Copy(os.Stdout, resp.Body); err != nil {
		return fmt.Errorf("failed to read body: %w", err)
	}
	// Ensure newline at the end for friendliness in CLI
	fmt.Println()

	return nil
}
