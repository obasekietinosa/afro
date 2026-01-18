package commands

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestDynamicVariables(t *testing.T) {
	var capturedEmail string
	var capturedBody string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedEmail = r.URL.Query().Get("email")
		// Read body to check substitution
		buf := make([]byte, 100)
		n, _ := r.Body.Read(buf)
		capturedBody = string(buf[:n])
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	viper.Reset()
	viper.Set("base_url", ts.URL)
	viper.Set("requests.dynamic_req.method", "POST")
	viper.Set("requests.dynamic_req.url", "/test?email=user_{{$timestamp}}@test.com")
	viper.Set("requests.dynamic_req.body", `{"id": "{{$uuid}}"}`)

	steps := []map[string]interface{}{
		{"request": "dynamic_req"},
	}
	viper.Set("chains.dynamic_flow", steps)

	// Capture output
	oldStderr := os.Stderr
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stderr = w
	os.Stdout = w

	err := runChain(context.Background(), "dynamic_flow")

	w.Close()
	os.Stderr = oldStderr
	os.Stdout = oldStdout
	r.Close()

	if err != nil {
		t.Errorf("runChain failed: %v", err)
	}

	if !strings.Contains(capturedEmail, "user_") || !strings.Contains(capturedEmail, "@test.com") {
		t.Errorf("Email '%s' does not match expected pattern", capturedEmail)
	}
	// Check if timestamp was replaced (it shouldn't be {{$timestamp}})
	if strings.Contains(capturedEmail, "{{$timestamp}}") {
		t.Errorf("Timestamp variable not substituted in URL")
	}

	if strings.Contains(capturedBody, "{{$uuid}}") {
		t.Errorf("UUID variable not substituted in Body: %s", capturedBody)
	}
}

func TestAssertions(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"count": 5, "status": "active"}`)
	}))
	defer ts.Close()

	viper.Reset()
	viper.Set("base_url", ts.URL)
	viper.Set("requests.check_status.method", "GET")
	viper.Set("requests.check_status.url", "/")

	// Test Passing Assertion
	stepsPass := []map[string]interface{}{
		{
			"request": "check_status",
			"extract": map[string]string{
				"count":  "$.count",
				"status": "$.status",
			},
			"assert": []map[string]string{
				{"left": "{{count}}", "op": ">=", "right": "1"},
				{"left": "{{status}}", "op": "==", "right": "active"},
			},
		},
	}
	viper.Set("chains.pass_flow", stepsPass)

	// Capture output
	oldStderr := os.Stderr
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stderr = w
	os.Stdout = w

	err := runChain(context.Background(), "pass_flow")

	w.Close()
	os.Stderr = oldStderr
	os.Stdout = oldStdout
	r.Close()

	if err != nil {
		t.Errorf("runChain (pass) failed: %v", err)
	}

	// Test Failing Assertion
	stepsFail := []map[string]interface{}{
		{
			"request": "check_status",
			"extract": map[string]string{
				"count": "$.count",
			},
			"assert": []map[string]string{
				{"left": "{{count}}", "op": ">", "right": "10"}, // 5 > 10 should fail
			},
		},
	}
	viper.Set("chains.fail_flow", stepsFail)

	// Capture output
	r2, w2, _ := os.Pipe()
	os.Stderr = w2
	os.Stdout = w2

	errFail := runChain(context.Background(), "fail_flow")

	w2.Close()
	os.Stderr = oldStderr
	os.Stdout = oldStdout
	r2.Close()

	if errFail == nil {
		t.Errorf("expected runChain (fail) to fail, but it passed")
	} else if !strings.Contains(errFail.Error(), "assertion 1 failed") {
		t.Errorf("expected assertion error, got: %v", errFail)
	}
}
