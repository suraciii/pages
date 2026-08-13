# CLI

pages is one binary with three subcommands. The product has three
actions: `serve` runs the publish service, `publish` puts one page live,
and `generate-token` issues an upload Token.

## Design Drivers

- One product surface. The implementation has two processes: an upload
  server and an upload client. Users and agents must not learn
  implementation names. The verbs state the product.
- A precise command language. Every subcommand has one grammar, one flag
  set, one stdout contract, and exact exit codes. No aliases, no
  abbreviations, no optional words.
- Agent-friendly output. `publish` and `generate-token` write exactly one
  machine-readable line to stdout and nothing else. Errors go to stderr
  with a non-zero exit code.
- The command surface stays minimal: exactly three verbs. Token listing,
  token deletion, and page listing are not part of the contract and get
  no verbs.
- Deployment configures one static-resources directory. The rest of the
  runtime layout is owned and maintained by `serve`.

Rejected alternatives:

- Two separate binaries (`pages-server`, `pages-cli`). Rejected: the names
  leak the internal process split into the product surface.
- A `serve` mode that also serves reads. Rejected: the static host owns
  the public read path. Keeping the token-holding process off the public
  route is worth the extra host configuration. See
  [deployment.md](deployment.md).
- `pages token` as the token verb. Rejected: the verb must state the
  action. `generate-token` says what the command does.
- A new target parameter such as `-to`. Rejected: the existing
  `-base-url`, `PAGES_BASE_URL`, and `PAGES_PUBLIC_ROOT` already name the
  two targets. A new parameter adds a knob without new information.

## Model

```text literal
pages serve          [flags]
pages publish        [flags]
pages generate-token <identity> [flags]
```

`pages` with no subcommand prints usage to stderr and exits 2.

No global flags. Each subcommand parses its own flag set.

## Semantics

### pages serve

Runs the upload server until it receives SIGINT or SIGTERM, then shuts
down gracefully.

| Flag | Environment | Default | Required |
| --- | --- | --- | --- |
| -public-root | PAGES_PUBLIC_ROOT | none | yes |
| -tokens-file | PAGES_TOKENS_FILE | none | yes |
| -listen | PAGES_LISTEN_ADDR | 127.0.0.1:3103 | no |
| -max-upload-bytes | PAGES_MAX_UPLOAD_BYTES | 10485760 | no |

`-public-root` is the static-resources directory. Deployment picks it;
`serve` creates it at startup and maintains everything under it, including
the internal `.pages/` staging area. See
[architecture.md](architecture.md).

`serve` responds to `GET /healthz` with `200` on the loopback listener.
The path does not authenticate and discloses no state.

On `SIGHUP`, `serve` reloads the tokens file. A failed reload keeps the
previous tokens and logs the error. This lets deployment add an Identity
without a restart.

At startup `serve` logs one line with the resolved values: listen address,
public root, tokens file, and upload byte limit.

The Upload behavior is the contract in [architecture.md](architecture.md).
Exit codes: 0 on clean shutdown, 2 for invalid flags, 1 for startup
failures such as an unreadable tokens file.

### pages publish

Uploads one Page and prints its public URL, or writes it directly into a
public root on the local machine.

Inputs are resolved in this order. Every derivation is deterministic
and reuses existing parameters. A config file supplies the lowest
precedence defaults.

| Input | Resolution |
| --- | --- |
| remote address | `-base-url`, else `PAGES_BASE_URL`, else `config.base-url` |
| mode | a remote address is set → remote; otherwise local |
| local target | `PAGES_PUBLIC_ROOT`, else `config.public-root`, else usage error when local |
| slug | `-slug` only; the flag is required |
| token | `PAGES_UPLOAD_TOKEN`, else `identity.secret` from the config file |
| identity | env token prefix, else `-identity`, else `config.identity`, else the single entry of `config.tokens`, else usage error |

The remote upload address is the only mode switch. Every other value is
a plain configuration. The identity selects which token to use: the
config file stores one token per identity.

| Flag | Environment | Default | Required |
| --- | --- | --- | --- |
| -file | none | none | yes |
| -slug | none | none | yes |
| -base-url | PAGES_BASE_URL | none | no |
| -identity | none | none | no, derived |
| -timeout | none | 90s | no |
| -config | none | ~/.config/pages/config.json | no |

