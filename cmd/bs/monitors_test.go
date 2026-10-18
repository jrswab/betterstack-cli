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

// TestMonitorsGetPrintsRawBodyOnSuccess covers A3 (spec R3): a 2xx body is
// written to stdout byte-for-byte with nothing appended or removed, stderr is
// empty, and the exit code is 0. Non-JSON bodies print raw too.
func TestMonitorsGetPrintsRawBodyOnSuccess(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"json without trailing newline", `{"data":{"id":"789"}}`},
		{"non-json", "not json at all"},
		{"body with its own trailing newline", "kept\n"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = io.WriteString(w, tc.body)
			}))
			defer srv.Close()

			t.Setenv(tokenEnvVar, testToken)

			code, stdout, stderr := invokeAPI(srv.URL, newHTTPClient(), "monitors", "get", "789")
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

// TestMonitorsGetNon2xxPassthrough covers B1 (spec R4): every non-2xx
// response, 3xx included, writes the raw body to stderr byte-for-byte, writes
// nothing to stdout, exits 1, and reaches the server exactly once (no redirect
// following, no retry).
func TestMonitorsGetNon2xxPassthrough(t *testing.T) {
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
			var requests int
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				w.Header().Set("Location", "/redirect-target")
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, tc.body)
			}))
			defer srv.Close()

			t.Setenv(tokenEnvVar, testToken)

			code, stdout, stderr := invokeAPI(srv.URL, newHTTPClient(), "monitors", "get", "789")
			if code != 1 {
				t.Fatalf("exit code = %d, want 1", code)
			}
			if stdout != "" {
				t.Errorf("stdout = %q, want empty", stdout)
			}
			if stderr != tc.body {
				t.Errorf("stderr = %q, want %q", stderr, tc.body)
			}
			if requests != 1 {
				t.Errorf("server requests = %d, want 1", requests)
			}
		})
	}
}

// TestMonitorsGetTimeoutOverride covers B2 (spec R5): a short `--timeout`
// fails a slower server with exit 1, a token-free stderr message, empty
// stdout, and one request; the same server responding inside a longer override
// succeeds.
func TestMonitorsGetTimeoutOverride(t *testing.T) {
	t.Run("expires", func(t *testing.T) {
		var requests atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests.Add(1)
			time.Sleep(150 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		t.Setenv(tokenEnvVar, testToken)

		code, stdout, stderr := invokeAPI(srv.URL, newHTTPClient(), "monitors", "get", "789", "--timeout", "10ms")
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

		code, stdout, stderr := invokeAPI(srv.URL, newHTTPClient(), "monitors", "get", "789", "--timeout", "5s")
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

// TestMonitorsGetInvalidTimeout covers B2/R8: a non-positive or unparseable
// `--timeout` value is invalid usage — readable stderr, exit 1, no request.
func TestMonitorsGetInvalidTimeout(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"zero", []string{"monitors", "get", "789", "--timeout", "0s"}},
		{"negative", []string{"monitors", "get", "789", "--timeout", "-5s"}},
		{"not a duration", []string{"monitors", "get", "789", "--timeout", "nonsense"}},
		{"missing value", []string{"monitors", "get", "789", "--timeout"}},
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

// TestMonitorsGetLocalFailureIsTokenFree covers B3 (spec R6, R7): a local
// connection failure exits 1 with a concise stderr message, empty stdout, and
// no token anywhere — including when the token text is part of the request URL
// that a transport error would otherwise echo.
func TestMonitorsGetLocalFailureIsTokenFree(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	closedURL := srv.URL
	srv.Close()

	t.Run("connection refused", func(t *testing.T) {
		t.Setenv(tokenEnvVar, testToken)

		code, stdout, stderr := invokeAPI(closedURL, newHTTPClient(), "monitors", "get", "789")
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
	})

	t.Run("token in diagnostic url", func(t *testing.T) {
		t.Setenv(tokenEnvVar, testToken)

		code, stdout, stderr := invokeAPI(closedURL, newHTTPClient(), "monitors", "get", testToken)
		if code != 1 {
			t.Fatalf("exit code = %d, want 1", code)
		}
		if stdout != "" {
			t.Errorf("stdout = %q, want empty", stdout)
		}
		if strings.Contains(stderr, testToken) {
			t.Errorf("stderr leaks the token: %q", stderr)
		}
	})
}

// assertNoToken fails when the dummy token appears on either output channel.
func assertNoToken(t *testing.T, stdout, stderr string) {
	t.Helper()
	if strings.Contains(stdout, testToken) || strings.Contains(stderr, testToken) {
		t.Errorf("token leaked; stdout = %q, stderr = %q", stdout, stderr)
	}
}

// TestMonitorsGetNeverPrintsToken covers C1 (spec R7): with the dummy token in
// the environment and in the bearer header the mock observed, the value never
// appears on stdout or stderr on the success, non-2xx, and local-failure paths.
func TestMonitorsGetNeverPrintsToken(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		var gotAuth string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotAuth = r.Header.Get("Authorization")
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, `{"data":{}}`)
		}))
		defer srv.Close()

		t.Setenv(tokenEnvVar, testToken)

		code, stdout, stderr := invokeAPI(srv.URL, newHTTPClient(), "monitors", "get", "789")
		if code != 0 {
			t.Fatalf("exit code = %d, want 0", code)
		}
		if want := "Bearer " + testToken; gotAuth != want {
			t.Errorf("Authorization = %q, want %q", gotAuth, want)
		}
		assertNoToken(t, stdout, stderr)
	})

	t.Run("non-2xx", func(t *testing.T) {
		var gotAuth string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotAuth = r.Header.Get("Authorization")
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `{"errors":"not found"}`)
		}))
		defer srv.Close()

		t.Setenv(tokenEnvVar, testToken)

		code, stdout, stderr := invokeAPI(srv.URL, newHTTPClient(), "monitors", "get", "789")
		if code != 1 {
			t.Fatalf("exit code = %d, want 1", code)
		}
		if want := "Bearer " + testToken; gotAuth != want {
			t.Errorf("Authorization = %q, want %q", gotAuth, want)
		}
		assertNoToken(t, stdout, stderr)
	})

	t.Run("local failure", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		closedURL := srv.URL
		srv.Close()

		t.Setenv(tokenEnvVar, testToken)

		code, stdout, stderr := invokeAPI(closedURL, newHTTPClient(), "monitors", "get", "789")
		if code != 1 {
			t.Fatalf("exit code = %d, want 1", code)
		}
		assertNoToken(t, stdout, stderr)
	})
}

