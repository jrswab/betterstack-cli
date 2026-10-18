# Milestone Checkpoint

The durable recovery record and sole home of run state for the current milestone. Another agent must be able to restart or take over from this file alone.

## Identity

- Milestone ID: `005`
- Milestone source: `docs/plans/001_betterstack-cli-first-release_milestones.md`
- Repository root: `/home/jaron/dev/betterstack-cli/master`
- Branch: `master`
- Baseline / pre-existing changes (verbatim `git status --porcelain=v1 -uall` at setup, HEAD `a1baf30` — the 004 completion commit `004: record review and completion gates, render checkboxes`):

```text
?? .wayfinder/MAP.md
?? .wayfinder/README.md
?? .wayfinder/assets/01-api-contract-research.md
?? .wayfinder/assets/02-incident-url-identity-research.md
?? .wayfinder/assets/06-go-toolchain-research.md
?? .wayfinder/tickets/01-api-contracts-for-five-first-release-commands.md
?? .wayfinder/tickets/02-incident-url-identity-resolution.md
?? .wayfinder/tickets/03-command-naming-and-argument-conventions.md
?? .wayfinder/tickets/04-pagination-and-json-envelope.md
?? .wayfinder/tickets/05-errors-timeouts-retries-token-safety.md
?? .wayfinder/tickets/06-installation-distribution-acceptance.md
?? verify-incident-id.sh
```

All pre-existing changes are untracked files preserved untouched. Predecessor state: milestones 001–004 are complete (source entries `- [x] 001`–`- [x] 004`; the 004 checkpoint passed restart validation at this setup). The credential-safety rules carry forward: every shell begins with `unset BETTERSTACK_UPTIME_API_TOKEN` **and** `unset BETTERSTACK_API_TOKEN`; no environment content is printed or logged; no live API call, external request, publication, push, tag, or deletion is authorized. The `cmd/bs` `TestMain` credential isolation and loopback-only network guard (committed at `812ec5a`) remain enabled and must not be weakened or bypassed; future tests use `testHTTPClient()` and never inject a custom transport into product code. Jaron's deferred token rotation stays a user action; the preserved unauthorized live-GET record and first-token-exposure warning stay preserved (never erased), and Jaron's 2026-10-18 "yes" approval covers local continuation only.

## Artifacts

- Spec: `docs/plans/001_betterstack-cli-first-release_005_spec.md` (not yet written — Phase 1 is the next phase)
- Implementation checklist: `docs/plans/001_betterstack-cli-first-release_005_implement.md` (not yet written — Phase 2)
- Checkpoint: `docs/plans/001_betterstack-cli-first-release_005_checkpoint.md` (this file, written by atomic rename at setup)

## State

- Current phase: `setup` — the outer loop validated the 004 handoff and selected 005; no 005 artifact exists yet
- Current task: `none` — Phase 1 (spec) is the next action; no 005 skill load has happened and no product code has been touched for 005
- Task states: none yet (the 005 checklist does not exist; checkboxes will render from it in Phase 2)
- Milestone checkbox: `unchecked` (source entry `- [ ] 005`, never edited by this run)
- Last completed phase/task/gate and timestamp: 004 completion gate passed 2026-10-18 at commit `a1baf30` (recorded in `docs/plans/001_betterstack-cli-first-release_004_checkpoint.md`); this 005 setup follows it

## Milestone scope (verbatim source entry, for restart grounding)

- 005: `bs incidents timeline <id-or-url>` — timeline against `GET /api/v3/incidents/{incident_id}/timeline` under the same positional `<id-or-url>` target rule as `bs incidents get`, with no pagination, the body passed through raw including any `timeline_truncated` items, and no completeness promise. Acceptance: mock-API checks cover URL-form and ID-form targets, heterogeneous timeline item shapes passing through untouched, and error passthrough.
- Depends on: 004 (complete).

## Gate evidence

