# Milestone 005 Specification — S4 SQL search executor

- Milestone source: `docs/plans/008_telemetry-logs-search_milestones.md`, entry `005` (S4)
- Scope: milestone 005 only. Implement and verify SQL URL parsing, SQL HTTP execution, credentials, and response handling. Do not activate `logs search` command dispatch or add help text. The executor may consume the existing query-builder output.
- Acceptance in one line: the SQL executor sends one bounded, authenticated POST to the approved SQL endpoint, follows no redirects, retries nothing, passes 2xx bytes unchanged to stdout, and redacts secrets from non-2xx bodies sent to stderr with exit status 1.

## Section 1: Context & Constraints

### Repository and milestone boundary

The Go module is rooted at `/home/jaron/dev/betterstack-cli/master`. The command package is `cmd/bs`, package `main`, and it uses the standard library. `runWithEndpoints` in `cmd/bs/main.go` accepts separate Uptime, Telemetry REST, and Telemetry SQL endpoint inputs. The SQL input is currently unused. `runWithAPI` keeps its current signature and delegates to that seam. `buildLogsSearchQuery` in `cmd/bs/logs.go` already returns SQL text, `url.Values` query parameters, and a local error. `redactSecrets` in `cmd/bs/telemetry.go` already replaces the Telemetry token, SQL username, SQL password, and derived Basic-auth value. No current response path calls it.

Milestone 005 owns the executor and its focused tests. It may connect builder output to that executor, but it must not add `logs search` dispatch or help. Milestone 006 owns the user-facing route and help. D5 requires an `httptest` proving SQL requests use the SQL endpoint injected through the `telemetrySQLURL` seam, distinct from Uptime and Telemetry REST endpoints. Preserve existing `runWithAPI` callers and signature. Do not change the shared Uptime/source request contract. Do not add a dependency.

The full lane applies under D1 because this milestone changes an external HTTP contract and handles credentials. Tests use fake strings and `httptest` only. They must not make live requests, use real credentials, or bypass the test network guard with a custom transport. The repository test harness clears inherited API credentials and permits guarded loopback HTTP. The standard verification commands are `GOFLAGS=-o=/dev/null go build ./...`, `go vet ./...`, `go test -count=1 ./...`, and `gofmt -l cmd/bs/*.go`.

### Approved SQL endpoint and authentication contract

Tickets 02 and 10 define the separate environment variables `BETTERSTACK_TELEMETRY_SQL_URL`, `BETTERSTACK_TELEMETRY_SQL_USERNAME`, and `BETTERSTACK_TELEMETRY_SQL_PASSWORD`. There is no fallback to `BETTERSTACK_UPTIME_API_TOKEN` or `BETTERSTACK_TELEMETRY_API_TOKEN`. Ticket 07 allows the SQL host as the only user-configurable API host for Telemetry. The Uptime and Telemetry REST hosts remain fixed.

Ticket 10 O5 approves these SQL URL forms: `host`, `host:443`, and `https://host[:port]`. A host-only form receives the `https` scheme. Reject every other scheme, a path other than empty or `/`, any query, any fragment, and URL userinfo. The request URL uses the approved host and the builder's encoded `param_*` values as its query string. URL parsing must not allow malformed or unsafe input to become a request target.

Use HTTP Basic authentication with the two SQL credential values. For deterministic local behavior, both SQL credential variables must be set to non-empty values before a request is sent. Missing or empty SQL configuration fails locally with exit status 1, no request, and no credential values in diagnostics. Do not use either Telemetry API token as SQL authentication. Document these SQL environment variables in `README.md` if milestone 005 implements their runtime use.

### Approved request and response behavior

Ticket 04 documents `POST`, HTTP Basic authentication, the SQL request body, `Content-Type: plain/text`, and `FORMAT JSONEachRow`. Ticket 05 confirms that the API accepts both `plain/text` and `text/plain`, forwards `param_*` values, and accepts the server setting emitted by the builder. Use the documented `plain/text` value. Ticket 10 O3 records parameter forwarding. Send the builder's SQL as the POST body and its `url.Values` separately in the URL query string.

Each invocation sends exactly one POST. Set the request timeout from the search's existing positive `--timeout` value, whose default is 30 seconds. Do not follow redirects, retry, add client-side concurrency control, or inspect HTTP 200 bodies for embedded errors. Use one client request path and disable redirects even when the caller supplies a client with redirect behavior. The documented four-query concurrency limit remains server-enforced (tickets 04 and 11).

