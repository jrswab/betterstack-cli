# Milestone 001 Implementation Checklist — Installable `bs` command skeleton with three-level help

- Spec: `docs/plans/001_betterstack-cli-first-release_001_spec.md`
- Milestone source: `docs/plans/001_betterstack-cli-first-release_milestones.md`, entry `001`
- Executable spec requirements: R1.1–R1.3, R2.1–R2.2, R3.1–R3.3, R4.1–R4.6, R5.1–R5.5, R6.1–R6.2

## Section 1: Context Summary

Milestone 001 establishes the installable skeleton that every later `bs` slice builds on: a dependency-free Go module `github.com/jrswab/betterstack-cli` with a `go 1.26` directive and no `toolchain` directive, an installable command package `cmd/bs` producing the executable `bs`, and the complete three-level human-readable help surface (root, resource, per-command) with usage lines, positional placeholders (`<id-or-url>`, `<id>`), the settled first-release flag surface in help text, and examples using approved flag spellings. Milestone 001 implements no command runtime behavior: `--help` forms exit 0, and every other invocation exits 1 with a readable stderr error, with no network request and no credential reading (credential rules arrive with milestone 002). Builds and verification run with `GOTOOLCHAIN=go1.26.9` from the local module cache because the local default go1.22.2 is below the module minimum. Pre-existing files (`.wayfinder/`, `docs/plans/`, `verify-incident-id.sh`, `LICENSE`, `README.md`, `.gitignore`) stay untouched.

Per the spec (section 1.8), milestone 001 has no test-framework requirement; each task below is verified by executing the built or installed binary at the CLI argv seam (the public interface), per the tdd skill's seam rule with the seams already fixed by the spec's acceptance boundaries.

## Section 2: Implementation Checklist

### Slice A — Module foundation (enables Slices B–E: the command surface and help all build on the installable skeleton)

- [x] T1: Create `go.mod` at the repository root with module `github.com/jrswab/betterstack-cli`, directive `go 1.26`, no `toolchain` line, and no `require` entries — spec R1.1. Verify: `GOTOOLCHAIN=go1.26.9 go list -m` prints `github.com/jrswab/betterstack-cli`; the file contains `go 1.26` and contains neither `toolchain` nor `require`.
- [x] T2: Create `cmd/bs/main.go` — `package main`, `func main()` — that builds an executable named `bs` and initially implements the 001 default: any invocation prints a readable invalid-usage error to stderr and exits 1 (this is R5.2/R5.3 baseline; later tasks carve out the help forms). Verify: `GOTOOLCHAIN=go1.26.9 go build ./cmd/bs` exits 0 and produces `bs`; running `./bs` exits 1 with a message on stderr and nothing on stdout.

### Slice B — Root and resource help (spec R2, R3; depends on Slice A)

- [x] T3: Implement root help in `cmd/bs/main.go` (+ help content in `cmd/bs/help.go`): argv exactly `--help` prints human-readable text to stdout listing the resource groups `incidents` and `monitors` each with a one-line description, exits 0, writes nothing to stderr — spec R2.1/R2.2. Verify with the built binary: `./bs --help` exits 0; stdout contains `incidents` and `monitors`; stderr empty; stdout is not JSON.
- [x] T4: Implement resource help: argv `incidents --help` prints help listing `list`, `get`, `timeline` with usage shapes; argv `monitors --help` lists `list`, `get`; both exit 0, stdout only, listing exactly the resource's commands — spec R3.1–R3.3. Verify both invocations with the built binary (exit codes, command lists, empty stderr).

### Slice C — Per-command help (spec R4; depends on Slice A; T5–T9 are mutually parallel)

