# CLI

pages is one binary with three product actions and one self-description
command. `serve` runs the publish service, `publish` puts one page live,
`generate-token` issues an upload Token, and `skill` prints the agent skill
for this CLI version.

This specification is the source of truth for command grammar, Destination
syntax, input precedence, defaults, output, and exit codes. User documents
show runnable examples and link here for the exact contract.

## Design Drivers

- One product surface. The implementation has two processes: an upload
  server and an upload client. Users and agents must not learn
  implementation names. The verbs state the product.
- A precise command language. Every subcommand has one grammar, one flag
  set, one stdout contract, and exact exit codes. `--dest` is the only
  abbreviation; it is an explicit alias for `--destination`.
- Agent-friendly output. `publish` and `generate-token` write exactly one
  machine-readable line to stdout and nothing else. Errors go to stderr
  with a non-zero exit code.
- The product surface stays minimal: exactly three action verbs. Token
  listing, token deletion, and page listing are not part of the contract and
  get no verbs. `skill` is read-only CLI metadata, not a product action.
- Version-matched agent instructions. The distributed bootstrap skill only
  finds or installs the CLI. The CLI owns the complete operational skill, so
  command changes cannot leave an installed skill behind.
- Deployment configures one static-resources directory. The rest of the
  runtime layout is owned and maintained by `serve`.
- User configuration defaults to the operating system's user config
  directory, but every command may use an explicit configuration directory.

Rejected alternatives:

- Two separate binaries (`pages-server`, `pages-cli`). Rejected: the names
  leak the internal process split into the product surface.
- A `serve` mode that also serves reads. Rejected: the static host owns
  the public read path. Keeping the token-holding process off the public
  route is worth the extra host configuration. See
  [deployment.md](deployment.md).
- `pages token` as the token verb. Rejected: the verb must state the
  action. `generate-token` says what the command does.
- Separate local and remote target parameters. Rejected: two mutually
  exclusive parameters expose the execution mode as another user decision.
  One Destination value contains all required information.

## Model

```text literal
pages serve          [flags]
pages publish        [flags]
pages generate-token [flags] [identity]
pages skill
pages --version
```

`pages` with no subcommand prints usage to stderr and exits 2. `--help` and
`-h` print usage to stdout and exit 0. `--version` prints `pages <version>` to
stdout and exits 0. A tagged module install reports its module version. A
source build without a module version reports `(devel)`. `--version` accepts no
other arguments. Configuration flags belong to their subcommands.

## Semantics

### pages serve

Runs the upload server until it receives SIGINT or SIGTERM, then shuts
down gracefully.

| Flag | Environment | Default | Required |
| --- | --- | --- | --- |
| --destination, --dest | PAGES_DESTINATION | current directory | no |
| --config-dir | PAGES_CONFIG_DIR | user config directory/.pages | no |
| --tokens-file | PAGES_TOKENS_FILE | user config directory/.pages/tokens.json | no |
| --listen | PAGES_LISTEN_ADDR | 127.0.0.1:3103 | no |
| --max-upload-bytes | PAGES_MAX_UPLOAD_BYTES | 10485760 | no |

The Destination must be a local path. It resolves to the Public Root.
`serve` rejects an HTTP(S) URL or another URI with a usage error. Deployment
usually sets an explicit Destination; `serve` creates it at startup and
maintains everything under it, including the internal `.pages/` staging area.
See [architecture.md](architecture.md).

`serve` and `generate-token` share `tokens.json` under the configuration
directory by default. The default configuration directory is `.pages/` under
the operating system's user config directory. `--config-dir` overrides
`PAGES_CONFIG_DIR`; `--tokens-file` and `PAGES_TOKENS_FILE` select an exact
Token file and take precedence over the directory.

