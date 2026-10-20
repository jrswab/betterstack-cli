package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// invoke runs one CLI argument vector through the argv seam and returns the
// exit code plus both output channels as strings.
func invoke(args ...string) (code int, stdout, stderr string) {
	var out, err bytes.Buffer
	code = run(args, &out, &err)
	return code, out.String(), err.String()
}

// TestRunWithAPIKeepsSignatureAndUptimeRouting preserves the Uptime injection
// interface and raw response behavior against a guarded loopback server.
func TestRunWithAPIKeepsSignatureAndUptimeRouting(t *testing.T) {
	var route func([]string, io.Writer, io.Writer, string, *http.Client) int = runWithAPI
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.Method != http.MethodGet || r.URL.Path != "/api/v2/monitors/789" || r.Header.Get("Authorization") != "Bearer "+testToken {
			t.Errorf("request = %s %s, Authorization = %q", r.Method, r.URL.Path, r.Header.Get("Authorization"))
		}
		_, _ = io.WriteString(w, "raw uptime body\n")
	}))
	defer srv.Close()
	t.Setenv(tokenEnvVar, testToken)

	var out, err bytes.Buffer
	code := route([]string{"monitors", "get", "789"}, &out, &err, srv.URL, testHTTPClient())
	if code != 0 || out.String() != "raw uptime body\n" || err.String() != "" || requests.Load() != 1 {
		t.Fatalf("code = %d, stdout = %q, stderr = %q, requests = %d", code, out.String(), err.String(), requests.Load())
	}
}

// TestRunWithEndpointsUsesDistinctEndpoints routes Uptime commands only to
// the Uptime input when all three endpoint inputs are distinct.
func TestRunWithEndpointsUsesDistinctEndpoints(t *testing.T) {
	var uptimeRequests, telemetryRequests atomic.Int32
	uptime := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uptimeRequests.Add(1)
		if r.Method != http.MethodGet || r.Header.Get("Authorization") != "Bearer "+testToken {
			t.Errorf("method = %q, Authorization = %q", r.Method, r.Header.Get("Authorization"))
		}
		_, _ = io.WriteString(w, r.URL.Path)
	}))
	defer uptime.Close()
	rejectTelemetry := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		telemetryRequests.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	})
	rest := httptest.NewServer(rejectTelemetry)
	defer rest.Close()
	sql := httptest.NewServer(rejectTelemetry)
	defer sql.Close()
	t.Setenv(tokenEnvVar, testToken)

	cases := []struct {
		args []string
		path string
	}{
		{[]string{"monitors", "list"}, "/api/v2/monitors"},
		{[]string{"monitors", "get", "789"}, "/api/v2/monitors/789"},
		{[]string{"incidents", "list"}, "/api/v3/incidents"},
		{[]string{"incidents", "get", "123"}, "/api/v3/incidents/123"},
		{[]string{"incidents", "timeline", "123"}, "/api/v3/incidents/123/timeline"},
	}
	for _, tc := range cases {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			var out, err bytes.Buffer
			code := runWithEndpoints(tc.args, &out, &err, uptime.URL, rest.URL, sql.URL+"/query", testHTTPClient())
			if code != 0 || out.String() != tc.path || err.String() != "" {
				t.Fatalf("code = %d, stdout = %q, stderr = %q, want path %q", code, out.String(), err.String(), tc.path)
			}
		})
	}
	if uptimeRequests.Load() != int32(len(cases)) || telemetryRequests.Load() != 0 {
		t.Fatalf("Uptime requests = %d, Telemetry requests = %d", uptimeRequests.Load(), telemetryRequests.Load())
	}
}

