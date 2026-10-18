package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// TestParseIncidentsGetArgs covers A1 (spec R1.1, R1.2, R7.1): the incidents
// parser mirrors monitors get, accepting exactly one positional target and an
// optional --timeout in any position, and rejects every invalid vector before
// any request. No server is involved.
func TestParseIncidentsGetArgs(t *testing.T) {
	accepted := []struct {
		name    string
		args    []string
		target  string
		timeout time.Duration
	}{
		{"bare id", []string{"123"}, "123", defaultTimeout},
		{"url target", []string{"https://incidents.betterstack.com/team/t1/incidents/9"}, "https://incidents.betterstack.com/team/t1/incidents/9", defaultTimeout},
		{"timeout before target", []string{"--timeout", "60s", "123"}, "123", time.Minute},
		{"timeout after target", []string{"123", "--timeout", "250ms"}, "123", 250 * time.Millisecond},
	}
	for _, tc := range accepted {
		t.Run(tc.name, func(t *testing.T) {
			target, timeout, err := parseIncidentsGetArgs(tc.args)
			if err != nil {
				t.Fatalf("parseIncidentsGetArgs(%q) error = %v", tc.args, err)
			}
			if target != tc.target {
				t.Errorf("target = %q, want %q", target, tc.target)
			}
			if timeout != tc.timeout {
				t.Errorf("timeout = %v, want %v", timeout, tc.timeout)
			}
		})
	}

	rejected := []struct {
		name string
		args []string
	}{
		{"missing target", nil},
		{"too many positionals", []string{"1", "2"}},
		{"timeout missing value", []string{"123", "--timeout"}},
		{"timeout unparseable", []string{"123", "--timeout", "soon"}},
		{"timeout zero", []string{"123", "--timeout", "0s"}},
		{"timeout negative", []string{"123", "--timeout", "-1s"}},
		{"unknown long option", []string{"123", "--all"}},
		{"unknown short option", []string{"-h"}},
		{"help after target", []string{"123", "--help"}},
	}
	for _, tc := range rejected {
		t.Run(tc.name, func(t *testing.T) {
			target, timeout, err := parseIncidentsGetArgs(tc.args)
			if err == nil {
				t.Fatalf("parseIncidentsGetArgs(%q) = (%q, %v, nil), want an error", tc.args, target, timeout)
			}
			if strings.TrimSpace(err.Error()) == "" {
				t.Error("error message is empty, want a readable problem")
			}
		})
	}
}

// TestResolveIncidentTarget covers A1 (spec R2.1-R2.3): only the observed
// dashboard URL form resolves to its terminal ASCII-digit segment; every other
// scheme-ful form is rejected locally, while scheme-less targets stay opaque.
func TestResolveIncidentTarget(t *testing.T) {
	accepted := []struct {
		name   string
		target string
		want   string
	}{
		{"bare numeric", "123", "123"},
		{"bare with space", "a b", "a b"},
		{"bare lookalike path", "incidents.betterstack.com/team/t1/incidents/123", "incidents.betterstack.com/team/t1/incidents/123"},
		{"bare protocol-relative lookalike", "//incidents.betterstack.com/team/t1/incidents/123", "//incidents.betterstack.com/team/t1/incidents/123"},
		{"observed url", "https://incidents.betterstack.com/team/t581731/incidents/1033092002", "1033092002"},
		{"observed url second sample", "https://incidents.betterstack.com/team/team-2/incidents/9", "9"},
		{"uppercase scheme normalizes", "HTTPS://incidents.betterstack.com/team/t1/incidents/123", "123"},
		{"percent-encoded digits decode", "https://incidents.betterstack.com/team/t1/incidents/%31%32%33", "123"},
	}
	for _, tc := range accepted {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveIncidentTarget(tc.target)
			if err != nil {
				t.Fatalf("resolveIncidentTarget(%q) error = %v", tc.target, err)
			}
			if got != tc.want {
				t.Errorf("resolveIncidentTarget(%q) = %q, want %q", tc.target, got, tc.want)
			}
		})
	}

	rejected := []struct {
		name   string
		target string
	}{
		{"http scheme", "http://incidents.betterstack.com/team/t1/incidents/123"},
		{"ftp scheme", "ftp://incidents.betterstack.com/team/t1/incidents/123"},
		{"wrong host", "https://uptime.betterstack.com/team/t1/incidents/123"},
		{"near host suffix", "https://incidents.betterstack.com.evil.test/team/t1/incidents/123"},
		{"near host prefix", "https://evil.test/incidents.betterstack.com/team/t1/incidents/123"},
		{"empty host", "https:///team/t1/incidents/123"},
		{"trailing dot host", "https://incidents.betterstack.com./team/t1/incidents/123"},
		{"uppercase host", "https://INCIDENTS.betterstack.com/team/t1/incidents/123"},
		{"explicit port", "https://incidents.betterstack.com:8443/team/t1/incidents/123"},
		{"explicit 443 port", "https://incidents.betterstack.com:443/team/t1/incidents/123"},
		{"userinfo", "https://user:pass@incidents.betterstack.com/team/t1/incidents/123"},
		{"username only", "https://user@incidents.betterstack.com/team/t1/incidents/123"},
		{"query", "https://incidents.betterstack.com/team/t1/incidents/123?x=1"},
		{"empty query marker", "https://incidents.betterstack.com/team/t1/incidents/123?"},
		{"fragment", "https://incidents.betterstack.com/team/t1/incidents/123#frag"},
		{"empty fragment marker", "https://incidents.betterstack.com/team/t1/incidents/123#"},
		{"query and fragment", "https://incidents.betterstack.com/team/t1/incidents/123?x=1#frag"},
		{"trailing slash", "https://incidents.betterstack.com/team/t1/incidents/123/"},
		{"too few segments", "https://incidents.betterstack.com/team/t1/incidents"},
		{"too many segments", "https://incidents.betterstack.com/team/t1/incidents/123/extra"},
		{"empty team", "https://incidents.betterstack.com/team//incidents/123"},
		{"wrong path words", "https://incidents.betterstack.com/org/t1/incidents/123"},
		{"uppercase path words", "https://incidents.betterstack.com/Team/t1/Incidents/123"},
		{"missing terminal id", "https://incidents.betterstack.com/team/t1/incidents/"},
		{"non-numeric terminal", "https://incidents.betterstack.com/team/t1/incidents/abc"},
		{"mixed terminal", "https://incidents.betterstack.com/team/t1/incidents/12a"},
		{"negative terminal", "https://incidents.betterstack.com/team/t1/incidents/-123"},
		{"decimal terminal", "https://incidents.betterstack.com/team/t1/incidents/12.3"},
		{"plus terminal", "https://incidents.betterstack.com/team/t1/incidents/+123"},
		{"unicode digits", "https://incidents.betterstack.com/team/t1/incidents/١٢٣"},
		{"fullwidth digits", "https://incidents.betterstack.com/team/t1/incidents/１２３"},
		{"scheme-ful opaque", "https:team/t1/incidents/123"},
		{"encoded slash in id", "https://incidents.betterstack.com/team/t1/incidents/123%2F456"},
		{"lowercase encoded slash in id", "https://incidents.betterstack.com/team/t1/incidents/123%2f456"},
		{"encoded slash in team", "https://incidents.betterstack.com/team/t%2F1/incidents/123"},
		{"encoded query delimiter", "https://incidents.betterstack.com/team/t1/incidents/123%3Fx"},
		{"encoded fragment delimiter", "https://incidents.betterstack.com/team/t1/incidents/123%23x"},
		{"encoded backslash", "https://incidents.betterstack.com/team/t1/incidents/123%5C456"},
		{"encoded dot dot", "https://incidents.betterstack.com/team/t1/incidents/%2e%2e"},
		{"malformed percent escape", "https://incidents.betterstack.com/team/t1/incidents/123%zz"},
		{"space suffix", "https://incidents.betterstack.com/team/t1/incidents/123 "},
	}
	for _, tc := range rejected {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveIncidentTarget(tc.target)
			if err == nil {
				t.Fatalf("resolveIncidentTarget(%q) = %q, nil; want an error", tc.target, got)
			}
			if strings.TrimSpace(err.Error()) == "" {
				t.Error("error message is empty, want a readable problem")
			}
		})
	}
}

// invokeIncidentsGet calls the command directly, keeping the B1 tests on the
// same injected base URL and client seam used by the routed command.
func invokeIncidentsGet(baseURL string, client *http.Client, args ...string) (code int, stdout, stderr string) {
	var out, err bytes.Buffer
	code = incidentsGet(args, &out, &err, baseURL, client)
	return code, out.String(), err.String()
}

// TestIncidentsGetReachesInjectedServer covers B1 (spec R4): both target forms
// reach the mock exactly once at the v3 path with the bearer header.
func TestIncidentsGetReachesInjectedServer(t *testing.T) {
	cases := []struct {
		name string
		args []string
		path string
	}{
		{"bare id", []string{"123"}, "/api/v3/incidents/123"},
		{"dashboard url", []string{"https://incidents.betterstack.com/team/t581731/incidents/1033092002"}, "/api/v3/incidents/1033092002"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotMethod, gotPath, gotAuth string
			var gotCount int
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotCount++
				gotMethod = r.Method
				gotPath = r.URL.EscapedPath()
				gotAuth = r.Header.Get("Authorization")
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			t.Setenv(tokenEnvVar, testToken)

			code, _, stderr := invokeIncidentsGet(srv.URL, newHTTPClient(), tc.args...)
			if code != 0 {
				t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr)
			}
			if gotCount != 1 {
				t.Fatalf("server requests = %d, want 1", gotCount)
			}
			if gotMethod != http.MethodGet {
				t.Errorf("method = %q, want %q", gotMethod, http.MethodGet)
			}
			if gotPath != tc.path {
				t.Errorf("path = %q, want %q", gotPath, tc.path)
			}
			if want := "Bearer " + testToken; gotAuth != want {
				t.Errorf("Authorization = %q, want %q", gotAuth, want)
			}
		})
	}
}

// TestIncidentsGetRawSuccessBody covers B1 (spec R5): raw 2xx bodies pass
// through byte-for-byte, including data/included, non-JSON, and a trailing
// newline the API supplied.
func TestIncidentsGetRawSuccessBody(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"included envelope", `{"data":{"id":"1033092002"},"included":[{"type":"monitor"}]}`},
		{"non-json", "not json at all"},
		{"own trailing newline", "kept\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = io.WriteString(w, tc.body)
			}))
			defer srv.Close()

			t.Setenv(tokenEnvVar, testToken)

			code, stdout, stderr := invokeIncidentsGet(srv.URL, newHTTPClient(), "123")
			if code != 0 {
				t.Fatalf("exit code = %d, want 0", code)
			}
			if stdout != tc.body {
				t.Errorf("stdout = %q, want %q", stdout, tc.body)
			}
			if stderr != "" {
				t.Errorf("stderr = %q, want empty", stderr)
			}
		})
	}
}

