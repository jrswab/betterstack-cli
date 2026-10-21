# Anchor index

Each row identifies one production boundary occurrence.

| Anchor | Source | Symbol | Boundary |
|---|---|---|---|
| `uptime:router` | [cmd/bs/main.go](cmd/bs/main.go) | `runWithAPI` | Compatible Uptime injection interface |
| `uptime:router` | [cmd/bs/main.go](cmd/bs/main.go) | `runWithEndpoints` | Endpoint injection seam and Command routing boundary |
| `telemetry:config` | [cmd/bs/main.go](cmd/bs/main.go) | `defaultTelemetryRESTBaseURL` | Telemetry configuration boundary |
| `telemetry:config` | [cmd/bs/sources.go](cmd/bs/sources.go) | `telemetryTokenEnvVar` | Telemetry API credential name |
| `telemetry:config` | [cmd/bs/logs.go](cmd/bs/logs.go) | `telemetrySQLURLEnvVar`, `telemetrySQLUsernameEnvVar`, `telemetrySQLPasswordEnvVar` | SQL configuration names |
| `telemetry:request-executor` | [cmd/bs/logs.go](cmd/bs/logs.go) | `doLogsSQLRequest` | SQL request executor |
| `telemetry:command-handler` | [cmd/bs/sources.go](cmd/bs/sources.go) | `sourcesList` | Sources list command handler |
| `telemetry:command-handler` | [cmd/bs/sources.go](cmd/bs/sources.go) | `sourcesGet` | Sources get command handler |
| `uptime:config` | [cmd/bs/monitors.go](cmd/bs/monitors.go) | `defaultAPIBaseURL`, `defaultTimeout`, `tokenEnvVar` | Uptime configuration boundary |
| `uptime:command-handler` | [cmd/bs/monitors.go](cmd/bs/monitors.go) | `monitorsList` | Monitors list command handler |
| `uptime:command-handler` | [cmd/bs/monitors.go](cmd/bs/monitors.go) | `monitorsGet` | Monitors get command handler |
| `uptime:request-executor` | [cmd/bs/monitors.go](cmd/bs/monitors.go) | `doMonitorsRequest` | Request executor |
| `uptime:command-handler` | [cmd/bs/incidents.go](cmd/bs/incidents.go) | `incidentsList` | Incidents list command handler |
| `uptime:command-handler` | [cmd/bs/incidents.go](cmd/bs/incidents.go) | `incidentsGet` | Incidents get command handler |
| `uptime:command-handler` | [cmd/bs/incidents.go](cmd/bs/incidents.go) | `incidentsTimeline` | Incidents timeline command handler |
