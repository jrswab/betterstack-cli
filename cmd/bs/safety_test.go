// Test-only safety harness. This file adds no production behavior.
//
// Two independent defenses keep the test suite off the real network:
//
//  1. TestMain removes the credential environment variables before any test
//     runs. Without a token the CLI fails before it builds a request, so an
//     inherited credential can never authenticate a stray test.
//  2. installNetworkGuard replaces http.DefaultTransport with a transport
//     that hard-fails every request whose host is not a literal loopback IP
//     (127.0.0.0/8 or ::1). It never resolves DNS, it disables proxy lookup,
//     and it rejects before the dialer is reached.
//
// Coverage and limits:
//
//   - Covered: any net/http client whose Transport is nil (http.DefaultTransport),
//     which includes newHTTPClient() and run/runWithAPI, plus the
//     testHTTPClient() helper.
//   - Not covered: a test that injects its own http.Transport or
//     http.RoundTripper, replaces http.DefaultTransport, dials net.Conn
//     directly, or uses a non-HTTP network call. Future tests must not do any
//     of those against a non-loopback address. Use testHTTPClient() instead of
//     a hand-built client so the guard applies.
//   - Only literal loopback IPs pass. Hostnames (including localhost), the
//     RFC 5737 documentation ranges, and every other address are rejected.

package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// unguardedTransport is the process's original http.DefaultTransport,
// captured before installNetworkGuard replaces the package variable. Cloning
// it keeps newLoopbackOnlyTransport idempotent and stops the guard from
// wrapping itself.
var unguardedTransport = http.DefaultTransport

// TestMain removes inherited credentials, installs the loopback-only network
// guard, then runs the suite.
func TestMain(m *testing.M) {
	if err := clearCredentialEnv(); err != nil {
		log.Fatalf("safety_test: %v", err)
	}
	installNetworkGuard()
	os.Exit(m.Run())
}

// clearCredentialEnv removes the two credential variables the CLI could read.
// It reports an error instead of dumping the environment.
func clearCredentialEnv() error {
	for _, name := range []string{tokenEnvVar, "BETTERSTACK_API_TOKEN"} {
		if err := os.Unsetenv(name); err != nil {
			return fmt.Errorf("could not unset %s: %w", name, err)
		}
	}
	return nil
}

// loopbackOnlyTransport rejects every request whose host is not a literal
// loopback IP before delegating to the base transport.
type loopbackOnlyTransport struct {
	base http.RoundTripper
}

func (t *loopbackOnlyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if err := requireLoopbackHost(req.URL.Hostname()); err != nil {
		return nil, err
	}
	return t.base.RoundTrip(req)
}

// requireLoopbackHost accepts only literal loopback IPs. It never resolves
// DNS, so hostnames such as localhost or example.com are rejected, and it
// rejects the RFC 5737 documentation ranges like any other non-loopback IP.
func requireLoopbackHost(host string) error {
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("safety_test: blocked non-loopback request to %q; tests may reach literal loopback IPs only", host)
	}
	return nil
}

// guardDialContext wraps a dialer so it also rejects non-loopback addresses.
// It is defense in depth behind loopbackOnlyTransport.RoundTrip.
func guardDialContext(base func(context.Context, string, string) (net.Conn, error)) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, _, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, fmt.Errorf("safety_test: blocked malformed dial address %q: %w", addr, err)
		}
		if err := requireLoopbackHost(host); err != nil {
			return nil, err
		}
		return base(ctx, network, addr)
	}
}

// newLoopbackOnlyTransport clones the original default transport, disables
// proxy lookup so HTTP_PROXY cannot route a blocked request around the guard,
// and installs both the request-host and dial-address checks.
func newLoopbackOnlyTransport() *loopbackOnlyTransport {
	base, ok := unguardedTransport.(*http.Transport)
	if !ok {
		return &loopbackOnlyTransport{base: unguardedTransport}
	}
	guarded := base.Clone()
	guarded.Proxy = nil
	dial := guarded.DialContext
	if dial == nil {
		dial = (&net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}).DialContext
	}
	guarded.DialContext = guardDialContext(dial)
	return &loopbackOnlyTransport{base: guarded}
}

// installNetworkGuard makes every nil-Transport http.Client use the guard.
func installNetworkGuard() {
	http.DefaultTransport = newLoopbackOnlyTransport()
}

// testHTTPClient is the guarded client future tests must use when they need a
// real client. A hand-built client with a custom Transport bypasses the guard.
func testHTTPClient() *http.Client {
	return &http.Client{Transport: newLoopbackOnlyTransport()}
}

// spyRoundTripper records whether the wrapped base transport was reached.
type spyRoundTripper struct {
	calls int
	err   error
}

func (s *spyRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	s.calls++
	return nil, s.err
}

