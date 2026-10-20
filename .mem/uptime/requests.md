# Uptime requests

## Credentials and validation

All five [command handlers](commands.md) read only `BETTERSTACK_UPTIME_API_TOKEN` through `tokenEnvVar`. They do not use `BETTERSTACK_API_TOKEN` as a fallback. The handler reads the token before parsing so local argument diagnostics can redact its value. Argument validation, incident target resolution, and duplicate metadata rejection precede the missing-token check. Invalid arguments and a missing token return 1 before HTTP execution. Exact help forms return before any handler reads credentials.

`redactToken` replaces every occurrence of the token with `[REDACTED]` in locally generated argument, request, and response-read diagnostics. It does not redact raw upstream response bodies.

## Request executor

[doMonitorsRequest](../../cmd/bs/monitors.go) serves all five handlers despite its monitor-oriented name. It creates a timeout context, builds an HTTP GET with no body, and sets the bearer header:

```go
req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
```

After checking `err`, the Request executor sets authentication:

```go
req.Header.Set("Authorization", "Bearer "+token)
```

The default timeout is 30 seconds. Each handler can override it with a positive `--timeout <duration>`. The Request executor copies the supplied client and sets `CheckRedirect` to return `http.ErrUseLastResponse`. Redirects remain non-2xx responses even if an injected client normally follows them. It calls `client.Do` on the copy once and adds no retry or pagination loop.

## Response contract

The Request executor reads the entire response with `io.ReadAll` before writing output and closes the body afterward.

| Result | stdout | stderr | Exit |
|---|---|---|---|
| HTTP 200–299, body read succeeds | Raw body bytes | Empty | 0 |
| Other HTTP status, including 3xx | Empty | Raw body bytes | 1 |
| Request construction, transport, timeout, or body-read failure | Empty | Local redacted diagnostic | 1 |

Bodies remain opaque. The CLI does not parse JSON, add a newline, wrap errors, or redact upstream bytes. If a body read fails, it reports the local error rather than printing a partial body. The current Request executor ignores output-writer errors, so its exit code reflects HTTP status and read success, not successful downstream delivery.

Sources: [monitors.go](../../cmd/bs/monitors.go) and [incidents.go](../../cmd/bs/incidents.go). Related: [incident rules](incidents.md), [testing](testing.md), and [summary](summary.md).
