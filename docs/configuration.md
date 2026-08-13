# Configuration

This document defines every configuration input: the tokens file, the
serve flags, the publish config file, and the environment variables. A
complete run from zero to a live page is in
[getting-started.md](getting-started.md). Deployment is in
[deployment.md](deployment.md).

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
command creates a missing tokens file, saves the secret, and prints the
Token once:

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

## 2. pages serve

The serve flags and their environment fallbacks:

```text literal
pages serve -public-root /srv/pages/public -tokens-file /etc/pages/tokens.json
```

| Flag | Environment | Default | Required |
| --- | --- | --- | --- |
| -public-root | PAGES_PUBLIC_ROOT | none | yes |
| -tokens-file | PAGES_TOKENS_FILE | none | yes |
| -listen | PAGES_LISTEN_ADDR | 127.0.0.1:3103 | no |
| -max-upload-bytes | PAGES_MAX_UPLOAD_BYTES | 10485760 | no |

At startup the service creates the public root and logs one line with the
resolved configuration. It answers `GET /healthz` on the listener and
reloads the tokens file on `SIGHUP`; a failed reload keeps the previous
tokens.

## 3. The publish config file

The config file supplies defaults for every publish input. Flag and
environment values always win over the config file. The file sits at
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

- `base-url` is the remote upload address; `public-root` is the local
  public root. A remote address selects remote mode; without one,
  publishing is local.
- `tokens` maps each identity to its secret, like the server tokens
  file. `identity` names the default identity; with exactly one entry
  in `tokens`, the field is optional.
- The secret is used as `identity.secret`, the Token format.
- Keep the file owner-only when it holds secrets.
- `serve` does not read the config file. Its configuration stays
  explicit.

The environment variable `PAGES_UPLOAD_TOKEN` carries the full Token
`identity.secret` and always wins; its prefix decides the Identity.
