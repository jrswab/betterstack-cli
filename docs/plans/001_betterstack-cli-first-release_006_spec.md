# Milestone 006 Specification — `bs incidents list` with the full documented incident filter surface

Milestone source: `docs/plans/001_betterstack-cli-first-release_milestones.md`, entry 006 (unchecked). This spec covers milestone 006 only; `incidents get`, `incidents timeline`, monitor behavior changes, and all other milestones are out of scope.

## Section 1: Context & Constraints

### Codebase state and established patterns

- Milestones 001–005 are complete. `cmd/bs/main.go` exposes the test seam `runWithAPI(args []string, stdout, stderr io.Writer, baseURL string, client *http.Client) int`; tests inject a loopback base URL and client (`cmd/bs/main_test.go`, `cmd/bs/monitors_test.go` `invokeAPI`, `cmd/bs/incidents_test.go`). Every request stays on a local mock server; no flag, environment variable, or file changes the injection at runtime.
- `bs incidents list` currently answers "not implemented" through `resourceUsageError` (`cmd/bs/main.go`): the `list` branch returns `bs: "incidents list" is not implemented in this milestone; run 'bs incidents list --help' for usage.` with exit 1. This is the exact behavior 006 replaces for the bare `incidents list` vector only. The `list` branch also rejects extra positional arguments after `list`; after 006 routes the command, that rejection for `incidents` happens inside the new command's parser instead, and the branch becomes defensively unreachable but stays.
- `cmd/bs/monitors.go` provides the structural precedent 006 reuses: `monitorsList` (token read for redaction, parse, credential check, query build, one `GET`), `parseMonitorsListArgs` (repeatable flag slices, per-flag value consumption from the following argument, `--timeout` duration grammar), `query()` (stdlib `url.Values` translation, omission sends nothing, empty string value sends the parameter with an empty value), and the shared endpoint-agnostic executor `doMonitorsRequest` (bearer header, one request, redirects not followed via `http.ErrUseLastResponse`, raw 2xx body to stdout with no appended newline, raw non-2xx body to stderr, exit 0/1, `redactToken` on locally generated diagnostics only). `cmd/bs/incidents.go` adds `resolveIncidentTarget` (not needed by 006 — list takes no target).
- Help text: `incidentsListHelp` (`cmd/bs/help.go`) already documents `bs incidents list` with `--from`, `--to`, `--monitor-id`, `--heartbeat-id`, `--resolved <true|false>`, `--acknowledged <true|false>`, `--metadata <key=value>`, `--team-name`, `--page`, `--per-page`, `--timeout`, and examples including `--monitor-id 123 --resolved=false` and `--metadata Origin=Linear --per-page 50`. No help-text change is anticipated (the 004/005 situation); help text stays frozen in this milestone.
- Test-preservation surface: `TestUnimplementedCommandsMakeNoRequest` (`cmd/bs/monitors_test.go`, line ~392) lists `incidents list` as not-implemented and must drop that case when the vector becomes working (the 004 B1/005 B1 precedent). `TestInvalidUsageMatrix` (`cmd/bs/main_test.go`) has no `incidents list` case, so no stale entry exists there.
- Verification environment: `GOTOOLCHAIN=go1.26.9 GOPROXY=off` from the local module cache (local default `go` is 1.22.2, below the module minimum `go 1.26`).

### Decisions already made (settled; never re-evaluate)

Sources: `.wayfinder/tickets/03-command-naming-and-argument-conventions.md` (Resolution and the second through seventh approvals), `.wayfinder/tickets/04-pagination-and-json-envelope.md` (Resolution), `.wayfinder/tickets/05-errors-timeouts-retries-token-safety.md` (Resolution), `.wayfinder/assets/01-api-contract-research.md` (sections 3 and 5).

