# Milestone 005 Implementation Checklist — S4 SQL search executor

- Spec: `docs/plans/008_telemetry-logs-search_005_spec.md`
- Checkpoint: `docs/plans/008_telemetry-logs-search_005_checkpoint.md`
- Milestone source: `docs/plans/008_telemetry-logs-search_milestones.md`, entry `005`
- Scope: SQL URL parsing, SQL credentials, one SQL POST, timeout/redirect/retry behavior, and SQL response handling. Do not activate logs search dispatch or add help.
- State: the checkpoint owns task state. The checklist boxes render from its task states.
- Preserve the five baseline untracked paths. Never stage, modify, or delete `.agents/`, `.wayfinder/`, `AGENTS.md`, `docs/plans/008_telemetry-logs-search_milestones.md`, or `verify-incident-id.sh`.
- Use fake credentials only and `httptest` loopback servers only. Never make a live request or build a `bs` binary into the repository root.

## Section 1: Context Summary

Milestone 005 activates the SQL executor behind the existing endpoint injection seam while leaving command dispatch and help to milestone 006. The executor must use the regional SQL endpoint forms approved by ticket 10, separate SQL Basic-auth credentials, builder-produced SQL and query parameters, and one timed POST with `plain/text`. It must block redirects and retries, preserve every 2xx body byte on stdout, and apply `redactSecrets` only to non-2xx SQL bodies on stderr. D5 requires an explicit test proving that `runWithEndpoints` routes SQL through its `telemetrySQLURL` argument. Tests use fake strings and guarded `httptest` servers only.

**Planned code and test surfaces:** `cmd/bs/logs.go` for SQL URL parsing and execution helpers, `cmd/bs/main.go:runWithEndpoints` only if a minimal executor-facing seam connection is required, and `cmd/bs/logs_test.go` plus narrowly scoped `cmd/bs/main_test.go` tests. These paths and function names are planning targets. Keep the existing `runWithAPI` signature. Do not add anchors to helpers. Add a production boundary anchor only if the implementation introduces a real boundary, then update `TAGS.md` in the records task.

## Section 2: Implementation Checklist

Tasks are sequential because they converge on one SQL request and response contract. Each behavior task adds or extends tests using fake credentials and `httptest`, then runs its focused test before and after implementation. No test may use a custom transport that bypasses the loopback guard.

### Slice A — Parse the SQL endpoint and validate credentials

- [x] **A1 — Specify and test the accepted SQL URL forms.** Add test-first table cases in `cmd/bs/logs_test.go` for the SQL URL parser in `cmd/bs/logs.go`. Accept `host`, `host:443`, and `https://host[:port]`; normalize host-only inputs to HTTPS; allow only no path or `/`; reject other schemes, malformed or empty hosts, invalid ports, non-root paths, query, fragment, userinfo, and unsupported forms. Ensure errors do not echo credential values. Cover empty input, IPv4/IPv6 host syntax where valid under approved forms, default and explicit ports, and long malformed strings. Run the focused parser test before and after implementation. Covers R1.
- [x] **A2 — Validate SQL configuration and separate Basic-auth credentials.** In `cmd/bs/logs.go`, read `BETTERSTACK_TELEMETRY_SQL_URL`, `BETTERSTACK_TELEMETRY_SQL_USERNAME`, and `BETTERSTACK_TELEMETRY_SQL_PASSWORD` for SQL execution only. Require all three values and reject empty username/password. Do not fall back to API tokens. Add tests in `cmd/bs/logs_test.go` for each unset/empty variable, dummy Uptime and Telemetry API tokens without SQL credentials, and a valid pair. Assert no request for local failures, safe diagnostics, and no bearer-header fallback. Validate that the configured request uses Basic auth derived from the two SQL values. Run focused tests before and after implementation. Covers R2.

### Slice B — Send the builder query through the injected SQL endpoint

- [x] **B1 — Implement one bounded SQL POST and prove endpoint routing.** Add a test-first `httptest` case that calls `runWithEndpoints` with distinct Uptime, REST, and SQL loopback servers. The SQL request must reach only the server passed as `telemetrySQLURL`, proving D5. Connect `buildLogsSearchQuery` output to the executor without exposing the command route. Assert one `POST`, SQL body bytes, `Content-Type: plain/text`, encoded builder `param_*` query values, Basic auth, no Bearer auth, and a request context with the requested timeout. In `cmd/bs/logs.go`, send one POST with that context and disable redirects on a copy of the supplied `http.Client`. Preserve `runWithAPI`'s signature. Run focused tests before and after implementation. Covers R3, R4, and R8.
- [x] **B2 — Reject redirects and prove there are no retries.** Add loopback `httptest` tests with separate redirect source and target servers. Assert that the redirect response is handled once, the target receives zero requests, and no retry occurs after redirect or a server error. Add a deterministic transport failure case that uses the existing guarded loopback test facilities and does not bypass the network guard. Assert exactly one request attempt per invocation. Run focused tests before and after implementation. Covers R4.

