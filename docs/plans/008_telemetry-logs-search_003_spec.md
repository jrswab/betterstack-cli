# Milestone 003 Specification — S2 Telemetry sources commands

- Milestone source: `docs/plans/008_telemetry-logs-search_milestones.md`, entry `003` (S2)
- Scope: milestone 003 only. This specification covers `bs sources list` and `bs sources get`.
- Acceptance in one line: both commands send one authenticated GET to the fixed Telemetry REST endpoint through the shared request contract, validate the approved arguments, pass response bodies through unchanged, and warn users that source output contains a secret credential.

## Section 1: Context & Constraints

### Repository and current interfaces

- The command is a standard-library Go CLI in `cmd/bs`, package `main`. `runWithAPI` preserves the existing Uptime injection interface and delegates to `runWithEndpoints`. The latter accepts distinct Uptime, Telemetry REST, and Telemetry SQL endpoint inputs. S1 deliberately left Telemetry endpoints unused. S2 must prove Telemetry REST routing at an actual request boundary.
- `defaultTelemetryRESTBaseURL` is `https://telemetry.betterstack.com`. The production Telemetry REST host stays fixed. The Telemetry REST test endpoint comes from `runWithEndpoints`; it is not a production override. The existing `runWithAPI` signature and Uptime routes remain unchanged.
- `doMonitorsRequest` performs one bearer-authenticated GET. It applies a timeout, disables redirects, sends 2xx bodies unchanged to stdout, and sends non-2xx bodies unchanged to stderr. Ticket 02 and the milestone plan require the source commands to reuse this shared request path. Prefer reusing its existing name; rename it only if implementation makes reuse impossible.
- Uptime get commands use an opaque single positional target, URL-escape it as one path segment, accept `--timeout <duration>` before or after the target, reject non-positive or invalid durations, and default to 30 seconds. List parsers accept space-separated flag/value pairs, reject unknown options and positionals, preserve repeated ordinary filters, and add each value to the query. `--timeout` is not repeat-preserving: the last valid occurrence applies. The equals form is not accepted.
- `usageError` routes known resources to `resourceUsageError`, and `knownAction` lists accepted actions. Adding `sources` requires extending this resource/action inventory so missing actions, unknown actions, and malformed command vectors receive source-specific usage diagnostics rather than “unknown resource” errors.
- `helpFor` recognizes exact help argument vectors. Root, resource, and command help return before credentials or requests are used. `help.go` owns human-readable help text. The root help lists resources, and each resource help lists its actions.
- Existing tests use loopback `httptest` servers and dummy credentials. `TestMain` clears inherited Uptime credentials and installs a loopback-only network guard. S2 tests must use the Telemetry REST endpoint input and dummy Telemetry credentials. They must not make live requests or read real credentials.
- Repository instructions require a changelog fragment, accurate anchor index, memory updates for changed behavior, README documentation for active environment variables, endpoints, or commands, and a report of checks run and not run. S2 activates `BETTERSTACK_TELEMETRY_API_TOKEN` and source commands, so the implementation commit must update `README.md`, `.mem/`, `TAGS.md`, and `changelog.d/` as applicable. Update `LOG.md` or `log.d/` only if implementation adds non-obvious rationale worth preserving. Do not add a Makefile.

### Approved source API contract

