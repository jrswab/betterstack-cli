# Milestone 001 Specification — S0 repository records for the Uptime CLI

- Milestone source: `docs/plans/008_telemetry-logs-search_milestones.md`, entry `001` (S0)
- Scope: milestone 001 only. This specification covers repository records for the current Uptime CLI. It authorizes no Telemetry product behavior.
- Acceptance in one line: `.mem/` describes the current Uptime CLI, eight source anchors match `TAGS.md`, `changelog.d/` contains one valid working-log fragment, `LOG.md` and `log.d/` contain no provenance entries, and no Makefile or unrelated behavior changes.

## Section 1: Context & Constraints

### Repository and product state

- The repository is a standard-library Go CLI. The executable is `bs`, and the command package is `cmd/bs`. The current product implements Uptime API commands, not Telemetry commands. Sources: `go.mod`, `README.md`, `cmd/bs/main.go`, `cmd/bs/monitors.go`, `cmd/bs/incidents.go`, and `cmd/bs/help.go`.
- `runWithAPI(args, stdout, stderr, baseURL, client)` is the routing and dependency-injection boundary. It routes five implemented commands: monitors list/get and incidents list/get/timeline. The current command implementations are `monitorsList`, `monitorsGet`, `incidentsList`, `incidentsGet`, and `incidentsTimeline`.
- `doMonitorsRequest` is the shared HTTP request executor. It sends one bearer-authenticated GET, applies the 30-second default timeout unless a command overrides it, does not follow redirects, writes 2xx response bodies unchanged to stdout, and writes non-2xx response bodies unchanged to stderr. Local diagnostic paths redact the Uptime token.
- Incident URL parsing and incident-list argument translation have distinct behavior worth documenting. `resolveIncidentTarget` accepts a bare target or the observed dashboard URL form and rejects other schemeful URL forms locally. Incident-list metadata accepts repeatable `key=value` values, rejects duplicate keys before a request, and maps them to nested API query parameters.
- Tests use `runWithAPI` and loopback `httptest` servers. `cmd/bs/safety_test.go` clears inherited credentials and installs a loopback-only network guard. Test credentials are fixed dummy values. These protections must remain unchanged.
- `README.md` contains only a project title and a generic description. No repository memory, anchor index, changelog fragments, provenance log, provenance directory, or Makefile exists. No source file currently has an `@anchor` tag. Git tracks files, not empty directories.

### Decisions and constraints

- Ticket 08 is closed and records Jaron Swab's choice of Option 1: establish records for the existing Uptime CLI before any Telemetry implementation. It calls for the memory layout, anchors at the routing, Uptime handler, and HTTP executor boundaries, `TAGS.md`, changelog fragments, an empty `LOG.md`, and `log.d/`. It excludes a Makefile.
- Ticket 08 corrects the premise that `AGENTS.md` requires a Makefile. `AGENTS.md` names `make help` as one discovery option but does not require a Makefile or target list. The repository has no Makefile. S0 must not add one.
- Ticket 08 rejects creating records only for Telemetry code and rejects weakening `AGENTS.md`. S0 documents and tags the complete current Uptime surface. The stale `AGENTS.md` reference to `internal/handlers/gloocode/prompts/` is outside scope and remains untouched.
- The `mem` skill requires `.mem/summary.md`, `.mem/terminology.md`, `.mem/practices.md`, `.mem/mem-map.md`, and focused domain files. Topic files describe present behavior, include a code example, and include a Mermaid diagram where flow or structure matters. Each file must remain at or below 250 lines, and every memory file must be indexed from `.mem/mem-map.md`.
- The changelog skill requires a `changelog.d/<ticket>.md` fragment, not an edit to `CHANGELOG.md`. A fragment begins with one `### <Category>` heading, a blank line, and one or more factual bullets. Categories are `Added`, `Changed`, `Deprecated`, `Removed`, `Fixed`, and `Security`. The skill's usual `make fragments-check` target and referenced `changelog.d/README.md` are absent here. S0 therefore defines the fragment name and performs direct format and content checks without claiming equivalence to an unavailable parser.
- The `mem` skill names `make docs-check` as its completion check. That target is also unavailable, and the no-Makefile decision is settled. S0 uses direct checks for line limits, required memory files, map links, anchor inventory, changelog shape, and empty provenance records. It does not add a Makefile to satisfy either absent target.
- `LOG.md` can be a zero-byte tracked file. Git cannot preserve an empty `log.d/` directory in a commit. S0 will keep `log.d/` durable with a zero-byte `log.d/.gitkeep` marker. In this specification, “empty” means that `LOG.md` has zero bytes and `log.d/` contains no provenance entries; the marker is directory bookkeeping, not a log entry.
- The source milestone says planning does not authorize commits. The user's Factory invocation authorizes local commits on `master`. No push, live API request, or credential use is authorized.

