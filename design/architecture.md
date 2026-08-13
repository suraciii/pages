# Architecture

The publish service has two paths that never share a process boundary: a
loopback write path and a public read path.

## Design Drivers

- The Default Identity must stay invisible in Tokens, Page paths, and URLs.
  Named Identity is optional. Identity must come from a verified Token, never
  from an upload path or header.
- Readers must never see a partial Page during normal operation. They may see
  a brief absence while one complete Page replaces another.
- The service must stay minimal. It has no database, no archives, and no
  revisions. A static file server already serves files; the service must
  not replace it.
- Deployment configures one static-resources directory. pages owns and
  maintains everything under it, including its internal staging.
- The pages program supports Linux, macOS, and Windows. Other operating
  systems are outside the product contract and do not need to compile.

## Model

**Page** — one published resource tree. The Default Identity target is
`<public-root>/<slug>`. A Named Identity target is
`<public-root>/@<identity>/<slug>`. The root file is `index.html`.

**Destination** — where `pages publish` sends a Page. A local path resolves
to a Public Root. An absolute HTTP(S) URL resolves to the public Upload and
Verification route. `pages serve` accepts only the local-path form and uses
it as its Public Root.

**Identity** — the namespace that owns a Page. The Default Identity is the
empty internal value. A Named Identity is a valid name. The server derives it
from the verified Token. The upload path must not name an Identity.

**Slug** — the page name inside an Identity. A Slug matches
`^[a-z0-9][a-z0-9-]{0,62}$`.

**Token** — one secret for the Default Identity, or `identity.secret` for a
Named Identity. The server compares the secret against the tokens file with a
constant-time hash comparison.

**Upload shape** — one of two body formats:

- `text/html` — one standalone HTML document.
- `application/zip` — a page directory archive. `index.html` must exist at
  the archive root.

## Storage layout

```text diagram
<public-root>/                              <- deployment-specified; pages creates it
│
├── .pages/                                 <- internal staging, never served
│   └── staging/
│       ├── default/                      <- internal Default Identity scope
│       │   ├── new-<rand>/                 <- unpacked new version
│       │   ├── old-<rand>-<slug>/          <- displaced old version, pending delete
│       │   └── .upload-<rand>.zip          <- pending zip body, transient
│       └── @<identity>/                  <- internal Named Identity scope
│
├── report/                                 <- Default Identity Page
│   ├── index.html
│   └── img/chart.png
└── @<identity>/                            <- Named Identity Pages
    ├── report/                             <- directory Page from a zip
    │   ├── index.html
    │   └── img/chart.png
    └── hello/                              <- single-file page
        └── index.html
```

Staging uses `default` for the Default Identity and `@<identity>` for a Named
Identity. These internal scopes cannot overlap a public Default Identity Page.
`<rand>` is random hex without
hyphens, so `old-<rand>-<slug>` splits unambiguously at the first
hyphen: everything after is the Slug, even when the Slug contains
hyphens.

## Boundaries

```text diagram
pages publish (remote) --POST/GET--> static host --> static files (reads)
                                       |
                                       +--> pages serve (loopback, writes)
                                              |
                                              v
                                         public root

pages publish (local) ----------------------------------> public root
```

- The server accepts Uploads only on loopback. It binds to `127.0.0.1` by
  default.
- A static host serves the public root read-only. The server never serves
  reads. Caddy is the reference host; any static file server works. See
  [deployment.md](deployment.md).
- `pages publish` has two modes. Remote mode uses one remote address for
  Upload and Verification; the host must route `POST /...` to the
  server, block `/.pages/...`, and serve other requests from the
  static files. The host requirements are in
  [deployment.md](deployment.md). Local mode writes directly into a
  public root with the same staging and swap rules; no server or host is
  involved.

## Semantics

### Upload

1. Reject any method that is not `POST` with `404`.
2. Require the path `POST /<slug>` with a valid Slug. Reject with
   `404` otherwise.
3. Authenticate the Bearer Token and derive the Default or Named Identity
   from it. Reject with `401` on failure.
4. Require a supported Content-Type: `text/html` with an optional
   `charset=utf-8` parameter, or `application/zip`. Require a non-empty
   body. Reject with `400`.
5. A `text/html` body must be valid UTF-8. Reject with `400`.
6. Reject bodies over the upload byte limit with `413`.

### Zip validation

A zip Upload must pass every rule before extraction:

- `index.html` must exist at the archive root.
- Directory entries are allowed and skipped during extraction. File
  entries must be regular files; symlink and special entries are
  rejected.
- Entry names must be relative POSIX paths: no absolute paths, no `..`
  components, no backslashes, no leading dot, valid UTF-8, and no
  duplicates.
- At most 512 entries.
- The total uncompressed size must not exceed four times the upload byte
  limit.

### Swap

The server stages the new Page under `.pages/staging/` inside the public
root. The staging area lives inside the public root by construction, so
every rename stays on one filesystem.

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

Readers see the complete old Page or the complete new Page, or a brief
absence during the swap of an existing Page.

Rejected alternative: a symlink swap (`<slug>` points at a version
directory, one rename replaces the link). It removes the absence window
and is the industry pattern for directory deploys. Rejected because the
window is microseconds wide, the link layout needs host cooperation
(hide the version store, follow symlinks), and Windows cannot create
symlinks without elevated privileges, which would break local publish on
Windows. Enable it only when a real absence incident appears.

### Crash recovery

At startup the server scans `.pages/staging/`. It maps `default` back to the
Public Root and `@<identity>` back to the matching Named Identity directory.
When `old-<rand>-<slug>` exists and the target Page is missing, recovery
renames the old version back. It removes every other staging leftover,
including pending upload bodies.

### Durability

The write path does not call `fsync`. Durability is best-effort: a machine
crash or power loss right after a publish may lose or truncate the new
Page. Publishing again restores it. Atomicity against concurrent readers
during normal operation is preserved by the renames.

## Status

Implemented. The server owns the public root, stages inside `.pages/`,
recovers interrupted swaps at startup, and accepts both Upload shapes
without `fsync`.
