package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// TestSourcesRoutedRawResponses verifies unchanged bytes and no redirect or pagination requests.
func TestSourcesRoutedRawResponses(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
	}{
		{"non-JSON", 200, "raw\x00body"},
		{"JSON", 201, ` { "data": [{"token":"dummy-source-secret"}], "pagination":{"next":"ignored"} } `},
		{"newline", 299, "body\n"},
		{"empty", 204, ""},
		{"redirect", 302, `{"token":"dummy-source-secret","error":"moved"}`},
		{"unauthorized", 401, "denied\n"},
		{"not found", 404, ` {"token":"dummy-source-secret","errors":"not found"} `},
		{"rate limit", 429, "slow down"},
		{"server error", 500, "failure\x00"},
	}
	for _, args := range [][]string{{"sources", "list"}, {"sources", "get", "opaque"}} {
		for _, tc := range cases {
			t.Run(strings.Join(args, " ")+"/"+tc.name, func(t *testing.T) {
				var requests, redirects atomic.Int32
				target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					redirects.Add(1)
					_, _ = io.WriteString(w, "followed")
				}))
				defer target.Close()
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					requests.Add(1)
					w.Header().Set("Location", target.URL)
					w.WriteHeader(tc.status)
					_, _ = io.WriteString(w, tc.body)
				}))
				defer srv.Close()
				t.Setenv(telemetryTokenEnvVar, "dummy-telemetry-token")
				var out, err bytes.Buffer
				code := runWithEndpoints(args, &out, &err, target.URL+"/uptime", srv.URL, target.URL+"/sql", testHTTPClient())
				wantCode, wantOut, wantErr := 1, "", tc.body
				if tc.status >= 200 && tc.status <= 299 {
					wantCode, wantOut, wantErr = 0, tc.body, ""
				}
				if code != wantCode || out.String() != wantOut || err.String() != wantErr {
					t.Fatalf("raw response mismatch: code=%d stdout matches=%t stderr matches=%t", code, out.String() == wantOut, err.String() == wantErr)
				}
				if requests.Load() != 1 || redirects.Load() != 0 {
					t.Fatalf("requests=%d redirect/other endpoint requests=%d", requests.Load(), redirects.Load())
				}
			})
		}
	}
}

// TestSourcesRoutedLocalFailures verifies the shared transport, timeout, and body-read diagnostics.
func TestSourcesRoutedLocalFailures(t *testing.T) {
	for _, command := range [][]string{{"sources", "list"}, {"sources", "get", "dummy-telemetry-token"}} {
		for _, failure := range []string{"transport", "timeout", "body read"} {
			t.Run(strings.Join(command, " ")+"/"+failure, func(t *testing.T) {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if failure == "timeout" {
						<-r.Context().Done()
						return
					}
					w.Header().Set("Content-Length", "100")
					_, _ = io.WriteString(w, "partial")
				}))
				defer srv.Close()
				if failure == "transport" {
					srv.Close()
				}
				t.Setenv(telemetryTokenEnvVar, "dummy-telemetry-token")
				args := append(append([]string{}, command...), "--timeout", "100ms")
				var out, err bytes.Buffer
				code := runWithEndpoints(args, &out, &err, srv.URL, srv.URL, srv.URL, testHTTPClient())
				want := "request to the Better Stack API failed"
				if failure == "body read" {
					want = "could not read the API response"
				}
				if code != 1 || out.Len() != 0 || !strings.Contains(err.String(), want) || strings.Contains(err.String(), "dummy-telemetry-token") {
					t.Fatalf("local %s failure did not preserve the shared diagnostic contract", failure)
				}
			})
		}
	}
}

