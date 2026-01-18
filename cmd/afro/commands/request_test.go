package commands

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/spf13/viper"
)

func TestMakeRequest(t *testing.T) {
	// Setup a test server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/test" {
			t.Errorf("Expected path /test, got %s", r.URL.Path)
		}
		if r.Header.Get("X-Custom") != "MyValue" {
			t.Errorf("Expected Header X-Custom: MyValue, got %s", r.Header.Get("X-Custom"))
		}

		fmt.Fprint(w, "OK")
	}))
	defer ts.Close()

	// Setup Viper config
	viper.Reset()
	viper.Set("base_url", ts.URL)

	// Test case
	opts := RequestOptions{
		Method:  "GET",
		URL:     "/test",
		Headers: []string{"X-Custom: MyValue"},
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	_, err := makeRequest(context.Background(), opts, os.Stdout)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Errorf("makeRequest failed: %v", err)
	}

	// Read captured output
	out := make([]byte, 100)
	n, _ := r.Read(out)
	output := string(out[:n])
	// The output should be the body "OK" plus a newline from fmt.Println
	if output != "OK\n" {
		t.Errorf("Expected output OK, got %q", output)
	}
}

func TestOtherVerbs(t *testing.T) {
	verbs := []string{"PUT", "PATCH", "DELETE", "OPTIONS", "HEAD"}

	for _, verb := range verbs {
		t.Run(verb, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != verb {
					t.Errorf("Expected method %s, got %s", verb, r.Method)
				}
				if verb != "HEAD" {
					fmt.Fprint(w, "OK")
				}
			}))
			defer ts.Close()

			viper.Reset()
			// We don't set base_url here to test absolute URL logic if we wanted,
			// but for simplicity let's just pass full URL to opts.

			opts := RequestOptions{
				Method: verb,
				URL:    ts.URL,
			}

			// Capture stdout
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			_, err := makeRequest(context.Background(), opts, os.Stdout)

			w.Close()
			os.Stdout = oldStdout

			if err != nil {
				t.Errorf("makeRequest failed for %s: %v", verb, err)
			}

			// Read captured output
			out := make([]byte, 100)
			n, _ := r.Read(out)
			output := string(out[:n])

			if verb != "HEAD" {
				if output != "OK\n" {
					t.Errorf("Expected output OK for %s, got %q", verb, output)
				}
			} else {
				// HEAD should not return a body, but our code adds a newline.
				if output != "\n" {
					t.Errorf("Expected only newline for HEAD, got %q", output)
				}
			}
		})
	}
}
