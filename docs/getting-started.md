# Getting Started

Run pages from zero to a live page in one pass. Every command is
complete; replace the example paths and names with your own. Details and
alternatives are linked at each step.

## 1. Install

Build the binary and put it on the PATH of the service host:

```text literal
go build -o /usr/local/bin/pages ./cmd/pages
```

## 2. Issue a Token

Create the tokens file and the public root, then issue a Token for one
Identity:

```text literal
mkdir -p /etc/pages /srv/pages/public
echo '{}' > /etc/pages/tokens.json
chmod 600 /etc/pages/tokens.json

pages generate-token -tokens-file /etc/pages/tokens.json bumble
```

The command prints the Token once:

```text literal
bumble.7v9A...
```

`generate-token` creates a missing tokens file, so the `echo` line is
only needed when the file must exist before the service starts. See
[configuration.md](configuration.md) for the tokens file rules.

## 3. Run pages serve

`pages serve` is one HTTP service: it accepts Uploads on loopback,
answers `GET /healthz`, and writes Pages into the public root. Run it in
the foreground to try:

```text literal
pages serve -public-root /srv/pages/public -tokens-file /etc/pages/tokens.json
```

It logs one line with the resolved configuration. For a real host, run
it under systemd or Docker; both are in [deployment.md](deployment.md).

## 4. Serve the public root

A static host serves the public root to readers. Any static file server
works; Caddy is the reference. The host must route `POST /pages/*` to
`pages serve`, block `/pages/.pages/*`, and serve everything else from
the public root. The complete Caddyfile is in
[deployment.md](deployment.md).

## 5. Publish a Page

On the publisher machine, the remote upload address is the only mode
switch. Publish through the public URL:

```text literal
pages publish -file report.html -slug report -base-url https://pages.example.com
```

The command uploads the file, verifies the public URL, and prints it
when the Page is live:

```text literal
https://pages.example.com/pages/bumble/report/
```

A `.zip` file publishes a directory page whose root file is
`index.html`.

Without `-base-url`, `pages publish` writes straight into a public root
and no service runs:

```text literal
pages publish -file report.html -slug report -identity bumble
/srv/pages/public/bumble/report/
```

The local target comes from `PAGES_PUBLIC_ROOT` or the config file. See
[configuration.md](configuration.md) for the publish inputs.
