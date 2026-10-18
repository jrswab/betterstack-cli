package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
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

// TestMonitorsGetSendsBearerToken covers A2 (spec R1): the request carries the
// `Authorization: Bearer <token>` header built from the environment value, and
// the target is passed through as one escaped path segment.
func TestMonitorsGetSendsBearerToken(t *testing.T) {
	var gotAuth, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.EscapedPath()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	t.Setenv(tokenEnvVar, testToken)

	code, _, stderr := invokeAPI(srv.URL, newHTTPClient(), "monitors", "get", "a b")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr)
	}
	if want := "Bearer " + testToken; gotAuth != want {
		t.Errorf("Authorization = %q, want %q", gotAuth, want)
	}
	if want := "/api/v2/monitors/a%20b"; gotPath != want {
		t.Errorf("path = %q, want %q", gotPath, want)
	}
}

// TestMonitorsGetCredentialFailures covers A2 (spec R2): an unset or empty
// token, and the former `BETTERSTACK_API_TOKEN` name set alone, each fail
// before any request with a message naming `BETTERSTACK_UPTIME_API_TOKEN`.
func TestMonitorsGetCredentialFailures(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T)
	}{
		{
			name: "unset",
			setup: func(t *testing.T) {
				t.Setenv(tokenEnvVar, "placeholder")
				if err := os.Unsetenv(tokenEnvVar); err != nil {
					t.Fatalf("Unsetenv: %v", err)
				}
			},
		},
		{
			name: "empty",
			setup: func(t *testing.T) {
				t.Setenv(tokenEnvVar, "")
			},
		},
		{
			name: "former name only",
			setup: func(t *testing.T) {
				t.Setenv("BETTERSTACK_API_TOKEN", testToken)
				t.Setenv(tokenEnvVar, "placeholder")
				if err := os.Unsetenv(tokenEnvVar); err != nil {
					t.Fatalf("Unsetenv: %v", err)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var requests int
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			tc.setup(t)

			code, stdout, stderr := invokeAPI(srv.URL, newHTTPClient(), "monitors", "get", "789")
			if code != 1 {
				t.Fatalf("exit code = %d, want 1", code)
			}
			if stdout != "" {
				t.Errorf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, tokenEnvVar) {
				t.Errorf("stderr = %q, want it to name %s", stderr, tokenEnvVar)
			}
			if requests != 0 {
				t.Errorf("server requests = %d, want 0", requests)
			}
		})
	}
}