// TestParseSourcesListArgs verifies pagination queries and default or repeated timeouts.
func TestParseSourcesListArgs(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		query   string
		timeout time.Duration
	}{
		{"omitted", nil, "", 30 * time.Second},
		{"minimum", []string{"--page", "1", "--per-page", "1"}, "page=1&per_page=1", 30 * time.Second},
		{"maximum", []string{"--per-page", "50"}, "per_page=50", 30 * time.Second},
		{"repeated", []string{"--page", "1", "--page", "2", "--per-page", "1", "--per-page", "50"}, "page=1&page=2&per_page=1&per_page=50", 30 * time.Second},
		{"decimal", []string{"--page", "0001", "--per-page", "050"}, "page=0001&per_page=050", 30 * time.Second},
		{"large page", []string{"--page", "9999999999999999999999999999999999999999"}, "page=9999999999999999999999999999999999999999", 30 * time.Second},
		{"timeout", []string{"--timeout", "1m", "--timeout", "250ms"}, "", 250 * time.Millisecond},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			parsed, err := parseSourcesListArgs(tc.args)
			if err != nil {
				t.Fatal(err)
			}
			if parsed.query().Encode() != tc.query || parsed.timeout != tc.timeout {
				t.Fatalf("query = %q, timeout = %s", parsed.query().Encode(), parsed.timeout)
			}
		})
	}
}

func invalidSourcesListArgs() [][]string {
	var cases [][]string
	for _, flag := range []string{"--page", "--per-page"} {
		for _, value := range []string{"", "0", "000", "-1", "+1", " 1", "1 ", "1.0", "1e1", "a", "١"} {
			cases = append(cases, []string{flag, value})
		}
		cases = append(cases, []string{flag}, []string{flag + "=1"})
	}
	cases = append(cases, []string{"--per-page", "51"}, []string{"--per-page", "9999999999999999999999999"},
		[]string{"--page", "0", "--page", "1"}, []string{"--per-page", "51", "--per-page", "1"},
		[]string{"--url", "x"}, []string{"--unknown"}, []string{"positional"}, []string{""})
	for _, value := range []string{"", "bad", "0s", "-1s"} {
		cases = append(cases, []string{"--timeout", value}, []string{"--timeout", value, "--timeout", "1s"})
	}
	return append(cases, []string{"--timeout"}, []string{"--timeout=1s"})
}

// TestParseSourcesListArgsRejectsInvalid verifies strict numeric and option syntax.
func TestParseSourcesListArgsRejectsInvalid(t *testing.T) {
	for _, args := range invalidSourcesListArgs() {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			_, err := parseSourcesListArgs(args)
			if err == nil || !strings.Contains(err.Error(), args[0]) {
				t.Fatalf("error = %v, want flag-specific diagnostic", err)
			}
		})
	}
}

func invokeSources(handler func([]string, io.Writer, io.Writer, string, *http.Client) int, baseURL string, args ...string) (int, string, string) {
	var out, err bytes.Buffer
	code := handler(args, &out, &err, baseURL, testHTTPClient())
	return code, out.String(), err.String()
}

// TestSourcesListQueryAndAuth verifies omitted and repeated queries with Telemetry-only auth.
func TestSourcesListQueryAndAuth(t *testing.T) {
	t.Setenv(telemetryTokenEnvVar, "dummy-telemetry-token")
	t.Setenv(tokenEnvVar, testToken)
	cases := []struct {
		args  []string
		query map[string][]string
	}{
		{nil, map[string][]string{}},
		{[]string{"--page", "1", "--page", "2", "--per-page", "1", "--per-page", "50", "--timeout", "1s"}, map[string][]string{"page": {"1", "2"}, "per_page": {"1", "50"}}},
	}
	for _, tc := range cases {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			var requests atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				if r.Method != http.MethodGet || r.URL.Path != "/api/v2/sources" || !reflect.DeepEqual(map[string][]string(r.URL.Query()), tc.query) {
					t.Errorf("request = %s %s", r.Method, r.URL.RequestURI())
				}
				if r.Header.Get("Authorization") != "Bearer dummy-telemetry-token" {
					t.Error("request did not use the dummy Telemetry credential")
				}
				_, _ = io.WriteString(w, "body")
			}))
			defer srv.Close()
			code, out, err := invokeSources(sourcesList, srv.URL+"/", tc.args...)
			if code != 0 || out != "body" || err != "" || requests.Load() != 1 {
				t.Fatalf("code = %d, expected output = %t, stderr empty = %t, requests = %d", code, out == "body", err == "", requests.Load())
			}
		})
	}
}

