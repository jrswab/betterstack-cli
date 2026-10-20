# Milestone 002 Implementation Checklist — S1 Telemetry endpoint seam and secret redaction

- Spec: `docs/plans/008_telemetry-logs-search_002_spec.md`
- Milestone source: `docs/plans/008_telemetry-logs-search_milestones.md`, entry `002`
- Scope: milestone 002 only; do not implement source requests or SQL execution
- Requirement coverage: R1 → A1 and V1; R2 → A1 and V1; R3 → A2 and V1; R4–R5 → A1, A2, and V1; R6 → A3; R7 → V1
- State: the checkpoint owns task state. Task boxes below are rendered from its passed task states.
- Factory Phase 3 override: execute A1–A3, commit implementation and records together, then commit checkpoint/render separately. Stop before V1. Do not modify the milestone source.

## Section 1: Context Summary

The standard-library Go CLI currently routes Uptime commands through `cmd/bs/main.go:runWithAPI` and sends requests through `cmd/bs/monitors.go:doMonitorsRequest`. S1 adds a test injection seam for distinct Uptime, Telemetry REST, and Telemetry SQL endpoints while preserving the exact `runWithAPI` signature and current Uptime behavior. S1 also adds reusable literal secret redaction for Telemetry diagnostics, but does not apply it to any response path. Preserve the five baseline untracked paths without modification, staging, or deletion. Use only loopback mock servers and dummy test values. Do not send live requests or use real credentials.

**Implementation assumptions:** The spec does not prescribe exact signatures. Use `runWithEndpoints(args, stdout, stderr, uptimeBaseURL, telemetryRESTBaseURL, telemetrySQLURL string, client *http.Client) int` and `redactSecrets(text, telemetryToken, sqlUsername, sqlPassword string) string`. These signatures are planning assumptions, not public interfaces. S1 cannot verify Telemetry REST or SQL request routing because those command paths belong to later milestones. Tests can verify that `runWithEndpoints` accepts distinct endpoint inputs and that `runWithAPI` preserves existing Uptime routing. S2 and S4 verify Telemetry routing at real request boundaries.

## Section 2: Implementation Checklist

The checkpoint records state for A1–A3 and V1. A1 and A2 can proceed in parallel. A3 waits for both behavior slices. V1 waits for all implementation and record work, then gates the S1 implementation commit. Each behavior task follows test-first order and runs the required full Go commands after implementation. Write direct checks as files under `/tmp/factory-002/` using quoted shell heredocs, then execute those scripts. Do not use inline Python. Bundle all code, tests, and applicable repository records in one implementation commit so every commit satisfies `AGENTS.md` Before Committing.

### Slice A — Compatibility and endpoint seam

- [x] A1: In `cmd/bs/main_test.go`, add test-first coverage for `runWithEndpoints` accepting three distinct endpoint values, the unchanged `runWithAPI(args, stdout, stderr, baseURL, client)` signature, loopback Uptime routing, and help or invalid usage without requests. Then implement `cmd/bs/main.go:runWithEndpoints` with the assumed signature `runWithEndpoints(args, stdout, stderr, uptimeBaseURL, telemetryRESTBaseURL, telemetrySQLURL string, client *http.Client) int`, and make `runWithAPI` delegate without changing its signature or existing caller behavior. Keep production Uptime and Telemetry REST hosts fixed. Keep `BETTERSTACK_TELEMETRY_SQL_URL` as the only configurable service host. Do not add source or SQL request behavior, SQL URL parsing, flags, environment overrides, or fallback to the Uptime token. Before implementation, run `go test -run 'TestRunWithEndpoints|TestRunWithAPI' ./cmd/bs` and record the expected failure. After implementation, run `GOFLAGS=-o=/dev/null go build ./...`, `go vet ./...`, `go test -count=1 ./...`, and `gofmt -l cmd/bs/*.go`. Create `/tmp/factory-002/check-endpoints.sh` with a quoted heredoc, then run `bash /tmp/factory-002/check-endpoints.sh`. Map to R1, R2, R4, and R5.

### Slice B — Reusable Telemetry secret redaction

