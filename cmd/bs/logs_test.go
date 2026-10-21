package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func setFakeSQLConfig(t *testing.T) {
	t.Helper()
	t.Setenv("BETTERSTACK_TELEMETRY_SQL_URL", "sql.example")
	t.Setenv("BETTERSTACK_TELEMETRY_SQL_USERNAME", "fake-sql-user")
	t.Setenv("BETTERSTACK_TELEMETRY_SQL_PASSWORD", "fake-sql-password")
	t.Setenv(telemetryTokenEnvVar, "fake-telemetry-token")
	t.Setenv(tokenEnvVar, "fake-uptime-token")
}

// TestLogsSQLUsesConfiguredEndpoint verifies the configured HTTPS destination without network access.
func TestLogsSQLUsesConfiguredEndpoint(t *testing.T) {
	setFakeSQLConfig(t)
	sql, params, err := buildLogsSearchQuery([]string{"--table", "t1_fake", "--query", "fake & 'text'"}, time.Unix(3600, 0))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ endpoint, host, path string }{
		{"regional.sql.example", "regional.sql.example", ""},
		{"regional.sql.example:443", "regional.sql.example:443", ""},
		{"https://regional.sql.example", "regional.sql.example", ""},
		{"https://regional.sql.example:8443/", "regional.sql.example:8443", "/"},
	} {
		t.Run(tc.endpoint, func(t *testing.T) {
			t.Setenv(telemetrySQLURLEnvVar, tc.endpoint)
			transport := &recordingRoundTripper{}
			client := &http.Client{Transport: transport}
			var out, diagnostic bytes.Buffer
			code := doLogsSQLRequest(sql, params, time.Second, &out, &diagnostic, "", client)
			if code != 0 || out.String() != "recorded response" || diagnostic.Len() != 0 || len(transport.requests) != 1 {
				t.Fatalf("code=%d, stdout=%q, stderr=%q, requests=%d", code, out.String(), diagnostic.String(), len(transport.requests))
			}
			req := transport.requests[0]
			if req.URL.Scheme != "https" || req.URL.Host != tc.host || req.URL.Path != tc.path || req.URL.RawQuery != params.Encode() {
				t.Fatalf("request URL = %s, want https://%s%s?%s", req.URL, tc.host, tc.path, params.Encode())
			}
			for name, values := range req.Header {
				for _, value := range values {
					if strings.Contains(value, os.Getenv(telemetryTokenEnvVar)) {
						t.Errorf("request header %s contains the fake Telemetry API token", name)
					}
				}
			}
		})
	}
}

// TestRunWithEndpointsRoutesSQL verifies argv routing through the injected SQL endpoint.
func TestRunWithEndpointsRoutesSQL(t *testing.T) {
	setFakeSQLConfig(t)
	var uptimeRequests, restRequests, sqlRequests atomic.Int32
	uptime := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { uptimeRequests.Add(1) }))
	defer uptime.Close()
	rest := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { restRequests.Add(1) }))
	defer rest.Close()
	const wantSQL = "SELECT dt, raw FROM remote(t1_fake_logs) WHERE dt >= fromUnixTimestamp64Milli({from:Int64}, 'UTC') AND dt < fromUnixTimestamp64Milli({to:Int64}, 'UTC') AND positionCaseInsensitiveUTF8(raw, {query:String}) > 0 ORDER BY dt DESC LIMIT 100 SETTINGS max_execution_time = 2 FORMAT JSONEachRow"
	sqlServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sqlRequests.Add(1)
		body, err := io.ReadAll(r.Body)
		if err != nil || string(body) != wantSQL || r.Method != http.MethodPost || r.Header.Get("Content-Type") != "plain/text" || r.URL.Path != "/" {
			t.Errorf("request shape = %s %s, body=%q, error=%v", r.Method, r.URL, body, err)
		}
		if r.URL.RawQuery != "param_from=0&param_query=fake+%26+%27text%27&param_to=3600000" {
			t.Errorf("query = %q", r.URL.RawQuery)
		}
		username, password, ok := r.BasicAuth()
		if !ok || username != "fake-sql-user" || password != "fake-sql-password" {
			t.Error("request did not use SQL Basic credentials")
		}
		_, _ = io.WriteString(w, "SQL body")
	}))
	defer sqlServer.Close()
	args := []string{"--table", "t1_fake", "--from", "1970-01-01T00:00:00Z", "--to", "1970-01-01T01:00:00Z", "--query", "fake & 'text'", "--timeout", "1500ms"}
	sql, params, err := buildLogsSearchQuery(args, time.Unix(3600, 0))
	if err != nil || sql != wantSQL || params.Encode() != "param_from=0&param_query=fake+%26+%27text%27&param_to=3600000" {
		t.Fatalf("builder SQL=%q, params=%v, error=%v", sql, params, err)
	}
	client := testHTTPClient()
	guard := client.Transport.(*loopbackOnlyTransport)
	probe := &sqlDeadlineTransport{base: guard.base, timeout: 1500 * time.Millisecond}
	guard.base = probe
	var out, diagnostic bytes.Buffer
	code := runWithEndpoints(append([]string{"logs", "search"}, args...), &out, &diagnostic, uptime.URL, rest.URL, sqlServer.URL+"/", client)
	if code != 0 || out.String() != "SQL body" || diagnostic.Len() != 0 || !probe.valid || probe.calls != 1 || sqlRequests.Load() != 1 || uptimeRequests.Load() != 0 || restRequests.Load() != 0 {
		t.Fatalf("code=%d, stdout=%q, stderr=%q, deadline=%t, attempts=%d, SQL=%d, Uptime=%d, REST=%d", code, out.String(), diagnostic.String(), probe.valid, probe.calls, sqlRequests.Load(), uptimeRequests.Load(), restRequests.Load())
	}
}

