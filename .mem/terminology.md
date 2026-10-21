# Terminology

- **Uptime CLI** - the read-only `bs` command package for Better Stack monitors and incidents. Avoid: Telemetry CLI.
- **Uptime surface (anchor surface `uptime`)** - the production boundaries shared by the five Uptime CLI commands for monitors and incidents. Avoid: Telemetry surface.
- **Command routing boundary (anchor role `router`)** - `runWithAPI` preserves the Uptime injection interface and delegates help and handler selection to `runWithEndpoints`. Avoid: configurable endpoint.
- **Endpoint injection seam** - `runWithEndpoints`, which accepts distinct Uptime base URL, Telemetry REST base URL, and Telemetry SQL URL test inputs. Uptime and Telemetry REST routes are active. The SQL input remains unused. Avoid: runtime endpoint override.
- **Command handler (anchor role `command-handler`)** - `monitorsList`, `monitorsGet`, `incidentsList`, `incidentsGet`, `incidentsTimeline`, `sourcesList`, or `sourcesGet`, which validates arguments and builds its service endpoint. Avoid: request executor.
- **Request executor (anchor role `request-executor`)** - `doMonitorsRequest`, the shared bearer-authenticated GET and raw-response path for all seven handlers. Avoid: monitor-only client.
- **Uptime configuration boundary (anchor role `config`)** - the const block in `cmd/bs/monitors.go` defining `defaultAPIBaseURL`, `defaultTimeout`, and `tokenEnvVar`. Avoid: runtime endpoint override.
- **Telemetry configuration boundary (anchor surface `telemetry`, role `config`)** - `defaultTelemetryRESTBaseURL` in `cmd/bs/main.go` defines the fixed Telemetry REST host, and `telemetryTokenEnvVar` in `cmd/bs/sources.go` names its separate credential. Avoid: runtime endpoint override.
- **Incident target** - a scheme-less opaque ID or an accepted dashboard URL resolved by `resolveIncidentTarget`. Avoid: numeric-only bare ID.
- **Dashboard URL** - the HTTPS `incidents.betterstack.com/team/<team-id>/incidents/<incident-id>` form with a nonempty team segment and terminal ASCII digits. Avoid: arbitrary incident URL.
- **Metadata pair** - a repeatable incident-list `--metadata key=value` filter with a nonempty, case-sensitive key. Avoid: raw query flag.
- **Uptime token** - the value of `BETTERSTACK_UPTIME_API_TOKEN`, used in the bearer header and local diagnostic redaction. Avoid: legacy token fallback.
- **Source ID** - one nonempty opaque Telemetry source target escaped as a single REST path segment, without URL interpretation or numeric restrictions. Avoid: source URL, numeric-only source ID.
- **Telemetry API token** - the value of `BETTERSTACK_TELEMETRY_API_TOKEN`, used only by source REST commands for bearer authentication and local diagnostic redaction. Avoid: Uptime fallback, source response token.
- **Telemetry secret redaction** - `redactSecrets`, a direct helper for non-empty Telemetry API token, SQL username, SQL password, and derived Basic-auth encoding. No response path calls it. Avoid: response redaction.
- **Raw response body** - upstream bytes passed through without JSON decoding, formatting, redaction, or an added newline. Avoid: normalized JSON.
- **Log search arguments** - the locally validated builder input from `parseLogsSearchArgs`, without a command route or request. Avoid: raw SQL input.
- **Full table identifier** - the required log-search `--table` value matching `^t[0-9]+_[a-z0-9_]+$`. Avoid: Source ID, SQL fragment.
- **Loopback-only network guard** - the test transport and dial checks that admit literal loopback IPs and reject other hosts without DNS lookup. Avoid: localhost allowlist.

Sources: [routing](../cmd/bs/main.go), [requests](../cmd/bs/monitors.go), [source handlers](../cmd/bs/sources.go), [secret redaction](../cmd/bs/telemetry.go), [incidents](../cmd/bs/incidents.go), and [test safety](../cmd/bs/safety_test.go). See the [memory map](mem-map.md).