- [x] A2: Add test-first direct tests in `cmd/bs/telemetry_test.go` for `redactSecrets`, then implement `cmd/bs/telemetry.go:redactSecrets` with the assumed signature `redactSecrets(text, telemetryToken, sqlUsername, sqlPassword string) string`. Cover the Telemetry API token, SQL username, SQL password, base64 of `username:password`, repeated occurrences, longer-before-shorter overlap, case-sensitive matching, empty individual credentials, both SQL credentials empty, and unrelated text. Use direct dummy values and do not read process credentials. Replace literal occurrences globally with `[REDACTED]`, ignore empty secrets, and include the derived Basic-auth secret when either SQL credential is non-empty. Do not call the helper from `doMonitorsRequest`, source output, successful SQL output, or any current error path. Before implementation, run `go test -run 'TestRedactSecrets' ./cmd/bs` and record the expected failure. After implementation, run `GOFLAGS=-o=/dev/null go build ./...`, `go vet ./...`, `go test -count=1 ./...`, and `gofmt -l cmd/bs/*.go`. Create `/tmp/factory-002/check-redaction.sh` with a quoted heredoc, then run `bash /tmp/factory-002/check-redaction.sh`. Map to R3 and R4.

### Slice C — Repository records with the implementation commit

- [x] A3: After A1 and A2, create `changelog.d/008-telemetry-logs-search-002.md` through the changelog skill. Use one factual fragment about delivered S1 work. Update `TAGS.md` for every added, removed, or changed production anchor. Update the relevant existing `.mem/` topic files and `.mem/terminology.md` through the mem skill if the seam or redaction behavior changes documented structure or behavior. Update `.mem/mem-map.md` if memory files are added or moved. Update `README.md` only if S1 activates or changes an environment variable, endpoint, Make target, or setup instruction. Do not document future commands. Keep `LOG.md` and `log.d/` consistent with repository conventions, and add provenance only if implementation introduces non-obvious rationale. Run `make fragments-check` if available and report it unavailable if it is not. Run `GOFLAGS=-o=/dev/null go build ./...`, `go vet ./...`, `go test -count=1 ./...`, and `gofmt -l cmd/bs/*.go`. Create `/tmp/factory-002/check-records.sh` with a quoted heredoc, then run `bash /tmp/factory-002/check-records.sh`. Do not use inline Python. Stage records with implementation and tests in the same commit. Map to R6.

### Slice D — Final acceptance and commit gate

- [ ] V1: After A1–A3, run `GOFLAGS=-o=/dev/null go build ./...`, `go vet ./...`, `go test -count=1 ./...`, and `gofmt -l cmd/bs/*.go` (expect no paths). Run `/tmp/factory-002/check-endpoints.sh`, `/tmp/factory-002/check-redaction.sh`, and `/tmp/factory-002/check-records.sh`. Confirm R1–R7 against the spec. Confirm existing Uptime raw-response tests pass unchanged, `TestMain` and the loopback-only network guard remain intact, no current response path calls `redactSecrets`, and no live request or real credential was used. Run `git diff --check`, stage only approved S1 code, tests, and repository-record paths explicitly, inspect `git diff --cached --name-only`, and run `git diff --cached --check`. Record exact command outputs and unavailable checks in the checkpoint. Commit the complete S1 change only after all checks pass. Do not edit or render the milestone source checkbox. Map to R1–R7.

## Requirement Coverage Map

| Spec requirement | Checklist coverage |
|---|---|
| R1 — Preserve `runWithAPI` and existing routing | A1 tests compatibility first, implements delegation without changing the signature, and verifies Uptime routing; V1 repeats the acceptance audit. |
| R2 — Provide three distinct endpoint inputs | A1 tests the three-input seam and existing Uptime injection; V1 records that REST and SQL request routing remain downstream S2/S4 verification. |
| R3 — Redact approved Telemetry secrets | A2 adds direct test-first coverage for all secret forms and edge cases, then implements the reusable helper; V1 runs focused and full checks. |
| R4 — Keep redaction unapplied to response paths | A1 and A2 constrain scope; V1 checks current raw-response tests and confirms no response path calls redaction. |
| R5 — Preserve endpoint and credential policy | A1 retains fixed hosts and separate credentials; A1 and V1 use loopback servers and dummy values only. |
| R6 — Add repository records before committing | A3 bundles the changelog, applicable anchor index, memory, conditional README update, and provenance audit with code and tests. |
| R7 — Verify without external access | A1–A3 and V1 name the required Go commands and direct scripts; V1 checks formatting, whitespace, staged paths, and network protections. |

## Parallelism and commit boundary

A1 and A2 can proceed in parallel because they cover separate contracts. A3 waits for both behavior slices. V1 waits for A1–A3 and is the pre-commit acceptance gate. The implementation and required records land together in one commit. Any unresolved requirement must be recorded as `BLOCKED` in the checkpoint. Do not pause for clarification.
