### Added

- `cmd/bs/logs.go` adds builder-only search argument parsing with a validated full table identifier, single-value duplicate rejection, an idempotent cold switch, and explicit query presence. `cmd/bs/logs_test.go` covers local validation without requests or credentials.
