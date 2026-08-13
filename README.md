# pages

Minimal static Page publishing. One Go command has three subcommands:

- `pages serve` accepts authenticated Uploads on loopback and writes Pages
  into a Public Root.
- `pages publish` publishes one HTML file or zip file and prints its public
  URL or local Page directory.
- `pages generate-token` issues a Token for one Identity.

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

Publish one HTML file into a local Public Root:

```text literal
printf '<!doctype html><title>Report</title><h1>Ready</h1>\n' > report.html
mkdir -p .scratch/public
PAGES_PUBLIC_ROOT="$PWD/.scratch/public" pages publish \
  --file report.html \
  --slug report \
  --identity bumble
```

The command prints the Page directory:

```text literal
<current-directory>/.scratch/public/bumble/report/
```

Inspect the published Page:

```text literal
cat .scratch/public/bumble/report/index.html
```

This local Publish needs no Token or service. Follow the
[Getting Started guide](docs/getting-started.md) to publish through a public
URL.

## How It Works

A remote Publish sends one Upload:

```text literal
POST <remote>/<slug>
Authorization: Bearer <identity>.<secret>
Content-Type: text/html; charset=utf-8 | application/zip
```

The service derives the Identity from the verified Token. It never takes the
Identity from the upload path, request body, or a custom header. A valid
Upload replaces one Page at:

```text literal
<public-root>/<identity>/<slug>/
```

The Publisher verifies and prints the public URL:

```text literal
<remote>/<identity>/<slug>/
```

A local Publish writes directly into a Public Root. It takes the Identity
from `--identity` and does not use a service or network. Both modes prepare a
complete Page before they replace the old Page.

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
