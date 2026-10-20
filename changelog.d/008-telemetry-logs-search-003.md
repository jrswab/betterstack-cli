### Added

- `cmd/bs/sources.go` adds `bs sources list` and `bs sources get` with Telemetry-only bearer authentication, validated pagination and opaque IDs, and unchanged response bodies through the shared request path. `cmd/bs/help.go` warns that source output contains a secret credential. `README.md`, `TAGS.md`, and `.mem/telemetry/sources.md` document the S2 commands and fixed REST endpoint.
