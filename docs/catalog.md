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

status/       Page   2026-10-16 21:02   /status/
report/       Page   2026-10-16 14:35   /report/
```

An Identity Index lists only the Pages in that Identity and includes the
Identity navigation plus a link to its parent Index. The current Identity is
marked `Current`. A Page link opens the Page at its trailing-slash URL. The
Index does not list files inside a Page.

## Page Record

The Public Root also serves a Page Record at `/index.json`, and a Named
Identity serves its own at `/@<identity>/index.json`. A Page Record carries
the same Page entries as its Index in a structured form, so a program can
read the current Pages without parsing HTML:

```text literal
{
  "identity": "",
  "pages": [
    {"slug": "status", "published": "2026-10-16T21:02:13+08:00"},
    {"slug": "report", "published": "2026-10-16T14:35:00+08:00"}
  ]
}
```

A Page Record lists the Pages in one scope only, in the same order as its
Index. The `published` value is the exact Published Time in RFC 3339 form.
`identity` is the empty string for the root Record and the Named Identity
name for an Identity Record. A Page Record has no field for Page content,
size, or assets.

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
- Every Page entry shows its Published Time, the same `index.html`
  modification time that orders the list. The time is shown to the minute,
  in the host's local time zone, and matches what `ls -l` reports for the
  Page directory.
- Named Identity entries use lexicographic order.
- The Index uses relative links so a URL path prefix remains valid.
- An empty Index states that no Page is published in that scope.
- The Index contains no scripts, external assets, versions, tags, search, or
  history. It shows the Published Time of each Page and nothing else about it.
- The Page Record carries the same entries as the Index and nothing more. It
  has no scripts and no links.
- The Index is public wherever the Public Root is public. It must not be
  treated as an access-control boundary.

Pages generates the root and Identity Indexes and Page Records from the
current Public Root. The existing static host serves them like every other
static file. The Publisher's output and the Upload contract do not change.

`index.html` at the Public Root and inside a Named Identity is reserved for a
Page Index, and `index.json` is reserved for a Page Record. Pages does not
replace an existing file that is not one of these; an operator must move that
file before enabling this feature.

The Index and the Page Record are system artifacts. Pages replaces them on
every refresh, so a manual edit to either one is lost on the next refresh.

## Status

Implemented. Page Index generation is part of `pages serve` startup and both
local and remote Publish paths.
