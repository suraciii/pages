# Testing

The test suite uses only the standard library. It must not need a running
server, a static host, or any external service.

## Strategy

- Server tests use `httptest` and a real temporary directory from
  `t.TempDir()`. They publish through `Server.ServeHTTP` and read the
  resulting files from the public root.
- Client tests use `httptest.NewServer` to fake the upload endpoint and
  the static route. One test combines a real `pages.Server` with a real
  static file server to cover the routing boundary.
- Every rejection test must first publish a valid Page and then assert
  that a rejected Upload leaves the existing files unchanged.
- Zip tests build archives in memory with `archive/zip` and cover: a valid
  directory page, a missing root `index.html`, directory entries that are
  skipped, symlink entries, path traversal names, absolute names,
  backslashes, leading-dot names, non-UTF-8 names, duplicate names, too
  many entries, and an oversized uncompressed total.
- Swap tests cover: first publish, replace of a directory page, the
  brief-absence window is not asserted but the end state is, and startup
  recovery restores `old-<rand>-<slug>` under the identity staging
  directory when the target is missing and clears other staging
  leftovers.
- Token tests cover `SIGHUP` reload: a changed tokens file takes effect
  after reload, and a failed reload keeps the previous tokens.
- `GET /healthz` returns 200 without authentication.
- Size limits must stay configurable through `ServerConfig` so tests do
  not wait on real time or write large files.

## Gate

`make ci` runs `fmt-check`, `tidy-check`, `vet`, and `test`. Run it before
handoff.
