# Uptime commands

## Entry points and usage diagnostics

`main` calls `runWithAPI` directly. The compatibility entry point `run(args, stdout, stderr)` supplies `defaultAPIBaseURL` and `newHTTPClient()` to `runWithAPI`. It does not accept an injected endpoint or client.

`runWithAPI` delegates unchanged Uptime dependencies to `runWithEndpoints`. `runWithEndpoints` checks help and dispatches recognized commands before falling through to `usageError`. The Command routing boundary writes that diagnostic to stderr and returns 1. `usageError` handles missing commands, invalid root help, unknown options, and unknown resources. Its hard-coded resource gate recognizes `incidents`, `monitors`, `sources`, and `logs` for delegation to `resourceUsageError`. That helper uses `knownAction`: incidents accepts `list`, `get`, and `timeline`; monitors and sources accept `list` and `get`; logs accepts `search`; other resources return false. Routed commands validate their remaining arguments in their handlers, not in this fallthrough.

Later resources must extend the resource gate in `usageError` and the action inventory in `knownAction`, alongside routing and `helpFor`. Without this gate update, the diagnostic fallthrough classifies the new resource as unknown.

## Routes and endpoints

[runWithEndpoints](../../cmd/bs/main.go), reached through the compatible `runWithAPI` interface, checks exact help forms first, then routes these five Uptime commands alongside [sources](../telemetry/sources.md) and [logs search](../telemetry/logs.md). Uptime endpoints use the base URL `https://incidents.betterstack.com` from [monitors.go](../../cmd/bs/monitors.go).

| Command | Handler | GET path |
|---|---|---|
| `bs monitors list` | `monitorsList` | `/api/v2/monitors` |
| `bs monitors get <id>` | `monitorsGet` | `/api/v2/monitors/<escaped-target>` |
| `bs incidents list` | `incidentsList` | `/api/v3/incidents` |
| `bs incidents get <id-or-url>` | `incidentsGet` | `/api/v3/incidents/<escaped-id>` |
| `bs incidents timeline <id-or-url>` | `incidentsTimeline` | `/api/v3/incidents/<escaped-id>/timeline` |

List commands take no positional target. Get and timeline commands take exactly one nonempty target. `--timeout <duration>` accepts a positive `time.ParseDuration` value, defaults to `30s`, and can occur before or after a get/timeline target. Repeated timeouts use the last value. The equals form `--timeout=60s` is not accepted.

Monitor get accepts one nonempty bare ID with no numeric restriction and escapes it as one API path segment.
The parser rejects targets containing `/` or starting with an RFC 3986 scheme prefix, including `https:` and `mailto:`.
This rejection returns 1 with empty stdout before the missing-token check or any request. It writes this diagnostic to stderr:

```text
bs: expected a bare monitor ID, not a URL or path; run 'bs monitors get --help' for usage.
```

Incident get/timeline apply the separate [incident target rules](incidents.md).

## List flags

`monitorsList` accepts these space-separated flag values:

| Flag | API parameter |
|---|---|
| `--url` | `url` |
| `--pronounceable-name` | `pronounceable_name` |
| `--team-name` | `team_name` |
| `--page` | `page` |
| `--per-page` | `per_page` |

`incidentsList` accepts `--from`, `--to`, `--monitor-id`, `--heartbeat-id`, `--team-name`, `--page`, and `--per-page`. They map to `from`, `to`, `monitor_id`, `heartbeat_id`, `team_name`, `page`, and `per_page`. It also accepts explicit boolean `--resolved`/`--acknowledged` values and repeatable `--metadata` pairs. [Incidents](incidents.md) defines their validation and encoding.

Non-timeout monitor filters and ordinary incident-list filters pass through unchanged for upstream validation. Absent flags add no parameter. Explicit empty values remain present. Repeated filters preserve each occurrence with `url.Values.Add`. The CLI sends one page and never follows pagination links.

## Help boundary

`helpFor` returns constants from [help.go](../../cmd/bs/help.go) for exactly `bs --help`, `bs monitors --help`, `bs incidents --help`, and `<resource> <action> --help` for the five Uptime commands, plus source resource/list/get help and logs resource/search help. [Sources](../telemetry/sources.md) and [logs search](../telemetry/logs.md) document the Telemetry behavior. Help writes human-readable text to stdout and exits 0 without reading a token or making a request. Extra tokens and `-h` are not help forms. Invalid arguments produce local stderr diagnostics and exit 1.

This example calls the help boundary without credentials or HTTP:

```go
text, ok := helpFor([]string{"incidents", "timeline", "--help"})
```

`ok` is true, and `text` is `incidentsTimelineHelp`. See [summary](summary.md), [requests](requests.md), and [testing](testing.md).