type sqlDeadlineTransport struct {
	base    http.RoundTripper
	timeout time.Duration
	valid   bool
	calls   int
}

// RoundTrip observes the client deadline behind the loopback guard and delegates to its guarded transport.
func (p *sqlDeadlineTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	p.calls++
	deadline, ok := req.Context().Deadline()
	remaining := time.Until(deadline)
	p.valid = ok && remaining > p.timeout-100*time.Millisecond && remaining <= p.timeout
	return p.base.RoundTrip(req)
}

// TestLogsSearchResponseChannels verifies routed SQL response bytes and error redaction.
func TestLogsSearchResponseChannels(t *testing.T) {
	setFakeSQLConfig(t)
	const body = "fake-sql-user fake-sql-password fake-telemetry-token\x00\n"
	for _, status := range []int{200, 500} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			var requests atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				w.WriteHeader(status)
				_, _ = io.WriteString(w, body)
			}))
			defer srv.Close()
			var out, err bytes.Buffer
			code := runWithEndpoints([]string{"logs", "search", "--table", "t1_fake"}, &out, &err, srv.URL, srv.URL, srv.URL, testHTTPClient())
			if status == 200 {
				if code != 0 || out.String() != body || err.Len() != 0 {
					t.Fatalf("code=%d, stdout=%q, stderr=%q", code, out.String(), err.String())
				}
			} else if code != 1 || out.Len() != 0 || err.String() != "[REDACTED] [REDACTED] [REDACTED]\x00\n" {
				t.Fatalf("code=%d, stdout=%q, stderr=%q", code, out.String(), err.String())
			}
			if requests.Load() != 1 {
				t.Fatalf("requests=%d, want 1", requests.Load())
			}
		})
	}
}

// TestLogsSearchOptionShapedQuery verifies query data does not become a cold switch.
func TestLogsSearchOptionShapedQuery(t *testing.T) {
	setFakeSQLConfig(t)
	for _, cold := range []bool{false, true} {
		t.Run(strconv.FormatBool(cold), func(t *testing.T) {
			args := []string{"logs", "search", "--table", "t1_fake"}
			if cold {
				args = append(args, "--cold")
			}
			args = append(args, "--query", "--cold")
			transport := &recordingRoundTripper{}
			var out, err bytes.Buffer
			code := runWithEndpoints(args, &out, &err, "http://127.0.0.1/uptime", "http://127.0.0.1/rest", "http://127.0.0.1/sql", &http.Client{Transport: transport})
			if code != 0 || out.String() != "recorded response" || err.Len() != 0 || len(transport.requests) != 1 {
				t.Fatalf("code=%d, stdout=%q, stderr=%q, requests=%d", code, out.String(), err.String(), len(transport.requests))
			}
			req := transport.requests[0]
			body, readErr := io.ReadAll(req.Body)
			if readErr != nil || req.URL.Path != "/sql" || req.URL.Query().Get("param_query") != "--cold" || strings.Contains(string(body), "UNION ALL") != cold || strings.Contains(string(body), "--cold") {
				t.Fatalf("request=%s, body=%q, error=%v", req.URL, body, readErr)
			}
		})
	}
}

// TestLogsSQLLocalFailures verifies cancellation, transport errors, and incomplete bodies stay safe.
func TestLogsSQLLocalFailures(t *testing.T) {
	setFakeSQLConfig(t)
	for _, kind := range []string{"timeout", "closed server", "truncated success", "truncated error", "invalid URL", "invalid port", "zero timeout"} {
		t.Run(kind, func(t *testing.T) {
			var requests atomic.Int32
			cancelled := make(chan struct{})
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				if kind == "timeout" {
					_, _ = io.Copy(io.Discard, r.Body)
					<-r.Context().Done()
					close(cancelled)
					return
				}
				w.Header().Set("Content-Length", "1000")
				if kind == "truncated error" {
					w.WriteHeader(500)
				}
				_, _ = io.WriteString(w, "fake-sql-user fake-sql-password fake-telemetry-token")
			}))
			defer srv.Close()
			endpoint, timeout := srv.URL, time.Second
			want, attempts, served := "bs: could not read SQL response\n", 1, int32(1)
			switch kind {
			case "timeout":
				timeout = 100 * time.Millisecond
				want = "bs: SQL request failed\n"
			case "closed server":
				srv.Close()
				want, served = "bs: SQL request failed\n", 0
			case "invalid URL":
				endpoint = "http://fake-sql-user:fake-sql-password@127.0.0.1/%zzfake-telemetry-token"
				want, attempts, served = "bs: could not build SQL request\n", 0, 0
			case "invalid port":
				endpoint = "http://127.0.0.1:fake-sql-password"
				want, attempts, served = "bs: could not build SQL request\n", 0, 0
			case "zero timeout":
				timeout = 0
				want, attempts, served = "bs: invalid SQL request timeout\n", 0, 0
			}
			client := testHTTPClient()
			guard := client.Transport.(*loopbackOnlyTransport)
			probe := &sqlDeadlineTransport{base: guard.base, timeout: timeout}
			guard.base = probe
			var out, err bytes.Buffer
			code := doLogsSQLRequest("SELECT 1", nil, timeout, &out, &err, endpoint, client)
			if code != 1 || out.Len() != 0 || err.String() != want || requests.Load() != served || probe.calls != attempts {
				t.Fatalf("code=%d, stdout=%q, stderr=%q, requests=%d, attempts=%d", code, out.String(), err.String(), requests.Load(), probe.calls)
			}
			if kind == "timeout" {
				select {
				case <-cancelled:
				case <-time.After(time.Second):
					t.Fatal("timeout did not cancel the server request context")
				}
			}
		})
	}
}

