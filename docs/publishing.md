# Publishing

A Publisher puts one Page in place and gets back where it lives.

## Two Ways To Publish

Destination selects the Publish mode. A local path writes into that Public
Root. An absolute HTTP(S) URL Uploads through the service and verifies the
public URL. When Destination is not set, the current working directory is the
Public Root.

Remote mode uses a Token and verifies the public URL:

```text literal
PAGES_UPLOAD_TOKEN='secret' pages publish \
  --file report.html --slug report --dest https://pages.example.com
https://pages.example.com/report/
```

Local mode writes directly into a Public Root. It needs no Token or service:

```text literal
pages publish --file report.html --slug report --dest pages-public
<current-directory>/pages-public/report/
```

When no Destination is configured, the Publisher writes to the current
directory:

```text literal
pages publish --file report.html --slug report
<current-directory>/report/
```

Local output uses the operating system's path separator. Windows output uses
`\` instead of `/`.

## Default And Named Identity

Identity is optional. Without a Named Identity, Publish uses the hidden
Default Identity:

- Its Token is one secret, with no Identity prefix.
- Its Page path is `<public-root>/<slug>/`.
- Its public URL is `<destination>/<slug>/`.

A Named Identity is an enhanced scope:

- Its Token is `identity.secret`.
- Its Page path is `<public-root>/@<identity>/<slug>/`.
- Its public URL is `<destination>/@<identity>/<slug>/`.

The `@` prefix keeps Named Identity scopes separate from Default Identity
Slugs. A Default Identity zip Page can contain any internal path without
overlapping a Named Identity.

Named remote example:

```text literal
PAGES_UPLOAD_TOKEN='bumble.secret' pages publish \
  --file report.html --slug report --dest https://pages.example.com
https://pages.example.com/@bumble/report/
```

Named local example:

```text literal
pages publish --file report.html --slug report --identity bumble \
  --dest pages-public
<current-directory>/pages-public/@bumble/report/
```

## Upload Contract

The server accepts exactly one Upload shape. Upload does not put an Identity
in the request path:

```text literal
POST <destination>/<slug>
Authorization: Bearer <secret> | Bearer <identity>.<secret>
Content-Type: text/html | application/zip
```

The server derives the Default or Named Identity from the verified Token. It
never takes Identity from the upload path, request body, or a custom header.
One Upload replaces exactly one Page.

A zip must contain `index.html` at its root. The server rejects archives with
symlinks, path tricks, duplicate names, hidden names, more than 512 entries,
or an uncompressed size over four times the upload limit.

The server rejects a body that is empty, not UTF-8, not a supported type, or
larger than the upload limit. A rejected Upload must not change the existing
Page.

## Reader Guarantees

A reader sees the complete old Page or the complete new Page. When a Page is
replaced, a request that lands exactly on the swap can see a brief absence.
Both Publish modes apply the same staging and swap rules.

Publish is best-effort durable. After a machine crash, the newest Page may be
missing or truncated. Publishing it again restores it.

## Limits

- Publish replaces a Page. There is no archive, revision, or history.
- The upload limit defaults to 10 MiB and applies to both shapes in remote
  mode. Local mode has no size limit; only the zip safety checks run.
- A Token publishes only inside its Default or Named Identity scope.
- Pages are static display documents. They must not need scripts.
