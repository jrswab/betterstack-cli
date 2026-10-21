# Telemetry log search

## Argument validation

[parseLogsSearchArgs](../../cmd/bs/logs.go) accepts space-separated search flags without requests, endpoint access, or credentials. It requires one full table identifier matching `^t[0-9]+_[a-z0-9_]+$`. All value-bearing flags reject duplicates before checking the repeated value. Unknown flags, equals forms, positionals, and missing values fail locally. Repeated `--cold` switches enable one option. A query pointer distinguishes an omitted query from an explicitly empty string.

This parser example preserves an empty query and enables cold storage:

```go
parsed, err := parseLogsSearchArgs([]string{"--table", "t1_fake", "--query", "", "--cold"})
```

## Defaults and bounded values

Order defaults to `desc` and accepts only `asc` or `desc`. Limit defaults to 100 and accepts ASCII decimal integers from 1 through 1000, including leading zeros. The parser reuses the source pagination digit check without changing source behavior. The relative window defaults to one hour. `--since` and `--timeout` require positive Go durations. The timeout defaults to the existing 30 seconds. Day suffixes, malformed durations, zero, and negatives fail locally.

The parser has no command route or help entry. [Parser tests](../../cmd/bs/logs_test.go) use only fake strings.

Related: [memory map](../mem-map.md), [terminology](../terminology.md), and [sources](sources.md).
