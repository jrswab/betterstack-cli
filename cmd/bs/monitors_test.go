package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

// testToken is the only token value any test uses. It is a fixed dummy and is
// never read from the ambient environment.
const testToken = "dummy-token-for-tests"

// invokeAPI runs one CLI vector through the test-only HTTP seam: baseURL and
// client replace the production endpoint and client, so no real request leaves
// the machine.
func invokeAPI(baseURL string, client *http.Client, args ...string) (code int, stdout, stderr string) {
	var out, err bytes.Buffer
	code = runWithAPI(args, &out, &err, baseURL, client)
	return code, out.String(), err.String()
}

// TestMonitorsGetReachesInjectedServer covers A1 (spec R9): the seam routes a
// `monitors get` invocation to the injected base URL through the injected
// client, and exactly one GET request arrives at the mock server.
func TestMonitorsGetReachesInjectedServer(t *testing.T) {
	var gotMethod, gotPath string
	var gotCount int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCount++
		gotMethod = r.Method
		gotPath = r.URL.EscapedPath()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":{}}`))
	}))
	defer srv.Close()

	t.Setenv(tokenEnvVar, testToken)

	code, _, stderr := invokeAPI(srv.URL, newHTTPClient(), "monitors", "get", "789")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr)
	}
	if gotCount != 1 {
		t.Fatalf("server requests = %d, want 1", gotCount)
	}
	if gotMethod != http.MethodGet {
		t.Errorf("method = %q, want %q", gotMethod, http.MethodGet)
	}
	if gotPath != "/api/v2/monitors/789" {
		t.Errorf("path = %q, want %q", gotPath, "/api/v2/monitors/789")
	}
}
