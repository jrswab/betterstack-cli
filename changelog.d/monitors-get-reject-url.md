### Fixed

- `bs monitors get` rejects URL and path targets locally before credential checks or HTTP access, matching its bare-ID help contract. `cmd/bs/monitors.go` preserves opaque nonnumeric IDs and rejects targets containing `/` or starting with an RFC 3986 scheme prefix. `skills/bs/SKILL.md` and `.mem/uptime/commands.md` document the local error.
