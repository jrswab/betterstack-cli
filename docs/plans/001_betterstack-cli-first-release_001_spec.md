# Milestone 001 Specification — Installable `bs` command skeleton with three-level help

- Milestone source: `docs/plans/001_betterstack-cli-first-release_milestones.md`, entry `- [ ] 001:`
- Scope: milestone 001 only. Requirements for milestones 002–007 appear here only as context or exclusions; nothing in this spec requires them.
- Acceptance in one line: a clean local checkout passes `go install ./cmd/bs`, the installed `bs` serves all three help levels (root, resource, per-command), `--help` exits 0, invalid usage exits 1 with readable errors, and no invocation makes an API call.

## Section 1: Context & Constraints

### 1.1 Repository state (grounded)

- The repository `github.com/jrswab/betterstack-cli` (origin `git@github.com:jrswab/betterstack-cli.git`, private) contains no code and no `go.mod`. The root holds only `LICENSE`, `README.md`, `.gitignore`, the `.wayfinder/` planning tree, `docs/plans/`, and `verify-incident-id.sh` (verified in the setup checkpoint at HEAD `a6f5ed7879ff7ace05d692899233e1bc8c6b1893`).
- `verify-incident-id.sh` is a one-shot user helper. It stays untouched at the root. It is not part of the CLI build, and no milestone removes or absorbs it (milestone research, Section 1 "Codebase structure and current state").

### 1.2 Module, layout, and toolchain (ticket 06 and asset `06-go-toolchain-research.md`)