### Settled scope boundary

- S0 creates only Uptime memory and repository records, anchor comments on the listed product boundaries, and documentation needed to index those anchors and fragments. It does not add Telemetry code, change runtime behavior, create a Makefile, or update `README.md`.
- The setup baseline contains untracked `.agents/`, `.wayfinder/`, `AGENTS.md`, the source milestone document, and `verify-incident-id.sh`. These paths are pre-existing and must remain untouched and unstaged. The source milestone is itself untracked, so no checkbox-rendering edit is part of this run.
- The exact prior-plan naming convention is `{plan}_{milestone}_{slug}_{artifact}.md`. The artifact paths for this milestone are `docs/plans/008_telemetry-logs-search_001_spec.md` and `docs/plans/008_telemetry-logs-search_001_checkpoint.md`.

## Section 2: Requirements

### R1 — Create a complete, current Uptime memory set

Create exactly these nine Markdown files under `.mem/`:

1. `.mem/summary.md`
2. `.mem/terminology.md`
3. `.mem/practices.md`
4. `.mem/mem-map.md`
5. `.mem/uptime/summary.md`
6. `.mem/uptime/commands.md`
7. `.mem/uptime/requests.md`
8. `.mem/uptime/incidents.md`
9. `.mem/uptime/testing.md`

The memory content must describe the current code, not planned Telemetry behavior. It must cover the five routed commands and help boundary, the shared request and response contract, incident URL and metadata rules, credential handling, and the loopback-only test protections. Each topic file must contain at least one accurate code example. The Uptime domain summary must include a Mermaid flow diagram for command routing and the shared request path. Every memory file must be linked from `.mem/mem-map.md` and related topic files must use relative links. Every file must be 250 lines or fewer.

**Acceptance:** all nine paths exist; the map links each path; links resolve to files tracked by Git (`git ls-files --error-unmatch`); each topic file contains a code example; the Uptime summary has a Mermaid diagram; and a line-count check reports no file above 250 lines. The content agrees with the listed current source files.

### R2 — Add and index the Uptime boundary anchors

Add exactly eight production-code anchors using the repository syntax `// @anchor:<surface>:<role>`. Review finding F4 adds the configuration boundary because `AGENTS.md` requires tags on configuration boundaries.

| Anchor | Boundary | Source location |
|---|---|---|
| `uptime:router` | Command routing boundary | `cmd/bs/main.go`, `runWithAPI` |
| `uptime:config` | Uptime configuration boundary | `cmd/bs/monitors.go`, const block: `defaultAPIBaseURL`, `defaultTimeout`, `tokenEnvVar` |
| `uptime:command-handler` | Monitors list command handler | `cmd/bs/monitors.go`, `monitorsList` |
| `uptime:command-handler` | Monitors get command handler | `cmd/bs/monitors.go`, `monitorsGet` |
| `uptime:request-executor` | Request executor | `cmd/bs/monitors.go`, `doMonitorsRequest` |
| `uptime:command-handler` | Incidents list command handler | `cmd/bs/incidents.go`, `incidentsList` |
| `uptime:command-handler` | Incidents get command handler | `cmd/bs/incidents.go`, `incidentsGet` |
| `uptime:command-handler` | Incidents timeline command handler | `cmd/bs/incidents.go`, `incidentsTimeline` |

Use the `uptime` surface for both resource groups because all five commands currently call the Uptime API. Put each anchor directly above its boundary declaration, including the configuration const block, not on helpers or tests. The `uptime:command-handler` tag intentionally appears once on each of five distinct command functions. Create root `TAGS.md` as a Markdown index with exactly eight occurrence entries, one per tagged boundary. Each entry names its source file, symbol or constants, and boundary, even when the tag label repeats. Do not add duplicate tags to any one boundary.

**Acceptance:** a source search finds exactly eight anchor occurrences matching the rows above. Each tag is attached once to its named function or const block. `TAGS.md` has exactly eight occurrence entries, including five distinct entries for `uptime:command-handler`, with no missing or stale entry.

### R3 — Add one valid changelog fragment

