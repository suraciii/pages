# Getting Started

Run pages from zero to a live Page in one pass. Every command is
complete; replace the example paths and names with your own. Details and
alternatives are linked at each step.

## 1. Install

Install the binary with the Go toolchain:

```text literal
go install github.com/suraciii/pages@latest
```

The binary lands in `$(go env GOPATH)/bin/pages`. Put that directory on
the `PATH` of the service host.

Developers build from the repository instead:

```text literal
go build -o pages .
```

## 2. Issue a Token

On the service host, create the Public Root, then issue a Default Identity
Token:

```text literal
mkdir -p pages-public
pages generate-token
```

The command prints the Token once:

```text literal
7v9A_example-secret
```

Keep the printed Token for the Publisher. `generate-token` creates
`pages/tokens.json` under the operating system's user config directory with
mode `0600`. See
[configuration.md](configuration.md) for the tokens file rules.

## 3. Run pages serve

`pages serve` is one HTTP service: it accepts Uploads on loopback,
answers `GET /healthz`, and writes Pages into the Public Root. Run it in
the foreground to try:

```text literal
pages serve --public-root pages-public
```

It logs one line with the resolved configuration. Leave it running and use a
second terminal for the remaining steps. For a real host, run it under
systemd or Docker; both are in [deployment.md](deployment.md).

## 4. Serve the public root

A static host serves the Public Root to readers. Any static file server
works; Caddy is the reference. The host must route `POST /<slug>` to
`pages serve`, block `/.pages/*`, and serve everything else from
the Public Root. The complete Caddyfile is in
[deployment.md](deployment.md).

## 5. Publish a Page

On the publisher machine, put the Token from step 2 in the environment. The
remote upload address is the only mode switch. Publish through the public
URL:

```text literal
printf '<!doctype html><title>Report</title><h1>Ready</h1>\n' > report.html
export PAGES_UPLOAD_TOKEN='7v9A_example-secret'
pages publish --file report.html --slug report \
  --remote https://pages.example.com
```

The command uploads the file, verifies the public URL, and prints it
when the Page is live:

```text literal
https://pages.example.com/report/
```

A `.zip` file publishes a directory Page whose root file is
`index.html`.

Without `--remote`, `pages publish` writes straight into a Public Root
and no service runs:

```text literal
PAGES_PUBLIC_ROOT=pages-public pages publish \
  --file report.html --slug report
```

The command prints the Page directory:

```text literal
<current-directory>/pages-public/report/
```

The local Public Root comes from `PAGES_PUBLIC_ROOT` or the config file. If
neither is set, `pages publish` uses the current working directory. See
[configuration.md](configuration.md) for the Publish inputs and
[publishing.md](publishing.md) for optional Named Identity scopes.
