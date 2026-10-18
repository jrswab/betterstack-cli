# Milestone 007 Specification — First-release acceptance verification

Milestone source: `docs/plans/001_betterstack-cli-first-release_milestones.md`, entry 007 (unchecked, the last unchecked entry). This spec covers milestone 007 only. Milestone 007 is a verification milestone: it changes no product behavior, adds no feature, and executes no release action. It runs the approved acceptance sweep over the complete first-release surface, runs the pre-publication local install smoke test from a clean checkout, and durably records the deferred, separately authorized release actions.

## Section 1: Context & Constraints

### Codebase state (grounded at HEAD `3b41a27`)

- Milestones 001–006 are complete (source entries `- [x] 001`–`- [x] 006`; the 006 completion gate passed at commit `70d9502`, and the 007 checkpoint was committed at `3b41a27`). All five first-release commands are implemented and routed in `cmd/bs/main.go` (`runWithAPI`): `monitors get`, `monitors list`, `incidents get`, `incidents timeline`, `incidents list`.
- The established test seams are: `runWithAPI(args, stdout, stderr, baseURL, client)` (routing-level injection), `invokeAPI` (test helper over the seam), `newHTTPClient()` (production client shape, wrapped by the guard in tests), and `testHTTPClient()` (guarded test client). The `safety_test.go` `TestMain` credential isolation (unsets `BETTERSTACK_UPTIME_API_TOKEN` and `BETTERSTACK_API_TOKEN`) and the loopback-only network guard (rejects every request whose host is not a literal loopback IP before the dialer) remain enabled and must not be weakened or bypassed.
- The existing approved test suite already covers, per command: argument parsing, query translation, bearer-token sending, raw success bodies, non-2xx/3xx passthrough, redirect non-following with exactly one request, timeout override/expiry/invalid values, local-failure token-free messages, credential failures (unset, empty, former-name-only), never-prints-token checks, help-exits-zero checks, absent-feature rejections, and (monitors list) no-pagination-drain. The `main_test.go` tests cover the help surface (`TestRootHelp`, `TestResourceHelp`, `TestCommandHelp`), bare-invocation invalid usage, and the invalid-usage matrix. The 007 sweep verifies this surface; where a contract item is proven by an existing test, the sweep maps it by name and re-runs it; no new feature obligation exists.
- The shared executor `doMonitorsRequest` (`cmd/bs/monitors.go`) carries the response contract for every command: one request, redirects not followed (`http.ErrUseLastResponse`), raw 2xx body to stdout with no appended newline, raw non-2xx body to stderr, exit 0/1, `redactToken` applied only to locally generated diagnostics. The credential check reads only `tokenEnvVar` (`BETTERSTACK_UPTIME_API_TOKEN`); product code contains no read of `BETTERSTACK_API_TOKEN`. The timeout default is the `defaultTimeout` constant (`30 * time.Second`, `cmd/bs/monitors.go`).
- Verification environment (carried from 001–006): `GOTOOLCHAIN=go1.26.9 GOPROXY=off` from the local module cache; the local default `go` is 1.22.2, below the module's `go 1.26` directive.

### Approved acceptance shape (ticket 06 and asset 06)

- The four first-release acceptance criteria from `.wayfinder/tickets/06-installation-distribution-acceptance.md` (Resolution):
  1. **Mock-API automated checks** over all five commands covering the documented filters (including repeatable `--metadata` with duplicate-key rejection and `--team-name` passthrough), the raw byte-for-byte output envelope, the error behavior (non-2xx including 3xx raw on stderr, exit 1, no redirect following, single attempt, 30-second default timeout and `--timeout` override, local/network failure messaging), and token safety (missing/empty token fails before any request, old name never read, token never printed, logged, or stored).
  2. **Pre-publication local install smoke test**: from a clean local checkout, `go install ./cmd/bs` succeeds and the installed `bs` runs `bs --help` successfully. Requires no tag.
  3. **Post-tag remote version check** (conditional): `go install github.com/jrswab/betterstack-cli/cmd/bs@v0.1.0` with `GOPRIVATE=github.com/jrswab/betterstack-cli` and preconfigured authenticated Git access, after a tag exists on the private origin — a release action requiring separate future user authorization to commit/tag/push.
  4. **Real-account check**: user-controlled, separate, never automated; no token shared.
- Criteria 1 and 2 are milestone 007's scope. Criteria 3 and 4 are recorded as deferred by the milestone entry itself; they do not block local completion and are not executed by this milestone under any reading of the source.
- Distribution facts that shape the deferred record (ticket 06, Decision 3): the repository stays private for the first release; `GOPRIVATE` bypasses the module proxy and checksum database but does not grant Git authentication; `go install module@version` resolves over HTTPS Git by default, so authenticated HTTPS Git access (credentials or user-configured SSH `insteadOf` rewriting, with its whole-environment scope warning) is a documented user prerequisite. A toolchain auto-download from the public Go toolchain source is independent of `GOPRIVATE`.

