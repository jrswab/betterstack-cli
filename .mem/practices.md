# Practices

## Command boundaries

The command package uses the Go standard library without third-party dependencies. [go.mod](../go.mod) declares the module and Go version. `main` supplies the production endpoint, client, and output writers to the Command routing boundary (`runWithAPI`). `runWithAPI` delegates to the Endpoint injection seam (`runWithEndpoints`), passing the Uptime base URL, the fixed Telemetry REST host, and an empty SQL URL. The seam accepts separate test inputs for all three service destinations. Uptime and Telemetry REST routes use their separate endpoints. The SQL input remains unused. Tests supply loopback endpoints through either interface. There is no runtime endpoint override flag, environment variable, or configuration file.

Handlers keep argument parsing and query translation separate from HTTP execution. `url.Values` encodes query parameters, and `url.PathEscape` escapes target path segments. All handlers share the [request contract](uptime/requests.md). API validation handles opaque Uptime filter values except locally validated booleans, metadata grammar, and positive timeout durations. [Sources](telemetry/sources.md) also validate digit-only pagination and the per-page maximum of 50.

## Checks and test safety

The repository has no Makefile, documentation formatter, or Markdown linter configuration. Direct checks validate memory paths, relative links, code examples, diagrams, line limits, and changelog shape. Go checks use the existing command package:

```sh
GOFLAGS=-o=/dev/null go build ./...
go vet ./...
go test ./...
gofmt -l cmd/bs/*.go
```

Tests use dummy tokens and loopback `httptest` servers. `TestMain` clears inherited credentials and installs the network guard. A custom transport can bypass that guard. [Testing](uptime/testing.md) documents the Command routing boundary, the loopback-only network guard, and the guard's limits.

## Repository records

`AGENTS.md` defines repository conventions. Memory records present behavior and links back to source. Changelog fragments use a category heading and factual bullets with repository paths. Historical plans are not the source of current runtime behavior.

Related: [memory map](mem-map.md), [terminology](terminology.md), and [commands](uptime/commands.md).
