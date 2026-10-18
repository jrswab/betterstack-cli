# First-release build milestones — betterstack-cli (read-only Better Stack Uptime CLI)

Planning artifact produced by the wayfinder map [betterstack-cli first release — Wayfinder Map](../../.wayfinder/MAP.md) (cleared 2026-10-17) via the milestones skill. It is a **plan only**: no code, no `go.mod`, no tests, no release actions, and no commits are authorized by this document. Downstream, the factory skill processes the unchecked milestones below through spec, checklist, and verified execution; the factory and its spec/implement/exec phases decide implementation details this document deliberately leaves open (test framework, CI provider, exact message wording, duration grammar specifics, internal package structure).

## Section 1: Research Findings

Sources: the closed wayfinder tickets' resolution comments and their linked assets — [Better Stack API contract research for the five first-release commands](../../.wayfinder/tickets/01-api-contracts-for-five-first-release-commands.md) (asset: [API contract research](../../.wayfinder/assets/01-api-contract-research.md)), [Incident URL to incident ID identity resolution](../../.wayfinder/tickets/02-incident-url-identity-resolution.md) (asset: [Incident URL identity research](../../.wayfinder/assets/02-incident-url-identity-research.md)), [Command naming and argument conventions](../../.wayfinder/tickets/03-command-naming-and-argument-conventions.md), [Pagination policy and JSON output envelope](../../.wayfinder/tickets/04-pagination-and-json-envelope.md), [Error handling, timeouts, retries, and token safety](../../.wayfinder/tickets/05-errors-timeouts-retries-token-safety.md), and [Installation, distribution, and first-release acceptance criteria](../../.wayfinder/tickets/06-installation-distribution-acceptance.md) (asset: [Go toolchain and module-layout research](../../.wayfinder/assets/06-go-toolchain-research.md)). All decisions below were approved by Jaron Swab in conversation on 2026-10-16/17 unless marked as technical decisions on authorized judgment.

### Codebase structure and current state