### Constraints and assumptions

- Every shell begins with `unset BETTERSTACK_UPTIME_API_TOKEN` **and** `unset BETTERSTACK_API_TOKEN`. No environment content is printed or logged; no `.env` file is read; no secret value is echoed in logs or evidence.
- Pinned verification toolchain: `GOTOOLCHAIN=go1.26.9 GOPROXY=off`. No toolchain download, no proxy fetch.
- All work stays local: no live API call, external request, push, tag, publication, visibility change, deploy, or deletion is authorized. Tests use only the dummy token and mock loopback servers.
- Milestone 007 changes no product code, no help text, and no existing test's asserted behavior. `README.md`, `LICENSE`, `.gitignore`, `verify-incident-id.sh`, and `.wayfinder/` stay untouched. The only files 007 may write are its own three docs (`007_spec.md`, `007_implement.md`, `007_checkpoint.md`).
- The milestone source entry is the acceptance boundary; it is not broadened. Release documentation beyond the milestone's own durable record is optional and is not required by the milestone text: the 007 docs themselves are the durable artifact that records the exact install usage, the checks performed, and the deferred steps.
- The milestone checkbox `- [ ] 007` renders `[x]` only at 007's own passing completion gate (factory rule), never in this spec or checklist phase.

### Out of scope for milestone 007

- Any product source, help-text, or test-behavior change; any new feature or new command; any refactor beyond what verification requires (none is needed).
- Execution of the deferred release actions: the post-tag remote version check and the real-account check. No tag, push, or publication of any kind.
- CI provider, OS/architecture matrix, release binaries, package managers (deferred by ticket 06).
- README or any other repository-documentation edit outside `docs/plans/001_betterstack-cli-first-release_007_*`.

## Section 2: Requirements

### R1 — Mock-API acceptance sweep over all five commands

- **R1.1 Sweep execution.** The sweep runs the existing approved mock-API tests for all five commands through the established seams (`invokeAPI`/`runWithAPI`, `httptest` loopback servers, dummy token only) under the pinned toolchain, and re-runs them green. No new production behavior is exercised beyond the existing suite's surface.
- **R1.2 Per-command contract items.** The sweep evidence maps, per command, at minimum:
  - `bs monitors get <id>`: bare-ID target; one bearer-authenticated `GET /api/v2/monitors/{id}`; success/error/timeout/credential/token-safety contract items.
  - `bs monitors list`: `--url`, `--pronounceable-name`, `--team-name` passthrough with no token-type detection; `--page`/`--per-page` passthrough with omission sending no parameter; one page only (no auto-drain); the `pagination` object passes through untouched.
  - `bs incidents get <id-or-url>`: bare-ID and dashboard-URL targets of the observed form `https://incidents.betterstack.com/team/<team-id>/incidents/<incident-id>`; narrow local parsing; rejection of every other host/path shape as invalid usage; `included` passthrough when the API sends it.
  - `bs incidents timeline <id-or-url>`: both target forms; no pagination; `timeline_truncated` items pass through untouched.
  - `bs incidents list`: `--from`, `--to`, `--monitor-id`, `--heartbeat-id`, explicit `--resolved`/`--acknowledged` (both forms, exact lowercase literals only, bare flag a syntax error, omission sends no parameter), repeatable `--metadata key=value` translated to `metadata[<key>][][value]` with duplicate keys rejected before any request, `--team-name` passthrough, `--page`/`--per-page` passthrough.
- **R1.3 Raw success envelope.** Every success path prints the raw upstream body byte-for-byte to stdout with no appended newline — including a body with a `pagination` object, a non-JSON 2xx body, and a body carrying its own trailing newline.
- **R1.4 Error contract.** Every non-2xx response, including 3xx, prints the upstream body raw to stderr with exit 1; redirects are not followed; there is exactly one request per invocation (no retries); nothing reaches stdout on failure.
- **R1.5 Timeout contract.** The default timeout is the 30-second `defaultTimeout` constant; `--timeout` accepts a duration value and is honored end-to-end (short override expires against a slow server with one request; longer override succeeds); invalid values fail before any request.
- **R1.6 Credential contract.** Missing or empty `BETTERSTACK_UPTIME_API_TOKEN` exits 1 before any request with an actionable stderr message naming the variable; the former `BETTERSTACK_API_TOKEN` name set alone (or at all) is never read.
- **R1.7 Token safety.** The token value never appears on stdout or stderr on success, non-2xx, or local-failure paths; product code contains no logging or file-writing of the token (source audit: no `log` package use, no credential file writes in product code).
- **R1.8 Exit codes.** Every exercised invocation exits 0 or 1 only.
- **R1.9 Absent features.** The excluded surface is rejected: `--all`, `--pretty`, `--query`, the typed metadata variants `metadata[key][][type]`/`metadata[key][][email]` (no flag exists; no such query parameter can be constructed), and any monitor URL form.

