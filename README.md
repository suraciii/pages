# pages

Minimal static Page publishing. One Go command has three product actions:

- `pages serve` accepts authenticated Uploads on loopback and writes Pages
  into a Public Root.
- `pages publish` publishes one HTML file or zip file and prints its public
  URL or local Page directory.
- `pages generate-token` issues a Token for the Default or one Named Identity.

`pages skill` prints version-matched instructions for agents.

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
service:

```text literal
pages publish --file report.html --slug report --dest pages-public
```

```text literal
<current-directory>/pages-public/report/
```

### Remote Publish

Remote Publish adds a Token, `pages serve`, and a static host. On the prepared
service host:

```text literal
pages generate-token
pages serve --dest /srv/pages/public
```

Use this minimal Caddyfile:

```text literal
pages.example.com {
    @upload method POST
    reverse_proxy @upload 127.0.0.1:3103

    @internal path /.pages/*
    respond @internal 404

    root * /srv/pages/public
    file_server
}
```

Set `PAGES_UPLOAD_TOKEN` to the Token printed by `generate-token`, then
Publish:

```text literal
pages publish --file report.html --slug report \
  --dest https://pages.example.com
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
POST <destination>/<slug>
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
<destination>/<slug>/
<destination>/@<identity>/<slug>/
```

A local Publish writes directly into a Public Root and does not use a service
or network. `--identity` selects an optional Named Identity. Both modes
prepare a complete Page before they replace the old Page.

See the [`pages publish` command contract](design/cli.md#pages-publish) for
Destination syntax, defaults, and input precedence.

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

The [pages skill](skills/pages/SKILL.md) is a stable bootstrap. It installs or
finds the CLI, then reads the complete version-matched skill from
`pages skill`. From a repository checkout, install the bootstrap into the
agent skill directory:

```text literal
mkdir -p ~/.agents/skills
cp -r skills/pages ~/.agents/skills/
```

The bootstrap is one file and does not copy operational command syntax. CLI
updates automatically provide their matching instructions through
`pages skill`; the bootstrap does not need to be copied again.

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
