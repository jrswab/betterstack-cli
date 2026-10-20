# Uptime incidents

## Target resolution

[resolveIncidentTarget](../../cmd/bs/incidents.go) performs local parsing without network access. A scheme-less target remains an opaque bare ID, including lookalike paths and protocol-relative strings. A malformed scheme-less target also remains opaque. The command handler subsequently escapes the candidate as one API path segment.

A target with a scheme must parse as the dashboard URL form `https://incidents.betterstack.com/team/<team-id>/incidents/<incident-id>`:

- The parsed scheme is `https`. Go URL parsing normalizes an uppercase scheme.
- The host matches `incidents.betterstack.com` exactly. Uppercase hosts, trailing dots, and explicit ports fail.
- The URL contains no userinfo, opaque URL component, query, or fragment. Empty `?` and `#` markers also fail.
- The decoded path has exactly the `team`, nonempty team ID, `incidents`, and terminal incident ID segments. Trailing slashes and decoded slashes inside IDs fail.
- The terminal incident ID contains one or more ASCII digits. Percent-encoded digits decode successfully. Unicode digits do not qualify.

Malformed URLs with a leading scheme are rejected through `leadingScheme` when `url.Parse` fails. Bare IDs have no numeric-only restriction. A dashboard URL supplies only the terminal ID, not a configurable API host or a lookup request.

This local example resolves a dashboard URL to `"123"` with a nil error:

```go
candidate, err := resolveIncidentTarget("https://incidents.betterstack.com/team/t1/incidents/123")
```

Get uses `/api/v3/incidents/<escaped-id>`. Timeline appends `/timeline` and shares the same parser. Timeline bodies remain raw, including `timeline_truncated` items. The CLI does not promise a complete history or fetch additional pages.

## Incident-list booleans and metadata

`--resolved` and `--acknowledged` require exact lowercase `true` or `false`. Both `--resolved false` and `--resolved=false` forms work, with equivalent acknowledged forms. A bare boolean flag is invalid.

`--metadata` consumes the next argument as `key=value`. `parseMetadataPair` splits at the first `=`. The key must be nonempty. The value can be empty or contain more equals signs. `--metadata=key=value` is not supported.

`duplicateMetadataKey` rejects the first exact, case-sensitive repeated key before the credential check or request. `Origin` and `origin` are distinct. Ordinary filter occurrences remain repeatable.

The query builder maps each pair to `metadata[<key>][][value]` with `url.Values.Add`. For `--metadata Origin=Linear`, the encoded query is:

```text
metadata%5BOrigin%5D%5B%5D%5Bvalue%5D=Linear
```

Other incident-list flags and their mappings appear in [commands](commands.md). Shared credentials and raw output follow [requests](requests.md). [Testing](testing.md) links the URL and query acceptance tests. See [summary](summary.md).
