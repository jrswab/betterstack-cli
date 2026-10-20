package main

const rootHelp = `bs - read-only Better Stack Uptime and Telemetry CLI

Usage:
  bs <resource> <action> [arguments] [flags]

Resources:
  incidents   List and inspect incidents
  monitors    List and inspect monitors
  sources     List and inspect Telemetry sources

Run 'bs <resource> --help' for a resource's commands.
Run 'bs <resource> <action> --help' for a command's usage and flags.
`

const sourcesHelp = `bs sources - commands

Usage:
  bs sources <command> [arguments] [flags]

Commands:
  list      List Telemetry sources
  get <id>  Get one Telemetry source

Run 'bs sources <command> --help' for a command's usage and flags.
`

const sourceSecretWarning = "The output includes each source's `token` field. Treat it as a secret credential. Keep it private and do not paste it into shared logs or tickets.\n"

const sourcesListHelp = `bs sources list - list Telemetry sources

Usage:
  bs sources list [flags]

Arguments:
  none - this command takes no positional target.

Flags:
  --page <integer>      Request a positive decimal page number. Repeatable. No default is sent.
  --per-page <integer>  Request 1 through 50 sources. Repeatable. No default is sent.
  --timeout <duration>  Total request timeout, a positive Go duration. Default 30 seconds (30s).
                        Repeated timeouts use the last valid value. Use space-separated values.

Examples:
  bs sources list
  bs sources list --page 1 --per-page 50 --timeout 60s

` + sourceSecretWarning

const sourcesGetHelp = `bs sources get - get one Telemetry source

Usage:
  bs sources get <id> [--timeout <duration>]

Arguments:
  <id>  One nonempty opaque source ID, escaped as one path segment.

Flags:
  --timeout <duration>  Total request timeout, a positive Go duration. Default 30 seconds (30s).
                        The flag can precede or follow the ID. Repeated timeouts use the last valid value.
                        Use space-separated values.

Examples:
  bs sources get source-id
  bs sources get --timeout 60s source-id

` + sourceSecretWarning

const incidentsHelp = `bs incidents - commands

Usage:
  bs incidents <command> [arguments] [flags]

Commands:
  list                  List incidents
  get <id-or-url>       Get one incident
  timeline <id-or-url>  Get one incident's timeline

Run 'bs incidents <command> --help' for a command's usage and flags.
`

const monitorsHelp = `bs monitors - commands

Usage:
  bs monitors <command> [arguments] [flags]

Commands:
  list      List monitors
  get <id>  Get one monitor

Run 'bs monitors <command> --help' for a command's usage and flags.
`

const incidentsListHelp = `bs incidents list - list incidents

Usage:
  bs incidents list

Arguments:
  none - this command takes no positional target.

Flags:
  --from <value>             Filter incidents created at or after this time.
  --to <value>               Filter incidents created at or before this time.
  --monitor-id <id>          Filter by monitor ID.
  --heartbeat-id <id>        Filter by heartbeat ID.
  --resolved <true|false>    Filter by resolved state. An explicit true or false is required.
  --acknowledged <true|false>
                             Filter by acknowledged state. An explicit true or false is required.
  --metadata <key=value>     Filter by metadata. Repeatable, one key=value pair per occurrence.
  --team-name <name>         Filter by team name. Passed through unchanged.
  --page <n>                 Request a specific page. Passed through unchanged.
  --per-page <n>             Request a page size. Passed through unchanged.
  --timeout <duration>       Total request timeout, a duration value. Default 30 seconds (30s).

Examples:
  bs incidents list
  bs incidents list --monitor-id 123 --resolved=false
  bs incidents list --metadata Origin=Linear --per-page 50
`

const incidentsGetHelp = `bs incidents get - get one incident

Usage:
  bs incidents get <id-or-url>

Arguments:
  <id-or-url>  An incident ID (including the bare numeric form) or a full
               dashboard URL of the form
               https://incidents.betterstack.com/team/<team-id>/incidents/<incident-id>

Flags:
  --timeout <duration>  Total request timeout, a duration value. Default 30 seconds (30s).

Examples:
  bs incidents get 123 --timeout 60s
`

const incidentsTimelineHelp = `bs incidents timeline - get one incident's timeline

Usage:
  bs incidents timeline <id-or-url>

Arguments:
  <id-or-url>  An incident ID (including the bare numeric form) or a full
               dashboard URL of the form
               https://incidents.betterstack.com/team/<team-id>/incidents/<incident-id>

Flags:
  --timeout <duration>  Total request timeout, a duration value. Default 30 seconds (30s).

Examples:
  bs incidents timeline 123
`

const monitorsListHelp = `bs monitors list - list monitors

Usage:
  bs monitors list

Arguments:
  none - this command takes no positional target.

Flags:
  --url <value>                Filter by monitor URL.
  --pronounceable-name <name>  Filter by pronounceable name.
  --team-name <name>           Filter by team name. Passed through unchanged.
  --page <n>                   Request a specific page. Passed through unchanged.
  --per-page <n>               Request a page size. Passed through unchanged.
  --timeout <duration>         Total request timeout, a duration value. Default 30 seconds (30s).

Examples:
  bs monitors list
  bs monitors list --pronounceable-name Example
`

const monitorsGetHelp = `bs monitors get - get one monitor

Usage:
  bs monitors get <id>

Arguments:
  <id>  A bare monitor ID only. No URL form is accepted.

Flags:
  --timeout <duration>  Total request timeout, a duration value. Default 30 seconds (30s).

Examples:
  bs monitors get 789
`
