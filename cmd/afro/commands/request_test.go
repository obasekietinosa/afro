package commands

import (
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
		// Basic Auth check
		user, pass, ok := r.BasicAuth()
		if !ok || user != "user" || pass != "pass" {
			t.Errorf("Expected Basic Auth user:pass, got %s:%s", user, pass)
		}
		fmt.Fprint(w, "OK")
	}))
	defer ts.Close()

	// Setup Viper config
	viper.Reset()
	viper.Set("base_url", ts.URL)
	viper.Set("auth.username", "user")
	viper.Set("auth.password", "pass")

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

	err := makeRequest(opts)

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

func TestMakeRequestNoAuth(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _, ok := r.BasicAuth()
		if ok {
			t.Error("Did not expect Basic Auth")
		}
		fmt.Fprint(w, "OK")
	}))
	defer ts.Close()

	viper.Reset()
	viper.Set("base_url", ts.URL)
	viper.Set("auth.username", "user")
	viper.Set("auth.password", "pass")

	opts := RequestOptions{
		Method: "GET",
		URL:    "/",
		NoAuth: true,
	}

    // Silence stdout
    oldStdout := os.Stdout
    r, w, _ := os.Pipe()
    os.Stdout = w
	err := makeRequest(opts)
    w.Close()
    os.Stdout = oldStdout
    _, _ = r.Read(make([]byte, 100))

	if err != nil {
		t.Errorf("makeRequest failed: %v", err)
	}
}
