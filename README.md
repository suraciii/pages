# pages

Minimal static page publishing. One Go command with three subcommands:

- `pages serve` accepts identity-bound uploads only on loopback.
- `pages publish` uploads a page file and prints its public URL, or
  writes it directly into a local public root and prints its path.
- `pages generate-token` issues an upload Token for one identity.

The service intentionally does not implement archives, revisions, histories, rollback, SQLite, or CI.

## TLDR

Issue a Token, run the service, publish a page:

```bash
pages generate-token -tokens-file /etc/pages/tokens.json bumble
pages serve -public-root /srv/pages/public -tokens-file /etc/pages/tokens.json
pages publish -file report.html -slug report -base-url https://pages.example.com
```

`pages publish` prints the public URL when the page is live:

```text
https://pages.example.com/pages/bumble/report/
```

`generate-token` creates the tokens file when it is missing. Without
`-base-url`, `pages publish` writes straight into a local public root
and no server runs. Walk through the complete setup in
[docs/getting-started.md](docs/getting-started.md).

## Contract

The upload client sends:

```text
POST /pages/<slug>
Authorization: Bearer <identity>.<random-secret>
Content-Type: text/html; charset=utf-8 | application/zip
```

A `text/html` body publishes one standalone page. An `application/zip`
body publishes a directory page whose root file is `index.html`.

The server derives identity from the verified token, never from an upload path, request body, or custom header. A valid upload replaces exactly:

```text
<public-root>/<identity>/<slug>/
```

The public address is:

```text
https://<your-host>/pages/<identity>/<slug>/
```

The server stages each upload under `<public-root>/.pages/` and swaps it into place with same-filesystem renames, so readers observe the complete old page or complete new page. Durability is best-effort: after a machine crash, publishing again restores the page.

See [docs/publishing.md](docs/publishing.md) for the product spec,
[docs/getting-started.md](docs/getting-started.md) to set pages up,
and [design/architecture.md](design/architecture.md) for the design.

## Build And Test

```bash
make ci
```

## Local Run

Create a token file that only the service account can read. The value must be a high-entropy secret and must not contain `.`:

```json
{
  "bumble": "replace-with-a-high-entropy-secret"
}
```

Run the server:

```bash
go run ./cmd/pages serve \
  -public-root "$PWD/.scratch/public" \
  -tokens-file "$PWD/.scratch/tokens.json"
```

Smoke-test a direct local server with `curl`. The token identity supplies the target namespace:

```bash
export PAGES_UPLOAD_TOKEN='bumble.replace-with-a-high-entropy-secret'
curl -i -X POST http://127.0.0.1:3103/pages/hello \
  -H "Authorization: Bearer $PAGES_UPLOAD_TOKEN" \
  -H 'Content-Type: text/html; charset=utf-8' \
  --data-binary '@example.html'
cat ./.scratch/public/bumble/hello/index.html
```

Use the publish subcommand after a static host (Caddy is the reference)
serves the public directory:

```bash
go run ./cmd/pages publish \
  -file ./example.html \
  -slug hello \
  -base-url https://pages.example.com
```

`pages publish` verifies the public URL after uploading before it prints
that URL. A `.zip` file publishes a directory page. Without `-base-url`,
`pages publish` writes directly into the public root from
`PAGES_PUBLIC_ROOT` or the config file; no service is involved.

## Docker

Build with the included [Dockerfile](Dockerfile):

```bash
docker build -t pages:local .
```