// TestUnimplementedCommandsMakeNoRequest covers C1 (spec R8): the 001
// invalid-usage vectors and the not-implemented commands keep exit 1 with a
// readable error, empty stdout, and zero mock requests.
func TestUnimplementedCommandsMakeNoRequest(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"incidents list", []string{"incidents", "list"}},
		{"incidents get", []string{"incidents", "get", "123"}},
		{"incidents timeline", []string{"incidents", "timeline", "123"}},
		{"monitors get missing id", []string{"monitors", "get"}},
		{"monitors get extra positional", []string{"monitors", "get", "1", "2"}},
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

// TestMonitorsGetHelpStillExitsZero covers C1 (spec R8): `bs monitors get
// --help` still exits 0 with empty stderr and makes no request.
func TestMonitorsGetHelpStillExitsZero(t *testing.T) {
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	t.Setenv(tokenEnvVar, testToken)

	code, stdout, stderr := invokeAPI(srv.URL, newHTTPClient(), "monitors", "get", "--help")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty", stderr)
	}
	if !strings.Contains(stdout, "bs monitors get <id>") {
		t.Errorf("stdout missing usage line:\n%s", stdout)
	}
	if got := requests.Load(); got != 0 {
		t.Errorf("server requests = %d, want 0", got)
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

// TestParseMonitorsListArgs covers A1 (spec R1, R6.1, R7): the parser accepts
// the six list flags in any order, records every occurrence of a repeated
// flag, distinguishes an absent flag (nil) from an explicitly empty value
// ([]string{""}), and rejects unknown options, positionals, missing values,
// and invalid timeouts. No server is involved.
func TestParseMonitorsListArgs(t *testing.T) {
	t.Run("no flags keeps every field absent", func(t *testing.T) {
		got, err := parseMonitorsListArgs(nil)
		if err != nil {
			t.Fatalf("parse error: %v", err)
		}
		want := monitorsListArgs{timeout: defaultTimeout}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %+v, want %+v", got, want)
		}
	})

	t.Run("each flag alone", func(t *testing.T) {
		cases := []struct {
			name string
			args []string
			want monitorsListArgs
		}{
			{"page", []string{"--page", "2"}, monitorsListArgs{pages: []string{"2"}, timeout: defaultTimeout}},
			{"per-page", []string{"--per-page", "50"}, monitorsListArgs{perPages: []string{"50"}, timeout: defaultTimeout}},
			{"url", []string{"--url", "https://example.com/x"}, monitorsListArgs{urls: []string{"https://example.com/x"}, timeout: defaultTimeout}},
			{"pronounceable-name", []string{"--pronounceable-name", "Example Monitor"}, monitorsListArgs{pronounceableNames: []string{"Example Monitor"}, timeout: defaultTimeout}},
			{"team-name", []string{"--team-name", "Acme"}, monitorsListArgs{teamNames: []string{"Acme"}, timeout: defaultTimeout}},
			{"timeout", []string{"--timeout", "5s"}, monitorsListArgs{timeout: 5 * time.Second}},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				got, err := parseMonitorsListArgs(tc.args)
				if err != nil {
					t.Fatalf("parse error: %v", err)
				}
				if !reflect.DeepEqual(got, tc.want) {
					t.Errorf("got %+v, want %+v", got, tc.want)
				}
			})
		}
	})

	t.Run("all flags in any order", func(t *testing.T) {
		args := []string{
			"--team-name", "Acme",
			"--url", "https://example.com/x",
			"--page", "2",
			"--pronounceable-name", "Example",
			"--per-page", "50",
			"--timeout", "5s",
		}
		got, err := parseMonitorsListArgs(args)
		if err != nil {
			t.Fatalf("parse error: %v", err)
		}
		want := monitorsListArgs{
			pages:              []string{"2"},
			perPages:           []string{"50"},
			urls:               []string{"https://example.com/x"},
			pronounceableNames: []string{"Example"},
			teamNames:          []string{"Acme"},
			timeout:            5 * time.Second,
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %+v, want %+v", got, want)
		}
	})

	t.Run("repeated flag keeps each occurrence", func(t *testing.T) {
		got, err := parseMonitorsListArgs([]string{"--page", "1", "--page", "2", "--url", "first", "--url", "second"})
		if err != nil {
			t.Fatalf("parse error: %v", err)
		}
		want := monitorsListArgs{
			pages:   []string{"1", "2"},
			urls:    []string{"first", "second"},
			timeout: defaultTimeout,
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %+v, want %+v", got, want)
		}
	})

	t.Run("empty explicit value stays present", func(t *testing.T) {
		got, err := parseMonitorsListArgs([]string{"--url", ""})
		if err != nil {
			t.Fatalf("parse error: %v", err)
		}
		want := monitorsListArgs{urls: []string{""}, timeout: defaultTimeout}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %+v, want %+v", got, want)
		}
	})

	t.Run("errors", func(t *testing.T) {
		cases := []struct {
			name string
			args []string
		}{
			{"unknown --all", []string{"--all"}},
			{"unknown --pretty", []string{"--pretty"}},
			{"unknown --query", []string{"--query", "x"}},
			{"unknown short help", []string{"-h"}},
			{"help in flag position", []string{"--help"}},
			{"unknown option", []string{"--bogus"}},
			{"positional", []string{"extra"}},
			{"page missing value", []string{"--page"}},
			{"url missing value", []string{"--url"}},
			{"timeout missing value", []string{"--timeout"}},
			{"timeout zero", []string{"--timeout", "0s"}},
			{"timeout negative", []string{"--timeout", "-5s"}},
			{"timeout not a duration", []string{"--timeout", "nonsense"}},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				if _, err := parseMonitorsListArgs(tc.args); err == nil {
					t.Fatalf("parseMonitorsListArgs(%q) = nil error, want error", tc.args)
				}
			})
		}
	})
}

