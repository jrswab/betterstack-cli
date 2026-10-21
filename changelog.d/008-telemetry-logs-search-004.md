### Added

- `cmd/bs/logs.go` adds validated hot log-search SQL with an optional filtered cold union and separately bound text and Unix-millisecond time parameters. The builder enforces bounded limits, positive durations, duplicate rejection, control-character rejection, and rounded timeout settings. `cmd/bs/logs_test.go` verifies local parsing, SQL, and parameters without requests or credentials. Command routing and help remain unchanged.
