# Pages Publishing

This context defines the product language used by pages. Use these terms in
product docs, design docs, examples, and new code names. Keep one term for
one concept.

## Pages

**Page**:
One published resource tree under a Slug and, optionally, a Named Identity.
The root file is `index.html`. A Page is one standalone HTML document or a
directory of files from a zip. A Page may contain same-origin scripts,
stylesheets, images, and data files. pages never executes them.
_Avoid_: file, article, site, document

**Identity**:
The namespace that owns a Page. The Default Identity is unscoped and never
appears in a Token, URL, or Page path. A Named Identity is an optional scope
shown as `@<identity>`. The Service derives the Identity from the verified
Token, never from an upload path, request body, or custom header.
_Avoid_: user, account, owner, tenant

**Slug**:
The stable name of a Page inside the Default Identity or one Named Identity.
A Slug has at most 63 lowercase letters, digits, or hyphens, and starts with a
letter or digit.
_Avoid_: name, path, identifier

**Token**:
An upload credential. A Default Identity Token is one secret. A Named
Identity Token has `identity.secret` form. The Service compares the secret
against the tokens file before it accepts an Upload.
_Avoid_: key, password, credential

## Publishing

**Upload**:
A `POST /<slug>` request to the Service with a Bearer Token and an HTML or
zip body. One Upload replaces exactly one Page.
_Avoid_: write, save, commit

**Publish**:
The complete client operation: put a Page in place and return where it
lives. Remote mode Uploads through the Service and verifies the public
URL. Local mode writes directly into a Public Root and returns its path.
_Avoid_: deploy, release, push

**Service**:
The `pages serve` process. It is the only public listener: it serves Pages,
the Page Index, and the Page Record, and it accepts authenticated Uploads.
It owns the Public Root. A reverse proxy may forward to it, but pages does
not require one.
_Avoid_: server, static host, web server

**Destination**:
Where Publish puts a Page. A local path is a Public Root and selects local
mode. An absolute HTTP(S) URL is an Upload and Verification endpoint and
selects remote mode. An omitted Destination is `pages-public/` under the
current directory.
_Avoid_: remote, target, mode switch

**Public Root**:
The directory that holds published Pages and that the Service serves. The
Service writes Pages under `<slug>/index.html` for the Default Identity and
`@<identity>/<slug>/index.html` for a Named Identity. Local Publish may write
to a Public Root without a running Service.
_Avoid_: webroot, docroot, static root

**Page Index**:
A generated navigation document at the Public Root or inside one Named
Identity. The Service serves the root Index at `/` and an Identity Index at
`/@<identity>/`. It links to current Pages and Identity scopes. It shows the
Published Time of each Page. It is a system artifact: pages replaces it on
every refresh, so a manual edit to it is lost. It is not a Page, an Upload
target, or an access-control boundary.
_Avoid_: catalog, directory listing

**Published Time**:
The modification time of a Page's root `index.html`, shown in the Page Index.
It is the ordering basis for the Page list. It is not a recorded event time.
_Avoid_: upload time, timestamp

**Page Record**:
The machine-readable Page Index. It carries the same Page entries as the Page
Index in a structured form for programmatic callers. It is a system artifact
with the same replacement rules as the Page Index.
_Avoid_: manifest, API response, listing

**Staging Area**:
The `.pages/` directory inside the Public Root. The Service unpacks a new
Page there and swaps it into place. Readers see the complete old Page or the
complete new Page, except for a brief absence during replacement. The Service
never serves the staging area.
_Avoid_: temp dir, spool, buffer

**Publisher**:
The `pages publish` subcommand that performs Publish.
_Avoid_: uploader, client

**Verification**:
A `GET` of the public URL after an Upload. The Publisher accepts the Page
only when the Service response is `200` with an HTML content type.
_Avoid_: check, probe, ping