// TestIncidentsGetNon2xxPassthrough covers B1 (spec R6.1): every non-2xx
// status, 3xx included, passes the raw body to stderr, reaches the server
// once, follows no redirect, and writes no stdout.
func TestIncidentsGetNon2xxPassthrough(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
	}{
		{"moved permanently", http.StatusMovedPermanently, "redirect body"},
		{"found", http.StatusFound, `{"errors":"moved"}`},
		{"temporary redirect", http.StatusTemporaryRedirect, "temporary"},
		{"unauthorized", http.StatusUnauthorized, `{"errors":"unauthorized"}`},
		{"not found", http.StatusNotFound, `{"errors":"not found"}`},
		{"too many requests", http.StatusTooManyRequests, "slow down"},
		{"server error", http.StatusInternalServerError, "boom"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var requests atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				if r.URL.Path == "/redirect-target" {
					w.WriteHeader(http.StatusOK)
					return
				}
				w.Header().Set("Location", "/redirect-target")
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, tc.body)
			}))
			defer srv.Close()

			t.Setenv(tokenEnvVar, testToken)

			code, stdout, stderr := invokeIncidentsGet(srv.URL, newHTTPClient(), "123")
			if code != 1 {
				t.Fatalf("exit code = %d, want 1", code)
			}
			if stdout != "" {
				t.Errorf("stdout = %q, want empty", stdout)
			}
			if stderr != tc.body {
				t.Errorf("stderr = %q, want raw body %q", stderr, tc.body)
			}
			if got := requests.Load(); got != 1 {
				t.Errorf("server requests = %d, want 1 (no redirect follow, no retry)", got)
			}
		})
	}
}

// TestIncidentsGetWrongExtractedIDPassthrough covers B1 (spec R2.4, R6.1): a
// URL-form target whose extracted ID the API rejects gets the ordinary raw
// non-2xx passthrough with one request to the fixed API path.
func TestIncidentsGetWrongExtractedIDPassthrough(t *testing.T) {
	var gotPath string
	var requests int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		gotPath = r.URL.EscapedPath()
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"errors":"not found"}`)
	}))
	defer srv.Close()

	t.Setenv(tokenEnvVar, testToken)

	code, stdout, stderr := invokeIncidentsGet(srv.URL, newHTTPClient(), "https://incidents.betterstack.com/team/t1/incidents/999")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
	if want := `{"errors":"not found"}`; stderr != want {
		t.Errorf("stderr = %q, want %q", stderr, want)
	}
	if requests != 1 {
		t.Errorf("server requests = %d, want 1", requests)
	}
	if want := "/api/v3/incidents/999"; gotPath != want {
		t.Errorf("path = %q, want %q", gotPath, want)
	}
}

// TestIncidentsGetTimeoutOverride covers B1 (spec R7.1): a short override
// expires with one request; a longer override succeeds.
func TestIncidentsGetTimeoutOverride(t *testing.T) {
	t.Run("expires", func(t *testing.T) {
		var requests atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests.Add(1)
			time.Sleep(150 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		t.Setenv(tokenEnvVar, testToken)

		code, stdout, stderr := invokeIncidentsGet(srv.URL, newHTTPClient(), "123", "--timeout", "10ms")
		if code != 1 {
			t.Fatalf("exit code = %d, want 1", code)
		}
		if stdout != "" {
			t.Errorf("stdout = %q, want empty", stdout)
		}
		if strings.TrimSpace(stderr) == "" {
			t.Error("stderr is empty, want a readable error")
		}
		if got := requests.Load(); got != 1 {
			t.Errorf("server requests = %d, want 1", got)
		}
	})

	t.Run("within override", func(t *testing.T) {
		const body = "ok body"
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, body)
		}))
		defer srv.Close()

		t.Setenv(tokenEnvVar, testToken)

		code, stdout, stderr := invokeIncidentsGet(srv.URL, newHTTPClient(), "123", "--timeout", "5s")
		if code != 0 {
			t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr)
		}
		if stdout != body {
			t.Errorf("stdout = %q, want %q", stdout, body)
		}
		if stderr != "" {
			t.Errorf("stderr = %q, want empty", stderr)
		}
	})
}

// TestIncidentsGetInvalidTimeout covers B1 (spec R7.1, R1): invalid durations
// fail before any request.
func TestIncidentsGetInvalidTimeout(t *testing.T) {
	cases := [][]string{
		{"123", "--timeout"},
		{"123", "--timeout", "soon"},
		{"123", "--timeout", "0s"},
		{"123", "--timeout", "-1s"},
	}
	for _, args := range cases {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var requests atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			t.Setenv(tokenEnvVar, testToken)

			code, stdout, stderr := invokeIncidentsGet(srv.URL, newHTTPClient(), args...)
			if code != 1 {
				t.Fatalf("exit code = %d, want 1", code)
			}
			if stdout != "" {
				t.Errorf("stdout = %q, want empty", stdout)
			}
			if strings.TrimSpace(stderr) == "" {
				t.Error("stderr is empty, want a readable error")
			}
			if got := requests.Load(); got != 0 {
				t.Errorf("server requests = %d, want 0", got)
			}
		})
	}
}

// TestIncidentsGetLocalFailureIsTokenFree covers B1 (spec R6.2, R6.3): a
// closed local port exits 1 with one concise token-free message.
func TestIncidentsGetLocalFailureIsTokenFree(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	closedURL := srv.URL
	srv.Close()

	t.Setenv(tokenEnvVar, testToken)

	code, stdout, stderr := invokeIncidentsGet(closedURL, newHTTPClient(), "123")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
	if strings.TrimSpace(stderr) == "" {
		t.Error("stderr is empty, want a readable error")
	}
	if strings.Contains(stderr, testToken) {
		t.Errorf("stderr leaks the token: %q", stderr)
	}
}

// TestIncidentsGetNeverPrintsToken covers B1 (spec R6.3): the token is absent
// from both output channels on success, non-2xx, and local failure.
func TestIncidentsGetNeverPrintsToken(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		var gotAuth string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotAuth = r.Header.Get("Authorization")
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, `{"data":{}}`)
		}))
		defer srv.Close()

		t.Setenv(tokenEnvVar, testToken)

		code, stdout, stderr := invokeIncidentsGet(srv.URL, newHTTPClient(), "123")
		if code != 0 {
			t.Fatalf("exit code = %d, want 0", code)
		}
		if want := "Bearer " + testToken; gotAuth != want {
			t.Errorf("Authorization = %q, want %q", gotAuth, want)
		}
		assertNoToken(t, stdout, stderr)
	})

	t.Run("non-2xx", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `{"errors":"not found"}`)
		}))
		defer srv.Close()

		t.Setenv(tokenEnvVar, testToken)

		code, stdout, stderr := invokeIncidentsGet(srv.URL, newHTTPClient(), "123")
		if code != 1 {
			t.Fatalf("exit code = %d, want 1", code)
		}
		assertNoToken(t, stdout, stderr)
	})

	t.Run("local failure", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		closedURL := srv.URL
		srv.Close()

		t.Setenv(tokenEnvVar, testToken)

		code, stdout, stderr := invokeIncidentsGet(closedURL, newHTTPClient(), "123")
		if code != 1 {
			t.Fatalf("exit code = %d, want 1", code)
		}
		assertNoToken(t, stdout, stderr)
	})
}

// TestIncidentsGetCredentialFailures covers B1 (spec R3.1): unset, empty, and
// former-name-only each fail before any request naming the current variable.
func TestIncidentsGetCredentialFailures(t *testing.T) {
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
			var requests atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			tc.setup(t)

			code, stdout, stderr := invokeIncidentsGet(srv.URL, newHTTPClient(), "123")
			if code != 1 {
				t.Fatalf("exit code = %d, want 1", code)
			}
			if stdout != "" {
				t.Errorf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, tokenEnvVar) {
				t.Errorf("stderr = %q, want it to name %s", stderr, tokenEnvVar)
			}
			if got := requests.Load(); got != 0 {
				t.Errorf("server requests = %d, want 0", got)
			}
		})
	}
}

// TestIncidentsGetRoutesThroughRunWithAPI covers B2 (spec R1, R9.1): the
// routed vector reaches the injected mock once, proving main's incidents get
// branch is wired ahead of the old not-implemented fallthrough.
func TestIncidentsGetRoutesThroughRunWithAPI(t *testing.T) {
	var requests atomic.Int32
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		gotPath = r.URL.EscapedPath()
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"data":{}}`)
	}))
	defer srv.Close()

	t.Setenv(tokenEnvVar, testToken)

	code, stdout, stderr := invokeAPI(srv.URL, newHTTPClient(), "incidents", "get", "123")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr)
	}
	if stdout != `{"data":{}}` {
		t.Errorf("stdout = %q, want the raw body", stdout)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty", stderr)
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("server requests = %d, want 1", got)
	}
	if want := "/api/v3/incidents/123"; gotPath != want {
		t.Errorf("path = %q, want %q", gotPath, want)
	}
}

// TestIncidentsGetUsageFailuresThroughRouting covers B2 (spec R1.1, R9.1):
// missing and extra positional targets for incidents get fail with zero
// requests through the shared resourceUsageError branch.
func TestIncidentsGetUsageFailuresThroughRouting(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"missing target", []string{"incidents", "get"}},
		{"extra target", []string{"incidents", "get", "1", "2"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var requests atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			t.Setenv(tokenEnvVar, testToken)

			code, stdout, stderr := invokeAPI(srv.URL, newHTTPClient(), tc.args...)
			if code != 1 {
				t.Fatalf("exit code = %d, want 1", code)
			}
			if stdout != "" {
				t.Errorf("stdout = %q, want empty", stdout)
			}
			if strings.TrimSpace(stderr) == "" {
				t.Error("stderr is empty, want a readable error")
			}
			if got := requests.Load(); got != 0 {
				t.Errorf("server requests = %d, want 0", got)
			}
		})
	}
}

// TestIncidentsGetHelpExitsZeroThroughRouting covers B3 (spec R1.3): the
// unchanged incidentsGetHelp text exits 0 with empty stderr and no request.
func TestIncidentsGetHelpExitsZeroThroughRouting(t *testing.T) {
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	t.Setenv(tokenEnvVar, testToken)

	code, stdout, stderr := invokeAPI(srv.URL, newHTTPClient(), "incidents", "get", "--help")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stdout != incidentsGetHelp {
		t.Errorf("stdout = %q, want incidentsGetHelp", stdout)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty", stderr)
	}
	if got := requests.Load(); got != 0 {
		t.Errorf("server requests = %d, want 0", got)
	}
}

// TestIncidentsGetRejectsAbsentFeatures covers B3 (spec R1.3, R8): unknown
// options and --help in flag position are invalid usage with zero requests.
func TestIncidentsGetRejectsAbsentFeatures(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"--all", []string{"incidents", "get", "123", "--all"}},
		{"--pretty", []string{"incidents", "get", "123", "--pretty"}},
		{"--query", []string{"incidents", "get", "123", "--query", "x"}},
		{"short help", []string{"incidents", "get", "-h"}},
		{"help after target", []string{"incidents", "get", "123", "--help"}},
		{"unknown flag", []string{"incidents", "get", "123", "--bogus"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var requests atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			t.Setenv(tokenEnvVar, testToken)

			code, stdout, stderr := invokeAPI(srv.URL, newHTTPClient(), tc.args...)
			if code != 1 {
				t.Fatalf("exit code = %d, want 1", code)
			}
			if stdout != "" {
				t.Errorf("stdout = %q, want empty", stdout)
			}
			if strings.TrimSpace(stderr) == "" {
				t.Error("stderr is empty, want a readable error")
			}
			if strings.Contains(stderr, testToken) {
				t.Errorf("stderr leaks the token: %q", stderr)
			}
			if got := requests.Load(); got != 0 {
				t.Errorf("server requests = %d, want 0", got)
			}
		})
	}
}

