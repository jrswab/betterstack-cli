---
name: bs
description: >-
  Read-only Better Stack CLI (`bs`) for inspecting Uptime incidents and monitors,
  discovering Telemetry sources, and searching logs.
  Use for Better Stack/betterstack incidents or monitors, Better Stack incident
  timelines, and Better Stack telemetry sources.
  Also use for Better Stack log search, Better Stack Telemetry SQL, or the `bs` command.
  Do not use to create, acknowledge, or resolve incidents or monitors.
---

# Better Stack CLI

Use `bs` to inspect Uptime incidents and monitors, discover Telemetry sources, and search logs.

## Prerequisites

**Installation.** Use Go 1.26 or newer. Install the command package:

```sh
go install github.com/jrswab/betterstack-cli/cmd/bs@latest
```

For private repository access, include `github.com/jrswab/betterstack-cli` in `GOPRIVATE` before installation.
Preserve existing comma-separated patterns. Git must already have authenticated access. `GOPRIVATE` does not provide authentication.
Go installs into `GOBIN`, or `GOPATH/bin` when `GOBIN` is unset. Include that directory in `PATH`.
Use `bs --help` to check the installed command inventory before using this skill.

**Examples.** `jq` is optional for the CLI but required for the JSON validation and field-selection examples.

**Configuration.** The user must supply credentials through the environment or a secret manager.

| Command group | Required environment variables | Purpose |
|---|---|---|
| `incidents list/get/timeline`, `monitors list/get` | `BETTERSTACK_UPTIME_API_TOKEN` | Uptime bearer token. |
| `sources list/get` | `BETTERSTACK_TELEMETRY_API_TOKEN` | Telemetry REST bearer token. |
| `logs search` | `BETTERSTACK_TELEMETRY_SQL_URL`, `BETTERSTACK_TELEMETRY_SQL_USERNAME`, `BETTERSTACK_TELEMETRY_SQL_PASSWORD` | SQL endpoint and HTTP Basic credentials. |
| Exact help forms | None. | Help sends no request. |

SQL URL forms are `host`, `host:443`, or `https://host[:port]`.
Absolute HTTPS URLs allow an empty path or `/`, without userinfo, query, or fragment.
Uptime and Telemetry REST hosts are fixed. SQL credentials never fall back to API tokens.
Log search uses an existing Telemetry API token only to redact SQL error output. It does not require that token.
The legacy `BETTERSTACK_API_TOKEN` has no effect.

## Command reference

**Shared flags.** Every command accepts `--timeout <duration>`, a positive Go duration with default `30s`.
Use space-separated values. Incident booleans also accept equals forms. Get and timeline commands accept timeout before or after the target.
Uptime and source commands use the last valid repeated timeout. Log search rejects duplicate value-bearing flags.

**Help.** Use these exact forms without targets or other flags. Help exits 0 and writes human-readable text to stdout.

```sh
bs incidents list --help
bs incidents get --help
bs incidents timeline --help
bs monitors list --help
bs monitors get --help
bs sources list --help
bs sources get --help
bs logs search --help
```

### Incidents

All three commands require `BETTERSTACK_UPTIME_API_TOKEN`.

| Command | Positional arguments | Flags in addition to `--timeout` |
|---|---|---|
| `bs incidents list` | None. | `--from <value>`, `--to <value>`, `--monitor-id <id>`, `--heartbeat-id <id>`, `--resolved <true\|false>`, `--acknowledged <true\|false>`, `--metadata <key=value>`, `--team-name <name>`, `--page <n>`, `--per-page <n>`. |
| `bs incidents get <id-or-url>` | One nonempty incident ID or accepted dashboard URL. | None. |
| `bs incidents timeline <id-or-url>` | One nonempty incident ID or accepted dashboard URL. | None. |

List filters have no CLI defaults. `--from` and `--to` filter incident creation time, inclusive at both bounds.
Ordinary filters pass through unchanged for API validation. Repeated filters preserve each occurrence.
Booleans require exact lowercase `true` or `false`. For example, `--resolved=false` is valid, but bare `--resolved` is not.
Metadata splits at the first `=`. Keys must be nonempty. Exact duplicate keys are rejected. `a` and `A` are different keys.
Values can be empty or contain `=`.

Bare incident IDs are opaque, not numeric-only. Dashboard URLs must use this form: `https://incidents.betterstack.com/team/<team-id>/incidents/<incident-id>`.
The team segment must be nonempty. The terminal incident ID must contain ASCII digits only.
Use the exact host without a port, userinfo, query, fragment, or trailing slash.
The CLI extracts the terminal ID locally. It does not request the dashboard URL.

### Monitors

Both commands require `BETTERSTACK_UPTIME_API_TOKEN`.

