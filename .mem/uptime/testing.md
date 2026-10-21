# Uptime testing

## Command entry points

[main_test.go](../../cmd/bs/main_test.go) uses `invoke` → `run(args, stdout, stderr)` to capture output for exact help forms and invalid usage. `run` supplies the production base URL and `newHTTPClient()` before delegating to `runWithAPI` in [main.go](../../cmd/bs/main.go). It does not inject a loopback endpoint.

[Commands](commands.md) documents usage diagnostics and the resource gate that later resources must extend.

Routing tests use `invokeAPI` to call `runWithAPI` with captured stdout/stderr, a loopback `httptest.NewServer` URL, and an HTTP client. `runWithAPI` delegates to `runWithEndpoints` without changing its signature. `main_test.go` also calls `runWithEndpoints` directly with three distinct loopback endpoint inputs. It verifies all five Uptime routes use the Uptime input, not the Telemetry REST or SQL inputs, and that help and invalid usage send no requests through either interface. Source tests also verify Telemetry REST routing, with zero Uptime or SQL requests. The SQL input supports [prepared SQL execution](../telemetry/logs.md) without a CLI route. [monitors_test.go](../../cmd/bs/monitors_test.go) defines `invokeAPI` and the fixed dummy `testToken`. Other tests call handlers directly through helpers such as `invokeMonitorsList` and `invokeIncidentsList`. These tests verify path escaping, bearer headers, query translation, raw output, exit codes, invalid arguments, timeouts, and disabled redirects.

The in-memory `TestRunWithAPIUsesFixedTelemetryHost` transport records production routing without dialing or delegating to another transport. It asserts the exact Telemetry base URL, HTTPS source host and path, separate bearer credentials, and injected Uptime URL. [TestLogsSQLUsesConfiguredEndpoint](../../cmd/bs/logs_test.go) uses the same transport to verify the configured SQL scheme, host, path, and builder query parameters. It checks every request header value for the fake Telemetry API token.

This existing test pattern authenticates only to a local mock server:

```go
srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"data":{}}`))
}))
defer srv.Close()
t.Setenv(tokenEnvVar, testToken)
code, _, stderr := invokeAPI(srv.URL, newHTTPClient(), "monitors", "get", "789")
```

[incidents_test.go](../../cmd/bs/incidents_test.go) covers dashboard URL acceptance/rejection, metadata bracket encoding, duplicate-key rejection, and timeline passthrough. No test needs a live API response or an ambient credential.

## Credential and network protections

[TestMain](../../cmd/bs/safety_test.go) removes six environment variables before tests run: `BETTERSTACK_UPTIME_API_TOKEN`, `BETTERSTACK_API_TOKEN`, `BETTERSTACK_TELEMETRY_API_TOKEN`, `BETTERSTACK_TELEMETRY_SQL_URL`, `BETTERSTACK_TELEMETRY_SQL_USERNAME`, and `BETTERSTACK_TELEMETRY_SQL_PASSWORD`. Tests set dummy values explicitly. The isolated-process safety test uses six fixed sentinel values to verify inherited credential and SQL configuration removal. It checks that its captured log contains none of the six sentinels.

`installNetworkGuard` replaces `http.DefaultTransport`. `loopbackOnlyTransport` accepts only literal loopback IPs, including `127.0.0.0/8` and `::1`. It rejects hostnames such as `localhost`, documentation IP ranges, and other addresses before the underlying transport runs. It does not resolve DNS. The cloned standard transport disables proxy lookup and adds `guardDialContext` as a second address check.

The guard covers clients with a nil transport, including `newHTTPClient`, and clients from `testHTTPClient`. A custom `Transport` or `RoundTripper`, replacement default transport, direct connection, or non-HTTP call can bypass it. Tests must not make network requests through those paths to non-loopback addresses. Recording transports return in-memory responses without dialing or delegation. `testHTTPClient` supplies an explicitly guarded client when needed.

## Verification

```sh
go test ./...
```

The suite compiles an isolated test binary in a temporary test directory for its credential-removal probe. The production CLI is not needed for test HTTP requests. Related: [commands](commands.md), [requests](requests.md), [incidents](incidents.md), and [summary](summary.md).
