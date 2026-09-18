# Page Index

Pages needs a predictable starting point for readers who want to see the
Pages that are currently published. A Page Index provides that starting point
without adding a read API, a database, or publication history.

## Visible behavior

The Public Root serves a Page Index at `/`. A Named Identity serves its own
Page Index at `/@<identity>/`. Each Index links to the current Page roots:

```text literal
Pages

report/       Page       /report/
status/       Page       /status/

Identities

@bumble/      Identity   /@bumble/
```

An Identity Index lists only the Pages in that Identity and includes a link to
its parent Index. A Page link opens the Page at its trailing-slash URL. The
Index does not list files inside a Page.

## Rules

- The Default Identity stays unnamed in the URL.
- Page and Identity entries use deterministic lexicographic order.
- The Index uses relative links so a URL path prefix remains valid.
- An empty Index states that no Page is published in that scope.
- The Index contains no scripts, external assets, timestamps, versions, tags,
  search, or history.
- The Index is public wherever the Public Root is public. It must not be
  treated as an access-control boundary.

Pages generates the root and Identity Indexes from the current Public Root.
The existing static host serves them like every other static file. The
Publisher's output and the Upload contract do not change.

`index.html` at the Public Root and inside a Named Identity is reserved for a
Page Index. Pages does not replace an existing file that is not a Page Index;
an operator must move that file before enabling this feature.

## Status

Implemented. Page Index generation is part of `pages serve` startup and both
local and remote Publish paths.
