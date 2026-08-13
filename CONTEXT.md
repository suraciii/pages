# Pages Publishing

This context defines the product language used by pages. Use these terms in
product docs, design docs, examples, and new code names. Keep one term for
one concept.

## Pages

**Page**:
One published resource tree under an Identity and a Slug. The root file is
`index.html`. A Page is one standalone HTML document or a directory of
files from a zip.
_Avoid_: file, article, site, document

**Identity**:
The namespace that owns a Page. The server derives the Identity from the
verified Token, never from an upload path, request body, or custom header.
_Avoid_: user, account, owner, tenant

**Slug**:
The stable name of a Page inside an Identity. A Slug has at most 63 lowercase
letters, digits, or hyphens, and starts with a letter or digit.
_Avoid_: name, path, identifier

**Token**:
An upload credential in `identity.secret` form. The server compares the
secret against the token file before it accepts an Upload.
_Avoid_: key, password, credential

## Publishing

**Upload**:
A `POST /pages/<slug>` request with a Bearer Token and an HTML or zip
body. One Upload replaces exactly one Page.
_Avoid_: write, save, commit

**Publish**:
The complete client operation: put a Page in place and return where it
lives. Remote mode Uploads through the service and verifies the public
URL. Local mode writes directly into a Public Root and returns its path.
_Avoid_: deploy, release, push

**Public Root**:
The directory that the static host serves. The server writes Pages under
`<identity>/<slug>/index.html` in the Public Root.
_Avoid_: webroot, docroot

**Staging Area**:
The `.pages/` directory inside the Public Root. The server unpacks a new
Page there and swaps it into place, so readers see the complete old Page
or the complete new Page. The staging area is never served.
_Avoid_: temp dir, spool, buffer

**Publisher**:
The `pages publish` subcommand that performs Publish.
_Avoid_: uploader, client

**Verification**:
A `GET` of the public URL after an Upload. The Publisher accepts the Page
only when the response is `200` with an HTML content type.
_Avoid_: check, probe, ping
