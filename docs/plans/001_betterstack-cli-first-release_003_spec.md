# Milestone 003 Specification — `bs monitors list` with pagination and monitor filters

Milestone source: `docs/plans/001_betterstack-cli-first-release_milestones.md`, entry 003 (unchecked). This spec covers milestone 003 only; incidents commands, monitors get behavior changes, and all other milestones are out of scope.

## Section 1: Context & Constraints

### Codebase state and established patterns

- Milestones 001 and 002 are complete. `cmd/bs/main.go` exposes the test seam `runWithAPI(args []string, stdout, stderr io.Writer, baseURL string, client *http.Client) int`; tests inject a loopback base URL and client (`cmd/bs/main_test.go`, `cmd/bs/monitors_test.go` `invokeAPI`). Every request stays on a local mock server; no flag, environment variable, or file changes the injection at runtime.
- `cmd/bs/monitors.go` implements `monitorsGet`: token read from `BETTERSTACK_UPTIME_API_TOKEN` (`tokenEnvVar`), bearer header, one `GET`, redirect following disabled via `http.ErrUseLastResponse`, total timeout from `--timeout` (default `defaultTimeout` 30 seconds), raw success body written with `stdout.Write` (no appended newline), raw non-2xx body written to stderr, exit codes 0/1 only, and `redactToken` applied only to locally generated diagnostics — never to a passthrough body.
- `bs monitors list` currently answers "not implemented" through `resourceUsageError` (`cmd/bs/main.go`). Its `--help` already exists (`monitorsListHelp` in `cmd/bs/help.go`) and documents `--url`, `--pronounceable-name`, `--team-name`, `--page`, `--per-page`, and `--timeout` with passthrough wording. `TestUnimplementedCommandsMakeNoRequest` (`cmd/bs/monitors_test.go`) currently includes the `monitors list` vector as not-implemented; that case becomes a working command in this milestone.
- Verification environment: `GOTOOLCHAIN=go1.26.9 GOPROXY=off` from the local module cache (local default `go` is 1.22.2, below the module minimum `go 1.26`).

### Decisions already made (settled; never re-evaluate)

Sources: `.wayfinder/tickets/03-command-naming-and-argument-conventions.md` (Resolution), `.wayfinder/tickets/04-pagination-and-json-envelope.md` (Resolution), `.wayfinder/tickets/05-errors-timeouts-retries-token-safety.md` (Resolution), `.wayfinder/assets/01-api-contract-research.md`.

- Endpoint: `GET https://incidents.betterstack.com/api/v2/monitors` (ticket 01; monitors are v2). List monitors paginates with `page`/`per_page`, body `pagination` object of absolute URLs, documented default 50 and maximum 250 per page (asset 01, sections 4–5).
- Success envelope: raw byte-for-byte passthrough of the response body to stdout — no parsing, validation, re-serialization, pretty-printing, compact mode, `--pretty` flag, or appended newline (ticket 04). `data`, `included`, and `pagination` pass through as the API sent them. The only compatibility promise is that `bs` preserves the response as received.
- Pagination: one API page by default. Optional `--page <n>` and `--per-page <n>` map one-to-one onto `page` and `per_page`. Given values pass through unchanged for upstream validation — no hardcoded limits, no clamping, no substitution, no pre-request rejection, and no local numeric grammar beyond passthrough. Omitting a flag sends no parameter, so the API default applies (ticket 04).
- No auto-drain: the CLI never follows `pagination.next`; no `--all`-style flag exists (ticket 04).
- Filters: named hyphenated flags mapping onto underscore API parameters — `--url`, `--pronounceable-name`, `--team-name` (ticket 03). There is no `--query "..."` raw passthrough (rejected approach, ticket 03). No other monitor filters are documented; no additional flags are created.
- `--team-name` passes through unchanged with no token-type detection — no classification, warning, or rejection; omitting it sends no `team_name` parameter (ticket 03).
- Errors: every non-2xx response — including 3xx — prints the upstream body to stderr byte-for-byte with no wrapper or added status summary, and the CLI exits 1. Redirects are not followed (one request to the requested endpoint, so the `Authorization` header never reaches a server-chosen target). No automatic retries — one API request attempt per invocation, no `Retry-After` handling (ticket 05).
- Exit codes: 0 on success, 1 on every failure including invalid usage (ticket 05). stdout carries successful bodies only.
- Credentials: missing or empty `BETTERSTACK_UPTIME_API_TOKEN` prints an actionable stderr message naming the variable and exits 1 before any request. The former `BETTERSTACK_API_TOKEN` name is never read (ticket 05).
- Timeout: 30-second default total request timeout, overridable per invocation with `--timeout` accepting a duration value; duration grammar specifics are an implementer choice (ticket 05). Local/network failures print one concise, actionable, token-free stderr message and exit 1.
- Token safety: the token is never printed, logged, or stored. No redaction or inspection of response bodies; the raw passthrough stays untouched (ticket 05).

### Constraints and assumptions

- Tests use only dummy tokens and mock loopback servers; no live API call, external request, publication, push, tag, or deletion is authorized.
- Every shell begins with `unset BETTERSTACK_UPTIME_API_TOKEN`; no environment content is printed or logged.
- `verify-incident-id.sh` and the `.wayfinder/` planning tree stay untouched.
- The 002 seam and passthrough rules are reused unchanged. A minimal shared HTTP-helper refactor inside `cmd/bs` is acceptable if the observable behavior of `monitors get` does not change; avoid refactoring beyond what 003 needs.