// TestIncidentsGetRejectedURLShapesThroughRouting covers B3 (spec R2.2, R8):
// every non-observed URL shape is rejected locally, before any request, with a
// readable token-free message. It exercises the routing seam and repeats the
// escaped-separator, delimiter, Unicode-digit, near-host, query/fragment, and
// percent-encoding classes.
func TestIncidentsGetRejectedURLShapesThroughRouting(t *testing.T) {
	cases := []struct {
		name   string
		target string
	}{
		{"http scheme", "http://incidents.betterstack.com/team/t1/incidents/123"},
		{"ftp scheme", "ftp://incidents.betterstack.com/team/t1/incidents/123"},
		{"wrong host", "https://uptime.betterstack.com/team/t1/incidents/123"},
		{"near host suffix", "https://incidents.betterstack.com.evil.test/team/t1/incidents/123"},
		{"near host prefix", "https://evil.test/incidents.betterstack.com/team/t1/incidents/123"},
		{"empty host", "https:///team/t1/incidents/123"},
		{"uppercase host", "https://INCIDENTS.betterstack.com/team/t1/incidents/123"},
		{"explicit port", "https://incidents.betterstack.com:8443/team/t1/incidents/123"},
		{"explicit 443 port", "https://incidents.betterstack.com:443/team/t1/incidents/123"},
		{"userinfo", "https://user:pass@incidents.betterstack.com/team/t1/incidents/123"},
		{"query", "https://incidents.betterstack.com/team/t1/incidents/123?x=1"},
		{"empty query marker", "https://incidents.betterstack.com/team/t1/incidents/123?"},
		{"fragment", "https://incidents.betterstack.com/team/t1/incidents/123#frag"},
		{"empty fragment marker", "https://incidents.betterstack.com/team/t1/incidents/123#"},
		{"trailing slash", "https://incidents.betterstack.com/team/t1/incidents/123/"},
		{"too few segments", "https://incidents.betterstack.com/team/t1/incidents"},
		{"too many segments", "https://incidents.betterstack.com/team/t1/incidents/123/extra"},
		{"empty team", "https://incidents.betterstack.com/team//incidents/123"},
		{"wrong path words", "https://incidents.betterstack.com/org/t1/incidents/123"},
		{"missing terminal id", "https://incidents.betterstack.com/team/t1/incidents/"},
		{"non-numeric terminal", "https://incidents.betterstack.com/team/t1/incidents/abc"},
		{"mixed terminal", "https://incidents.betterstack.com/team/t1/incidents/12a"},
		{"negative terminal", "https://incidents.betterstack.com/team/t1/incidents/-123"},
		{"decimal terminal", "https://incidents.betterstack.com/team/t1/incidents/12.3"},
		{"unicode digits", "https://incidents.betterstack.com/team/t1/incidents/١٢٣"},
		{"fullwidth digits", "https://incidents.betterstack.com/team/t1/incidents/１２３"},
		{"scheme-ful opaque", "https:team/t1/incidents/123"},
		{"encoded slash in id", "https://incidents.betterstack.com/team/t1/incidents/123%2F456"},
		{"lowercase encoded slash in id", "https://incidents.betterstack.com/team/t1/incidents/123%2f456"},
		{"encoded slash in team", "https://incidents.betterstack.com/team/t%2F1/incidents/123"},
		{"encoded query delimiter", "https://incidents.betterstack.com/team/t1/incidents/123%3Fx"},
		{"encoded fragment delimiter", "https://incidents.betterstack.com/team/t1/incidents/123%23x"},
		{"encoded backslash", "https://incidents.betterstack.com/team/t1/incidents/123%5C456"},
		{"encoded dot dot", "https://incidents.betterstack.com/team/t1/incidents/%2e%2e"},
		{"malformed percent escape", "https://incidents.betterstack.com/team/t1/incidents/123%zz"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var requests atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			t.Setenv(tokenEnvVar, testToken)

			code, stdout, stderr := invokeAPI(srv.URL, newHTTPClient(), "incidents", "get", tc.target)
			if code != 1 {
				t.Fatalf("exit code = %d, want 1", code)
			}
			if stdout != "" {
				t.Errorf("stdout = %q, want empty", stdout)
			}
			if strings.TrimSpace(stderr) == "" {
				t.Error("stderr is empty, want a readable error")
			}
			if strings.Contains(stderr, testToken) {
				t.Errorf("stderr leaks the token: %q", stderr)
			}
			if got := requests.Load(); got != 0 {
				t.Errorf("server requests = %d, want 0", got)
			}
		})
	}
}

// TestIncidentsGetCredentialFailuresThroughRouting covers B3 (spec R3.1): the
// routed command fails before any request for unset, empty, and former-name
// credentials, naming BETTERSTACK_UPTIME_API_TOKEN.
func TestIncidentsGetCredentialFailuresThroughRouting(t *testing.T) {
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
			var requests atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			tc.setup(t)

			code, stdout, stderr := invokeAPI(srv.URL, newHTTPClient(), "incidents", "get", "123")
			if code != 1 {
				t.Fatalf("exit code = %d, want 1", code)
			}
			if stdout != "" {
				t.Errorf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, tokenEnvVar) {
				t.Errorf("stderr = %q, want it to name %s", stderr, tokenEnvVar)
			}
			if got := requests.Load(); got != 0 {
				t.Errorf("server requests = %d, want 0", got)
			}
		})
	}
}

// TestIncidentsGetNeverPrintsTokenThroughRouting covers B3 (spec R6.3): the
// routed command keeps the token off both channels on success, non-2xx, and
// local failure.
func TestIncidentsGetNeverPrintsTokenThroughRouting(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		var gotAuth string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotAuth = r.Header.Get("Authorization")
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, `{"data":{}}`)
		}))
		defer srv.Close()

		t.Setenv(tokenEnvVar, testToken)

		code, stdout, stderr := invokeAPI(srv.URL, newHTTPClient(), "incidents", "get", "123")
		if code != 0 {
			t.Fatalf("exit code = %d, want 0", code)
		}
		if want := "Bearer " + testToken; gotAuth != want {
			t.Errorf("Authorization = %q, want %q", gotAuth, want)
		}
		assertNoToken(t, stdout, stderr)
	})

	t.Run("non-2xx", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = io.WriteString(w, `{"errors":"unauthorized"}`)
		}))
		defer srv.Close()

		t.Setenv(tokenEnvVar, testToken)

		code, stdout, stderr := invokeAPI(srv.URL, newHTTPClient(), "incidents", "get", "123")
		if code != 1 {
			t.Fatalf("exit code = %d, want 1", code)
		}
		assertNoToken(t, stdout, stderr)
	})

	t.Run("local failure", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		closedURL := srv.URL
		srv.Close()

		t.Setenv(tokenEnvVar, testToken)

		code, stdout, stderr := invokeAPI(closedURL, newHTTPClient(), "incidents", "get", "123")
		if code != 1 {
			t.Fatalf("exit code = %d, want 1", code)
		}
		assertNoToken(t, stdout, stderr)
	})
}

// invokeIncidentsTimeline calls the command directly, keeping the A1 tests on
// the same injected base URL and client seam used by the routed command.
func invokeIncidentsTimeline(baseURL string, client *http.Client, args ...string) (code int, stdout, stderr string) {
	var out, err bytes.Buffer
	code = incidentsTimeline(args, &out, &err, baseURL, client)
	return code, out.String(), err.String()
}

// TestIncidentsTimelineReachesInjectedServer covers A1 (spec R2.1, R4.1,
// R4.2): both target forms reach the mock exactly once at the v3 timeline path
// with the bearer header and an empty query string.
func TestIncidentsTimelineReachesInjectedServer(t *testing.T) {
	cases := []struct {
		name string
		args []string
		path string
	}{
		{"bare id", []string{"123"}, "/api/v3/incidents/123/timeline"},
		{"dashboard url", []string{"https://incidents.betterstack.com/team/t581731/incidents/1033092002"}, "/api/v3/incidents/1033092002/timeline"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotMethod, gotPath, gotAuth, gotRawQuery string
			var gotCount int
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotCount++
				gotMethod = r.Method
				gotPath = r.URL.EscapedPath()
				gotAuth = r.Header.Get("Authorization")
				gotRawQuery = r.URL.RawQuery
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			t.Setenv(tokenEnvVar, testToken)

			code, _, stderr := invokeIncidentsTimeline(srv.URL, newHTTPClient(), tc.args...)
			if code != 0 {
				t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr)
			}
			if gotCount != 1 {
				t.Fatalf("server requests = %d, want 1", gotCount)
			}
			if gotMethod != http.MethodGet {
				t.Errorf("method = %q, want %q", gotMethod, http.MethodGet)
			}
			if gotPath != tc.path {
				t.Errorf("path = %q, want %q", gotPath, tc.path)
			}
			if want := "Bearer " + testToken; gotAuth != want {
				t.Errorf("Authorization = %q, want %q", gotAuth, want)
			}
			if gotRawQuery != "" {
				t.Errorf("query = %q, want empty", gotRawQuery)
			}
		})
	}
}

// timelineHeterogeneousBody is a pagination-less JSON:API envelope carrying
// the heterogeneous item IDs and all seven documented item_type shapes,
// including timeline_truncated and the object-valued comment content (spec
// R5.2).
const timelineHeterogeneousBody = `{"data":[` +
	`{"id":"23","type":"timeline_item","attributes":{"item_type":"generic","at":"2021-02-12 23:59:59 UTC","data":{"text":"a"}}},` +
	`{"id":"2021-02-12 23:59:59 UTC","type":"timeline_item","attributes":{"item_type":"generic_card","at":"2021-02-12 23:59:59 UTC","data":{}}},` +
	`{"id":"incident-started","type":"timeline_item","attributes":{"item_type":"timeline_truncated","at":"2021-02-12 23:59:59 UTC","data":{}}},` +
	`{"id":"2470302-content-response","type":"timeline_item","attributes":{"item_type":"response_item","at":"2021-02-12 23:59:59 UTC","data":{}}},` +
	`{"id":"payload-1","type":"timeline_item","attributes":{"item_type":"payload","at":"2021-02-12 23:59:59 UTC","data":{}}},` +
	`{"id":"comment-1","type":"timeline_item","attributes":{"item_type":"comment","at":"2021-02-12 23:59:59 UTC","data":{"content":{"text":"hello"}}}},` +
	`{"id":"status-report-1","type":"timeline_item","attributes":{"item_type":"status_report_card","at":"2021-02-12 23:59:59 UTC","data":{}}}` +
	`]}`

