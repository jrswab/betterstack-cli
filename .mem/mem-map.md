# Memory map

## Project

- [System summary](summary.md) describes the current executable and its boundaries.
- [Terminology](terminology.md) defines names used in the memory set.
- [Practices](practices.md) records implementation, verification, and documentation patterns.
- [Memory map](mem-map.md) indexes itself and every memory file except session scratch under `.mem/tmp/`.

## Telemetry

- [Telemetry summary](telemetry/summary.md) describes active routes, separate REST and SQL contracts, help, and test safety.
- [Sources](telemetry/sources.md) describes REST routing, credentials, pagination, opaque IDs, raw responses, and secret warnings.
- [Log search](telemetry/logs.md) describes the active logs search route, exact help, argument validation, typed SQL parameters, time bounds, the optional cold union, and the separate SQL HTTP executor.

## Uptime

- [Uptime summary](uptime/summary.md) shows routing and the shared request flow.
- [Commands](uptime/commands.md) lists the five handlers, endpoints, arguments, flags, and help forms.
- [Requests](uptime/requests.md) describes credentials, timeouts, redirects, and output behavior.
- [Incidents](uptime/incidents.md) describes dashboard URL resolution, metadata, and timeline behavior.
- [Testing](uptime/testing.md) describes the Command routing boundary and loopback-only protections.
