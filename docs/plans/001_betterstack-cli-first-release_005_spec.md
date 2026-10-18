# Milestone 005 Specification — `bs incidents timeline <id-or-url>`

Milestone source: `docs/plans/001_betterstack-cli-first-release_milestones.md`, entry 005 (unchecked). This spec covers milestone 005 only; `bs incidents list` (006), the 007 acceptance sweep, and every other milestone are out of scope.

## Section 1: Context & Constraints

### Codebase state and established patterns

- Milestones 001–004 are complete. `cmd/bs/main.go` routes inside `runWithAPI` (help via `helpFor`, then `monitors list`, `monitors get`, `incidents get`); the `incidents timeline <target>` vector (non-help, one positional) still falls through to `usageError` → `resourceUsageError`, whose shared get/timeline branch returns `bs: "incidents timeline" is not implemented in this milestone; run 'bs incidents timeline --help' for usage.` with exit 1. This is the exact behavior 005 replaces for the timeline vector only.
- `bs incidents timeline --help` already exists and works (`incidentsTimelineHelp` in `cmd/bs/help.go`): usage `bs incidents timeline <id-or-url>`, the bare-numeric and dashboard-URL argument forms, the `--timeout` flag, and example `bs incidents timeline 123`. No help text changes are anticipated (the same 004 situation).
- `cmd/bs/incidents.go` already provides everything the timeline target needs: `parseIncidentsGetArgs` (one positional + `--timeout`) and `resolveIncidentTarget` (the narrow 002/004 identity rule) are directly reusable; the shared executor `doMonitorsRequest` in `cmd/bs/monitors.go` is endpoint-agnostic (`endpoint, token, timeout, stdout, stderr, client`) and serves the timeline path unchanged.
- The test seam is `runWithAPI(args, stdout, stderr, baseURL, client)` with the `invokeAPI` helper (`cmd/bs/monitors_test.go`): tests inject a loopback base URL and client, use only the fixed dummy token `dummy-token-for-tests`, and never read the ambient environment. `cmd/bs/safety_test.go` keeps the `TestMain` credential isolation and loopback-only transport guard enabled.
- Test-preservation surface: `TestUnimplementedCommandsMakeNoRequest` (`cmd/bs/monitors_test.go`) currently lists `incidents timeline` as not-implemented and must drop that case when the vector becomes working (the 004 B2 precedent). `TestInvalidUsageMatrix` (`cmd/bs/main_test.go`) has no `incidents timeline` case, so no stale-entry hazard exists there.
- Verification environment: `GOTOOLCHAIN=go1.26.9 GOPROXY=off` from the local module cache (local default `go` is 1.22.2, below the module minimum `go 1.26`).

### Decisions already made (settled; never re-evaluate)

Sources: `.wayfinder/assets/01-api-contract-research.md` (sections 6.3 and 7, re-checked this session), `.wayfinder/tickets/02-incident-url-identity-resolution.md` (Resolution), `.wayfinder/tickets/03-command-naming-and-argument-conventions.md` (Resolution), `.wayfinder/tickets/04-pagination-and-json-envelope.md` (Resolution), `.wayfinder/tickets/05-errors-timeouts-retries-token-safety.md` (Resolution).