// TestIncidentsTimelineRawSuccessBody covers A1 (spec R5.1, R5.2): raw 2xx
// bodies pass through byte-for-byte with no appended newline, including the
// heterogeneous timeline envelope, a non-JSON body, and a body with its own
// trailing newline.
func TestIncidentsTimelineRawSuccessBody(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"heterogeneous envelope", timelineHeterogeneousBody},
		{"non-json", "not json at all"},
		{"own trailing newline", "kept\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = io.WriteString(w, tc.body)
			}))
			defer srv.Close()

			t.Setenv(tokenEnvVar, testToken)

			code, stdout, stderr := invokeIncidentsTimeline(srv.URL, newHTTPClient(), "123")
			if code != 0 {
				t.Fatalf("exit code = %d, want 0", code)
			}
			if stdout != tc.body {
				t.Errorf("stdout = %q, want %q", stdout, tc.body)
			}
			if stderr != "" {
				t.Errorf("stderr = %q, want empty", stderr)
			}
		})
	}
}

// TestIncidentsTimelineTruncatedPassthroughIsSilent covers A1 (spec R5.2,
// R5.3): a timeline_truncated item passes through untouched, and stdout equals
// the raw body with an empty stderr, so the CLI adds no truncation notice,
// item count, or completeness claim of its own.
func TestIncidentsTimelineTruncatedPassthroughIsSilent(t *testing.T) {
	body := `{"data":[{"id":"incident-started","type":"timeline_item","attributes":{"item_type":"timeline_truncated","at":"2021-02-12 23:59:59 UTC","data":{}}}]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, body)
	}))
	defer srv.Close()

	t.Setenv(tokenEnvVar, testToken)

	code, stdout, stderr := invokeIncidentsTimeline(srv.URL, newHTTPClient(), "123")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stdout != body {
		t.Errorf("stdout = %q, want the raw body %q", stdout, body)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty", stderr)
	}
}

// TestIncidentsTimelineNon2xxPassthrough covers A1 (spec R6.1): every non-2xx
// status, 3xx included, passes the raw body to stderr, reaches the server
// once, follows no redirect, and writes no stdout.
func TestIncidentsTimelineNon2xxPassthrough(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
	}{
		{"moved permanently", http.StatusMovedPermanently, "redirect body"},
		{"found", http.StatusFound, `{"errors":"moved"}`},
		{"temporary redirect", http.StatusTemporaryRedirect, "temporary"},
		{"unauthorized", http.StatusUnauthorized, `{"errors":"unauthorized"}`},
		{"not found", http.StatusNotFound, `{"errors":"not found"}`},
		{"too many requests", http.StatusTooManyRequests, "slow down"},
		{"server error", http.StatusInternalServerError, "boom"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var requests atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				if r.URL.Path == "/redirect-target" {
					w.WriteHeader(http.StatusOK)
					return
				}
				w.Header().Set("Location", "/redirect-target")
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, tc.body)
			}))
			defer srv.Close()

			t.Setenv(tokenEnvVar, testToken)

			code, stdout, stderr := invokeIncidentsTimeline(srv.URL, newHTTPClient(), "123")
			if code != 1 {
				t.Fatalf("exit code = %d, want 1", code)
			}
			if stdout != "" {
				t.Errorf("stdout = %q, want empty", stdout)
			}
			if stderr != tc.body {
				t.Errorf("stderr = %q, want raw body %q", stderr, tc.body)
			}
			if got := requests.Load(); got != 1 {
				t.Errorf("server requests = %d, want 1 (no redirect follow, no retry)", got)
			}
		})
	}
}

// TestIncidentsTimelineWrongExtractedIDPassthrough covers A1 (spec R2.4,
// R6.1): a URL-form target whose extracted ID the API rejects gets the
// ordinary raw non-2xx passthrough with one request to the fixed timeline
// path.
func TestIncidentsTimelineWrongExtractedIDPassthrough(t *testing.T) {
	var gotPath string
	var requests int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		gotPath = r.URL.EscapedPath()
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"errors":"not found"}`)
	}))
	defer srv.Close()

	t.Setenv(tokenEnvVar, testToken)

	code, stdout, stderr := invokeIncidentsTimeline(srv.URL, newHTTPClient(), "https://incidents.betterstack.com/team/t1/incidents/999")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
	if want := `{"errors":"not found"}`; stderr != want {
		t.Errorf("stderr = %q, want %q", stderr, want)
	}
	if requests != 1 {
		t.Errorf("server requests = %d, want 1", requests)
	}
	if want := "/api/v3/incidents/999/timeline"; gotPath != want {
		t.Errorf("path = %q, want %q", gotPath, want)
	}
}

// TestIncidentsTimelineTimeoutOverride covers A1 (spec R7.1): a short override
// expires with one request; a longer override succeeds.
func TestIncidentsTimelineTimeoutOverride(t *testing.T) {
	t.Run("expires", func(t *testing.T) {
		var requests atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests.Add(1)
			time.Sleep(150 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		t.Setenv(tokenEnvVar, testToken)

		code, stdout, stderr := invokeIncidentsTimeline(srv.URL, newHTTPClient(), "123", "--timeout", "10ms")
		if code != 1 {
			t.Fatalf("exit code = %d, want 1", code)
		}
		if stdout != "" {
			t.Errorf("stdout = %q, want empty", stdout)
		}
		if strings.TrimSpace(stderr) == "" {
			t.Error("stderr is empty, want a readable error")
		}
		if got := requests.Load(); got != 1 {
			t.Errorf("server requests = %d, want 1", got)
		}
	})

	t.Run("within override", func(t *testing.T) {
		const body = "ok body"
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, body)
		}))
		defer srv.Close()

		t.Setenv(tokenEnvVar, testToken)

		code, stdout, stderr := invokeIncidentsTimeline(srv.URL, newHTTPClient(), "123", "--timeout", "5s")
		if code != 0 {
			t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr)
		}
		if stdout != body {
			t.Errorf("stdout = %q, want %q", stdout, body)
		}
		if stderr != "" {
			t.Errorf("stderr = %q, want empty", stderr)
		}
	})
}

// TestIncidentsTimelineInvalidTimeout covers A1 (spec R1.2, R7.1): invalid
// durations fail before any request.
func TestIncidentsTimelineInvalidTimeout(t *testing.T) {
	cases := [][]string{
		{"123", "--timeout"},
		{"123", "--timeout", "soon"},
		{"123", "--timeout", "0s"},
		{"123", "--timeout", "-1s"},
	}
	for _, args := range cases {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var requests atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			t.Setenv(tokenEnvVar, testToken)

			code, stdout, stderr := invokeIncidentsTimeline(srv.URL, newHTTPClient(), args...)
			if code != 1 {
				t.Fatalf("exit code = %d, want 1", code)
			}
			if stdout != "" {
				t.Errorf("stdout = %q, want empty", stdout)
			}
			if strings.TrimSpace(stderr) == "" {
				t.Error("stderr is empty, want a readable error")
			}
			if got := requests.Load(); got != 0 {
				t.Errorf("server requests = %d, want 0", got)
			}
		})
	}
}

// TestIncidentsTimelineLocalFailureIsTokenFree covers A1 (spec R6.2, R6.3): a
// closed local port exits 1 with one concise token-free message.
func TestIncidentsTimelineLocalFailureIsTokenFree(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	closedURL := srv.URL
	srv.Close()

	t.Setenv(tokenEnvVar, testToken)

	code, stdout, stderr := invokeIncidentsTimeline(closedURL, newHTTPClient(), "123")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
	if strings.TrimSpace(stderr) == "" {
		t.Error("stderr is empty, want a readable error")
	}
	if strings.Contains(stderr, testToken) {
		t.Errorf("stderr leaks the token: %q", stderr)
	}
}

// TestIncidentsTimelineNeverPrintsToken covers A1 (spec R6.3): the token is
// absent from both output channels on success, non-2xx, and local failure.
func TestIncidentsTimelineNeverPrintsToken(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		var gotAuth string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotAuth = r.Header.Get("Authorization")
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, `{"data":[]}`)
		}))
		defer srv.Close()

		t.Setenv(tokenEnvVar, testToken)

		code, stdout, stderr := invokeIncidentsTimeline(srv.URL, newHTTPClient(), "123")
		if code != 0 {
			t.Fatalf("exit code = %d, want 0", code)
		}
		if want := "Bearer " + testToken; gotAuth != want {
			t.Errorf("Authorization = %q, want %q", gotAuth, want)
		}
		assertNoToken(t, stdout, stderr)
	})

	t.Run("non-2xx", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `{"errors":"not found"}`)
		}))
		defer srv.Close()

		t.Setenv(tokenEnvVar, testToken)

		code, stdout, stderr := invokeIncidentsTimeline(srv.URL, newHTTPClient(), "123")
		if code != 1 {
			t.Fatalf("exit code = %d, want 1", code)
		}
		assertNoToken(t, stdout, stderr)
	})

	t.Run("local failure", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		closedURL := srv.URL
		srv.Close()

		t.Setenv(tokenEnvVar, testToken)

		code, stdout, stderr := invokeIncidentsTimeline(closedURL, newHTTPClient(), "123")
		if code != 1 {
			t.Fatalf("exit code = %d, want 1", code)
		}
		assertNoToken(t, stdout, stderr)
	})
}

// TestIncidentsTimelineCredentialFailures covers A1 (spec R3.1, R3.2): unset,
// empty, and former-name-only each fail before any request naming the current
// variable.
func TestIncidentsTimelineCredentialFailures(t *testing.T) {
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
			var requests atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			tc.setup(t)

			code, stdout, stderr := invokeIncidentsTimeline(srv.URL, newHTTPClient(), "123")
			if code != 1 {
				t.Fatalf("exit code = %d, want 1", code)
			}
			if stdout != "" {
				t.Errorf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, tokenEnvVar) {
				t.Errorf("stderr = %q, want it to name %s", stderr, tokenEnvVar)
			}
			if got := requests.Load(); got != 0 {
				t.Errorf("server requests = %d, want 0", got)
			}
		})
	}
}

