# Anchor index

Each row identifies one production boundary occurrence.

| Anchor | Source | Symbol | Boundary |
|---|---|---|---|
| `uptime:router` | [cmd/bs/main.go](cmd/bs/main.go) | `runWithAPI` | Command routing and dependency injection |
| `uptime:command-handler` | [cmd/bs/monitors.go](cmd/bs/monitors.go) | `monitorsList` | Monitors list command handler |
| `uptime:command-handler` | [cmd/bs/monitors.go](cmd/bs/monitors.go) | `monitorsGet` | Monitors get command handler |
| `uptime:request-executor` | [cmd/bs/monitors.go](cmd/bs/monitors.go) | `doMonitorsRequest` | Shared HTTP request executor |
| `uptime:command-handler` | [cmd/bs/incidents.go](cmd/bs/incidents.go) | `incidentsList` | Incidents list command handler |
| `uptime:command-handler` | [cmd/bs/incidents.go](cmd/bs/incidents.go) | `incidentsGet` | Incidents get command handler |
| `uptime:command-handler` | [cmd/bs/incidents.go](cmd/bs/incidents.go) | `incidentsTimeline` | Incidents timeline command handler |
