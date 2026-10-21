# Milestone 007 Specification — S6 Telemetry repository records

- Milestone source: `docs/plans/008_telemetry-logs-search_milestones.md:55` (entry `007`, S6)
- Scope: finalize repository records for implemented Uptime and Telemetry behavior. No code changes, live requests, credentials, commits, or changes to protected baseline files.
- Acceptance in one line: every memory file is indexed, memory and README describe current commands and configuration, `TAGS.md` matches every code anchor, changelog fragments accurately cover the milestones, and provenance records follow repository conventions.

## Section 1: Context & Constraints

### Current behavior and records

The command routes `sources list`, `sources get`, and `logs search` through `runWithEndpoints`. `logs search` builds parameterized SQL and sends it to the separate SQL executor. Uptime, Telemetry source, and SQL credentials and request paths remain separate. See `.mem/summary.md:3`, `.mem/telemetry/sources.md:5-12`, `.mem/telemetry/logs.md:5-7,73-99`, and `README.md:3-4,44-64`.

The memory map currently links the project records and all listed Uptime and Telemetry topics. The acceptance requires that this remain true for every current and newly added memory file. See `.mem/mem-map.md:1-21` and the memory skill's index requirement in `.agents/skills/mem/SKILL.md:30`.

`TAGS.md` describes one row per production boundary occurrence. The current index covers router, configuration, command-handler, and request-executor anchors. The code has these anchors in `cmd/bs/main.go:13,29,35`, `cmd/bs/logs.go:18,26,38`, `cmd/bs/sources.go:14,18,38`, `cmd/bs/monitors.go:15,29,55,81`, and `cmd/bs/incidents.go:50,82,332`. Compare the current index at `TAGS.md:1-22` with the source anchors.

`README.md` already documents `bs sources list|get`, `bs logs search`, and five environment variables in three groups: the Uptime API token, the Telemetry API token, and the three SQL credentials. See `README.md:5-87`. Milestone 007 must verify and correct the complete current contract, not replace correct content with a shorter summary. In particular, `BETTERSTACK_TELEMETRY_API_TOKEN` authenticates source commands and is used by log search only to redact it from SQL error output. See `cmd/bs/logs.go:92`.

The current changelog fragments use a category heading, one factual bullet, and repository-root paths in backticks. The 005 fragment says the SQL command route remains unavailable, while the 006 fragment says `bs logs search` now executes SQL. See `changelog.d/008-telemetry-logs-search-005.md:1-3` and `changelog.d/008-telemetry-logs-search-006.md:1-3`. The 007 fragment records the 007 record updates in one `### Changed` bullet and leaves the 005 fragment unchanged. The release skill owns reconciliation of accumulated entries at release time. The 006 fragment already states that the route is active. See `.agents/skills/changelog/SKILL.md:11` and `changelog.d/008-telemetry-logs-search-006.md:1-3`.

`LOG.md` is zero bytes, and `log.d/` contains only a zero-byte `.gitkeep`. See `LOG.md` and `log.d/.gitkeep`. The provenance skill permits an entry only when repository-level rationale blocks a plausible incorrect future change. The tracked 001 specification states, “Git cannot preserve an empty `log.d/` directory in a commit,” and requires the zero-byte marker while defining “empty” as no provenance entries (`docs/plans/008_telemetry-logs-search_001_spec.md:26`). It also says, “Do not add a rationale entry because S0 adds no non-obvious behavior change that needs historical explanation” (`docs/plans/008_telemetry-logs-search_001_spec.md:84`). The tracked 006 specification says, “Do not modify `LOG.md` or `log.d/` unless implementation introduces a new non-obvious rationale not already settled here,” and confirms no change is needed for decisions already captured (`docs/plans/008_telemetry-logs-search_006_spec.md:98,100`). Unless completion review identifies rationale that passes the provenance skill's gate, leave `LOG.md` empty and preserve `log.d/.gitkeep`. See `.agents/skills/provenance-log/SKILL.md:40-42`.

The repository has no Makefile. `make help` returned exit 2 with `No rule to make target 'help'`. Decision ticket 08 corrects the premise that repository guidance requires a Makefile. See `.wayfinder/telemetry/tickets/08-task-repo-record-gaps-mem-tags-log-changelog-makefile.md:17-19` and current `AGENTS.md:5-7`.

### Settled decisions and boundaries

