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
- A reader must be able to tell which Page was published most recently without
  reading a separate manifest. The Index shows the Published Time of each Page
  and uses the same value for the list order.
- A program must be able to read the current Pages without parsing HTML. The
  Page Record carries the same entries in a structured form.
- The first version must stay small. It has no public API, CLI command,
  metadata model, or customization surface.

## Model

**Page Index** — a generated static `index.html` that links to the current
  Page roots in one scope. It is a system artifact, not a Page and not an
  Upload target.

**Page Record** — a generated static `index.json` that carries the same Page
  entries as the Index in one scope in a structured form. It is a system
  artifact with the same ownership and replacement rules as the Index. It is
  not an API response: it is a static file the host serves.

The root Index is `<public-root>/index.html`. A Named Identity Index is
`<public-root>/@<identity>/index.html`. A Page remains at
`<public-root>/<slug>/index.html` or
`<public-root>/@<identity>/<slug>/index.html`.

The root Page Record is `<public-root>/index.json` and a Named Identity Page
Record is `<public-root>/@<identity>/index.json`.

The Default Identity has no separate directory. Its Pages are listed under
`Default Identity` in the root Index. A root Index may list Named Identity
directories in its Identity navigation block.

When Named Identities exist, every Index renders the same Identity navigation
block first. It lists the Default Identity first, then Named Identity entries;
each entry is a name and a link, and the current scope links to itself. No
entry carries a state label: a reader is already on the current scope, and a
label adds no information the name does not. The root Default Identity Page
list follows this block, so scope navigation stays visible when the Default
Identity contains many Pages.

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
no script, external resource, or user-provided title.

### Published Time

Every Page entry shows its Published Time next to the Slug link. The Page
Index orders Pages by the same value, so the reader sees the list order and
each time agree.

Published Time is the Page root `index.html` modification time. The renderer
never converts to UTC and never invents a zone: a reader sees the same
wall-clock time that `ls -l` shows on the server. The value has minute
precision, because a file modification time carries nanoseconds that no
reader can act on.

Published Time is a display value, not a contract. Pages that a different tool
wrote before this feature existed keep their current file times. A Page whose
file time changes outside Publish shows a different Published Time after the
next refresh; no migration or manifest repairs it.

The time is rendered as a `<time>` element whose `datetime` attribute carries
the exact value and whose text carries the reader-facing form. The rendered
text is minute precision and carries no zone suffix: it is the host's
wall-clock time, the same value `ls -l` shows, and a suffix would repeat what
the times already imply. A reader who needs the exact zone reads the
`datetime` attribute or the Page Record, both of which carry it.

### Page Record

The Page Record is JSON with one object per scope. It carries `pages`, an
array of objects with `slug` and `published`. The `published` value is the
exact Published Time in RFC 3339 form, so a caller can order or filter
without re-reading the file system.

The Record lists the same Pages in the same order as its Index. It carries no
Page content, size, asset list, or Identity navigation. An empty scope
renders an empty `pages` array, not a message.

The Record names no scope. Its path already says which scope it belongs to,
and a scope field would repeat the path on every entry. A Record is
recognized as a pages artifact by its serialized prefix, so a foreign
`index.json` is never replaced.

A Record and its Index are generated from one scan in one refresh, so they
cannot disagree about which Pages exist or what order they take.

### Refresh

Pages refreshes the Indexes and Page Records after `pages serve` recovers
staging at startup and after a successful Page swap. Local Publish and server
Upload use the same generator. A refresh scans the complete Public Root, so
the root and all Identity artifacts converge to the same snapshot.

Before replacing any target, refresh checks every existing target. An existing
`index.html` must contain the Page Index marker and an existing `index.json`
must begin with the Page Record marker. A Record is matched at the start
because JSON that embeds the `pages` key inside a nested object is a foreign
file; an Index is matched anywhere in the document because the host may have
wrapped the generated HTML. A different file is preserved and refresh fails
without changing any artifact. The operator must move that file before
enabling Page Index generation.

Refresh uses a temporary file under `.pages/` and an atomic replacement for
each target. The targets are not one filesystem transaction: if a later
target fails, earlier targets may already be replaced. A successful Page swap
is not rolled back because a refresh fails; the failure is reported to the
caller or service log and the next startup refresh repairs any mixed state.

The artifacts are refreshed under one Public Root lock so two local Publishers
cannot interleave Index writes. The lock excludes other processes; refreshes
inside one process are serialized separately, because a POSIX record lock is
owned by the process rather than the file descriptor. Existing Page swap
locking and its caller responsibility for concurrent local Publish remain
unchanged.

### Ownership

The Index and the Page Record are system artifacts. Every refresh replaces
them from the current Public Root, so a manual edit to either one is lost.
The replacement is not reported as a change to the Public Root's Page set: a
refresh that produces byte-identical output writes nothing new, and a refresh
after a Page swap writes the new Page set.

A refresh that cannot write its targets leaves the previous artifacts in
place. Readers then see an Index that does not list the newest Page. The
failure appears in the service log for `pages serve` and in the Publisher's
error output for local Publish. No signal reaches a reader of the stale
Index; the operator owns that check.

### Boundaries

The static host serves the generated files. `pages serve` does not answer
Index GET requests. `.pages/` remains private and is never an Index entry.
The Index does not expose Page assets or any token/configuration state. The
Page Record carries no Page content, so it exposes no more than the Index
does.

## Examples

```text diagram
<public-root>/
├── index.html                 <- root Page Index
├── index.json                 <- root Page Record
├── report/index.html         <- Default Identity Page
└── @bumble/
    ├── index.html             <- @bumble Page Index
    ├── index.json             <- @bumble Page Record
    └── overview/index.html    <- Named Identity Page
```

The root Index links `report/` to `./report/` and `@bumble/` to `./@bumble/`.
The `@bumble` Index links `overview/` to `./overview/` and its parent to
`../`.

One root Index entry:

```text literal
report/   2026-10-16 21:02   ./report/
```

The matching root Page Record entry:

```text literal
{"slug": "report", "published": "2026-10-16T21:02:13+08:00"}
```

## Status

Implemented. The scanner, renderer, atomic replacement, startup refresh, and
Publish integration are covered by unit tests.