// TestRunWithEndpointsAndRunWithAPIMakeNoRequestForHelpOrInvalidUsage checks
// compatibility and zero requests for local help and invalid arguments.
func TestRunWithEndpointsAndRunWithAPIMakeNoRequestForHelpOrInvalidUsage(t *testing.T) {
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
	}))
	defer srv.Close()
	t.Setenv(tokenEnvVar, testToken)

	cases := []struct {
		name string
		args []string
		code int
	}{
		{"root help", []string{"--help"}, 0},
		{"resource help", []string{"incidents", "--help"}, 0},
		{"command help", []string{"monitors", "get", "--help"}, 0},
		{"empty", nil, 1},
		{"unknown resource", []string{"unknown", "list"}, 1},
		{"missing target", []string{"monitors", "get"}, 1},
		{"extra target", []string{"incidents", "get", "1", "2"}, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out, err bytes.Buffer
			code := runWithEndpoints(tc.args, &out, &err, srv.URL+"/uptime", srv.URL+"/rest", srv.URL+"/sql", testHTTPClient())
			oldCode, oldOut, oldErr := invokeAPI(srv.URL+"/uptime", testHTTPClient(), tc.args...)
			if code != tc.code || code != oldCode || out.String() != oldOut || err.String() != oldErr {
				t.Fatalf("endpoints = (%d, %q, %q), API = (%d, %q, %q), want code %d", code, out.String(), err.String(), oldCode, oldOut, oldErr, tc.code)
			}
		})
	}
	if requests.Load() != 0 {
		t.Fatalf("requests = %d, want 0", requests.Load())
	}
}

// TestBareInvocationIsInvalidUsage covers T2's baseline: bare `bs` is invalid
// usage. The CLI must write a readable error to stderr, write nothing to
// stdout, and exit 1.
func TestBareInvocationIsInvalidUsage(t *testing.T) {
	code, stdout, stderr := invoke()

	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
	if strings.TrimSpace(stderr) == "" {
		t.Error("stderr is empty, want a readable error")
	}
}

// TestRootHelp covers T3/R2: `bs --help` exits 0, writes human-readable text
// naming both resource groups to stdout, and writes nothing to stderr.
func TestRootHelp(t *testing.T) {
	code, stdout, stderr := invoke("--help")

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty", stderr)
	}
	for _, want := range []string{"incidents", "monitors"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout missing %q\nstdout:\n%s", want, stdout)
		}
	}
	if strings.HasPrefix(strings.TrimSpace(stdout), "{") {
		t.Errorf("stdout looks like JSON, want human-readable text:\n%s", stdout)
	}
}

// TestResourceHelp covers T4/R3: each resource help lists exactly its own
// commands and exits 0 with empty stderr.
func TestResourceHelp(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		wants   []string
		notWant []string
	}{
		{
			name:  "incidents",
			args:  []string{"incidents", "--help"},
			wants: []string{"list", "get", "timeline"},
		},
		{
			name:    "monitors",
			args:    []string{"monitors", "--help"},
			wants:   []string{"list", "get"},
			notWant: []string{"timeline"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, stdout, stderr := invoke(tc.args...)
			if code != 0 {
				t.Fatalf("exit code = %d, want 0", code)
			}
			if stderr != "" {
				t.Errorf("stderr = %q, want empty", stderr)
			}
			for _, want := range tc.wants {
				if !strings.Contains(stdout, want) {
					t.Errorf("stdout missing %q\nstdout:\n%s", want, stdout)
				}
			}
			for _, bad := range tc.notWant {
				if strings.Contains(stdout, bad) {
					t.Errorf("stdout unexpectedly contains %q\nstdout:\n%s", bad, stdout)
				}
			}
		})
	}
}

