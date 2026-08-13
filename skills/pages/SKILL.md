---
name: pages
description: Publish HTML pages and zip directory pages to a public site. Use when the user wants to publish, upload, share, or update a report, artifact, dashboard, or web page; when they mention a pages publish, an upload token, a slug, or a public pages URL; or when a generated HTML file or zip should become a public page.
allowed-tools: Bash(pages:*)
---

# pages

Publish one `index.html` file, or one directory of files, to a public
site. Each publish replaces the Page at the same slug. The live address
is `<remote>/<identity>/<slug>/`.

## Publish (the core action)

Remote mode — the service accepts the upload and the command verifies
the live address:

```bash
PAGES_UPLOAD_TOKEN=bumble.7v9A... pages publish \
  --file report.html --slug report --remote https://pages.example.com
```

Local mode — no service; the machine needs write access to a public
root:

```bash
pages publish --file report.html --slug report --identity bumble
```

A `.zip` file publishes a directory page; the root file must be
`index.html`, and relative links inside the page work normally.

The command prints the live address when the Page is in place. Treat
that printed address as the verification: if a printout follows the
publish, the Page is live.

## How the inputs work

`--remote` is the only mode switch: set it for remote mode, leave it
out for local mode. The upload Token `identity.secret` decides the
Identity. The local target and the defaults come from the environment or
the config file. Run `pages publish --help` for every flag, environment
variable, and default.

## When you administer the service

`pages generate-token` issues an upload Token for one Identity; the
owner hands that Token to publishers. `pages serve` runs the publish
service. Run `pages <command> --help` for their flags.

## Failure handling

A failed publish exits 1 and prints one error line. Read it first.
Common cases:

- **Token rejected (`401`)** — the Token must be `identity.secret`, and
  the identity must exist in the service tokens file.
- **Body rejected (`400`)** — follow the server message; a zip page
  needs `index.html` at its root.
- **Timeout** — retry; a slower service may need a longer `--timeout`.
