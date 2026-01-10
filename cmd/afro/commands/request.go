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
	// Determine filename logic similar to init/root
	// Ideally we just write back to whatever config was loaded.
	// viper.WriteConfig() works if a config file was discovered/loaded.
	// If it wasn't (e.g. no file yet, or config set programmatically), it might fail.
	// But init guarantees a file creation.
	// If we are using --bundle, viper should know the config file used.

	if err := viper.WriteConfig(); err != nil {
        // Fallback or explicit write?
        // If viper.ConfigFileUsed() is empty, we might need to default to afro.yaml or the bundle name.
        if viper.ConfigFileUsed() == "" {
             // Try to write to afro.yaml by default or the bundle name
             // But we need to know the bundle name if passed via flag?
             // It's cleaner to assume viper knows the file if it was loaded.
             // If not loaded (first run without init?), we might need to handle it.
             // But let's assume valid environment.
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
			// It's a file (legacy implicit check)
			// TODO: Consider removing implicit check in future versions
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

	if _, err := io.Copy(os.Stdout, resp.Body); err != nil {
		return fmt.Errorf("failed to read body: %w", err)
	}
	// Ensure newline at the end if needed? curl doesn't usually add one unless requested or formatted.
	// fmt.Println(string(body)) added one. io.Copy won't.
	// Let's add a newline for friendliness in CLI unless we want raw output.
	// Typical behavior is raw output.
	fmt.Println()

	return nil
}
