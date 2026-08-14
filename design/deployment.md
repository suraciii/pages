# Deployment — Hosting the Public Root

`pages serve` accepts Uploads and places static resources into the public
root. Hosting that public root is a separate choice. Caddy is one option;
any static file server works. pages does not require any specific host.

## Design Drivers

- The upload server holds tokens and writes files. It must stay on
  loopback. The static host is the only public listener.
- pages owns the write path only. TLS, headers, and static serving belong
  to the host.
- Publish verification must prove what a reader sees: the `GET` must
  travel the same public route a reader uses.
- Published pages come from agents and are untrusted. The public read
  path must not execute scripts.

## Model

```text diagram
reader ──GET /<slug>/ or /@<identity>/<slug>/──► static host ──► public root
publisher ──POST /<slug>──► static host ──proxy──► pages serve (loopback)
                                                             │
                                                             ▼ swaps in
                                                        public root
```

The deployment chooses the public root once and gives the same path to
`pages serve --destination` and to the static host's document root.

The tokens file must sit outside the public root and outside every host
document root.

## Host requirements

A host that serves the public root must:

- Serve the public root read-only at the remote address path.
- Never serve `/.pages/` or other dot-prefixed paths. The staging area is
  not public.
- Proxy `POST /<slug>` to the loopback server when `pages publish`
  must work through the public URL. Without this route, publishing
  works only against the loopback server directly and the read
  verification cannot run.
- Send the reference headers below, or equivalents.

On Unix, pages creates the Public Root, Identity directories, and Page
directories with mode `0755`. It creates `.pages/` and staging scope
directories with mode `0700`. The static host can use a separate operating
system identity without gaining access to staging.

The remote address is fully custom: any host and any path prefix. A deployment
that mounts the public root under a prefix, for example `handle_path
/docs/*`, uses that URL in `pages publish --destination`, and uploads go to
`POST <destination>/<slug>`.

`pages publish` always uploads and verifies through one remote Destination,
so a full deployment needs a host with both routes.

## Caddy reference

Caddy is the reference host. The snippet below is one valid
configuration; other hosts can follow the same rules.

```text literal
@pages_upload {
    method POST
}

handle @pages_upload {
    request_body {
        max_size 10485760
    }
    reverse_proxy 127.0.0.1:3103
}

@pages_internal {
    path /.pages/*
}

respond @pages_internal 404

header {
    Content-Security-Policy "default-src 'none'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self' data:; object-src 'none'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'"
    X-Content-Type-Options nosniff
    Referrer-Policy no-referrer
    X-Frame-Options DENY
    Cache-Control "no-store"
}

root * /srv/pages/public
file_server
```

## Semantics

- Only `POST` reaches the server. Every other method goes to the
  static handler. `/.pages/*` never serves: the staging area is not
  public. The server also rejects non-POST methods: host routing is the
  primary boundary and the server check is defense in depth.
- The host body limit and `pages serve --max-upload-bytes` must be the
  same value. The host enforces the limit for the public; the server
  enforces it for the loopback hop. Operators change both together.
- The static host root and `pages serve --destination` must name the same
  Public Root.
- The CSP header blocks script execution: pages are static display
  documents, never interactive applications. `style-src 'self'` allows
  stylesheets from a zip page; `img-src 'self' data:` allows bundled
  images; `'unsafe-inline'` keeps inline styles working in single-file
  pages.
- `Cache-Control: no-store` follows from latest-only semantics: the same
  slug is replaced in place, so a cached copy would go stale after the
  next publish.
- The verification `GET` in `pages publish` passes through the same static
  route as a reader. A `200` with an HTML content type proves both the
  write and the read path.

## Docker

The image runs `pages serve`. Deployment mounts the public root and keeps
the write port on the host loopback:

```text literal
docker run \
  -v /srv/pages:/srv/pages \
  -v /etc/pages/tokens.json:/run/secrets/pages_tokens:ro \
  -p 127.0.0.1:3103:3103 \
  --user "$(id -u pages):$(id -g pages)" \
  pages:local
```

- `-p 127.0.0.1:3103:3103` publishes the write port on the host loopback
  only. Publishing it on `0.0.0.0` puts the upload endpoint on the public
  network and must not happen.
- The health probe is `GET /healthz` on the container port.
- `/srv/pages` is the host volume for the Public Root.
- `/etc/pages/tokens.json` is a Linux host file mounted read-only at
  `/run/secrets/pages_tokens`. The tokens file stays outside the Public Root.
- The container runs with the host `pages` account's numeric UID and GID so it
  can read the mounted Token and write the Public Root.

## systemd

`pages serve` is one HTTP service. systemd and Docker are two equivalent
ways to run it; see [docs/deployment.md](../docs/deployment.md)
for complete units.

- Start: `systemctl start pages`.
- Add an Identity without downtime: edit the tokens file, then
  `systemctl reload pages` (SIGHUP).
- Stop: `systemctl stop pages` (SIGTERM, graceful shutdown).

## Status

The Caddy snippet above is the reference configuration. Deployment files
stay outside the repository by design; this document is their reference.