// TestLogsSQLErrorRedaction verifies only non-2xx SQL bodies lose credential literals.
func TestLogsSQLErrorRedaction(t *testing.T) {
	setFakeSQLConfig(t)
	const body = "unrelated\x00 SELECT dt, raw FROM remote(t1_fake_logs)\nfake-telemetry-token fake-sql-user fake-sql-password Basic ZmFrZS1zcWwtdXNlcjpmYWtlLXNxbC1wYXNzd29yZA==\n"
	const want = "unrelated\x00 SELECT dt, raw FROM remote(t1_fake_logs)\n[REDACTED] [REDACTED] [REDACTED] Basic [REDACTED]\n"
	for _, status := range []int{300, 302, 304, 400, 401, 429, 500, 503, 599} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			var requests atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				w.WriteHeader(status)
				_, _ = io.WriteString(w, body)
			}))
			defer srv.Close()
			var out, err bytes.Buffer
			code := doLogsSQLRequest("SELECT dt, raw FROM remote(t1_fake_logs)", nil, time.Second, &out, &err, srv.URL, testHTTPClient())
			expected := want
			if status == 304 {
				expected = ""
			}
			if code != 1 || out.Len() != 0 || err.String() != expected || requests.Load() != 1 {
				t.Fatalf("code=%d, stdout=%q, stderr=%q, requests=%d", code, out.String(), err.String(), requests.Load())
			}
		})
	}
}

// TestSQLRedactionLeavesOtherResponsesRaw verifies Uptime and source bodies retain SQL and API secrets.
func TestSQLRedactionLeavesOtherResponsesRaw(t *testing.T) {
	setFakeSQLConfig(t)
	const body = "fake-uptime-token fake-telemetry-token fake-sql-user fake-sql-password\n"
	for _, status := range []int{200, 400} {
		for _, args := range [][]string{{"monitors", "list"}, {"sources", "list"}} {
			t.Run(strconv.Itoa(status)+"/"+args[0], func(t *testing.T) {
				var requests atomic.Int32
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					requests.Add(1)
					w.WriteHeader(status)
					_, _ = io.WriteString(w, body)
				}))
				defer srv.Close()
				var out, err bytes.Buffer
				code := runWithEndpoints(args, &out, &err, srv.URL, srv.URL, srv.URL, testHTTPClient())
				if status == 200 {
					if code != 0 || out.String() != body || err.Len() != 0 {
						t.Fatal("non-SQL success body changed")
					}
				} else if code != 1 || out.Len() != 0 || err.String() != body {
					t.Fatal("non-SQL error body changed")
				}
				if requests.Load() != 1 {
					t.Fatal("unexpected request count")
				}
			})
		}
	}
}

// TestLogsSQLSuccessBytes verifies all success status branches preserve complete raw bodies.
func TestLogsSQLSuccessBytes(t *testing.T) {
	setFakeSQLConfig(t)
	for _, status := range []int{200, 201, 202, 204, 205, 206, 299} {
		for _, body := range []string{"", `{"error":"not interpreted"}`, "not JSON\x00\xff", "line\n", "fake-sql-user fake-sql-password fake-telemetry-token ZmFrZS1zcWwtdXNlcjpmYWtlLXNxbC1wYXNzd29yZA=="} {
			if status == 204 && body != "" {
				continue
			}
			t.Run(strconv.Itoa(status)+"/"+body, func(t *testing.T) {
				var requests atomic.Int32
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					requests.Add(1)
					w.WriteHeader(status)
					_, _ = io.WriteString(w, body)
				}))
				defer srv.Close()
				var out, err bytes.Buffer
				code := doLogsSQLRequest("SELECT 1", nil, time.Second, &out, &err, srv.URL, testHTTPClient())
				if code != 0 || out.String() != body || err.Len() != 0 || requests.Load() != 1 {
					t.Fatalf("code=%d, stdout=%q, stderr=%q, requests=%d", code, out.String(), err.String(), requests.Load())
				}
			})
		}
	}
}

