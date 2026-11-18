# Roadmap

This document tracks implementation progress. Treat it as the source of
truth for what is done and what is next.

## Done

- One `pages` binary with three product actions: `pages serve`, `pages
  publish`, and `pages generate-token`. `publish --help` carries the agent
  guidance. The command contract is in [design/cli.md](design/cli.md).
- Directory Pages: zip Uploads with the validation rules and the swap
  sequence in [design/architecture.md](design/architecture.md).
- `pages publish` local mode: write into a public root directly, with the
  same validation and swap rules.
- One Destination input for Publish: a local path selects local mode, an
  HTTP(S) URL selects remote mode, and an omitted value uses `pages-public/`
  under the current directory. `--dest` is an alias for `--destination`.
- The Service owns its Public Root: it creates the directory, stages inside
  `.pages/`, and recovers crashed swaps at startup.
- No `fsync`: best-effort durability, documented in
  [docs/publishing.md](docs/publishing.md).
- Self-hosted Service: `pages serve` is the only public listener. It serves
  Pages, the Page Index, and the Page Record; keeps Uploads authenticated;
  sends the reference security headers with a Content Security Policy
  override; supports file-based TLS with independent token and certificate
  reload on `SIGHUP`; starts read-only when no tokens are present; and answers
  `GET /__pages/healthz` and `HEAD /__pages/healthz`. The static-host read
  path is not part of the product.
- The pages program supports Linux, macOS, and Windows.
- The Go module is `github.com/suraciii/pages`.
- Page Index: generated root and Named Identity navigation pages are refreshed
  on startup and after local or remote Publish.

## Next

No implementation item is open. Run the release gate in
[design/release.md](design/release.md).

## Not planned

The product intentionally does not implement archives, revisions,
histories, rollback, SQLite, or CI. A change that adds one of these must
first change the product contract.
