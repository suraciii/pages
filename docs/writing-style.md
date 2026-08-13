# Writing Style

Every document in this repository follows these rules: product docs in
`docs/`, design specs in `design/`, and agent instruction files.

## Language

- Write active prose in English.
- Use short sentences and American spelling.
- Use `must`, `may`, or `must not` for normative rules.
- Treat ASD-STE100 as a writing target, not a compliance claim.

## Terms And Structure

- Keep terms consistent with [`../CONTEXT.md`](../CONTEXT.md).
- Use one section for one purpose.
- Prefer lists to paragraphs and tables to lists.
- Define a rule once; other docs link it, never copy it.
- Keep commands and examples runnable as written.

## Markup

- Use `text diagram` fences for ASCII diagrams and `text literal` fences for
  command output, syntax, protocols, and user text. Bare `text` fences are
  invalid.
- Use ASCII only in diagrams. Do not use PlantUML, Mermaid, Unicode line art,
  or Unicode arrows.
- Do not use raw HTML, including HTML comments.