// TestLogsSQLNoRedirectsOrRetries verifies one attempt for redirects, server errors, and broken connections.
func TestLogsSQLNoRedirectsOrRetries(t *testing.T) {
	setFakeSQLConfig(t)
	var targetRequests atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { targetRequests.Add(1) }))
	defer target.Close()
	for _, status := range []int{301, 302, 303, 307, 308, 500, 503, 0} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			var requests atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				if status == 0 {
					conn, _, err := w.(http.Hijacker).Hijack()
					if err != nil {
						t.Error(err)
						return
					}
					_ = conn.Close()
					return
				}
				w.Header().Set("Location", target.URL)
				w.WriteHeader(status)
				_, _ = io.WriteString(w, "original error body\n")
			}))
			defer srv.Close()
			client := testHTTPClient()
			guard := client.Transport.(*loopbackOnlyTransport)
			probe := &sqlDeadlineTransport{base: guard.base, timeout: time.Second}
			guard.base = probe
			callbackCalls := 0
			client.CheckRedirect = func(*http.Request, []*http.Request) error { callbackCalls++; return nil }
			var out, err bytes.Buffer
			code := doLogsSQLRequest("SELECT 1", nil, time.Second, &out, &err, srv.URL, client)
			want := "original error body\n"
			if status == 0 {
				want = "bs: SQL request failed\n"
			}
			if code != 1 || out.Len() != 0 || err.String() != want || requests.Load() != 1 || targetRequests.Load() != 0 || probe.calls != 1 || callbackCalls != 0 {
				t.Fatalf("code=%d, stderr=%q, requests=%d, target=%d, attempts=%d, callbacks=%d", code, err.String(), requests.Load(), targetRequests.Load(), probe.calls, callbackCalls)
			}
			if client.CheckRedirect == nil || client.CheckRedirect(nil, nil) != nil || callbackCalls != 1 {
				t.Fatal("executor changed the supplied client")
			}
		})
	}
}

// TestLogsSQLCredentials verifies required SQL variables, safe failures, and separate Basic auth.
func TestLogsSQLCredentials(t *testing.T) {
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		username, password, ok := r.BasicAuth()
		if !ok || username != "fake-sql-user" || password != "fake-sql-password" || strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			t.Error("SQL request did not use separate Basic credentials")
		}
		_, _ = io.WriteString(w, "ok")
	}))
	defer srv.Close()
	for _, name := range []string{"BETTERSTACK_TELEMETRY_SQL_URL", "BETTERSTACK_TELEMETRY_SQL_USERNAME", "BETTERSTACK_TELEMETRY_SQL_PASSWORD"} {
		for _, unset := range []bool{false, true} {
			t.Run(name+"/unset="+strconv.FormatBool(unset), func(t *testing.T) {
				setFakeSQLConfig(t)
				t.Setenv(name, "")
				if unset {
					if err := os.Unsetenv(name); err != nil {
						t.Fatal(err)
					}
				}
				var out, err bytes.Buffer
				code := doLogsSQLRequest("SELECT 1", nil, time.Second, &out, &err, srv.URL, testHTTPClient())
				if code != 1 || out.Len() != 0 || err.String() != "bs: "+name+" is not set\n" || requests.Load() != 0 {
					t.Fatalf("local validation failed: code=%d, stderr=%q, requests=%d", code, err.String(), requests.Load())
				}
			})
		}
	}
	t.Run("API tokens do not replace SQL credentials", func(t *testing.T) {
		setFakeSQLConfig(t)
		t.Setenv("BETTERSTACK_TELEMETRY_SQL_USERNAME", "")
		t.Setenv("BETTERSTACK_TELEMETRY_SQL_PASSWORD", "")
		var out, err bytes.Buffer
		if code := doLogsSQLRequest("SELECT 1", nil, time.Second, &out, &err, srv.URL, testHTTPClient()); code != 1 || out.Len() != 0 || requests.Load() != 0 {
			t.Fatal("API tokens substituted for SQL credentials")
		}
	})
	t.Run("invalid configured URL", func(t *testing.T) {
		setFakeSQLConfig(t)
		t.Setenv("BETTERSTACK_TELEMETRY_SQL_URL", "https://fake-sql-user:fake-sql-password@sql.example")
		var out, err bytes.Buffer
		if code := doLogsSQLRequest("SELECT 1", nil, time.Second, &out, &err, srv.URL, testHTTPClient()); code != 1 || out.Len() != 0 || err.String() != "bs: invalid BETTERSTACK_TELEMETRY_SQL_URL\n" || requests.Load() != 0 {
			t.Fatal("invalid configured URL was not rejected safely before a request")
		}
	})
	t.Run("valid separate credentials", func(t *testing.T) {
		setFakeSQLConfig(t)
		var out, err bytes.Buffer
		if code := doLogsSQLRequest("SELECT 1", nil, time.Second, &out, &err, srv.URL, testHTTPClient()); code != 0 || out.String() != "ok" || err.Len() != 0 || requests.Load() != 1 {
			t.Fatalf("code=%d, stdout=%q, stderr=%q, requests=%d", code, out.String(), err.String(), requests.Load())
		}
	})
}

