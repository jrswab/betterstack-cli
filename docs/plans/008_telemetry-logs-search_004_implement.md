# Milestone 004 Implementation Checklist — S3 validated ClickHouse search query builder

- Spec: `docs/plans/008_telemetry-logs-search_004_spec.md`
- Checkpoint: `docs/plans/008_telemetry-logs-search_004_checkpoint.md`
- Milestone source: `docs/plans/008_telemetry-logs-search_milestones.md`, entry `004`
- Scope: builder and flag parsing/validation only. Do not issue requests, activate SQL routing, or add logs help.
- State: the checkpoint owns task state. These boxes render from checkpoint task states.
- Preserve the five baseline untracked paths. Never stage, modify, or delete `.agents/`, `.wayfinder/`, `AGENTS.md`, `docs/plans/008_telemetry-logs-search_milestones.md`, or `verify-incident-id.sh`.
- Use only fake strings and fixed in-memory time values in tests. Do not make live requests or read real credentials. Never build a `bs` binary into the repository root.

## Section 1: Context Summary

Milestone 004 builds the validated SQL query that milestones 005 and 006 will consume. Search arguments must prevent unsafe table interpolation, bind user query and time values, implement the approved hot and optional cold table forms, and define deterministic bounds and settings. Jaron Swab's D10–D13 settle positive Go duration syntax, limit bounds, duplicate rejection, and descending default order. The implementation must preserve existing repeatable pagination flags in other commands. The query builder has no request, routing, credentials, or help responsibility. Use `/tmp/factory-004/` for direct check scripts when execution starts, and write any such scripts with quoted heredocs.

**Planned code and test surfaces:** add the builder and parser in `cmd/bs/logs.go`, with `parseLogsSearchArgs` and `buildLogsSearchQuery` as the behavior boundaries. Add fake-input tests in `cmd/bs/logs_test.go`. These are planned implementation surfaces, not existing symbols. A private argument or result representation may differ if it preserves the spec and the checklist's observable test boundaries. Do not add anchors to helpers. If implementation adds a production boundary anchor, update `TAGS.md` in the record task.

## Section 2: Implementation Checklist

Tasks are sequential because each slice depends on the same parsed search arguments. Each behavior task adds focused tests before the matching implementation and runs its focused tests before and after. Tests must inspect returned SQL and parameter values only. They must not construct an HTTP client or send a request.

### Slice A — Parse and validate search arguments

- [x] **A1 — Parse required table and enforce the search-flag contract.** In `cmd/bs/logs_test.go`, add test-first cases for `cmd/bs/logs.go:parseLogsSearchArgs`. Require exactly one `--table` value matching `^t[0-9]+_[a-z0-9_]+$`. Validate space-separated values for `--table`, `--since`, `--from`, `--to`, `--query`, `--order`, `--limit`, and `--timeout`. Reject unknown flags, equals forms, positionals, missing values, and invalid table forms. Reject a repeated value-bearing flag with a diagnostic naming that flag, including identical values. Detect repetition before accepting or validating the repeated value. Accept `--cold` as a valueless switch, including repeated occurrences, without multiplying cold branches. Preserve an explicitly empty `--query`; distinguish it from an omitted query. Run `go test -run 'TestParseLogsSearchArgs' ./cmd/bs` before and after implementation. Covers R1, R8, and R9.
- [x] **A2 — Validate order, limit, `--since`, and timeout values.** Extend the parser tests in `cmd/bs/logs_test.go`, then implement the matching validation in `cmd/bs/logs.go:parseLogsSearchArgs`. Use descending order when `--order` is absent. Accept only `asc` and `desc`. Use limit 100 when absent and accept decimal integers 1–1000 inclusive. Use `--since 1h` when absent and accept positive values parsed by `time.ParseDuration`; reject `7d`, malformed, empty, zero, and negative values. Use the existing 30-second default timeout and require positive Go durations. Reject duplicate `--order`, `--limit`, `--since`, and `--timeout` values under D12. Run the focused parser tests before and after implementation. Covers R5–R8.

### Slice B — Build the hot-storage query