The accepted command surface is read-only and includes `bs sources list`, `bs sources get <id>`, and `bs logs search`. Source REST uses the fixed Telemetry REST host and `BETTERSTACK_TELEMETRY_API_TOKEN`; SQL uses separate SQL URL, username, and password configuration. Do not add token fallback or a configurable REST host. See ticket 01 at `.wayfinder/telemetry/tickets/01-charting-decisions-destination-scope-and-command-surface.md:11-23`, ticket 02 as amended by tickets 10 and 11 at `.wayfinder/telemetry/tickets/02-technical-design-decisions-recorded-at-charting.md:11-25`, `.wayfinder/telemetry/tickets/10-technical-decisions-from-live-verification.md:18,20`, and `.wayfinder/telemetry/tickets/11-sql-api-errors-concurrency-time-bounds.md:15-40`, plus current README at `README.md:44-64`.

The SQL response policy remains unchanged: 2xx bodies pass through unchanged, non-2xx SQL bodies are redacted and written to stderr, and the CLI adds no retry or client concurrency guard. Time bounds use RFC3339Nano and require `from < to`; `--since` is exclusive with explicit bounds. See ticket 11 at `.wayfinder/telemetry/tickets/11-sql-api-errors-concurrency-time-bounds.md:15-40` and `.mem/telemetry/logs.md:45-49,73-97`.

The optional hook-comment nit noted in the 004 checkpoint is out of scope. It concerns a hook comment and historical review notes, not the 007 acceptance records or current Uptime/Telemetry behavior. See `docs/plans/008_telemetry-logs-search_004_checkpoint.md:464-465`.

This is a repository-record milestone. Do not modify code, protected paths, or unrelated historical planning artifacts. Do not modify the milestone tracking document except for the completion-gate checkbox render `- [ ] 007:` → `- [x] 007:`. The run handoff decision is to never record the Go 1.26.10 PATH workaround in `.mem/`; this is a planning boundary, not a rule attributed to the memory skill.

## Section 2: Requirements

### R1 — Describe current Uptime and Telemetry behavior in memory

The `.mem/` set must describe the implemented Uptime and Telemetry command behavior in the present tense. It must preserve the separate REST and SQL request contracts, credentials, validation, response handling, help behavior, and test-safety boundaries supported by the current code and records. Correct stale or conflicting statements where they affect current behavior. Create `.mem/telemetry/summary.md` to describe the implemented Telemetry domain. Link it from `.mem/mem-map.md` and point the Telemetry link in `.mem/summary.md` to `.mem/telemetry/summary.md`, following the memory skill's layout. Keep each touched memory file within the skill's 250-line limit. See `.mem/summary.md:3`, `.mem/uptime/commands.md:1-25`, `.mem/uptime/requests.md:1-49`, `.mem/telemetry/sources.md:1-50`, `.mem/telemetry/logs.md:1-101`, and `.agents/skills/mem/SKILL.md:10-25,31`.

**Acceptance:** a review against current command routing and handlers finds no stale present-tense claim about active routes, credentials, endpoints, validation, help, response handling, or tests. `.mem/telemetry/summary.md` describes the implemented Telemetry domain and is linked from `.mem/mem-map.md`; the Telemetry link in `.mem/summary.md` points to `.mem/telemetry/summary.md`. Every changed memory file is at most 250 lines, and no Go toolchain PATH workaround appears in `.mem/`.

### R2 — Index every memory file

`.mem/mem-map.md` must link every Markdown memory file under `.mem/`, except scratch files under `.mem/tmp/`, which the mem skill designates as session scratch (`.agents/skills/mem/SKILL.md:20,41`), including itself, and must describe each link accurately. If the milestone adds a memory file, the map and related relative links must include it. See `.mem/mem-map.md:1-21`, `.agents/skills/mem/SKILL.md:20,30-31,41`.

**Acceptance:** enumerate all memory files and resolve every corresponding map link. Each file appears in the map, every local link resolves, and the map links itself. The map includes `.mem/telemetry/summary.md`.

### R3 — Match `TAGS.md` to all production code anchors

The anchor index must contain one accurate row for every `@anchor` occurrence in production code. It must not list removed, absent, or duplicate boundary occurrences. Keep the established surfaces and roles unless current code boundaries require a different existing classification. See `TAGS.md:3-22`, the source anchor locations in `cmd/bs/main.go:13,29,35`, `cmd/bs/logs.go:18,26,38`, `cmd/bs/sources.go:14,18,38`, `cmd/bs/monitors.go:15,29,55,81`, and `cmd/bs/incidents.go:50,82,332`.

