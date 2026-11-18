# Deployment - Hosting the Service

`pages serve` is the public Service. It serves Pages and accepts Uploads on
one listener. A static host is not part of the deployment. A reverse proxy is
optional and may add TLS termination, access logging, authentication, or an
existing network edge.

## Design Drivers

- One Service, one listener, one directory. Deployment must not require a
  second server configuration that can disagree about the Public Root, the
  Upload route, or the headers.
- TLS must be available without an external dependency. The Service accepts a
  certificate and key file. A trusted proxy may terminate TLS instead. pages
  does not implement ACME.
- Public Pages come from agents and are untrusted. The Service sends the
  reference security headers. The default Content Security Policy blocks
  scripts. The operator decides whether to relax it.
- Uploads carry secrets. The default listener is loopback. A public listener
  should use TLS. Plaintext is allowed only behind a trusted TLS terminator.
- Deployment owns certificate issuance, process supervision, and backups.
  The Service owns the Public Root and its staging area.
- The Service must not cut off a Page with a short fixed write deadline. It
  bounds header and idle time instead.

## Model

```text diagram
reader/publisher --GET, HEAD, POST--> pages serve --read/write--> public root
                                            |
                                            +--> .pages staging (private)
                                            +--> tokens file
                                            +--> certificate and key (optional)

optional:
reader --HTTPS--> TLS proxy --HTTP loopback--> pages serve
```

One public address carries Reads and Uploads. The HTTP method selects the
route. The public address is also the remote Destination: a Publisher sends
`POST <destination>/<slug>` and verifies `GET <destination>/<slug>/` through
the same Service.

The Service always serves the Public Root at `/`. If a proxy mounts it under a
path prefix, the proxy must strip the prefix before it forwards the request.
The Publisher then uses the prefixed URL as its Destination.

## Listener

- `--listen` selects the address. The default is `127.0.0.1:3103`.
- `--tls-cert` and `--tls-key` name a PEM certificate chain and an unencrypted
  PEM private key. Both must be set together. TLS 1.2 is the minimum version.
  HTTP/2 is available on a TLS listener.
- The tokens file, certificate chain, and private key stay outside the Public
  Root. The Service refuses to start when one is the Public Root or below it.
- With TLS, the listener serves HTTPS only. The Service does not listen on
  port 80 and does not redirect HTTP to HTTPS. A deployment that needs a
  redirect uses a proxy or a separate front end.
- Without TLS, a non-loopback listener logs a warning at startup. A
  non-loopback plaintext listener exposes Tokens and Page content and must sit
  behind a trusted TLS terminator.
- `SIGHUP` re-reads the tokens file and the certificate pair independently. A
  failed reload keeps the previous value for that input and logs the error.
  Every other input is startup-only and needs a restart.
- The health path is `GET /__pages/healthz` or `HEAD /__pages/healthz` on the
  same listener, with the same scheme. It returns `200` after startup recovery
  and the initial Page Index refresh attempt. A refresh failure is logged and
  does not stop the listener.

## Headers

Every response carries the reference security headers defined in the product
contract at [../docs/deployment.md](../docs/deployment.md#headers). This
includes `404`, `405`, redirects, health, and Upload responses.

The default policy blocks all scripts and every external resource. It allows
the inline style that the generated Page Index needs, same-origin styles,
images, and fonts, and data: images and fonts. A Page that uses only
same-origin styles, images, and fonts, and no script, renders under the
default policy.

`--content-security-policy` or `PAGES_CSP` replaces the whole policy. The value
must be one non-empty line without carriage return or line feed. The product
contract shows the reference policy and the same-origin script variant. The
policy is global to the Service, so an inline-script hash binds the policy to
one exact script and must be updated when that Page changes. Prefer a
same-origin script file.

The Service owns the headers. A proxy must pass them through unchanged. A
proxy that sets its own Content Security Policy takes ownership of script
execution for every Page.

`Cache-Control: no-store` follows from latest-only semantics: a Slug is
replaced in place, so a cached copy would go stale after the next Publish.

## Reverse proxy

A proxy is optional. When one is used:

- Bind the Service to loopback and forward to it. The proxy is the only public
  listener.
- Forward every method and path. There is no separate static route: the
  Service serves both reads and Uploads.
- Strip a path prefix before forwarding. The Service serves `/`.
- Preserve the `Authorization`, `Content-Type`, and `Content-Length` request
  headers and the response headers.
- Allow at least `--max-upload-bytes` in the request body. A lower proxy limit
  rejects a valid Upload before the Service sees it.
- Terminate TLS at the proxy if the loopback hop is trusted and private.

A proxy may serve its own ACME HTTP-01 challenge. The Service cannot: it
blocks `/.well-known/` with every other dot-prefixed path.

## Certificates

The Service reads certificate files; it never requests a certificate.

- Issue the certificate with any ACME client or certificate authority. DNS-01
  works without a port. HTTP-01 works with a standalone challenge on port 80
  or through a proxy that serves the challenge.
- Give the Service account read access to the certificate chain and the
  private key. Keep both outside the Public Root; the Service refuses to start
  otherwise.
- The private key should be readable only by the Service account or its group,
  and it must be an unencrypted PEM private key.
- Renew, then reload: `systemctl reload pages` or `kill -HUP <pid>`. A failed
  reload keeps the old certificate.
- A certificate for a proxy is outside the Service. Renew it with the proxy.

## systemd

The unit runs the Service directly with TLS on port 443. The non-root account
needs `CAP_NET_BIND_SERVICE` for a port below 1024.

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

A deployment that terminates TLS at a proxy removes the TLS flags, sets
`--listen 127.0.0.1:3103`, and removes the capability line.

Operate the Service:

- Add or rotate a Token: edit the tokens file, then `systemctl reload pages`.
  The Service keeps the previous tokens when the file fails to load.
- Start without tokens: a missing or empty tokens file starts the Service
  read-only. Reads and local mode work; remote Uploads return `401`. Create
  the file and reload to enable remote Uploads.
- Renew a certificate: replace the files, then `systemctl reload pages`.
- Stop: `systemctl stop pages`. The Service shuts down gracefully and exits 0.
- Health: `GET /__pages/healthz` over the configured scheme.

## Docker

The project does not publish an official container image. Build it from the
release tag. The image runs `pages serve` as an unprivileged user and listens
on `0.0.0.0:3103` inside the container.

```text literal
git checkout v0.0.1
docker build -t pages:0.0.1 .
```

Direct TLS, with the Public Root, Token file, and certificate mounted:

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

Behind a TLS proxy, omit the certificate mount and TLS variables, and publish
the port on the host loopback only:

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

## Operational limits

The Service is intentionally small. A deployment that needs these features
places a proxy in front of it:

- No read authentication. Every published Page is public.
- No virtual hosts. One Service serves one Public Root.
- No access log. The Service logs startup, reload results, and errors only.
- No compression.
- No range requests or conditional responses.
- No rate limit on Uploads. A Token and the upload byte limit are the
  controls.
- No ACME client and no HTTP-to-HTTPS redirect.
- No HSTS header.

## Status

Implemented. The Service is the only public listener, supports direct
file-based TLS with reload, sends the reference security headers, and starts
read-only when the tokens file is missing or empty.