When the operating system does not provide a user config directory, the
command must fail instead of using the current directory. An explicit
`--config-dir`, `PAGES_CONFIG_DIR`, `--tokens-file`, or `PAGES_TOKENS_FILE`
does not require the operating system default.

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
| Destination | `--destination` or `--dest`, else `PAGES_DESTINATION`, else `config.destination`, else current directory |
| mode | HTTP(S) URL Destination -> remote; local path Destination -> local |
| slug | `--slug` only; the flag is required |
| token | `PAGES_UPLOAD_TOKEN`, else the selected Token from the config file |
| remote identity | env Token scope, else `--identity`, else `config.identity`, else Default Identity |
| local identity | `--identity`, else `config.identity`, else Default Identity |

Destination is the only mode input. Every other value is plain configuration.
An absent Identity selects the hidden Default Identity. A Named Identity
selects one entry from `config.identities`.

| Flag | Environment | Default | Required |
| --- | --- | --- | --- |
| --file | none | none | yes |
| --slug | none | none | yes |
| --destination, --dest | PAGES_DESTINATION | current directory | no |
| --identity | none | Default Identity | no |
| --timeout | none | 90s | no |
| --config-dir | PAGES_CONFIG_DIR | user config directory/.pages | no |
| --config | none | user config directory/.pages/config.json | no |

### Config file

The config file supplies defaults for every publish input. Flag and
environment values always win over the config file. JSON format:

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

- The default path is `config.json` under the configuration directory. The
  default directory is `.pages/` under the operating system's user config
  directory. `--config-dir` overrides `PAGES_CONFIG_DIR`; `--config` names an
  exact file and takes precedence over the directory. A missing default file
  is fine; a named file must exist.
- When the operating system does not provide a user config directory,
  `publish` has no default config file. It continues without one and must not
  search the current directory for `config.json`. `--config`, `--config-dir`,
  or `PAGES_CONFIG_DIR` still supplies an explicit path.
- All fields are optional. `destination` is a local path or an absolute
  HTTP(S) URL.
- `token` stores the Default Identity Token. `identities` maps each Named
  Identity to its secret. `identity` selects one Named Identity when the
  Publisher should not use the Default Identity.
- `--timeout` is intentionally not in the config file: it has one
  sensible default and per-invocation overrides via the flag.
- The file should be readable only by its owner when it holds secrets.
- `serve` does not read the config file. Its configuration stays
  explicit.

Destination syntax:

- A value that starts with `http://` or `https://` must be an absolute URL
  with a Host. It must not contain a query or fragment. It selects remote
  mode and may contain a path prefix.
- A value that starts with another URI scheme followed by `://` is invalid.
- Every other value is a local OS path and selects local mode. This includes
  relative paths, absolute paths, Windows drive paths, and UNC paths.
- `--destination` and `--dest` are the same input. A command that sets both
  is invalid, even when the values are equal.

Remote mode (the Destination is an HTTP(S) URL):

- The Token comes from `PAGES_UPLOAD_TOKEN` or the config file. A pure secret
  selects the Default Identity. `identity.secret` selects a Named Identity.
  A flag for the Token is rejected: secrets must not enter shell history.
- When `PAGES_UPLOAD_TOKEN` is set, its scope decides the Identity and
  `--identity` is ignored. Otherwise `--identity` or `config.identity` selects
  a Named Identity; without one, `config.token` supplies the Default Identity
  Token.
- The public URL is `<destination>/<slug>/` for the Default Identity and
  `<destination>/@<identity>/<slug>/` for a Named Identity.
- The server enforces the upload byte limit.

Local mode (the Destination is a local path):

- No server is involved. `PAGES_UPLOAD_TOKEN` is ignored. `--identity` or
  `config.identity` selects a Named Identity; without one, Publish uses the
  Default Identity. `--timeout` is ignored. No network request happens.
- The command applies the same zip validation and swap rules as `serve`.
  It has no size limit; only the safety checks run: at most 512 entries
  and an uncompressed total of at most four times the compressed size.
  The caller owns the directory already, so a size limit adds no
  security.
- The target directory is a pages public root: `.pages/` staging is an
  inherent part of it, and the host must not serve it.