- The repository `github.com/jrswab/betterstack-cli` (origin `git@github.com:jrswab/betterstack-cli.git`, **private**) contains no code and no `go.mod`: only `LICENSE`, `README.md`, `.gitignore`, the `.wayfinder/` planning tree, and the user-requested planning helper `verify-incident-id.sh` at the repository root. The helper stays untouched; it is a one-shot verification script, not part of the CLI build, and no milestone removes or absorbs it.
- Local toolchain observed read-only: `go1.22.2 linux/amd64` — below the chosen minimum. Installing Go 1.26 or newer locally (current stable go1.27.2) is a reported build prerequisite; under default `GOTOOLCHAIN` the go command may auto-download a newer toolchain from the **public** Go toolchain source, independent of `GOPRIVATE` (https://go.dev/doc/toolchain).

### Product scope and command surface (ticket 03)

- A simple **read-only** Go CLI, executable `bs`, over the Better Stack Uptime API. Resource-first two-level syntax, exactly five first-release commands:
  - `bs incidents list`
  - `bs incidents get <id-or-url>`
  - `bs incidents timeline <id-or-url>`
  - `bs monitors list`
  - `bs monitors get <id>`
- Targets are positional. Incident commands accept either an incident ID (including the bare numeric form) or a full dashboard incident URL; monitor commands take a bare monitor ID only (no monitor URL form).
- Help output is human-readable text — the one explicit, agreed exception to JSON-only output, scoped to help only. Three levels: `bs --help` (resource groups with one-line descriptions), resource-level help (`bs incidents --help`, `bs monitors --help`), and per-command help (usage line, arguments, flags, examples) documenting placeholders such as `bs incidents get <id-or-url>`.
- Future resources follow the same naming pattern (lowercase plural resource groups, explicit verbs, one positional target, hyphenated flags); no future commands are charted.

### API contracts (ticket 01 and asset)

- All five endpoints sit on `https://incidents.betterstack.com`: list incidents `GET /api/v3/incidents`, retrieve incident `GET /api/v3/incidents/{incident_id}`, timeline `GET /api/v3/incidents/{incident_id}/timeline`, list monitors `GET /api/v2/monitors`, retrieve monitor `GET /api/v2/monitors/{monitor_id}`. Incident resources are v3, monitors v2; the version split applies to list and retrieve alike.
- Auth is a bearer token (`Authorization: Bearer $TOKEN`, JSON:API-style API). Two token types exist (global, team-scoped Uptime/Incidents token); **no read-only token scope exists** — the CLI's read-only agreement is self-imposed and the token can write if leaked. Token capability scope is unverified and must not be assumed.
- Both lists paginate via `page`/`per_page` with a body `pagination` object of absolute `first`/`last`/`prev`/`next` URLs (`null` at the ends). Per-resource limits differ: list incidents default 10, max 50 per page; list monitors default 50, max 250. The timeline has **no pagination** and may contain `timeline_truncated` items — output cannot promise completeness.
- Documented list filters — incidents: `from`, `to`, `monitor_id`, `heartbeat_id`, `resolved`, `acknowledged`, `metadata[key][][value]` (plus typed variants `metadata[key][][type]`/`[email]` — documented syntax, undocumented matching behavior); monitors: `url`, `pronounceable_name`. `team_name` exists on both lists and applies only with a global token.
- Responses are JSON:API-style (`data`/`attributes`/`relationships`, optional `included` on incident retrieve only). IDs are **string-typed** in JSON (`"id": "123456789"`) and must be treated as opaque strings, not parsed as numbers. Timeline item IDs are heterogeneous strings.
- Documented error convention: top-level `errors` **string** (401 and 404 bodies shown on other official pages). 429 is documented with no body, no headers, and no numbers. 400/5xx bodies, exact five-endpoint error wording, and rate-limit numbers are unknown — the CLI must not build on them.

### Incident URL identity rule (ticket 02 and asset, resolved 2026-10-17)

- **Same ID, confirmed for one user-reported sample.** Jaron ran the `verify-incident-id.sh` helper in his own terminal with his own token against `GET https://incidents.betterstack.com/api/v3/incidents/1033092002` — the final path number of the example dashboard URL `https://incidents.betterstack.com/team/t581731/incidents/1033092002` — and reported verbatim: `HTTP status: 200` and `incident id: 1033092002`. The evidence is user-reported; no agent performed an authenticated request and no token was shared.
- **Identity rule for the CLI:** a dashboard incident URL of the observed form `https://incidents.betterstack.com/team/<team-id>/incidents/<incident-id>` is resolved by narrow, **local** parsing — accept exactly this host and path shape, extract the terminal numeric segment as the API incident ID, and call the documented API with it. No extra "resolve" API call, no probe-then-match fallback, no network fetch to the user-supplied URL's host, no team-segment validation against the token. Output follows the raw passthrough envelope (nothing echoed of the CLI's own). If the extracted ID is wrong for some incident, the ordinary non-2xx error rules apply unchanged.
- **Documented limitations (accepted):** the same-ID rule rests on one sample, not on documentation (no official page states it); no cross-team verification was performed and cross-team behavior under a team-scoped token is undocumented; alternate URL shapes are unknown and only the observed form is supported in the first release.

### Flags and filter conventions (ticket 03)

- List commands expose the documented API filters as **named, hyphenated flags** mapping onto the underscore API parameters (`monitor_id` → `--monitor-id`, `heartbeat_id` → `--heartbeat-id`, `pronounceable_name` → `--pronounceable-name`, `team_name` → `--team-name`, `per_page` → `--per-page`). There is no `--query "..."` raw passthrough.
- Boolean flags `--resolved` and `--acknowledged` require an explicit `true`/`false`; a bare flag with no value is a syntax error. Omitting a flag omits the query parameter, so the documented API default applies — the CLI supplies no defaults of its own.
- `--metadata` is repeatable, one `key=value` pair per occurrence, translated into the documented nested query form (`--metadata Origin=Linear` → `metadata[Origin][][value]=Linear`). Repeats with different keys are accepted; a duplicate key in one invocation is a syntax error (future-revisitable). The typed variants `metadata[key][][type]` and `metadata[key][][email]` (documented syntax, undocumented matching behavior) are excluded from the first release — no flag exists for them.
- `--team-name <name>` is optional on both list commands, passed through unchanged with **no token-type detection** (no classification, warning, or rejection); omitting it sends no `team_name` parameter. The documented filter applies only with a global token; the undocumented team-scoped-token case is left to whatever the API returns, handled by the error rules.

### Output envelope and pagination policy (ticket 04)

- Successful API responses print the raw Better Stack body **byte-for-byte** to stdout: no wrapper or envelope, no JSON parsing, validation, or re-serialization (a non-JSON 2xx body still prints raw), no pretty-printing, no compact mode, no `--pretty`, and **no appended newline**. `data`, `included`, and `pagination` pass through as the API sent them.
- Both list commands default to one API page. Optional `--page <n>` and `--per-page <n>` map one-to-one onto the API parameters; given values pass through unchanged for **upstream validation** — no hardcoded limits, no clamping, no substitution, no pre-request rejection, no local numeric grammar beyond passthrough. Omission sends no parameter (API defaults apply).
- No auto-drain: the CLI never follows `pagination.next`; no `--all`-style flag exists. The passthrough `pagination` object's `next` URL shows the user whether further pages exist.
- Compatibility promise: only that `bs` preserves the response as received. No promise that upstream fields never change.