// TestSourcesListInvalidArgumentsBeforeCredentials verifies local validation and no requests.
func TestSourcesListInvalidArgumentsBeforeCredentials(t *testing.T) {
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests.Add(1) }))
	defer srv.Close()
	for _, token := range []string{"", "dummy-telemetry-token"} {
		t.Setenv(telemetryTokenEnvVar, token)
		for _, args := range invalidSourcesListArgs() {
			t.Run(strings.Join(args, " "), func(t *testing.T) {
				code, out, err := invokeSources(sourcesList, srv.URL, args...)
				if code != 1 || out != "" || !strings.Contains(err, "bs sources list --help") || !strings.Contains(err, args[0]) || strings.Contains(err, "is not set") {
					t.Fatalf("invalid argument did not return source usage before credentials: code=%d", code)
				}
			})
		}
	}
	if requests.Load() != 0 {
		t.Fatalf("requests = %d, want 0", requests.Load())
	}
}

// TestParseSourcesGetArgs verifies opaque IDs and timeout placement or repetition.
func TestParseSourcesGetArgs(t *testing.T) {
	cases := []struct {
		args    []string
		id      string
		timeout time.Duration
	}{
		{[]string{"opaque"}, "opaque", 30 * time.Second},
		{[]string{"a b/c%?x#y"}, "a b/c%?x#y", 30 * time.Second},
		{[]string{"https://example.com/a?x#y"}, "https://example.com/a?x#y", 30 * time.Second},
		{[]string{"--timeout", "1m", "id"}, "id", time.Minute},
		{[]string{"id", "--timeout", "250ms"}, "id", 250 * time.Millisecond},
		{[]string{"--timeout", "1m", "id", "--timeout", "250ms"}, "id", 250 * time.Millisecond},
	}
	for _, tc := range cases {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			id, timeout, err := parseSourcesGetArgs(tc.args)
			if err != nil || id != tc.id || timeout != tc.timeout {
				t.Fatalf("id = %q, timeout = %s, error = %v", id, timeout, err)
			}
		})
	}
}

func invalidSourcesGetArgs() [][]string {
	cases := [][]string{nil, {""}, {"id", "extra"}, {"", "id"}, {"id", ""}, {"--unknown"}, {"--timeout"}, {"id", "--timeout"}, {"id", "--timeout=1s"}, {"--timeout", "1s"}, {"-id"}}
	for _, value := range []string{"", "bad", "0s", "-1s"} {
		cases = append(cases, []string{"id", "--timeout", value}, []string{"--timeout", value, "id", "--timeout", "1s"})
	}
	return cases
}

// TestParseSourcesGetArgsRejectsInvalid verifies positional and option syntax failures.
func TestParseSourcesGetArgsRejectsInvalid(t *testing.T) {
	for _, args := range invalidSourcesGetArgs() {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			if _, _, err := parseSourcesGetArgs(args); err == nil {
				t.Fatal("invalid source get arguments accepted")
			}
		})
	}
}