| Command | Positional arguments | Flags in addition to `--timeout` |
|---|---|---|
| `bs monitors list` | None. | `--url <value>`, `--pronounceable-name <name>`, `--team-name <name>`, `--page <n>`, `--per-page <n>`. |
| `bs monitors get <id>` | One nonempty monitor ID. Use a bare ID, not a dashboard URL. | None. |

Monitor IDs have no numeric restriction. The CLI rejects targets containing `/` or starting with a URL scheme prefix, including `https:` and `mailto:`.
It exits 1 with empty stdout before credential checks or HTTP access. The stderr diagnostic is:

```text
bs: expected a bare monitor ID, not a URL or path; run 'bs monitors get --help' for usage.
```
List filters have no CLI defaults. They pass through unchanged for API validation. Repeated filters preserve each occurrence.

### Sources

Both commands require `BETTERSTACK_TELEMETRY_API_TOKEN`.

| Command | Positional arguments | Flags in addition to `--timeout` |
|---|---|---|
| `bs sources list` | None. | `--page <integer>`, `--per-page <integer>`. |
| `bs sources get <id>` | One nonempty opaque source ID, escaped as one path segment. | None. |

Pagination flags have no CLI defaults. Their repeated values remain separate query parameters.
The CLI does not interpret source IDs as URLs or require numeric IDs. Source output includes secret `token` fields.

### Logs

`bs logs search` requires all three SQL environment variables. It takes no positional target or raw SQL.

| Flag | Meaning and default |
|---|---|
| `--table <table_name>` | Required full table identifier matching `^t[0-9]+_[a-z0-9_]+$`. |
| `--since <duration>` | Positive Go duration. Default `1h`, ending now. Cannot combine with `--from` or `--to`. |
| `--from <time>` | Inclusive lower bound in RFC3339Nano. Must precede the upper bound. |
| `--to <time>` | Exclusive upper bound in RFC3339Nano. Requires `--from`. Default now. |
| `--query <text>` | Case-insensitive substring of `raw`. Default no text filter. Control characters are rejected. |
| `--order <asc\|desc>` | Timestamp order. Default `desc`. |
| `--limit <integer>` | ASCII decimal integer from 1 through 1000. Default `100`. |
| `--timeout <duration>` | Positive Go duration. Default `30s`. |
| `--cold` | Include cold storage with hot storage. Default hot storage only. May repeat. |

Use `bs sources get <id>` to obtain `team_id` and `table_name` for `t<team_id>_<table_name>`.
A source ID or unprefixed `table_name` is not a full table identifier. The CLI does not discover the table automatically.
`--query` consumes the next argument as text, even option-shaped text.
`--query --cold` searches for literal `--cold` without enabling cold storage.
Use `--cold --query --cold` to enable cold storage and search that text.

## Output handling

**Capture first.** A successful HTTP response passes raw API body bytes to stdout, without formatting, wrapping, or an added newline.
Non-2xx bodies go to stderr with exit 1. Uptime and source error bodies remain raw.
Only SQL error bodies receive literal Telemetry credential redaction. Do not assume other output is redacted.
Local failures also exit 1. Successful responses and exact help exit 0. The CLI has no `--json` flag.

Create a private temporary directory before the workflow examples:

```sh
private_dir=$(mktemp -d)
bs incidents list --per-page 10 >"$private_dir/incidents.json" 2>"$private_dir/error"
```

Check the command's exit status before processing stdout. On failure, inspect stderr privately for the error category.
Validate JSON before field selection. This example selects only incident identifiers and types:

```sh
jq . "$private_dir/incidents.json" >/dev/null
jq '.data[] | {id, type}' "$private_dir/incidents.json"
```

Run field selection only if validation succeeds. API bodies are not guaranteed to contain JSON.
Log SQL requests use `JSONEachRow`, a stream of JSON objects, not a `.data` envelope.
Use `jq .` to validate the stream, then select `dt` and `raw`. Empty log output can mean no matching rows.
Keep context small through filters, small page sizes, limits, and field selection. Review log text for secrets before sharing it.
Delete private captures after the task.

## Pagination and limits

**One request.** The CLI sends one request, follows no redirects, retries nothing, and never automatically paginates.

- Incident and monitor lists accept `--page` and `--per-page` unchanged. Absent flags send no pagination defaults.
- Source pagination flags are optional. If present, `--page` must be a positive decimal and `--per-page` must be 1 through 50.
- Request subsequent pages explicitly when the user needs more results. Inspect the returned pagination information first.
- Timeline has no pagination flags or complete-history guarantee. Report any `timeline_truncated` items as incomplete history.
- Log search uses `--limit`, not pagination flags. Reduce the time window and limit before expanding a search.
- All commands default to `--timeout 30s`. Go durations use values such as `30m` or `24h`, not a `d` suffix.

