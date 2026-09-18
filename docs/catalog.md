# Page Index

Pages needs a predictable starting point for readers who want to see the
Pages that are currently published. A Page Index provides that starting point
without adding a read API, a database, or publication history.

## Visible behavior

The Public Root serves a Page Index at `/`. A Named Identity serves its own
Page Index at `/@<identity>/`. Each Index links to the current Page roots and,
when Named Identities exist, provides the same Identity navigation:

```text literal
Pages

Default Identity   Current   /
@bumble/           Named Identity   /@bumble/

Default Identity

report/       Page       /report/
status/       Page       /status/
```

An Identity Index lists only the Pages in that Identity and includes the
Identity navigation plus a link to its parent Index. The current Identity is
marked `Current`. A Page link opens the Page at its trailing-slash URL. The
Index does not list files inside a Page.

## Rules

- Pages without a Named Identity belong to the Default Identity. The root
  Index labels their group `Default Identity`; their URLs stay unscoped.
- When Named Identities exist, the root Index shows an Identity navigation
  block first. Default Identity is the first entry and links to `/`; Named
  Identity entries follow. The Default Identity Page list comes after this
  block, so scope navigation stays visible when it has many Pages.
- Every Identity Index uses the same navigation. Its current Identity links to
  itself, and links to other scopes use relative URLs.
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