- Module path: `github.com/jrswab/betterstack-cli`, derived from the origin remote. (Ticket 06, Decision 1.)
- Installable command package at `cmd/bs` — the conventional `cmd/<progname>/main.go` layout (https://go.dev/doc/modules/layout). The executable name is `bs` (ticket 03, first approval).
- Install forms: the local smoke-test form is `go install ./cmd/bs`; the intended first-release remote form is `go install github.com/jrswab/betterstack-cli/cmd/bs@v0.1.0`. The remote form is recorded only; it is a conditional post-tag release action requiring separate future user authorization and is not executed by this milestone. (Ticket 06, Decisions 1 and 3.)
- The `go.mod` declares `go 1.26` and has no `toolchain` directive. Rationale: Go 1.26 is the oldest release inside the official support window on 2026-10-17, and no `toolchain` directive is needed for a dependency-free module (ticket 06, Decision 2; asset 06, sections 2 and 8; https://go.dev/doc/toolchain).
- The `go` directive declares the minimum Go version required to compile the module; since Go 1.21 the go command auto-switches toolchains when the module requires a newer one (asset 06, section 2; https://go.dev/doc/toolchain).
- Toolchain constraint: the local default toolchain is `go1.22.2 linux/amd64`, below the minimum. Go 1.26.9 is available from the local module cache via `GOTOOLCHAIN=go1.26.9` (setup checkpoint evidence). Every build and verification command in this milestone must run with a Go toolchain of version 1.26 or newer.
- The module is dependency-free in this milestone: standard library only, no third-party `require` entries. This matches the dependency-free rationale behind the "no `toolchain` directive" decision (asset 06, section 8).

### 1.3 Command surface and help conventions (ticket 03, approved by Jaron 2026-10-16)

- Executable name: `bs`. Resource-first, two levels: `bs <resource> <action>`. Exactly five first-release commands:
  - `bs incidents list`
  - `bs incidents get <id-or-url>`
  - `bs incidents timeline <id-or-url>`
  - `bs monitors list`
  - `bs monitors get <id>`
- Targets are positional. Incident commands accept an incident ID (including the bare numeric form) or a full dashboard incident URL. Monitor commands take a bare monitor ID only; no monitor URL form exists in the first release.
- Help output is human-readable text — the one explicit, agreed exception to the JSON-only output agreement, scoped to help only (ticket 03, eighth approval). Successful API responses stay JSON; milestone 001 makes no API calls, so no JSON output exists yet.
- Three help levels: `bs --help` lists the resource groups (`incidents`, `monitors`) with one-line descriptions; resource-level help (`bs incidents --help`, `bs monitors --help`) lists each resource's commands; each command's `--help` shows a usage line, the arguments, the flags, and examples. Usage lines document positional targets with placeholders, for example `bs incidents get <id-or-url>` and `bs monitors get <id>`.
- Examples in per-command help show the approved flag spellings in place, so the flag conventions stay visible at the call site (ticket 03, eighth approval rationale).
- Future naming consistency (recorded, nothing charted): lowercase plural resource groups, explicit verbs such as `list` and `get`, one positional target per command, hyphenated flags mapping onto underscore API parameters. (Ticket 03, ninth approval.)

### 1.4 Flag surface for help content (tickets 03, 04, 05 — settled, never re-evaluated)

Milestone 001 requires per-command help to include flags and examples (milestone 001 text), and ticket 03 fixes that help examples show the approved flag spellings. The settled first-release flag surface per command is:

- `bs incidents list`: `--from`, `--to`, `--monitor-id`, `--heartbeat-id`, `--resolved` (explicit `true`/`false` required), `--acknowledged` (explicit `true`/`false` required), `--metadata` (repeatable, one `key=value` pair per occurrence), `--team-name` (optional, passed through unchanged, no token-type detection), `--page`, `--per-page`.
- `bs monitors list`: `--url`, `--pronounceable-name`, `--team-name`, `--page`, `--per-page`.
- All five commands: `--timeout` accepting a duration value, default 30 seconds (ticket 05: the timeout is overridable per invocation).
- Excluded from the first release — help must not document any of these: the typed metadata variants `metadata[key][][type]` and `metadata[key][][email]` (ticket 03, fifth approval), a `--query` raw passthrough (ticket 03, second approval and rejected approaches), an `--all` auto-drain flag (ticket 04), `--pretty` or compact modes (ticket 04), and any monitor URL form (ticket 03 resolution).
- The flags' runtime behavior (query translation, pagination, timeout enforcement) is delivered by milestones 002–006. Milestone 001 documents this surface in help; it does not implement it.

### 1.5 Exit codes and error shape (ticket 05; milestone 001)

- Exit code 0 on success, exit 1 on every failure, including invalid command usage. No other exit codes exist (ticket 05 resolution).
- `--help` exits 0 — the conventional technical implementation default for help output, not a separately approved human decision (milestone 001 wording).

### 1.6 Milestone 001 stage boundary

- Milestone 001 delivers the skeleton: module layout plus the complete help surface. Command runtime behavior (requests, credential handling, filters, pagination, error passthrough) is delivered by milestones 002–006 and is out of scope here.
- In milestone 001, no invocation performs a network request and no invocation reads `BETTERSTACK_UPTIME_API_TOKEN` or any credential source. The credential rules of milestone 002 (missing-token pre-request failure, no `BETTERSTACK_API_TOKEN` fallback) do not apply to milestone 001's help and error paths.

### 1.7 Settled interpretations for milestone 001

These interpretations apply the milestone text to the skeleton stage. They are grounded in the sources above and are not open questions:

- Per-command help documents each command's agreed first-release flags and examples using the approved spellings (milestone 001 lists "flags, and examples" as required help content; ticket 03, eighth approval). The flags are documentation of the fixed command surface that later slices build on; their runtime behavior is not implemented in milestone 001.
- In milestone 001, any invocation that is not a recognized help invocation exits 1 with a readable error. This covers invalid usage and — because no command behavior exists yet — well-formed non-help invocations of the five commands, such as `bs incidents get 123`. The error must not simulate API behavior: it prints no fake response body, prints no token-related message, and makes no request.
- Bare `bs` (no arguments) is invalid usage: exit 1, with a readable error that points to `bs --help`.
- The raw-passthrough rules of ticket 04 (byte-for-byte body, no appended newline) apply to API response bodies, not to help text. Help text formatting, line width, and trailing newlines are free.
- Exact message and help wording is an implementation decision (the milestone document delegates "exact message wording" to the downstream phases).

### 1.8 Out of scope for milestone 001

- All command runtime behavior: API requests, credential or environment reading, filter and pagination logic, timeout enforcement, response handling.
- Mock-API automated checks (they start with milestone 002). A test framework is deliberately unspecified (ticket 06); milestone 001's acceptance is the install smoke test plus direct behavioral verification of help and invalid usage, with no test-suite requirement.
- CI provider, OS/architecture matrix, release binaries, package managers.
- Changes to `README.md`, `LICENSE`, `.gitignore`, `.wayfinder/`, `docs/plans/`, or `verify-incident-id.sh`.
- Any credential configuration (`GOPRIVATE`, Git credentials, SSH rewriting) — documented for users elsewhere, never executed by the build.
- Tags, pushes, publications, or any release action.

## Section 2: Requirements

### R1 — Module foundation (enables every later slice)

- **R1.1** The repository root contains `go.mod` declaring the module path `github.com/jrswab/betterstack-cli` and the directive `go 1.26`, with no `toolchain` directive and no third-party `require` entries.
- **R1.2** The command package lives at `cmd/bs`; building it produces an executable named `bs`, and `go install ./cmd/bs` from a clean checkout succeeds, installing a working `bs` binary.
- **R1.3** Every build and verification command runs with a Go toolchain of version 1.26 or newer (constraint 1.2).

**Acceptance:** `go install ./cmd/bs` exits 0; `go.mod` content matches R1.1 exactly; the installed `bs` runs.

### R2 — Root help

- **R2.1** `bs --help` writes human-readable text (not JSON) to stdout, lists the resource groups `incidents` and `monitors` each with a one-line description, and exits 0.
- **R2.2** Root help writes nothing to stderr and makes no network request.

**Acceptance:** running the installed binary with argv `bs --help` exits 0; stdout names `incidents` and `monitors`; stdout is not a JSON document; stderr is empty.

### R3 — Resource-level help

- **R3.1** `bs incidents --help` writes human-readable help to stdout that lists the incident commands (`list`, `get`, `timeline`) with their usage shapes, and exits 0.
- **R3.2** `bs monitors --help` writes human-readable help to stdout that lists the monitor commands (`list`, `get`) with their usage shapes, and exits 0.
- **R3.3** Resource help lists exactly the commands its resource defines — nothing more — and writes nothing to stderr.

**Acceptance:** both invocations exit 0; each stdout names its resource's commands with correct usage shapes; stdout is not a JSON document.

### R4 — Per-command help (five commands)

- **R4.1** Each of the five commands answers `--help` with a usage line, an arguments section, a flags section, and examples.
- **R4.2** Usage lines document the positional placeholders exactly: `bs incidents get <id-or-url>`, `bs incidents timeline <id-or-url>`, `bs monitors get <id>`. The list commands take no positional target; their usage lines are `bs incidents list` and `bs monitors list`.
- **R4.3** The arguments section states what the positional target accepts: incident commands accept an incident ID (including the bare numeric form) or a full dashboard incident URL of the observed form `https://incidents.betterstack.com/team/<team-id>/incidents/<incident-id>`; monitor commands take a bare monitor ID only. List commands have no positional target.
- **R4.4** The flags section documents that command's settled first-release flags (section 1.4), including: `--resolved` and `--acknowledged` require an explicit `true` or `false`; `--metadata` is repeatable with one `key=value` pair per occurrence; `--timeout` defaults to 30 seconds. It documents no excluded surface (section 1.4 exclusions).
- **R4.5** Each command's help shows at least one example that uses only that command's documented flags and the correct positional placeholder, with approved flag spellings (for example `bs incidents list --monitor-id 123 --resolved=false`, `bs incidents get 123 --timeout 60s`, `bs monitors get 789`).
- **R4.6** All five per-command help invocations exit 0, write the help to stdout, write nothing to stderr, and make no network request.

**Acceptance:** for each of the five commands, `bs <resource> <action> --help` exits 0; stdout contains the usage line with the correct placeholder, an arguments section, a flags section matching section 1.4, and at least one example; no excluded flag appears.

### R5 — Help routing and non-help behavior

- **R5.1** Exactly these eight argument vectors print help and exit 0 (each token exact; no extra tokens):

  | # | argv (after the program name) | help level |
  |---|-------------------------------|------------|
  | 1 | `--help` | root |
  | 2 | `incidents --help` | resource |
  | 3 | `monitors --help` | resource |
  | 4 | `incidents list --help` | command |
  | 5 | `incidents get --help` | command |
  | 6 | `incidents timeline --help` | command |
  | 7 | `monitors list --help` | command |
  | 8 | `monitors get --help` | command |

- **R5.2** Every other argument vector is invalid usage in milestone 001: the CLI writes one readable error to stderr, writes nothing to stdout, and exits 1. This includes, at minimum:
  - bare `bs` (no arguments);
  - a resource without an action (`bs incidents`, `bs monitors`);
  - an unknown resource (for example `bs uptime list`) or an unknown action (for example `bs incidents delete`);
  - a recognized help form combined with any additional token (for example `bs --help incidents`, `bs incidents get 123 --help`);
  - any flag other than the exact `--help` (for example `-h`, `--timeout`, `--page`);
  - a missing required positional target (`bs incidents get`) or extra positional arguments (`bs monitors get 1 2`);
  - a well-formed non-help invocation of a command (`bs incidents get 123`, `bs monitors list`), whose runtime behavior arrives in later milestones.
- **R5.3** Error messages are readable: each names what was wrong and how to correct it (for example by pointing to the relevant `--help` level). Exact wording is an implementation choice. Errors go to stderr only.
- **R5.4** Every invocation exits with code 0 or 1 only.
- **R5.5** No invocation in milestone 001 performs a network request and no invocation reads `BETTERSTACK_UPTIME_API_TOKEN` or any credential source. All R2–R5 behavior is identical whether that variable is unset or set to any value.

**Acceptance:** each of the eight argv forms exits 0 with the expected help on stdout and empty stderr; each invalid-usage class exits 1 with a readable stderr message and empty stdout; with `BETTERSTACK_UPTIME_API_TOKEN` unset and separately set to a dummy value, all R2–R5 invocations produce identical exit codes and identical stdout/stderr content.

### R6 — Repository preservation and release restraint

- **R6.1** Milestone 001 adds only `go.mod` and the files of `cmd/bs`. No pre-existing file or directory is modified or removed: `.wayfinder/`, `docs/plans/`, `verify-incident-id.sh`, `LICENSE`, `README.md`, and `.gitignore` stay untouched.
- **R6.2** The milestone authorizes no release actions: no tag, no push, no publication, and no visibility change.

### Parallelism

R1 must come first; it grounds the module and the `cmd/bs` package. After R1, R2, R3, and the five R4 command-help items are independent of each other and can be built and verified in any order. R5 depends on all of R2–R4 existing, because its routing table covers their forms.

### Verification notes for the executor

- Build and verify with `GOTOOLCHAIN=go1.26.9` (or any Go ≥ 1.26) per R1.3.
- Run the acceptance commands against the installed binary, not only `go run`, so the smoke test matches the milestone's acceptance wording.
- Verify A6 (no network, no token read) by running the full R2–R5 invocation set twice — once with `BETTERSTACK_UPTIME_API_TOKEN` unset and once set to a dummy value — and comparing exit codes and output byte-for-byte. A stronger optional check: rerun the set with `HTTP_PROXY` and `HTTPS_PROXY` pointed at an unreachable local address and confirm no outcome changes.
