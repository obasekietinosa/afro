package commands

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"

	"github.com/spf13/viper"
)

func TestChainBranching(t *testing.T) {
	var loginCalled int32
	var dataCalled int32

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login" {
			atomic.AddInt32(&loginCalled, 1)
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"token": "secret-token"}`)
			return
		}
		if r.URL.Path == "/data" {
			atomic.AddInt32(&dataCalled, 1)
			token := r.Header.Get("Authorization")
			if token == "Bearer secret-token" {
				w.WriteHeader(http.StatusOK)
				fmt.Fprint(w, "success")
			} else {
				w.WriteHeader(http.StatusUnauthorized)
				fmt.Fprint(w, "unauthorized")
			}
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	viper.Reset()
	viper.Set("base_url", ts.URL)

	// Define requests
	viper.Set("requests.get_data.method", "GET")
	viper.Set("requests.get_data.url", "/data")
	viper.Set("requests.get_data.headers", []string{"Authorization: Bearer {{token}}"})

	viper.Set("requests.login.method", "POST")
	viper.Set("requests.login.url", "/login")

	// Define Chain
	steps := []map[string]interface{}{
		{
			"request": "get_data",
			"on_status": map[int]interface{}{
				401: []map[string]interface{}{
					{
						"request": "login",
						"extract": map[string]string{
							"token": "$.token",
						},
					},
					{
						"request": "get_data",
					},
				},
			},
		},
	}

	viper.Set("chains.test_flow", steps)

	// Capture output to avoid noise
	oldStderr := os.Stderr
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stderr = w
	os.Stdout = w

	// Run
	if err := runChain(context.Background(), "test_flow"); err != nil {
		t.Errorf("runChain failed: %v", err)
	}

	// Restore stdout/stderr
	w.Close()
	os.Stderr = oldStderr
	os.Stdout = oldStdout
	r.Close()

	// Verify
	if atomic.LoadInt32(&loginCalled) != 1 {
		t.Errorf("Expected login to be called once, got %d", loginCalled)
	}
	if atomic.LoadInt32(&dataCalled) != 2 { // Once failed (401), once success (200)
		t.Errorf("Expected data to be called twice, got %d", dataCalled)
	}
}