**Acceptance:** run `grep -rn '@anchor' --exclude-dir=.git --exclude-dir=docs --exclude-dir=.wayfinder --exclude-dir=.mem --exclude=TAGS.md --exclude=AGENTS.md .` to enumerate occurrences across code, tests, and scripts. This excludes documentation under `docs/`, the `TAGS.md` index itself, the `AGENTS.md` instruction example, `.git/`, `.wayfinder/`, and `.mem/`. The command currently yields exactly 16 code anchors. Compare every result with `TAGS.md`; each occurrence has exactly one matching row, and every row resolves to the stated source and boundary.

### R4 — Document the added Telemetry commands

`README.md` must document the added source commands (`bs sources list|get`) and log-search command (`bs logs search`), with the supported argument and flag behavior needed to use them. It must retain the source-token secret warning, the full log table identifier and its relation to source lookup, option-shaped query behavior, and missing-hot-table guidance. See `README.md:5-42,60-87` and `.mem/telemetry/sources.md:14-34` plus `.mem/telemetry/logs.md:3-7,20-53`.

**Acceptance:** README examples and text name all three command forms and accurately state the behavior currently documented in their help and memory. The source-token warning and the `CLUSTER_DOESNT_EXIST` guidance remain present.

### R5 — Document all command environment variables

README must identify the separate Uptime API token, Telemetry API token, SQL URL, SQL username, and SQL password; explain which command family uses each; and state that SQL credentials do not fall back to either API token. It must preserve the fixed Telemetry REST host and approved SQL URL and response contract. See `README.md:44-64`, `.mem/uptime/requests.md:3-5`, and `.mem/telemetry/logs.md:73-97`.

**Acceptance:** README must identify the five variables as three groups (Uptime API token, Telemetry API token, and three SQL credentials), with each correct role. It must state that `BETTERSTACK_TELEMETRY_API_TOKEN` authenticates source commands and is used by `bs logs search` only to redact the token from SQL error output. Verify the current README text and add this statement if it is missing. Cross-check every `os.Getenv` and `os.LookupEnv` use in `cmd/bs` against README, distinguishing runtime configuration from test-only environment access. README makes no claim that conflicts with the behavior in ticket 02 as amended by tickets 10 and 11, including the parameterized query behavior in ticket 10 and SQL URL forms in ticket 10. See `README.md:44-64`, `cmd/bs/logs.go:92`, `.mem/uptime/requests.md:3-5`, `.wayfinder/telemetry/tickets/10-technical-decisions-from-live-verification.md:18,20`, and `.wayfinder/telemetry/tickets/11-sql-api-errors-concurrency-time-bounds.md:8-40`.

### R6 — Keep changelog fragments valid and preserve release-time reconciliation

Fragments must follow the repository format: a Keep a Changelog category heading, factual bullets for logical changes, and backticked repository-root paths. They must cover the source and log-search changes. Do not edit `CHANGELOG.md` or rewrite the 005 fragment. Add `changelog.d/008-telemetry-logs-search-007.md` under `### Changed`, with one bullet describing the 007 record updates. Do not add a bullet that reconciles the 005 and 006 route statements. The release skill owns reconciliation of accumulated entries, and the 006 fragment already states the route is active. See `AGENTS.md:39`, `.agents/skills/changelog/SKILL.md:11,15-27`, `changelog.d/008-telemetry-logs-search-005.md:1-3`, and `changelog.d/008-telemetry-logs-search-006.md:1-3`.

**Acceptance:** the 007 fragment is `changelog.d/008-telemetry-logs-search-007.md`, uses `### Changed`, and has exactly one factual bullet for the 007 record updates that cites backticked repository-root paths. Validate the 007 fragment against every criterion in the 001 R3 shape definition: the file exists; its first non-empty line is exactly one supported category heading; the next line is blank; it contains at least one non-empty bullet; every bullet states an actual change and includes a repository path; and it has no unrelated front matter, release section, or extra category heading. Also confirm there is exactly one bullet under `### Changed`. Apply the 001 R3 shape check to every fragment in `changelog.d/`, and confirm `changelog.d/008-telemetry-logs-search-003.md:3` and `changelog.d/008-telemetry-logs-search-006.md:3` still cover source and log-search changes for R6. See `docs/plans/008_telemetry-logs-search_001_spec.md:78`. Leave the 005 fragment unchanged. The release skill reconciles accumulated entries at release time, and the 006 fragment already states that the route is active. See `.agents/skills/changelog/SKILL.md:11` and `changelog.d/008-telemetry-logs-search-006.md:1-3`.

### R7 — Preserve provenance record conventions

