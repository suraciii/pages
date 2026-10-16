# Publishing

A Publisher puts one Page in place and gets back where it lives.

The Public Root also provides a Page Index at its root. Open the Destination
root to browse the current Pages and see when each Page was published. A Named
Identity has a Page Index at its `/@<identity>/` path.

## Two Ways To Publish

Destination selects the Publish mode. A local path writes into that Public
Root. An absolute HTTP(S) URL Uploads through the service and verifies the
public URL. When Destination is not set, the current working directory is the
Public Root.

Remote mode uses a Token and verifies the public URL. Set
`PAGES_UPLOAD_TOKEN` in the Publisher's process environment, then run:

```text literal
pages publish --file report.html --slug report \
  --dest https://pages.example.com
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

For a Named remote Publish, set `PAGES_UPLOAD_TOKEN` to a Token in
`identity.secret` form, then run:

```text literal
pages publish --file report.html --slug report \
  --dest https://pages.example.com
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

Local mode applies the same body rules: an empty file and a file that is not
UTF-8 are usage errors, and a rejected Publish must not change the existing
Page. The size limit is the one rule that differs. Local mode has no size
limit because the caller already owns the directory; a limit adds no security
there.

## Reader Guarantees

A reader sees the complete old Page or the complete new Page. When a Page is
replaced, a request that lands exactly on the swap can see a brief absence.
Both Publish modes apply the same staging and swap rules.

Publish is best-effort durable. After a machine crash, the newest Page may be
missing or truncated. Publishing it again restores it.

Publishing the same Slug again replaces that Page. The result is the newest
content, so a repeated Publish after a timeout is safe: it cannot create a
second Page or duplicate the effect.

## Partial Effects

A Publish can fail after the Page is already in place. The two cases:

- Remote mode: the Upload succeeded but the verification `GET` failed. The
  Page is live. The static host is the likely cause.
- Local mode: the Page swap succeeded but the Page Index refresh failed. The
  Page is live. A non-Page-Index file at `<public-root>/index.html` or
  `<public-root>/@<identity>/index.html` is the likely cause.

When this happens the command reports the failure and exits 1, but the Page
is in place. Check the printed Page path or URL before publishing again. The
next startup refresh of `pages serve` repairs a mixed Page Index.

## Limits

- Publish replaces a Page. There is no archive, revision, or history.
- The upload limit defaults to 10 MiB and applies to both shapes in remote
  mode. Local mode has no size limit; only the zip safety checks run.
- A Token publishes only inside its Default or Named Identity scope.
- Publish takes its input from a file. It does not read stdin, because a zip
  Upload needs a reader that supports random access.
- A Page may contain scripts, stylesheets, images, and data files. pages
  never executes them. The static host decides which scripts a Page may run,
  so the host's Content Security Policy is part of the published result.
- The Page Index and the Page Record that pages generates contain no scripts.