// invokeMonitorsList calls monitorsList directly through the same writer
// contract used by the production route. B1 tests use this before routing is
// wired in B2.
func invokeMonitorsList(baseURL string, client *http.Client, args ...string) (code int, stdout, stderr string) {
	var out, err bytes.Buffer
	code = monitorsList(args, &out, &err, baseURL, client)
	return code, out.String(), err.String()
}

// TestMonitorsListQueryTranslation covers B1 (spec R3): exactly one GET to
// /api/v2/monitors, each flag maps to one query parameter with its value
// unchanged, absent flags send nothing, explicit empty values send the
// parameter, repeated occurrences stay repeated, and stdlib net/url encoding
// handles adversarial characters.
func TestMonitorsListQueryTranslation(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		wantRaw string
		wantURL string
	}{
		{name: "no flags", args: nil, wantRaw: ""},
		{name: "page", args: []string{"--page", "2"}, wantRaw: "page=2"},
		{name: "per page", args: []string{"--per-page", "50"}, wantRaw: "per_page=50"},
		{name: "url", args: []string{"--url", "https://example.com/x"}, wantRaw: "url=https%3A%2F%2Fexample.com%2Fx", wantURL: "https://example.com/x"},
		{name: "pronounceable name", args: []string{"--pronounceable-name", "Example Monitor"}, wantRaw: "pronounceable_name=Example+Monitor"},
		{name: "team name", args: []string{"--team-name", "Acme"}, wantRaw: "team_name=Acme"},
		{
			name: "all flags combined",
			args: []string{
				"--team-name", "Acme",
				"--url", "https://example.com/x",
				"--page", "2",
				"--pronounceable-name", "Example",
				"--per-page", "50",
			},
			wantRaw: "page=2&per_page=50&pronounceable_name=Example&team_name=Acme&url=https%3A%2F%2Fexample.com%2Fx",
			wantURL: "https://example.com/x",
		},
		{name: "empty explicit value", args: []string{"--url", ""}, wantRaw: "url=", wantURL: ""},
		{
			name:    "repeated flags keep every occurrence",
			args:    []string{"--page", "1", "--page", "2", "--url", "first", "--url", "second"},
			wantRaw: "page=1&page=2&url=first&url=second",
		},
		{
			name:    "adversarial characters",
			args:    []string{"--url", "a b&c=d/ü?é+%"},
			wantRaw: "url=a+b%26c%3Dd%2F%C3%BC%3F%C3%A9%2B%25",
			wantURL: "a b&c=d/ü?é+%",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var requests atomic.Int32
			var gotMethod, gotPath, gotRaw, gotURL, gotAuth string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				gotMethod = r.Method
				gotPath = r.URL.EscapedPath()
				gotRaw = r.URL.RawQuery
				gotURL = r.URL.Query().Get("url")
				gotAuth = r.Header.Get("Authorization")
				w.WriteHeader(http.StatusOK)
				_, _ = io.WriteString(w, `{"data":[]}`)
			}))
			defer srv.Close()

			t.Setenv(tokenEnvVar, testToken)

			code, stdout, stderr := invokeMonitorsList(srv.URL, newHTTPClient(), tc.args...)
			if code != 0 {
				t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr)
			}
			if got := requests.Load(); got != 1 {
				t.Errorf("server requests = %d, want 1", got)
			}
			if gotMethod != http.MethodGet {
				t.Errorf("method = %q, want %q", gotMethod, http.MethodGet)
			}
			if gotPath != "/api/v2/monitors" {
				t.Errorf("path = %q, want %q", gotPath, "/api/v2/monitors")
			}
			if gotRaw != tc.wantRaw {
				t.Errorf("raw query = %q, want %q", gotRaw, tc.wantRaw)
			}
			if tc.wantURL != "" && gotURL != tc.wantURL {
				t.Errorf("decoded url = %q, want %q", gotURL, tc.wantURL)
			}
			if want := "Bearer " + testToken; gotAuth != want {
				t.Errorf("Authorization = %q, want %q", gotAuth, want)
			}
			if stdout != `{"data":[]}` {
				t.Errorf("stdout = %q, want raw body", stdout)
			}
			if stderr != "" {
				t.Errorf("stderr = %q, want empty", stderr)
			}
		})
	}
}

