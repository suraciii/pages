# Configuration

This document defines every configuration input: the tokens file, the
serve flags, the publish config file, and the environment variables. A
complete run from zero to a live page is in
[getting-started.md](getting-started.md). Deployment is in
[deployment.md](deployment.md).

## 1. Tokens file

The tokens file stores one optional Default Identity Token and optional Named
Identity secrets. A secret uses base64url characters: letters, digits, `_`,
and `-`. Only the service account may read the file:

```text literal
{
  "token": "default-secret",
  "identities": {
    "bumble": "named-secret"
  }
}
```

Generate a Default Identity Token with no argument, or a Named Identity Token
with one Identity argument. The command creates a missing tokens file with
mode `0600`, saves the secret, and prints the Token once:

```text literal
pages generate-token
pages generate-token bumble
```

`generate-token` and `serve` use `/etc/pages/tokens.json` by default. Set
`PAGES_TOKENS_FILE` or `--tokens-file` to use another path.

The Default Identity Token is a pure secret. A Named Identity Token is
`<identity>.<secret>`. Give the printed Token to the Publisher, who stores it
in the config file or receives it as `PAGES_UPLOAD_TOKEN`. To rotate a Token,
run the same command with `--replace`. New entries take effect on the next
reload of `serve`; no restart is needed.

## 2. pages serve

The serve flags and their environment fallbacks:

```text literal
pages serve --public-root /srv/pages/public
```

| Flag | Environment | Default | Required |
| --- | --- | --- | --- |
| --public-root | PAGES_PUBLIC_ROOT | none | yes |
| --tokens-file | PAGES_TOKENS_FILE | /etc/pages/tokens.json | no |
| --listen | PAGES_LISTEN_ADDR | 127.0.0.1:3103 | no |
| --max-upload-bytes | PAGES_MAX_UPLOAD_BYTES | 10485760 | no |

At startup the service creates the public root and logs one line with the
resolved configuration. It answers `GET /healthz` on the listener and
reloads the tokens file on `SIGHUP`; a failed reload keeps the previous
tokens.

## 3. The publish config file

The config file supplies defaults for every publish input. Flag and
environment values always win over the config file. The file sits at
`~/.config/pages/config.json`, or wherever `--config` points:

```text literal
{
  "remote": "https://pages.example.com",
  "public-root": "/srv/pages/public",
  "token": "default-secret",
  "identities": {
    "bumble": "secret-one",
    "fizz": "secret-two"
  },
  "identity": "bumble"
}
```

- `remote` is the remote upload address; `public-root` is the local
  public root. A remote address selects remote mode; without one,
  publishing is local.
- `token` stores the Default Identity Token. `identities` maps each Named
  Identity to its secret. `identity` selects one Named Identity. Without it,
  Publish uses the Default Identity.
- Keep the file owner-only when it holds secrets.
- `serve` does not read the config file. Its configuration stays
  explicit.

`PAGES_UPLOAD_TOKEN` carries the full Token and always wins in remote mode. A
pure secret selects the Default Identity. `identity.secret` selects a Named
Identity. Local mode does not read this environment variable.
