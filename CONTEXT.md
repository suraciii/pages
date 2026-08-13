# Pages Publishing

This context defines the product language used by pages. Use these terms in
product docs, design docs, examples, and new code names. Keep one term for
one concept.

## Pages

**Page**:
One published resource tree under a Slug and, optionally, a Named Identity.
The root file is `index.html`. A Page is one standalone HTML document or a
directory of files from a zip.
_Avoid_: file, article, site, document

**Identity**:
The namespace that owns a Page. The Default Identity is unscoped and never
appears in a Token, URL, or Page path. A Named Identity is an optional scope
shown as `@<identity>`. The server derives the Identity from the verified
Token, never from an upload path, request body, or custom header.
_Avoid_: user, account, owner, tenant

**Slug**:
The stable name of a Page inside the Default Identity or one Named Identity.
A Slug has at most 63 lowercase letters, digits, or hyphens, and starts with a
letter or digit.
_Avoid_: name, path, identifier

**Token**:
An upload credential. A Default Identity Token is one secret. A Named
Identity Token has `identity.secret` form. The server compares the secret
against the tokens file before it accepts an Upload.
_Avoid_: key, password, credential

## Publishing

**Upload**:
A `POST /<slug>` request with a Bearer Token and an HTML or zip
body. One Upload replaces exactly one Page.
_Avoid_: write, save, commit

**Publish**:
The complete client operation: put a Page in place and return where it
lives. Remote mode Uploads through the service and verifies the public
URL. Local mode writes directly into a Public Root and returns its path.
_Avoid_: deploy, release, push

**Destination**:
Where Publish puts a Page. A local path is a Public Root and selects local
mode. An absolute HTTP(S) URL is an Upload and Verification endpoint and
selects remote mode. An omitted Destination is the current directory.
_Avoid_: remote, target, mode switch

**Public Root**:
The directory that the static host serves. The server writes Pages under
`<slug>/index.html` for the Default Identity and
`@<identity>/<slug>/index.html` for a Named Identity.
_Avoid_: webroot, docroot

**Staging Area**:
The `.pages/` directory inside the Public Root. The server unpacks a new
Page there and swaps it into place. Readers see the complete old Page or the
complete new Page, except for a brief absence during replacement. The staging
area is never served.
_Avoid_: temp dir, spool, buffer

**Publisher**:
The `pages publish` subcommand that performs Publish.
_Avoid_: uploader, client

**Verification**:
A `GET` of the public URL after an Upload. The Publisher accepts the Page
only when the response is `200` with an HTML content type.
_Avoid_: check, probe, ping
