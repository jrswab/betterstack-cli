# Uptime testing

## Injected command boundary

Tests call `runWithAPI` through `invokeAPI` with captured stdout/stderr, a loopback `httptest.NewServer` URL, and an HTTP client. They verify routing, path escaping, bearer headers, query translation, raw output, exit codes, invalid arguments, timeouts, and disabled redirects. [main_test.go](../../cmd/bs/main_test.go) checks exact help forms and invalid usage. [monitors_test.go](../../cmd/bs/monitors_test.go) defines `invokeAPI` and the fixed dummy `testToken`.

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

[TestMain](../../cmd/bs/safety_test.go) removes both `BETTERSTACK_UPTIME_API_TOKEN` and `BETTERSTACK_API_TOKEN` before tests run. Tests set dummy values explicitly. The isolated-process safety test uses fixed sentinel values to verify inherited credential removal and checks that its captured log contains neither sentinel.

`installNetworkGuard` replaces `http.DefaultTransport`. `loopbackOnlyTransport` accepts only literal loopback IPs, including `127.0.0.0/8` and `::1`. It rejects hostnames such as `localhost`, documentation IP ranges, and other addresses before the underlying transport runs. It does not resolve DNS. The cloned standard transport disables proxy lookup and adds `guardDialContext` as a second address check.

The guard covers clients with a nil transport, including `newHTTPClient`, and clients from `testHTTPClient`. A custom `Transport` or `RoundTripper`, replacement default transport, direct connection, or non-HTTP call can bypass it. Tests must not use those paths against non-loopback addresses. `testHTTPClient` supplies an explicitly guarded client when needed.

## Verification

```sh
go test ./...
```

The suite compiles an isolated test binary in a temporary test directory for its credential-removal probe. The production CLI is not needed for test HTTP requests. Related: [commands](commands.md), [requests](requests.md), [incidents](incidents.md), and [summary](summary.md).
