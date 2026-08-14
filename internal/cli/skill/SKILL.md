---
name: pages
description: Publish HTML pages and zip directory pages with the pages CLI. Use when the user wants to publish, upload, share, or update a report, artifact, dashboard, or web page; when they mention pages, a publish destination, a slug, a Token, or a public pages URL; or when a generated HTML file or zip should become a public Page.
---

# pages

Publish one HTML file or one zip directory Page. Each Publish replaces the
Page at the same Slug. Use `pages publish --help` for the exact flags and
defaults supported by this installed CLI.

## Publish

Local Publish writes directly to a Public Root. It does not use a service or
network:

```text literal
pages publish --file report.html --slug report
```

Use `--dest <path>` to select another local Public Root. An omitted
Destination uses the configured Destination, then the current directory.

Remote Publish uses an absolute HTTP(S) Destination and verifies the public
URL. Set `PAGES_UPLOAD_TOKEN` in the Publisher's process environment, then
run:

```text literal
pages publish --file report.html --slug report \
  --dest https://pages.example.com
```

A `.zip` file publishes a directory Page. Its archive root must contain
`index.html`.

The command prints exactly one result after the Page is in place. Local mode
prints the Page directory. Remote mode prints the verified public URL. Report
that printed result to the user.

## Identity

Publish uses the Default Identity unless configuration or `--identity`
selects a Named Identity. For remote Publish, `PAGES_UPLOAD_TOKEN` overrides
that selection: a pure secret selects the Default Identity and
`identity.secret` selects a Named Identity.

## Administration

Use `pages generate-token --help` to issue or rotate Tokens. Use
`pages serve --help` only when administering a remote Upload service. A local
Publish does not need `serve`.

## Failures

Read stderr and preserve the command's exit status. Usage errors exit 2.
Operational failures exit 1. Do not claim success unless `pages publish`
exits 0 and prints its result.
