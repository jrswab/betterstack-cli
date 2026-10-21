# Milestone 006 Specification — S5 route and document `bs logs search`

- Milestone source: `docs/plans/008_telemetry-logs-search_milestones.md`, entry `006` (S5)
- Scope: milestone 006 only. Activate `bs logs search` through the existing command routing boundary, use the completed query builder and SQL request executor, and document the user-facing command.
- Acceptance in one line: `logs search` builds a validated query and executes it through the injected SQL endpoint; root, resource, and command help document the search flags and table form; command help explains missing-hot-table `--cold` behavior and option-shaped query data; source token warnings remain present.

## Section 1: Context & Constraints

### Repository and milestone boundary

The CLI is a standard-library Go program in `cmd/bs`, package `main`. `runWithAPI` in `cmd/bs/main.go` preserves the Uptime injection interface and delegates to `runWithEndpoints`, which receives distinct Uptime, Telemetry REST, and Telemetry SQL endpoint inputs. `runWithEndpoints` currently recognizes the source and Uptime routes. Before regular routing, a variadic test-only prepared-SQL branch accepts an empty argv and a `logsSQLRequest`. Milestone 006 must replace this branch with the real `logs search` route while preserving the SQL endpoint proof through `telemetrySQLURL`.

`buildLogsSearchQuery` in `cmd/bs/logs.go` validates the search flags and returns SQL, `url.Values`, and an error. It uses the full table identifier `t<team_id>_<table_name>`, where the mapping is an approved inference from ticket 09. `doLogsSQLRequest` performs the completed timed SQL POST and owns SQL credentials and response handling. The existing builder and executor tests use fake strings and in-memory `RoundTripper` or loopback `httptest` servers. New route tests must use the same safe approaches. No test may make a live request or use real credentials.

`helpFor` in `cmd/bs/main.go` recognizes exact help argument vectors and returns constants from `cmd/bs/help.go`. `usageError`, `resourceUsageError`, and `knownAction` provide diagnostics for missing resources/actions and unknown actions. Established behavior is: bare `bs` reports “no command given”; a known resource without an action reports “missing action”; an unknown action reports “unknown action” and points to resource help. Apply these existing diagnostics to `logs` rather than introducing a new bare-command product behavior.

The existing `sourceSecretWarning` is appended to both source command help pages. `TestRunWithEndpointsSourcesHelp` already asserts that both help pages contain the generic warning, and the current focused test passes. Preserve this as a regression requirement. The warning must not claim what the source token permits.

### Approved search and help contract

Tickets 01 and 09 define `bs logs search`, search flags rather than raw SQL, the full table name form `t<team_id>_<table_name>`, hot storage by default, and opt-in cold storage. Ticket 09 says the source's `table_name` omits the team prefix, while the SQL identifier includes it. Help must describe the full identifier and state that `bs sources get` shows both parts. It must not imply that a source ID or source lookup is accepted as `--table`.

The completed builder contract from milestone 004 applies unchanged:

- `--table <table_name>` is required and matches `^t[0-9]+_[a-z0-9_]+$`.
- `--since <duration>` defaults to `1h`; explicit `--from` and optional `--to` use RFC3339Nano bounds. `--to` requires `--from`. `--since` cannot combine with either explicit bound.
- `--query <text>` is optional and matches `raw` case-insensitively as a substring. Search text and times remain typed query parameters.
- `--order <asc|desc>` defaults to `desc`. `--limit <1..1000>` defaults to `100`.
- `--timeout <duration>` defaults to 30 seconds. `--cold` is an opt-in switch.
- All value-bearing flags reject duplicates. Flags use space-separated values. `--cold` may repeat without adding branches.

A positional value immediately after `--query` is data even when it looks like an option. In particular, `bs logs search --table t1_example --query --cold` searches for the literal text `--cold`; it does not enable cold storage. This is intentional builder behavior, confirmed by milestone 004 review finding F3 and the parser's query-specific value handling. Help must explain this ambiguity. Users who want cold storage with an option-shaped query must place `--cold` before `--query`, for example `--cold --query --cold`.

