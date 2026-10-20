# Anchor index

Each row identifies one production boundary occurrence.

| Anchor | Source | Symbol | Boundary |
|---|---|---|---|
| `uptime:router` | [cmd/bs/main.go](cmd/bs/main.go) | `runWithAPI` | Compatible Uptime injection interface |
| `uptime:router` | [cmd/bs/main.go](cmd/bs/main.go) | `runWithEndpoints` | Endpoint injection seam and Command routing boundary |
| `uptime:config` | [cmd/bs/monitors.go](cmd/bs/monitors.go) | `defaultAPIBaseURL`, `defaultTimeout`, `tokenEnvVar` | Uptime configuration boundary |
| `uptime:command-handler` | [cmd/bs/monitors.go](cmd/bs/monitors.go) | `monitorsList` | Monitors list command handler |
| `uptime:command-handler` | [cmd/bs/monitors.go](cmd/bs/monitors.go) | `monitorsGet` | Monitors get command handler |
| `uptime:request-executor` | [cmd/bs/monitors.go](cmd/bs/monitors.go) | `doMonitorsRequest` | Request executor |
| `uptime:command-handler` | [cmd/bs/incidents.go](cmd/bs/incidents.go) | `incidentsList` | Incidents list command handler |
| `uptime:command-handler` | [cmd/bs/incidents.go](cmd/bs/incidents.go) | `incidentsGet` | Incidents get command handler |
| `uptime:command-handler` | [cmd/bs/incidents.go](cmd/bs/incidents.go) | `incidentsTimeline` | Incidents timeline command handler |
