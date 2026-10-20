# Milestone 002 Specification — S1 Telemetry endpoint seam and secret redaction

- Milestone source: `docs/plans/008_telemetry-logs-search_milestones.md`, entry `002` (S1)
- Scope: milestone 002 only. This specification covers the endpoint-injection seam and reusable secret-redaction behavior. It does not implement source commands, SQL search, or response handling.
- Acceptance in one line: `runWithEndpoints` supports isolated Uptime, Telemetry REST, and Telemetry SQL endpoint inputs without changing `runWithAPI` or its existing callers, and `redactSecrets` safely removes the approved Telemetry credentials from diagnostic text.

## Section 1: Context & Constraints

### Repository and current interfaces

- The repository is a standard-library Go CLI in package `main`. `cmd/bs/main.go` defines `run`, `runWithAPI`, and the help and usage routing. `cmd/bs/monitors.go` defines the Uptime endpoint defaults, `newHTTPClient`, `doMonitorsRequest`, and `redactToken`. The current routing seam accepts one base URL and an HTTP client.
- `run` supplies the production Uptime URL and client to `runWithAPI`. Tests use `invoke` → `run` for help and invalid usage, and `invokeAPI` → `runWithAPI` for loopback HTTP requests. Existing test helpers and callers must continue to compile and retain their current semantics.
- `runWithAPI(args, stdout, stderr, baseURL, client)` is an established compatibility boundary and must keep its exact signature. The approved Telemetry seam is `runWithEndpoints`. It must permit tests for the fixed Telemetry REST endpoint and the configured Telemetry SQL endpoint to use local mock servers.
- Existing HTTP tests use `httptest` servers and dummy credentials. `TestMain` clears inherited Uptime credentials and installs a loopback-only network guard. S1 must not add live requests, use credentials, or bypass this guard.
- `.mem/terminology.md` defines the repository's boundary names and cautions against calling test endpoints runtime endpoint overrides. Existing anchors mark `runWithAPI` as `uptime:router` and configuration in `monitors.go` as `uptime:config`. The current `TAGS.md` indexes every anchor occurrence.

### Approved Telemetry contracts

- Ticket 02 requires separate Telemetry credentials: `BETTERSTACK_TELEMETRY_API_TOKEN` for source requests, and `BETTERSTACK_TELEMETRY_SQL_USERNAME` plus `BETTERSTACK_TELEMETRY_SQL_PASSWORD` for SQL authentication. The Uptime token is not a fallback. Ticket 02 requires a `runWithEndpoints` seam and source requests reuse `doMonitorsRequest`.
- Ticket 07, summarized in the milestone research findings, makes `BETTERSTACK_TELEMETRY_SQL_URL` the only configurable service host. It must be HTTPS and accept only the approved host forms. Uptime and Telemetry REST hosts remain fixed. The source API host is `https://telemetry.betterstack.com`; the Uptime host is `https://incidents.betterstack.com`.
- Ticket 10 supersedes ticket 02's initial no-query-parameter decision for later SQL work. It also defines the SQL URL forms and confirms that SQL query values use parameter binding. These decisions do not change S1's seam or redaction scope.
- Ticket 11 requires `redactSecrets` for non-2xx SQL diagnostic bodies in S4. A successful SQL response remains raw and unredacted. S1 provides and tests the reusable redaction function only. S1 does not apply it to source output, successful SQL output, or SQL error bodies; S4 owns applying it to non-2xx SQL response bodies.
- The milestone research findings explicitly require `runWithEndpoints` to preserve `runWithAPI` and existing callers. They identify endpoint inputs as the Uptime base URL, Telemetry REST base URL, and SQL URL. The S1 seam treats these as test injection inputs, not as new user-facing endpoint overrides. The approved SQL environment variable remains the only configurable production host.

### Redaction decisions and constraints

- The redaction inputs are the Telemetry API token, SQL username, SQL password, and the standard base64 encoding of `username:password` used by HTTP Basic authentication. Redaction applies to literal occurrences in diagnostic text.
- Use the existing `[REDACTED]` marker from `redactToken` for consistency. Matching is case-sensitive because credential values are opaque byte strings and no case-folding contract exists.
- Empty input values are ignored. They must never match every position or alter unrelated text. The derived Basic-auth value is included when at least one username or password value is non-empty. Duplicate secret values are harmless and may be de-duplicated.
- Replace longer secrets before shorter secrets. This prevents a shorter credential substring from consuming a longer secret first. Replacement is global for each value. No SQL, response parsing, or JSON decoding is part of redaction.
- S4 owns applying `redactSecrets` to non-2xx SQL response bodies. S1's function and unit tests establish the reusable contract without changing any current output path.

### Settled decisions and exclusions

- Do not change `runWithAPI`'s argument list, `run`, `invoke`, or `invokeAPI` semantics. New tests for multiple endpoint values use `runWithEndpoints`.
- Do not add a configurable Uptime or Telemetry REST host. Do not fall back from Telemetry credentials to Uptime credentials.
- Do not redact successful data or apply S1 redaction to existing Uptime raw response bodies. Ticket 11 fixes S4's SQL non-2xx-only application boundary.
- Do not implement source routing, source HTTP behavior, SQL URL validation, SQL authentication, query construction, SQL execution, status-specific output, retries, redirects, help, or CLI flags in S1. Later slices own those behaviors.
- S1 has no unresolved design blocker. The seam carries three endpoint values because later slices need distinct Uptime, Telemetry REST, and SQL destinations, while the SQL URL remains the sole configurable production host.

