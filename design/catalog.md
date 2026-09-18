# Page Index Design

## Design Drivers

- The public read path stays in the static host. `pages serve` continues to
  own Uploads and the loopback write path only.
- The design must work without a database or a publication manifest. The
  Public Root is the source of truth for current Pages.
- Readers must not see a partially written Index. Index replacement must use
  the existing Public Root staging boundary.
- The output must remain valid under the reference Content Security Policy and
  must not execute Page-provided code.
- The first version must stay small. It has no public API, CLI command,
  metadata model, or customization surface.

## Model

**Page Index** — a generated static `index.html` that links to the current
  Page roots in one scope. It is a system artifact, not a Page and not an
  Upload target.

The root Index is `<public-root>/index.html`. A Named Identity Index is
`<public-root>/@<identity>/index.html`. A Page remains at
`<public-root>/<slug>/index.html` or
`<public-root>/@<identity>/<slug>/index.html`.

The Default Identity has no separate directory. Its Pages are listed under
`Default Identity` in the root Index. A root Index may list Named Identity
directories in its Identity navigation block.

When Named Identities exist, the root Index renders an Identity navigation
block first. It lists the Default Identity first, then Named Identity entries.
The Default Identity Page list follows this block, so scope navigation stays
visible when the Default Identity contains many Pages.

## Semantics

### Scan

The scanner reads direct children of the Public Root and ignores `.pages` and
dot-prefixed entries. A Default Identity Page is a direct directory whose
name is a valid Slug and whose root contains `index.html`. A Named Identity is
a direct directory whose name is `@` followed by a valid Identity; its Index
lists the valid Page directories below it.

Invalid or incomplete entries are ignored. The scanner never follows a Page's
child directories while building an Index.

Pages are sorted by their root `index.html` modification time, newest first,
then by Slug when times are equal. Both HTML and zip Publish write a new
`index.html`; neither preserves source timestamps. This file time is the
publication ordering basis, not a record of the exact Page swap time. It also
orders existing Pages without a migration or publication manifest. Named
Identity entries are sorted by name.

Links are relative to the Index location and end in `/`. Names are HTML-escaped
before they are written.

The generated document is plain HTML with a small inline style. It contains
no script, external resource, user-provided title, timestamp, or metadata.

### Refresh

Pages refreshes the Index after `pages serve` recovers staging at startup and
after a successful Page swap. Local Publish and server Upload use the same
generator. A refresh scans the complete Public Root, so the root and all
Identity Indexes converge to the same snapshot.

Before replacing any target, refresh checks every existing target. An existing
file must contain the Page Index marker. A different file is preserved and
refresh fails without changing any Index. The operator must move that file
before enabling Page Index generation.

Refresh uses a temporary file under `.pages/` and an atomic replacement for
each target. The root and Identity Indexes are not one filesystem transaction:
if a later target fails, earlier targets may already be replaced. A successful
Page swap is not rolled back because an Index refresh fails; the failure is
reported to the caller or service log and the next startup refresh repairs any
mixed state.

The root and Identity Indexes are refreshed under one Public Root lock so two
local Publishers cannot interleave Index writes. Existing Page swap locking
and its caller responsibility for concurrent local Publish remain unchanged.

### Boundaries

The static host serves the generated files. `pages serve` does not answer
Index GET requests. `.pages/` remains private and is never an Index entry.
The Index does not expose Page assets or any token/configuration state.

## Examples

```text diagram
<public-root>/
├── index.html                 <- root Page Index
├── report/index.html         <- Default Identity Page
└── @bumble/
    ├── index.html             <- @bumble Page Index
    └── overview/index.html    <- Named Identity Page
```

The root Index links `report/` to `./report/` and `@bumble/` to `./@bumble/`.
The `@bumble` Index links `overview/` to `./overview/` and its parent to
`../`.

## Status

Implemented. The scanner, renderer, atomic replacement, startup refresh, and
Publish integration are covered by unit tests.
