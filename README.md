# betterstack-cli

`bs` is a read-only CLI for Better Stack Uptime and Telemetry APIs.

## Telemetry log search

`bs logs search` validates search flags, builds parameterized SQL, and sends one request through the SQL executor.

```sh
bs logs search --table t1_example
bs logs search --table t1_example --since 30m --query error --limit 50
bs logs search --table t1_example --from 2026-01-01T00:00:00Z --to 2026-01-02T00:00:00Z --order asc
bs logs search --table t1_example --cold --query --cold --timeout 60s
bs logs --help
bs logs search --help
```

| Flag | Behavior and default |
|---|---|
| `--table <table_name>` | Required full identifier `t<team_id>_<table_name>`, matching `^t[0-9]+_[a-z0-9_]+$`. |
| `--since <duration>` | Positive Go duration. Default `1h`. Cannot combine with `--from` or `--to`. |
| `--from <time>` | Inclusive lower bound in RFC3339Nano. Must precede the upper bound. |
| `--to <time>` | Exclusive upper bound in RFC3339Nano. Requires `--from`. Default: now. |
| `--query <text>` | Case-insensitive substring of `raw`. Default: no text filter. |
| `--order <asc\|desc>` | Timestamp order. Default `desc`. |
| `--limit <integer>` | ASCII decimal integer from 1 through 1000. Default `100`. |
| `--timeout <duration>` | Positive Go duration. Default `30s`. |
| `--cold` | Include cold storage with hot storage. Default: hot storage only. |

`bs sources get <id>` shows both parts of the full table identifier. Its `table_name` omits the team prefix.
A source ID or an unprefixed `table_name` is not a valid `--table` value. The command accepts no positional target or raw SQL.

The default window ends now and starts one hour earlier. Explicit bounds must satisfy `from < to`.
Times use millisecond parameters. Values use space-separated flags. Value-bearing flags reject duplicates. `--cold` may repeat.

`--query` consumes the next argument as text, even option-shaped text. `--query --cold` searches for the literal `--cold` without enabling cold storage.
Use `--cold --query --cold` to enable cold storage while searching for that text. Search text remains parameter data. Control characters are rejected.

HTTP 500 `CLUSTER_DOESNT_EXIST` means the source has no hot table. Try rerunning with `--cold`.
The command preserves the SQL error response policy and does not guarantee that cold storage succeeds.

Exact root, resource, and command help requires no credentials and sends no request. Invalid search arguments fail before SQL configuration or HTTP access.

## Telemetry SQL configuration

The SQL executor uses these separate environment variables.

| Variable | Role |
|---|---|
| `BETTERSTACK_TELEMETRY_SQL_URL` | Regional SQL host, `host:443`, or `https://host[:port]`. |
| `BETTERSTACK_TELEMETRY_SQL_USERNAME` | Nonempty HTTP Basic-auth username. |
| `BETTERSTACK_TELEMETRY_SQL_PASSWORD` | Nonempty HTTP Basic-auth password. |

All three values are required. SQL credentials never fall back to Uptime or Telemetry API tokens. Only the SQL host is configurable. Absolute HTTPS URLs allow an empty path or `/`, but no userinfo, query, or fragment.

The executor sends one `POST` with `Content-Type: plain/text` and a positive timeout, defaulting to 30 seconds. It follows no redirects and retries nothing. It writes 2xx bytes unchanged to stdout. Other SQL responses go to stderr after literal credential redaction, with exit status 1. Local failures use safe diagnostics. Uptime and source bodies remain unchanged.

The `BETTERSTACK_TELEMETRY_API_TOKEN` authenticates source commands, and log search uses it only to redact SQL error output.
See [source authentication](cmd/bs/sources.go) and [SQL error redaction](cmd/bs/logs.go).

## Telemetry sources

Source commands use only `BETTERSTACK_TELEMETRY_API_TOKEN`. Set this environment variable to your Telemetry API token before requesting sources. Uptime commands continue to use `BETTERSTACK_UPTIME_API_TOKEN` without credential fallback.

The production Telemetry REST endpoint is fixed at `https://telemetry.betterstack.com`. No flag, environment variable, or configuration file overrides this host. Source commands send one authenticated GET to `/api/v2/sources` or `/api/v2/sources/<escaped-id>`.

```sh
bs sources list
bs sources list --page 1 --per-page 50 --timeout 60s
bs sources get source-id
bs sources get --timeout 60s source-id
bs sources --help
bs sources list --help
bs sources get --help
```

- `--page` accepts a positive decimal integer.
- `--per-page` accepts a decimal integer from 1 through 50.
- Both pagination flags preserve repeated values. Absent flags send no pagination defaults.
- `get` requires one nonempty opaque source ID. The CLI escapes it as one path segment.
- `--timeout` accepts a positive Go duration and defaults to 30 seconds. The last valid repeated timeout applies.
- Flag values use the space-separated form. The get timeout can precede or follow the ID.

The CLI preserves response bytes without formatting, redaction, or automatic pagination. It writes 2xx bodies to stdout and other HTTP bodies to stderr. It does not follow redirects. Help requires no credentials and sends no request.

**Warning:** The output includes each source's `token` field. Treat it as a secret credential. Keep it private and do not paste it into shared logs or tickets.

See [source handlers](cmd/bs/sources.go), [help](cmd/bs/help.go), and [the shared request contract](.mem/uptime/requests.md).