// TestParseTelemetrySQLURL verifies approved HTTPS authorities and safe rejection.
func TestParseTelemetrySQLURL(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{"sql.example", "https://sql.example"},
		{"sql.example:443", "https://sql.example:443"},
		{"https://sql.example", "https://sql.example"},
		{"https://sql.example/", "https://sql.example/"},
		{"https://sql.example:8443/", "https://sql.example:8443/"},
		{"127.0.0.1", "https://127.0.0.1"},
		{"127.0.0.1:443", "https://127.0.0.1:443"},
		{"[::1]", "https://[::1]"},
		{"[::1]:443", "https://[::1]:443"},
		{"https://[::1]:65535/", "https://[::1]:65535/"},
	} {
		t.Run(tc.input, func(t *testing.T) {
			endpoint, err := parseTelemetrySQLURL(tc.input)
			if err != nil || endpoint.String() != tc.want {
				t.Fatalf("endpoint = %v, error = %v, want %s", endpoint, err, tc.want)
			}
		})
	}
	for _, input := range []string{
		"", "http://sql.example", "ftp://sql.example", "//sql.example", "sql.example:8443",
		"sql.example/", "sql.example/path", "https://sql.example/path", "https://sql.example/%2f",
		"https://sql.example?", "https://sql.example?x=y", "https://sql.example#", "https://sql.example#frag",
		"https://fake-user:fake-password@sql.example", "fake-user:fake-password@sql.example",
		"https://", "https:///", "https://:443", "https://sql.example:", "https://sql.example:0",
		"https://sql.example:65536", "https://sql.example:bad", "https://sql.example:-1",
		"https://[bad]", "https://[::1", "::1", "https://::1", "https://sql..example",
		"https://-bad.example", "https://bad_.example", "https://sql.example\\evil", "https://sql.example\n",
		" sql.example", "sql.example ", "https://%73ql.example", strings.Repeat("x", 10000),
	} {
		t.Run(input, func(t *testing.T) {
			endpoint, err := parseTelemetrySQLURL(input)
			if err == nil || endpoint != nil || err.Error() != "invalid BETTERSTACK_TELEMETRY_SQL_URL" {
				t.Fatalf("invalid URL did not return a safe diagnostic")
			}
		})
	}
}

// TestParseLogsSearchArgsTable verifies the required full table identifier grammar.
func TestParseLogsSearchArgsTable(t *testing.T) {
	for _, table := range []string{"t1_a", "t123_fake_logs_01", "t0__", "t001_0"} {
		parsed, err := parseLogsSearchArgs([]string{"--table", table})
		if err != nil || parsed.table != table {
			t.Fatalf("table = %q, error = %v", parsed.table, err)
		}
	}
	for _, table := range []string{"", "fake", "t1_", "t_a", "t1_A", "T1_a", "t1_a.b", "t1_a-b", "t1_a;DROP", "t1_a b", " t1_a", "t1_a\n", "t١_a", "t1_é"} {
		t.Run(table, func(t *testing.T) {
			_, err := parseLogsSearchArgs([]string{"--table", table})
			if err == nil || !strings.Contains(err.Error(), "--table") {
				t.Fatalf("error = %v, want --table diagnostic", err)
			}
		})
	}
	if _, err := parseLogsSearchArgs(nil); err == nil || !strings.Contains(err.Error(), "--table") {
		t.Fatalf("missing table error = %v", err)
	}
}

// TestParseLogsSearchArgsSyntax verifies space-separated values and rejects extra arguments.
func TestParseLogsSearchArgsSyntax(t *testing.T) {
	for _, flag := range []string{"--table", "--since", "--from", "--to", "--query", "--order", "--limit", "--timeout"} {
		for _, tail := range [][]string{{flag}, {flag + "=x"}, {flag, "--cold"}} {
			if flag == "--query" && len(tail) == 2 {
				continue
			}
			args := tail
			if flag != "--table" {
				args = append([]string{"--table", "t1_fake"}, tail...)
			}
			_, err := parseLogsSearchArgs(args)
			if err == nil || !strings.Contains(err.Error(), flag) {
				t.Fatalf("args = %q, error = %v", args, err)
			}
		}
	}
	for _, tail := range [][]string{{"--unknown"}, {"positional"}, {""}, {"--cold=true"}, {"--cold", "true"}} {
		if _, err := parseLogsSearchArgs(append([]string{"--table", "t1_fake"}, tail...)); err == nil {
			t.Fatalf("accepted args %q", tail)
		}
	}
}

// TestParseLogsSearchArgsDuplicates verifies every single-value flag rejects repetition first.
func TestParseLogsSearchArgsDuplicates(t *testing.T) {
	values := map[string]string{
		"--table": "t1_fake", "--since": "1h", "--from": "2026-01-01T00:00:00Z", "--to": "2026-01-02T00:00:00Z",
		"--query": "fake", "--order": "asc", "--limit": "1", "--timeout": "1s",
	}
	for flag, value := range values {
		for _, repeated := range [][]string{{flag, value}, {flag, "different"}, {flag}, {flag, "--cold"}} {
			t.Run(flag+"/"+strings.Join(repeated, " "), func(t *testing.T) {
				args := []string{flag, value}
				if flag != "--table" {
					args = append([]string{"--table", "t1_fake"}, args...)
				}
				_, err := parseLogsSearchArgs(append(args, repeated...))
				if err == nil || !strings.Contains(err.Error(), flag) || !strings.Contains(err.Error(), "duplicate") {
					t.Fatalf("error = %v, want duplicate %s", err, flag)
				}
			})
		}
	}
}

