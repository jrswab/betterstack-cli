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
		{"monitors list", []string{"monitors", "list"}},
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