Create `changelog.d/008-telemetry-logs-search-001.md`. This name uses the plan identifier, plan slug, and milestone identifier because S0 has no separate issue or pull-request number. The fragment must start with `### Added`, then a blank line, then one factual bullet that describes the repository records added for the Uptime CLI and cites plain backticked repo-root paths. Do not use relative Markdown links because collection moves the bullet into root `CHANGELOG.md`. It must not claim Telemetry runtime behavior.

For S0, a valid fragment means: the named file exists; its first non-empty line is exactly one supported category heading; the next line is blank; it contains at least one non-empty bullet; each bullet states an actual change and includes a repository path; and it contains no unrelated front matter, release section, or extra category heading. This is a direct content check, not a claim that the unavailable `fragments-check` parser passed.

**Acceptance:** the named fragment meets each defined shape and content check; `CHANGELOG.md` is not changed; no other fragment is needed for this single logical records change.

### R4 — Establish empty provenance records

Create root `LOG.md` as a zero-byte file. Create `log.d/` with only the zero-byte `.gitkeep` marker and no provenance entries. Do not add a rationale entry because S0 adds no non-obvious behavior change that needs historical explanation.

**Acceptance:** `test ! -s LOG.md` succeeds; `log.d/` exists; a listing shows only `.gitkeep`; and the directory marker is tracked so the directory survives checkout.

### R5 — Preserve scope and existing behavior

S0 must not change runtime behavior, command routing, help text, API behavior, tests, credentials, endpoints, or network access. It must not add Telemetry product code, a Makefile, or changes to `README.md`. It must not modify, stage, or delete any pre-existing baseline path. Only the records in R1–R4 and the eight anchor comments in R2 are in scope.

**Acceptance:** the S0 implementation changes no paths outside the specified memory files, `TAGS.md`, the specified changelog fragment, `LOG.md`, `log.d/.gitkeep`, and the three named Go source files. The Go source diff consists only of the eight anchor comments. No Makefile or README change exists. No baseline path is staged or modified. This Phase 1 Factory commit contains only the two planning artifacts requested for this task.

### R6 — Verify the repository and Go code

Run the milestone's required checks after the records and anchors are complete: `go build ./...`, `go vet ./...`, `go test ./...`, and `gofmt -l cmd/bs/*.go`. The Go checks must pass, and `gofmt -l` must print no paths. Also run the direct acceptance checks in R1–R4, `git diff --check`, and an explicit staged-path audit before commit. Do not run live API calls or use credentials.

**Acceptance:** all four milestone commands exit successfully; `gofmt -l cmd/bs/*.go` is empty; direct checks pass; `git diff --check` is clean; and the staged path list contains only the two requested planning artifacts at the specification/checkpoint stage, then only the S0 implementation paths at the later execution stage.

### R7 — Record limitations and preserve the milestone boundary

The checkpoint must record actual commands and output, the baseline status verbatim, the specification phase result, and the next safe phase. It must state that `make help`, `make docs-check`, and `make fragments-check` are unavailable because no Makefile exists and S0 does not add one. The checkpoint must distinguish verified results from checks that could not run. The milestone source checkbox remains unchecked during specification and implementation; it is an untracked baseline file and is outside this run's write scope.

**Acceptance:** checkpoint evidence matches the actual run and does not claim an unavailable Make target passed. No unresolved specification question remains. No Telemetry implementation or live check is included.

### Parallelism

R1–R4 define independent record content, but their cross-links and acceptance checks require one final consistency pass. R2 depends on the current source boundaries listed in the table. R5–R7 apply to the entire milestone. The specification and checkpoint setup artifacts are the only files written in this Phase 1 task.

### Verification notes for the executor

- Read current source before writing memory descriptions or placing each anchor. Do not rely on memory content in place of source.
- Check exact anchor inventory by searching only production files under `cmd/bs/*.go`; tests remain untagged.
- Validate Markdown links against existing files tracked by Git (`git ls-files --error-unmatch`). Check all nine memory paths, per-file line limits, code examples, and the Mermaid fence directly because no `make docs-check` target exists.
- Validate the single changelog fragment against the shape defined in R3. Do not report `make fragments-check` as passed.
- The selected `log.d/.gitkeep` is a Git-preservation marker. “Empty” excludes this marker and means no provenance entries.
- Do not edit the milestone source to render its checkbox. The source document and all other listed baseline paths are untracked user artifacts.
- Keep all Git staging explicit. Never stage `.` or use `git add -A`.
