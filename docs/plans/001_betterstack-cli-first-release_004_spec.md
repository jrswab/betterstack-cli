# Milestone 004 Specification — `bs incidents get <id-or-url>` with dashboard URL support

Milestone source: `docs/plans/001_betterstack-cli-first-release_milestones.md`, entry 004 (unchecked). This spec covers milestone 004 only; `bs incidents timeline` (005), `bs incidents list` (006), and every other milestone are out of scope.

## Section 1: Context & Constraints

### Codebase state and established patterns

- Milestones 001–003 are complete. `cmd/bs/main.go` routes inside `runWithAPI` (help via `helpFor`, then `monitors list`, then `monitors get`); every `incidents ...` vector still falls through to `usageError`/`resourceUsageError`, so `bs incidents get <target>` currently answers "not implemented" with exit 1. `resourceUsageError` already renders the `<id-or-url>` placeholder for incident `get`/`timeline` and keeps a not-implemented branch for `incidents list`.
- `bs incidents get --help` already exists and works (`incidentsGetHelp` in `cmd/bs/help.go`): usage `bs incidents get <id-or-url>`, the bare-numeric and dashboard-URL argument forms, and the `--timeout` flag. No help text changes are needed in this milestone.
- `cmd/bs/monitors.go` implements the shared executor `doMonitorsRequest(endpoint, token, timeout, stdout, stderr, client)`: one bearer-authenticated GET, redirect following disabled via `http.ErrUseLastResponse`, total timeout context, raw 2xx body to stdout with `stdout.Write` (no appended newline), raw non-2xx body to stderr, exit codes 0/1 only, and `redactToken` applied only to locally generated diagnostics — never to a passthrough body. `monitorsGet` (parse → token check → endpoint build → `doMonitorsRequest`) is the structural template for `incidentsGet`; the shared executor differs only in endpoint path and target resolution and is reused unchanged (its `monitors` name is historical; renaming is out of scope).
- The test seam is `runWithAPI(args, stdout, stderr, baseURL, client)` with the `invokeAPI` helper (`cmd/bs/monitors_test.go`): tests inject a loopback base URL and client, use only the fixed dummy token `dummy-token-for-tests`, and never read the ambient environment.
- Verification environment: `GOTOOLCHAIN=go1.26.9 GOPROXY=off` from the local module cache (local default `go` is 1.22.2, below the module minimum `go 1.26`).

### Decisions already made (settled; never re-evaluate)

Sources: `.wayfinder/tickets/02-incident-url-identity-resolution.md` (Resolution), `.wayfinder/assets/02-incident-url-identity-research.md` (section 8), `.wayfinder/assets/01-api-contract-research.md` (re-checked this session), `.wayfinder/tickets/03-command-naming-and-argument-conventions.md` (Resolution), `.wayfinder/tickets/04-pagination-and-json-envelope.md` (Resolution), `.wayfinder/tickets/05-errors-timeouts-retries-token-safety.md` (Resolution).

- Endpoint: `GET https://incidents.betterstack.com/api/v3/incidents/{incident_id}` (asset 01, section 1: incident resources are **v3**, monitor resources are v2 — the version split applies per resource; the Get incident page documents `{incident_id}` as a string path parameter with numeric-string examples). This v3 path was re-verified against asset 01 and the MAP this session; no prior summarized path was trusted.
- Identity rule (ticket 02 Resolution, confirmed for one user-reported sample on 2026-10-17): a dashboard incident URL of the observed form `https://incidents.betterstack.com/team/<team-id>/incidents/<incident-id>` is resolved by **narrow local parsing** — the CLI accepts exactly this host and path shape, extracts the terminal numeric segment as the API incident ID candidate, and calls the documented API with it. The CLI invents no extra "resolve" API call, no probe-then-match fallback, performs no network fetch to the user-supplied URL's host, and does not validate the team segment against the token. If the extracted ID is wrong, the ordinary non-2xx error rules apply unchanged.
- Documented limitations of the identity rule (accepted, recorded): the same-ID rule rests on one user-reported sample, not on documentation; cross-team behavior is undocumented; alternate dashboard URL shapes are unknown — anything outside the observed form is unsupported in the first release (asset 02, section 8).
- Command syntax (ticket 03): `bs incidents get <id-or-url>` takes one positional target accepting either an incident ID (including the bare numeric form) or a **full** dashboard incident URL. A "full dashboard URL" is a URL; a bare incident ID is not. Bare IDs are treated as opaque strings (asset 01: "Treat ID fields as opaque strings in the CLI, not numbers"), matching the 002/003 pattern of passing the target through with `url.PathEscape` and no local content grammar.
- Success envelope (ticket 04): raw byte-for-byte passthrough of the response body to stdout — no parsing, validation, re-serialization, pretty-printing, or appended newline. `data`, `included`, and `pagination` pass through as the API sent them; on the retrieve response the API may send an `included` member with the triggering monitor (asset 01, section 6.1).
- Errors (ticket 05): every non-2xx response — including 3xx — prints the upstream body to stderr byte-for-byte with no wrapper or added status summary, and the CLI exits 1. Redirects are not followed (one request to the requested endpoint). No automatic retries, no `Retry-After` handling.
- Streams and exit codes (ticket 05): stdout carries successful bodies only; exit 0 on success, exit 1 on every failure including invalid usage.
- Credentials (ticket 05): missing or empty `BETTERSTACK_UPTIME_API_TOKEN` prints an actionable stderr message naming the variable and exits 1 before any request; the former `BETTERSTACK_API_TOKEN` name is never read.
- Timeout (ticket 05): 30-second default total request timeout, overridable per invocation with `--timeout` accepting a duration value (approved example: `bs incidents get 123 --timeout 60s`); duration grammar specifics are an implementer choice. Local/network failures print one concise, actionable, token-free stderr message and exit 1.
- Token safety (ticket 05): the token is never printed, logged, or stored; no redaction or inspection of response bodies.