// TestLoopbackOnlyTransportRejectsBeforeBase proves a blocked host fails
// before the base transport is reached, so no dial can happen.
func TestLoopbackOnlyTransportRejectsBeforeBase(t *testing.T) {
	hosts := []string{
		"incidents.betterstack.com",
		"localhost",
		"example.com",
		"192.0.2.1",    // RFC 5737 TEST-NET-1
		"198.51.100.7", // RFC 5737 TEST-NET-2
		"203.0.113.9",  // RFC 5737 TEST-NET-3
		"10.0.0.1",
		"8.8.8.8",
		"2001:db8::1", // documentation IPv6, not loopback
	}
	for _, host := range hosts {
		t.Run(host, func(t *testing.T) {
			spy := &spyRoundTripper{}
			tr := &loopbackOnlyTransport{base: spy}
			req := httptest.NewRequest(http.MethodGet, "http://"+net.JoinHostPort(host, "80")+"/", nil)
			_, err := tr.RoundTrip(req)
			if err == nil {
				t.Fatalf("RoundTrip to %q returned no error, want rejection", host)
			}
			if !strings.Contains(err.Error(), "non-loopback") {
				t.Fatalf("RoundTrip to %q error = %q, want a non-loopback rejection", host, err)
			}
			if spy.calls != 0 {
				t.Fatalf("base transport reached %d times for %q, want 0", spy.calls, host)
			}
		})
	}
}

// TestLoopbackOnlyTransportAllowsLoopback proves literal loopback IPs pass.
func TestLoopbackOnlyTransportAllowsLoopback(t *testing.T) {
	hosts := []string{"127.0.0.1", "127.0.0.5", "::1", "::ffff:127.0.0.1"}
	for _, host := range hosts {
		t.Run(host, func(t *testing.T) {
			spy := &spyRoundTripper{}
			tr := &loopbackOnlyTransport{base: spy}
			req := httptest.NewRequest(http.MethodGet, "http://"+net.JoinHostPort(host, "80")+"/", nil)
			if _, err := tr.RoundTrip(req); err != nil {
				t.Fatalf("RoundTrip to loopback %q failed: %v", host, err)
			}
			if spy.calls != 1 {
				t.Fatalf("base transport reached %d times for %q, want 1", spy.calls, host)
			}
		})
	}
}

// TestGuardDialContextRejectsNonLoopback proves the dial-address guard is
// defense in depth: a non-loopback address fails before the base dialer runs.
func TestGuardDialContextRejectsNonLoopback(t *testing.T) {
	baseCalls := 0
	base := func(context.Context, string, string) (net.Conn, error) {
		baseCalls++
		return nil, errors.New("base dial reached")
	}
	dial := guardDialContext(base)

	if _, err := dial(context.Background(), "tcp", "192.0.2.1:443"); err == nil {
		t.Fatal("dial to an RFC 5737 address returned no error, want rejection")
	}
	if baseCalls != 0 {
		t.Fatalf("base dial reached %d times for a blocked address, want 0", baseCalls)
	}

	if _, err := dial(context.Background(), "tcp", "127.0.0.1:443"); err == nil {
		t.Fatal("dial to loopback returned no error, want pass-through")
	}
	if baseCalls != 1 {
		t.Fatalf("base dial reached %d times for loopback, want 1", baseCalls)
	}
}

// TestSafetyGuardInstalledOnDefaultTransport proves TestMain installed the
// guard on the package transport every nil-Transport client uses.
func TestSafetyGuardInstalledOnDefaultTransport(t *testing.T) {
	if _, ok := http.DefaultTransport.(*loopbackOnlyTransport); !ok {
		t.Fatalf("http.DefaultTransport = %T, want *loopbackOnlyTransport", http.DefaultTransport)
	}
}

// safetyProbeEnv marks the isolated subprocess run by
// TestSafetyHarnessStripsInheritedCredentials.
const safetyProbeEnv = "BS_SAFETY_PROBE"

// TestSafetyGuardBlocksImplicitProductionClient proves the production client
// (nil Transport) cannot reach the fixed Better Stack host: the guard rejects
// the hostname before the base transport or dialer is reached.
func TestSafetyGuardBlocksImplicitProductionClient(t *testing.T) {
	t.Setenv(tokenEnvVar, testToken)

	code, stdout, stderr := invokeAPI(defaultAPIBaseURL, newHTTPClient(), "incidents", "get", "123")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "non-loopback") {
		t.Fatalf("stderr = %q, want a non-loopback rejection", stderr)
	}
}

// TestSafetyGuardBlocksProductionRunWithDummyToken proves run() (the full
// production path: default endpoint and newHTTPClient) cannot send even when a
// dummy token is present.
func TestSafetyGuardBlocksProductionRunWithDummyToken(t *testing.T) {
	t.Setenv(tokenEnvVar, testToken)

	code, stdout, stderr := invoke("incidents", "get", "123")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "non-loopback") {
		t.Fatalf("stderr = %q, want a non-loopback rejection", stderr)
	}
}