## Section 2: Requirements

### R1 — Command surface and argument parsing

- R1.1 `bs monitors list` is a working command. It accepts no positional target: any positional argument after `list` is invalid usage (readable stderr, exit 1, no request).
- R1.2 Optional flags: `--page <n>`, `--per-page <n>`, `--url <value>`, `--pronounceable-name <name>`, `--team-name <name>`, `--timeout <duration>`. Flags may appear in any order. Every flag requires its value in the following argument; a flag missing its value is invalid usage (exit 1, no request).
- R1.3 An unknown option (any argument starting with `-` that is not a listed flag, including a bare `-h` or `--help` after `list` in the argument position) is invalid usage: readable stderr, exit 1, no request. `bs monitors list --help` remains help: it exits 0, prints the existing `monitorsListHelp` text to stdout, empty stderr, no request.
- R1.4 If a flag repeats in one invocation, each occurrence's value is passed through as given (no local deduplication, rejection, or last-wins rule — no local grammar beyond passthrough). Upstream receives the parameters as encoded.
- Acceptance: mock-API checks prove each valid vector reaches the mock server exactly once and each invalid vector exits 1 with a readable, token-free stderr message and zero requests.

### R2 — Credential channel

- R2.1 The token is read only from `BETTERSTACK_UPTIME_API_TOKEN`. Missing or empty: an actionable stderr message naming the variable, exit 1, before any request. `BETTERSTACK_API_TOKEN` set alone is ignored and never read.
- R2.2 Argument validation errors (R1) may occur before the token check, matching the 002 `monitorsGet` order; the missing-token failure still occurs before any request.
- Acceptance: mock-API checks prove zero requests and a message naming `BETTERSTACK_UPTIME_API_TOKEN` for unset, empty, and former-name-only cases.

### R3 — Request construction

- R3.1 Exactly one `GET <baseURL>/api/v2/monitors` request per invocation, with `Authorization: Bearer <token>`.
- R3.2 Each given flag maps to one query parameter: `--page` → `page`, `--per-page` → `per_page`, `--url` → `url`, `--pronounceable-name` → `pronounceable_name`, `--team-name` → `team_name`. Values pass through unchanged: no clamping, substitution, rejection, type coercion, or validation beyond what the standard-library query builder itself encodes (the stdlib `net/url` query builder is the required implementation mechanism). A flag present with an empty string value sends the parameter with an empty value; a flag absent sends no parameter.
- R3.3 No extra query parameters, no `--all` parameter, no `pagination`-related logic in the request.
- Acceptance: mock-API checks assert the exact query string for (a) no flags — no query parameters; (b) each flag alone; (c) all flags combined; (d) empty-string values; (e) repeated occurrences of one flag.

### R4 — Success envelope

- R4.1 A 2xx response body is written to stdout byte-for-byte, with nothing appended or removed (no appended newline), including non-JSON bodies and bodies containing the `pagination` object. stderr stays empty; exit 0.
- R4.2 The body is not parsed, validated, or re-serialized; the `pagination` object is not read or acted on by the CLI.
- Acceptance: mock-API checks with bodies that include a `pagination` object, a non-JSON body, and a body with its own trailing newline; stdout equals the body exactly.

### R5 — Error behavior

- R5.1 Every non-2xx response — 3xx included — writes the raw upstream body to stderr byte-for-byte, writes nothing to stdout, exits 1, and reaches the server exactly once (no redirect following, no retry, no `Retry-After` handling).
- R5.2 Local and network failures (timeout expiry, connection refused, DNS/TLS failure) print one concise, actionable, token-free stderr message, write nothing to stdout, and exit 1.
- R5.3 The token never appears on stdout or stderr on any path (success, non-2xx, local failure). Locally generated diagnostics redact the token value; passthrough bodies are never redacted or inspected.
- Acceptance: mock-API checks mirror the 002 failure-class coverage for `monitors list`.

### R6 — Timeout

- R6.1 The total request timeout defaults to 30 seconds and is overridable with `--timeout <duration>`; a non-positive or unparseable value is invalid usage (exit 1, readable stderr, no request).
- Acceptance: mock-API checks prove a slow server fails under a short override with exit 1 and one request, and succeeds within a longer override.

### R7 — Absent features (fixed by the milestones doc; presence is a defect)

- No `--all` flag, no auto-drain of `pagination.next`, no `--pretty` flag, no `--query` flag, no monitor flags beyond those in R1.2, no token-type detection for `--team-name`, no retry flag, no output modes.
- Acceptance: unknown-option checks (R1.3) cover `--all`, `--pretty`, and `--query` as invalid usage.

### R8 — Regression preservation

- R8.1 `bs monitors get`, all help levels, and all invalid-usage vectors keep their 002 behavior; only the `monitors list` vector's outcome changes from "not implemented" to the working command. `TestUnimplementedCommandsMakeNoRequest` updates its `monitors list` case to match the new behavior while the other cases stay.
- R8.2 `monitorsGet`'s observable behavior is unchanged (including if a minimal shared helper refactor is made).
- Acceptance: the full existing test suite passes alongside the new checks.
