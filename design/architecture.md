# Architecture

The publish service has two paths that never share a process boundary: a
loopback write path and a public read path.

## Design Drivers

- Each Identity must publish Pages without trusting a shared upload
  endpoint. Identity must come from a verified Token, never from a path or
  header.
- Readers must see the complete old Page or the complete new Page. A
  partial write must never be visible during normal operation.
- The service must stay minimal. It has no database, no archives, and no
  revisions. A static file server already serves files; the service must
  not replace it.
- Deployment configures one static-resources directory. pages owns and
  maintains everything under it, including its internal staging.

## Model

**Page** — one published resource tree under
`<public-root>/<identity>/<slug>`. The root file is `index.html`. A Page is
either one standalone HTML document or a directory of files from a zip.

**Identity** — the namespace that owns a Page. The server derives it from
the verified Token. The upload path must not name an Identity.

**Slug** — the page name inside an Identity. A Slug matches
`^[a-z0-9][a-z0-9-]{0,62}$`.

**Token** — `identity.secret`. The server compares the secret against the
token file with a constant-time hash comparison.

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
│       ├── <identity>/
│       │   ├── new-<rand>/                 <- unpacked new version
│       │   ├── old-<rand>-<slug>/          <- displaced old version, pending delete
│       │   └── .upload-<rand>.zip          <- pending zip body, transient
│       └── ...
│
└── <identity>/
    ├── report/                             <- directory page from a zip
    │   ├── index.html
    │   └── img/chart.png
    └── hello/                              <- single-file page
        └── index.html
```

Staging is grouped per Identity, so the Identity never needs to be
parsed out of a directory name. `<rand>` is random hex without
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
  server, block `/pages/.pages/...`, and serve other requests from the
  static files. The host requirements are in
  [deployment.md](deployment.md). Local mode writes directly into a
  public root with the same staging and swap rules; no server or host is
  involved.

## Semantics

### Upload

1. Reject any method that is not `POST` with `404`.
2. Require the path `POST /<slug>` with a valid Slug. Reject with
   `404` otherwise.
3. Authenticate the Bearer Token and derive the Identity from it. Reject
   with `401` on failure.
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
1. unpack to   .pages/staging/<id>/new-<rand>/
2. rename      public/<id>/<slug>  --> .pages/staging/<id>/old-<rand>-<slug>
3. rename      .pages/staging/<id>/new-<rand> --> public/<id>/<slug>
4. remove      .pages/staging/<id>/old-<rand>-<slug>
```

When the target does not exist (the first publish of a Slug), step 2 is
skipped.

Concurrent Uploads for the same Slug are serialized with an in-process
per-Slug lock, so two swaps can never interleave.

A single-file Page replaces `index.html` with one atomic rename. No swap
steps apply.

Readers see the complete old Page or the complete new Page, or a brief
absence during the swap of an existing directory Page.

Rejected alternative: a symlink swap (`<slug>` points at a version
directory, one rename replaces the link). It removes the absence window
and is the industry pattern for directory deploys. Rejected because the
window is microseconds wide, the link layout needs host cooperation
(hide the version store, follow symlinks), and Windows cannot create
symlinks without elevated privileges, which would break local publish on
Windows. Enable it only when a real absence incident appears.

### Crash recovery

At startup the server scans `.pages/staging/`. When
`<id>/old-<rand>-<slug>` exists and `public/<id>/<slug>` is missing, it
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
