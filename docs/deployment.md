# Deployment

Run the publish service and host its pages. This guide shows complete
working configurations. Replace the example names and paths with your
own.

## Overview

A deployment has two parts:

- `pages serve` — accepts Uploads and writes Pages into one public root.
  It binds to loopback only.
- a static host — serves the public root to readers. Caddy is the
  reference; any static file server works.

Both parts must use the same public root directory. The service
configuration (tokens, flags, environment) is in
[configuration.md](configuration.md).

## 1. Prepare the service account and directories

Create the service account, the public root, and the tokens file. Only
the service account may read the tokens file:

```text literal
useradd --system --home-dir /srv/pages --shell /usr/sbin/nologin pages
mkdir -p /srv/pages/public
chown -R pages:pages /srv/pages
chown pages:pages /etc/pages/tokens.json
```

## 2. Run pages serve

`pages serve` is one HTTP service. It accepts Uploads on loopback,
answers `GET /healthz`, and writes Pages into the public root. It never
serves reads; the static host does that. Any process manager can run it.
Two ways follow: systemd on a Linux host, or Docker.

At startup the service creates the public root and logs one line with the
resolved configuration.

### Option A: systemd

The service unit:

```text literal
[Unit]
Description=pages publish service
After=network.target

[Service]
User=pages
ExecStart=/usr/local/bin/pages serve -public-root /srv/pages/public -tokens-file /etc/pages/tokens.json
ExecReload=/bin/kill -HUP $MAINPID
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

Operate the service:

- Add an Identity without downtime: edit the tokens file, then
  `systemctl reload pages`. The server keeps the previous tokens when
  the file fails to load.
- Stop: `systemctl stop pages`. The server shuts down gracefully and
  exits 0.

When the static host runs as a container, the container cannot reach the
server on `127.0.0.1`. Start `pages serve` with `-listen 172.17.0.1:3103`
(the Docker gateway) or use a host-network container; the address stays
private to the host.

### Option B: Docker

```text literal
docker run \
  -v /srv/pages:/srv/pages \
  -p 127.0.0.1:3103:3103 \
  -e PAGES_PUBLIC_ROOT=/srv/pages/public \
  -e PAGES_TOKENS_FILE=/run/secrets/pages_tokens \
  --secret pages_tokens \
  pages:local
```

The write port is published on the host loopback only. The health probe
is `GET /healthz` inside the container. The public root is a volume; the
tokens file is a secret mount.

## 3. Caddyfile

Caddy is the reference static host. The complete site block:

```text literal
pages.example.com {
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

    file_server
}
```

Each part:

- `@pages_upload` proxies every `POST` request to the service; uploads
  are the only writes on the site.
- `@pages_internal` blocks the service staging area. It is never public.
- `header` applies the security headers to every response.
- `file_server` serves the public root read-only.

The `request_body max_size` value must match `PAGES_MAX_UPLOAD_BYTES`.
A path prefix is a choice, not a requirement: with
`handle_path /docs/*` instead of `file_server`, the same site lives at
`https://pages.example.com/docs/...`. Set the same prefix in
`-base-url`.