// TestSourcesGetEscapedOpaqueIDAndAuth verifies literal path boundaries and Telemetry auth.
func TestSourcesGetEscapedOpaqueIDAndAuth(t *testing.T) {
	t.Setenv(telemetryTokenEnvVar, "dummy-telemetry-token")
	t.Setenv(tokenEnvVar, testToken)
	cases := []struct {
		id   string
		path string
	}{
		{"opaque", "/api/v2/sources/opaque"},
		{"a b/c%?x#y", "/api/v2/sources/a%20b%2Fc%25%3Fx%23y"},
		{"https://example.com/a?x#y", "/api/v2/sources/https:%2F%2Fexample.com%2Fa%3Fx%23y"},
		{"%2F", "/api/v2/sources/%252F"},
	}
	for _, tc := range cases {
		t.Run(tc.id, func(t *testing.T) {
			var requests atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				if r.Method != http.MethodGet || r.URL.EscapedPath() != tc.path || r.URL.RawQuery != "" {
					t.Errorf("request = %s %s, want GET %s", r.Method, r.URL.RequestURI(), tc.path)
				}
				if r.Header.Get("Authorization") != "Bearer dummy-telemetry-token" {
					t.Error("request did not use the dummy Telemetry credential")
				}
				_, _ = io.WriteString(w, "source")
			}))
			defer srv.Close()
			code, out, err := invokeSources(sourcesGet, srv.URL+"/", "--timeout", "1s", tc.id, "--timeout", "2s")
			if code != 0 || out != "source" || err != "" || requests.Load() != 1 {
				t.Fatalf("code = %d, expected output = %t, stderr empty = %t, requests = %d", code, out == "source", err == "", requests.Load())
			}
		})
	}
}

// TestSourcesGetInvalidArgumentsBeforeCredentials verifies local validation and no requests.
func TestSourcesGetInvalidArgumentsBeforeCredentials(t *testing.T) {
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests.Add(1) }))
	defer srv.Close()
	for _, token := range []string{"", "dummy-telemetry-token"} {
		t.Setenv(telemetryTokenEnvVar, token)
		for _, args := range invalidSourcesGetArgs() {
			t.Run(strings.Join(args, " "), func(t *testing.T) {
				code, out, err := invokeSources(sourcesGet, srv.URL, args...)
				if code != 1 || out != "" || !strings.Contains(err, "bs sources get --help") || strings.Contains(err, "is not set") {
					t.Fatalf("invalid argument did not return source usage before credentials: code=%d", code)
				}
			})
		}
	}
	if requests.Load() != 0 {
		t.Fatalf("requests = %d, want 0", requests.Load())
	}
}

// TestSourcesGetMissingTelemetryToken verifies unset and empty tokens without credential fallback.
func TestSourcesGetMissingTelemetryToken(t *testing.T) {
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests.Add(1) }))
	defer srv.Close()
	t.Setenv(tokenEnvVar, testToken)
	t.Setenv("BETTERSTACK_API_TOKEN", "dummy-legacy-token")
	for _, unset := range []bool{false, true} {
		t.Setenv(telemetryTokenEnvVar, "")
		if unset {
			if err := os.Unsetenv(telemetryTokenEnvVar); err != nil {
				t.Fatal(err)
			}
		}
		code, out, err := invokeSources(sourcesGet, srv.URL, "opaque")
		if code != 1 || out != "" || !strings.Contains(err, telemetryTokenEnvVar) || strings.Contains(err, testToken) {
			t.Fatal("missing Telemetry credential did not fail locally without leaking the Uptime token")
		}
	}
	if requests.Load() != 0 {
		t.Fatalf("requests = %d, want 0", requests.Load())
	}
}

// TestSourcesListMissingTelemetryToken verifies unset and empty tokens without credential fallback.
func TestSourcesListMissingTelemetryToken(t *testing.T) {
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests.Add(1) }))
	defer srv.Close()
	t.Setenv(tokenEnvVar, testToken)
	t.Setenv("BETTERSTACK_API_TOKEN", "dummy-legacy-token")
	for _, unset := range []bool{false, true} {
		t.Setenv(telemetryTokenEnvVar, "")
		if unset {
			if err := os.Unsetenv(telemetryTokenEnvVar); err != nil {
				t.Fatal(err)
			}
		}
		code, out, err := invokeSources(sourcesList, srv.URL)
		if code != 1 || out != "" || !strings.Contains(err, telemetryTokenEnvVar) || strings.Contains(err, testToken) {
			t.Fatal("missing Telemetry credential did not fail locally without leaking the Uptime token")
		}
	}
	if requests.Load() != 0 {
		t.Fatalf("requests = %d, want 0", requests.Load())
	}
}