// TestParseLogsSearchArgsColdAndQuery verifies an idempotent switch and query presence.
func TestParseLogsSearchArgsColdAndQuery(t *testing.T) {
	parsed, err := parseLogsSearchArgs([]string{"--cold", "--table", "t1_fake", "--cold"})
	if err != nil || !parsed.cold || parsed.query != nil {
		t.Fatalf("cold = %t, query = %v, error = %v", parsed.cold, parsed.query, err)
	}
	for _, query := range []string{"", "fake text"} {
		parsed, err := parseLogsSearchArgs([]string{"--query", query, "--table", "t1_fake"})
		if err != nil || parsed.cold || parsed.query == nil || *parsed.query != query {
			t.Fatalf("query = %v, cold = %t, error = %v", parsed.query, parsed.cold, err)
		}
	}
}

// TestBuildLogsSearchQueryHot verifies the complete default and explicit hot query.
func TestBuildLogsSearchQueryHot(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"default", nil, "SELECT dt, raw FROM remote(t1_fake_logs) WHERE dt >= fromUnixTimestamp64Milli({from:Int64}, 'UTC') AND dt < fromUnixTimestamp64Milli({to:Int64}, 'UTC') ORDER BY dt DESC LIMIT 100 SETTINGS max_execution_time = 30 FORMAT JSONEachRow"},
		{"explicit", []string{"--order", "asc", "--limit", "1000", "--timeout", "2s"}, "SELECT dt, raw FROM remote(t1_fake_logs) WHERE dt >= fromUnixTimestamp64Milli({from:Int64}, 'UTC') AND dt < fromUnixTimestamp64Milli({to:Int64}, 'UTC') ORDER BY dt ASC LIMIT 1000 SETTINGS max_execution_time = 2 FORMAT JSONEachRow"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sql, params, err := buildLogsSearchQuery(append([]string{"--table", "t1_fake"}, tc.args...), time.Unix(3600, 0))
			if err != nil || sql != tc.want {
				t.Fatalf("SQL = %q, error = %v", sql, err)
			}
			if !reflect.DeepEqual(map[string][]string(params), map[string][]string{"param_from": {"0"}, "param_to": {"3600000"}}) {
				t.Fatalf("params = %v", params)
			}
		})
	}
}

// TestBuildLogsSearchQueryText verifies that search text stays in a typed parameter.
func TestBuildLogsSearchQueryText(t *testing.T) {
	for _, text := range []string{"", "  fake  ", "MiXeD fake", "fake 'quotes' \"double\"", `fake\path`, "fake %_.*", "fake'); DROP TABLE fake; --", "--fake", "faké 世界"} {
		t.Run(text, func(t *testing.T) {
			sql, params, err := buildLogsSearchQuery([]string{"--table", "t1_fake", "--query", text}, time.Unix(3600, 0))
			if err != nil || !strings.Contains(sql, "AND positionCaseInsensitiveUTF8(raw, {query:String}) > 0") {
				t.Fatalf("SQL = %q, error = %v", sql, err)
			}
			value, present := params["param_query"]
			if !present || !reflect.DeepEqual(value, []string{text}) || len(params) != 3 {
				t.Fatalf("params = %v", params)
			}
			if text != "" && strings.Contains(sql, text) {
				t.Fatalf("SQL contains query text: %q", sql)
			}
		})
	}
}

// TestBuildLogsSearchQueryRejectsControls verifies control text never produces a query.
func TestBuildLogsSearchQueryRejectsControls(t *testing.T) {
	for _, text := range []string{"fake\x00text", "fake\ntext", "fake\rtext", "fake\ttext", "fake\x1btext", "fake\x7ftext", "fake\u0085text"} {
		sql, params, err := buildLogsSearchQuery([]string{"--table", "t1_fake", "--query", text}, time.Unix(3600, 0))
		if err == nil || !strings.Contains(err.Error(), "--query") || sql != "" || params != nil {
			t.Fatalf("SQL = %q, params = %v, error = %v", sql, params, err)
		}
	}
}

// TestBuildLogsSearchQueryTimeout verifies integer ceiling without duration overflow.
func TestBuildLogsSearchQueryTimeout(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  string
	}{{"1ns", "1"}, {"250ms", "1"}, {"1s", "1"}, {"5s", "5"}, {"5.01s", "6"}, {"2562047h47m16.854775807s", "9223372037"}} {
		sql, _, err := buildLogsSearchQuery([]string{"--table", "t1_fake", "--timeout", tc.value}, time.Unix(3600, 0))
		if err != nil || !strings.Contains(sql, "SETTINGS max_execution_time = "+tc.want+" FORMAT JSONEachRow") {
			t.Fatalf("timeout %s: SQL = %q, error = %v", tc.value, sql, err)
		}
	}
}

// TestBuildLogsSearchQueryRejectsArguments verifies parsing failure returns no SQL or parameters.
func TestBuildLogsSearchQueryRejectsArguments(t *testing.T) {
	for _, args := range [][]string{nil, {"--table", "t1_fake); DROP"}, {"--table", "t1_fake", "--limit", "0"}, {"--table", "t1_fake", "--order", "ASC"}, {"--table", "t1_fake", "--timeout", "0s"}, {"--table", "t1_fake", "--since", "7d"}} {
		sql, params, err := buildLogsSearchQuery(args, time.Unix(3600, 0))
		if err == nil || sql != "" || params != nil {
			t.Fatalf("args = %q, SQL = %q, params = %v, error = %v", args, sql, params, err)
		}
	}
}