- [x] **B1 — Build and test the parameterized hot query.** Add test-first cases in `cmd/bs/logs_test.go` for `cmd/bs/logs.go:buildLogsSearchQuery`, using a fixed `time.Time` input. Build the default hot query from the validated table with `remote(<table>_logs)`, selecting `dt, raw`, with `ORDER BY dt DESC`, the selected limit, `SETTINGS max_execution_time`, and `FORMAT JSONEachRow`. Map explicit `--order asc` to `ORDER BY dt ASC`. Convert the timeout to whole seconds by rounding any positive fractional second up. If `--query` is present, use `positionCaseInsensitiveUTF8` on `raw`, a typed ClickHouse placeholder, and a separate `param_*` value. Reject control characters, including NUL and newline. Keep quotes, backslashes, punctuation, and SQL-looking text in the parameter value, never in an interpolated SQL literal. Omit the raw predicate and parameter when `--query` is absent. Test empty query as a present empty parameter. Run `go test -run 'TestBuildLogsSearchQuery' ./cmd/bs` before and after implementation. Covers R2, R4, R5, R7, and R9.

### Slice C — Build deterministic time bounds

- [x] **C1 — Resolve relative and explicit bounds to Unix milliseconds.** Add test-first fixed-clock cases in `cmd/bs/logs_test.go`, then implement time resolution in `cmd/bs/logs.go:buildLogsSearchQuery` or a private helper called by it. For `--since`, calculate `from = now - duration` and `to = now` from the same captured `now`. For `--from` and optional `--to`, parse with `time.RFC3339Nano`; use `now` as `to` when it is omitted. Reject `--to` without `--from`, malformed values, equal/reversed bounds, and any `--since` combined with either explicit bound. Convert both bounds using Unix milliseconds, including truncation of sub-millisecond precision. Bind `from` and `to` as typed `Int64` query parameters in the approved `dt >= fromUnixTimestamp64Milli(...) AND dt < fromUnixTimestamp64Milli(...)` expression. Test the lower-inclusive and upper-exclusive operators and the separate numeric parameter values. Run focused builder/time tests before and after implementation. Covers R4, R6, and R8.

### Slice D — Add cold storage without changing hot defaults

- [x] **D1 — Add the opt-in cold branch and row filter.** Add test-first cases in `cmd/bs/logs_test.go`, then implement the `--cold` query form in `cmd/bs/logs.go:buildLogsSearchQuery`. Add exactly one `s3Cluster(primary, <table>_s3)` branch with `_row_type = 1`, combined with the hot branch through `UNION ALL`. Apply the chosen time and raw-text predicates to the complete result, followed by one final order, limit, timeout setting, and `FORMAT JSONEachRow`. Repeated `--cold` flags remain one enabled option. Verify default hot-only output and that the cold query has one branch of each kind. Run focused builder tests before and after implementation. Covers R2, R3, and R9.

### Slice E — Records and implementation verification

- [x] **E1 — Update records for the builder implementation.** Read and follow `.agents/skills/changelog/SKILL.md` and `.agents/skills/mem/SKILL.md`. Add `changelog.d/008-telemetry-logs-search-004.md` with a factual builder entry. Update `.mem/` to describe implemented SQL query construction in present tense, include a code example and a Mermaid flow where useful, add any new domain term to `.mem/terminology.md`, and link any new topic from `.mem/mem-map.md`. Keep each touched memory file at or below 250 lines. Update `TAGS.md` for every production anchor added, removed, or changed; do not add helper anchors. Update `README.md` only if this milestone changes a user-visible surface. Since command routing and help remain inactive until later milestones, do not document an unavailable `logs search` command here. Do not add `LOG.md` or `log.d/` rationale unless implementation creates a non-obvious decision not already captured by this spec. Run `make fragments-check` and `make docs-check` if available; otherwise record their actual unavailable results and use focused direct checks. Covers the repository-record portion of R9.
- [ ] **V1 — Pass the final S3 completion gate.** Run `GOFLAGS=-o=/dev/null go build ./...`, `go vet ./...`, `go test -count=1 ./...`, and `gofmt -l cmd/bs/*.go` (expect no paths). Run focused parser/builder tests and direct checks for SQL placeholders, parameter values, table validation, cold union, flags, bounds, duplicates, no request code, and record consistency. Create direct scripts only under `/tmp/factory-004/` using quoted heredocs. Run `git diff --check`; stage only the explicitly approved S3 implementation, tests, required records, and this milestone's planning artifacts. Inspect the exact staged paths and run `git diff --cached --check`. Obtain the separate review gate required by D9 before declaring completion. Compare implementation with every R1–R9 criterion, account for all baseline paths, and update the checkpoint with command results, `HEAD`, status, and any limitations. Render milestone 004 checked only if every completion-gate condition passes. Commit locally on `master` as authorized. Do not push. Covers R1–R9.