Ticket 10 O7 defines the missing-hot-table case: the SQL API returns HTTP 500 `CLUSTER_DOESNT_EXIST` for a source without a hot table. Preserve the executor's raw non-2xx diagnostic handling, and add a command-help note that this means the source has no hot table and suggests rerunning with `--cold`. Do not change response policy or claim that `--cold` always succeeds.

Root help must list the `logs` resource. Resource help must list the `search` command. Command help must give usage, every supported flag, defaults and important validation, the full table identifier format, the missing-hot-table note, the option-shaped query-value note, and useful examples. Exact help forms must return before credentials or HTTP requests, matching existing help behavior.

### Repository records and settled carry-forward decisions

The repository requires a changelog fragment, matching `TAGS.md` entries for production anchors, current `.mem/` records for behavior or routing changes, and README documentation for a newly available command. The existing `.mem/telemetry/logs.md` currently describes the builder and executor as lacking a command route. The routing diagrams and prose in `.mem/summary.md`, `.mem/uptime/summary.md`, `.mem/uptime/commands.md`, `.mem/uptime/testing.md`, `.mem/practices.md`, and `.mem/terminology.md` also describe prepared SQL execution through an optional test-only seam. Update only records that become inaccurate when the real route replaces that seam. Keep each touched memory file at or below 250 lines, use present tense, and add no local toolchain workaround to shared records. No new memory topic is needed.

D1–D9 from `/tmp/betterstack-cli-factory-008-handoff.md` remain authoritative. D1 assigns the full lane. D4 fixes the changelog path to `changelog.d/008-telemetry-logs-search-006.md`. D5 requires proof that the SQL route uses the `telemetrySQLURL` argument. D6 confines secret redaction to non-2xx SQL response bodies. D8 requires any direct-check script to live under `/tmp/factory-006/` and use a quoted heredoc. D9 requires the separate review gate in Phase 3. The user requires explicit-path staging, no pushes, no live requests, no real credentials, and no modification or staging of the five baseline untracked paths.

Carry-forward decisions:

- **004 F3:** preserve `--query --cold` as query data and explain it in help. Add a regression test for both the data interpretation and the alternate flag ordering.
- **005 F5 and D5:** remove the variadic `sqlRequest` branch from `runWithEndpoints`. Route argv through the real `logs search` command and prove that the resulting SQL request reaches only the injected `telemetrySQLURL` server. This carries the D5 proof without a test-only argv bypass. Preserve the public `runWithAPI` signature and the fixed production SQL configuration behavior.
- **005 F8:** update the routing memory diagram and related present-state prose because this milestone changes routing. Do not expand into unrelated memory cleanup.
- **README:** replace “The command route is not available yet.” with `logs search` usage, flags, and behavior documentation. Retain the existing SQL environment-variable and executor contract documentation.
- **Source-token warning:** current source list/get help and `TestRunWithEndpointsSourcesHelp` already satisfy the requirement. Keep or extend that regression test; no source warning wording change is required.

No production boundary name or new domain term is required. Reuse the existing `telemetry:command-handler` anchor role for the new `logs search` handler, if it is introduced as a distinct handler, and update `TAGS.md` to match the code. Keep helpers unanchored. Keep `telemetry:request-executor` on `doLogsSQLRequest` and `uptime:router` on the existing routing boundary.

## Section 2: Requirements

### R1 — Route search through the production command path

`bs logs search` must be recognized by the command routing boundary. The route must pass the search arguments to `buildLogsSearchQuery`, then pass the resulting SQL and parameters plus the parsed timeout to `doLogsSQLRequest`. The executor must receive the SQL endpoint selected by the `telemetrySQLURL` argument to `runWithEndpoints`. Production invocation through `runWithAPI` must continue to use the configured SQL URL. Preserve `runWithAPI`'s signature and all existing Uptime and source routes.

