# Terminology

- **Uptime CLI** - the read-only `bs` command package for Better Stack monitors and incidents. Avoid: Telemetry CLI.
- **Uptime surface (anchor surface `uptime`)** - the production boundaries shared by the five Uptime CLI commands for monitors and incidents. Avoid: Telemetry surface.
- **Command routing boundary (anchor role `router`)** - `runWithAPI`, which selects help or one of five handlers and accepts injected writers, base URL, and HTTP client. Avoid: configurable endpoint.
- **Command handler (anchor role `command-handler`)** - `monitorsList`, `monitorsGet`, `incidentsList`, `incidentsGet`, or `incidentsTimeline`, which validates arguments and builds an Uptime endpoint. Avoid: request executor.
- **Request executor (anchor role `request-executor`)** - `doMonitorsRequest`, the shared bearer-authenticated GET and raw-response path for all five handlers. Avoid: monitor-only client.
- **Uptime configuration boundary (anchor role `config`)** - the const block in `cmd/bs/monitors.go` defining `defaultAPIBaseURL`, `defaultTimeout`, and `tokenEnvVar`. Avoid: runtime endpoint override.
- **Incident target** - a scheme-less opaque ID or an accepted dashboard URL resolved by `resolveIncidentTarget`. Avoid: numeric-only bare ID.
- **Dashboard URL** - the HTTPS `incidents.betterstack.com/team/<team-id>/incidents/<incident-id>` form with a nonempty team segment and terminal ASCII digits. Avoid: arbitrary incident URL.
- **Metadata pair** - a repeatable incident-list `--metadata key=value` filter with a nonempty, case-sensitive key. Avoid: raw query flag.
- **Uptime token** - the value of `BETTERSTACK_UPTIME_API_TOKEN`, used in the bearer header and local diagnostic redaction. Avoid: legacy token fallback.
- **Raw response body** - upstream bytes passed through without JSON decoding, formatting, redaction, or an added newline. Avoid: normalized JSON.
- **Loopback-only network guard** - the test transport and dial checks that admit literal loopback IPs and reject other hosts without DNS lookup. Avoid: localhost allowlist.

Sources: [routing](../cmd/bs/main.go), [requests](../cmd/bs/monitors.go), [incidents](../cmd/bs/incidents.go), and [test safety](../cmd/bs/safety_test.go). See the [memory map](mem-map.md).