- Endpoint: `GET https://incidents.betterstack.com/api/v3/incidents` (ticket 01; incidents are v3). List incidents paginates with `page`/`per_page`, body `pagination` object of absolute URLs, documented default 10 and maximum 50 per page (asset 01, sections 3 and 5) — upstream knowledge the CLI never enforces locally.
- Documented incident filters: `team_name` (string, global-token filter), `from` and `to` (date, YYYY-MM-DD), `monitor_id` and `heartbeat_id` (integer), `resolved` and `acknowledged` (bool — omitted lists both), `metadata` (nested, e.g. `?metadata[key][][value]=value`); the typed variants `metadata[key][][type]` and `metadata[key][][email]` exist in the API but are excluded from the first release (asset 01, section 3).
- Success envelope: raw byte-for-byte passthrough of the response body to stdout — no parsing, validation, re-serialization, pretty-printing, compact mode, `--pretty` flag, or appended newline (ticket 04). `data`, `included`, and `pagination` pass through as the API sent them. The only compatibility promise is that `bs` preserves the response as received.
- Pagination: one API page by default. Optional `--page <n>` and `--per-page <n>` map one-to-one onto `page` and `per_page`. Given values pass through unchanged for upstream validation — no hardcoded limits, no clamping, no substitution, no pre-request rejection, and no local numeric grammar beyond passthrough. Omitting a flag sends no parameter, so the API default applies (ticket 04).
- No auto-drain: the CLI never follows `pagination.next`; no `--all`-style flag exists (ticket 04).
- Filter flags: named hyphenated flags mapping onto the underscore API parameters — `--from`, `--to`, `--monitor-id`, `--heartbeat-id` (ticket 03). There is no `--query "..."` raw passthrough (rejected approach, ticket 03).
- Boolean flags: `--resolved` and `--acknowledged` require an explicit value. The approved documented shape is the equals form — `--resolved=true`, `--resolved=false`, `--acknowledged=true`, `--acknowledged=false` (ticket 03 third approval, which names exactly these forms); the established 003 parser convention and the existing help text (`--resolved <true|false>`) use the space form, where the flag consumes the following argument. Both forms are approved surfaces: the equals form is the documented shape, the space form is the established flag grammar. A bare flag with no value is a syntax error (ticket 03).
- Boolean omission: omitting a boolean flag omits the query parameter, so the documented API default applies (omitted `resolved` lists both resolved and unresolved incidents); the CLI supplies no default of its own (ticket 03).
- Metadata flag: repeatable `--metadata key=value`, one pair per occurrence; each pair is translated into the documented nested query form (`--metadata Origin=Linear` → `metadata[Origin][][value]=Linear`) (ticket 03). Different keys may repeat; a duplicate key within one invocation is a syntax error (ticket 03, future-revisitable). The typed variants have no flag in the first release (ticket 03).
- `--team-name <name>`: optional, passed through unchanged with no token-type detection — no classification, warning, or rejection; omitting it sends no `team_name` parameter (ticket 03). The documented filter applies only with a global token; the undocumented team-scoped-token case is left to whatever the API returns, handled by the error rules (ticket 05).
- Errors: every non-2xx response — including 3xx — prints the upstream body to stderr byte-for-byte with no wrapper or added status summary, and the CLI exits 1. Redirects are not followed (one request to the requested endpoint, so the `Authorization` header never reaches a server-chosen target). No automatic retries — one API request attempt per invocation, no `Retry-After` handling (ticket 05).
- Exit codes: 0 on success, 1 on every failure including invalid usage (ticket 05). stdout carries successful bodies only.
- Credentials: missing or empty `BETTERSTACK_UPTIME_API_TOKEN` prints an actionable stderr message naming the variable and exits 1 before any request. The former `BETTERSTACK_API_TOKEN` name is never read (ticket 05).
- Timeout: 30-second default total request timeout, overridable per invocation with `--timeout` accepting a duration value; duration grammar specifics are an implementer choice (ticket 05). Local/network failures print one concise, actionable, token-free stderr message and exit 1.
- Token safety: the token is never printed, logged, or stored. No redaction or inspection of response bodies; the raw passthrough stays untouched (ticket 05).

### Constraints and assumptions

- Tests use only dummy tokens and mock loopback servers; no live API call, external request, publication, push, tag, or deletion is authorized.
- Every shell begins with `unset BETTERSTACK_UPTIME_API_TOKEN` and `unset BETTERSTACK_API_TOKEN`; no environment content is printed or logged.
- `verify-incident-id.sh` and the `.wayfinder/` planning tree stay untouched.
- The 002–005 seams and passthrough rules are reused unchanged. A minimal shared refactor inside `cmd/bs` is acceptable only if the observable behavior of the implemented commands does not change; avoid refactoring beyond what 006 needs.
- Technical decisions delegated to this spec by the task framing (settled here, not open questions): (a) the `--metadata` pair grammar — a pair must contain `=`, the first `=` splits key and value, the key must be nonempty, and an empty value is supported and passes through as an empty value; (b) the boolean value grammar — only the exact lowercase literals `true` and `false` are valid in either form; (c) repeated boolean occurrences pass through as given, aligned with the 003 repeated-flag passthrough rule.

