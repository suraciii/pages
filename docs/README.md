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
- [getting-started.md](getting-started.md) — one pass from zero to a live
  page: install, token, service, host, publish.
- [publishing.md](publishing.md) — the publish contract: Upload shape, public
  address, rejection rules, and limits.
- [configuration.md](configuration.md) — every configuration input: tokens
  file, serve flags, publish config file, and environment variables.
- [deployment.md](deployment.md) — running the service: systemd, Docker,
  and the Caddy reference.
