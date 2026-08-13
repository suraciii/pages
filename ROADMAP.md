# Roadmap

This document tracks implementation progress. Treat it as the source of
truth for what is done and what is next.

## Done

- One `pages` binary with three subcommands: `pages serve`, `pages
  publish`, and `pages generate-token`. The command contract is in
  [design/cli.md](design/cli.md).
- Directory Pages: zip Uploads with the validation rules and the swap
  sequence in [design/architecture.md](design/architecture.md).
- `pages publish` local mode: write into a public root directly, with the
  same validation and swap rules.
- The server owns its public root: it creates the directory, stages
  inside `.pages/`, and recovers crashed swaps at startup.
- No `fsync`: best-effort durability, documented in
  [docs/publishing.md](docs/publishing.md).
- Serve operations: `GET /healthz`, tokens reload on `SIGHUP`, one startup
  log line with the resolved configuration.
- The Go module is `github.com/suraciii/pages`.

## Next

- A public deployment with the host contract in
  [design/deployment.md](design/deployment.md): the internal-path block
  and the asset CSP.

## Not planned

The product intentionally does not implement archives, revisions,
histories, rollback, SQLite, or CI. A change that adds one of these must
first change the product contract.