## Section 2: Requirements

### R1 — Command surface and argument parsing

- R1.1 `bs incidents list` is a working command. It accepts no positional target: any positional argument after `list` is invalid usage (readable stderr, exit 1, no request).
- R1.2 Optional flags: `--page <n>`, `--per-page <n>`, `--from <value>`, `--to <value>`, `--monitor-id <id>`, `--heartbeat-id <id>`, `--resolved`, `--acknowledged`, `--metadata <key=value>` (repeatable), `--team-name <name>`, `--timeout <duration>`. Flags may appear in any order. In the space form every flag consumes its value from the following argument; a flag missing its value is invalid usage (exit 1, no request).
- R1.3 Boolean forms and values. Accepted forms for `--resolved` are exactly: `--resolved=true`, `--resolved=false`, `--resolved true`, `--resolved false`; likewise `--acknowledged=true`, `--acknowledged=false`, `--acknowledged true`, `--acknowledged false`. The value must be the exact lowercase literal `true` or `false`; any other value (`True`, `TRUE`, `1`, `0`, `yes`, an empty string) is invalid usage. A bare `--resolved` or `--acknowledged` with no following value is a syntax error. Omitting the flag sends no parameter.
- R1.4 `--metadata` pair grammar. The flag's value must contain `=`; the first `=` splits key and value; the key must be nonempty; the remainder of the value after the first `=` (including any further `=`) is the value and passes through unchanged; an empty value is valid and passes through as an empty value. A pair with no `=` or with an empty key is invalid usage. The equals form `--metadata=...` is not an accepted flag shape (unknown option).
- R1.5 Duplicate `--metadata` keys are rejected as invalid usage: giving the same key (exact, case-sensitive string equality after the first-`=` split) more than once in one invocation exits 1 with a readable stderr message and makes no request. Repeated occurrences with different keys are accepted.
- R1.6 Repeated non-boolean flags (`--page`, `--per-page`, `--from`, `--to`, `--monitor-id`, `--heartbeat-id`, `--team-name`): each occurrence's value passes through as given (no local deduplication, rejection, or last-wins rule — the 003 repeated-flag rule). Repeated boolean occurrences are likewise passed through as given; each occurrence must independently satisfy R1.3.
- R1.7 An unknown option (any argument starting with `-` that is not a listed flag, including `--all`, `--pretty`, `--query`, a bare `-h` or `--help` after `list` in the argument position, and equals forms of non-boolean flags such as `--page=2`) is invalid usage: readable stderr, exit 1, no request. `bs incidents list --help` remains help: it exits 0, prints the existing `incidentsListHelp` text to stdout, empty stderr, no request.
- Acceptance: mock-API checks prove each valid vector reaches the mock server exactly once and each invalid vector exits 1 with a readable, token-free stderr message and zero requests — including every boolean form, each boolean value rejection, each metadata grammar rejection, and the duplicate-key rejection.

### R2 — Credential channel

- R2.1 The token is read only from `BETTERSTACK_UPTIME_API_TOKEN`. Missing or empty: an actionable stderr message naming the variable, exit 1, before any request. `BETTERSTACK_API_TOKEN` set alone is ignored and never read.
- R2.2 Argument validation errors (R1) occur before the token check, matching the 002–005 order; the missing-token failure still occurs before any request.
- Acceptance: mock-API checks prove zero requests and a message naming `BETTERSTACK_UPTIME_API_TOKEN` for unset, empty, and former-name-only cases.

### R3 — Request construction

