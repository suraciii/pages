# Agents — Writing Product Docs

`docs/` is the product specification layer. It defines what pages must
satisfy. Write it for users and agents who do not read the source code.

## Rules

- Write the spec before implementing.
- Lead with the user problem and why the product behavior exists. Explain the
  constraint or trade-off that makes a rule necessary before listing the
  rule itself.
- Describe the product model and the visible contract. Do not turn classes,
  methods, handlers, or storage steps into prose.
- Use one section for one purpose. Prefer lists to paragraphs and tables to
  lists.
- Define a rule once; other docs link it, never copy it.
- Keep commands and examples runnable as written.
- The body is the spec. Put divergence in a `Gap` section.
- Keep terms consistent with [`../CONTEXT.md`](../CONTEXT.md).
- Follow the language, structure, and markup rules in
  [`writing-style.md`](writing-style.md).

Full conventions: [`docs/README.md`](README.md).
