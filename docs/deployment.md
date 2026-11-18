# Linux Deployment

`pages serve` is the self-hosted Service. It serves Pages to readers and
accepts Uploads on one listener. No static host is required. A reverse proxy
is optional and may add TLS termination, access logging, authentication, or an
existing network edge.

## Overview

One Service process owns one Public Root and one listener:

- Readers use `GET` and `HEAD` for Pages, Page assets, the Page Index, the
  Page Record, and health.
- Publishers use `POST` with a Token to replace one Page.
- The Service sends the security headers and applies the Content Security
  Policy. There is no second server to configure.
- The Service can terminate TLS itself from a certificate and key file, or a
  proxy can terminate TLS and forward to a loopback listener.

The Service configuration (tokens, flags, environment) is in
[configuration.md](configuration.md).

## 1. Prepare the service account and directories

Create the service account, its Public Root, the tokens directory, and the
certificate directory:

```text literal
useradd --system --home-dir /srv/pages --shell /usr/sbin/nologin pages
install -d -o pages -g pages /srv/pages/public /etc/pages /etc/pages/tls
runuser -u pages -- pages generate-token --tokens-file /etc/pages/tokens.json
```

Keep the tokens file outside the Public Root. The Service refuses to start when
the tokens file, certificate chain, or private key is the Public Root or below
it. Give the Service account read access to the tokens file and to the
certificate chain and private key.

A same-host Publisher that uses local mode needs write access to the Public
Root, for example by running as the `pages` account. That path uses no Token.
The Service's loopback HTTP route still requires a Token.

A local-only deployment can skip `generate-token`. A Service with no tokens
starts read-only: reads and local mode work; remote Uploads return `401`.
Create the tokens file and reload to enable remote Uploads.

## 2. Obtain a certificate

A public listener needs TLS: an Upload carries a Token.

- Issue a certificate with any ACME client or certificate authority.
- DNS-01 works without a port. HTTP-01 works with a standalone challenge on
  port 80, or through a proxy that serves the challenge.
- The Service blocks `/.well-known/` with every other dot-prefixed path. It
  cannot answer an ACME challenge itself.
- Put the chain and key outside the Public Root. The private key should be
  readable only by the Service account or its group. The key must be an
  unencrypted PEM private key.
- Renew the files, then reload the Service: `systemctl reload pages` or
  `kill -HUP <pid>`. A failed reload keeps the old certificate.

A certificate for a reverse proxy is outside the Service. Renew it with the
proxy.

## 3. Run the Service with systemd

This unit runs the Service directly with TLS on port 443. The non-root
account needs `CAP_NET_BIND_SERVICE` for a port below 1024.

```text literal
[Unit]
Description=pages publishing service
After=network-online.target
Wants=network-online.target

[Service]
User=pages
Group=pages
ExecStart=/usr/local/bin/pages serve --destination /srv/pages/public --tokens-file /etc/pages/tokens.json --listen :443 --tls-cert /etc/pages/tls/fullchain.pem --tls-key /etc/pages/tls/privkey.pem
ExecReload=/bin/kill -HUP $MAINPID
AmbientCapabilities=CAP_NET_BIND_SERVICE
CapabilityBoundingSet=CAP_NET_BIND_SERVICE
Restart=on-failure

[Install]
WantedBy=multi-user.target
```

```text literal
systemctl daemon-reload
systemctl enable --now pages
systemctl status pages
journalctl -u pages -f
```

At startup the Service recovers interrupted swaps, refreshes the Page Index,
and logs one line with the resolved configuration. It is ready when
`GET /__pages/healthz` returns `200`. A startup refresh failure is logged and
does not stop the listener.

`SIGHUP` reloads the tokens file and the certificate pair independently. A
failed reload keeps the previous value for that input and logs the error.
Changing the listener, Destination, upload limit, CSP, configuration
directory, tokens path, or certificate path needs a restart.

### Reverse proxy

If a proxy already terminates TLS, bind the Service to loopback and let the
proxy forward requests:

```text literal
[Service]
User=pages
Group=pages
ExecStart=/usr/local/bin/pages serve --destination /srv/pages/public --tokens-file /etc/pages/tokens.json --listen 127.0.0.1:3103
ExecReload=/bin/kill -HUP $MAINPID
Restart=on-failure
```

The proxy must:

- Forward every method and path. The Service serves both reads and Uploads.
- Strip a path prefix before forwarding. The Service serves `/`.
- Preserve the `Authorization`, `Content-Type`, and `Content-Length` request
  headers and the response headers.
- Allow at least the configured upload byte limit in the request body.
- Add TLS and, if needed, access logging. It must not serve the Public Root
  itself.

A proxy may serve its own ACME HTTP-01 challenge before it forwards to the
Service.

## 4. Run the Service with Docker

The project does not publish an official container image. Build the image from
the release tag. The image runs `pages serve` as an unprivileged user and
listens on `0.0.0.0:3103` inside the container.