### Constraints and assumptions

- Tests use only dummy tokens and mock loopback servers; no live API call, external request, publication, push, tag, or deletion is authorized.
- Every shell begins with `unset BETTERSTACK_UPTIME_API_TOKEN`; no environment content is printed or logged.
- `verify-incident-id.sh` and the `.wayfinder/` planning tree stay untouched.
- The 002 shared executor, passthrough rules, and test seam are reused unchanged. No refactor beyond adding the incidents command is in scope.

## Section 2: Requirements

### R1 — Command surface and argument parsing

- R1.1 `bs incidents get` becomes a working command. It takes exactly one positional target `<id-or-url>`: a missing target and a second positional argument are invalid usage (readable stderr, exit 1, no request).
- R1.2 Optional flag: `--timeout <duration>` with the same grammar and invalid-value handling as `monitors get`. Flags may appear before or after the target; a flag missing its value is invalid usage (exit 1, no request).
- R1.3 An unknown option (any argument starting with `-` that is not a listed flag, including `--help` in flag position) is invalid usage: readable stderr, exit 1, no request. `bs incidents get --help` remains help: it exits 0, prints the existing `incidentsGetHelp` text to stdout, empty stderr, no request.
- Acceptance: mock-API checks prove each invalid vector exits 1 with a readable, token-free stderr message and zero requests, and that `--help` exits 0 with the unchanged help text.

### R2 — Target resolution (narrow local parsing, decided in ticket 02)

- R2.1 The positional target has exactly two accepted forms:
  - **Bare ID form:** any non-empty target that the standard-library URL parser (`net/url`.Parse) does not parse with a non-empty scheme. It is opaque: no local content validation of any kind (no numeric check, no length or character rules); it is passed through unchanged into the endpoint path with `url.PathEscape`, as `monitors get` does with `<id>`.
  - **Dashboard URL form:** the target parses with a non-empty scheme and matches **exactly** the observed form — scheme `https`; host exactly `incidents.betterstack.com` with no port and no userinfo; decoded path consisting of exactly the four segments `/team/<team-id>/incidents/<incident-id>` with a non-empty `<team-id>` and an `<incident-id>` of one or more ASCII digits; empty query; empty fragment; no trailing slash. The `<incident-id>` segment is extracted as the API incident ID candidate.
- R2.2 Every other target is invalid usage — readable stderr naming the problem, exit 1, **before any request**. This includes, for any scheme-ful (URL-form) target: any non-`https` scheme, any host other than `incidents.betterstack.com`, an explicit port, userinfo or embedded credentials, a query string, a fragment, a trailing slash, a wrong segment count, an empty team segment, or a non-numeric terminal segment.
- R2.3 The bare/URL discriminator follows the approved wording "a bare incident ID or a **full dashboard URL**" (ticket 03): a string the stdlib parser reads as having a scheme is a URL and must satisfy the narrow rule exactly; every other string remains an opaque bare ID whose existence the API alone judges — a wrong bare ID gets the ordinary non-2xx error rules (R6), not local rejection. Scheme normalization is the stdlib parser's documented behavior; the host comparison is exact as received.
- R2.4 No network fetch to the user-supplied URL's host, no extra resolve or probe API call, and no team-segment validation against the token ever happen. The only request of any successful invocation goes to the fixed API base URL with the extracted candidate ID in `/api/v3/incidents/{id}`.
- Acceptance: mock-API checks cover (a) a bare ID target and a full observed-form URL target each reaching the mock exactly once with path `/api/v3/incidents/<candidate>`; (b) every rejected shape in R2.2 exiting 1 with zero requests; (c) a URL-form target whose candidate ID the mock answers non-2xx producing the ordinary error passthrough, unchanged.

