### Added

- `cmd/bs/logs.go` adds builder-only search argument parsing with a validated full table identifier, single-value duplicate rejection, an idempotent cold switch, explicit query presence, descending default order, a 1–1000 limit, and positive duration windows and timeouts. `cmd/bs/logs_test.go` covers local validation without requests or credentials.
