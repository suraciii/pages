---
name: pages
description: Publish HTML pages and zip directory pages to a public site. Use when the user wants to publish, upload, share, or update a report, artifact, dashboard, or web page; when they mention a pages publish, an upload token, a slug, or a public pages URL; or when a generated HTML file or zip should become a public page.
allowed-tools: Bash(pages:*)
---

# pages

Publish one `index.html` file, or one directory of files, to a public
site. Each publish replaces the Page at the same slug. The live address
is `<destination>/<slug>/` for the Default Identity or
`<destination>/@<identity>/<slug>/` for a Named Identity.

## Publish (the core action)

Remote mode — the service accepts the upload and the command verifies
the live address:

```bash
PAGES_UPLOAD_TOKEN=7v9A_example-secret pages publish \
  --file report.html --slug report --dest https://pages.example.com
```

Local mode — no service; the machine needs write access to a public
root:

```bash
pages publish --file report.html --slug report
```

A `.zip` file publishes a directory page; the root file must be
`index.html`, and relative links inside the page work normally.

The command prints the live address when the Page is in place. Treat
that printed address as the verification: if a printout follows the
publish, the Page is live.

## How the inputs work

Destination is the only mode input. Use a local path for local Publish and an
absolute HTTP(S) URL for remote Publish. A pure-secret Token uses the Default
Identity. An `identity.secret` Token uses a Named Identity. Local mode uses
the Default Identity unless `--identity` selects a Named Identity. Run
`pages publish --help` for every flag and default. The
[command contract](../../design/cli.md#pages-publish) defines the exact input
precedence and Destination syntax.

## When you administer the service

`pages generate-token` issues a Default Identity Token. An optional Identity
argument issues a Named Identity Token. The owner hands that Token to
publishers. `pages serve` runs the Publish service. Run
`pages <command> --help` for their flags.

## Failure handling

A failed publish exits 1 and prints one error line. Read it first.
Common cases:

- **Token rejected (`401`)** — use a pure secret for the Default Identity or
  `identity.secret` for a Named Identity. The Token must exist in the service
  tokens file.
- **Body rejected (`400`)** — follow the server message; a zip page
  needs `index.html` at its root.
- **Timeout** — retry; a slower service may need a longer `--timeout`.