// TestBuildLogsSearchQueryTimeBounds verifies fixed-clock windows and RFC3339Nano conversion.
func TestBuildLogsSearchQueryTimeBounds(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		from string
		to   string
	}{
		{"default", nil, "3600123", "7200123"},
		{"half hour", []string{"--since", "30m"}, "5400123", "7200123"},
		{"one hour", []string{"--since", "1h"}, "3600123", "7200123"},
		{"compound", []string{"--since", "2h45m"}, "-2699877", "7200123"},
		{"submillisecond since", []string{"--since", "1ns"}, "7200123", "7200123"},
		{"both", []string{"--from", "1970-01-01T00:00:01Z", "--to", "1970-01-01T00:00:02Z"}, "1000", "2000"},
		{"from only", []string{"--from", "1970-01-01T00:00:01Z"}, "1000", "7200123"},
		{"offset", []string{"--from", "1970-01-01T02:00:00+01:00", "--to", "1970-01-01T03:00:00+01:00"}, "3600000", "7200000"},
		{"nanoseconds", []string{"--from", "1970-01-01T00:00:01.123456789Z", "--to", "1970-01-01T00:00:02.987654321Z"}, "1123", "2987"},
		{"same millisecond", []string{"--from", "1970-01-01T00:00:01.0001Z", "--to", "1970-01-01T00:00:01.0009Z"}, "1000", "1000"},
		{"pre epoch", []string{"--from", "1969-12-31T23:59:59.9991Z", "--to", "1970-01-01T00:00:00.0001Z"}, "-1", "0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sql, params, err := buildLogsSearchQuery(append([]string{"--table", "t1_fake"}, tc.args...), time.Unix(7200, 123456789))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(map[string][]string(params), map[string][]string{"param_from": {tc.from}, "param_to": {tc.to}}) {
				t.Fatalf("params = %v", params)
			}
			if !strings.Contains(sql, "dt >= fromUnixTimestamp64Milli({from:Int64}, 'UTC') AND dt < fromUnixTimestamp64Milli({to:Int64}, 'UTC')") {
				t.Fatalf("SQL = %q", sql)
			}
		})
	}
}

// TestBuildLogsSearchQueryRejectsTimeBounds verifies invalid and mutually exclusive time flags.
func TestBuildLogsSearchQueryRejectsTimeBounds(t *testing.T) {
	sql, params, err := buildLogsSearchQuery([]string{"--table", "t1_fake", "--to", "1970-01-01T01:59:00Z"}, time.Unix(7200, 123456789))
	if err == nil || err.Error() != "--to requires --from" || sql != "" || params != nil {
		t.Fatalf("SQL = %q, params = %v, error = %v, want --to requires --from", sql, params, err)
	}
	for _, tc := range []struct {
		args []string
		flag string
	}{
		{[]string{"--from", ""}, "--from"},
		{[]string{"--from", "1970-01-01"}, "--from"},
		{[]string{"--from", "fake'); DROP TABLE fake; --"}, "--from"},
		{[]string{"--from", "1970-13-01T00:00:00Z"}, "--from"},
		{[]string{"--from", "1970-01-01T00:00:00Z", "--to", ""}, "--to"},
		{[]string{"--from", "1970-01-01T00:00:00Z", "--to", "bad"}, "--to"},
		{[]string{"--from", "1970-01-01T00:00:02Z", "--to", "1970-01-01T00:00:02Z"}, "--from"},
		{[]string{"--from", "1970-01-01T00:00:03Z", "--to", "1970-01-01T00:00:02Z"}, "--from"},
		{[]string{"--from", "1970-01-01T02:00:00.123456789Z"}, "--from"},
		{[]string{"--from", "1970-01-01T03:00:00Z"}, "--from"},
		{[]string{"--since", "1h", "--from", "1970-01-01T00:00:01Z"}, "--since"},
		{[]string{"--since", "1h", "--to", "1970-01-01T00:00:02Z"}, "--since"},
		{[]string{"--from", "1970-01-01T00:00:01Z", "--to", "1970-01-01T00:00:02Z", "--since", "1h"}, "--since"},
	} {
		sql, params, err := buildLogsSearchQuery(append([]string{"--table", "t1_fake"}, tc.args...), time.Unix(7200, 123456789))
		if err == nil || !strings.Contains(err.Error(), tc.flag) || sql != "" || params != nil {
			t.Fatalf("args = %q, SQL = %q, params = %v, error = %v", tc.args, sql, params, err)
		}
	}
}

