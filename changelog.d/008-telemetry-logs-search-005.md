### Added

- `cmd/bs/logs.go` adds approved SQL URL parsing and a separate, timed Basic-authenticated SQL POST executor with no redirects or retries. It preserves 2xx bytes and redacts credential literals only from non-2xx SQL bodies. `README.md` documents the three required SQL environment variables. The SQL command route remains unavailable.
