# Milestone 004 Specification — S3 validated ClickHouse search query builder

- Milestone source: `docs/plans/008_telemetry-logs-search_milestones.md`, entry `004` (S3)
- Scope: milestone 004 only. Build and validate the search query and its parameters. Do not issue an HTTP request, add SQL routing, or add help text.
- Acceptance in one line: valid search flags produce a validated hot-table query, with the approved optional cold-table union, parameterized values, bounded ordering and limits, and valid time bounds; invalid flags fail locally.

## Section 1: Context & Constraints

### Repository and milestone boundary

The CLI is a standard-library Go program in `cmd/bs`, package `main`. `runWithEndpoints` accepts a Telemetry SQL endpoint input, but no SQL request route is active. Milestone 004 supplies the query builder and its flag validation for the later SQL executor in 005 and command routing/help in 006. This milestone does not add either later behavior. Its tests use fake input strings and inspect builder results. They make no HTTP request and use no live service or credentials. The current shared parser conventions use space-separated flag/value pairs, return local errors before request execution, and test parsing separately from request behavior (`cmd/bs/sources.go`, `cmd/bs/incidents.go`).

The milestone is full lane under D1 because the project milestones affect external API contracts, security, or repository-wide records. The query builder crosses the SQL API boundary: it interpolates only a validated table identifier, binds user query and time values, and emits the approved SQL shape. The builder must not add a dependency.

### Approved SQL and search contract

- Ticket 01 defines `bs logs search`, search flags rather than raw SQL, the full `--table` selector, `--limit` default 100 and maximum 1000, hot storage by default, and opt-in cold storage.
- Ticket 02 defines `--since` default `1h`, mutual exclusion with `--from` and `--to`, `--query` as a case-insensitive substring search on `raw`, rejection of control characters, `--order asc|desc`, the table grammar `^t[0-9]+_[a-z0-9_]+$`, and the initial flag contract. Its no-parameter-binding choice is superseded by ticket 10.
- Tickets 04 and 05, including assets `04-sql-api-contract-research.md` and `05-live-verification-results.md`, establish `raw`, `dt` as `DateTime64(6, 'UTC')`, hot table `remote(<table>_logs)`, cold table `s3Cluster(primary, <table>_s3)`, cold `_row_type = 1`, and verified forwarding of `param_*` query parameters. The live evidence does not authorize another request.
- Ticket 10, decision O3, requires ClickHouse query parameters for search text and time values. Table identifiers cannot be parameters in `remote()` or `s3Cluster()`, so validate the full identifier before interpolation. Decision O4 requires `dt >= fromUnixTimestamp64Milli({from:Int64}, 'UTC') AND dt < fromUnixTimestamp64Milli({to:Int64}, 'UTC')`, using Unix-millisecond `Int64` values.
- Ticket 11 requires `--from` and `--to` to parse with `time.RFC3339Nano`, converts them with `UnixMilli()`, requires `from < to`, treats the lower bound as inclusive and upper bound as exclusive, uses the current time when `--from` is provided without `--to`, and rejects `--to` without `--from`. `--since` remains mutually exclusive with either explicit bound.
- Ticket 10 decision O6 and ticket 05 authorize `SETTINGS max_execution_time` from the positive timeout, rounded up to whole seconds. The default timeout is 30 seconds for hot and cold searches. Server acceptance was observed, but cancellation of server work was not proven.
- Ticket 06 confirms that cold searches keep the same 30-second default. A user can provide a larger timeout. No latency guarantee is made.
- The output contract is `FORMAT JSONEachRow`. Ticket 11 assigns HTTP behavior and non-2xx redaction to 005, not this builder.

### Decisions and attribution

D1–D9 are inherited from `/tmp/betterstack-cli-factory-008-handoff.md`. Relevant constraints are: full lane (D1); the 005 SQL routing and response-redaction ownership (D5 and D6); quoted scratch scripts under `/tmp/factory-NNN/` when execution starts (D8); and a separate review sub-agent per milestone (D9). The repository requires a changelog fragment, matching anchor index, memory updates for changed behavior or structure, and README updates only when a user-visible surface changes. D4 fixes the changelog name pattern to `changelog.d/008-telemetry-logs-search-NNN.md`.

Jaron Swab supplied these decisions in this session. Record them unchanged in the milestone checkpoint:

- **D10 — “Positive Go durations.”** `--since` accepts positive Go durations through `time.ParseDuration`, such as `30m`, `1h`, and `2h45m`. Its default is `1h`. Reject zero, negative, and malformed values. Go durations have no day suffix, so reject `7d`; users can write `168h`.
- **D11 — “1 through 1000.”** `--limit` accepts integers from 1 through 1000 inclusive. Its default is 100. Reject zero, negative, greater-than-1000, and non-integer values.
- **D12 — “Reject duplicates.”** Reject a repeated single-value search flag and name that flag in the error. The precedent is duplicate metadata-key rejection in `cmd/bs/incidents.go:315-344`, tested in `cmd/bs/incidents_test.go:1944-1970`. This decision does not change existing `--page`, `--per-page`, or other repeatable-flag behavior. The source pagination precedent is `cmd/bs/sources_test.go:100-123`; other examples are `cmd/bs/monitors.go:160-163` and `cmd/bs/incidents.go:218-220`.
- **D13 — “Descending, newest first.”** If `--order` is omitted, the query uses `ORDER BY dt DESC`.

### Settled parser details and scope exclusions

Use the established space-separated flag/value form. Treat value-bearing search flags as single-value flags under D12. `--cold` is a valueless switch, so repeated occurrences have the same enabled state and are accepted idempotently. Match the existing strict parser style for enumerated values and decimal integer input. An explicitly empty `--query` is a valid string and matches every `raw` value as an empty substring; an omitted query adds no raw-text predicate. These interpretations make the accepted inputs deterministic without changing the approved search meaning.

Do not add an HTTP client call, SQL POST, endpoint routing, retries, redirect behavior, credentials, SQL URL parsing, response handling/redaction, command routing, root/resource/command help, or README command documentation. Milestone 005 owns SQL execution and milestone 006 owns routing and help. Do not change existing repeatable flag behavior in other commands.

## Section 2: Requirements

### R1 — Validate the table identifier before query construction

Search requires exactly one `--table` value. The value must match `^t[0-9]+_[a-z0-9_]+$`. Reject an absent, empty, repeated, or non-matching value locally. Only a validated value may be interpolated into a hot or cold table expression. No user-provided SQL fragment may be accepted.

**Acceptance:** tests cover a valid full table name and invalid empty, prefix-less, uppercase, punctuation, whitespace, and malformed-prefix values. Invalid table inputs return an error naming `--table`; no query is produced.

### R2 — Build the hot-table query by default

A valid search without `--cold` reads `dt, raw` from `remote(<table>_logs)`. The query applies its selected time predicate and optional raw-text predicate, orders by `dt`, applies the validated limit, sets `max_execution_time`, and requests `FORMAT JSONEachRow`. The default order is `dt DESC` under D13. An explicit `--order asc` uses `dt ASC`.

**Acceptance:** fake-string tests verify the hot table suffix, selected columns, filter predicates, ordering, limit, timeout setting, and output format. They prove cold-table expressions are absent by default.

### R3 — Add the opt-in cold-table union

With `--cold`, combine the hot table and `s3Cluster(primary, <table>_s3)` with `UNION ALL`. Restrict cold rows with `_row_type = 1`. Apply the same selected time and raw-text filters to the combined search result, then apply one ordering, limit, timeout setting, and output format to the complete result. Repeated `--cold` switches remain enabled and do not create extra union branches.

**Acceptance:** fake-string tests verify one hot branch, one cold branch, `UNION ALL`, the `_row_type = 1` predicate, one final ordering/limit/settings/format, and no extra branch for repeated switches.

### R4 — Bind query text instead of interpolating it

When `--query` is present, match it case-insensitively as a substring of `raw`, using the verified ClickHouse `positionCaseInsensitiveUTF8` function. Put the query text in a ClickHouse typed query parameter and return its matching `param_*` value separately from SQL text. Reject any control character in the supplied value. Preserve ordinary punctuation, quotes, backslashes, and SQL-looking text as parameter data. An empty explicit string is valid and behaves as an empty substring. An omitted flag produces no raw predicate or query parameter.

**Acceptance:** fake-string tests verify the typed placeholder and parameter value separately. Cases include mixed case, empty text, quotes, backslashes, SQL-looking text, NUL, newline, and another control character. The input text never appears as an interpolated SQL literal.

### R5 — Validate ordering and bounded limits

`--order` accepts only `asc` or `desc`. If absent, use `desc` under D13. `--limit` accepts a decimal integer from 1 through 1000 inclusive and defaults to 100 under D11. Reject invalid values locally. Both flags are single-value flags under D12.

**Acceptance:** tests cover both order values, omitted order, invalid case/value, the limit default, 1, 1000, 0, a negative value, 1001, non-integer input, and repeated values. Errors name the repeated or invalid flag. Invalid inputs produce no query.

### R6 — Validate duration windows and resolve `--since`