For every 2xx response, write the complete response body byte-for-byte to stdout. Do not redact successful data, decode it, format it, or add a newline. For every non-2xx response, apply `redactSecrets` to the response body only, write the resulting bytes to stderr, and return exit status 1. Do not redact the submitted SQL text or wrap the upstream error body. Transport, request-construction, and response-read failures return exit status 1 and a safe local diagnostic without credential values. Do not call `redactSecrets` for 2xx bodies or unrelated source/Uptime responses. Ticket 11 D6 expressly limits response-body redaction to non-2xx SQL responses.

### Decisions, constraints, and deferred work

The handoff decisions apply unchanged: D1 assigns the full lane; D5 requires proof that the SQL endpoint injection seam reaches the SQL request; D6 requires `redactSecrets` on non-2xx SQL bodies only. D4 fixes the future execution changelog fragment name as `changelog.d/008-telemetry-logs-search-005.md`. D8 requires direct-check scripts under `/tmp/factory-005/` with quoted heredocs. D9 requires an independent review gate in execution.

Ticket 02's original rejection of ClickHouse parameter binding is superseded by ticket 10 O3 and live evidence in ticket 05. Ticket 02's generic redaction wording is narrowed by ticket 11: only non-2xx SQL response bodies are redacted in this milestone. Ticket 11 rejects retries, redirect following, response-body parsing, an HTTP 200 error detector, SQL masking, and a client-side concurrency guard. Ticket 07's SQL host exception does not make other endpoint hosts configurable.

The user confirmed that prior ticket review found no unresolved product decisions. This specification resolves the remaining implementation detail that both SQL Basic-auth values must be non-empty before request execution. No live verification is authorized or needed. `logs search` route activation, command help, and any route-specific usage text remain assigned to milestone 006. Repository records required for implementation belong in the later execution checklist.

## Section 2: Requirements

### R1 — Parse only approved SQL URL forms

Accept a host, `host:443`, or an absolute `https://host[:port]` URL. Add `https://` to an approved host-only form. For absolute URLs, allow only an empty path or `/`. Reject a non-HTTPS scheme, unsupported raw host-port form, empty or malformed host, invalid port, userinfo, any query, any fragment, or any other path. Return a parsed endpoint suitable for adding the builder's request parameters. Fail locally before any request and do not include credentials in diagnostics.

**Acceptance:** table-driven tests accept each approved URL form, including valid explicit HTTPS ports and the optional `/` path. Tests reject `http://`, other schemes, empty input, host with a non-root path, query, fragment, userinfo, malformed host/port, and unsupported raw `host:port`. Invalid forms produce no request and do not expose secret values.

### R2 — Require separate SQL credentials

Read SQL Basic-auth values only from `BETTERSTACK_TELEMETRY_SQL_USERNAME` and `BETTERSTACK_TELEMETRY_SQL_PASSWORD`. Require both values to be non-empty before sending a request. Do not fall back to either API token. Configure HTTP Basic authentication from those two values, not a bearer header. A missing URL or credential fails locally with exit status 1, no request, and no secret in output.

**Acceptance:** tests cover each unset/empty SQL variable, both values set, and dummy Uptime/Telemetry API tokens present without SQL credentials. Local failures send no request and diagnostics identify the missing variable without printing any credential value. A successful request carries the expected Basic-auth header and no Bearer header.

### R3 — Send one POST with the approved request shape and timeout

Send the builder's SQL text as the request body and its `url.Values` as encoded URL query parameters. Use exactly one `POST`, set `Content-Type` to `plain/text`, and apply the builder's positive search timeout to the request context. Preserve the approved SQL endpoint as the request destination. Do not add retries, a client-side concurrency guard, or another SQL request.

**Acceptance:** an `httptest` handler asserts method, request body, content type, encoded query values, Basic-auth header, and a positive request deadline. It observes exactly one request. The test uses distinct injected service endpoints and proves that SQL traffic uses `telemetrySQLURL`, not the Uptime or Telemetry REST endpoint.

### R4 — Do not follow redirects or retry

Disable redirects on the client used for SQL execution, even if the supplied client has a redirect callback. A redirect response is handled as the original non-2xx response when its status is outside 2xx. Do not issue another request after a redirect or any other response. Do not implement explicit retry behavior for transport errors or HTTP statuses.