// TestSafetyGuardRejectsExplicitNonLoopbackURL proves an explicitly injected
// non-loopback base URL is rejected, including the RFC 5737 documentation
// ranges and hostnames such as localhost.
func TestSafetyGuardRejectsExplicitNonLoopbackURL(t *testing.T) {
	t.Setenv(tokenEnvVar, testToken)

	baseURLs := []string{
		"http://192.0.2.1:8080",
		"http://198.51.100.7",
		"http://203.0.113.9:9000",
		"http://10.0.0.1",
		"https://example.com",
		"http://localhost:8080",
	}
	for _, baseURL := range baseURLs {
		t.Run(baseURL, func(t *testing.T) {
			code, stdout, stderr := invokeAPI(baseURL, testHTTPClient(), "monitors", "get", "789")
			if code != 1 {
				t.Fatalf("exit code = %d, want 1", code)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, "non-loopback") {
				t.Fatalf("stderr = %q, want a non-loopback rejection", stderr)
			}
		})
	}
}

// TestSafetyGuardAllowsLoopbackMock proves the guard still lets a loopback
// mock server serve the implicit production client.
func TestSafetyGuardAllowsLoopbackMock(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":{"id":"1"}}`))
	}))
	defer srv.Close()

	t.Setenv(tokenEnvVar, testToken)

	code, stdout, stderr := invokeAPI(srv.URL, newHTTPClient(), "monitors", "get", "789")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr)
	}
	if stdout != `{"data":{"id":"1"}}` {
		t.Fatalf("stdout = %q, want the raw body", stdout)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
}

// envWithoutCredentials returns the current environment with both credential
// variables removed. It never reads or returns their values.
func envWithoutCredentials() []string {
	out := make([]string, 0, len(os.Environ()))
	for _, entry := range os.Environ() {
		name, _, ok := strings.Cut(entry, "=")
		if ok && (name == tokenEnvVar || name == "BETTERSTACK_API_TOKEN") {
			continue
		}
		out = append(out, entry)
	}
	return out
}

// TestSafetyHarnessStripsInheritedCredentials compiles the package's test
// binary and runs one probe test with sentinel credentials in its environment.
// The probe (running under TestMain) must observe both variables as unset, and
// the captured log must not contain the sentinel values. No ambient credential
// is read or printed.
func TestSafetyHarnessStripsInheritedCredentials(t *testing.T) {
	const (
		uptimeSentinel = "sentinel-uptime-token-do-not-use"
		legacySentinel = "sentinel-legacy-token-do-not-use"
		probeTestName  = "TestSafetyHarnessIsolatedProcessSeesNoCredentials"
	)

	dir := t.TempDir()
	bin := filepath.Join(dir, "bs-safety-probe.test")

	build := exec.Command("go", "test", "-c", "-o", bin, ".")
	build.Env = envWithoutCredentials()
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("compiling the isolated test binary failed: %v\n%s", err, out)
	}

	logPath := filepath.Join(dir, "probe.log")
	logFile, err := os.Create(logPath)
	if err != nil {
		t.Fatalf("creating the probe log failed: %v", err)
	}

	run := exec.Command(bin, "-test.run=^"+probeTestName+"$", "-test.v")
	run.Env = append(envWithoutCredentials(),
		safetyProbeEnv+"=1",
		tokenEnvVar+"="+uptimeSentinel,
		"BETTERSTACK_API_TOKEN="+legacySentinel,
	)
	run.Stdout = logFile
	run.Stderr = logFile
	runErr := run.Run()
	if closeErr := logFile.Close(); closeErr != nil {
		t.Fatalf("closing the probe log failed: %v", closeErr)
	}

	logBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("reading the probe log failed: %v", err)
	}
	logText := string(logBytes)

	// Check for leaked sentinels before any failure path prints the log.
	for _, sentinel := range []string{uptimeSentinel, legacySentinel} {
		if strings.Contains(logText, sentinel) {
			t.Fatal("the probe log contains a sentinel credential value; the harness must never print credentials")
		}
	}
	if runErr != nil {
		t.Fatalf("the isolated probe failed: %v\n%s", runErr, logText)
	}
}

// TestSafetyHarnessIsolatedProcessSeesNoCredentials runs only in the isolated
// child process compiled by TestSafetyHarnessStripsInheritedCredentials. The
// child receives sentinel credentials in its environment; TestMain must have
// removed them before this test runs.
func TestSafetyHarnessIsolatedProcessSeesNoCredentials(t *testing.T) {
	if os.Getenv(safetyProbeEnv) != "1" {
		t.Skip("isolated-process probe; run through TestSafetyHarnessStripsInheritedCredentials")
	}
	for _, name := range []string{tokenEnvVar, "BETTERSTACK_API_TOKEN"} {
		if os.Getenv(name) != "" {
			t.Fatalf("inherited credential %s survived TestMain", name)
		}
	}
}
