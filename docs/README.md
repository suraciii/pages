# docs — Product Spec

This layer describes what pages must satisfy: the publish contract, the user
model, and the boundaries of responsibility. Write it in product language for
users who do not read the source. Implementation terms belong to
[`../design/`](../design/README.md).

Use the terms in [`../CONTEXT.md`](../CONTEXT.md). Follow
[`_agents.md`](_agents.md) before editing this layer.

## Annotation Conventions

`docs/` describes the target state, not the current state. When a document
and the implementation diverge significantly, the document gets a `Gap`
section. The body is the spec; the Gap is the footnote.

## Documents

- [_agents.md](_agents.md) — writing rules for agents that edit this layer.
- [publishing.md](publishing.md) — the publish contract: Upload shape, public
  address, rejection rules, and limits.
- [configuration.md](configuration.md) — worked deployment examples: tokens
  file, `pages serve`, Caddyfile, Docker, and publish commands.