### Config file

The config file supplies defaults for every publish input. Flag and
environment values always win over the config file. JSON format:

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

- The default path is `~/.config/pages/config.json` (the user config
  directory). `-config` names another file. A missing default file is
  fine; a named file must exist.
- All fields are optional. `base-url` is the remote upload address;
  `public-root` is the local public root.
- `tokens` maps each identity to its secret, like the server tokens
  file. `identity` names the default identity. With exactly one entry
  in `tokens`, no `identity` field is needed.
- `-timeout` is intentionally not in the config file: it has one
  sensible default and per-invocation overrides via the flag.
- The file should be readable only by its owner when it holds secrets.
- `serve` does not read the config file. Its configuration stays
  explicit.

Remote mode (a remote address is set):

- The Token comes from `PAGES_UPLOAD_TOKEN`, or from the config
  file. The config file stores only the secret, like the server tokens
  file; the command uses it as `identity.secret`. A flag for the Token
  is rejected: secrets must not enter shell history. The Token is
  required.
- When `PAGES_UPLOAD_TOKEN` is set, the Identity is its prefix and
  `-identity` is ignored. Otherwise the Identity selects the secret
  from the config file.
- The server enforces the upload byte limit.

Local mode (no remote address):

- No server is involved. The Identity is resolved with the table above;
  no token is needed. `-timeout` is ignored. No network request happens.
- The command applies the same zip validation and swap rules as `serve`.
  It has no size limit; only the safety checks run: at most 512 entries
  and an uncompressed total of at most four times the compressed size.
  The caller owns the directory already, so a size limit adds no
  security.
- The target directory is a pages public root: `.pages/` staging is an
  inherent part of it, and the host must not serve it.
- The CLI does not lock. Two concurrent local publishes of the same
  slug are the caller's responsibility.

Common steps:

1. Resolve the inputs with the table above. A resolved value that fails
   validation is a usage error.
2. `-file` ends in `.html` or `.zip`. A `.html` file uploads as
   `text/html` and a `.zip` file as `application/zip`. A directory is
   rejected with a usage error: package it as a zip first.
3. Remote mode: `POST <url>/pages/<slug>` with the Bearer Token and the
   content type of the file. Expect `204`. Then
   `GET <url>/pages/<identity>/<slug>/`. Expect `200` with an HTML
   content type.
4. Local mode: stage the file under `<dir>/.pages/`, run the validation,
   and swap it into place with the same two-rename sequence as `serve`.
   A successful swap is the verification.
5. Remote mode prints the public URL with a trailing slash. Local mode
   prints the absolute page directory with a trailing slash. Both print
   exactly one line and exit 0.

Any step failure prints one error line to stderr and exits 1. Usage errors
exit 2.

### pages generate-token

Issues an upload Token for one Identity, saves it to the tokens file, and
prints it. This is a file operation: no service is involved.

| Argument | Environment | Default | Required |
| --- | --- | --- | --- |
| identity (positional) | none | none | yes |
| -tokens-file | PAGES_TOKENS_FILE | none | yes |
| -replace | none | false | no |

Steps:

1. Validate the identity name with the Slug rules.
2. Read the tokens file; a missing file counts as empty. When the
   identity already exists and `-replace` is not set, print one error
   line to stderr and exit 1.
3. Generate the secret: 32 random bytes, base64url without padding. The
   encoding never contains `.`.
4. Write the tokens file atomically (temporary file in the same
   directory, then rename) with mode `0600`, keeping every other entry.
5. Print one line with the Token:

```text literal
bumble.7v9A...
```

Exit codes: 0 on success, 1 for file failures and a refused replacement,
2 for usage errors.

A newly written Token takes effect on the next tokens reload of `serve`
(`SIGHUP`).

## Examples
```text literal
pages serve -public-root /srv/pages/public -tokens-file /etc/pages/tokens.json

pages generate-token -tokens-file /etc/pages/tokens.json bumble

PAGES_UPLOAD_TOKEN='bumble.secret' pages publish -file report.zip -slug report -base-url https://pages.example.com

pages publish -file report.zip -slug report -config ~/.config/pages/config.json
```

## Status

Implemented. One `pages` binary with the three subcommands above. The Go
module path is `github.com/suraciii/pages`.