# Practices

## Command boundaries

The command package uses the Go standard library without third-party dependencies. [go.mod](../go.mod) declares the module and Go version. `main` supplies the production endpoint, client, and output writers to the Command routing boundary (`runWithAPI`). `runWithAPI` delegates to the Endpoint injection seam (`runWithEndpoints`), passing the Uptime base URL, the fixed Telemetry REST host, and an empty SQL URL. The seam accepts separate test inputs for all three service destinations and checks exact help before routing. Uptime and Telemetry REST routes use their separate endpoints. `logs search` validates flags through `buildLogsSearchQuery` and passes SQL, typed parameters, and parsed timeout to `doLogsSQLRequest` through the SQL input. SQL alone reads a user-configurable URL from its required environment configuration. Tests supply guarded loopback endpoints through either interface. Uptime and REST hosts remain fixed.

Handlers keep argument parsing and query translation separate from HTTP execution. `url.Values` encodes query parameters, and `url.PathEscape` escapes target path segments. The seven Uptime/source command handlers share the [GET request contract](uptime/requests.md). `logsSearch` uses the [SQL request executor](telemetry/logs.md) with separate Basic auth and redaction only for non-2xx SQL bodies. API validation handles opaque Uptime filter values except locally validated booleans, metadata grammar, and positive timeout durations. [Sources](telemetry/sources.md) also validate digit-only pagination and the per-page maximum of 50.

## Checks and test safety

The repository has no Makefile, documentation formatter, or Markdown linter configuration. Direct checks validate memory paths, relative links, code examples, diagrams, line limits, and changelog shape. Go checks use the existing command package:

```sh
GOFLAGS=-o=/dev/null go build ./...
go vet ./...
go test ./...
gofmt -l cmd/bs/*.go
```

Tests use dummy tokens and loopback `httptest` servers. `TestMain` clears inherited API credentials and SQL configuration, then installs the network guard. A custom transport can bypass that guard. [Testing](uptime/testing.md) documents the Command routing boundary, the loopback-only network guard, and the guard's limits.

## Repository records

### Agent skill

[skills/bs/SKILL.md](../skills/bs/SKILL.md) is the self-contained agent skill for using all eight `bs` commands.
It covers installation, credentials, flags, raw output, pagination, workflows, and safety.
[README.md](../README.md#install-the-agent-skill) documents copying or symlinking `skills/bs` into the agent's skills directory.

Keep the skill current when commands, flags, defaults, environment variables, or output contracts change.
Verify it against [command sources](../cmd/bs/main.go), [help](../cmd/bs/help.go), [Uptime](uptime/commands.md), and [Telemetry](telemetry/summary.md).
Build the current binary with the Go 1.26 toolchain or newer and capture exact help without credentials.
Remove the output path first and require a successful build before execution so a stale binary cannot be reused.
Every verification invocation uses `env -i PATH="$PATH" HOME="$HOME"` to exclude inherited credentials and SQL configuration:

```sh
rm -f /tmp/bs-fix && GOPROXY=off GOTOOLCHAIN=go1.26.10 go build -o /tmp/bs-fix ./cmd/bs &&
    test -x /tmp/bs-fix && env -i PATH="$PATH" HOME="$HOME" /tmp/bs-fix logs search --help
```

### Record conventions

`AGENTS.md` defines repository conventions. Memory records present behavior and links back to source. Changelog fragments use a category heading and factual bullets with repository paths. Historical plans are not the source of current runtime behavior.

Related: [memory map](mem-map.md), [terminology](terminology.md), and [commands](uptime/commands.md).
