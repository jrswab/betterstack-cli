# Uptime commands

## Routes and endpoints

[runWithAPI](../../cmd/bs/main.go) checks exact help forms first, then routes these five commands. Endpoints use the base URL `https://incidents.betterstack.com` from [monitors.go](../../cmd/bs/monitors.go).

| Command | Handler | GET path |
|---|---|---|
| `bs monitors list` | `monitorsList` | `/api/v2/monitors` |
| `bs monitors get <id>` | `monitorsGet` | `/api/v2/monitors/<escaped-target>` |
| `bs incidents list` | `incidentsList` | `/api/v3/incidents` |
| `bs incidents get <id-or-url>` | `incidentsGet` | `/api/v3/incidents/<escaped-id>` |
| `bs incidents timeline <id-or-url>` | `incidentsTimeline` | `/api/v3/incidents/<escaped-id>/timeline` |

List commands take no positional target. Get and timeline commands take exactly one nonempty target. `--timeout <duration>` accepts a positive `time.ParseDuration` value, defaults to `30s`, and can occur before or after a get/timeline target. Repeated timeouts use the last value. The equals form `--timeout=60s` is not accepted.

Monitor help describes a bare ID. The current monitor parser treats the target as opaque and escapes it without a numeric or URL-format check. Incident get/timeline apply the separate [incident target rules](incidents.md).

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

`helpFor` returns constants from [help.go](../../cmd/bs/help.go) for exactly `bs --help`, `bs monitors --help`, `bs incidents --help`, and `<resource> <action> --help` for the five routed commands. Help writes human-readable text to stdout and exits 0 without reading a token or making a request. Extra tokens and `-h` are not help forms. Invalid arguments produce local stderr diagnostics and exit 1.

This example calls the help boundary without credentials or HTTP:

```go
text, ok := helpFor([]string{"incidents", "timeline", "--help"})
```

`ok` is true, and `text` is `incidentsTimelineHelp`. See [summary](summary.md), [requests](requests.md), and [testing](testing.md).
