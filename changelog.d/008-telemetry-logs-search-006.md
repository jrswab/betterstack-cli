### Added

- `bs logs search` now validates search flags and executes parameterized SQL through `cmd/bs/main.go` and `cmd/bs/logs.go`. `cmd/bs/help.go` and `README.md` document the flags, full table identifier, option-shaped query values, and missing-hot-table `--cold` guidance.
