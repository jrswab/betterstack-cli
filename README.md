# betterstack-cli

`bs` is a read-only CLI for Better Stack Uptime and Telemetry APIs.

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