`LOG.md` and `log.d/` must follow the provenance skill. Add no `LOG.md` entry or `log.d/` fragment unless the completion review finds rationale meeting `.agents/skills/provenance-log/SKILL.md:40-42`. Unless that gate finds qualifying rationale, keep `LOG.md` zero bytes and preserve the sole zero-byte `log.d/.gitkeep`. The tracked 001 specification states that “Git cannot preserve an empty `log.d/` directory in a commit” and requires the `.gitkeep` marker (`docs/plans/008_telemetry-logs-search_001_spec.md:26`), and says no rationale entry is needed when there is no non-obvious behavior change (`docs/plans/008_telemetry-logs-search_001_spec.md:84`). The tracked 006 specification says not to modify these records unless implementation introduces new, unsettled non-obvious rationale, and confirms that no change is needed for settled decisions (`docs/plans/008_telemetry-logs-search_006_spec.md:98,100`). See `.agents/skills/provenance-log/SKILL.md:40-42`.

**Acceptance:** add no `LOG.md` entry or `log.d/` fragment unless the completion review finds rationale meeting `.agents/skills/provenance-log/SKILL.md:40-42`. When no qualifying rationale is found, `LOG.md` remains zero bytes and `log.d/` holds only `.gitkeep`.

### R8 — Verify record links, format, and scope

The completed records must preserve existing cross-links and current repository vocabulary. Validation must check memory links and line limits, all anchor rows, README command/configuration coverage, changelog fragment format, and `LOG.md`/`log.d/` state. No code or protected baseline path may change. The earlier 004 F5 hook-comment observation remains out of scope. See `AGENTS.md:11-23`, `.mem/mem-map.md:1-21`, `.agents/skills/mem/SKILL.md:30`, and `docs/plans/008_telemetry-logs-search_004_checkpoint.md:464-465`.

**Acceptance:** `spec-lint` and `docs-lint` pass; direct record checks confirm R1–R7; the completion report names `make fragments-check` and `make docs-check` as unavailable because no Makefile exists, and records the 001 R3 changelog shape check and `docs-lint` as replacements. Run `go build -o /dev/null ./...` as a deliberate deviation from `docs/plans/008_telemetry-logs-search_milestones.md:55`'s `go build ./...`; the output path avoids writing a `bs` binary into the repository root. Also run `go vet ./...`, `go test ./...`, and `gofmt -l cmd/bs/*.go`, with `gofmt` printing no paths. The checkpoint report names the unavailable Make targets and replacement checks. `git status` shows only authorized 007 planning and repository-record changes plus the pre-existing untracked baseline. No protected path, code file, or milestone tracking document is changed, except for the named completion-gate checkbox render `- [ ] 007:` → `- [x] 007:`. No live API or credential is used.

## Requirement sources

- Milestone acceptance and decision links: `docs/plans/008_telemetry-logs-search_milestones.md:55`.
- Repository record decision and no-Makefile correction: `.wayfinder/telemetry/tickets/08-task-repo-record-gaps-mem-tags-log-changelog-makefile.md:11-25`.
- SQL parameter binding and accepted SQL URL forms: `.wayfinder/telemetry/tickets/10-technical-decisions-from-live-verification.md:18,20`; response and time-bound behavior: `.wayfinder/telemetry/tickets/11-sql-api-errors-concurrency-time-bounds.md:15-40`.
- Memory map indexing requirement: `.agents/skills/mem/SKILL.md:30`.
- Commands and surface: `.wayfinder/telemetry/tickets/01-charting-decisions-destination-scope-and-command-surface.md:11-23`.
- Credentials and initial request design: ticket 02 as amended by tickets 10 and 11, at `.wayfinder/telemetry/tickets/02-technical-design-decisions-recorded-at-charting.md:11-25`, `.wayfinder/telemetry/tickets/10-technical-decisions-from-live-verification.md:18,20`, and `.wayfinder/telemetry/tickets/11-sql-api-errors-concurrency-time-bounds.md:15-40`.
- SQL errors and time bounds: `.wayfinder/telemetry/tickets/11-sql-api-errors-concurrency-time-bounds.md:15-40`.
- Current source-token handling, SQL builder, and executor: `.mem/telemetry/sources.md:12-34` and `.mem/telemetry/logs.md:20-99`.
- Changelog category and fragment requirements: `AGENTS.md:39`, `.agents/skills/changelog/SKILL.md:15-27`; fragment-shape definition: `docs/plans/008_telemetry-logs-search_001_spec.md:78`.
- Provenance log decision rule and autonomous completion gate: `.agents/skills/provenance-log/SKILL.md:11-20,26-34,40-42`.
- Milestone verification commands: `docs/plans/008_telemetry-logs-search_milestones.md:47,55`.

## BLOCKED

None. The ticket decisions settle product behavior. The changelog correction follows the repository-local skill and does not require a product decision.
