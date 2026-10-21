### Added

- `cmd/bs/logs.go` adds a validated, builder-only hot log search with an optional filtered cold union, bound text and relative or explicit Unix-millisecond time parameters, descending default order, a 1–1000 limit, positive durations, overflow-safe timeout rounding, single-value duplicate rejection, and control-character rejection. `cmd/bs/logs_test.go` verifies SQL and parameters without requests or credentials.