```text literal
git checkout v0.0.1
docker build -t pages:0.0.1 .
```

Direct TLS:

```text literal
docker run \
  -v /srv/pages:/srv/pages \
  -v /etc/pages/tokens.json:/run/secrets/pages_tokens:ro \
  -v /etc/pages/tls:/run/secrets/pages_tls:ro \
  -e PAGES_TLS_CERT=/run/secrets/pages_tls/fullchain.pem \
  -e PAGES_TLS_KEY=/run/secrets/pages_tls/privkey.pem \
  -p 443:3103 \
  --user "$(id -u pages):$(id -g pages)" \
  pages:0.0.1
```

Behind a TLS proxy:

```text literal
docker run \
  -v /srv/pages:/srv/pages \
  -v /etc/pages/tokens.json:/run/secrets/pages_tokens:ro \
  -p 127.0.0.1:3103:3103 \
  --user "$(id -u pages):$(id -g pages)" \
  pages:0.0.1
```

- `-p 443:3103` publishes the TLS listener. Publishing the plaintext port on
  the host's `0.0.0.0` exposes Tokens and must not happen.
- In the proxy case the container still listens on `0.0.0.0:3103`, so the
  Service logs its plaintext warning. The host publishes that port on loopback
  only, and that mapping is the exposure boundary.
- `/srv/pages` is the host volume for the Public Root.
- `/etc/pages/tokens.json` is mounted read-only. It stays outside the Public
  Root. A local-only container can omit the token mount: the Service starts
  read-only and local mode still works.
- The container runs with the host `pages` account's numeric UID and GID so it
  can read the mounted files and write the Public Root.
- The health probe uses the same scheme as the listener:
  `GET /__pages/healthz`.

## 5. HTTP contract and headers

Readers use these paths:

| Path | Content |
| --- | --- |
| `/` | root Page Index |
| `/index.json` | root Page Record |
| `/<slug>/` | Default Identity Page |
| `/<slug>/<path>` | Page asset |
| `/@<identity>/` | Named Identity Page Index |
| `/@<identity>/index.json` | Named Identity Page Record |
| `/@<identity>/<slug>/` | Named Identity Page |
| `/__pages/healthz` | health, no authentication |

A Page URL without its trailing slash redirects to the slash form. Dot-prefixed
paths, including `/.pages/` and `/.well-known/`, return `404`. The Service
does not list directories.

### Headers

Every response carries these headers:

```text literal
Content-Security-Policy: default-src 'none'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self' data:; object-src 'none'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'
X-Content-Type-Options: nosniff
Referrer-Policy: no-referrer
X-Frame-Options: DENY
Cache-Control: no-store
```

The default policy blocks scripts and external resources. It allows inline and
same-origin styles, same-origin images and fonts, and data-URI images and
fonts, so the generated Page Index works without an exception. A Page that
uses only same-origin styles, images, and fonts, and no script, renders under
the default policy.

To allow same-origin script files, replace the whole policy and add
`script-src 'self'`:

```text literal
--content-security-policy "default-src 'none'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self' data:; object-src 'none'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'"
```

An inline script needs `'unsafe-inline'` or a `sha256-` hash in the policy. The
policy is global to the Service, so a hash binds the policy to one exact
script and must be updated when that Page changes. Prefer a same-origin script
file.

The Service owns these headers. A proxy must pass them through unchanged. A
proxy that sets its own Content Security Policy takes ownership of script
execution for every Page.

`Cache-Control: no-store` follows from latest-only semantics: a Slug is
replaced in place, so a cached copy would go stale after the next Publish.

## 6. Operations

- Health: `GET /__pages/healthz` or `HEAD /__pages/healthz` over the configured
  scheme.
- Add or rotate a Token: edit the tokens file, then `systemctl reload pages`.
  A failed reload keeps the previous tokens.
- Start without tokens: a missing or empty tokens file starts the Service
  read-only. Reads and local mode work; remote Uploads return `401`. Create
  the file and reload to enable remote Uploads.
- Renew a certificate: replace the files, then `systemctl reload pages`. A
  failed reload keeps the old certificate.
- The token reload and the certificate reload are independent. One failure
  does not block the other.
- Stop: `systemctl stop pages` or `SIGTERM`. The Service shuts down gracefully
  and exits 0.
- Back up the Public Root if the current Pages matter. pages keeps no archive,
  revision, or rollback state.

The Service is intentionally small. It has:

- no read authentication: every published Page is public;
- no virtual hosts: one Service serves one Public Root;
- no access log beyond startup, reload, and error messages;
- no compression;
- no range requests or conditional responses;
- no rate limit on Uploads; a Token and the upload byte limit are the controls;
- no ACME client and no HTTP-to-HTTPS redirect;
- no HSTS header.

Place a proxy in front of the Service when one of these is required.