// TestBuildLogsSearchQueryCold verifies one cold union with shared filters and final clauses.
func TestBuildLogsSearchQueryCold(t *testing.T) {
	for _, tc := range []struct {
		name  string
		flags []string
	}{
		{"once", []string{"--cold"}},
		{"repeated", []string{"--cold", "--cold", "--cold"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args := append([]string{"--table", "t1_fake", "--from", "1970-01-01T00:00:01Z", "--to", "1970-01-01T00:00:02Z", "--query", "fake cold", "--order", "asc", "--limit", "1", "--timeout", "500ms"}, tc.flags...)
			sql, params, err := buildLogsSearchQuery(args, time.Unix(3600, 0))
			want := "SELECT dt, raw FROM (SELECT dt, raw FROM remote(t1_fake_logs) UNION ALL SELECT dt, raw FROM s3Cluster(primary, t1_fake_s3) WHERE _row_type = 1) WHERE dt >= fromUnixTimestamp64Milli({from:Int64}, 'UTC') AND dt < fromUnixTimestamp64Milli({to:Int64}, 'UTC') AND positionCaseInsensitiveUTF8(raw, {query:String}) > 0 ORDER BY dt ASC LIMIT 1 SETTINGS max_execution_time = 1 FORMAT JSONEachRow"
			if err != nil || sql != want {
				t.Fatalf("SQL = %q, error = %v", sql, err)
			}
			if !reflect.DeepEqual(map[string][]string(params), map[string][]string{"param_from": {"1000"}, "param_to": {"2000"}, "param_query": {"fake cold"}}) {
				t.Fatalf("params = %v", params)
			}
			for _, clause := range []string{"remote(", "s3Cluster(", "UNION ALL", "_row_type = 1", "dt >=", "dt <", "positionCaseInsensitiveUTF8", "ORDER BY", "LIMIT", "SETTINGS", "FORMAT"} {
				if strings.Count(sql, clause) != 1 {
					t.Fatalf("SQL has %d occurrences of %q", strings.Count(sql, clause), clause)
				}
			}
		})
	}
}

// TestBuildLogsSearchQueryColdDefaults verifies cold keeps hot defaults and omits text filtering.
func TestBuildLogsSearchQueryColdDefaults(t *testing.T) {
	sql, params, err := buildLogsSearchQuery([]string{"--table", "t1_fake", "--cold"}, time.Unix(3600, 0))
	want := "SELECT dt, raw FROM (SELECT dt, raw FROM remote(t1_fake_logs) UNION ALL SELECT dt, raw FROM s3Cluster(primary, t1_fake_s3) WHERE _row_type = 1) WHERE dt >= fromUnixTimestamp64Milli({from:Int64}, 'UTC') AND dt < fromUnixTimestamp64Milli({to:Int64}, 'UTC') ORDER BY dt DESC LIMIT 100 SETTINGS max_execution_time = 30 FORMAT JSONEachRow"
	if err != nil || sql != want || !reflect.DeepEqual(map[string][]string(params), map[string][]string{"param_from": {"0"}, "param_to": {"3600000"}}) {
		t.Fatalf("SQL = %q, params = %v, error = %v", sql, params, err)
	}
}

// TestParseLogsSearchArgsDefaults verifies the approved search defaults.
func TestParseLogsSearchArgsDefaults(t *testing.T) {
	parsed, err := parseLogsSearchArgs([]string{"--table", "t1_fake"})
	if err != nil || parsed.order != "desc" || parsed.limit != 100 || parsed.since != time.Hour || parsed.timeout != 30*time.Second {
		t.Fatalf("parsed = %+v, error = %v", parsed, err)
	}
}

// TestParseLogsSearchArgsValues verifies allowed order, limit, and duration values.
func TestParseLogsSearchArgsValues(t *testing.T) {
	for _, order := range []string{"asc", "desc"} {
		parsed, err := parseLogsSearchArgs([]string{"--table", "t1_fake", "--order", order})
		if err != nil || parsed.order != order {
			t.Fatalf("order = %q, error = %v", parsed.order, err)
		}
	}
	for _, tc := range []struct {
		value string
		want  int
	}{{"1", 1}, {"1000", 1000}, {"00100", 100}} {
		parsed, err := parseLogsSearchArgs([]string{"--table", "t1_fake", "--limit", tc.value})
		if err != nil || parsed.limit != tc.want {
			t.Fatalf("limit = %d, error = %v", parsed.limit, err)
		}
	}
	for _, flag := range []string{"--since", "--timeout"} {
		for _, tc := range []struct {
			value string
			want  time.Duration
		}{{"1ns", time.Nanosecond}, {"30m", 30 * time.Minute}, {"1h", time.Hour}, {"2h45m", 165 * time.Minute}, {"168h", 168 * time.Hour}} {
			parsed, err := parseLogsSearchArgs([]string{"--table", "t1_fake", flag, tc.value})
			got := parsed.since
			if flag == "--timeout" {
				got = parsed.timeout
			}
			if err != nil || got != tc.want {
				t.Fatalf("%s = %s, error = %v", flag, got, err)
			}
		}
	}
}

// TestParseLogsSearchArgsRejectsValues verifies strict enums, decimal limits, and positive durations.
func TestParseLogsSearchArgsRejectsValues(t *testing.T) {
	values := map[string][]string{
		"--order":   {"", "ASC", "Desc", "newest", " asc"},
		"--limit":   {"", "0", "000", "-1", "+1", "1001", "1.5", "1e2", "one", " 1", "1 ", "١", "99999999999999999999999999999999"},
		"--since":   {"", "7d", "bad", "0", "0s", "-1h", "1", "1e3s", "999999999999999999h"},
		"--timeout": {"", "7d", "bad", "0s", "-1s", "999999999999999999h"},
	}
	for flag, inputs := range values {
		for _, value := range inputs {
			t.Run(flag+"/"+value, func(t *testing.T) {
				_, err := parseLogsSearchArgs([]string{"--table", "t1_fake", flag, value})
				if err == nil || !strings.Contains(err.Error(), flag) {
					t.Fatalf("error = %v, want %s diagnostic", err, flag)
				}
			})
		}
	}
}