- When no Destination is set, the current working directory is the
  Destination and Public Root. The Publisher resolves it when the command
  starts and prints the resulting absolute Page directory.
- The CLI does not lock. Two concurrent local publishes of the same
  slug are the caller's responsibility.

Common steps:

1. Resolve the inputs with the table above. A resolved value that fails
   validation is a usage error.
2. `--file` ends in `.html` or `.zip`. A `.html` file uploads as
   `text/html` and a `.zip` file as `application/zip`. A directory is
   rejected with a usage error: package it as a zip first.
3. Remote mode: `POST <url>/<slug>` with the Bearer Token and the content type
   of the file. Expect `204`. Then `GET <url>/<slug>/` for the Default Identity
   or `GET <url>/@<identity>/<slug>/` for a Named Identity. Expect `200` with
   an HTML content type.
4. Local mode: stage the file under `<dir>/.pages/`, run the validation, and
   swap it into `<dir>/<slug>/` or `<dir>/@<identity>/<slug>/`. A successful
   swap is the Verification.
5. Remote mode prints the public URL with a trailing slash. Local mode prints
   the absolute Page directory with the operating system's trailing path
   separator. Both print exactly one line and exit 0.

Any step failure prints one error line to stderr and exits 1. Usage errors
exit 2.

### pages generate-token

Issues a Token, saves it to the tokens file, and prints it. With no Identity
argument it issues a pure-secret Default Identity Token. With an Identity it
issues an `identity.secret` Named Identity Token. This is a file operation: no
service is involved.

| Argument | Environment | Default | Required |
| --- | --- | --- | --- |
| identity (positional) | none | Default Identity | no |
| --config-dir | PAGES_CONFIG_DIR | user config directory/.pages | no |
| --tokens-file | PAGES_TOKENS_FILE | user config directory/.pages/tokens.json | no |
| --replace | none | false | no |

Steps:

1. When present, validate the Identity name with the Slug rules.
2. Create the parent directory with mode `0700` when it is missing. Read the
   tokens file; a missing file counts as empty. When the selected Default or
   Named Identity already has a Token and `--replace` is not set, print one
   error line to stderr and exit 1.
3. Generate the secret: 32 random bytes, base64url without padding. The
   encoding never contains `.`.
4. Hold an exclusive lock for the complete read, check, and write transaction.
   The lock must serialize calls in one process and calls from separate
   processes. Write the tokens file atomically (temporary file in the same
   directory, then rename) with mode `0600`, keeping every other entry.
5. Print one line with the Token:

```text literal
7v9A_example-secret
bumble.7v9A_example-secret
```

Exit codes: 0 on success, 1 for file failures and a refused replacement,
2 for usage errors.

A newly written Token takes effect on the next tokens reload of `serve`
(`SIGHUP`).

### pages skill

Prints the complete agent skill for the installed CLI version. The output is
a valid `SKILL.md`, including its YAML frontmatter, and is the source of truth
for how an agent uses pages.

`skill` accepts no arguments or flags other than `--help` and `-h`. On
success, it writes only the skill to stdout and exits 0. Help writes only the
command usage to stdout and exits 0. Any other input writes the error and
usage to stderr and exits 2.

The command is deterministic. It must not read configuration, inspect the
file system, access the network, or depend on the current directory. The
skill is embedded in the binary so its instructions and command grammar have
the same version.

The separately distributed bootstrap skill is intentionally incomplete. It
tries `pages skill`. When the CLI is absent or does not support that command,
it runs `go install github.com/suraciii/pages@latest`, then runs `pages skill`
and follows that output. It must not manage the user's shell or copy
operational command syntax that can become stale.

## Examples
```text literal
pages serve --destination pages-public

pages generate-token bumble

pages publish --file report.zip --slug report --dest https://pages.example.com --config publisher.json

pages publish --file report.zip --slug report --config <user-config-dir>/.pages/config.json

pages skill
```

## Status

Implemented. One `pages` binary has the commands above. Publish and Serve use
the Destination grammar. The Go module path is
`github.com/suraciii/pages`.
