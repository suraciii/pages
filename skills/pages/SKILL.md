---
name: pages
description: Publish standalone HTML pages and zip directory pages to a static site. Use when the user wants to publish, upload, share, or update a report, artifact, dashboard, or web page; when they mention a pages publish, an upload token, a slug, or a public pages URL; or when a generated HTML file or zip should become a public page.
allowed-tools: Bash(pages:*)
---

# pages

Publish Pages over HTTP. A Page is one standalone `index.html` file or
one directory of files, live at `<base-url>/<identity>/<slug>/`. The
base URL is the publisher's remote address, fully custom.

Three commands:

- `pages publish` — upload or write a Page (publisher side)
- `pages generate-token` — issue an upload Token (service side)
- `pages serve` — the publish HTTP service (service side)

Install the binary from the pages repository. Publish your own work with
`pages publish`; run `pages generate-token` and `pages serve` only when
you administer the service.

## Publish

Two modes. The remote address is the only mode switch: `-base-url`
selects remote mode; without it, publishing is local.

### Remote (through the service)

The upload Token is `identity.secret`. Supply it as `PAGES_UPLOAD_TOKEN`
(its prefix decides the Identity) or store it in the config file:

```bash
PAGES_UPLOAD_TOKEN=bumble.7v9A... pages publish \
  -file report.html -slug report -base-url https://pages.example.com
```

Success prints the public URL:

```text
https://pages.example.com/bumble/report/
```

### Local (no service)

Write straight into a public root; the machine must have write access:

```bash
pages publish -file report.html -slug report -identity bumble
```

Success prints the local directory:

```text
/srv/pages/public/bumble/report/
```

The local target comes from `PAGES_PUBLIC_ROOT` or the config file.

### Directory pages

A `.zip` file publishes a directory page. The root file must be
`index.html`; relative links inside the page work normally:

```bash
pages publish -file report.zip -slug report -base-url https://pages.example.com
```

## Input resolution

Flag beats environment beats config file. The config file sits at
`~/.config/pages/config.json`; with exactly one token entry, the
identity is implied:

```json
{
  "base-url": "https://pages.example.com",
  "tokens": { "bumble": "secret" },
  "identity": "bumble"
}
```

With this file, `pages publish -file report.html -slug report` is
complete on its own.

## Rules and limits

- `-file` must end in `.html` or `.zip` and must not be a directory.
- Slug: lowercase letters, digits, and hyphens; first char is a letter
  or digit; at most 63 chars.
- Zip: root file `index.html`, at most 512 entries, entries must not
  escape the page directory, names must be UTF-8. Path traversal and
  symlinks are rejected.
- Upload limit: `PAGES_MAX_UPLOAD_BYTES`, default 10485760.
- Identity: same name rules as the slug.
- Re-publishing the same slug replaces the old Page atomically; readers
  see the complete old or complete new Page.

## Troubleshooting

`pages publish` exits 1 and prints the reason:

| Output | Meaning | Fix |
| --- | --- | --- |
| `usage:` message | command line or file problem | follow the message: slug shape, file extension, missing identity or public root |
| `401` | upload Token rejected | Token is `identity.secret`; the identity must exist in the service tokens file, the secret must be exact |
| `404` | request path wrong | publish to `POST /<slug>`; check the base URL |
| `400` | body or zip rejected | see the server message; zip: missing `index.html`, traversal, non-UTF-8 names, too many entries |
| `413` | body too large | raise the limit on the service, or shrink the page |
| `503` | storage problem on the service | retry; the service keeps the previous Page when a swap fails |
| timeout | verification did not finish in time | retry with a higher `-timeout`; the Page may still be live |

After a successful publish, verify by opening the printed URL. The
static host may add a short delay before the new Page is readable.