- Endpoint: `GET https://incidents.betterstack.com/api/v3/incidents/{incident_id}/timeline` (asset 01, section 1: incident resources are **v3** — the same incident resource version as 004's retrieve; section 6.3 documents the timeline path and shape). The same `{incident_id}` string path parameter applies.
- Target rule: the milestone entry states the positional `<id-or-url>` target follows "the same positional target rule as `bs incidents get`" — the narrow local parsing rule decided in ticket 02 and implemented in 004 (`resolveIncidentTarget`) applies unchanged: bare ID form opaque, dashboard URL form exactly `https://incidents.betterstack.com/team/<team-id>/incidents/<incident-id>` with a terminal all-digits segment, every other URL shape rejected locally as invalid usage before any request, no fetch to the user-supplied host, no extra resolve call, no team-segment validation.
- Timeline response shape (asset 01, section 6.3): `{"data": [...]}` where each item is `{id, type: "timeline_item", attributes: {item_type, at, data}}`. Item `id` values are heterogeneous strings (numeric `"23"`, composite `"2021-02-12 23:59:59 UTC"`, `"2470302-content-response"`, `"incident-started"`) and stay opaque. Seven documented `item_type` shapes exist (`generic`, `generic_card`, `timeline_truncated`, `response_item`, `payload`, `comment` — whose `content` is an object, unlike the string `content` elsewhere — and `status_report_card`). The CLI does no decoding, filtering, or reshaping of any of this.
- No pagination (asset 01, section 7): the timeline endpoint "does not support pagination and returns all timeline items instead"; no `pagination` object appears in the example response. There are no `--page`/`--per-page` flags and no auto-drain.
- Truncation semantics (asset 01, sections 6.3 and 7): the API may include `timeline_truncated` items marking API-side truncation of retained history. The CLI passes such items through untouched and makes **no completeness promise** — no message, flag, or behavior claims the timeline is complete.
- Success envelope (ticket 04): raw byte-for-byte passthrough of the response body to stdout — no parsing, validation, re-serialization, pretty-printing, or appended newline. `data` (including every item shape above) passes through as the API sent it.
- Errors (ticket 05): every non-2xx response — including 3xx — prints the upstream body to stderr byte-for-byte with no wrapper or added status summary, and the CLI exits 1. Redirects are not followed (one request to the requested endpoint). No automatic retries, no `Retry-After` handling.
- Streams and exit codes (ticket 05): stdout carries successful bodies only; exit 0 on success, exit 1 on every failure including invalid usage.
- Credentials (ticket 05): missing or empty `BETTERSTACK_UPTIME_API_TOKEN` prints an actionable stderr message naming the variable and exits 1 before any request; the former `BETTERSTACK_API_TOKEN` name is never read.
- Timeout (ticket 05): 30-second default total request timeout, overridable per invocation with `--timeout` accepting a duration value (approved example: `bs incidents timeline 123 --timeout 60s`); duration grammar specifics are an implementer choice. Local/network failures print one concise, actionable, token-free stderr message and exit 1.
- Token safety (ticket 05): the token is never printed, logged, or stored; no redaction or inspection of response bodies.

### Constraints and assumptions

- Tests use only dummy tokens and mock loopback servers; no live API call, external request, publication, push, tag, or deletion is authorized.
- Every shell begins with `unset BETTERSTACK_UPTIME_API_TOKEN` **and** `unset BETTERSTACK_API_TOKEN`; no environment content is printed or logged.
- `verify-incident-id.sh` and the `.wayfinder/` planning tree stay untouched.
- The 004 resolver, shared executor, passthrough rules, and test seam are reused unchanged. No refactor beyond adding the timeline command is in scope.

## Section 2: Requirements

### R1 — Command surface and argument parsing

- R1.1 `bs incidents timeline` becomes a working command. It takes exactly one positional target `<id-or-url>`: a missing target and a second positional argument are invalid usage (readable stderr, exit 1, no request).
- R1.2 Optional flag: `--timeout <duration>` with the same grammar and invalid-value handling as `incidents get`. Flags may appear before or after the target; a flag missing its value is invalid usage (exit 1, no request).
- R1.3 An unknown option (any argument starting with `-` that is not a listed flag, including `--help` in flag position) is invalid usage: readable stderr, exit 1, no request. `bs incidents timeline --help` remains help: it exits 0, prints the existing `incidentsTimelineHelp` text to stdout, empty stderr, no request.
- Acceptance: mock-API checks prove each invalid vector exits 1 with a readable, token-free stderr message and zero requests, and that `--help` exits 0 with the unchanged help text.

### R2 — Target resolution (the same narrow rule as `bs incidents get`)

- R2.1 The positional target has exactly the two accepted forms of 004 R2.1: the opaque bare ID form (any non-empty target the stdlib URL parser does not read as scheme-ful; no local content validation; passed through with `url.PathEscape`) and the dashboard URL form (scheme `https`; host exactly `incidents.betterstack.com` with no port and no userinfo; exactly `/team/<team-id>/incidents/<incident-id>` with non-empty `<team-id>` and an `<incident-id>` of one or more ASCII digits; empty query and fragment; no trailing slash), whose `<incident-id>` segment is the API incident ID candidate.
- R2.2 Every other target is invalid usage — readable stderr naming the problem, exit 1, **before any request**, exactly as in 004 R2.2.
- R2.3 The bare/URL discriminator and the "wrong ID → ordinary non-2xx rules" behavior follow 004 R2.3 unchanged; no re-derivation happens in this milestone.
- R2.4 No network fetch to the user-supplied URL's host, no extra resolve or probe API call, and no team-segment validation against the token ever happen. The only request of any successful invocation goes to the fixed API base URL with the resolved candidate ID in the timeline path.
- Acceptance: mock-API checks cover (a) a bare ID target and a full observed-form URL target each reaching the mock exactly once with the timeline path for `<candidate>`; (b) the 004 rejected shapes exiting 1 with zero requests through the timeline routing; (c) a URL-form target whose candidate ID the mock answers non-2xx producing the ordinary error passthrough, unchanged.

### R3 — Credential channel

- R3.1 The token is read only from `BETTERSTACK_UPTIME_API_TOKEN`. Missing or empty: an actionable stderr message naming the variable, exit 1, before any request. `BETTERSTACK_API_TOKEN` set alone is ignored and never read.
- R3.2 Argument and target validation errors (R1, R2) occur before the token check, matching the 004 `incidentsGet` order; the missing-token failure still occurs before any request.
- Acceptance: mock-API checks prove zero requests and a message naming `BETTERSTACK_UPTIME_API_TOKEN` for unset, empty, and former-name-only cases.

### R4 — Request construction

- R4.1 Exactly one `GET <baseURL>/api/v3/incidents/<candidate>/timeline` request per invocation, with `Authorization: Bearer <token>`, where `<candidate>` is the bare ID or the extracted URL segment, path-escaped with `url.PathEscape` (a no-op for the observed all-digits segment).
- R4.2 No query parameters (the timeline has none — no `--page`, no `--per-page`, no filters), no extra headers beyond the bearer header, no second request of any kind.
- Acceptance: mock-API checks assert the method, the exact path including the `/timeline` suffix, the bearer header, an empty query string, and exactly one request for ID-form and URL-form targets.

### R5 — Success envelope (raw passthrough, no completeness promise)

- R5.1 A 2xx response body is written to stdout byte-for-byte, with nothing appended or removed (no appended newline), including non-JSON bodies. stderr stays empty; exit 0.
- R5.2 The body is not parsed, validated, or re-serialized; the CLI echoes nothing of its own (no extracted-ID echo, no wrapper, no item count, no truncation notice). Heterogeneous timeline item shapes — including `timeline_truncated` items and the object-valued `comment` content — pass through untouched.
- R5.3 No output, message, exit code, or behavior promises that the returned timeline is complete; the CLI treats the body as opaque bytes and stays silent about truncation.
- Acceptance: mock-API checks with a JSON:API-style timeline body carrying heterogeneous item IDs and multiple `item_type` shapes (including `timeline_truncated`), a non-JSON body, and a body with its own trailing newline; stdout equals the body exactly.

### R6 — Error behavior

- R6.1 Every non-2xx response — 3xx included — writes the raw upstream body to stderr byte-for-byte, writes nothing to stdout, exits 1, and reaches the server exactly once (no redirect following, no retry, no `Retry-After` handling). This applies unchanged when the candidate ID from a URL is wrong.
- R6.2 Local and network failures (timeout expiry, connection refused, DNS/TLS failure) print one concise, actionable, token-free stderr message, write nothing to stdout, and exit 1.
- R6.3 The token never appears on stdout or stderr on any path (success, non-2xx, local failure). Locally generated diagnostics redact the token value; passthrough bodies are never redacted or inspected.
- Acceptance: mock-API checks mirror the 004 failure-class coverage for `incidents timeline`.

### R7 — Timeout

- R7.1 The total request timeout defaults to 30 seconds and is overridable with `--timeout <duration>`; a non-positive or unparseable value is invalid usage (exit 1, readable stderr, no request).
- Acceptance: mock-API checks prove a slow server fails under a short override with exit 1 and one request, and succeeds within a longer override.

### R8 — Absent features (fixed by the milestone and tickets; presence is a defect)

- No `--page`, `--per-page`, or any filter flag (the timeline supports no pagination and documents no filters), no fetch of the user-supplied URL, no resolve/probe call, no team-segment validation, no URL normalization or shape variants beyond R2.1, no flags beyond `--timeout`, no retry, no output modes, no truncation summary, and no work toward `incidents list` (006).
- Acceptance: unknown-option checks (R1.3) plus the zero-request checks for every rejected shape (R2.2) and a source audit that no second request path exists.

### R9 — Regression preservation

- R9.1 `bs incidents list` remains not-implemented with exit 1, a readable error, and zero requests; `bs incidents get` keeps its 004 observable behavior unchanged, including its missing/extra-target errors via the shared `resourceUsageError` structure. Only the `incidents timeline` vector's outcome changes from "not implemented" to the working command; `bs incidents timeline --help` keeps exiting 0 with unchanged text.
- R9.2 `monitors get` and `monitors list` keep their 002/003 observable behavior unchanged; all help surfaces are unchanged.
- Acceptance: the full existing test suite passes alongside the new checks, with the `incidents timeline` case in `TestUnimplementedCommandsMakeNoRequest` updated to the new behavior and all other cases kept.
