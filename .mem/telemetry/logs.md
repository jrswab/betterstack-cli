# Telemetry log search

## Argument validation

[parseLogsSearchArgs](../../cmd/bs/logs.go) accepts space-separated search flags without requests, endpoint access, or credentials. It requires one full table identifier matching `^t[0-9]+_[a-z0-9_]+$`. All value-bearing flags reject duplicates before checking the repeated value. Unknown flags, equals forms, positionals, and missing values fail locally. Repeated `--cold` switches enable one option. A query pointer distinguishes an omitted query from an explicitly empty string.

This parser example preserves an empty query and enables cold storage:

```go
parsed, err := parseLogsSearchArgs([]string{"--table", "t1_fake", "--query", "", "--cold"})
```

## Defaults and bounded values

Order defaults to `desc` and accepts only `asc` or `desc`. Limit defaults to 100 and accepts ASCII decimal integers from 1 through 1000, including leading zeros. The parser reuses the source pagination digit check without changing source behavior. The relative window defaults to one hour. `--since` and `--timeout` require positive Go durations. The timeout defaults to the existing 30 seconds. Day suffixes, malformed durations, zero, and negatives fail locally.

## Hot query construction

[buildLogsSearchQuery](../../cmd/bs/logs.go) accepts raw arguments and a captured `time.Time`, validates arguments, and returns SQL plus `url.Values`. Invalid input returns empty SQL and nil parameters. It selects `dt, raw` from `remote(<table>_logs)`, applies typed inclusive-lower and exclusive-upper time parameters, then uses one order, bounded limit, timeout setting, and `FORMAT JSONEachRow`. It computes the relative window from the supplied time.

An optional query uses `positionCaseInsensitiveUTF8(raw, {query:String}) > 0` and a separate `param_query`. Unicode control characters fail locally. Quotes, backslashes, punctuation, and SQL-looking strings remain parameter data. Even option-shaped query text is data. Empty query text supplies an empty parameter. Omitted query text supplies no raw predicate or query parameter.

Timeout settings use integer division and a remainder check to round up without overflowing the duration. This setting does not guarantee server cancellation.

The parser has no command route or help entry. [Parser tests](../../cmd/bs/logs_test.go) use only fake strings.

Related: [memory map](../mem-map.md), [terminology](../terminology.md), and [sources](sources.md).
