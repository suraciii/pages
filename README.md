# pages

Minimal static Page publishing. One Go command has three subcommands:

- `pages serve` accepts authenticated Uploads on loopback and writes Pages
  into a Public Root.
- `pages publish` publishes one HTML file or zip file and prints its public
  URL or local Page directory.
- `pages generate-token` issues a Token for the Default or one Named Identity.

pages keeps no publication archive, revision history, rollback state, or
database.

## Install

Install the binary with the Go toolchain:

```text literal
go install github.com/suraciii/pages@latest
```

The binary is at `$(go env GOPATH)/bin/pages`. Put that directory on your
`PATH`.

## TL;DR

Create one Page file:

```text literal
printf '<!doctype html><title>Report</title><h1>Ready</h1>\n' > report.html
```

### Local Publish

Local Publish writes directly into a Public Root. It needs no Token or
service:

```text literal
PAGES_PUBLIC_ROOT=pages-public pages publish \
  --file report.html --slug report
```

```text literal
<current-directory>/pages-public/report/
```

### Remote Publish

Remote Publish adds a Token, `pages serve`, and a static host. On the prepared
service host:

```text literal
pages generate-token
pages serve --public-root pages-public
```

Use this minimal Caddyfile:

```text literal
pages.example.com {
    @upload method POST
    reverse_proxy @upload 127.0.0.1:3103

    @internal path /.pages/*
    respond @internal 404

    root * {$PAGES_PUBLIC_ROOT}
    file_server
}
```

Use the Token printed by `generate-token` to Publish:

```text literal
export PAGES_UPLOAD_TOKEN='7v9A_example-secret'
pages publish --file report.html --slug report \
  --remote https://pages.example.com
```

```text literal
https://pages.example.com/report/
```

See [Getting Started](docs/getting-started.md) and
[Deployment](docs/deployment.md) for host setup, permissions, limits, headers,
and process management.

## How It Works

A remote Publish sends one Upload:

```text literal
POST <remote>/<slug>
Authorization: Bearer <secret> | Bearer <identity>.<secret>
Content-Type: text/html; charset=utf-8 | application/zip
```

The service derives the Default or Named Identity from the verified Token. It
never takes Identity from the upload path, request body, or a custom header.
The Default Identity stays hidden:

```text literal
<public-root>/<slug>/
<public-root>/@<identity>/<slug>/
```

The Publisher verifies and prints the public URL:

```text literal
<remote>/<slug>/
<remote>/@<identity>/<slug>/
```

A local Publish writes directly into a Public Root and does not use a service
or network. `--identity` selects an optional Named Identity. Both modes
prepare a complete Page before they replace the old Page.

## Documentation

- [Getting Started](docs/getting-started.md) sets up remote Publish from
  install to a live Page.
- [Publishing](docs/publishing.md) defines the Publish contract and limits.
- [Configuration](docs/configuration.md) lists flags, environment variables,
  and config files.
- [Deployment](docs/deployment.md) covers systemd, Docker, and the reference
  Caddy configuration.
- [Architecture](design/architecture.md) explains the write path and Identity
  boundary.
- [Design Index](design/README.md) links every design specification.

## Agent Skill

The [pages skill](skills/pages/SKILL.md) teaches an agent how to Publish with
this product. From a repository checkout, install it into the agent skill
directory:

```text literal
mkdir -p ~/.agents/skills
cp -r skills/pages ~/.agents/skills/
```

The skill is one file. Copy it again after each update.

## Development

Run the repository gate and build the binary:

```text literal
make ci
mkdir -p dist
go build -o dist/pages .
```

## Docker

Build the included [Dockerfile](Dockerfile):

```text literal
docker build -t pages:local .
```

See [Deployment](docs/deployment.md) for a complete runtime configuration.

## License

pages is released under the [GNU AGPL-3.0](LICENSE).
