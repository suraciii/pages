# Publishing

A Publisher puts one Page in place and gets back where it lives.

## Two ways to publish

The remote upload address is the only mode switch: `-base-url` or
`PAGES_BASE_URL` selects remote mode. Without it, publishing is local,
into the public root from `PAGES_PUBLIC_ROOT` or the config file at
`~/.config/pages/config.json` (or `-config`).

Remote mode publishes through the running service, and the command
verifies the public URL:

```text literal
PAGES_UPLOAD_TOKEN='bumble.secret' pages publish -file report.zip -slug report -base-url https://pages.example.com
https://pages.example.com/pages/bumble/report/
```

Local mode publishes directly into a public root on the local machine.
The public root comes from `PAGES_PUBLIC_ROOT`. No service and no
network are involved; the Identity is named explicitly:

```text literal
PAGES_PUBLIC_ROOT=/srv/pages/public pages publish -file report.zip -slug report -identity bumble
/srv/pages/public/bumble/report/
```

## What a user does

1. Get an upload Token from the service owner. The Token decides the
   Identity: `bumble.secret` publishes under `bumble`. The owner issues
   Tokens with `pages generate-token`.
2. Publish a file with `pages publish`. The file is one standalone HTML
   document, or a zip with `index.html` at its root. `-base-url` or
   `PAGES_BASE_URL` publishes through the service; `PAGES_PUBLIC_ROOT`
   writes into that local public root. The Identity comes from the
   Token, or from `-identity` in local mode.
3. The command prints the public URL or the local page directory when
   the Page is in place.

## The Upload contract

The server accepts exactly one Upload shape:

```text literal
POST /pages/<slug>
Authorization: Bearer <identity>.<secret>
Content-Type: text/html | application/zip
```

A valid Upload replaces exactly `<public-root>/<identity>/<slug>`. The
Identity comes from the verified Token. It never comes from the upload
path or a header.

A zip must contain `index.html` at its root. The server rejects archives
with symlinks, path tricks, duplicate names, hidden names, more than 512
entries, or an uncompressed size over four times the upload limit.

The server rejects a body that is empty, not UTF-8, not a supported
type, or larger than the upload limit. A rejected Upload must not change
the existing Page.

## Reader guarantees

A reader sees the complete old Page or the complete new Page. When a
directory Page is replaced, a request that lands exactly on the swap can
see a brief absence. Both publish modes apply the same staging and swap
rules.

Publishing is best-effort durable: after a machine crash, the newest Page
may be missing or truncated. Publishing it again restores it.

## Limits

- Publishing replaces a Page. There is no archive, revision, or history.
- The upload limit defaults to 10 MiB and applies to both shapes in
  remote mode. Local mode has no size limit; only the zip safety checks
  run.
- One Token publishes only inside its own Identity.
- Pages are static display documents. They must not need scripts.

