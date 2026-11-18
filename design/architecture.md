# Architecture

The Service is one HTTP process. It serves published Pages to readers and
accepts authenticated Uploads that replace them. Reads and writes share one
listener and one Public Root, but they never share a route.

## Design Drivers

- The Default Identity must stay invisible in Tokens, Page paths, and URLs.
  Named Identity is optional. Identity must come from a verified Token, never
  from an upload path or header.
- Readers must never see a partial Page during normal operation. They may see
  a brief absence while one complete Page replaces another.
- One public Service. Deployment configures one listener and one directory.
  There is no second static host to keep in sync, no second read route, and no
  second permission boundary.
- The read model is the file system. The Service serves the Public Root
  directly. It adds no read API, database, query language, or publication
  manifest.
- The Service holds Tokens and is therefore a sensitive process. Uploads stay
  authenticated and size-limited. The listener stays on loopback by default.
  A public listener should use TLS.
- The Service must stay minimal. It has no database, archives, or revisions.
  It does not execute Page content and does not generate scripts.
- Deployment configures the Public Root, the listener, and optionally a
  certificate. Every other layout under the Public Root is owned by the
  Service, including its internal staging.
- The pages program supports Linux, macOS, and Windows. Other operating
  systems are outside the product contract and do not need to compile.

## Model

**Page** - one published resource tree. The Default Identity target is
`<public-root>/<slug>`. A Named Identity target is
`<public-root>/@<identity>/<slug>`. The root file is `index.html`.

**Service** - the `pages serve` process. It owns one HTTP listener and the
Public Root. It serves reads, accepts Uploads, answers health checks, and
refreshes the Page Index and Page Record. A reverse proxy may forward to it,
but the Service remains the only read path and never depends on one.

**Destination** - where `pages publish` sends a Page. A local path resolves
to a Public Root. An absolute HTTP(S) URL resolves to the public Upload and
Verification route. `pages serve` accepts only the local-path form and uses
it as its Public Root.

**Identity** - the namespace that owns a Page. The Default Identity is the
empty internal value. A Named Identity is a valid name. The Service derives it
from the verified Token. The upload path must not name an Identity.

**Slug** - the page name inside an Identity. A Slug matches
`^[a-z0-9][a-z0-9-]{0,62}$`.

**Token** - one secret for the Default Identity, or `identity.secret` for a
Named Identity. The Service compares the secret against the tokens file with a
constant-time hash comparison.

**Read** - one `GET` or `HEAD` response for a Page, a Page asset, the Page
Index, the Page Record, or health. A Read never authenticates and never
changes the Public Root.

**Upload shape** - one of two body formats:

- `text/html` - one standalone HTML document.
- `application/zip` - a page directory archive. `index.html` must exist at
  the archive root.

## Storage layout

```text diagram
<public-root>/                              <- deployment-specified; the Service creates it
|
+-- .pages/                                 <- internal staging, never served
|   +-- staging/
|       +-- default/                        <- internal Default Identity scope
|       |   +-- new-<rand>/                 <- unpacked new version
|       |   +-- old-<rand>-<slug>/          <- displaced old version, pending delete
|       |   `-- .upload-<rand>.zip          <- pending zip body, transient
|       `-- @<identity>/                    <- internal Named Identity scope
|
+-- index.html                              <- generated root Page Index
+-- index.json                              <- generated root Page Record
+-- report/                                 <- Default Identity Page
|   +-- index.html
|   `-- img/chart.png
`-- @<identity>/                            <- Named Identity Pages
    +-- index.html                          <- generated Identity Page Index
    +-- index.json                          <- generated Identity Page Record
    +-- report/                             <- directory Page from a zip
    |   +-- index.html
    |   `-- img/chart.png
    `-- hello/                              <- single-file page
        `-- index.html
```

Staging uses `default` for the Default Identity and `@<identity>` for a Named
Identity. These internal scopes cannot overlap a public Default Identity Page.
`<rand>` is random hex without hyphens, so `old-<rand>-<slug>` splits
unambiguously at the first hyphen: everything after is the Slug, even when the
Slug contains hyphens.

## Boundaries

```text diagram
reader                  --GET/HEAD-->  pages serve  --read-->  public root
pages publish (remote)  --POST------>  pages serve  --write-->  staging + swap
                                             |
                                             +-- tokens file, optional certificate