Remove the variadic test-only prepared-SQL request path from `runWithEndpoints`; it must no longer accept `logsSQLRequest` as a side channel. The completed builder and executor retain their behavior and signatures unless repository-wide caller search proves a change necessary.

**Acceptance:** a route-level `httptest` uses distinct Uptime, REST, and SQL endpoints and fake SQL credentials. It invokes `logs search` with valid arguments, observes exactly one POST at the SQL endpoint, and observes zero requests at Uptime and REST endpoints. It verifies the body and encoded query parameters match `buildLogsSearchQuery`, and verifies the executor's normal response behavior. Tests no longer call `runWithEndpoints` with an extra `logsSQLRequest` argument. Existing source and Uptime route tests pass.

### R2 — Preserve local validation and SQL executor behavior

Invalid search arguments must fail locally through the completed builder, before any SQL request. Valid search arguments must use the builder's SQL, parameters, and timeout without changing query construction, credentials, redirects, retries, response bytes, or redaction behavior. A 2xx SQL body stays unchanged on stdout. A non-2xx SQL body follows the existing redaction contract on stderr and returns exit status 1.

**Acceptance:** route tests cover at least one malformed/missing required `--table` case and prove that it sends no request. Existing focused builder and executor tests remain green. The route-level success and failure tests use only fake credential strings and a recording in-memory transport or loopback `httptest` server.

### R3 — Provide root, resource, and command help

Exact help forms `bs --help`, `bs logs --help`, and `bs logs search --help` must return human-readable usage with exit status 0, empty stderr, no credential requirement, and no HTTP request. Root help lists `logs`; resource help lists `search`; command help describes every search flag, including defaults, bounds, time behavior, duplicate rejection, and space-separated values.

Command help must state the full table identifier format `t<team_id>_<table_name>` and that `bs sources get` shows both parts. It must state that `--query` consumes the next argument as text, including option-shaped text such as `--cold`, and show how to enable cold storage with an option-shaped query. It must state that HTTP 500 `CLUSTER_DOESNT_EXIST` means the source has no hot table and suggests `--cold`.

**Acceptance:** help tests assert all three levels and the required flag/default/table/error/query guidance. A request-counting test with credentials absent and another with fake credentials confirms help makes no request. Tests assert exact help vectors; extra arguments do not become help.

### R4 — Preserve source-token warning help

Both `bs sources list --help` and `bs sources get --help` must continue to warn that output includes each source's `token` field and that it is a secret credential. The wording must not claim token permissions or function. Preserve source help behavior without changing source runtime behavior.

**Acceptance:** retain or extend `TestRunWithEndpointsSourcesHelp` to assert the warning in both command pages and zero requests. The existing focused regression currently passes against both pages.

### R5 — Apply established usage behavior to `logs`

`bs logs` with no action must use the existing known-resource “missing action” diagnostic and refer to `bs logs --help`. An unknown action must use the existing “unknown action” diagnostic and refer to the resource help. Invalid `logs search` argument vectors must identify the search command help, following the existing source handler and resource usage conventions. Do not add special behavior for bare `bs`; it continues to report “no command given”.

**Acceptance:** routing/usage tests cover `logs`, an unknown action, malformed `logs search` flags, bare `bs`, and exact help vectors. They assert exit status, output channel, diagnostic class, and that no invalid vector reaches the SQL executor.

### R6 — Document the available command and preserve existing configuration guidance

`README.md` must no longer say the command route is unavailable. Document `bs logs search`, its supported flags and defaults, the full table-name format and source lookup context, query option-shaped data behavior, and the missing-hot-table `--cold` note. Keep the current SQL environment-variable names, role descriptions, Basic-auth separation, timeout, redirect/retry, response, and redaction contracts accurate.

**Acceptance:** direct record validation confirms the unavailable-route sentence is absent, the command and all flags appear, the table format and both help notes appear, and all three SQL environment variables and existing executor behavior remain documented.