### Slice C — Preserve success and redact SQL errors

- [ ] **C1 — Pass all 2xx response bodies unchanged.** Add table-driven loopback tests for representative 2xx statuses, including 200 and 204, with JSON-looking, non-JSON, NUL-containing, newline-terminated, and empty bodies. Implement the success branch in the SQL executor to copy body bytes unchanged to stdout, write nothing to stderr, and return 0. Verify that fake secrets in a 2xx body remain unchanged and that the executor does not inspect the body. Run focused tests before and after implementation. Covers R5 and R6.
- [ ] **C2 — Redact non-2xx SQL response bodies only.** Add loopback tests covering 3xx and representative 4xx/5xx responses. Include fake Telemetry API token, SQL username, SQL password, and derived Basic-auth text in error bodies. Apply `redactSecrets` to the body only in the non-2xx SQL branch, write its bytes to stderr, write nothing to stdout, and return 1. Assert unrelated bytes and newlines remain, SQL text is not masked, and 2xx SQL/source/Uptime response paths are not redacted. Run focused tests before and after implementation. Covers R6.
- [ ] **C3 — Handle timeout, transport, and body-read failures safely.** Add guarded loopback tests for a request that exceeds its timeout and a response that fails while reading its body. Use a closed loopback listener or an equivalent deterministic error path for transport failure. Ensure diagnostics omit all fake credentials, stdout remains empty, the exit code is 1, the timeout cancels the request, and no case retries. Implement only the SQL executor's local failure handling. Run focused tests before and after implementation. Covers R7.

### Slice D — Document runtime configuration and update repository records

- [ ] **D1 — Document the SQL environment variables and record the implemented contract.** Update `README.md` with the three SQL environment variables and their roles because milestone 005 introduces runtime use. Read `.agents/skills/changelog/SKILL.md` and `.agents/skills/mem/SKILL.md`. Add `changelog.d/008-telemetry-logs-search-005.md` with a factual SQL-executor entry. Update `.mem/telemetry/logs.md` to describe current SQL URL parsing, Basic auth, request and response behavior, and a code example; update `.mem/telemetry/sources.md`, `.mem/summary.md`, `.mem/practices.md`, `.mem/terminology.md`, and `.mem/mem-map.md` only where current behavior or names require it. Keep each touched memory file at or below 250 lines, use present tense, and do not record the local Go toolchain workaround in shared records. Update `TAGS.md` for every production boundary anchor added, removed, or changed. Do not add a `LOG.md` entry unless implementation introduces non-obvious rationale not captured by the spec. Run `make fragments-check` and `make docs-check` if available; otherwise record their actual unavailable results and run direct checks. Covers the records for R1–R8.

### Slice E — Final verification and handoff for review

- [ ] **E1 — Run milestone verification and map each requirement to evidence.** Run `GOFLAGS=-o=/dev/null go build ./...`, `go vet ./...`, `go test -count=1 ./...`, and `gofmt -l cmd/bs/*.go` with the approved local toolchain environment. Run focused SQL executor tests and direct checks for URL forms, credential separation, one request, SQL endpoint injection, content type, timeout, no redirects/retries, exact success bytes, non-2xx redaction, no command route/help, README env documentation, memory links/line limits, changelog shape, and tag consistency. Use `/tmp/factory-005/` for scratch check scripts and quoted heredocs. Record the exact Go pre-flight result and any vulnerability scan limitation. Produce an R1–R8 requirement-to-evidence table. Inspect all changes and confirm the five protected baseline paths are unchanged. Do not check milestone 005 or edit the milestone source. The independent review gate and Factory completion gate occur after this planning/execution task sequence. Covers R1–R8.

## Requirement Coverage Map