## Section 2: Requirements

### R1 — Preserve the existing routing seam

`runWithAPI(args, stdout, stderr, baseURL, client)` retains its exact function signature and remains usable by `run`, existing tests through `invoke`/`invokeAPI`, and existing production routes. Existing Uptime callers keep using the supplied Uptime base URL and client. Delegation to the new seam must not change current help, usage, request, credential, or output behavior.

**Acceptance:** all existing call sites compile unchanged; existing Uptime routing tests pass; `runWithAPI` still routes existing Uptime commands against the injected base URL; and help or invalid-usage paths perform no request.

### R2 — Provide the Telemetry endpoint seam

`runWithEndpoints` accepts the command arguments, output writers, an HTTP client, and endpoint inputs that distinguish the Uptime base URL, fixed Telemetry REST base URL, and Telemetry SQL URL. The seam must permit a test to direct each service class to local loopback endpoints without changing production endpoint policy. The SQL URL input represents the value configured by `BETTERSTACK_TELEMETRY_SQL_URL`; S1 does not define or implement SQL URL parsing.

**Acceptance:** tests confirm that `runWithAPI` retains existing Uptime request behavior and that `runWithEndpoints` receives and keeps three distinct endpoint values. Because S1 adds no source or SQL command, S1 does not claim that Telemetry REST or SQL requests are issued. S2 and S4 verify those endpoint values at their real request boundaries. No live request occurs.

### R3 — Redact the approved Telemetry secrets

`redactSecrets` removes every literal occurrence of each non-empty configured secret from diagnostic text: the Telemetry API token, SQL username, SQL password, and base64-encoded `username:password`. It replaces each occurrence with `[REDACTED]`, uses case-sensitive matching, and replaces longer values before shorter values. It ignores empty values and does not modify unrelated text. If either SQL username or password is non-empty, the derived Basic-auth value is eligible for redaction; if both are empty, no derived credential is added.

**Acceptance:** unit tests cover all four secret forms, repeated occurrences, overlapping values with the longer value handled first, case differences, empty credential values, both username/password empty, and text with no matching secret. No test requires credentials from the process environment.

### R4 — Keep S1 redaction reusable and unapplied to response paths

S1 adds the redaction capability and its direct tests. S1 does not change existing Uptime response passthrough, successful source or SQL data, or any current error body. S4 applies the function only to non-2xx SQL response bodies, as ticket 11 requires. S1 must not implement S4 behavior early.

**Acceptance:** existing raw response tests pass unchanged; focused S1 tests call `redactSecrets` directly; source output, successful SQL output, and SQL error-body integration are absent from S1 changes.

### R5 — Preserve endpoint and credential policy

The production policy remains fixed Uptime and Telemetry REST hosts, with `BETTERSTACK_TELEMETRY_SQL_URL` as the only configurable endpoint host. Telemetry authentication remains separate from `BETTERSTACK_UPTIME_API_TOKEN`. The new test seam does not introduce a CLI flag, environment override, or configuration file for Uptime or Telemetry REST.

**Acceptance:** no code path falls back to the Uptime token for Telemetry. No non-SQL production host becomes configurable. Tests use only loopback endpoints and fixed dummy credential values.

### R6 — Add applicable repository records before committing

Before the S1 implementation commit, add a factual `changelog.d/` fragment using the repository changelog convention. Update `TAGS.md` for every added, removed, or changed anchor. Update `.mem/` through the `mem` skill if S1 changes behavior or structure that memory records. Update `README.md` if S1 makes an environment variable, endpoint, Make target, or setup instruction active or changes its documented contract. Keep `LOG.md` and `log.d/` consistent with existing conventions; add provenance only if the implementation introduces non-obvious rationale that future contributors could incorrectly simplify.

**Acceptance:** the changelog fragment exists and describes only delivered S1 work; every production anchor matches `TAGS.md`; memory accurately covers any changed behavior or structure; README documents any newly active configuration or setup; and no speculative runtime behavior is documented as implemented. This requirement carries the repository's Before Committing checklist into the S1 implementation gate.

### R7 — Verify the S1 change without external access

Run `go build ./...`, `go vet ./...`, `go test ./...`, and `gofmt -l cmd/bs/*.go`. Add focused seam and redaction tests, and retain the existing loopback-only safety protections. Run `git diff --check` and explicitly audit staged paths. Do not run live API requests or use real credentials.

**Acceptance:** all four commands exit successfully, `gofmt -l` prints no paths, focused and existing tests pass, the network guard remains intact, and only approved S1 implementation and required repository-record paths are staged.

### Ordering and parallel work

R1 and R2 define one compatibility and endpoint-seam contract. R3 defines an independent redaction contract. Their implementation and focused tests can proceed in parallel after the endpoint value contract in R2 is fixed. R4 and R5 constrain both slices. R6 and R7 apply to the complete S1 change. S2 source behavior and S4 SQL error application remain downstream work and are not acceptance criteria for S1.