// TestMonitorsListRawSuccessBody covers B1 (spec R4): a 2xx body is written
// byte-for-byte with no appended newline, including a pagination object, a
// non-JSON body, and a body carrying its own trailing newline.
func TestMonitorsListRawSuccessBody(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"pagination object", `{"data":[],"pagination":{"first":"https://x/1","last":"https://x/1","prev":null,"next":null}}`},
		{"non-json", "not json at all"},
		{"body with its own trailing newline", "kept\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = io.WriteString(w, tc.body)
			}))
			defer srv.Close()

			t.Setenv(tokenEnvVar, testToken)

			code, stdout, stderr := invokeMonitorsList(srv.URL, newHTTPClient())
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

// TestMonitorsListNon2xxPassthrough covers B1 (spec R5.1): every non-2xx,
// 3xx included, prints the raw body to stderr, writes nothing to stdout,
// exits 1, and reaches only the requested endpoint even when the redirect
// target is another local path.
func TestMonitorsListNon2xxPassthrough(t *testing.T) {
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
			var requests, redirectTarget atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/redirect-target" {
					redirectTarget.Add(1)
					w.WriteHeader(http.StatusOK)
					return
				}
				requests.Add(1)
				w.Header().Set("Location", "/redirect-target")
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, tc.body)
			}))
			defer srv.Close()

			t.Setenv(tokenEnvVar, testToken)

			code, stdout, stderr := invokeMonitorsList(srv.URL, newHTTPClient())
			if code != 1 {
				t.Fatalf("exit code = %d, want 1", code)
			}
			if stdout != "" {
				t.Errorf("stdout = %q, want empty", stdout)
			}
			if stderr != tc.body {
				t.Errorf("stderr = %q, want %q", stderr, tc.body)
			}
			if got := requests.Load(); got != 1 {
				t.Errorf("endpoint requests = %d, want 1", got)
			}
			if got := redirectTarget.Load(); got != 0 {
				t.Errorf("redirect target requests = %d, want 0", got)
			}
		})
	}
}