// TestIncidentsTimelineRoutesThroughRunWithAPI covers B1 (spec R1, R9.1): the
// routed `incidents timeline` vector reaches the injected mock once at the v3
// timeline path, proving main's timeline branch is wired ahead of the old
// not-implemented fallthrough.
func TestIncidentsTimelineRoutesThroughRunWithAPI(t *testing.T) {
	var requests atomic.Int32
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		gotPath = r.URL.EscapedPath()
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"data":[]}`)
	}))
	defer srv.Close()

	t.Setenv(tokenEnvVar, testToken)

	code, stdout, stderr := invokeAPI(srv.URL, newHTTPClient(), "incidents", "timeline", "123")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr)
	}
	if stdout != `{"data":[]}` {
		t.Errorf("stdout = %q, want the raw body", stdout)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty", stderr)
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("server requests = %d, want 1", got)
	}
	if want := "/api/v3/incidents/123/timeline"; gotPath != want {
		t.Errorf("path = %q, want %q", gotPath, want)
	}
}

// TestIncidentsTimelineHelpExitsZeroThroughRouting covers B2 (spec R1.3): the
// unchanged incidentsTimelineHelp text exits 0 with empty stderr and no
// request.
func TestIncidentsTimelineHelpExitsZeroThroughRouting(t *testing.T) {
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	t.Setenv(tokenEnvVar, testToken)

	code, stdout, stderr := invokeAPI(srv.URL, newHTTPClient(), "incidents", "timeline", "--help")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stdout != incidentsTimelineHelp {
		t.Errorf("stdout = %q, want incidentsTimelineHelp", stdout)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty", stderr)
	}
	if got := requests.Load(); got != 0 {
		t.Errorf("server requests = %d, want 0", got)
	}
}

// TestIncidentsTimelineRejectsAbsentFeatures covers B2 (spec R1.3, R8):
// unknown options and --help in flag position are invalid usage with zero
// requests, proving no pagination, filter, or output-mode flag exists.
func TestIncidentsTimelineRejectsAbsentFeatures(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"--page", []string{"incidents", "timeline", "123", "--page", "2"}},
		{"--per-page", []string{"incidents", "timeline", "123", "--per-page", "5"}},
		{"--pretty", []string{"incidents", "timeline", "123", "--pretty"}},
		{"--query", []string{"incidents", "timeline", "123", "--query", "x"}},
		{"short help", []string{"incidents", "timeline", "-h"}},
		{"help after target", []string{"incidents", "timeline", "123", "--help"}},
		{"unknown flag", []string{"incidents", "timeline", "123", "--bogus"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var requests atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			t.Setenv(tokenEnvVar, testToken)

			code, stdout, stderr := invokeAPI(srv.URL, newHTTPClient(), tc.args...)
			if code != 1 {
				t.Fatalf("exit code = %d, want 1", code)
			}
			if stdout != "" {
				t.Errorf("stdout = %q, want empty", stdout)
			}
			if strings.TrimSpace(stderr) == "" {
				t.Error("stderr is empty, want a readable error")
			}
			if strings.Contains(stderr, testToken) {
				t.Errorf("stderr leaks the token: %q", stderr)
			}
			if got := requests.Load(); got != 0 {
				t.Errorf("server requests = %d, want 0", got)
			}
		})
	}
}

// TestIncidentsTimelineRejectedURLShapesThroughRouting covers B2 (spec R2.2,
// R8): every non-observed URL shape is rejected locally, before any request,
// with a readable token-free message through the routing seam.
func TestIncidentsTimelineRejectedURLShapesThroughRouting(t *testing.T) {
	cases := []struct {
		name   string
		target string
	}{
		{"http scheme", "http://incidents.betterstack.com/team/t1/incidents/123"},
		{"ftp scheme", "ftp://incidents.betterstack.com/team/t1/incidents/123"},
		{"wrong host", "https://uptime.betterstack.com/team/t1/incidents/123"},
		{"near host suffix", "https://incidents.betterstack.com.evil.test/team/t1/incidents/123"},
		{"near host prefix", "https://evil.test/incidents.betterstack.com/team/t1/incidents/123"},
		{"empty host", "https:///team/t1/incidents/123"},
		{"uppercase host", "https://INCIDENTS.betterstack.com/team/t1/incidents/123"},
		{"explicit port", "https://incidents.betterstack.com:8443/team/t1/incidents/123"},
		{"explicit 443 port", "https://incidents.betterstack.com:443/team/t1/incidents/123"},
		{"userinfo", "https://user:pass@incidents.betterstack.com/team/t1/incidents/123"},
		{"query", "https://incidents.betterstack.com/team/t1/incidents/123?x=1"},
		{"empty query marker", "https://incidents.betterstack.com/team/t1/incidents/123?"},
		{"fragment", "https://incidents.betterstack.com/team/t1/incidents/123#frag"},
		{"empty fragment marker", "https://incidents.betterstack.com/team/t1/incidents/123#"},
		{"trailing slash", "https://incidents.betterstack.com/team/t1/incidents/123/"},
		{"too few segments", "https://incidents.betterstack.com/team/t1/incidents"},
		{"too many segments", "https://incidents.betterstack.com/team/t1/incidents/123/extra"},
		{"empty team", "https://incidents.betterstack.com/team//incidents/123"},
		{"wrong path words", "https://incidents.betterstack.com/org/t1/incidents/123"},
		{"missing terminal id", "https://incidents.betterstack.com/team/t1/incidents/"},
		{"non-numeric terminal", "https://incidents.betterstack.com/team/t1/incidents/abc"},
		{"mixed terminal", "https://incidents.betterstack.com/team/t1/incidents/12a"},
		{"negative terminal", "https://incidents.betterstack.com/team/t1/incidents/-123"},
		{"decimal terminal", "https://incidents.betterstack.com/team/t1/incidents/12.3"},
		{"unicode digits", "https://incidents.betterstack.com/team/t1/incidents/\u0661\u0662\u0663"},
		{"fullwidth digits", "https://incidents.betterstack.com/team/t1/incidents/\uff11\uff12\uff13"},
		{"scheme-ful opaque", "https:team/t1/incidents/123"},
		{"encoded slash in id", "https://incidents.betterstack.com/team/t1/incidents/123%2F456"},
		{"lowercase encoded slash in id", "https://incidents.betterstack.com/team/t1/incidents/123%2f456"},
		{"encoded slash in team", "https://incidents.betterstack.com/team/t%2F1/incidents/123"},
		{"encoded query delimiter", "https://incidents.betterstack.com/team/t1/incidents/123%3Fx"},
		{"encoded fragment delimiter", "https://incidents.betterstack.com/team/t1/incidents/123%23x"},
		{"encoded backslash", "https://incidents.betterstack.com/team/t1/incidents/123%5C456"},
		{"encoded dot dot", "https://incidents.betterstack.com/team/t1/incidents/%2e%2e"},
		{"malformed percent escape", "https://incidents.betterstack.com/team/t1/incidents/123%zz"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var requests atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			t.Setenv(tokenEnvVar, testToken)

			code, stdout, stderr := invokeAPI(srv.URL, newHTTPClient(), "incidents", "timeline", tc.target)
			if code != 1 {
				t.Fatalf("exit code = %d, want 1", code)
			}
			if stdout != "" {
				t.Errorf("stdout = %q, want empty", stdout)
			}
			if strings.TrimSpace(stderr) == "" {
				t.Error("stderr is empty, want a readable error")
			}
			if strings.Contains(stderr, testToken) {
				t.Errorf("stderr leaks the token: %q", stderr)
			}
			if got := requests.Load(); got != 0 {
				t.Errorf("server requests = %d, want 0", got)
			}
		})
	}
}

// TestIncidentsTimelineCredentialFailuresThroughRouting covers B2 (spec R3.1):
// the routed command fails before any request for unset, empty, and
// former-name credentials, naming BETTERSTACK_UPTIME_API_TOKEN.
func TestIncidentsTimelineCredentialFailuresThroughRouting(t *testing.T) {
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
			var requests atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			tc.setup(t)

			code, stdout, stderr := invokeAPI(srv.URL, newHTTPClient(), "incidents", "timeline", "123")
			if code != 1 {
				t.Fatalf("exit code = %d, want 1", code)
			}
			if stdout != "" {
				t.Errorf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, tokenEnvVar) {
				t.Errorf("stderr = %q, want it to name %s", stderr, tokenEnvVar)
			}
			if got := requests.Load(); got != 0 {
				t.Errorf("server requests = %d, want 0", got)
			}
		})
	}
}

// TestIncidentsTimelineNeverPrintsTokenThroughRouting covers B2 (spec R6.3):
// the routed command keeps the token off both channels on success, non-2xx,
// and local failure.
func TestIncidentsTimelineNeverPrintsTokenThroughRouting(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		var gotAuth string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotAuth = r.Header.Get("Authorization")
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, `{"data":[]}`)
		}))
		defer srv.Close()

		t.Setenv(tokenEnvVar, testToken)

		code, stdout, stderr := invokeAPI(srv.URL, newHTTPClient(), "incidents", "timeline", "123")
		if code != 0 {
			t.Fatalf("exit code = %d, want 0", code)
		}
		if want := "Bearer " + testToken; gotAuth != want {
			t.Errorf("Authorization = %q, want %q", gotAuth, want)
		}
		assertNoToken(t, stdout, stderr)
	})

	t.Run("non-2xx", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = io.WriteString(w, `{"errors":"unauthorized"}`)
		}))
		defer srv.Close()

		t.Setenv(tokenEnvVar, testToken)

		code, stdout, stderr := invokeAPI(srv.URL, newHTTPClient(), "incidents", "timeline", "123")
		if code != 1 {
			t.Fatalf("exit code = %d, want 1", code)
		}
		assertNoToken(t, stdout, stderr)
	})

	t.Run("local failure", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		closedURL := srv.URL
		srv.Close()

		t.Setenv(tokenEnvVar, testToken)

		code, stdout, stderr := invokeAPI(closedURL, newHTTPClient(), "incidents", "timeline", "123")
		if code != 1 {
			t.Fatalf("exit code = %d, want 1", code)
		}
		assertNoToken(t, stdout, stderr)
	})
}

// TestParseIncidentsListArgs covers A1 (spec R1.1-R1.7, R6.1, R7): the
// incidents list parser records every accepted flag occurrence unchanged,
// validates only the boolean literals and the metadata pair grammar, and
// rejects every invalid vector before any request. No server is involved.
func TestParseIncidentsListArgs(t *testing.T) {
	accepted := []struct {
		name string
		args []string
		want incidentsListArgs
	}{
		{"no flags", nil, incidentsListArgs{timeout: defaultTimeout}},
		{"page alone", []string{"--page", "2"}, incidentsListArgs{pages: []string{"2"}, timeout: defaultTimeout}},
		{"per-page alone", []string{"--per-page", "50"}, incidentsListArgs{perPages: []string{"50"}, timeout: defaultTimeout}},
		{"from alone", []string{"--from", "2024-01-01"}, incidentsListArgs{froms: []string{"2024-01-01"}, timeout: defaultTimeout}},
		{"to alone", []string{"--to", "2024-02-01"}, incidentsListArgs{tos: []string{"2024-02-01"}, timeout: defaultTimeout}},
		{"monitor-id alone", []string{"--monitor-id", "123"}, incidentsListArgs{monitorIDs: []string{"123"}, timeout: defaultTimeout}},
		{"heartbeat-id alone", []string{"--heartbeat-id", "456"}, incidentsListArgs{heartbeatIDs: []string{"456"}, timeout: defaultTimeout}},
		{"team-name alone", []string{"--team-name", "Acme"}, incidentsListArgs{teamNames: []string{"Acme"}, timeout: defaultTimeout}},
		{"team-name empty value", []string{"--team-name", ""}, incidentsListArgs{teamNames: []string{""}, timeout: defaultTimeout}},
		{"resolved space true", []string{"--resolved", "true"}, incidentsListArgs{resolveds: []string{"true"}, timeout: defaultTimeout}},
		{"resolved space false", []string{"--resolved", "false"}, incidentsListArgs{resolveds: []string{"false"}, timeout: defaultTimeout}},
		{"resolved equals true", []string{"--resolved=true"}, incidentsListArgs{resolveds: []string{"true"}, timeout: defaultTimeout}},
		{"resolved equals false", []string{"--resolved=false"}, incidentsListArgs{resolveds: []string{"false"}, timeout: defaultTimeout}},
		{"acknowledged space true", []string{"--acknowledged", "true"}, incidentsListArgs{acknowledgeds: []string{"true"}, timeout: defaultTimeout}},
		{"acknowledged equals false", []string{"--acknowledged=false"}, incidentsListArgs{acknowledgeds: []string{"false"}, timeout: defaultTimeout}},
		{"metadata simple", []string{"--metadata", "Origin=Linear"}, incidentsListArgs{metadatas: []metadataPair{{key: "Origin", value: "Linear"}}, timeout: defaultTimeout}},
		{"metadata empty value", []string{"--metadata", "Empty="}, incidentsListArgs{metadatas: []metadataPair{{key: "Empty", value: ""}}, timeout: defaultTimeout}},
		{"metadata value keeps later equals", []string{"--metadata", "Eq=a=b=c"}, incidentsListArgs{metadatas: []metadataPair{{key: "Eq", value: "a=b=c"}}, timeout: defaultTimeout}},
		{"timeout override", []string{"--timeout", "45s"}, incidentsListArgs{timeout: 45 * time.Second}},
		{
			"all flags any order",
			[]string{"--team-name", "Acme", "--resolved=false", "--page", "3", "--metadata", "Origin=Linear", "--to", "2024-02-01", "--monitor-id", "1", "--per-page", "25", "--acknowledged", "true", "--from", "2024-01-01", "--heartbeat-id", "2", "--timeout", "5s"},
			incidentsListArgs{
				pages: []string{"3"}, perPages: []string{"25"},
				froms: []string{"2024-01-01"}, tos: []string{"2024-02-01"},
				monitorIDs: []string{"1"}, heartbeatIDs: []string{"2"},
				teamNames: []string{"Acme"},
				resolveds: []string{"false"}, acknowledgeds: []string{"true"},
				metadatas: []metadataPair{{key: "Origin", value: "Linear"}},
				timeout:   5 * time.Second,
			},
		},
		{"repeated page preserves order", []string{"--page", "1", "--page", "2"}, incidentsListArgs{pages: []string{"1", "2"}, timeout: defaultTimeout}},
		{"repeated resolved preserves order", []string{"--resolved", "true", "--resolved=false"}, incidentsListArgs{resolveds: []string{"true", "false"}, timeout: defaultTimeout}},
		{"different metadata keys", []string{"--metadata", "Origin=Linear", "--metadata", "Service=api"}, incidentsListArgs{metadatas: []metadataPair{{key: "Origin", value: "Linear"}, {key: "Service", value: "api"}}, timeout: defaultTimeout}},
	}
	for _, tc := range accepted {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseIncidentsListArgs(tc.args)
			if err != nil {
				t.Fatalf("parseIncidentsListArgs(%q) error = %v", tc.args, err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("parseIncidentsListArgs(%q) = %+v, want %+v", tc.args, got, tc.want)
			}
		})
	}

	rejected := []struct {
		name string
		args []string
	}{
		{"page missing value", []string{"--page"}},
		{"per-page missing value", []string{"--per-page"}},
		{"from missing value", []string{"--from"}},
		{"to missing value", []string{"--to"}},
		{"monitor-id missing value", []string{"--monitor-id"}},
		{"heartbeat-id missing value", []string{"--heartbeat-id"}},
		{"team-name missing value", []string{"--team-name"}},
		{"resolved bare", []string{"--resolved"}},
		{"acknowledged bare", []string{"--acknowledged"}},
		{"resolved uppercase True", []string{"--resolved", "True"}},
		{"resolved uppercase TRUE", []string{"--resolved", "TRUE"}},
		{"resolved one", []string{"--resolved", "1"}},
		{"resolved zero", []string{"--resolved", "0"}},
		{"resolved yes", []string{"--resolved", "yes"}},
		{"resolved empty space", []string{"--resolved", ""}},
		{"resolved equals uppercase", []string{"--resolved=True"}},
		{"resolved equals one", []string{"--resolved=1"}},
		{"resolved equals empty", []string{"--resolved="}},
		{"acknowledged uppercase", []string{"--acknowledged", "False"}},
		{"acknowledged equals bogus", []string{"--acknowledged=bogus"}},
		{"metadata missing value", []string{"--metadata"}},
		{"metadata no equals", []string{"--metadata", "noequals"}},
		{"metadata empty key", []string{"--metadata", "=value"}},
		{"metadata equals flag form", []string{"--metadata=Origin=Linear"}},
		{"page equals form", []string{"--page=2"}},
		{"all", []string{"--all"}},
		{"pretty", []string{"--pretty"}},
		{"query", []string{"--query", "x"}},
		{"unknown flag", []string{"--bogus"}},
		{"short help", []string{"-h"}},
		{"long help", []string{"--help"}},
		{"positional", []string{"foo"}},
		{"timeout missing value", []string{"--timeout"}},
		{"timeout unparseable", []string{"--timeout", "soon"}},
		{"timeout zero", []string{"--timeout", "0s"}},
		{"timeout negative", []string{"--timeout", "-1s"}},
	}
	for _, tc := range rejected {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseIncidentsListArgs(tc.args)
			if err == nil {
				t.Fatalf("parseIncidentsListArgs(%q) = %+v, nil; want an error", tc.args, got)
			}
			if strings.TrimSpace(err.Error()) == "" {
				t.Error("error message is empty, want a readable problem")
			}
		})
	}
}

// TestDuplicateMetadataKey covers A1 (spec R1.5): the helper reports the first
// exact, case-sensitive repeated key and nothing when every key is distinct.
func TestDuplicateMetadataKey(t *testing.T) {
	cases := []struct {
		name  string
		pairs []metadataPair
		want  string
		dup   bool
	}{
		{"none", nil, "", false},
		{"unique", []metadataPair{{key: "a", value: "1"}, {key: "b", value: "2"}}, "", false},
		{"exact repeat", []metadataPair{{key: "a", value: "1"}, {key: "a", value: "2"}}, "a", true},
		{"case sensitive", []metadataPair{{key: "a", value: "1"}, {key: "A", value: "2"}}, "", false},
		{"repeat with empty value", []metadataPair{{key: "k", value: ""}, {key: "k", value: ""}}, "k", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotKey, gotDup := duplicateMetadataKey(tc.pairs)
			if gotKey != tc.want || gotDup != tc.dup {
				t.Errorf("duplicateMetadataKey(%+v) = (%q, %v), want (%q, %v)", tc.pairs, gotKey, gotDup, tc.want, tc.dup)
			}
		})
	}
}

// invokeIncidentsList calls the command directly on the injected seam.
func invokeIncidentsList(baseURL string, client *http.Client, args ...string) (code int, stdout, stderr string) {
	var out, err bytes.Buffer
	code = incidentsList(args, &out, &err, baseURL, client)
	return code, out.String(), err.String()
}

// TestIncidentsListQueryTranslation covers B1 (spec R3.1-R3.4): each flag maps
// to exactly one documented query parameter, an absent flag adds nothing, and
// repetition preserves every occurrence. The mock decodes the query so the
// assertion is independent of the encoder's byte order.
func TestIncidentsListQueryTranslation(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want map[string][]string
	}{
		{"no flags", nil, map[string][]string{}},
		{"page", []string{"--page", "2"}, map[string][]string{"page": {"2"}}},
		{"per-page", []string{"--per-page", "50"}, map[string][]string{"per_page": {"50"}}},
		{"from", []string{"--from", "2024-01-01"}, map[string][]string{"from": {"2024-01-01"}}},
		{"to", []string{"--to", "2024-02-01"}, map[string][]string{"to": {"2024-02-01"}}},
		{"monitor-id", []string{"--monitor-id", "123"}, map[string][]string{"monitor_id": {"123"}}},
		{"heartbeat-id", []string{"--heartbeat-id", "456"}, map[string][]string{"heartbeat_id": {"456"}}},
		{"team-name", []string{"--team-name", "Acme"}, map[string][]string{"team_name": {"Acme"}}},
		{"team-name empty value", []string{"--team-name", ""}, map[string][]string{"team_name": {""}}},
		{"resolved space true", []string{"--resolved", "true"}, map[string][]string{"resolved": {"true"}}},
		{"resolved equals false", []string{"--resolved=false"}, map[string][]string{"resolved": {"false"}}},
		{"acknowledged space false", []string{"--acknowledged", "false"}, map[string][]string{"acknowledged": {"false"}}},
		{"acknowledged equals true", []string{"--acknowledged=true"}, map[string][]string{"acknowledged": {"true"}}},
		{"metadata", []string{"--metadata", "Origin=Linear"}, map[string][]string{"metadata[Origin][][value]": {"Linear"}}},
		{"metadata empty value", []string{"--metadata", "Empty="}, map[string][]string{"metadata[Empty][][value]": {""}}},
		{"repeated page", []string{"--page", "1", "--page", "2"}, map[string][]string{"page": {"1", "2"}}},
		{"different metadata keys", []string{"--metadata", "Origin=Linear", "--metadata", "Service=api"}, map[string][]string{"metadata[Origin][][value]": {"Linear"}, "metadata[Service][][value]": {"api"}}},
		{
			"all filters combined",
			[]string{"--from", "2024-01-01", "--to", "2024-02-01", "--monitor-id", "1", "--heartbeat-id", "2", "--team-name", "Acme", "--resolved", "false", "--acknowledged", "true", "--metadata", "Origin=Linear", "--metadata", "Service=api", "--page", "2", "--per-page", "5"},
			map[string][]string{
				"from": {"2024-01-01"}, "to": {"2024-02-01"},
				"monitor_id": {"1"}, "heartbeat_id": {"2"}, "team_name": {"Acme"},
				"resolved": {"false"}, "acknowledged": {"true"},
				"metadata[Origin][][value]": {"Linear"}, "metadata[Service][][value]": {"api"},
				"page": {"2"}, "per_page": {"5"},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotMethod, gotPath, gotAuth string
			var gotQuery url.Values
			var requests atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				gotMethod = r.Method
				gotPath = r.URL.EscapedPath()
				gotAuth = r.Header.Get("Authorization")
				gotQuery = r.URL.Query()
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			t.Setenv(tokenEnvVar, testToken)

			code, _, stderr := invokeIncidentsList(srv.URL, newHTTPClient(), tc.args...)
			if code != 0 {
				t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr)
			}
			if got := requests.Load(); got != 1 {
				t.Fatalf("server requests = %d, want 1", got)
			}
			if gotMethod != http.MethodGet {
				t.Errorf("method = %q, want %q", gotMethod, http.MethodGet)
			}
			if gotPath != "/api/v3/incidents" {
				t.Errorf("path = %q, want %q", gotPath, "/api/v3/incidents")
			}
			if want := "Bearer " + testToken; gotAuth != want {
				t.Errorf("Authorization = %q, want %q", gotAuth, want)
			}
			if !reflect.DeepEqual(map[string][]string(gotQuery), tc.want) {
				t.Errorf("query = %#v, want %#v", map[string][]string(gotQuery), tc.want)
			}
		})
	}
}

// TestIncidentsListMetadataBracketEncoding covers B1 (spec R3.3): the nested
// metadata parameter name reaches the wire with its brackets percent-encoded
// by the stdlib query builder.
func TestIncidentsListMetadataBracketEncoding(t *testing.T) {
	var gotRawQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRawQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	t.Setenv(tokenEnvVar, testToken)

	code, _, stderr := invokeIncidentsList(srv.URL, newHTTPClient(), "--metadata", "Origin=Linear")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr)
	}
	if want := "metadata%5BOrigin%5D%5B%5D%5Bvalue%5D=Linear"; gotRawQuery != want {
		t.Errorf("raw query = %q, want %q", gotRawQuery, want)
	}
}

// TestIncidentsListRawSuccessBody covers B1 (spec R4.1, R4.2): raw 2xx bodies
// pass through byte-for-byte with no appended newline, including a pagination
// object, a non-JSON body, and a body with its own trailing newline.
func TestIncidentsListRawSuccessBody(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"pagination envelope", `{"data":[],"pagination":{"first":"https://incidents.betterstack.com/api/v3/incidents?page=1","last":"https://incidents.betterstack.com/api/v3/incidents?page=9","prev":null,"next":null}}`},
		{"non-json", "not json at all"},
		{"own trailing newline", "kept\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = io.WriteString(w, tc.body)
			}))
			defer srv.Close()

			t.Setenv(tokenEnvVar, testToken)

			code, stdout, stderr := invokeIncidentsList(srv.URL, newHTTPClient())
			if code != 0 {
				t.Fatalf("exit code = %d, want 0", code)
			}
			if stdout != tc.body {
				t.Errorf("stdout = %q, want %q", stdout, tc.body)
			}
			if stderr != "" {
				t.Errorf("stderr = %q, want empty", stderr)
			}
		})
	}
}

// TestIncidentsListDuplicateMetadataKeyRejected covers B1 (spec R1.5): a
// repeated metadata key exits 1 before any request and names the key.
func TestIncidentsListDuplicateMetadataKeyRejected(t *testing.T) {
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	t.Setenv(tokenEnvVar, testToken)

	code, stdout, stderr := invokeIncidentsList(srv.URL, newHTTPClient(), "--metadata", "Origin=Linear", "--metadata", "Origin=Other")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "Origin") {
		t.Errorf("stderr = %q, want it to name the duplicate key", stderr)
	}
	if strings.Contains(stderr, testToken) {
		t.Errorf("stderr leaks the token: %q", stderr)
	}
	if got := requests.Load(); got != 0 {
		t.Errorf("server requests = %d, want 0", got)
	}
}

// TestIncidentsListNon2xxPassthrough covers B1 (spec R5.1): every non-2xx
// status, 3xx included, passes the raw body to stderr, reaches the server
// once, follows no redirect, and writes no stdout.
func TestIncidentsListNon2xxPassthrough(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
	}{
		{"moved permanently", http.StatusMovedPermanently, "redirect body"},
		{"found", http.StatusFound, `{"errors":"moved"}`},
		{"temporary redirect", http.StatusTemporaryRedirect, "temporary"},
		{"unauthorized", http.StatusUnauthorized, `{"errors":"unauthorized"}`},
		{"not found", http.StatusNotFound, `{"errors":"not found"}`},
		{"too many requests", http.StatusTooManyRequests, "slow down"},
		{"server error", http.StatusInternalServerError, "boom"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var requests atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				if r.URL.Path == "/redirect-target" {
					w.WriteHeader(http.StatusOK)
					return
				}
				w.Header().Set("Location", "/redirect-target")
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, tc.body)
			}))
			defer srv.Close()

			t.Setenv(tokenEnvVar, testToken)

			code, stdout, stderr := invokeIncidentsList(srv.URL, newHTTPClient())
			if code != 1 {
				t.Fatalf("exit code = %d, want 1", code)
			}
			if stdout != "" {
				t.Errorf("stdout = %q, want empty", stdout)
			}
			if stderr != tc.body {
				t.Errorf("stderr = %q, want raw body %q", stderr, tc.body)
			}
			if got := requests.Load(); got != 1 {
				t.Errorf("server requests = %d, want 1 (no redirect follow, no retry)", got)
			}
		})
	}
}

// TestIncidentsListTimeoutOverride covers B1 (spec R6.1): a short override
// expires with one request; a longer override succeeds.
func TestIncidentsListTimeoutOverride(t *testing.T) {
	t.Run("expires", func(t *testing.T) {
		var requests atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests.Add(1)
			time.Sleep(150 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		t.Setenv(tokenEnvVar, testToken)

		code, stdout, stderr := invokeIncidentsList(srv.URL, newHTTPClient(), "--timeout", "10ms")
		if code != 1 {
			t.Fatalf("exit code = %d, want 1", code)
		}
		if stdout != "" {
			t.Errorf("stdout = %q, want empty", stdout)
		}
		if strings.TrimSpace(stderr) == "" {
			t.Error("stderr is empty, want a readable error")
		}
		if got := requests.Load(); got != 1 {
			t.Errorf("server requests = %d, want 1", got)
		}
	})

	t.Run("within override", func(t *testing.T) {
		const body = "ok body"
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, body)
		}))
		defer srv.Close()

		t.Setenv(tokenEnvVar, testToken)

		code, stdout, stderr := invokeIncidentsList(srv.URL, newHTTPClient(), "--timeout", "5s")
		if code != 0 {
			t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr)
		}
		if stdout != body {
			t.Errorf("stdout = %q, want %q", stdout, body)
		}
		if stderr != "" {
			t.Errorf("stderr = %q, want empty", stderr)
		}
	})
}

// TestIncidentsListInvalidTimeout covers B1 (spec R6.1): invalid durations
// fail before any request.
func TestIncidentsListInvalidTimeout(t *testing.T) {
	cases := [][]string{
		{"--timeout"},
		{"--timeout", "soon"},
		{"--timeout", "0s"},
		{"--timeout", "-1s"},
	}
	for _, args := range cases {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var requests atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			t.Setenv(tokenEnvVar, testToken)

			code, stdout, stderr := invokeIncidentsList(srv.URL, newHTTPClient(), args...)
			if code != 1 {
				t.Fatalf("exit code = %d, want 1", code)
			}
			if stdout != "" {
				t.Errorf("stdout = %q, want empty", stdout)
			}
			if strings.TrimSpace(stderr) == "" {
				t.Error("stderr is empty, want a readable error")
			}
			if got := requests.Load(); got != 0 {
				t.Errorf("server requests = %d, want 0", got)
			}
		})
	}
}

// TestIncidentsListLocalFailureIsTokenFree covers B1 (spec R5.2, R5.3): a
// closed local port exits 1 with one concise token-free message.
func TestIncidentsListLocalFailureIsTokenFree(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	closedURL := srv.URL
	srv.Close()

	t.Setenv(tokenEnvVar, testToken)

	code, stdout, stderr := invokeIncidentsList(closedURL, newHTTPClient())
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
	if strings.TrimSpace(stderr) == "" {
		t.Error("stderr is empty, want a readable error")
	}
	if strings.Contains(stderr, testToken) {
		t.Errorf("stderr leaks the token: %q", stderr)
	}
}

// TestIncidentsListRoutesThroughRunWithAPI covers B2 (spec R1.1, R8.1): the
// routed vector reaches the injected mock once at the v3 incidents path,
// proving main's incidents list branch is wired ahead of the old
// not-implemented fallthrough.
func TestIncidentsListRoutesThroughRunWithAPI(t *testing.T) {
	var requests atomic.Int32
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		gotPath = r.URL.EscapedPath()
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"data":[]}`)
	}))
	defer srv.Close()

	t.Setenv(tokenEnvVar, testToken)

	code, stdout, stderr := invokeAPI(srv.URL, newHTTPClient(), "incidents", "list")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr)
	}
	if stdout != `{"data":[]}` {
		t.Errorf("stdout = %q, want the raw body", stdout)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty", stderr)
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("server requests = %d, want 1", got)
	}
	if want := "/api/v3/incidents"; gotPath != want {
		t.Errorf("path = %q, want %q", gotPath, want)
	}
}