- Gate: 004 handoff restart validation (performed this session before 005 selection)
  - Command(s) and actual result(s):
    - `git rev-parse --show-toplevel` → `/home/jaron/dev/betterstack-cli/master`; `git branch --show-current` → `master`; `git rev-parse HEAD` → `a1baf30641fe2d988d6a1f6108d597173c37f200` (004 completion commit); `git rev-parse HEAD^{tree}` → `f74883595092dc968ffde5cda2eaa988616fc70d`; `git status --porcelain=v1 -uall` → the verbatim baseline above (12 untracked entries, 0 tracked modifications, 0 staged entries); `git stash list` → 0 entries
    - Milestone ID/paths: 004 checkpoint ID `004`, artifact paths `docs/plans/001_betterstack-cli-first-release_004_{spec,implement,checkpoint}.md` all present and matching; root and branch match this worktree
    - Checkbox agreement: source `- [x] 004` at line 131 agrees with the 004 checkpoint `checked` completion gate; all five 004 task boxes render `[x]` in the 004 implement checklist; source `- [ ] 005` is the first remaining unchecked entry (001–004 render `[x]`)
    - Gate-evidence refs: every 004 checkpoint gate records a real HEAD or snapshot SHA; the nine snapshot refs and both product anchors were inspected with native Git (`git show --stat`) and remain inspectable; none were applied. Product code is unchanged since the review-passed HEAD `812ec5a` (`git diff 812ec5a..a1baf30 -- cmd/` is empty — the completion commit is doc-only)
    - 004 completion-gate commands re-verified this session under the standing regime (all shells began with `unset BETTERSTACK_UPTIME_API_TOKEN` and `unset BETTERSTACK_API_TOKEN`; no env dump, no xtrace, no `.env` read, no old-secret logs): `go vet` exit 0, `gofmt -l .` empty, full suite `ok ... 1.500s`, race suite `ok ... 2.565s`, `git diff --check` clean, and the isolated-GOBIN install/help smoke passed (`GOBIN=/tmp/bs004-install-gate` under `GOTOOLCHAIN=go1.26.9 GOPROXY=off`, `bs --help` exit 0, temp dir removed, `ROOT_BS_ABSENT`)
    - One recording-procedure incident disclosed and recovered inside the 004 checkpoint itself: the review verdict was first written as a whole-file rewrite (destroying the committed gate history), restored from HEAD in the same session before any commit, and re-added by append; the 004 checkpoint carries the disclosure
  - `HEAD` commit SHA: `a1baf30641fe2d988d6a1f6108d597173c37f200`
  - `git status --porcelain`: the verbatim baseline above
  - Relevant work: `committed` (004 fully committed; nothing of this validation is uncommitted)

