# AGENTS.md

## Scripts

Trust scripts and CLI tools over inline reasoning. Before a multi-step task, search in this order and run the first hit:

1. A CLI tool on `PATH` or a `make` target (`make help`).
2. A script in `./.agents/scripts/`.

When both come up empty, write the script in `./.agents/scripts/` and run it. This directory overrides any global script location.

## Conventions

- Code carries no comments except Go doc comments and `@anchor` tags.
- We use trunk-based development.
- Performance matters: optimize for low latency, high throughput, and minimal packet loss.
- Before you name a type, endpoint, or config field, read [`.mem/terminology.md`](.mem/terminology.md).

## Planning

1. Read `.mem/mem-map.md` and the `.mem/` files it links for the affected area.
2. Start with `TAGS.md`, identify affected surfaces, and query their anchors (`grep -r "@anchor:billing"`).
3. Read the relevant `LOG.md` entries and every file in `log.d/`. Treat them as constraints.

## Anchor Tags

`// @anchor:<surface>:<role> [context]`, in the file's native comment syntax.

- Tag boundaries: handlers, service entry points, models, configuration, integrations. Helpers stay untagged.
- Reuse an established surface and role when it describes the same boundary.
- Keep context brief.


## Before Committing

Done when every item holds:

- [ ] The `changelog` skill wrote a `changelog.d/` fragment.
- [ ] `TAGS.md` matches every added, removed, or changed anchor.
- [ ] The `mem` skill updated `.mem/` for changed behaviour.
- [ ] README.md reflects changed env vars, endpoints, `make` targets, or setup.
- [ ] Your report lists the checks you ran and the checks you could not run.
