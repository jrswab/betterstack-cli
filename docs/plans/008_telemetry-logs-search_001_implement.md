# Milestone 001 Implementation Checklist — S0 repository records for the Uptime CLI

- Spec: `docs/plans/008_telemetry-logs-search_001_spec.md`
- Milestone source: `docs/plans/008_telemetry-logs-search_milestones.md`, entry `001`
- Scope: milestone 001 only; no Telemetry product behavior
- Requirement coverage: R1 → A1; R2 → A2; R3 → A3; R4 → A4; R5–R7 → V1 and the constraints on every task

## Section 1: Context Summary

Milestone 001 establishes repository records for the current standard-library Uptime CLI before Telemetry work begins. The CLI routes five implemented commands through `cmd/bs/main.go: runWithAPI`; command handlers in `monitors.go` and `incidents.go` share `doMonitorsRequest`, while `helpFor` and `cmd/bs/help.go` define the current help boundary. This milestone documents the existing behavior, adds boundary anchors, and creates the specified repository records only. It does not change runtime behavior, tests, credentials, endpoints, help text, `README.md`, or add a Makefile. Preserve the pre-existing untracked paths `.agents/`, `.wayfinder/`, `AGENTS.md`, `docs/plans/008_telemetry-logs-search_milestones.md`, and `verify-incident-id.sh` without modifying, staging, or deleting them. Do not make live requests or use credentials.

## Section 2: Implementation Checklist

The checkpoint is the source of task state. Task boxes render its passed gates. Tasks A1–A4 can be implemented independently, but V1 must follow them and check consistency across the records. No task changes test code. The Command routing boundary and credential-clearing network guard remain unchanged.

### Slice A — Current Uptime repository records

- [x] A1: Create exactly the nine memory files specified in R1: `.mem/summary.md`, `.mem/terminology.md`, `.mem/practices.md`, `.mem/mem-map.md`, and `.mem/uptime/{summary,commands,requests,incidents,testing}.md`. Describe present code only, including all five routed commands and help boundary, request/response behavior, incident target and metadata rules, credential handling, and loopback-only test protections. Include an accurate code example in every topic file, a Mermaid flow diagram in `.mem/uptime/summary.md`, relative links between related topics, and an index entry for every memory file in `.mem/mem-map.md`. Keep every file at or below 250 lines. Do not describe planned Telemetry behavior. Verify: inspect each memory file against `cmd/bs/main.go` (`runWithAPI`, `helpFor`), `cmd/bs/help.go`, `cmd/bs/monitors.go` (`monitorsList`, `monitorsGet`, `doMonitorsRequest`), `cmd/bs/incidents.go` (`incidentsList`, `incidentsGet`, `incidentsTimeline`, `resolveIncidentTarget`, metadata parsing), and relevant `cmd/bs/*_test.go`; check all nine paths exist; check every map link resolves to a file tracked by Git (`git ls-files --error-unmatch`); check each topic has a code example; check the Uptime summary's Mermaid fence; run `wc -l` over all nine files and confirm no count exceeds 250; run `go build ./...`, `go vet ./...`, and `go test ./...`.

- [x] A2: Add exactly eight production anchors, each on the named boundary declaration and nowhere in tests: `uptime:router` at `cmd/bs/main.go: runWithAPI`; `uptime:config` directly above the `cmd/bs/monitors.go` const block for `defaultAPIBaseURL`, `defaultTimeout`, and `tokenEnvVar`; `uptime:command-handler` at `cmd/bs/monitors.go: monitorsList` and `monitorsGet`; `uptime:request-executor` at `cmd/bs/monitors.go: doMonitorsRequest`; and `uptime:command-handler` at `cmd/bs/incidents.go: incidentsList`, `incidentsGet`, and `incidentsTimeline`. Review finding F4 adds configuration tagging required by `AGENTS.md`. Create root `TAGS.md` with exactly eight occurrence entries, each naming the file, symbol or constants, and boundary. Verify: search only `cmd/bs/*.go` production source for anchors and compare the eight occurrences with the specified boundary inventory; confirm every tag is attached to its named declaration; confirm `TAGS.md` has eight matching occurrence entries with no missing or stale entry; inspect `git diff -- cmd/bs/main.go cmd/bs/monitors.go cmd/bs/incidents.go` and confirm the only source changes are these comments; run `go build ./...`, `go vet ./...`, `go test ./...`, and `gofmt -l cmd/bs/*.go` (expect no paths).

