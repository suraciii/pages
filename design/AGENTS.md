# Agents — Writing Design Specs

`design/` is the design spec layer: why the system has its boundaries and
which contracts an implementation must preserve. Write it for developers and
agents who implement the design, not for readers tracing the current code.

## Rules

- Write the spec before implementing. Current code does not decide the target
  design.
- Start with the problem, design forces, and chosen trade-off. Explain why
  the boundary exists before describing its mechanics.
- Keep models minimal: only concepts and fields the current behavior needs.
- Define what a concept is and what it is not. State ownership, scope,
  identity, lifecycle, and invariants.
- Write deterministic rules: order, timing, write targets, failure behavior.
  Reject illegal states; do not fail silently.
- Do not rewrite classes, methods, call chains, or storage operations as
  prose. Mention a code symbol only when it names a durable boundary or links
  a current gap to its source.
- One noun, one meaning. Define one rule in one doc; other docs link it.
- Technical language is allowed here. Define terms a new reader may not know.
- Use the minimal structure: Design Drivers / Model / Semantics / Examples /
  Status.
- The body is the target design; implementation gaps go to Status.
- Follow the language, structure, and markup rules in
  [`../docs/writing-style.md`](../docs/writing-style.md).

Full conventions: [`README.md`](README.md).