- Ticket 01 and ticket 03's research asset establish `GET https://telemetry.betterstack.com/api/v2/sources` and `GET https://telemetry.betterstack.com/api/v2/sources/{id}` with `Authorization: Bearer <token>`.
- `BETTERSTACK_TELEMETRY_API_TOKEN` is the source credential. No source command may fall back to `BETTERSTACK_UPTIME_API_TOKEN` or another credential variable. Validate arguments before reporting a missing token, consistent with the existing command handlers. A valid command with an unset or empty Telemetry token fails locally, names `BETTERSTACK_TELEMETRY_API_TOKEN`, and sends no request.
- The list API names its integer query parameters `page` and `per_page`. The documented maximum for `per_page` is 50. The sources documentation gives no default for either parameter. Do not invent or send a default value when its flag is absent. The general Uptime pagination reference says pages start at 1, but this is only an inference for Telemetry. Use an unsigned base-10 integer value containing decimal digits only. A supplied page must be positive. A supplied per-page value must be from 1 through 50 inclusive. Reject non-integer, zero, negative, or over-limit values locally before credentials or HTTP access. Repeated `--page` and `--per-page` occurrences follow the existing list-parser convention: validate each occurrence and preserve each as a query occurrence. An explicit empty value is invalid for these numeric flags.
- `sources get` takes one required string ID. The API documents the ID as a string, not a numeric type. Treat it as opaque and escape it as one URL path segment. Reject a missing or empty ID and extra positional arguments locally. Use the established option parsing convention for `--timeout`; do not add URL lookup or numeric-only restrictions.
- Both commands accept `--timeout <duration>`. Use the existing 30-second default and positive Go duration semantics. Support the space-separated value form used by current Uptime commands, including the established placement before or after a get ID. Repeated timeout flags use the last valid value. Reject missing, invalid, zero, or negative durations before making a request. Do not add `--timeout=<duration>` unless the existing command convention changes independently, which is outside S2.
- List success responses contain a top-level `data` array and `pagination` object. The get response is a source object with `id`, `type`, and `attributes`, without the list `data` wrapper. The shared request path treats bodies as opaque; the CLI does not validate, decode, reformat, append a newline, or paginate them.
- Official source examples include a `token` field. Its exact meaning and permissions remain unconfirmed. Ticket 09 and ticket 10 settle the user-facing policy: preserve the complete upstream body, including `token`, and give a generic secret-credential warning. Do not claim what the token permits.
- Ticket 11 limits secret-body redaction to non-2xx SQL responses in S4. Source success and error bodies remain unchanged under the shared request contract. Do not redact a source `token` from either response channel.
- The source record does not document the regional SQL query host. Its `table_name` example is `nginx`, without the `t<team_id>_` prefix used by the approved SQL table format. This S2 command does not derive SQL hosts or table names.
- Ticket 03 documents a 404 body for an unknown source ID. Other source API error formats, list defaults, Telemetry-specific pagination defaults, and source rate limits are not documented. Preserve upstream bodies without assumptions about their schema.

### Settled decisions and exclusions

- Keep the production Telemetry REST host fixed and use `runWithEndpoints` only to inject local test endpoints. Keep the SQL URL as the only user-configurable service host for later SQL work.
- Reuse `doMonitorsRequest` for both source GETs. Preserve its one-request, no-redirect, timeout, and raw-body contract. Do not create a second HTTP response policy for sources.
- Keep raw source JSON unchanged, including its `token` field. Warn users on both `sources list` and `sources get` help. Use this generic wording guidance: “The output includes each source's `token` field. Treat it as a secret credential. Keep it private and do not paste it into shared logs or tickets.” Do not state that the token is an ingest token or make any permission claim.
- Extend exact root, resource, and command help for `sources`, `sources list`, and `sources get`. Extend source usage routing and known-action handling. Do not add `logs` help or logs resource usage; milestone 006 owns logs help.
- Do not add source writes, automatic pagination, source lookup by name, source-to-SQL mapping, response redaction, retries, configurable REST hosts, Uptime-token fallback, or live API access.

## Section 2: Requirements

### R1 — Route source commands through the Telemetry REST endpoint

`bs sources list` and `bs sources get <id>` each issue exactly one GET to the corresponding API path under the Telemetry REST endpoint supplied to `runWithEndpoints`. The production endpoint remains the fixed Telemetry REST host. Both commands use the shared `doMonitorsRequest` response path and bearer authentication. Existing `runWithAPI` callers and all Uptime routing retain their current behavior.

**Acceptance:** loopback tests observe one GET to `/api/v2/sources` or the escaped `/api/v2/sources/<id>` path, the Telemetry Bearer header, and no request to the Uptime or SQL endpoint. Existing Uptime tests continue to pass.

### R2 — Use only the Telemetry API token