- [x] A3: Create only `changelog.d/008-telemetry-logs-search-001.md`. Use one `### Added` heading, a blank line, and at least one factual bullet that describes the Uptime repository records and cites plain backticked repo-root paths, not relative Markdown links. Do not claim Telemetry runtime behavior or edit `CHANGELOG.md`. Verify: confirm the file exists; confirm its first non-empty line is exactly `### Added`; confirm the following line is blank; confirm at least one non-empty bullet exists and every bullet states an actual change with a repository path; confirm no front matter, release section, or second category heading exists; run `go build ./...`, `go vet ./...`, and `go test ./...`. The unavailable `make fragments-check` is not a passing check.

- [x] A4: Create zero-byte root `LOG.md` and `log.d/.gitkeep`; add no provenance entry. Verify: `test ! -s LOG.md`; confirm `log.d/` exists and its only entry is the zero-byte `.gitkeep`; confirm Git tracks the marker; run `go build ./...`, `go vet ./...`, and `go test ./...`.

### Slice B — Acceptance and preservation gate

- [x] V1: Check the completed records against every acceptance clause in spec R1–R7. Run the complete direct checks for R1–R4: all nine memory paths and map links, link resolution to Git-tracked files, code examples, Mermaid diagram, 250-line maximum, exact eight-anchor inventory and matching `TAGS.md` entries, changelog shape/content, zero-byte `LOG.md`, and `log.d/` containing only `.gitkeep`. Run `go build ./...`, `go vet ./...`, `go test ./...`, and `gofmt -l cmd/bs/*.go` (expect no paths). Run `git diff --check` and inspect the exact changed-path list. Confirm runtime behavior, tests, credentials, endpoints, help text, `README.md`, and Makefile remain unchanged; the source diff has only the eight anchor comments; and all five pre-existing untracked baseline paths remain untouched and unstaged. Record actual command output and distinguish unavailable Make targets from completed direct checks in the checkpoint. Verify that no live request or credential use occurred. Do not render the source milestone checkbox.

## Requirement Coverage Map

| Spec requirement | Checklist coverage |
|---|---|
| R1 — Complete current Uptime memory set | A1 creates and verifies all nine memory files, content, links, examples, diagram, and line limits; V1 repeats the acceptance audit. |
| R2 — Eight source anchors and exact index | A2 installs and verifies all eight production anchors and eight `TAGS.md` entries; V1 checks exact inventory and source-only diff. |
| R3 — One valid changelog fragment | A3 creates and verifies the specified fragment shape and content; V1 repeats the direct content check. |
| R4 — Empty provenance records | A4 creates and verifies zero-byte `LOG.md` and marker-only `log.d/`; V1 repeats the checks. |
| R5 — Scope and existing behavior | Every task follows the scope limits; V1 audits changed paths, source-only anchor comments, baseline preservation, and absence of unrelated changes. |
| R6 — Go and direct verification | A1–A4 list applicable Go checks; A2 and V1 include `gofmt -l cmd/bs/*.go`; V1 runs the full Go suite and direct R1–R4 checks plus `git diff --check`. |
| R7 — Checkpoint, limitations, and milestone boundary | V1 requires actual evidence, limitation reporting for unavailable Make targets, no unresolved question, no live checks, and an unchecked milestone source entry. |

## Parallelism

A1–A4 can proceed in parallel because they create separate record surfaces. V1 depends on all four tasks because it checks cross-record consistency, complete scope, and final acceptance. The Go verification commands can run after each task and again as the complete V1 gate. No implementation task authorizes a live request, credential use, push, or edit to a baseline path.