### R7 — Keep current repository records consistent with active routing

Update `.mem/` to describe `logs search` as an active route that builds a query and calls the SQL request executor. Update routing diagrams and statements that currently say the SQL input has no route or describe the optional prepared request as the current route. Keep the records in present tense and preserve the established loopback-only testing contract. Update `TAGS.md` for the new command-handler anchor and any changed or removed anchor. Add `changelog.d/008-telemetry-logs-search-006.md` with one factual user-facing entry. Do not modify `LOG.md` or `log.d/` unless implementation introduces a new non-obvious rationale not already settled here.

**Acceptance:** focused direct checks verify that `.mem/` links resolve, changed memory files stay within 250 lines, current routing documentation agrees with `cmd/bs/main.go`, `TAGS.md` exactly indexes production anchors, and the changelog fragment follows repository shape. No change to `LOG.md` or `log.d/` is needed for decisions already captured in this specification.

### R8 — Preserve test safety and scope

All tests use fake credential strings and either an in-memory `RoundTripper` that never dials or a guarded loopback `httptest` server. Do not make a live request, use real credentials, add dependencies, alter source/Uptime request contracts, or change builder/executor behavior outside what is necessary to route their existing interfaces. Do not stage or modify the protected baseline paths.

**Acceptance:** test and diff inspection confirms the safety boundaries, `runWithAPI` signature remains unchanged, no dependency changes, and existing command tests pass.

### Requirement sources

- R1–R2 and R8: milestone 006 acceptance in [milestones](008_telemetry-logs-search_milestones.md), current `cmd/bs/main.go:runWithAPI,runWithEndpoints`, `cmd/bs/logs.go:buildLogsSearchQuery,doLogsSQLRequest`, and D5–D6 in `/tmp/betterstack-cli-factory-008-handoff.md`.
- R3–R5: current `cmd/bs/help.go` and `cmd/bs/main.go:helpFor,usageError,resourceUsageError,knownAction`; source warning precedent in `cmd/bs/help.go:sourceSecretWarning` and `cmd/bs/main_test.go:TestRunWithEndpointsSourcesHelp`.
- R3 missing-hot-table note: ticket 10 O7. Full table form and source-token warning: ticket 09.
- R3 option-shaped query behavior: `cmd/bs/logs.go:parseLogsSearchArgs`, `.mem/telemetry/logs.md`, and 004 review finding F3 in `docs/plans/008_telemetry-logs-search_004_checkpoint.md`.
- R6–R7: README.md, `.mem/mem-map.md` and its linked routing records, `TAGS.md`, repository `AGENTS.md`, and repo-local changelog/mem skills.
- R8: `.mem/uptime/testing.md` and the existing test safety harness in `cmd/bs/safety_test.go`.
- Decisions: [ticket 01](../../.wayfinder/telemetry/tickets/01-charting-decisions-destination-scope-and-command-surface.md), [ticket 09](../../.wayfinder/telemetry/tickets/09-grilling-table-name-format-and-source-token-output.md), and [ticket 10](../../.wayfinder/telemetry/tickets/10-technical-decisions-from-live-verification.md).

## Ordering and parallel work

R1, R2, and R5 form the route slice: replace the test-only seam with argv dispatch, connect builder output to the executor, and establish local usage failure behavior. R3 and R4 add exact help and protect existing source warning coverage. R6 and R7 update user and project records after the routed behavior is fixed. R8 governs every test and task. Implement sequentially because route dispatch, help, usage, and current-state records describe one new command surface. The source warning assertion may be checked in parallel with the route implementation, but no implementation task depends on changing source help.

## Unresolved questions

None. The milestone, closed tickets, current repository patterns, and carry-forward decisions define observable behavior. Existing usage diagnostics resolve `bs logs` with no subcommand. The source-token warning already exists and needs regression coverage, not a product wording decision.
