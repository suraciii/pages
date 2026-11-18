# Getting Started

Run pages from zero to a live Page in one pass. Every command is
complete; replace the example paths and names with your own. Details and
alternatives are linked at each step.

## 1. Install

Install the binary with the Go toolchain:

```text literal
go install github.com/suraciii/pages@latest
```

Make sure Go's binary install directory is on the service host's `PATH`.

Developers build from the repository instead:

```text literal
go build -o pages .
```

## 2. Issue a Token

Skip this step if you publish only from the service host with local mode. A
Token is needed for remote mode.

On the service host, issue a Default Identity Token:

```text literal
pages generate-token
```

The command prints the Token once:

```text literal
7v9A_example-secret
```

Keep the printed Token for the Publisher. `generate-token` creates the default
tokens file under the user config directory with mode `0600`. See
[configuration.md](configuration.md) for the tokens file rules. A real host
uses the service paths in [deployment.md](deployment.md) instead.

## 3. Run the Service

`pages serve` is the Service: it serves Pages to readers, accepts Uploads, and
answers health checks on one listener. Run it in the foreground to try:

```text literal
pages serve
```

The default listener is `127.0.0.1:3103`. The default Public Root is
`pages-public/` under the current directory; the Service creates it. Check
that it is ready:

```text literal
curl --fail http://127.0.0.1:3103/__pages/healthz
```

It logs one line with the resolved configuration. If you skipped step 2, the
line shows `tokens=0` and the Service warns that remote Uploads are disabled;
local mode still works. Leave it running and use a second terminal for the
remaining steps. For a public host, run it under systemd or Docker; both are in
[deployment.md](deployment.md).

## 4. Publish a Page

Create one Page file:

```text literal
printf '<!doctype html><title>Report</title><h1>Ready</h1>\n' > report.html
```

### Same host: no Token

When the Publisher can write the Public Root directly, use local mode:

```text literal
pages publish --file report.html --slug report
<current-directory>/pages-public/report/
```

No Token and no Upload are involved. The running Service serves the result at
`http://127.0.0.1:3103/report/`.

### Remote: Token

For a Publisher on another machine, or to exercise the remote path on the
service host, put the Token from step 2 in the environment and use the
Service URL as the Destination:

```text literal
export PAGES_UPLOAD_TOKEN='7v9A_example-secret'
pages publish --file report.html --slug report \
  --dest http://127.0.0.1:3103
```

The command uploads the file, verifies the public URL through the same
Service, and prints it when the Page is live:

```text literal
http://127.0.0.1:3103/report/
```

A loopback HTTP Destination is still remote mode and needs a Token.

A `.zip` file publishes a directory Page whose root file is `index.html`.

Open the Destination root to browse the Page Index:

```text literal
http://127.0.0.1:3103/
```

## 5. Put It on the Public Internet

A public host needs TLS because an Upload carries a Token. The Service reads a
certificate and key file:

- Issue a certificate with any ACME client or certificate authority. DNS-01
  works without a port; HTTP-01 needs a standalone challenge on port 80 or a
  proxy that serves the challenge. The Service does not serve `/.well-known/`.
- Make the certificate chain and private key readable by the service account.
  Keep both outside the Public Root.
- Run the Service on port 443. A non-root account needs
  `CAP_NET_BIND_SERVICE`; the systemd unit in
  [deployment.md](deployment.md) shows it.

```text literal
pages serve \
  --listen :443 \
  --tls-cert /etc/pages/tls/fullchain.pem \
  --tls-key /etc/pages/tls/privkey.pem
```

This uses the default `pages-public/` root and the tokens file from step 2.
The deployment guide shows the service-account paths for a long-running host.
Check the public health path:

```text literal
curl --fail https://pages.example.com/__pages/healthz
```

Publish to the public Destination:

```text literal
export PAGES_UPLOAD_TOKEN='7v9A_example-secret'
pages publish --file report.html --slug report \
  --dest https://pages.example.com
```

```text literal
https://pages.example.com/report/
```

A reverse proxy is optional. It can terminate TLS, log requests, or preserve an
existing edge. It forwards every method and path to the Service; it does not
serve the Public Root itself. See
[deployment.md](deployment.md#reverse-proxy).

## Local Publish

A local-path Destination writes straight into a Public Root and needs no
Service, Token, or network. With no Destination, it uses the default
`pages-public/` root:

```text literal
pages publish --file report.html --slug report
```

The command prints the Page directory:

```text literal
<current-directory>/pages-public/report/
```

The Service serves that Page when it uses the same Public Root.

See [configuration.md](configuration.md) for Publish inputs,
[the command contract](../design/cli.md#pages-publish) for their exact
precedence, and [publishing.md](publishing.md) for optional Named Identity
scopes.