### R3 — Credential channel

- R3.1 The token is read only from `BETTERSTACK_UPTIME_API_TOKEN`. Missing or empty: an actionable stderr message naming the variable, exit 1, before any request. `BETTERSTACK_API_TOKEN` set alone is ignored and never read.
- R3.2 Argument and target validation errors (R1, R2) occur before the token check, matching the 002/003 `monitorsGet` order; the missing-token failure still occurs before any request.
- Acceptance: mock-API checks prove zero requests and a message naming `BETTERSTACK_UPTIME_API_TOKEN` for unset, empty, and former-name-only cases.

### R4 — Request construction

- R4.1 Exactly one `GET <baseURL>/api/v3/incidents/<candidate>` request per invocation, with `Authorization: Bearer <token>`, where `<candidate>` is the bare ID or the extracted URL segment, path-escaped with `url.PathEscape` (a no-op for the observed all-digits segment).
- R4.2 No query parameters, no extra headers beyond the bearer header, no second request of any kind.
- Acceptance: mock-API checks assert the method, the exact path, the bearer header, and exactly one request for ID-form and URL-form targets.

### R5 — Success envelope

- R5.1 A 2xx response body is written to stdout byte-for-byte, with nothing appended or removed (no appended newline), including non-JSON bodies and bodies carrying the `included` member. stderr stays empty; exit 0.
- R5.2 The body is not parsed, validated, or re-serialized; the CLI echoes nothing of its own (no extracted-ID echo, no wrapper).
- Acceptance: mock-API checks with a JSON:API-style retrieve body containing `data` and `included`, a non-JSON body, and a body with its own trailing newline; stdout equals the body exactly.

### R6 — Error behavior

- R6.1 Every non-2xx response — 3xx included — writes the raw upstream body to stderr byte-for-byte, writes nothing to stdout, exits 1, and reaches the server exactly once (no redirect following, no retry, no `Retry-After` handling). This applies unchanged when the candidate ID from a URL is wrong.
- R6.2 Local and network failures (timeout expiry, connection refused, DNS/TLS failure) print one concise, actionable, token-free stderr message, write nothing to stdout, and exit 1.
- R6.3 The token never appears on stdout or stderr on any path (success, non-2xx, local failure). Locally generated diagnostics redact the token value; passthrough bodies are never redacted or inspected.
- Acceptance: mock-API checks mirror the 002/003 failure-class coverage for `incidents get`.

### R7 — Timeout

- R7.1 The total request timeout defaults to 30 seconds and is overridable with `--timeout <duration>`; a non-positive or unparseable value is invalid usage (exit 1, readable stderr, no request).
- Acceptance: mock-API checks prove a slow server fails under a short override with exit 1 and one request, and succeeds within a longer override.

### R8 — Absent features (fixed by the milestone and tickets; presence is a defect)

- No fetch of the user-supplied URL, no resolve/probe call, no team-segment validation, no URL normalization or shape variants beyond R2.1 (no trailing slash, no query, no fragment accepted), no flags beyond `--timeout`, no retry, no output modes, and no work toward `incidents timeline` (005) or `incidents list` (006).
- Acceptance: unknown-option checks (R1.3) plus the zero-request checks for every rejected shape (R2.2) and a source audit that no second request path exists.

### R9 — Regression preservation

- R9.1 `bs incidents timeline <id-or-url>` and `bs incidents list` remain not-implemented with exit 1, a readable error, and zero requests; `bs incidents timeline --help` keeps exiting 0. Only the `incidents get` vector's outcome changes from "not implemented" to the working command.
- R9.2 `monitors get` and `monitors list` keep their 002/003 observable behavior unchanged; all help surfaces are unchanged.
- Acceptance: the full existing test suite passes alongside the new checks, with the `incidents get` case in `TestUnimplementedCommandsMakeNoRequest` updated to the new behavior and all other cases kept.
