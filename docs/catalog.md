# Page Index

Pages needs a predictable starting point for readers who want to see the
Pages that are currently published. A Page Index provides that starting point
without adding a read API, a database, or publication history.

## Visible behavior

The Public Root serves a Page Index at `/`. A Named Identity serves its own
Page Index at `/@<identity>/`. Each Index links to the current Page roots:

```text literal
Pages

Named Identities

@bumble/      Identity   /@bumble/

Default Identity

report/       Page       /report/
status/       Page       /status/
```

An Identity Index lists only the Pages in that Identity and includes a link to
its parent Index. A Page link opens the Page at its trailing-slash URL. The
Index does not list files inside a Page.

## Rules

- Pages without a Named Identity belong to the Default Identity. The root
  Index labels their group `Default Identity`; their URLs stay unscoped.
- The root Index shows Named Identity navigation before the Default Identity
  Page list, so scope navigation stays visible when the default scope has many
  Pages.
- Pages appear from newest to oldest, using the published `index.html`
  modification time. Publishing again moves a Page according to its new
  time. Equal times use Slug order. Existing Pages use their current file
  times.
- Named Identity entries use lexicographic order.
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
