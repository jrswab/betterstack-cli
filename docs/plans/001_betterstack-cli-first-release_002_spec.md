# Spec — 002: `bs monitors get <id>` end-to-end against a mock API

Milestone source: `docs/plans/001_betterstack-cli-first-release_milestones.md` (entry 002, unchecked; 001 is complete). This spec covers milestone 002 only. Milestones 003–007 (list commands, incidents, acceptance sweep) are out of scope and are not built, tested, or broadened here. The planning helper `verify-incident-id.sh` and the `.wayfinder/` tree stay untouched.

## Section 1: Context & Constraints

Sources: the milestone document's research findings, closed wayfinder tickets 01 and 05 with asset `.wayfinder/assets/01-api-contract-research.md`, and the current repository state at HEAD `b2c90314cbd99362c2ecacabe45ff92895373b1c`.

### Codebase state and established patterns (grounded in current files)

- `cmd/bs` (module `github.com/jrswab/betterstack-cli`, `go 1.26`, no `toolchain` directive) delivers the 001 skeleton: `main.go` routes argument vectors through `run(args []string, stdout, stderr io.Writer) int` with injected output writers; help text lives in `help.go`; tests in `main_test.go` call `run` directly through this argv/writer seam (`invoke` helper). This writer-injection pattern is the established test seam; the new HTTP seam must follow the same shape (deterministic injection, no globals, no subprocesses).
- `bs monitors get` currently has argument validation only: exactly one positional target is required (`missing <id>` / `too many arguments` errors exist), and any non-help vector reports `"bs monitors get" is not implemented in this milestone` (main.go `resourceUsageError`). Milestone 002 replaces that not-implemented path for `monitors get`.
- Per-command help for `bs monitors get` already documents `--timeout <duration>` with default 30 seconds and example `bs monitors get 789` (help.go). The help surface is not changed by this milestone.

### Decisions already made (settled — never re-evaluate)

- Endpoint and auth: one bearer-authenticated `GET https://incidents.betterstack.com/api/v2/monitors/{monitor_id}`; `Authorization: Bearer $BETTERSTACK_UPTIME_API_TOKEN` (ticket 01 + asset section 1). Monitor IDs are opaque strings in the API (`"id": "123456789"`); no numeric parsing.
- Success output: the raw 2xx response body prints to stdout byte-for-byte — no parsing, validation, re-serialization, pretty-printing, wrapper, or appended newline; a non-JSON 2xx body still prints raw (ticket 04).
- Error output: every non-2xx response, including 3xx, prints the upstream body to stderr byte-for-byte with no wrapper and no added status summary, and the CLI exits 1; stdout never carries failure output (ticket 05).
- Redirects are not followed: one request to the requested endpoint, so the `Authorization` header never reaches a server-chosen redirect target; no claim is made about whether the API itself redirects (ticket 05, final approval).
- No automatic retries: one API request attempt per invocation; no retry flag, backoff, or `Retry-After` handling (ticket 05).
- Timeout: total request timeout default 30 seconds, overridable per invocation with `--timeout <duration>` (approved example `bs incidents get 123 --timeout 60s`); duration grammar beyond "accepts a duration value" is an implementer choice (ticket 05).
- Missing or empty `BETTERSTACK_UPTIME_API_TOKEN`: an actionable message naming the variable prints to stderr, the CLI exits 1 before any request; there is no fallback to or read of `BETTERSTACK_API_TOKEN` (ticket 05).
- Token safety: the token is never printed, logged, or stored; it is the only credential the CLI reads. Transient in-memory use to build the header and sending it to the intended endpoint are normal operation. No redaction or inspection of response bodies (ticket 05).
- Local/network failures (timeout expiry, connection refused, DNS/TLS failure): one concise, actionable, token-free stderr message, exit 1, nothing on stdout (ticket 05).
- Exit codes: 0 on success, 1 on every failure including invalid usage (ticket 05).
- Rejected approaches that bound this milestone: no `--query` passthrough, no local validation of API limits, no retries, no redirect following, no distinct exit codes, no token-type detection, no pretty-print (milestone document, rejected approaches).

