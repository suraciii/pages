# design — Design Spec

This layer describes how pages should be implemented: boundaries, data model,
interfaces, and trade-offs. Use the writing rules in
[`AGENTS.md`](AGENTS.md). Use the terms in [`../CONTEXT.md`](../CONTEXT.md).

[`docs/`](../docs/README.md) says what must be satisfied; `design/` says how.

## Annotation Conventions

The body is the spec, not a description of the current code. When a document
diverges significantly from the code, it lists a `Gap` section.

## Documents

- [AGENTS.md](AGENTS.md) — design-document writing rules for agents. Read it
  before writing a spec in `design/`.
- [architecture.md](architecture.md) — write path, read path, the identity
  boundary, and atomic replacement.
- [cli.md](cli.md) — the `pages` command: `serve`, `publish`,
  `generate-token`, and `skill` subcommands, flags, output, and exit codes.
- [deployment.md](deployment.md) — hosting the public root: host
  requirements, the Caddy reference, and the verification path.
- [testing.md](testing.md) — unit test strategy and the verification gate.

## Decision Records

Important trade-offs are written directly in the relevant document; no
separate ADR directory. Each trade-off must state what was rejected and what
it costs.
