# Configuration and Deployment

Run the publish service and host its pages. This guide shows complete
working configurations. Replace the example names and paths with your
own.

## Overview

A deployment has two parts:

- `pages serve` — accepts Uploads and writes Pages into one public root.
  It binds to loopback only.
- a static host — serves the public root to readers. Caddy is the
  reference; any static file server works.

Both parts must use the same public root directory.

## 1. Tokens file

The tokens file maps each Identity to a high-entropy secret. The secret
must not contain `.`. Only the service account may read the file:

```text literal
{
  "bumble": "replace-with-a-high-entropy-secret"
}
```

```text literal
chmod 600 /etc/pages/tokens.json
```

Generate an upload Token for an Identity with `pages generate-token`. The
command saves the secret and prints the Token once:

```text literal
pages generate-token -tokens-file /etc/pages/tokens.json bumble
```

```text literal
bumble.7v9A...
```

The upload Token for an Identity is `<identity>.<secret>`. Give the
printed Token to the publisher, who stores it in the `tokens` map of
their config file or receives it as `PAGES_UPLOAD_TOKEN` from their
environment. To rotate a Token, run the command again with `-replace`.
New entries take effect on the next reload of `serve`; no restart is
needed.

## 2. Run pages serve

```text literal
pages serve -public-root /srv/pages/public -tokens-file /etc/pages/tokens.json
```

The same configuration as environment variables:

| Variable | Value |
| --- | --- |
| PAGES_PUBLIC_ROOT | /srv/pages/public |
| PAGES_TOKENS_FILE | /etc/pages/tokens.json |
| PAGES_LISTEN_ADDR | 127.0.0.1:3103 |
| PAGES_MAX_UPLOAD_BYTES | 10485760 |

At startup the service creates the public root and logs one line with the
resolved configuration. It answers `GET /healthz` on the loopback
listener.

A systemd unit:

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

## 3. Caddyfile

Caddy is the reference static host. The complete site block:

```text literal
pages.example.com {
    @pages_upload {
        method POST
        path /pages/*
    }

    handle @pages_upload {
        request_body {
            max_size 10485760
        }
        reverse_proxy 127.0.0.1:3103
    }

    @pages_internal {
        path /pages/.pages/*
    }

    respond @pages_internal 404

    redir /pages /pages/ 308

    handle_path /pages/* {
        root * /srv/pages/public
        header {
            Content-Security-Policy "default-src 'none'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self' data:; object-src 'none'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'"
            X-Content-Type-Options nosniff
            Referrer-Policy no-referrer
            X-Frame-Options DENY
            Cache-Control "no-store"
        }
        file_server
    }
}
```

Each part:

- `@pages_upload` proxies only `POST /pages/*` to the service.
- `@pages_internal` blocks the service staging area. It is never public.
- `redir` gives the pages root a trailing slash.
- `handle_path` serves the public root read-only with the security
  headers.

The `request_body max_size` value must match `PAGES_MAX_UPLOAD_BYTES`.

## 4. Docker

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
is `GET /healthz` inside the container.

## 5. Publish a Page

The remote upload address is the only mode switch: `-base-url` or
`PAGES_BASE_URL` selects remote mode. Without it, publishing is local.
The config file supplies the lowest-precedence defaults and sits at
`~/.config/pages/config.json`, or wherever `-config` points:

```text literal
{
  "base-url": "https://pages.example.com",
  "public-root": "/srv/pages/public",
  "tokens": {
    "bumble": "secret-one",
    "fizz": "secret-two"
  },
  "identity": "bumble"
}
```

`tokens` maps each identity to its secret, like the server tokens file.
`identity` names the default identity; with exactly one entry in
`tokens`, the field is optional. Flag and environment values always win
over the config file. Keep the file owner-only when it holds secrets.

Remote mode publishes through the service and verifies the public URL:

```text literal
pages publish -file report.html -slug report -base-url https://pages.example.com
```

The command prints the public URL when the Page is live:

```text literal
https://pages.example.com/pages/bumble/report/
```

Local mode writes directly into the public root on the local machine, for
example on the server itself. No service or network is involved:

```text literal
PAGES_PUBLIC_ROOT=/srv/pages/public pages publish -file report.zip -slug report -identity bumble
/srv/pages/public/bumble/report/
```

With the target and identity in the config file, the same publish is:

```text literal
pages publish -file report.zip -slug report
/srv/pages/public/bumble/report/
```
