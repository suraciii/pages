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
directory by default. The default configuration directory is `.pages/` under
the operating system's user config directory. On Linux this is typically
`~/.config/.pages`; on macOS it is typically `~/Library/Application
Support/.pages`; on Windows it is typically `%AppData%\.pages`.

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

The serve flags and their environment fallbacks are in the
[command contract](../design/cli.md#pages-serve). The inputs an operator most
often changes:

```text literal
pages serve
```

- `--destination` must be a local path. It is the Public Root that the
  Service reads and writes. When omitted, it is `pages-public/` under the
  current directory. Serve rejects a URL.
- `--listen` defaults to `127.0.0.1:3103`. Set `:443` for a direct public
  listener, or keep the loopback default behind a TLS proxy. It is one
  `host:port` address: the port is 1 to 65535, and an IPv6 host uses brackets.
  An empty host means every interface. A non-loopback listener without TLS
  logs a warning: plaintext exposes Tokens and Page content.
- `--tls-cert` and `--tls-key` name an unencrypted PEM certificate chain and
  private key. Set both or neither. TLS 1.2 is the minimum version.
- The tokens file, certificate chain, and private key must stay outside the
  Public Root. The Service resolves symbolic links and refuses to start when
  one is the Public Root or below it.
- `--content-security-policy` replaces the reference policy. The value must be
  one non-empty line. The default blocks scripts. See
  [deployment.md](deployment.md#headers).
- `--tokens-file` and the certificate pair are reloaded on `SIGHUP`,
  independently. A failed reload keeps the previous value. Changing any other
  input needs a restart.
- `--max-upload-bytes` must be positive. It applies to Uploads only; it does
  not limit Reads.
- A missing or empty tokens file starts the Service read-only: reads and local
  mode work; remote Uploads return `401`. Create the file and send `SIGHUP` to
  enable remote Uploads. An unreadable or malformed tokens file exits 1.

A malformed flag or environment value exits 2. A malformed or unreadable
tokens file, a missing certificate pair, a credential path inside the Public
Root, or a bind failure exits 1. A missing or empty tokens file starts
read-only.

`serve` writes no product output to stdout. Startup, warning, reload, and
error logs go to stderr. The startup line carries the listener, Public Root,
tokens file, upload limit, Token count (`tokens=<n>`), TLS state
(`tls=on|off`), and CSP state (`csp=default|custom`). It never carries a Token.
When no Tokens load, a warning says that remote Uploads are disabled.

At startup the Service creates the Public Root and logs the resolved
configuration. It answers `GET /__pages/healthz` and
`HEAD /__pages/healthz` on the listener.

## 3. The publish config file

The config file supplies defaults for every publish input. Flag and
environment values always win over the config file. The file is
`config.json` under the configuration directory. The default directory is
`.pages/` under the operating system's user config directory. Use
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
  `pages-public/` under the current directory.
- `token` stores the Default Identity Token. `identities` maps each Named
  Identity to its secret. `identity` selects one Named Identity. Without it,
  Publish uses the Default Identity.
- Keep the file owner-only when it holds secrets.
- `serve` does not read the config file. Its configuration stays
  explicit.

`PAGES_UPLOAD_TOKEN` carries the full Token and always wins in remote mode. A
pure secret selects the Default Identity. `identity.secret` selects a Named
Identity, so no `--identity` flag is needed for that scope. A `--identity`
value that names a different scope than the Token is a usage error. Local mode
does not read this environment variable and needs no Token at all.

Remote mode reads `identity` and `identities` from this file only when
`PAGES_UPLOAD_TOKEN` is unset. Local mode reads `identity` for the output
directory and never reads a Token.

See the [`pages publish` command contract](../design/cli.md#pages-publish) for
the exact Destination precedence and alias rules.