**Acceptance:** the full suite (including race) passes under the pinned toolchain, and the checkpoint records a per-command contract→test mapping in which every R1.2–R1.9 item names the passing test(s) or recorded direct check that proves it.

### R2 — Full regression verification battery

- **R2.1** `GOTOOLCHAIN=go1.26.9 GOPROXY=off go test -count=1 ./...` passes (full suite, all packages).
- **R2.2** `GOTOOLCHAIN=go1.26.9 GOPROXY=off go test -race -count=1 ./...` passes.
- **R2.3** `GOTOOLCHAIN=go1.26.9 GOPROXY=off go vet ./...` exits 0; `gofmt -l .` is empty; `git diff --check` is clean; `GOTOOLCHAIN=go1.26.9 GOPROXY=off go mod verify` reports all modules verified.
- **R2.4** The help surface regresses cleanly: the eight exact help vectors exit 0 with human-readable help on stdout and empty stderr; the invalid-usage matrix (bare `bs`, unknown resource/action, missing/extra positionals, unknown options) exits 1 with readable stderr and empty stdout.

**Acceptance:** every R2 command's actual result is recorded in the checkpoint gate evidence, each green.

### R3 — Pre-publication local install smoke test (clean checkout, isolated GOBIN)

- **R3.1** A clean checkout of the verified HEAD is materialized outside the working tree (for example `git archive HEAD | tar -x -C <tmpdir>`) so the smoke test runs against exactly the committed tree, not uncommitted state.
- **R3.2** From that clean checkout, with an isolated `GOBIN` under a temporary directory and the pinned toolchain (`GOTOOLCHAIN=go1.26.9 GOPROXY=off`), `go install ./cmd/bs` exits 0 and installs a working `bs` binary.
- **R3.3** The installed binary answers `bs --help` with exit 0, human-readable help on stdout, nothing on stderr, and no network request. The invocation runs with both credential variables unset, so it also demonstrates the install smoke needs no token.
- **R3.4** The temporary checkout and GOBIN directory are removed afterwards; no stray `bs` binary or build artifact remains in the repository tree; `git status --porcelain` afterwards shows only the preserved pre-existing untracked entries.

**Acceptance:** the recorded smoke commands and actual results show install exit 0, `bs --help` exit 0, clean status, and cleanup performed.

### R4 — Release restraint and the deferred-action record

- **R4.1** No tag, push, publication, visibility change, deploy, live API call, or external modification happens in milestone 007.
- **R4.2** The checkpoint records the deferred release actions verbatim as deferred, separately authorized, user-controlled steps, not executed by this milestone:
  - the conditional post-tag remote version check — `go install github.com/jrswab/betterstack-cli/cmd/bs@v0.1.0` with `GOPRIVATE=github.com/jrswab/betterstack-cli` and preconfigured authenticated Git access, after a tag exists on the private origin, requiring separate future user authorization to commit/tag/push;
  - the real-account check — Jaron runs the installed `bs` against the live Better Stack Uptime API with his own token in his own environment; no token is shared and no automated step touches the real account.
- **R4.3** The record states the documented user prerequisites for the deferred remote check (GOPRIVATE, authenticated HTTPS Git access or SSH `insteadOf` rewriting with its scope warning, per ticket 06 Decision 3) so a future authorized run can execute it without re-research.

**Acceptance:** the checkpoint's deferred section names both actions verbatim with their prerequisites and the statement that neither is executed by milestone 007.

### R5 — Durable artifact and repository accounting

- **R5.1** The 007 checkpoint is the durable record of the sweep mapping (R1), the verification battery results (R2), the smoke-test commands and results (R3), and the deferred record (R4). No separate release-documentation artifact is created; the milestone text requires no README change, and `README.md` stays untouched.
- **R5.2** `git status` at the completion gate accounts for every change: only the three 007 docs are new or modified relative to the 007 setup commit, plus the rendered `- [x] 007` checkbox in the milestones source at the passing completion gate; the twelve pre-existing untracked entries (`.wayfinder/` tree and `verify-incident-id.sh`) remain preserved and untouched; `cmd/bs` is unchanged.
- **R5.3** The milestone checkbox `- [ ] 007` renders `- [x] 007` only after R1–R4 pass, at the factory completion gate.

**Acceptance:** the completion gate records a clean status diff against the setup commit showing docs-only changes, and the checkbox render follows the gate.

### Parallelism

R1, R2, and R3 are independent verification activities and can run in any order, but all three feed the completion gate. R4 and R5 are recording requirements satisfied while writing the final checkpoint state. No task in this milestone touches product code, so no implementation parallelism constraint exists.