- Gate: 005 setup (this gate)
  - Command(s) and actual result(s):
    - Source re-read: the full milestone document `docs/plans/001_betterstack-cli-first-release_milestones.md` was read this session; the first entry matching `- [ ] NNN:` is `005` (001–004 render `[x]`). Milestone 005's entry and acceptance line are quoted verbatim in the scope section above
    - Artifact paths resolved per the established convention: `docs/plans/001_betterstack-cli-first-release_005_{spec,implement,checkpoint}.md`; no 005 artifact existed before this checkpoint (`ls docs/plans | grep 005` → none)
    - Path research grounded in current files read this session:
      - `.wayfinder/assets/01-api-contract-research.md` sections 6.3 and 7 — the timeline endpoint is `GET https://incidents.betterstack.com/api/v3/incidents/{incident_id}/timeline` (v3, same incident resource version as 004's retrieve); the documented response is `{"data": [...]}` where each item is `{id, type: "timeline_item", attributes: {item_type, at, data}}` with heterogeneous `id` strings (numeric `"23"`, composite `"2021-02-12 23:59:59 UTC"`, `"2470302-content-response"`, `"incident-started"`) and seven documented `item_type` shapes (`generic`, `generic_card`, `timeline_truncated`, `response_item`, `payload`, `comment` — whose `content` is an object, unlike the string `content` elsewhere — and `status_report_card`); section 7 confirms the timeline **does not support pagination** and returns all retained items, with `timeline_truncated` marking API-side truncation and no completeness promise; no `pagination` object appears in the example response
      - Milestone constraints reused unchanged: the positional `<id-or-url>` target rule is "the same positional target rule as `bs incidents get`" — the narrow local parsing rule decided in ticket 02 and implemented in 004 (`resolveIncidentTarget`, `cmd/bs/incidents.go`) applies unchanged; raw byte-for-byte passthrough with no appended newline (ticket 04); the 002/004 error/timeout/credential/token-safety rules (ticket 05); `--timeout <duration>` per the established per-command flag pattern (`incidentsTimelineHelp` already documents it)
      - Current product state: `cmd/bs/main.go` routes `incidents get` to `incidentsGet`; `incidents timeline <target>` (non-help, one positional) still falls through `runWithAPI` to `usageError` → `resourceUsageError`, whose get/timeline branch returns `bs: "incidents timeline" is not implemented in this milestone; run 'bs incidents timeline --help' for usage.` with exit 1 — this is the exact behavior 005 replaces for the timeline vector only (`incidents list` stays not-implemented, reserved for 006). `cmd/bs/help.go` `incidentsTimelineHelp` already documents `bs incidents timeline <id-or-url>`, the URL form, `--timeout`, and example `bs incidents timeline 123` — no help text changes anticipated (same 004 situation)
      - Reusable seams: `parseIncidentsGetArgs` (one positional + `--timeout`) and `resolveIncidentTarget` (narrow rule) already exist and are directly reusable for the timeline target; the shared executor `doMonitorsRequest` is endpoint-agnostic (`endpoint, token, timeout, stdout, stderr, client`) and serves the timeline path unchanged; the `invokeAPI`/`runWithAPI` mock-loopback test seam and the `safety_test.go` guard (`TestMain` credential isolation + loopback-only transport + `testHTTPClient()`) are reused unchanged
      - Test-preservation surface for the 005 checklist to plan: `TestUnimplementedCommandsMakeNoRequest` (`cmd/bs/monitors_test.go`) currently lists `incidents timeline` as not-implemented and must drop that case when the vector becomes working (the 004 B2 precedent); `TestInvalidUsageMatrix` (`cmd/bs/main_test.go`) currently has no `incidents timeline` case (removed with `monitors list` in 004 B2 after the live-GET incident), so no stale-entry hazard exists there; `resourceUsageError`'s get/timeline shared branch needs the same structural treatment as 004 B2 (route `incidents timeline` before the fallthrough; keep the branch serving `incidents get` missing/extra-target errors)
      - Script discovery in required order: no relevant CLI on `PATH` for this setup; the repository has no `.agents/scripts/`; shared `~/.agents/scripts/` has no factory-setup script — direct commands used
      - Toolchain: `GOTOOLCHAIN=go1.26.9 GOPROXY=off` from the local module cache remains the verified verification environment; local default `go` is 1.22.2
    - Approval boundaries: Jaron's "yes" approval stands for local work only; no live API call, push, tag, deploy, publication, external write, or deletion is authorized; the 005 work is local commits scoped to 005 artifacts, checkpoint, and (at completion) product code
    - This checkpoint was written by replacing a temporary file with an atomic rename (factory setup step 2)
  - `HEAD` commit SHA: `a1baf30641fe2d988d6a1f6108d597173c37f200` (setup; the checkpoint file itself is untracked until the first 005 commit)
  - `git status --porcelain`: the verbatim baseline above plus `?? docs/plans/001_betterstack-cli-first-release_005_checkpoint.md` once written
  - Relevant work: `uncommitted` — stash ref `none` (the run-created checkpoint file is the only new path; the standing Phase-1-gate snapshot procedure covers it when the spec exists)

## Limitations & blockers

- Limitations:
  - No 005 spec or checklist exists yet; this checkpoint is setup-only. The spec must be written and pass the Phase 1 gate before any product change (outer-loop rule: no 005 skill work has happened and none may precede the spec).
  - The credential-safety regime and the `safety_test.go` guard limits (covers only clients using `http.DefaultTransport` or `testHTTPClient()`; cannot stop a test that injects its own transport or dials `net.Conn` directly) carry forward from 004 and are documented in `cmd/bs/safety_test.go`.
  - All work stays local: no push, tag, release, deploy, live API call, or external modification. Tests use only dummy tokens and mock loopback servers.
- Blockers: none.
- Next action (the single next safe action): Phase 1 — write `docs/plans/001_betterstack-cli-first-release_005_spec.md` from the 005 milestone entry and the research above, inspect it against the entry phrase by phrase, and record the Phase 1 gate here. No product code changes before the Phase 2 checklist passes.