### Error handling, timeouts, retries, token safety (ticket 05)

- Every non-2xx HTTP response — **including 3xx** — prints the upstream body to stderr byte-for-byte (no wrapper, no added status summary, no parsing) and exits 1. Redirects are **not followed**: one request to the requested endpoint, so the `Authorization` header never reaches a server-chosen redirect target. 401/403/404/429/5xx and malformed error bodies all fall under this one rule.
- Exit codes: 0 on success, 1 on every failure including invalid command usage. stdout carries successful bodies only.
- Missing or empty `BETTERSTACK_UPTIME_API_TOKEN`: an actionable message naming the variable prints to stderr and the CLI exits 1 **before any request**. There is no fallback to the former `BETTERSTACK_API_TOKEN` name — unsupported, no alias, and the CLI must not read the old name.
- One API request attempt per invocation; **no automatic retries** (no retry flag, backoff, or `Retry-After` handling).
- Timeout: a configurable total request timeout, default **30 seconds**, overridable per invocation with `--timeout` accepting a duration value (approved example `bs incidents get 123 --timeout 60s`). Duration grammar specifics are an implementer choice.
- Local and network failures (timeout expiry, connection refused, DNS/TLS failure): one concise, actionable, token-free stderr message and exit 1; nothing on stdout.
- Token safety: the token is never printed, logged, or stored (no files, cache, or diagnostics capture). It is the only credential the CLI reads. Transient in-memory use to build the header is normal operation. No redaction or inspection of response bodies exists; if a real passthrough/no-print contradiction ever appears, that is a separate decision for Jaron.

### Installation, module, versioning, acceptance (ticket 06 and asset)