- R3.1 Exactly one `GET <baseURL>/api/v3/incidents` request per invocation, with `Authorization: Bearer <token>`.
- R3.2 Each given flag maps to one query parameter: `--page` → `page`, `--per-page` → `per_page`, `--from` → `from`, `--to` → `to`, `--monitor-id` → `monitor_id`, `--heartbeat-id` → `heartbeat_id`, `--resolved` → `resolved` (value the given validated literal `true` or `false`), `--acknowledged` → `acknowledged`, `--team-name` → `team_name`, and `--metadata <key=value>` → `metadata[<key>][][value]=<value>` (the documented nested form; one parameter per accepted pair). Values pass through unchanged: no clamping, substitution, rejection, type coercion, or validation beyond the boolean literal check (R1.3) and the metadata key rule (R1.4) — the documented `from`/`to` date grammar and the integer-ness of `monitor_id`/`heartbeat_id` are upstream concerns with no local date or number grammar. A flag present with an empty string value sends the parameter with an empty value; a flag absent sends no parameter.
- R3.3 The stdlib `net/url` query builder is the required encoding mechanism (the 003 precedent): it percent-encodes values and the `[`/`]` characters of the nested metadata key exactly as it encodes any parameter. Because duplicate metadata keys are rejected (R1.5), no two accepted pairs share a parameter name. No extra query parameters exist and no `--all` parameter exists.
- R3.4 Omission: any absent flag sends no parameter. With no flags given, the request carries no query parameters and the documented API defaults apply (incidents: default 10, maximum 50 per page — upstream knowledge only, never enforced locally).
- Acceptance: mock-API checks assert the exact query string for (a) no flags — no query parameters; (b) each flag alone in both accepted boolean forms; (c) all filters combined; (d) empty-string values including an empty metadata value; (e) repeated occurrences of one non-boolean flag and of different metadata keys; (f) the exact nested metadata parameter names, including bracket encoding.

### R4 — Success envelope

- R4.1 A 2xx response body is written to stdout byte-for-byte, with nothing appended or removed (no appended newline), including non-JSON bodies and bodies containing the `pagination` object. stderr stays empty; exit 0.
- R4.2 The body is not parsed, validated, or re-serialized; the `pagination` object is not read or acted on by the CLI.
- Acceptance: mock-API checks with bodies that include a `pagination` object, a non-JSON body, and a body with its own trailing newline; stdout equals the body exactly.

### R5 — Error behavior

- R5.1 Every non-2xx response — 3xx included — writes the raw upstream body to stderr byte-for-byte, writes nothing to stdout, exits 1, and reaches the server exactly once (no redirect following, no retry, no `Retry-After` handling).
- R5.2 Local and network failures (timeout expiry, connection refused, DNS/TLS failure) print one concise, actionable, token-free stderr message, write nothing to stdout, and exit 1.
- R5.3 The token never appears on stdout or stderr on any path (success, non-2xx, local failure). Locally generated diagnostics redact the token value; passthrough bodies are never redacted or inspected.
- Acceptance: mock-API checks mirror the 002–005 failure-class coverage for `incidents list`.

### R6 — Timeout

- R6.1 The total request timeout defaults to 30 seconds and is overridable with `--timeout <duration>`; a non-positive or unparseable value is invalid usage (exit 1, readable stderr, no request).
- Acceptance: mock-API checks prove a slow server fails under a short override with exit 1 and one request, and succeeds within a longer override.

### R7 — Absent features (fixed by the milestones doc and ticket 03; presence is a defect)

- No typed metadata variants: no flag or translation can ever produce `metadata[<key>][][type]` or `metadata[<key>][][email]`. No `--all` flag, no auto-drain of `pagination.next`, no `--pretty` flag, no `--query` flag, no incident flags beyond those in R1.2, no token-type detection for `--team-name`, no retry flag, no output modes, and no local boolean, date, or numeric default.- Acceptance: unknown-option checks (R1.7) cover `--all`, `--pretty`, and `--query` as invalid usage, and the source audit confirms no typed-variant parameter can be constructed.

### R8 — Regression preservation

- R8.1 `bs incidents get`, `bs incidents timeline`, `bs monitors list`, `bs monitors get`, all help levels, and all invalid-usage vectors keep their existing behavior; only the bare `incidents list` vector's outcome changes from "not implemented" to the working command. `TestUnimplementedCommandsMakeNoRequest` drops its `incidents list` case and keeps the remaining cases.
- R8.2 The implemented commands' observable behavior is unchanged (including if a minimal shared refactor is made). Help text, including `incidentsListHelp`, stays unchanged.
- Acceptance: the full existing test suite passes alongside the new checks.