// TestMonitorsListNeverDrainsPagination covers B1 (spec R4.2, R7): a 2xx body
// with a pagination.next pointing at another local path is printed unchanged
// and never followed, even when the target is on the mock host.
func TestMonitorsListNeverDrainsPagination(t *testing.T) {
	var requests, nextRequests atomic.Int32
	var nextURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/monitors" && r.URL.Query().Get("page") == "2" {
			nextRequests.Add(1)
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, `{"data":["second page"]}`)
			return
		}
		requests.Add(1)
		body := `{"data":[],"pagination":{"next":"` + nextURL + `"}}`
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, body)
	}))
	defer srv.Close()
	nextURL = srv.URL + "/api/v2/monitors?page=2"

	t.Setenv(tokenEnvVar, testToken)

	code, stdout, stderr := invokeMonitorsList(srv.URL, newHTTPClient())
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr)
	}
	want := `{"data":[],"pagination":{"next":"` + nextURL + `"}}`
	if stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty", stderr)
	}
	if got := requests.Load(); got != 1 {
		t.Errorf("first-page requests = %d, want 1", got)
	}
	if got := nextRequests.Load(); got != 0 {
		t.Errorf("pagination.next requests = %d, want 0", got)
	}
}

// TestMonitorsListTimeoutOverride covers B1 (spec R6.1): a short --timeout
// fails a slower server with exit 1, token-free stderr, empty stdout, and one
// request; the same server responding inside a longer override succeeds.
func TestMonitorsListTimeoutOverride(t *testing.T) {
	t.Run("expires", func(t *testing.T) {
		var requests atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests.Add(1)
			time.Sleep(150 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		t.Setenv(tokenEnvVar, testToken)

		code, stdout, stderr := invokeMonitorsList(srv.URL, newHTTPClient(), "--timeout", "10ms")
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

		code, stdout, stderr := invokeMonitorsList(srv.URL, newHTTPClient(), "--timeout", "5s")
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

// TestMonitorsListInvalidTimeout covers B1 (spec R6.1): a non-positive,
// unparseable, or missing --timeout value is invalid usage with zero requests.
func TestMonitorsListInvalidTimeout(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"zero", []string{"--timeout", "0s"}},
		{"negative", []string{"--timeout", "-5s"}},
		{"not a duration", []string{"--timeout", "nonsense"}},
		{"missing value", []string{"--timeout"}},
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

			code, stdout, stderr := invokeMonitorsList(srv.URL, newHTTPClient(), tc.args...)
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

// TestMonitorsListRouting covers B2 (spec R1.1, R8.1): `runWithAPI` routes the
// now-working `monitors list` command to its implementation, while a
// positional argument after `list` stays invalid usage with zero requests.
func TestMonitorsListRouting(t *testing.T) {
	t.Run("valid command reaches server", func(t *testing.T) {
		var requests atomic.Int32
		var gotPath, gotRaw string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests.Add(1)
			gotPath = r.URL.EscapedPath()
			gotRaw = r.URL.RawQuery
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, `{"data":[]}`)
		}))
		defer srv.Close()

		t.Setenv(tokenEnvVar, testToken)

		code, stdout, stderr := invokeAPI(srv.URL, newHTTPClient(), "monitors", "list", "--page", "2")
		if code != 0 {
			t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr)
		}
		if got := requests.Load(); got != 1 {
			t.Errorf("server requests = %d, want 1", got)
		}
		if gotPath != "/api/v2/monitors" {
			t.Errorf("path = %q, want %q", gotPath, "/api/v2/monitors")
		}
		if gotRaw != "page=2" {
			t.Errorf("raw query = %q, want %q", gotRaw, "page=2")
		}
		if stdout != `{"data":[]}` {
			t.Errorf("stdout = %q, want raw body", stdout)
		}
		if stderr != "" {
			t.Errorf("stderr = %q, want empty", stderr)
		}
	})

	t.Run("positional argument is invalid usage", func(t *testing.T) {
		var requests atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests.Add(1)
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		t.Setenv(tokenEnvVar, testToken)

		code, stdout, stderr := invokeAPI(srv.URL, newHTTPClient(), "monitors", "list", "extra")
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