// TestIncidentsListUsageFailuresThroughRouting covers B2 (spec R1.1, R1.7):
// positional and unknown-option vectors exit 1 with a readable error and zero
// requests through the new command's parser, replacing the old shared list
// rejection.
func TestIncidentsListUsageFailuresThroughRouting(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"positional", []string{"incidents", "list", "extra"}},
		{"unknown option", []string{"incidents", "list", "--bogus"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var requests atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			t.Setenv(tokenEnvVar, testToken)

			code, stdout, stderr := invokeAPI(srv.URL, newHTTPClient(), tc.args...)
			if code != 1 {
				t.Fatalf("exit code = %d, want 1", code)
			}
			if stdout != "" {
				t.Errorf("stdout = %q, want empty", stdout)
			}
			if strings.TrimSpace(stderr) == "" {
				t.Error("stderr is empty, want a readable error")
			}
			if strings.Contains(stderr, testToken) {
				t.Errorf("stderr leaks the token: %q", stderr)
			}
			if got := requests.Load(); got != 0 {
				t.Errorf("server requests = %d, want 0", got)
			}
		})
	}
}

// TestIncidentsListRoutesBearerTokenThroughRunWithAPI covers B3 (spec R3.1,
// R3.2): the routed command sends exactly one bearer-authenticated GET with
// the given flags encoded in the query string.
func TestIncidentsListRoutesBearerTokenThroughRunWithAPI(t *testing.T) {
	var requests atomic.Int32
	var gotAuth, gotPath, gotRawQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.EscapedPath()
		gotRawQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"data":[]}`)
	}))
	defer srv.Close()

	t.Setenv(tokenEnvVar, testToken)

	code, stdout, stderr := invokeAPI(srv.URL, newHTTPClient(), "incidents", "list", "--monitor-id", "123", "--resolved=false")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr)
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("server requests = %d, want 1", got)
	}
	if want := "Bearer " + testToken; gotAuth != want {
		t.Errorf("Authorization = %q, want %q", gotAuth, want)
	}
	if gotPath != "/api/v3/incidents" {
		t.Errorf("path = %q, want /api/v3/incidents", gotPath)
	}
	if want := "monitor_id=123&resolved=false"; gotRawQuery != want {
		t.Errorf("query = %q, want %q", gotRawQuery, want)
	}
	if stdout != `{"data":[]}` || stderr != "" {
		t.Errorf("stdout = %q, stderr = %q; want the raw body and empty stderr", stdout, stderr)
	}
}

// TestIncidentsListCredentialFailuresThroughRouting covers B3 (spec R2.1):
// unset, empty, and former-name-only each fail before any request naming
// BETTERSTACK_UPTIME_API_TOKEN.
func TestIncidentsListCredentialFailuresThroughRouting(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T)
	}{
		{"unset", func(t *testing.T) {
			t.Setenv(tokenEnvVar, "placeholder")
			if err := os.Unsetenv(tokenEnvVar); err != nil {
				t.Fatalf("Unsetenv: %v", err)
			}
		}},
		{"empty", func(t *testing.T) { t.Setenv(tokenEnvVar, "") }},
		{"former name only", func(t *testing.T) {
			t.Setenv("BETTERSTACK_API_TOKEN", testToken)
			t.Setenv(tokenEnvVar, "placeholder")
			if err := os.Unsetenv(tokenEnvVar); err != nil {
				t.Fatalf("Unsetenv: %v", err)
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var requests atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			tc.setup(t)

			code, stdout, stderr := invokeAPI(srv.URL, newHTTPClient(), "incidents", "list")
			if code != 1 {
				t.Fatalf("exit code = %d, want 1", code)
			}
			if stdout != "" {
				t.Errorf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, tokenEnvVar) {
				t.Errorf("stderr = %q, want it to name %s", stderr, tokenEnvVar)
			}
			if got := requests.Load(); got != 0 {
				t.Errorf("server requests = %d, want 0", got)
			}
		})
	}
}

// TestIncidentsListNeverPrintsTokenThroughRouting covers B3 (spec R5.3): the
// routed command keeps the token off both channels on success, non-2xx, and
// local failure.
func TestIncidentsListNeverPrintsTokenThroughRouting(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		var gotAuth string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotAuth = r.Header.Get("Authorization")
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, `{"data":[]}`)
		}))
		defer srv.Close()

		t.Setenv(tokenEnvVar, testToken)

		code, stdout, stderr := invokeAPI(srv.URL, newHTTPClient(), "incidents", "list")
		if code != 0 {
			t.Fatalf("exit code = %d, want 0", code)
		}
		if want := "Bearer " + testToken; gotAuth != want {
			t.Errorf("Authorization = %q, want %q", gotAuth, want)
		}
		assertNoToken(t, stdout, stderr)
	})

	t.Run("non-2xx", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = io.WriteString(w, `{"errors":"unauthorized"}`)
		}))
		defer srv.Close()

		t.Setenv(tokenEnvVar, testToken)

		code, stdout, stderr := invokeAPI(srv.URL, newHTTPClient(), "incidents", "list")
		if code != 1 {
			t.Fatalf("exit code = %d, want 1", code)
		}
		assertNoToken(t, stdout, stderr)
	})

	t.Run("local failure", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		closedURL := srv.URL
		srv.Close()

		t.Setenv(tokenEnvVar, testToken)

		code, stdout, stderr := invokeAPI(closedURL, newHTTPClient(), "incidents", "list")
		if code != 1 {
			t.Fatalf("exit code = %d, want 1", code)
		}
		assertNoToken(t, stdout, stderr)
	})
}

// TestIncidentsListBooleanFormsThroughRouting covers B3 (spec R1.3): both the
// space and equals boolean forms reach the query string for both flags.
func TestIncidentsListBooleanFormsThroughRouting(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"resolved space true", []string{"incidents", "list", "--resolved", "true"}, "resolved=true"},
		{"resolved space false", []string{"incidents", "list", "--resolved", "false"}, "resolved=false"},
		{"resolved equals true", []string{"incidents", "list", "--resolved=true"}, "resolved=true"},
		{"resolved equals false", []string{"incidents", "list", "--resolved=false"}, "resolved=false"},
		{"acknowledged space true", []string{"incidents", "list", "--acknowledged", "true"}, "acknowledged=true"},
		{"acknowledged equals false", []string{"incidents", "list", "--acknowledged=false"}, "acknowledged=false"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotRawQuery string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotRawQuery = r.URL.RawQuery
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			t.Setenv(tokenEnvVar, testToken)

			code, _, stderr := invokeAPI(srv.URL, newHTTPClient(), tc.args...)
			if code != 0 {
				t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr)
			}
			if gotRawQuery != tc.want {
				t.Errorf("query = %q, want %q", gotRawQuery, tc.want)
			}
		})
	}
}

// TestIncidentsListBooleanRejectionsThroughRouting covers B3 (spec R1.3): a
// bare flag and every non-lowercase-true/false value exit 1 with zero requests.
func TestIncidentsListBooleanRejectionsThroughRouting(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"bare resolved", []string{"incidents", "list", "--resolved"}},
		{"bare acknowledged", []string{"incidents", "list", "--acknowledged"}},
		{"resolved True", []string{"incidents", "list", "--resolved", "True"}},
		{"resolved TRUE", []string{"incidents", "list", "--resolved", "TRUE"}},
		{"resolved 1", []string{"incidents", "list", "--resolved", "1"}},
		{"resolved 0", []string{"incidents", "list", "--resolved", "0"}},
		{"resolved yes", []string{"incidents", "list", "--resolved", "yes"}},
		{"resolved empty", []string{"incidents", "list", "--resolved", ""}},
		{"resolved equals empty", []string{"incidents", "list", "--resolved="}},
		{"acknowledged False", []string{"incidents", "list", "--acknowledged", "False"}},
		{"acknowledged equals bogus", []string{"incidents", "list", "--acknowledged=bogus"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var requests atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			t.Setenv(tokenEnvVar, testToken)

			code, stdout, stderr := invokeAPI(srv.URL, newHTTPClient(), tc.args...)
			if code != 1 {
				t.Fatalf("exit code = %d, want 1", code)
			}
			if stdout != "" {
				t.Errorf("stdout = %q, want empty", stdout)
			}
			if strings.TrimSpace(stderr) == "" {
				t.Error("stderr is empty, want a readable error")
			}
			if got := requests.Load(); got != 0 {
				t.Errorf("server requests = %d, want 0", got)
			}
		})
	}
}

// TestIncidentsListMetadataThroughRouting covers B3 (spec R1.4, R1.5): distinct
// metadata keys are accepted with both nested parameters on the wire, and a
// duplicate key is rejected before any request.
func TestIncidentsListMetadataThroughRouting(t *testing.T) {
	t.Run("distinct keys accepted", func(t *testing.T) {
		var gotQuery url.Values
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotQuery = r.URL.Query()
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		t.Setenv(tokenEnvVar, testToken)

		code, _, stderr := invokeAPI(srv.URL, newHTTPClient(), "incidents", "list", "--metadata", "Origin=Linear", "--metadata", "Service=api")
		if code != 0 {
			t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr)
		}
		if got := gotQuery["metadata[Origin][][value]"]; len(got) != 1 || got[0] != "Linear" {
			t.Errorf("Origin parameter = %#v, want [Linear]", got)
		}
		if got := gotQuery["metadata[Service][][value]"]; len(got) != 1 || got[0] != "api" {
			t.Errorf("Service parameter = %#v, want [api]", got)
		}
	})

	t.Run("duplicate key rejected", func(t *testing.T) {
		var requests atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests.Add(1)
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		t.Setenv(tokenEnvVar, testToken)

		code, stdout, stderr := invokeAPI(srv.URL, newHTTPClient(), "incidents", "list", "--metadata", "Origin=Linear", "--metadata", "Origin=Other")
		if code != 1 {
			t.Fatalf("exit code = %d, want 1", code)
		}
		if stdout != "" {
			t.Errorf("stdout = %q, want empty", stdout)
		}
		if !strings.Contains(stderr, "Origin") {
			t.Errorf("stderr = %q, want it to name the duplicate key", stderr)
		}
		if got := requests.Load(); got != 0 {
			t.Errorf("server requests = %d, want 0", got)
		}
	})
}

// TestIncidentsListHelpExitsZeroThroughRouting covers B3 (spec R1.7): the
// unchanged incidentsListHelp text exits 0 with empty stderr and no request.
func TestIncidentsListHelpExitsZeroThroughRouting(t *testing.T) {
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	t.Setenv(tokenEnvVar, testToken)

	code, stdout, stderr := invokeAPI(srv.URL, newHTTPClient(), "incidents", "list", "--help")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stdout != incidentsListHelp {
		t.Errorf("stdout = %q, want incidentsListHelp", stdout)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty", stderr)
	}
	if got := requests.Load(); got != 0 {
		t.Errorf("server requests = %d, want 0", got)
	}
}

// TestIncidentsListRejectsAbsentFeaturesThroughRouting covers B3 (spec R1.7,
// R7): unknown options and non-boolean equals forms exit 1 with zero requests.
func TestIncidentsListRejectsAbsentFeaturesThroughRouting(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"--all", []string{"incidents", "list", "--all"}},
		{"--pretty", []string{"incidents", "list", "--pretty"}},
		{"--query", []string{"incidents", "list", "--query", "x"}},
		{"unknown flag", []string{"incidents", "list", "--bogus"}},
		{"page equals form", []string{"incidents", "list", "--page=2"}},
		{"metadata equals form", []string{"incidents", "list", "--metadata=Origin=Linear"}},
		{"short help", []string{"incidents", "list", "-h"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var requests atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			t.Setenv(tokenEnvVar, testToken)

			code, stdout, stderr := invokeAPI(srv.URL, newHTTPClient(), tc.args...)
			if code != 1 {
				t.Fatalf("exit code = %d, want 1", code)
			}
			if stdout != "" {
				t.Errorf("stdout = %q, want empty", stdout)
			}
			if strings.TrimSpace(stderr) == "" {
				t.Error("stderr is empty, want a readable error")
			}
			if strings.Contains(stderr, testToken) {
				t.Errorf("stderr leaks the token: %q", stderr)
			}
			if got := requests.Load(); got != 0 {
				t.Errorf("server requests = %d, want 0", got)
			}
		})
	}
}

// TestIncidentsListTimeoutThroughRouting covers B3 (spec R6.1): a short
// override expires with one request; a longer override succeeds; an invalid
// value fails before any request.
func TestIncidentsListTimeoutThroughRouting(t *testing.T) {
	t.Run("expires", func(t *testing.T) {
		var requests atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests.Add(1)
			time.Sleep(150 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		t.Setenv(tokenEnvVar, testToken)

		code, stdout, stderr := invokeAPI(srv.URL, newHTTPClient(), "incidents", "list", "--timeout", "10ms")
		if code != 1 {
			t.Fatalf("exit code = %d, want 1", code)
		}
		if stdout != "" {
			t.Errorf("stdout = %q, want empty", stdout)
		}
		if strings.TrimSpace(stderr) == "" {
			t.Error("stderr is empty, want a readable error")
		}
		if got := requests.Load(); got != 1 {
			t.Errorf("server requests = %d, want 1", got)
		}
	})

	t.Run("within override", func(t *testing.T) {
		const body = "ok body"
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, body)
		}))
		defer srv.Close()

		t.Setenv(tokenEnvVar, testToken)

		code, stdout, stderr := invokeAPI(srv.URL, newHTTPClient(), "incidents", "list", "--timeout", "5s")
		if code != 0 {
			t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr)
		}
		if stdout != body {
			t.Errorf("stdout = %q, want %q", stdout, body)
		}
		if stderr != "" {
			t.Errorf("stderr = %q, want empty", stderr)
		}
	})

	t.Run("invalid value", func(t *testing.T) {
		var requests atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests.Add(1)
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		t.Setenv(tokenEnvVar, testToken)

		code, stdout, stderr := invokeAPI(srv.URL, newHTTPClient(), "incidents", "list", "--timeout", "soon")
		if code != 1 {
			t.Fatalf("exit code = %d, want 1", code)
		}
		if stdout != "" {
			t.Errorf("stdout = %q, want empty", stdout)
		}
		if strings.TrimSpace(stderr) == "" {
			t.Error("stderr is empty, want a readable error")
		}
		if got := requests.Load(); got != 0 {
			t.Errorf("server requests = %d, want 0", got)
		}
	})
}