**Acceptance:** a loopback redirect test observes one request at the SQL server, zero requests at the redirect target, the redirect body on stderr, and exit status 1. A request-counting test confirms that server errors and transport failures do not trigger a second attempt.

### R5 — Pass successful response bytes unchanged

For every 2xx status, including an empty body, write the response body unchanged to stdout and return 0. Write nothing to stderr. Do not decode, format, redact, or append a newline. Do not inspect successful bodies for embedded errors.

**Acceptance:** `httptest` cases cover representative 2xx statuses, JSON-looking text, non-JSON bytes, NUL, a trailing newline, and an empty body. Each case compares exact output bytes and confirms one request.

### R6 — Redact only non-2xx SQL response bodies

For every non-2xx status, pass the complete response body through `redactSecrets` using the SQL credential values and the Telemetry API token, then write the redacted body to stderr and return 1. Write nothing to stdout. Preserve other body bytes and the helper's established literal replacement behavior. Do not redact 2xx SQL bodies, Uptime responses, or source responses. Do not mask submitted SQL or add an error wrapper.

**Acceptance:** fake secret strings for the Telemetry API token, username, password, and derived Basic-auth encoding are absent from non-2xx stderr output. Unrelated text and newlines remain intact. A matching 2xx body remains byte-identical, including fake secret strings. Tests assert status, output channel, exit code, and one request.

### R7 — Handle local request failures safely

Request construction, timeout, transport, and response-body read failures return 1, write no success output, and produce a local diagnostic that does not contain SQL credentials or API tokens. A timeout cancels the request context. Local errors do not trigger a retry. Apply `redactSecrets` only to non-2xx SQL response bodies as required by R6.

**Acceptance:** loopback tests cover a timed-out request and a response-body read failure. A closed loopback listener or deterministic fake error path covers transport failure. Each case confirms exit 1, no stdout, no credential leakage, and no second request.

### R8 — Keep milestone 005 separate from command activation

The SQL executor can consume the existing query-builder result and the SQL endpoint supplied through the `runWithEndpoints` test seam. Do not add `logs search` command dispatch, root/resource/command help, or route-specific usage behavior. Keep `runWithAPI`'s existing signature and preserve all Uptime and source behavior.

**Acceptance:** tests prove SQL endpoint injection at the executor boundary without making the `logs search` CLI route available. Existing route and help tests remain unchanged and pass. Diff inspection finds no new logs command route or help text.

### Requirement sources

- R1–R3: [ticket 02](../../.wayfinder/telemetry/tickets/02-technical-design-decisions-recorded-at-charting.md), [ticket 04](../../.wayfinder/telemetry/tickets/04-research-sql-api-contract-from-docs.md), [ticket 05](../../.wayfinder/telemetry/tickets/05-task-live-verification-sql-api-log-shape.md), [ticket 07](../../.wayfinder/telemetry/tickets/07-grilling-sql-host-override-policy.md), and [ticket 10](../../.wayfinder/telemetry/tickets/10-technical-decisions-from-live-verification.md).
- R3–R7: [ticket 11](../../.wayfinder/telemetry/tickets/11-sql-api-errors-concurrency-time-bounds.md), [ticket 04](../../.wayfinder/telemetry/tickets/04-research-sql-api-contract-from-docs.md), and [ticket 05](../../.wayfinder/telemetry/tickets/05-task-live-verification-sql-api-log-shape.md).
- R8 and the milestone boundary: [milestone 005](008_telemetry-logs-search_milestones.md), the current `runWithEndpoints` implementation in `cmd/bs/main.go`, and handoff decisions D5 and D6 in `/tmp/betterstack-cli-factory-008-handoff.md`.

## Ordering and parallel work

SQL URL parsing, credential validation, and the request executor form one vertical slice. Implement local validation before request construction. Then test the request contract and response branches with loopback `httptest` servers. Connect the builder output to the executor only after the isolated request boundary passes. Add README environment-variable documentation and repository records after runtime behavior is established. Final verification and the independent review gate follow those tasks. No implementation tasks can run in parallel because the endpoint, credentials, and built query converge in one request.

## Unresolved questions

None. Ticket decisions and user clarifications settle product behavior. The implementation assumptions are explicit above and do not require another product decision.