// TestCommandHelp covers T5-T9/R4: each per-command help exits 0, carries a
// usage line, an arguments section, a flags section, and examples, and
// documents no excluded surface.
func TestCommandHelp(t *testing.T) {
	excluded := []string{"--query", "--all", "--pretty", "metadata["}

	cases := []struct {
		name     string
		args     []string
		wants    []string
		notWants []string
	}{
		{
			name: "incidents list",
			args: []string{"incidents", "list", "--help"},
			wants: []string{
				"bs incidents list", "Arguments:", "Flags:", "Examples:",
				"--from", "--to", "--monitor-id", "--heartbeat-id",
				"--resolved", "--acknowledged", "--metadata", "--team-name",
				"--page", "--per-page", "--timeout", "30 seconds", "Repeatable",
				"bs incidents list --monitor-id 123 --resolved=false",
			},
			notWants: []string{"--url", "--pronounceable-name", "https://"},
		},
		{
			name: "incidents get",
			args: []string{"incidents", "get", "--help"},
			wants: []string{
				"bs incidents get <id-or-url>", "Arguments:", "Flags:", "Examples:",
				"--timeout", "30 seconds", "bare numeric",
				"https://incidents.betterstack.com/team/<team-id>/incidents/<incident-id>",
				"bs incidents get 123 --timeout 60s",
			},
			notWants: []string{"--from", "--monitor-id", "--page", "--metadata", "/monitors/"},
		},
		{
			name: "incidents timeline",
			args: []string{"incidents", "timeline", "--help"},
			wants: []string{
				"bs incidents timeline <id-or-url>", "Arguments:", "Flags:", "Examples:",
				"--timeout", "30 seconds",
				"https://incidents.betterstack.com/team/<team-id>/incidents/<incident-id>",
			},
			notWants: []string{"--from", "--monitor-id", "--page"},
		},
		{
			name: "monitors list",
			args: []string{"monitors", "list", "--help"},
			wants: []string{
				"bs monitors list", "Arguments:", "Flags:", "Examples:",
				"--url", "--pronounceable-name", "--team-name",
				"--page", "--per-page", "--timeout", "30 seconds",
				"bs monitors list --pronounceable-name Example",
			},
			notWants: []string{"--from", "--monitor-id", "--metadata", "https://"},
		},
		{
			name: "monitors get",
			args: []string{"monitors", "get", "--help"},
			wants: []string{
				"bs monitors get <id>", "Arguments:", "Flags:", "Examples:",
				"--timeout", "30 seconds", "bare monitor ID", "bs monitors get 789",
			},
			notWants: []string{"--from", "--page", "https://"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, stdout, stderr := invoke(tc.args...)
			if code != 0 {
				t.Fatalf("exit code = %d, want 0", code)
			}
			if stderr != "" {
				t.Errorf("stderr = %q, want empty", stderr)
			}
			for _, want := range tc.wants {
				if !strings.Contains(stdout, want) {
					t.Errorf("stdout missing %q\nstdout:\n%s", want, stdout)
				}
			}
			for _, bad := range append(excluded, tc.notWants...) {
				if strings.Contains(stdout, bad) {
					t.Errorf("stdout unexpectedly contains %q\nstdout:\n%s", bad, stdout)
				}
			}
		})
	}
}

// TestInvalidUsageMatrix covers T10/R5.2: every non-help vector exits 1 with a
// readable error on stderr and nothing on stdout.
func TestInvalidUsageMatrix(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"bare", nil},
		{"resource without action", []string{"incidents"}},
		{"monitors without action", []string{"monitors"}},
		{"unknown resource", []string{"uptime", "list"}},
		{"unknown action", []string{"incidents", "delete"}},
		{"help with extra token", []string{"--help", "incidents"}},
		{"help after positional", []string{"incidents", "get", "123", "--help"}},
		{"short help flag", []string{"-h"}},
		{"missing positional", []string{"incidents", "get"}},
		{"extra positional", []string{"monitors", "get", "1", "2"}},
		{"unknown resource flag", []string{"--timeout", "30s"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, stdout, stderr := invoke(tc.args...)
			if code != 1 {
				t.Fatalf("exit code = %d, want 1", code)
			}
			if stdout != "" {
				t.Errorf("stdout = %q, want empty", stdout)
			}
			if strings.TrimSpace(stderr) == "" {
				t.Error("stderr is empty, want a readable error")
			}
		})
	}
}