`--since` defaults to `1h` and accepts only a positive value parsed by Go's `time.ParseDuration`. Reject zero, negative, malformed values, and unsupported day suffixes such as `7d`. A valid `--since` defines an inclusive lower time bound at the captured current time minus the duration and an exclusive upper bound at that same current time. Convert both bounds to Unix milliseconds and bind them as `Int64` parameters.

`--since` is mutually exclusive with `--from` and `--to`. `--from` and `--to` parse with `time.RFC3339Nano`. `--from` alone uses the captured current time as `to`; `--to` alone is rejected. Require `from < to`; equal or reversed bounds fail. Convert explicit bounds to Unix milliseconds, truncating sub-millisecond precision as `UnixMilli()` does. Both bounds use an inclusive lower and exclusive upper comparison. All these flags are single-value flags under D12.

**Acceptance:** parser and builder tests use a fixed time input to prove default and explicit duration windows deterministically. They cover `30m`, `1h`, `2h45m`, `7d`, empty/malformed/zero/negative durations, both explicit bounds, each single-bound case, invalid RFC3339Nano values, equal/reversed bounds, sub-millisecond conversion, mutual exclusion, repeated flags, and the exact typed time placeholders with Unix-millisecond parameter values.

### R7 — Derive server execution time from the client timeout

The search timeout defaults to the existing 30-second `defaultTimeout` and accepts a positive Go duration through `--timeout`. Under D12, reject a repeated `--timeout` and name the flag. Set `max_execution_time` to the timeout in whole seconds, rounding any positive fractional second up. Do not claim this setting guarantees cancellation of server work.

**Acceptance:** tests cover the default, one second, a fractional duration below one second mapping to 1, an exact multi-second value, a fractional multi-second value rounding upward, invalid/zero/negative values, and duplicates. The generated setting is an integer number of seconds.

### R8 — Reject duplicate single-value flags before producing a query

Reject a second occurrence of any value-bearing search flag: `--table`, `--since`, `--from`, `--to`, `--query`, `--order`, `--limit`, or `--timeout`. Report the repeated flag by name, even when both values match. Do not apply this rule to `--cold`, a valueless switch, or alter repeatable flags in other commands.

**Acceptance:** table-driven parser tests repeat every listed flag with equal and different values and assert a flag-specific duplicate error. Tests verify repeated `--cold` remains enabled and existing source, monitor, and incident repeatable-flag tests retain their behavior.

### R9 — Keep S3 a query-builder-only milestone

The builder and flag validation return local query data or a local error. They issue no request and do not inspect endpoints or credentials. SQL execution, routing, help, and response policy remain assigned to later milestones. Repository records required for any implementation change are specified in the implementation checklist. Tests use fake strings only.

**Acceptance:** the implementation adds no request call, route, or help text. Tests invoke only parser/builder boundaries. Repository diff and test review show no live request or credential use.

### Requirement sources

- R1–R3, R5: [ticket 01](../../.wayfinder/telemetry/tickets/01-charting-decisions-destination-scope-and-command-surface.md), [ticket 02](../../.wayfinder/telemetry/tickets/02-technical-design-decisions-recorded-at-charting.md), [ticket 04](../../.wayfinder/telemetry/tickets/04-research-sql-api-contract-from-docs.md), [ticket 05](../../.wayfinder/telemetry/tickets/05-task-live-verification-sql-api-log-shape.md), and [ticket 09](../../.wayfinder/telemetry/tickets/09-grilling-table-name-format-and-source-token-output.md).
- R2–R4, R6–R7: [ticket 10](../../.wayfinder/telemetry/tickets/10-technical-decisions-from-live-verification.md), including O3, O4, and O6.
- R3, R6–R7: [ticket 11](../../.wayfinder/telemetry/tickets/11-sql-api-errors-concurrency-time-bounds.md) and [ticket 06](../../.wayfinder/telemetry/tickets/06-research-cold-storage-timeout-default.md).
- D10–D13: Jaron Swab's decisions in this session, recorded above.

## Ordering and parallel work

The parser and query builder form one vertical slice because validated arguments directly determine safe SQL construction. Implement and test table and general flag validation first. Add the hot query next, then the cold union, parameterized query text, time handling, and timeout-derived settings. Duplicate rejection applies to every value-bearing flag and should be tested with the parser before the full query acceptance pass. Repository records and V1 verification gate the implementation commit. No implementation work is parallel because every builder slice depends on the same validated argument model.

## Unresolved questions

None. D10–D13 settle the additional session decisions. The remaining details above follow explicit ticket behavior or repository parser conventions and do not require another product decision.
