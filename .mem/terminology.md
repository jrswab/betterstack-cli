# Terminology

- **Uptime CLI** - the read-only `bs` command package for Better Stack monitors and incidents. Avoid: Telemetry CLI.
- **Uptime surface (anchor surface `uptime`)** - the production boundaries shared by the five Uptime CLI commands for monitors and incidents. Avoid: Telemetry surface.
- **Command routing boundary (anchor role `router`)** - `runWithAPI` preserves the Uptime injection interface and delegates help and handler selection to `runWithEndpoints`. Avoid: configurable endpoint.
- **Endpoint injection seam** - `runWithEndpoints`, which accepts distinct Uptime base URL, Telemetry REST base URL, and Telemetry SQL URL test inputs. Exact help returns before all active routes. `logs search` uses the SQL input through its query builder and SQL request executor. Avoid: runtime endpoint override.
- **Command handler (anchor role `command-handler`)** - `monitorsList`, `monitorsGet`, `incidentsList`, `incidentsGet`, `incidentsTimeline`, `sourcesList`, `sourcesGet`, or `logsSearch`, which validates arguments before its request executor. Avoid: request executor.
- **Request executor (anchor role `request-executor`)** - `doMonitorsRequest`, the shared bearer-authenticated GET and raw-response path for the seven Uptime/source handlers. Avoid: monitor-only client.
- **Uptime configuration boundary (anchor role `config`)** - the const block in `cmd/bs/monitors.go` defining `defaultAPIBaseURL`, `defaultTimeout`, and `tokenEnvVar`. Avoid: runtime endpoint override.
- **Telemetry configuration boundary (anchor surface `telemetry`, role `config`)** - `defaultTelemetryRESTBaseURL` in `cmd/bs/main.go` defines the fixed Telemetry REST host, and `telemetryTokenEnvVar` in `cmd/bs/sources.go` names its separate credential. The Telemetry configuration boundary also names the SQL URL, username, and password variables in `cmd/bs/logs.go`. Avoid: runtime REST endpoint override.
- **Incident target** - a scheme-less opaque ID or an accepted dashboard URL resolved by `resolveIncidentTarget`. Avoid: numeric-only bare ID.
- **Dashboard URL** - the HTTPS `incidents.betterstack.com/team/<team-id>/incidents/<incident-id>` form with a nonempty team segment and terminal ASCII digits. Avoid: arbitrary incident URL.
- **Metadata pair** - a repeatable incident-list `--metadata key=value` filter with a nonempty, case-sensitive key. Avoid: raw query flag.
- **Uptime token** - the value of `BETTERSTACK_UPTIME_API_TOKEN`, used in the bearer header and local diagnostic redaction. Avoid: legacy token fallback.
- **Source ID** - one nonempty opaque Telemetry source target escaped as a single REST path segment, without URL interpretation or numeric restrictions. Avoid: source URL, numeric-only source ID.
- **Telemetry API token** - the value of `BETTERSTACK_TELEMETRY_API_TOKEN`, used only by source REST commands for bearer authentication and local diagnostic redaction. The value also participates in non-2xx SQL response redaction, but never SQL authentication. Avoid: Uptime fallback, source response token.
- **Telemetry secret redaction** - `redactSecrets`, a direct helper for non-empty Telemetry API token, SQL username, SQL password, and derived Basic-auth encoding. It applies only to non-2xx SQL response bodies. Avoid: response redaction.
- **Raw response body** - upstream bytes passed through without JSON decoding, formatting, redaction, or an added newline. Avoid: normalized JSON.
- **Log search arguments** - the locally validated builder input from `parseLogsSearchArgs`, used by the `logs search` command handler. Avoid: raw SQL input.
- **SQL request executor (anchor surface `telemetry`, role `request-executor`)** - `doLogsSQLRequest`, the separate Basic-authenticated SQL POST boundary. Avoid: Request executor, source request path.
- **SQL configuration** - the required SQL URL, username, and password environment variables read by `doLogsSQLRequest`. Avoid: API-token fallback.
- **Log search query builder** - `buildLogsSearchQuery`, which validates search arguments and returns SQL and separate typed parameters without requests. Avoid: SQL executor.
- **Full table identifier** - the required log-search `--table` value matching `^t[0-9]+_[a-z0-9_]+$`. Avoid: Source ID, SQL fragment.
- **Loopback-only network guard** - the test transport and dial checks that admit literal loopback IPs and reject other hosts without DNS lookup. Avoid: localhost allowlist.

Sources: [routing](../cmd/bs/main.go), [requests](../cmd/bs/monitors.go), [source handlers](../cmd/bs/sources.go), [log search builder](../cmd/bs/logs.go), [secret redaction](../cmd/bs/telemetry.go), [incidents](../cmd/bs/incidents.go), and [test safety](../cmd/bs/safety_test.go). See the [memory map](mem-map.md).