### Stdlib facts verified this session (grounding for acceptance, not requirements)

Inspected from the local go1.26.9 toolchain source (`/home/jaron/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.9.linux-amd64/src/net/http/client.go`, `net/url/url.go`):

- The default `http.Client` follows up to 10 redirects (`defaultCheckRedirect`). Returning `http.ErrUseLastResponse` from `CheckRedirect` stops following and returns the most recent (3xx) response with its body unclosed and a nil error — the mechanism by which "3xx is an ordinary non-2xx failure with raw body passthrough" is implementable without any second request.
- Transport failures surface as `*url.Error` whose `Error()` renders only the operation, the URL (with any password masked by `stripPassword`), and the underlying error — never request headers. Surfacing the wrapped error text therefore cannot leak the `Authorization` header value.

These facts justify the acceptance checks below; they prescribe no implementation.

### Constraints and assumptions

- Every mock-API test must be deterministic and offline: a locally started HTTP test server replaces the real API; only dummy token values (for example `dummy-token-for-tests`) are used. No live API call, real token, or external request happens in any test.
- The test-only seam must not add user-facing configuration: no new flag, no environment variable, no config file for the API base URL or HTTP client (task constraint, consistent with the approved CLI surface).
- The exposed real token's rotation remains Jaron's deferred user action; no session reads it. Every shell starts `unset BETTERSTACK_UPTIME_API_TOKEN`.
- Unknowns not built on: 400/5xx error body shapes, 429 headers/numbers, exact error wording — the single non-2xx rule covers all of them; no test depends on a documented error body beyond it being passed through raw.

## Section 2: Requirements

Each requirement is verifiable through mock-API checks at the existing argv/writer seam plus the new HTTP seam. R-numbers are referenced by the Phase 2 checklist.

### R1 — Request construction (enables the whole milestone; no parallelism within 002)

The CLI reads the credential from the environment variable `BETTERSTACK_UPTIME_API_TOKEN` only and makes exactly one `GET` request to `https://incidents.betterstack.com/api/v2/monitors/{id}` where `{id}` is the positional target, forming exactly one terminal path segment. The request carries `Authorization: Bearer <token>`.
- Verification: a mock server records the received method, path, and header; a success-path test asserts `GET`, `/api/v2/monitors/<target>`, and the bearer header containing the dummy token. No second request occurs for any invocation (see R4).
- Acceptance boundary: the target is passed through verbatim as the path segment; the CLI performs no ID format validation (opaque-string rule). The endpoint host, method, and header are not configurable by the user.

### R2 — Credential failure before any request