- [x] T5: Per-command help for `bs incidents list --help` in `cmd/bs/help.go` (routed via `cmd/bs/main.go`): usage line `bs incidents list` (no positional target), arguments section stating no positional target, flags section documenting exactly `--from`, `--to`, `--monitor-id`, `--heartbeat-id`, `--resolved` (explicit `true`/`false`), `--acknowledged` (explicit `true`/`false`), `--metadata` (repeatable, one `key=value` per occurrence), `--team-name`, `--page`, `--per-page`, `--timeout` (default 30 seconds), plus at least one example with approved spellings (e.g. `bs incidents list --monitor-id 123 --resolved=false`) — spec R4.1–R4.5. Verify: exit 0, stdout sections present, no excluded flag (no `--query`, `--all`, `--pretty`, typed metadata variants), stderr empty.
- [x] T6: Per-command help for `bs incidents get --help`: usage line `bs incidents get <id-or-url>`, arguments section stating the target accepts an incident ID (including the bare numeric form) or a full dashboard URL of the form `https://incidents.betterstack.com/team/<team-id>/incidents/<incident-id>`, flags section documenting `--timeout` (default 30 seconds), example `bs incidents get 123 --timeout 60s` — spec R4.1–R4.5. Verify: exit 0, placeholder exact, no monitor/other URL form claimed, stderr empty.
- [x] T7: Per-command help for `bs incidents timeline --help`: usage line `bs incidents timeline <id-or-url>`, same arguments rule as T6, flags `--timeout` (default 30 seconds), example — spec R4.1–R4.5. Verify: exit 0, placeholder exact, stderr empty.
- [x] T8: Per-command help for `bs monitors list --help`: usage line `bs monitors list` (no positional target), flags `--url`, `--pronounceable-name`, `--team-name`, `--page`, `--per-page`, `--timeout` (default 30 seconds), example with approved spellings (e.g. `bs monitors list --pronounceable-name Example`) — spec R4.1–R4.5. Verify: exit 0, no monitor URL form anywhere, stderr empty.
- [x] T9: Per-command help for `bs monitors get --help`: usage line `bs monitors get <id>`, arguments section stating a bare monitor ID only (no URL form), flags `--timeout` (default 30 seconds), example (e.g. `bs monitors get 789`) — spec R4.1–R4.5. Verify: exit 0, placeholder exact, stderr empty.

### Slice D — Full routing, exit codes, and independence (spec R5; depends on Slices B–C)

- [x] T10: Complete the routing decision in `cmd/bs/main.go` so that exactly the eight argv forms of spec R5.1 (`--help`; `incidents --help`; `monitors --help`; `incidents list --help`; `incidents get --help`; `incidents timeline --help`; `monitors list --help`; `monitors get --help`) print their help level to stdout and exit 0, and every other argument vector prints one readable error to stderr naming what was wrong and pointing to the relevant `--help`, writes nothing to stdout, and exits 1 (R5.2/R5.3). Verify with the built binary across the full matrix: the eight forms exit 0; bare `bs`, `bs incidents`, `bs uptime list`, `bs incidents delete`, `bs --help incidents`, `bs incidents get 123 --help`, `./bs -h`, `bs incidents get`, `bs monitors get 1 2`, `bs incidents get 123`, `bs monitors list` each exit 1 with a readable stderr message and empty stdout (R5.2 acceptance list).
- [x] T11: Verify exit-code and channel hygiene plus independence (R5.4/R5.5): run the full R5.1/R5.2 matrix and confirm only exit codes 0 and 1 appear; help never writes to stderr; errors never write to stdout. Then run the identical matrix twice — with `BETTERSTACK_UPTIME_API_TOKEN` unset and set to a dummy value — and compare exit codes and stdout/stderr byte-for-byte (must be identical). Optional stronger check per spec: rerun with `HTTP_PROXY=http://127.0.0.1:1 HTTPS_PROXY=http://127.0.0.1:1` and confirm no outcome changes.

### Slice E — Milestone acceptance and preservation (spec R1.2 acceptance, R6; depends on Slice D)

- [x] T12: Clean-checkout install smoke test: `GOTOOLCHAIN=go1.26.9 go install ./cmd/bs` exits 0 from the checkout, and the installed `bs` binary serves the acceptance set: `bs --help`, `bs incidents --help`, `bs monitors --help`, and one per-command help (e.g. `bs incidents get --help`) all exit 0 with human-readable stdout — spec R1.2 acceptance and the milestone's acceptance wording.
- [x] T13: Preservation audit (R6.1/R6.2): confirm `git status --porcelain` shows only new files (`go.mod`, `cmd/bs/**`, plus the planning artifacts under `docs/plans/`), that no pre-existing path (`.wayfinder/`, `docs/plans/001_betterstack-cli-first-release_milestones.md`, `verify-incident-id.sh`, `LICENSE`, `README.md`, `.gitignore`) is modified or deleted, and that no tag, push, or publication action was performed.

### Parallelism

T1 must complete first; T2 depends on T1. Within Slices B–C, T3, T4, and T5–T9 are mutually parallel after T2. T10 depends on T3–T9. T11 depends on T10. T12 and T13 depend on T11 (T12 and T13 are mutually parallel).