A valid source command uses `BETTERSTACK_TELEMETRY_API_TOKEN`. An unset or empty value returns a readable error that names the variable and performs no request. The command never falls back to `BETTERSTACK_UPTIME_API_TOKEN` or another credential. Invalid arguments fail before the missing-token check, as existing handlers do.

**Acceptance:** tests cover a dummy Telemetry token in the observed Authorization header, unset and empty Telemetry token values, an Uptime token present without a Telemetry token, and zero requests for all missing-credential cases. Test output does not expose dummy credentials.

### R3 — Validate list pagination flags and map API parameters

`bs sources list` accepts `--page <integer>`, `--per-page <integer>`, and `--timeout <duration>` as space-separated flag/value pairs. A supplied page must be an unsigned base-10 integer containing decimal digits only and greater than zero. A supplied per-page value must be an unsigned base-10 integer containing decimal digits only and from 1 through 50 inclusive. Invalid, empty, zero, negative, and out-of-range numeric values fail locally with a flag-specific usage diagnostic and no request. An absent pagination flag is omitted from the query because the Telemetry documentation defines no default. Each valid `--page` maps to `page`, and each valid `--per-page` maps to `per_page`. Repeated pagination flags retain every valid occurrence, matching existing list parser behavior. No equals form, unrelated filters, or positional arguments are accepted.

**Acceptance:** tests cover absent values, valid boundary values, query names and values, repeated flags, non-integers, empty values, zero, negatives, `--per-page` 51, missing flag values, unknown flags, equals forms, and unexpected positionals. Rejected inputs make zero requests.

### R4 — Validate source get IDs and timeout flags

`bs sources get` accepts exactly one nonempty opaque string ID and an optional `--timeout <duration>`. It URL-escapes the ID as one path segment. It does not impose a numeric-only rule or interpret the ID as a URL. Missing, empty, and extra IDs fail locally without a request. A target token that conflicts with the existing option syntax is rejected as an option, consistent with current get parsers.

Both source commands use a 30-second default timeout. A supplied timeout must be a positive Go duration. The space-separated form is accepted, the established before-or-after-target placement is accepted for get, and the last repeated valid timeout applies. Missing, invalid, zero, or negative timeout values fail locally. The `--timeout=<duration>` form remains unsupported.

**Acceptance:** tests cover opaque and escapable IDs, encoded path boundaries, missing/empty/extra IDs, default and valid timeout values, timeout placement, repeated timeout values, and invalid duration cases. Rejected inputs make zero requests.

### R5 — Preserve source response bodies unchanged

For any 2xx response, each complete response body is written byte-for-byte to stdout, with no decoding, formatting, added newline, pagination follow-up, or secret redaction. For any non-2xx response, including 3xx, the complete response body is written byte-for-byte to stderr and the command exits 1. Redirects are not followed. Transport, timeout, and body-read failures use the shared request path's local diagnostic behavior.

**Acceptance:** loopback tests cover raw JSON and non-JSON 2xx bodies, a body with its own newline, representative non-2xx responses including 404, one request only, no redirect follow, and empty output on the opposite channel. Tests confirm that a dummy source `token` string is not altered in either response body.

### R6 — Integrate source usage diagnostics and help

Extend the routing usage gate so `sources` is a recognized resource and `list` and `get` are recognized actions. Missing actions, unknown actions, missing IDs, extra IDs, unknown options, and invalid flag values produce readable source-specific usage errors and no HTTP request. Add exact help forms for `bs sources --help`, `bs sources list --help`, and `bs sources get --help`. Add `sources` to root help and list `list` and `get` in source resource help. Each command help includes its arguments, flags, examples, timeout behavior, and the generic secret-credential warning. Help must return before credentials or HTTP requests are used.

**Acceptance:** tests cover all three help levels, root resource listing, source actions, warning text on both command help pages, malformed source usage vectors, empty stderr on help, exit codes, and zero requests for help and invalid usage. No logs help text or logs root/resource entry is added in this milestone.

### R7 — Preserve the secret warning without claiming token permissions

The help text for both source commands must warn that response output includes each source's `token` field and that the value is a secret credential. It must advise keeping the value private and not pasting it into shared logs or tickets. It must not claim what the token permits or call it an ingest token.