When `BETTERSTACK_UPTIME_API_TOKEN` is unset or empty, the CLI prints one readable, actionable message to stderr that names `BETTERSTACK_UPTIME_API_TOKEN`, exits 1, writes nothing to stdout, and makes no HTTP request.
- `BETTERSTACK_API_TOKEN` (the former name) is never read: setting it alone reproduces the missing-token failure exactly.
- Verification: mock server records zero requests under both conditions (unset; set-but-empty; old name set with new name unset).
- Acceptance boundary: ordering with invalid usage — argument validation happens first, so `bs monitors get` with a missing positional reports the usage error regardless of the environment; the token check runs only for a valid argument vector. (Technical decision on authorized judgment: consistent with 001's approved invalid-usage behavior and with "fails before any request".)

### R3 — Success passthrough

On a 2xx response the CLI writes the response body to stdout byte-for-byte, with no appended newline and nothing added or removed; stderr stays empty; exit code is 0.
- Verification: a mock server returns a fixed body containing a trailing-newline-free byte sequence and a non-JSON variant; tests compare exact bytes, assert no trailing newline, and assert empty stderr.
- Acceptance boundary: no validation of the body (2xx non-JSON still prints raw); output equivalence is byte equality, not JSON equality.

### R4 — Non-2xx failure (including 3xx) and single-attempt rule

On any non-2xx response — including every 3xx — the CLI writes the response body to stderr byte-for-byte with nothing added, writes nothing to stdout, exits 1, and does not follow the redirect: exactly one HTTP request reaches the server.
- Verification: a mock server counts requests and returns a 3xx with a fixed body; the test asserts one request, exact body bytes on stderr, empty stdout, exit 1. The same class of check covers representative 401, 404, 429, and 5xx responses with arbitrary (including non-JSON) bodies.
- Acceptance boundary: no status summary, wrapper, or parsing; all non-2xx statuses share one behavior; exit code is 1, never anything else.

### R5 — Timeout default and `--timeout` override

The total request timeout defaults to 30 seconds. `--timeout <duration>` overrides it for the invocation using the Go standard duration grammar (for example `60s`, `500ms`).
- A `--timeout` value the grammar rejects, or a non-positive duration, is invalid usage: readable stderr message, exit 1, no request. (Technical decision on authorized judgment within the approved "duration grammar is an implementer choice": non-positive values cannot bound a request and a silent no-timeout interpretation of `0s` would contradict the default-30-seconds policy.)
- Verification: with a mock server that delays longer than a short `--timeout`, the invocation fails with exit 1, a concise token-free stderr message, empty stdout, and one recorded request; the same server responding faster than the override succeeds. The 30-second default is verified without waiting 30 seconds by the code-level fact that the only timeout source is the default constant or a parsed `--timeout` value (checked in review, not by a 30-second test).
- Acceptance boundary: the timeout bounds the whole request; on expiry there is no retry and no second request.

### R6 — Local and network failure output

When no HTTP response exists (timeout expiry, connection refused, DNS failure, TLS failure), the CLI prints one concise, actionable message to stderr, exits 1, and writes nothing to stdout. The message never contains the token.
- Verification: a mock server reachable at an immediately closed port (connection refused) and a delay server (timeout expiry) both produce exit 1, non-empty stderr, empty stdout. A token-safety test runs the failure paths with a dummy token set and asserts the dummy value appears nowhere on stdout or stderr.
- Acceptance boundary: the message is the complete failure output (no stack traces, no added summaries); exact wording is an implementer choice.

### R7 — Token never printed, logged, or stored

Across success and every failure class, the dummy token value never appears on stdout or stderr in any test. The CLI stores no credential anywhere persistent and reads no credential channel other than `BETTERSTACK_UPTIME_API_TOKEN`.
- Verification: the token-leak assertion runs on the success path, one non-2xx path, and one local-failure path, each with the dummy token in the environment and in the bearer header the mock server observed.
- Acceptance boundary: this is an observable-output rule in tests plus a review check that no file write, log call, or other credential channel exists.

### R8 — Invalid usage unchanged

The 001 usage behavior for `bs monitors get` is preserved: `bs monitors get` with no positional or more than one positional exits 1 with the existing readable errors and no request; `--help` forms still exit 0; other commands (`monitors list`, incidents commands) still report not-implemented with exit 1 and no request.
- Verification: the existing `main_test.go` invalid-usage matrix continues to pass, extended with `--timeout` value cases (R5) and a not-implemented-command case asserting the mock server records zero requests.
- Acceptance boundary: milestone 002 adds no user-facing surface beyond `--timeout` behavior on `monitors get`; no flag list, env var, or config is added.

### R9 — Deterministic test-only injection seam

The HTTP transport layer is reachable from tests through a deterministic test-only injection seam: tests control both the base URL the client targets and the HTTP client used, without network access, subprocesses, sleeps longer than the test timeouts, or flakes. The seam introduces no new user-facing configuration: no flag, no environment variable, and no file affects the base URL or client at runtime.
- Verification: review confirms the seam is test-only; every mock-API test reaches the real request path through it; a runtime run against the real endpoint uses the fixed base URL of R1 with no override channel.
- Acceptance boundary: the seam follows the established 001 pattern of injected collaborators (`run`'s argv/writer injection); tests observe both output channels and the exit code without spawning the binary.

### Parallelism

None: 002 is one vertical slice through credential reading, one request, passthrough, error rules, and timeout. R1–R9 are verified as one end-to-end behavior; the checklist may order test cases within it but the slice lands as a unit.
