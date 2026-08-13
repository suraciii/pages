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

`generate-token` and `serve` use `tokens.json` under the configuration
directory by default. The default configuration directory is `pages/` under
the operating system's user config directory. On Linux this is typically
`~/.config/pages`; on macOS it is typically under `~/Library/Application
Support`; on Windows it is typically under `%AppData%`.

Use `--config-dir` or `PAGES_CONFIG_DIR` to choose another configuration
directory. `--config-dir` takes precedence over `PAGES_CONFIG_DIR`. Use
`--tokens-file` or `PAGES_TOKENS_FILE` to choose an exact Token file; an exact
file path takes precedence over the configuration directory. `generate-token`
creates a missing parent directory with mode `0700` and the tokens file with
mode `0600`.

If the operating system does not provide a user config directory, the command
fails. Set one of the explicit directory or file inputs above. It does not put
`tokens.json` in the current directory.

The Default Identity Token is a pure secret. A Named Identity Token is
`<identity>.<secret>`. Give the printed Token to the Publisher, who stores it
in the config file or receives it as `PAGES_UPLOAD_TOKEN`. To rotate a Token,
run the same command with `--replace`. New entries take effect on the next
reload of `serve`; no restart is needed.

## 2. pages serve

The serve flags and their environment fallbacks:

```text literal
pages serve --destination pages-public
```

| Flag | Environment | Default | Required |
| --- | --- | --- | --- |
| --destination, --dest | PAGES_DESTINATION | current directory | no |
| --config-dir | PAGES_CONFIG_DIR | user config directory/pages | no |
| --tokens-file | PAGES_TOKENS_FILE | user config directory/pages/tokens.json | no |
| --listen | PAGES_LISTEN_ADDR | 127.0.0.1:3103 | no |
| --max-upload-bytes | PAGES_MAX_UPLOAD_BYTES | 10485760 | no |

At startup the service creates the public root and logs one line with the
resolved configuration. It answers `GET /healthz` on the listener and
reloads the tokens file on `SIGHUP`; a failed reload keeps the previous
tokens. The Destination must be a local path. Serve rejects a URL.

## 3. The publish config file

The config file supplies defaults for every publish input. Flag and
environment values always win over the config file. The file is
`config.json` under the configuration directory. The default directory is
`pages/` under the operating system's user config directory. Use
`--config-dir` or `PAGES_CONFIG_DIR` to choose another directory, or use
`--config` to choose an exact file:

```text literal
{
  "destination": "https://pages.example.com",
  "token": "default-secret",
  "identities": {
    "bumble": "secret-one",
    "fizz": "secret-two"
  },
  "identity": "bumble"
}
```

If the operating system does not provide a user config directory, `publish`
continues without a default config file. It does not read `config.json` from
the current directory. Use one of these explicit inputs when a config file is
required.

- `destination` is a local path or an absolute HTTP(S) URL. A path selects
  local mode. A URL selects remote mode. When it is empty, local Publish uses
  the current working directory.
- `token` stores the Default Identity Token. `identities` maps each Named
  Identity to its secret. `identity` selects one Named Identity. Without it,
  Publish uses the Default Identity.
- Keep the file owner-only when it holds secrets.
- `serve` does not read the config file. Its configuration stays
  explicit.

`PAGES_UPLOAD_TOKEN` carries the full Token and always wins in remote mode. A
pure secret selects the Default Identity. `identity.secret` selects a Named
Identity. Local mode does not read this environment variable.

See the [`pages publish` command contract](../design/cli.md#pages-publish) for
the exact Destination precedence and alias rules.
