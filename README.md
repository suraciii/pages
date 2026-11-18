# pages

Minimal Page publishing. One Go binary has three product actions:

- `pages serve` is the self-hosted Service: it serves published Pages and
  accepts authenticated Uploads on one listener.
- `pages publish` publishes one HTML file or zip file and prints its public
  URL or local Page directory.
- The Service generates a Page Index so readers can browse the currently
  published Pages and see when each Page was published.
- `pages generate-token` issues a Token for the Default or one Named Identity.

No static web server is required. A reverse proxy is optional.

`pages publish --help` prints the mode, Identity, and recovery guidance along
with its flags.

Users and agents use these commands. The Service's HTTP routes are behind
`pages publish` and serve readers; they are not a separate API for users or
agents.

pages keeps no publication archive, revision history, rollback state, or
database.

## Install

Install the binary with the Go toolchain:

```text literal
go install github.com/suraciii/pages@latest
```

For a reproducible install, use an exact release tag:

```text literal
go install github.com/suraciii/pages@v0.0.1
```

Make sure Go's binary install directory is on your `PATH`.

Check the installed version:

```text literal
pages --version
```

## TL;DR

Create one Page file:

```text literal
printf '<!doctype html><title>Report</title><h1>Ready</h1>\n' > report.html
```

### Local Publish

Local Publish writes directly into a Public Root. It needs no Token or
Service, and with no Destination it uses `pages-public/` under the current
directory:

```text literal
pages publish --file report.html --slug report
```

```text literal
<current-directory>/pages-public/report/
```

A Service with no tokens starts read-only: reads and local mode work; remote
Uploads return `401`. Generate a Token to enable remote mode.

### Remote Publish

Remote Publish adds a Token and the Service. No static host is involved. On
the prepared service host:

```text literal
pages generate-token --tokens-file /etc/pages/tokens.json
pages serve --dest /srv/pages/public --tokens-file /etc/pages/tokens.json --listen :443 \
  --tls-cert /etc/pages/tls/fullchain.pem \
  --tls-key /etc/pages/tls/privkey.pem
```

The Service serves readers and publishers on the same address. Set
`PAGES_UPLOAD_TOKEN` to the Token printed by `generate-token`, then Publish:

```text literal
pages publish --file report.html --slug report \
  --dest https://pages.example.com
```

```text literal
https://pages.example.com/report/
```

For a same-host Publish, use local mode (the default `pages-public/`); no
Token is needed. To exercise the remote path, keep the default listener and
publish to `http://127.0.0.1:3103` with a Token. Use TLS before a public
listener.

See [Getting Started](docs/getting-started.md) and
[Deployment](docs/deployment.md) for certificates, permissions, limits,
headers, and process management.

## How It Works

In remote mode, `pages publish` sends one Upload to the Service:

```text literal
POST <destination>/<slug>
Authorization: Bearer <secret> | Bearer <identity>.<secret>
Content-Type: text/html; charset=utf-8 | application/zip
```

The Service derives the Default or Named Identity from the verified Token. It
never takes Identity from the upload path, request body, or a custom header.
The Default Identity stays hidden:

```text literal
<public-root>/<slug>/
<public-root>/@<identity>/<slug>/
```

The same Service serves the Page to readers and applies the security headers:

```text literal
GET <destination>/<slug>/
200 text/html
```

`pages publish` verifies that URL and prints it:

```text literal
<destination>/<slug>/
<destination>/@<identity>/<slug>/
```

A local Publish writes directly into a Public Root and does not use the
Service or the network. `--identity` selects an optional Named Identity. Both
modes prepare a complete Page before they replace the old Page.

See the [`pages publish` command contract](design/cli.md#pages-publish) for
Destination syntax, defaults, and input precedence.

## Documentation

- [Getting Started](docs/getting-started.md) sets up remote Publish from
  install to a live Page.
- [Publishing](docs/publishing.md) defines the Publish contract and limits.
- [Page Index](docs/catalog.md) defines the generated navigation pages and
  the Page Record.
- [Configuration](docs/configuration.md) lists flags, environment variables,
  and config files.
- [Deployment](docs/deployment.md) covers systemd, Docker, TLS, headers, and
  the optional reverse proxy.
- [Architecture](design/architecture.md) explains the read and write paths,
  TLS, and the Identity boundary.
- [Design Index](design/README.md) links every design specification.

## Agent use

`pages publish --help` carries the method guidance an agent needs: which mode
to choose, what each mode requires, how to check the result, and what to do
when a Token or Identity is missing. The help is part of the binary, so it
always matches the installed version. The authoritative command contract is
[design/cli.md](design/cli.md).

## Development

Run the repository gate and build the binary:

```text literal
make ci
mkdir -p dist
go build -o dist/pages .
```

## Docker

pages does not publish an official container image. Check out the release tag
and build the included [Dockerfile](Dockerfile):

```text literal
git checkout v0.0.1
docker build -t pages:0.0.1 .
```

See [Deployment](docs/deployment.md) for a complete runtime configuration.

## License

pages is released under the [GNU AGPL-3.0](LICENSE).
