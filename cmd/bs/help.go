package main

// Human-readable help text for milestone 001. Help is the one agreed exception
// to JSON-only output (spec section 1.3). Exact wording, formatting, and
// trailing newlines are implementation choices (spec section 1.7). Each
// per-command block carries a usage line, an arguments section, a flags
// section, and at least one example using the approved flag spellings.

const rootHelp = `bs - read-only Better Stack Uptime CLI

Usage:
  bs <resource> <action> [arguments] [flags]

Resources:
  incidents   List and inspect incidents
  monitors    List and inspect monitors

Run 'bs <resource> --help' for a resource's commands.
Run 'bs <resource> <action> --help' for a command's usage and flags.
`

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