**Acceptance:** direct help assertions find the generic warning in both command help outputs and reject wording that assigns token permissions or unsupported meaning.

### R8 — Add applicable repository records and verify the implementation

The S2 implementation commit updates `README.md` to document the newly active source commands, fixed REST endpoint, and `BETTERSTACK_TELEMETRY_API_TOKEN`; updates `.mem/` through the memory skill for implemented behavior and updates its map if paths change; updates `TAGS.md` for every changed or added production anchor; and adds a factual `changelog.d/` fragment. Keep `LOG.md` and `log.d/` consistent, adding provenance only for non-obvious rationale. Use existing anchors where they describe the same boundary and do not add anchors to helpers. Run the required Go build, vet, full tests, formatting, direct source checks, and staged-path/whitespace checks. Use only loopback test servers and dummy credentials.

**Acceptance:** repository records match the implementation; `go build ./...`, `go vet ./...`, `go test ./...`, and `gofmt -l cmd/bs/*.go` pass; direct tests prove request, auth, flag, ID, response, help, and safety behavior; `git diff --check` and `git diff --cached --check` pass; only authorized S2 implementation and record files are staged. The report names checks that could not run. No live request or real credential is used.

### R9 — Keep later milestone behavior out of S2

S2 does not implement `logs` routing or help, SQL query construction or execution, SQL URL parsing, source-derived SQL table names, token redaction, automatic pagination, source writes, or production endpoint overrides. Source output remains raw even when it contains a token. Milestone 006 owns logs help, and milestone 005 owns redaction of non-2xx SQL responses.

**Acceptance:** changes contain no logs command or SQL behavior, do not redact source bodies, do not paginate, and do not alter Uptime command behavior or endpoint policy.

### Ordering and parallel work

R1 and R2 establish routing and authentication boundaries. R3 and R4 define independent list/get argument contracts and can be developed in parallel after the shared timeout, request, and diagnostic behavior is fixed. R5 constrains both commands. R6 and R7 define help and secret-warning acceptance. R8 gates the complete S2 implementation commit. R9 constrains every slice.

## Evidence and decisions

- [Ticket 01](../../.wayfinder/telemetry/tickets/01-charting-decisions-destination-scope-and-command-surface.md) and [ticket 02](../../.wayfinder/telemetry/tickets/02-technical-design-decisions-recorded-at-charting.md) define the command surface, fixed REST host, separate credential, shared request path, and endpoint seam.
- [Ticket 03](../../.wayfinder/telemetry/tickets/03-research-sources-api-contract-from-docs.md) and [`assets/03-sources-api-contract-research.md`](../../.wayfinder/telemetry/assets/03-sources-api-contract-research.md) define the documented paths, methods, auth header, query names, `per_page` maximum, response shapes, string ID, documented 404, and gaps in defaults and other errors.
- [Ticket 09](../../.wayfinder/telemetry/tickets/09-grilling-table-name-format-and-source-token-output.md) settles unchanged source output and warnings for both source help pages. [Ticket 10](../../.wayfinder/telemetry/tickets/10-technical-decisions-from-live-verification.md) refines the warning to the generic secret-credential language and prohibits claims about token permissions.
- [Ticket 11](../../.wayfinder/telemetry/tickets/11-sql-api-errors-concurrency-time-bounds.md) assigns response-body redaction only to non-2xx SQL responses in S4. Therefore S2 source response bodies remain unchanged on success and failure.
- Existing parser conventions in [`cmd/bs/monitors.go`](../../cmd/bs/monitors.go) and [`cmd/bs/incidents.go`](../../cmd/bs/incidents.go) establish space-separated values, strict option handling, duration parsing, repeated-filter preservation, timeout defaults, and local validation before requests. The Telemetry source API's explicit integer constraints and `per_page` maximum require stricter validation than the existing pass-through monitor pagination values.
- No Better Stack API request or credential was used during specification work.

## Unresolved questions

None. The Telemetry API does not document `page` or `per_page` defaults. The spec omits absent query parameters instead of guessing a default. The source `token` meaning remains unknown, and the approved warning avoids claims about it. These are settled constraints, not blockers.
