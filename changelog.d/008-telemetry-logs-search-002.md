### Added

- `cmd/bs/main.go:runWithEndpoints` accepts distinct Uptime, Telemetry REST, and SQL test endpoints while preserving `runWithAPI` and existing Uptime behavior. `cmd/bs/telemetry.go:redactSecrets` provides literal Telemetry credential redaction with direct tests, without changing response output. `TAGS.md` and `.mem/` record these S1 contracts.