## Requirement Coverage Map

| Spec requirement | Checklist coverage |
|---|---|
| R1 — Validate table identifiers | A1 covers the required and unique `--table`, exact grammar, invalid forms, and local failure. |
| R2 — Build the hot query | B1 covers `remote(<table>_logs)`, selected columns, filters, order, limit, timeout setting, and format. |
| R3 — Add cold union | D1 covers one hot branch, one cold branch, `UNION ALL`, `_row_type = 1`, shared predicates, and one final clause set. |
| R4 — Bind query text | B1 and C1 cover typed placeholders, separate values, raw matching, control rejection, and time parameters. |
| R5 — Validate order and limit | A2 and B1 cover defaults, allowed values, boundary limits, and generated ordering. |
| R6 — Validate time windows | A2 and C1 cover duration syntax, explicit bounds, mutual exclusion, fixed-clock calculation, RFC3339Nano, and Unix milliseconds. |
| R7 — Derive server execution time | A2 and B1 cover timeout validation, default, ceiling to seconds, and generated setting. |
| R8 — Reject duplicate flags | A1/A2 cover every value-bearing flag, same/different repeated values, flag-specific errors, and repeatable `--cold`. |
| R9 — Keep milestone builder-only | A1–E1 constrain scope; V1 checks no request, route, or help behavior and verifies all tests and records. |

## Boundary Probes and Evidence

The following read-only probes establish current parser patterns. Their expected behavior is defined by the 004 spec. No query builder exists at this phase, so builder-specific probes are specified as tests for implementation rather than run against nonexistent code.

| Input or probe | Command and observed result | Spec-defined expected result |
|---|---|---|
| Current list-parser tests, including repeated pagination and values | `PATH=$HOME/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.9.linux-amd64/bin:$PATH GOTOOLCHAIN=local GOPROXY=off go test -count=1 -run 'TestParseSourcesListArgs|TestSourcesListQueryAndAuth|TestIncidentsList.*Duplicate|TestParseIncidentsListArgs' ./cmd/bs` → exit 0, `ok github.com/jrswab/betterstack-cli/cmd/bs 0.017s`. | Preserve existing repeatable `--page`, `--per-page`, and other repeatable flags outside logs search. Logs search alone rejects duplicate single-value flags. |
| Go duration grammar | `go doc time.ParseDuration` → units include `ns`, `us`/`µs`, `ms`, `s`, `m`, `h`; example includes compound `2h45m`; no day unit. | Accept positive Go durations including `30m`, `1h`, and `2h45m`; reject `7d`, zero, negatives, and malformed values. |
| `--limit` boundaries | Test inputs: `0`, `1`, `1000`, `1001`, `-1`, and `one`. The current source/incident pagination parser does not implement this search limit. | Accept only integers 1–1000 inclusive; default 100. Reject all listed values except 1 and 1000. |
| Duplicate search flags | Existing precedent: `cmd/bs/incidents.go:315-344` and `cmd/bs/incidents_test.go:1944-1970`. | Reject a repeated single-value flag and name it. Do not change existing repeatable pagination behavior. |
| Time-bound conversion | `go doc time.Time.UnixMilli` → converts to Unix milliseconds and does not depend on time location; sub-millisecond precision is discarded. | Parse RFC3339Nano, convert to Unix milliseconds, require `from < to`, use inclusive lower and exclusive upper bounds. |
| Query/time and table interpolation | Ticket 10 O3/O4 and live verification asset 05 establish `param_*` forwarding and typed bindings; table names cannot be parameters in these functions. | Bind query and time values; validate the table grammar before interpolation. |
| Cold query structure | Ticket 04 and live verification asset 05 establish `remote(..._logs)`, `s3Cluster(..._s3)`, `_row_type = 1`, and `UNION ALL`. | Keep hot reads as default; add exactly one filtered cold branch only when `--cold` is present. |

## Parallelism and Commit Boundaries

A1 through D1 depend on the same parser and builder contract, so execute them in order. E1 follows behavior changes. V1 is the final implementation, review, records, and completion gate. The milestone source remains an untracked baseline path and must never be staged. Stage only paths authorized for milestone 004, never `git add -A` or `git add .`.
