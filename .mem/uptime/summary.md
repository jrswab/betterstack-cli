# Uptime summary

The [command package](../../cmd/bs/main.go) implements five read-only Uptime commands. `main` supplies the fixed production base URL and an HTTP client. `runWithAPI` accepts those dependencies and the stdout/stderr writers so tests can use local servers. It delegates to `runWithEndpoints`, which accepts distinct Uptime, Telemetry REST, and SQL endpoint inputs. Uptime and [source REST handlers](../telemetry/sources.md) are active. The SQL input remains unused. `helpFor` runs before command routing.

```mermaid
flowchart TD
    Main[main] --> Router[runWithAPI]
    Router --> Endpoints[runWithEndpoints]
    Endpoints --> Help{helpFor matches?}
    Help -->|yes| Text[Help text to stdout, exit 0]
    Help -->|no| Route{Known command?}
    Route -->|no| Usage[Usage error to stderr, exit 1]
    Route -->|yes| Handler[One of five command handlers]
    Handler --> Validate[Parse arguments and resolve incident targets]
    Validate --> Credential[Check Uptime token and build endpoint]
    Credential --> Request[doMonitorsRequest]
    Request --> GET[Bearer GET with timeout, redirects disabled]
    GET --> Body[Read raw response body]
    Body --> Status{2xx?}
    Status -->|yes| Success[stdout, exit 0]
    Status -->|no| Failure[stderr, exit 1]
    Validate -->|invalid| Error[Local redacted diagnostic, exit 1]
    Credential -->|missing token| Error
    GET -->|request failure| Error
    Body -->|read failure| Error
```

The monitors-list route in `runWithEndpoints` delegates to its handler with the remaining arguments and injected Uptime dependencies:

```go
if len(args) >= 2 && args[0] == "monitors" && args[1] == "list" {
	return monitorsList(args[2:], stdout, stderr, uptimeBaseURL, client)
}
```

[Commands](commands.md) maps every route to its endpoint and help boundary. [Requests](requests.md) defines the common output and credential contract. [Incidents](incidents.md) covers target and metadata rules. [Testing](testing.md) describes loopback-only checks. The [memory map](../mem-map.md) indexes the complete set.