| Spec requirement | Checklist coverage |
|---|---|
| R1 — Parse only approved SQL URL forms | A1 covers accepted forms, path/authority rules, malformed inputs, and local rejection. |
| R2 — Require separate SQL credentials | A2 covers required environment values, no API-token fallback, Basic auth, and local no-request failures. |
| R3 — Send one POST with the approved request shape and timeout | B1 covers method, body, content type, query parameters, auth, timeout, and single-call contract. |
| R4 — Do not follow redirects or retry | B1 disables redirects; B2 proves no redirect follow or retry. |
| R5 — Pass successful response bytes unchanged | C1 covers 2xx status and exact body bytes. |
| R6 — Redact only non-2xx SQL response bodies | C1 and C2 cover unredacted success and redacted failures across channels. |
| R7 — Handle local request failures safely | C3 covers request construction/config validation, timeout, transport, and body-read failures. |
| R8 — Keep milestone 005 separate from command activation | B1 tests the injected SQL endpoint; E1 checks no logs route/help. |

## Boundary Probes and Evidence

No production SQL executor exists yet, so request behavior probes are acceptance tests for implementation, not runtime commands. Existing sources establish the repository's request test pattern and safety constraints. Existing builder tests establish that query construction returns SQL, `url.Values`, and a positive timeout. The following inputs must appear in the tests or direct verification:

| Input or probe | Existing evidence or planned test | Spec-defined expected result |
|---|---|---|
| `host`, `host:443`, `https://host`, `https://host:8443`, and root path | Ticket 10 O5; add table-driven A1 cases | Normalize accepted forms to HTTPS and preserve approved host/port. |
| Empty URL, `http://host`, another scheme, `/path`, `?x=y`, `#frag`, `user:pass@host`, malformed port | Ticket 10 O5; add A1 rejection cases | Fail locally with no request and no credential disclosure. |
| Empty/unset URL, username, or password | Approved variables in tickets 02 and 10; add A2 environment table | Fail locally with exit 1, no request, and safe variable-specific diagnostics. |
| SQL username/password plus distinct dummy API tokens | Ticket 02 separation; add A2 auth test | Use only HTTP Basic auth from SQL values; never use Bearer auth as fallback. |
| Builder SQL and `param_query`, `param_from`, `param_to` values | Current `buildLogsSearchQuery` and `cmd/bs/logs_test.go` | Send SQL in the POST body and encoded values in the SQL URL query. |
| Uptime, REST, and SQL endpoint inputs are three distinct loopback URLs | D5 and current `runWithEndpoints` signature | The SQL request reaches only the `telemetrySQLURL` server. |
| 200, 204, and other 2xx response bodies, including empty and raw bytes | Ticket 11; add C1 cases | Copy exactly to stdout without redaction or newline changes. |
| 3xx redirect, 4xx, and 5xx response with fake secrets | Ticket 11 and D6; add B2/C2 cases | No redirect or retry; redact only non-2xx SQL body to stderr and exit 1. |
| Timeout and read/transport failure with fake credentials present | Current `doMonitorsRequest` pattern; add C3 loopback tests | Exit 1, no stdout, no secret in local diagnostics, no retry. |

## Changed Signatures and Caller Audit

No existing `runWithAPI` signature changes. B1 extends `runWithEndpoints` with an optional prepared `logsSQLRequest` input. Existing call expressions remain valid. Before implementation, `rg -n 'runWithEndpoints|buildLogsSearchQuery|redactSecrets' cmd/bs --glob '*.go'` found the production caller `runWithAPI`, all existing callers in `main_test.go` and `sources_test.go`, and new B1 callers in `logs_test.go`. The existing caller list remains unchanged. The optional prepared input exercises execution without interpreting a logs command vector. No builder or redaction signature changes. Current caller search evidence:

- `runWithEndpoints`: production caller `cmd/bs/main.go:runWithAPI`; test callers `cmd/bs/main_test.go`, `cmd/bs/sources_test.go`, and `cmd/bs/logs_test.go` should any routing test be added there. The full `rg -n 'runWithEndpoints' cmd/bs --glob '*.go'` output is recorded in the milestone checkpoint.
- `buildLogsSearchQuery`: test callers in `cmd/bs/logs_test.go`. The executor should consume its current SQL, `url.Values`, and error result without changing this signature.
- `redactSecrets`: test callers in `cmd/bs/telemetry_test.go`; the SQL executor becomes its first production caller. Do not change its signature or broaden its use to other response paths.

## Parallelism and Commit Boundaries

A1 through C3 are sequential parts of one request slice. D1 follows runtime behavior because the memory must describe current behavior. E1 follows all code and record tasks. No checklist task is parallel. Implementation commits must stage explicit milestone paths only, never `git add -A` or `git add .`. Preserve all protected baseline paths and do not push.