- Distribution: `go install` from the module path is the only first-release channel (GitHub release binaries and package managers deferred). The repository stays **private** for the first release — no public visibility change.
- Module path `github.com/jrswab/betterstack-cli`, installable command package `cmd/bs` (conventional `cmd/<progname>` layout, https://go.dev/doc/modules/layout). Intended first-release install command: `go install github.com/jrswab/betterstack-cli/cmd/bs@v0.1.0`. (Technical decision made on authorized judgment within the approved shape; evidence-dated in the asset.)
- Minimum `go` directive `go 1.26`, **no `toolchain` directive** — chosen from the official support window (Go 1.27.x and 1.26.x supported on 2026-10-17; current stable go1.27.2), not from the outdated local go1.22.2 toolchain. (Technical decision made on authorized judgment within the approved shape; evidence-dated in the asset.)
- Private-module install prerequisites, **documented only, nothing configured now**: `GOPRIVATE=github.com/jrswab/betterstack-cli` (bypasses proxy/sumdb) plus preconfigured authenticated Git access — `GOPRIVATE` alone does not grant auth; `go install` resolves over HTTPS Git by default, and a local SSH remote does not ensure fetch auth. Optional user-configured HTTPS credentials or SSH `insteadOf` rewriting (whole-environment scope warning) are documented in the ticket.
- Versioning: semantic version tags starting at `v0.1.0` — **planning only**; no tag, push, or publication is authorized by this document or the map.
- First-release acceptance (approved shape, completed in ticket 06):
  1. Automated **mock-API checks** covering all five commands: documented filters (including repeatable `--metadata` with duplicate-key rejection and `--team-name` passthrough), the raw output envelope, error behavior, timeout, and token safety.
  2. Pre-publication **local install smoke test**: from a clean checkout, `go install ./cmd/bs` succeeds and `bs --help` runs. Requires no tag.
  3. **Conditional post-tag remote version check** (`go install ...@v0.1.0` with GOPRIVATE and authenticated Git): a release action requiring **separate future user authorization** to commit/tag/push to the private origin — not authorized now, not part of any automated gate.
  4. **Real-account check**: user-controlled, separate, not automated; no token shared.
- Test framework, test module layout, OS/architecture matrix, and CI provider are deliberately unspecified — later implementation decisions within the approved acceptance shape.

### Out of scope (fixed by the map — do not build)

- Write or state-changing operations (acknowledge/resolve incidents, monitor mutations).
- Automatic investigations, evidence bundles, diagnosis, or any agentic behavior.
- Notification and escalation commands.
- Any API resources beyond the five commands; telemetry and other non-Uptime Better Stack APIs (separate API key; the `BETTERSTACK_TELEMETRY_API_TOKEN`-style name is a non-binding recommendation only).
- The later agent-facing skill file that would guide agents to use the CLI.
- Any credential configuration (GOPRIVATE setup, Git credential/insteadOf edits) — documented for users, never executed by the build.

### Resolved questions (with sources)

- Do the dashboard URL number and the API incident ID coincide? — Same ID, user-reported HTTP 200 with equal `data.id` for one sample; limitations documented (ticket 02 resolution, asset section 8).
- Which base host and versions? — `incidents.betterstack.com`, incidents v3 / monitors v2 (ticket 01 resolution, asset section 1).
- Does any read-only token scope exist? — No; token treated as write-capable (ticket 01 resolution, asset section 2).
- Raw passthrough or an envelope? Raw success body or validated JSON? — Byte-for-byte raw passthrough, no validation, no newline (ticket 04 resolutions).
- Auto-drain lists? — No; one page default with `--page`/`--per-page` passthrough (ticket 04 resolutions).
- Redirect following? Retries? — Neither; one attempt, 3xx is an ordinary non-2xx failure (ticket 05 resolutions).
- Exit-code policy? — 0 success, 1 every failure (ticket 05 resolution).
- Module path, layout, minimum Go? — `github.com/jrswab/betterstack-cli`, `cmd/bs`, `go 1.26`, no toolchain directive (ticket 06 resolution, asset sections 4 and 8).
- Where do dashboard URL fragments vs IDs apply? — Bare numbers are the incident ID form; incident commands accept both positionally, monitors take an ID only (ticket 03 resolution).

### Rejected approaches (with reasons)

- Raw query-string passthrough (`--query`) for list filters — hides the accepted surface and permits undocumented parameters; named hyphenated flags chosen (ticket 03).
- Local validation of `--page`/`--per-page` against documented maxima — the maxima are hedged in the docs, would drift, and duplicate a limit the API enforces; upstream validation chosen (ticket 04).
- Pretty-print, compact mode, `--pretty`, JSON validation/re-serialization on the success path — the CLI stays between user and API; users pipe to `jq` themselves (ticket 04).
- Auto-drain / `--all` multi-page retrieval — would fabricate a body shape the API never sent; visible `pagination.next` suffices (ticket 04).
- Distinct exit codes per failure kind — one failure code keeps scripting simple (ticket 05).
- Automatic retries or redirect following — retries would guess at undocumented 429 behavior; redirects could carry the token to a server-chosen destination (ticket 05).
- Token-type detection for `--team-name` — no documented property distinguishes global from team-scoped tokens; any detection would guess (ticket 03).
- Typed metadata filter variants — excluded to keep the first-release flag surface minimal (ticket 03).
- Probe-then-match URL resolution and echoing the API's `data.id` — moot under the confirmed same-ID rule and superseded by raw passthrough; retired with ticket 02.
- GitHub release binaries, Homebrew, and other package managers for the first release — deferred; `go install` only (ticket 06).

### Constraints and assumptions

- The token is write-capable; token capability scope is unverified and never assumed.
- Unknowns that must not be built on: rate-limit numbers/headers, 400/5xx error bodies, exact five-endpoint error wording, cross-team URL-ID behavior, alternate dashboard URL shapes, whether the API itself ever redirects.
- Local Go ≥ 1.26 is a build prerequisite; the current local go1.22.2 is below it.
- The repository is private; remote installability depends on user-side GOPRIVATE and authenticated Git configuration that the build does not perform.
- The same-ID URL rule rests on one user-reported sample; help/spec text may state the observed URL form but the rule is empirical, not documented by Better Stack.

## Section 2: Milestones

Dependency-ordered vertical slices per the `/tdd` and codebase-design guidance: each numbered milestone delivers a working end-to-end outcome, with enabling or acceptance milestones naming the work they serve. Downstream factory phases (spec → implementation checklist → verified execution) handle all implementation detail; each entry states the outcome and acceptance boundary only.

- [x] 001: Installable `bs` command skeleton with three-level help — `go.mod` with module path `github.com/jrswab/betterstack-cli`, `go 1.26` directive, no `toolchain` directive, and an installable `cmd/bs` whose `main` answers `bs --help`, `bs incidents --help`, `bs monitors --help`, and per-command help with usage lines, positional placeholders (`<id-or-url>`, `<id>`), flags, and examples, in human-readable text; `--help` exits 0 — the conventional technical implementation default for help output, not a separately approved human decision — and invalid user usage exits 1 with readable errors. Enables every later slice (the command surface and module layout all slices build on). Acceptance: a clean local checkout passes `go install ./cmd/bs` and the installed binary serves all three help levels; no API calls yet.
- Depends on: none.
- [x] 002: `bs monitors get <id>` end-to-end against a mock API — the first working command: credential channel reading only `BETTERSTACK_UPTIME_API_TOKEN` (missing or empty fails before any request with an actionable stderr message naming the variable, exit 1; the former `BETTERSTACK_API_TOKEN` name is never read), one bearer-authenticated `GET /api/v2/monitors/{id}` attempt, raw success body byte-for-byte on stdout with no appended newline, any non-2xx including 3xx body raw on stderr with exit 1 and no redirect following, 30-second default total timeout overridable by `--timeout <duration>`, no retries, and concise actionable token-free stderr messages with exit 1 for local/network failures. Acceptance: mock-API checks cover success passthrough, each failure class, timeout override, missing-token, and the never-printed-token rule; exit codes are 0/1 only.
- Depends on: 001.
- [x] 003: `bs monitors list` with pagination and monitor filters — one API page by default against `GET /api/v2/monitors`, optional `--page`/`--per-page` passed through unchanged for upstream validation with omission sending no parameter, named hyphenated filters `--url`, `--pronounceable-name`, and optional `--team-name` passed through unchanged with no token-type detection, the `pagination` object passing through untouched, and no auto-drain or `--all` flag. Acceptance: mock-API checks cover flag-to-query translation, omission-passes-API-defaults, one-page-only behavior, and error passthrough under the 002 rules.
- Depends on: 002.
- [x] 004: `bs incidents get <id-or-url>` with dashboard URL support — retrieve against `GET /api/v3/incidents/{incident_id}` accepting a bare incident ID or a full dashboard URL of the observed form `https://incidents.betterstack.com/team/<team-id>/incidents/<incident-id>`, resolved by narrow local parsing (exact host and path shape only, terminal numeric segment extracted as the API incident ID candidate, no fetch to the user-supplied host, no extra resolve call, no team-segment validation), with ordinary non-2xx error rules applying unchanged when the candidate ID is wrong. Acceptance: mock-API checks cover ID-form and URL-form targets, rejection of every other host/path shape as invalid usage (exit 1), and passthrough of the API body including `included` when the API sends it.
- Depends on: 002.
- [x] 005: `bs incidents timeline <id-or-url>` — timeline against `GET /api/v3/incidents/{incident_id}/timeline` under the same positional `<id-or-url>` target rule as `bs incidents get`, with no pagination, the body passed through raw including any `timeline_truncated` items, and no completeness promise. Acceptance: mock-API checks cover URL-form and ID-form targets, heterogeneous timeline item shapes passing through untouched, and error passthrough.
- Depends on: 004.
- [x] 006: `bs incidents list` with the full documented incident filter surface — one API page by default against `GET /api/v3/incidents`, `--page`/`--per-page` passthrough as in 003, named hyphenated filters `--from`, `--to`, `--monitor-id`, `--heartbeat-id`, explicit `--resolved=true|false` and `--acknowledged=true|false` (bare flag is a syntax error; omission sends no parameter), repeatable `--metadata key=value` translated to the documented nested query form with duplicate keys rejected as invalid usage and the typed variants absent, and optional `--team-name` passthrough. Acceptance: mock-API checks cover each filter's query translation, boolean explicitness, metadata repetition and duplicate-key rejection, omission behavior, and error passthrough.
- Depends on: 003 and 004.
- [ ] 007: First-release acceptance verification — the release-readiness checkpoint over the whole surface: a mock-API acceptance sweep covering all five commands against the agreed contracts (filters, raw byte-for-byte stdout envelope with no newline, non-2xx-including-3xx raw stderr with exit 1, no redirects, no retries, 30-second `--timeout` default and override, missing-token pre-request failure, token never printed/logged/stored, old env name never read, exit 0/1 only), plus the pre-publication local smoke test (`go install ./cmd/bs`, then `bs --help`). The conditional post-tag remote check (`go install github.com/jrswab/betterstack-cli/cmd/bs@v0.1.0` with GOPRIVATE and authenticated Git) and the real-account check are recorded as deferred, separately authorized, user-controlled steps — not executed by this milestone. No tag, push, or publication happens here. Acceptance: the sweep and smoke test pass on a clean checkout; every deferred release action is documented as requiring separate future authorization.
- Depends on: 001–006 (verifies the complete first release).