pages publish (local) --------------------------->  public root
```

- The CLI is the authoring and operation surface. `pages publish` is the
  remote publisher in the diagram; users and agents do not call the HTTP
  routes directly. The routes are the Publisher's transport and the reader
  surface.
- The Service accepts Reads and Uploads on one listener. The method decides
  the route. Reads never require a Token; Uploads always do.
- The default listener is `127.0.0.1:3103`. Deployment sets a public address
  when readers and publishers are remote. TLS is optional in the Service and
  can also be terminated by a reverse proxy in front of a loopback listener.
- A reverse proxy is optional. It forwards every method and path to the
  Service. It must not serve the Public Root independently: one read path
  prevents the two servers from disagreeing about headers or content.
- The write path keeps staging private and makes each installed Page readable
  through the Service.
- `pages publish` has two modes. Remote mode uses one remote address for
  Upload and Verification. Local mode writes directly into a Public Root with
  the same staging and swap rules; no Service is involved.
- The tokens file, certificate chain, and private key stay outside the Public
  Root. The Service rejects a startup configuration where one is the Public
  Root or below it after resolving symbolic links. The Read path also blocks
  dot paths, but this check keeps a credential out of a root that may be
  broad.

## Read path

The Service resolves every Read against the Public Root. The path rules are
part of the security boundary, not a convenience.

### Routing

| Request | Response |
| --- | --- |
| `GET /__pages/healthz` | `200`, no body, no authentication |
| `HEAD /__pages/healthz` | `200`, no body, no authentication |
| `GET /` or `HEAD /` | root Page Index |
| `GET /index.json` or `HEAD /index.json` | root Page Record |
| `GET /<slug>/` or `HEAD /<slug>/` | Default Identity Page |
| `GET /<slug>/<path>` or `HEAD /<slug>/<path>` | Page asset |
| `GET /@<identity>/` or `HEAD /@<identity>/` | Named Identity Page Index |
| `GET /@<identity>/index.json` or `HEAD /@<identity>/index.json` | Named Identity Page Record |
| `GET /@<identity>/<slug>/` or `HEAD /@<identity>/<slug>/` | Named Identity Page |
| `POST /<slug>` | Upload |
| any other method | `405` with `Allow: GET, HEAD, POST` |
| any other path | `404` |

`__pages` is a reserved first segment. It cannot be a Slug because a Slug
starts with a letter or digit. A Page named `healthz` is reachable at
`/healthz/`; the health route never shadows it.

### Canonical path

A Read path must be a canonical path relative to the Public Root. The Service
rejects the request with `404` when any rule fails:

- The path is valid UTF-8 and contains no ASCII control character.
- The path starts with `/` and has no empty segment.
- No segment is `.` or `..`, and no segment starts with `.`. This blocks
  `.pages` and every other dot-prefixed path.
- No segment contains a backslash.
- The first segment is not `__pages`, except for the health route.
- A first segment that starts with `@` is a Named Identity. It must be a
  valid Identity name. A Page asset may contain `@` later in the path.

### Resolution

- The request path maps directly below the Public Root. A directory request
  must end with `/`; a request without the slash receives a `301` whose
  `Location` is the request path plus `/`. The redirect never uses the `Host`
  header.
- `/` resolves to `<public-root>/index.html`.
- A directory resolves to its `index.html`. A directory without `index.html`
  is `404`. There is no directory listing.
- A file resolves to a regular file. A path with a trailing slash that names a
  file is `404`.
- A missing path is `404`. A read failure that is not a missing path is a
  generic `500` with no file-system detail.

### Confinement

- The configured Public Root is trusted. Below it, the Service must not follow
  a symbolic link. A path with a symlink component is `404`.
- Only regular files are served. Directories are resolved to `index.html`;
  devices, sockets, pipes, and other special files are never served.
- The Service never serves `.pages`, pending upload bodies, or displaced old
  versions.
- A Read cannot change the Public Root, refresh an Index, or create a file.

### Representation

- HTML is served as `text/html; charset=utf-8`.
- Other files use the standard library extension mapping. An unknown
  extension is `application/octet-stream`.
- `GET` returns the complete file with `Content-Length`. `HEAD` returns the
  same headers with no body.
- The Service does not compress, does not support range requests, and does not
  answer conditional requests with `304`. It returns the complete
  representation for every Read.

### Headers

Every response carries the reference security headers in
[deployment.md](deployment.md#headers). This includes `404`, `405`, redirects,
health, and Upload responses. The headers have one owner: the Service. A
deployment that uses a proxy must not strip or replace them.

### Read during a swap

A Page replacement is two renames. A Read that lands in the window may see
the complete old Page, the complete new Page, or a brief `404`. It never sees a
partially written Page. A `404` during that window is allowed.

### TLS

- The Service may terminate TLS itself from a certificate and key file. The
  minimum protocol version is 1.2. HTTP/2 is available on TLS.
- `SIGHUP` re-reads the tokens file and the certificate pair independently. A
  failed reload keeps the previous value for that input and logs the error.
  New connections use a new certificate; new requests use new Tokens.
- Every other input is startup-only. Changing the listener, Destination,
  upload limit, Content Security Policy, configuration directory, tokens path,
  or certificate path needs a restart.
- The Service does not implement ACME. Certificate issuance and renewal belong
  to the operator. A reverse proxy may terminate TLS instead.
- A non-loopback listener without TLS logs a warning at startup. The operator
  may use plaintext only behind a trusted TLS terminator.

## Upload

1. Reject any method other than `POST` at the route layer with `405`.
2. Require the path `POST /<slug>` with a valid Slug. Reject with `404`
   otherwise. The path must not name an Identity.
3. Authenticate the Bearer Token and derive the Default or Named Identity
   from it. Reject with `401` on failure. The source address is never an
   identity: a loopback request without a Token is rejected, because a proxy
   can forward remote requests from loopback.
4. Require a supported Content-Type: `text/html` with an optional
   `charset=utf-8` parameter, or `application/zip`. Require a non-empty body.
   Reject with `400`.
5. A `text/html` body must be valid UTF-8. Reject with `400`.
6. Reject bodies over the upload byte limit with `413`.

An Upload response is `204` after the swap. The Identity comes from the
verified Token only. A Read never authenticates, and an Upload never serves
the new Page in the same request.

## Zip validation

A zip Upload must pass every rule before extraction:

- `index.html` must exist at the archive root.
- Directory entries are allowed and skipped during extraction. File entries
  must be regular files; symlink and special entries are rejected.
- Entry names must be canonical relative POSIX paths: no absolute paths,
  `.` or `..` components, repeated separators, backslashes, leading dot, or
  invalid UTF-8. Names that differ only by letter case are duplicates to avoid
  collisions on case-insensitive file systems.
- At most 512 entries.
- The total uncompressed size must not exceed four times the upload byte
  limit.

Zip contents are not executed. The Read path serves them as files.

## Swap

The Service stages the new Page under `.pages/staging/` inside the Public
Root. The staging area lives inside the Public Root by construction, so every
rename stays on one filesystem.

```text diagram
1. unpack to   .pages/staging/<scope>/new-<rand>/
2. rename      public/<target>/<slug> --> .pages/staging/<scope>/old-<rand>-<slug>
3. rename      .pages/staging/<scope>/new-<rand> --> public/<target>/<slug>
4. remove      .pages/staging/<scope>/old-<rand>-<slug>
```

For the Default Identity, `<scope>` is `default` and `<target>` is empty. For
a Named Identity, both values are `@<identity>`.

When the target does not exist (the first publish of a Slug), step 2 is
skipped.

Concurrent Uploads for the same Identity and Slug are serialized with an
in-process lock, so two swaps can never interleave.

A single-file Page uses the same directory swap. It replaces the complete old
Page, including assets from an earlier zip Upload.

Rejected alternative: a symlink swap (`<slug>` points at a version directory,
one rename replaces the link). It removes the absence window and is the
industry pattern for directory deploys. Rejected because the window is
microseconds wide, the read path would need symlink cooperation, and Windows
cannot create symlinks without elevated privileges, which would break local
publish on Windows. Enable it only when a real absence incident appears.

## Crash recovery

At startup the Service scans `.pages/staging/`. It maps `default` back to the
Public Root and `@<identity>` back to the matching Named Identity directory.
When `old-<rand>-<slug>` exists and the target Page is missing, recovery
renames the old version back. It removes every other staging leftover,
including pending upload bodies.

## Durability

The write path does not call `fsync`. Durability is best-effort: a machine
crash or power loss right after a publish may lose or truncate the new Page.
Publishing again restores it. Atomicity against concurrent readers during
normal operation is preserved by the renames.

## Rejected alternatives

- Keep a static host for reads. Rejected because it adds a second process, a
  second configuration, a second permission boundary, and a second place for
  headers to drift. The cost of the chosen design is that the Service owns
  HTTP read semantics and TLS. The token boundary and body limits contain the
  risk.
- Two listeners: a public read listener and a loopback Upload listener.
  Rejected because remote Publish would still need a proxy for Uploads, which
  restores the deployment problem this design removes.
- A path-prefix listener, for example `--base-path /docs`. Rejected because a
  reverse proxy can strip a prefix. The Service always serves the Public Root
  at `/`.
- A generated directory listing for every directory. Rejected because it
  exposes files a Page did not link and adds a second navigation surface.

## Status

Implemented. The Service serves Reads and Uploads on one listener, supports
optional file-based TLS and a Content Security Policy, and answers
`/__pages/healthz`.