## Workflows

Use the private capture directory above. Check each exit status and validate each response before proceeding.
Replace example IDs, URLs, and table identifiers with values from the user's account.

### Incident triage

Find unresolved incidents, choose an ID from the validated list, and request details followed by the timeline.

```sh
bs incidents list --resolved false --page 1 --per-page 10 >"$private_dir/incidents.json" 2>"$private_dir/error"
bs incidents get 123 --timeout 60s >"$private_dir/incident.json" 2>"$private_dir/error"
bs incidents timeline 123 >"$private_dir/timeline.json" 2>"$private_dir/error"
```

Summarize the returned state and relevant timeline items. State any history truncation.

### Monitor status check

Find the monitor by URL or name, then request its ID. Inspect the returned status fields without guessing their schema.

```sh
bs monitors list --url https://example.com --per-page 10 >"$private_dir/monitors.json" 2>"$private_dir/error"
bs monitors get 789 >"$private_dir/monitor.json" 2>"$private_dir/error"
```

### Log search scoped to a source

Capture sources privately. Select the source ID, then read only the needed nonsecret fields from its details.
Build the full table identifier from the returned team and table values. Search a short time window first.

```sh
bs sources list --page 1 --per-page 10 >"$private_dir/sources.json" 2>"$private_dir/error"
bs sources get source-id >"$private_dir/source.json" 2>"$private_dir/error"
bs logs search --table t1_example --since 30m --query error --limit 50 >"$private_dir/logs.jsonl" 2>"$private_dir/error"
bs logs search --table t1_example --from 2026-01-01T00:00:00Z --to 2026-01-02T00:00:00Z --order asc --limit 50 >"$private_dir/logs.jsonl" 2>"$private_dir/error"
```

The explicit-bounds example is an alternative, not a required second request. Choose bounds relevant to the incident.

## Error handling

**Missing configuration.** Unset and empty values have the same failure. API-token diagnostics are:

```text
bs: BETTERSTACK_UPTIME_API_TOKEN is not set; set it to a Better Stack API token and try again.
bs: BETTERSTACK_TELEMETRY_API_TOKEN is not set; set it to a Better Stack API token and try again.
```

SQL reports the first missing variable in URL, username, password order:

```text
bs: BETTERSTACK_TELEMETRY_SQL_URL is not set
bs: BETTERSTACK_TELEMETRY_SQL_USERNAME is not set
bs: BETTERSTACK_TELEMETRY_SQL_PASSWORD is not set
```

| Failure | Agent action |
|---|---|
| Missing environment variable | Ask the user to configure the named variable privately, then retry. |
| `invalid BETTERSTACK_TELEMETRY_SQL_URL` | Ask for a supported regional HTTPS SQL endpoint. Keep credentials out of the URL. |
| Argument diagnostic with a help hint | Run the exact command help form. Correct arguments before retrying. |
| Authentication failure in an API body | Ask the user to check the matching credentials and permissions. Do not switch token groups or repeatedly retry. |
| Uptime/source request failure or timeout | Check connectivity. If appropriate, retry once with a larger timeout. Local diagnostics include the underlying error. |
| `bs: SQL request failed` | Check SQL host, connectivity, and timeout. This fixed diagnostic does not distinguish timeout from other transport failures. |
| `bs: could not read SQL response` | Treat the response as incomplete. Retry only after checking connectivity. |
| HTTP 500 `CLUSTER_DOESNT_EXIST` | Try the same log search with `--cold`. Success is not guaranteed. |
| Other non-2xx body | Inspect it privately and report the safe error category. The CLI adds no HTTP status wrapper. |

## Safety

**Keep credentials private.** The CLI is read-only, including its SQL POST search. It has no mutation commands.
Never echo or print token or password values. Never paste credentials into commands, chat, shared logs, or tickets.
Use environment configuration or a secret manager, not command-line credential values.
Capture potentially sensitive stdout and stderr privately before inspection. Never display complete raw source responses.

## Common mistakes

- Use exact `--help` forms. `-h` and help appended after a target or flags are not help forms.
- Use `--timeout 60s`, not `--timeout=60s`. Only incident booleans support equals forms.
- Supply explicit incident booleans and unique metadata keys. Quote metadata values containing spaces.
- Match credentials to the command group. API tokens do not authenticate SQL requests.
- Use the full table identifier, not a source ID, unprefixed table name, positional target, or raw SQL.
- Treat `--query` as substring text, not SQL or a structured search expression.
- Validate captured JSON and check exit status. Do not add `--json` or merge stderr into data pipelines.
- Request pages explicitly. A successful single page or timeline does not prove complete results.
