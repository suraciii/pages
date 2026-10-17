---
name: pages
description: Publish one HTML file or one zip directory Page with the pages CLI, to a local public root or a remote pages service. Use when a generated HTML file or zip must become a public Page, be shared at a URL, or be updated; when the user asks to publish, upload, or share a page, report, dashboard, or artifact; or when `pages publish` fails with a Token, identity, or upload error.
---

# pages

Checked against `pages` at `main` commit `4d9c094`, which is v0.0.1 plus
eight commits. Run `pages --version` to compare. v0.0.1 has different Token
error text, no `--identity` conflict check, no partial effect errors, and no
local Page Index refresh. Links are repository-root relative; from a checkout
they resolve as written. This document covers the recurring task. The
authoritative contract is the installed CLI and the documents it links to:

| Question | Where it is answered |
| --- | --- |
| Which product, and what are its limits? | [README.md](README.md) |
| How do I connect? Which flags, environment, precedence? | `pages publish --help`, then [configuration.md](docs/configuration.md) |
| Which operation fits my goal? | This document. Publishing one Page is the whole agent-facing surface. |
| Can this action happen now? | Identity, below. Remote mode needs a Token; local mode needs nothing. No object state gates a Publish: the same Slug is replaced, never refused. |

## Recommended path: publish one HTML file

Preconditions:

- `pages` is on `PATH` and `pages --version` runs.
- One `.html` file, or one `.zip` whose archive root holds `index.html`,
  exists. A directory is not an input; package it as a zip.
- Local mode writes into a public root you already own. Remote mode needs a
  reachable service and a Token.

Local mode. No Token, no identity, no service, no network:

```text literal
pages publish --file report.html --slug report --dest public
public/report/
```

Remote mode. The Destination is an absolute HTTP(S) URL, so
`PAGES_UPLOAD_TOKEN` must carry the Token. Put it in the process
environment, not on the command line:

```text literal
PAGES_UPLOAD_TOKEN=bumble.sek pages publish --file report.html --slug report \
  --dest https://pages.example.com
https://pages.example.com/@bumble/report/
```

Result check: exit 0 and exactly one printed result. Local mode prints the
Page directory; remote mode prints the verified public URL. Report that
result to the user. A non-zero exit is a failure, and the Page may still be
live, so read Failures before publishing again.

## Identity

1. Local mode needs no Token and no Identity. It never reads
   `PAGES_UPLOAD_TOKEN`.
2. Remote mode needs a Token in `PAGES_UPLOAD_TOKEN`, or in the publish
   config file. There is no Token flag; a secret must not enter shell
   history.
3. A Token in `identity.secret` form selects that Named Identity
   automatically. Do not also pass `--identity`.
4. A Token with no dot selects the Default Identity, whose URL path has no
   `@` segment.
5. Let the Token decide the scope. When `PAGES_UPLOAD_TOKEN` is set it
   decides the Identity and `--identity` is not needed. Passing
   `--identity` with a different scope is a usage error that exits 2 and
   publishes nothing: a pure-secret Token plus `--identity bumble` does not
   publish to the Default Identity. Passing `--identity` with the same
   scope as the Token is accepted.
6. Every Token error names the recovery. Use this table:

| stderr, after `pages publish: usage: ` | Cause | Do this |
| --- | --- | --- |
| `no upload token: set PAGES_UPLOAD_TOKEN to a secret, or identity.secret for a Named Identity` | Remote mode, no `PAGES_UPLOAD_TOKEN`, no config Token | Set the variable, or use a local `--dest` |
| `token must use secret or identity.secret format: set PAGES_UPLOAD_TOKEN to a secret, or identity.secret for a Named Identity` | The Token has a dot, but the identity or the secret is invalid | Paste the full Token exactly as `generate-token` printed it |
| `no upload token for identity "bumble": set PAGES_UPLOAD_TOKEN to bumble.<secret>, or add the identity to the config file passed with --config` | `--identity bumble`, or `config.identity`, with no secret for it | Set `PAGES_UPLOAD_TOKEN=bumble.<secret>`, or drop `--identity` |
| `--identity "bumble" conflicts with PAGES_UPLOAD_TOKEN, which already selects the Default Identity: drop --identity and let the Token decide the scope` | `--identity` names a scope other than the Token's | Drop `--identity`, or use a Token for the scope you want |

## Failures

Read stderr and keep the exit status. Usage errors exit 2; operational
failures exit 1. Never claim success unless the command exits 0 and prints
its result.

Two failures happen after the Page is already in place:

- `verify page: ...; the page is in place at <url>; check the static host`.
  The Upload returned `204` and the verification `GET` failed. The Page is
  live. Check the static host, then publish again if it was the cause.
- `refresh page index: ...; the page is in place at <path>`. A file that is
  not a Page Index or Page Record sits at `index.html` or `index.json`
  under the public root or an `@<identity>` scope. The Page is live. Move
  that file, then publish again.

Repeating the same Slug replaces the Page, so a repeat after a timeout is
safe. `upload page: unexpected status 401` means the service rejected the
Token and the Page did not change.

## Other commands

`pages generate-token` issues and rotates Tokens. `pages serve` administers
a remote Upload service. A local Publish needs neither. Slug and Destination
rules, the upload limit, retention, and the full contract are in
[publishing.md](docs/publishing.md) and [design/cli.md](../design/cli.md).
