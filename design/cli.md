# CLI

pages is one binary with three product actions. `serve` runs the Service,
`publish` puts one page live, and `generate-token` issues an upload Token.
The root help and `serve --help` carry the deployment guidance an operator
needs; `publish --help` carries the method guidance an agent needs. There is
no separate instruction document to fall out of date.

This specification is the source of truth for command grammar, Destination
syntax, input precedence, defaults, output, and exit codes. User documents
show runnable examples and link here for the exact contract. It describes the
command contract for the self-hosted Service. Compare `pages --version` before
relying on a specific message or flag; `serve --help` and `publish --help`
report the contract revision they were checked against. The current contract
revision is `2026-11-18`.

## Design Drivers

- One product surface. The implementation has one Service process and one
  client process. Users and agents must not learn implementation names. The
  verbs state the product. The CLI is the authoring and operation surface;
  users and agents run `serve`, `publish`, and `generate-token` and do not
  call the HTTP routes directly. Those routes are the Publisher's transport
  and the reader surface.
- A precise command language. Every subcommand has one grammar, one flag
  set, one stdout contract, and exact exit codes. `--dest` is the only
  abbreviation; it is an explicit alias for `--destination`.
- Agent-friendly output. `publish` and `generate-token` write exactly one
  machine-readable line to stdout and nothing else. Errors go to stderr
  with a non-zero exit code.
- The product surface stays minimal: exactly three action verbs. Token
  listing, token deletion, and page listing are not part of the contract and
  get no verbs. Reading the current Pages is a Service read: the Page Index
  serves readers and the Page Record serves programs. Neither needs a verb.
- Version-matched agent instructions. The distributed bootstrap skill only
  finds or installs the CLI. The CLI owns the complete operational skill, so
  command changes cannot leave an installed skill behind.
- Deployment configures one Public Root and one listener. The rest of the
  runtime layout is owned and maintained by `serve`.
- User configuration defaults to the operating system's user config
  directory, but every command may use an explicit configuration directory.

Rejected alternatives:

- Two separate binaries (`pages-server`, `pages-cli`). Rejected: the names
  leak the internal process split into the product surface.
- A static host for reads in front of `serve`. Rejected: it adds a second
  process, a second configuration, and a second owner for headers and TLS.
  The Service serves reads itself. See [deployment.md](deployment.md).
- Two listeners: a public read listener and a loopback Upload listener.
  Rejected: remote Publish would still need a proxy for Uploads, which
  restores the deployment problem the single-Service design removes.
- `pages token` as the token verb. Rejected: the verb must state the
  action. `generate-token` says what the command does.
- Separate local and remote target parameters. Rejected: two mutually
  exclusive parameters expose the execution mode as another user decision.
  One Destination value contains all required information.
- A `--file -` standard-input form. Rejected: a zip Upload needs a reader
  that supports random access, so the form would work for HTML and fail for
  zip. One input channel for both shapes keeps the contract uniform. A
  caller that generates content writes a file first.

## Model

```text literal
pages serve          [flags]
pages publish        [flags]
pages generate-token [flags] [identity]
pages --version
```

`pages` with no subcommand prints usage to stderr and exits 2. `--help` and
`-h` print usage to stdout and exit 0. `--version` prints `pages <version>` to
stdout and exits 0. A tagged module install reports its module version. A
source build without a module version reports `(devel)`. `--version` accepts no
other arguments. Configuration flags belong to their subcommands.

## Semantics

### pages serve

Runs the Service until it receives SIGINT or SIGTERM, then shuts down
gracefully.

| Flag | Environment | Default | Required |
| --- | --- | --- | --- |
| --destination, --dest | PAGES_DESTINATION | `pages-public/` under current directory | no |
| --config-dir | PAGES_CONFIG_DIR | user config directory/.pages | no |
| --tokens-file | PAGES_TOKENS_FILE | user config directory/.pages/tokens.json | no |
| --listen | PAGES_LISTEN_ADDR | 127.0.0.1:3103 | no |
| --max-upload-bytes | PAGES_MAX_UPLOAD_BYTES | 10485760 | no |
| --tls-cert | PAGES_TLS_CERT | none | with --tls-key |
| --tls-key | PAGES_TLS_KEY | none | with --tls-cert |
| --content-security-policy | PAGES_CSP | reference policy | no |

The Destination must be a local path. It resolves to the Public Root.
`serve` rejects an HTTP(S) URL or another URI with a usage error. When no
Destination is set, `pages-public/` under the current directory is used.
Deployment usually sets an explicit Destination; `serve` creates it at startup
and maintains everything under it, including the internal `.pages/` staging
area. See [architecture.md](architecture.md).

The listener serves Reads and Uploads. It is loopback by default so a fresh
install is not public. Deployment sets a public address when readers and
publishers are remote. Without TLS, a non-loopback listener logs a warning:
plaintext exposes Tokens and Page content.

`--listen` is one `host:port` address. The port is a decimal number from 1 to
65535; an IPv6 host uses brackets. An empty host means every interface. A
missing port, port 0, a service name, a URL, or a path is a usage error. A
loopback listener is `localhost` or an IP loopback address. Every other host,
including an empty host or another hostname, is treated as non-loopback.

`--tls-cert` and `--tls-key` must be set together and must name a PEM
certificate chain and an unencrypted PEM private key. Setting only one is a
usage error. A pair that does not load is a startup failure. TLS 1.2 is the
minimum version. See [deployment.md](deployment.md#listener).

`--content-security-policy` replaces the reference policy. The value must be
one non-empty line without carriage return or line feed. The help states that
the default blocks scripts; the exact policy is in
[deployment.md](deployment.md#headers).

`--max-upload-bytes` must be a positive integer. It limits Upload bodies, not
Reads.

#### Configuration resolution

Flags override environment variables; environment variables override
defaults. `serve` reads no configuration file.

An unset or empty environment variable is absent. A flag that is present is
validated even when its value is empty: an empty `--listen`, `--tls-cert`,
`--tls-key`, or `--content-security-policy`, and a non-positive
`--max-upload-bytes`, are usage errors. An empty `--destination` keeps the
`pages-public/` default; an empty `--config-dir` or `--tokens-file` is absent.
A malformed environment value is a usage error; the command does not silently
fall back.

The TLS pair is resolved after precedence: both values or neither.

#### Tokens file

`serve` and `generate-token` share `tokens.json` under the configuration
directory by default. The default configuration directory is `.pages/` under
the operating system's user config directory. `--config-dir` overrides
`PAGES_CONFIG_DIR`; `--tokens-file` and `PAGES_TOKENS_FILE` select an exact
Token file and take precedence over the directory.

A missing or empty tokens file is not a startup failure. The Service starts
with no Tokens, logs `tokens=0`, and rejects remote Uploads with `401`. Local
mode still works because it writes the Public Root directly. Creating the file
and sending `SIGHUP` enables remote Uploads without a restart. An unreadable
or malformed tokens file is a startup failure and exits 1.

When the operating system does not provide a user config directory, the
command must fail instead of using the current directory. An explicit
`--config-dir`, `PAGES_CONFIG_DIR`, `--tokens-file`, or `PAGES_TOKENS_FILE`
does not require the operating system default.

#### Credential paths

The tokens file, certificate chain, and private key must be outside the Public
Root. The Service resolves symbolic links in the Public Root and in each
credential path; when a path does not exist, it resolves the longest existing
prefix and keeps the remaining names. The Service rejects a path that is the
Public Root or below it and exits 1. The tokens file and private key are
secrets. The certificate chain is public, but the Service keeps the complete
TLS file set outside the root so the root contains only Pages and generated
artifacts. The configuration directory may be anywhere as long as the tokens
file itself is outside.

#### Validation order

1. Parse flags. An unknown flag, a single-dash long flag, or a flag syntax
   error is a usage error.
2. Reject positional arguments, the legacy environment names, and a
   `--destination`/`--dest` conflict.
3. Validate the scalar inputs: local Destination, positive upload limit,
   listen format, TLS pair presence, and CSP format.
4. Resolve the tokens file path. An unavailable user config directory exits 1.
5. Resolve absolute paths, including symbolic links, for the Public Root,
   tokens file, certificate, and key. A tokens or TLS path inside the Public
   Root exits 1.
6. Load the tokens file. A missing or empty file starts with no Tokens and
   disables remote Uploads. An unreadable or malformed file exits 1.
7. Load the certificate pair when configured. A missing or malformed pair
   exits 1.
8. Prepare the Public Root: create it, recover staging, and refresh the Page
   Index and Page Record. A create, staging, or recovery failure exits 1. A
   refresh failure is logged and the Service continues.
9. Log the resolved configuration and the plaintext warning when applicable,
   then bind. A bind failure exits 1.

#### Health

`serve` responds to `GET /__pages/healthz` and `HEAD /__pages/healthz` with
`200` on the listener. The path does not authenticate and discloses no state.
It is ready after startup recovery and the initial Page Index refresh attempt.
A refresh failure is logged and does not stop the listener.

#### Output and signals

During operation, `serve` writes no product output to stdout. The startup
line, warning, reload results, and errors go to stderr. Help is the exception
described below.

The startup line carries the listen address, Public Root, tokens file, upload
byte limit, Token count (`tokens=<n>`), TLS state (`tls=on|off`), and CSP state
(`csp=default|custom`). It never carries a Token.

When TLS is off and the listener is non-loopback, one warning line precedes
the startup line. It names the address and says that Tokens and Page content
travel in plaintext. When no Tokens load, one warning line also precedes the
startup line and says that remote Uploads are disabled.

On Unix, SIGHUP reloads the tokens file and, when configured, the certificate
pair independently. A failed reload keeps the previous value for that input
and logs the error; a successful reload of the other input still applies. New
connections use a new certificate; new requests use new Tokens.

Every other input is read at startup. Changing the listener, Destination,
upload limit, CSP, configuration directory, tokens path, or certificate path
needs a restart. On Windows, SIGHUP is unavailable; restart the Service to
reload.

On Unix, SIGINT or SIGTERM stops accepting new requests, drains active
requests for up to 10 seconds, then exits 0. A listener failure exits 1. On
Windows, a console interrupt stops the Service the same way.

The Read and Upload behavior is the contract in
[architecture.md](architecture.md). Exit codes: 0 on SIGINT/SIGTERM shutdown,
2 for invalid flags or environment values, 1 for startup failures such as an
unreadable or malformed tokens file, a credential path inside the Public Root,
a certificate/key file that does not load, or a bind failure.

### pages publish

Uploads one Page and prints its public URL, or writes it directly into a
public root on the local machine.

Inputs are resolved in this order. Every derivation is deterministic
and reuses existing parameters. A config file supplies the lowest
precedence defaults.

| Input | Resolution |
| --- | --- |
| Destination | `--destination` or `--dest`, else `PAGES_DESTINATION`, else `config.destination`, else `pages-public/` under current directory |
| mode | HTTP(S) URL Destination -> remote; local path Destination -> local |
| slug | `--slug` only; the flag is required |
| token | `PAGES_UPLOAD_TOKEN`, else the selected Token from the config file |
| remote identity | env Token scope, which `--identity` must not contradict |
| local identity | `--identity`, else `config.identity`, else Default Identity |

Destination is the only mode input. Every other value is plain configuration.
An absent Identity selects the hidden Default Identity. A Named Identity
selects one entry from `config.identities`.

| Flag | Environment | Default | Required |
| --- | --- | --- | --- |
| --file | none | none | yes |
| --slug | none | none | yes |
| --destination, --dest | PAGES_DESTINATION | `pages-public/` under current directory | no |
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
- When `PAGES_UPLOAD_TOKEN` is set, its scope decides the Identity. A
  `--identity` value that names the same scope is accepted; one that names a
  different scope, or an invalid one, is a usage error, because the command
  cannot tell which scope the caller meant and the Token is authoritative.
  Without the environment Token, `--identity` or `config.identity` selects a
  Named Identity and `config.identities` must hold its secret; without one,
  `config.token` supplies the Default Identity Token.
- The public URL is `<destination>/<slug>/` for the Default Identity and
  `<destination>/@<identity>/<slug>/` for a Named Identity.
- A remote Destination may use HTTP or HTTPS. A non-loopback Destination must
  use HTTPS because an Upload carries a Token. HTTP is intended for a
  loopback trial. The CLI does not enforce this; the Service logs a warning
  when its listener is non-loopback without TLS.
- A loopback HTTP(S) Destination is still remote mode and requires a Token.
  The Service never treats a loopback source address as authenticated: a
  proxy forwards remote requests from loopback, so the source address is not
  an identity.
- The Service enforces the upload byte limit.

Local mode (the Destination is a local path):

- Local mode is the no-Token path for a Publisher on the Service host. It
  needs write access to the Public Root; the CLI does not borrow the Service
  account's permissions. A loopback HTTP(S) Destination is not local mode.
- No Service is involved. `PAGES_UPLOAD_TOKEN` is ignored. `--identity` or
  `config.identity` selects a Named Identity; without one, Publish uses the
  Default Identity. `--timeout` is ignored. No network request happens.
- The command applies the same zip validation and swap rules as `serve`.
  It has no size limit; only the safety checks run: at most 512 entries
  and an uncompressed total of at most four times the compressed size.
  The caller owns the directory already, so a size limit adds no
  security.
- The target directory is a pages public root: `.pages/` staging is an
  inherent part of it, and the Service must not serve it.
- When no Destination is set, `pages-public/` under the current working
  directory is the Destination and Public Root. The Publisher resolves it
  when the command starts and prints the resulting absolute Page directory.
- The CLI does not lock. Two concurrent local publishes of the same
  slug are the caller's responsibility.
- Local mode applies the same body rules as remote mode: an empty file and a
  file that is not valid UTF-8 are usage errors. The size limit is the one
  rule that differs, because the caller already owns the directory and a
  limit adds no security there.
- Local mode refreshes the Page Index and the Page Record after the swap. A
  refresh failure is reported and exits 1 while the Page stays in place.

Common steps:

1. Resolve the inputs with the table above. A resolved value that fails
   validation is a usage error.
2. `--file` names one existing regular file that ends in `.html` or `.zip`.
   A `.html` file uploads as `text/html` and a `.zip` file as
   `application/zip`. A directory is rejected with a usage error that says to
   package it as a zip. The check order is: missing `--file`, then directory,
   then unsupported extension, so a directory named `report.html` reports the
   directory rule rather than the extension rule.
3. The `--slug` value must be a valid Slug: at most 63 lowercase letters,
   digits, or hyphens, and it starts with a letter or digit. An invalid value
   is a usage error.
4. The file must not be empty and its bytes must be valid UTF-8. Both modes
   apply these two rules. An empty file or invalid UTF-8 is a usage error and
   must not change the existing Page.
5. Remote mode: `POST <url>/<slug>` with the Bearer Token and the content type
   of the file. Expect `204`. Then `GET <url>/<slug>/` for the Default Identity
   or `GET <url>/@<identity>/<slug>/` for a Named Identity. Expect `200` with
   an HTML content type.
6. Local mode: stage the file under `<dir>/.pages/`, run the validation, and
   swap it into `<dir>/<slug>/` or `<dir>/@<identity>/<slug>/`. A successful
   swap is the Verification.
7. Refresh the Page Index and the Page Record.
8. Remote mode prints the public URL with a trailing slash. Local mode prints
   the absolute Page directory with the operating system's trailing path
   separator. Both print exactly one line and exit 0.

Any step failure prints one error line to stderr and exits 1. Usage errors
exit 2.

### Failure after the effect

Two steps can fail after the Page is already in place:

- Remote mode: the Upload returned `204` but the verification `GET` failed.
  The error line reports the verification result. The Page is live; the
  Service, its listener, or a proxy is the likely cause. The error must not
  claim the Page is absent.
- Local mode: the swap succeeded but the refresh failed. The error line
  reports the refresh failure. The Page is live; a non-Page-Index file at
  `index.html` or `index.json` is the likely cause.

Both cases exit 1. The caller checks the printed Page path or URL before it
publishes again. Publishing the same Slug again replaces the Page, so a
repeat after a timeout is safe and cannot create a second Page.

### Input channel

`publish` reads its Page content from the file that `--file` names. It does
not read standard input: a zip Upload needs a reader that supports random
access, and one input channel for both shapes keeps the contract uniform.

### pages generate-token

Issues a Token, saves it to the tokens file, and prints it. With no Identity
argument it issues a pure-secret Default Identity Token. With an Identity it
issues an `identity.secret` Named Identity Token. This is a file operation: no
Service is involved.

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

### Help as the operator and agent surface

The root help is the product summary. Its first line is
`pages publishes static HTML pages as a self-hosted service.` It lists the
three actions, describes `serve` as `run the self-hosted service`, and ends
with the `pages <command> --help` pointer. `--version` is unchanged.

`serve --help` is the deployment surface for an operator. It prints the flag
definitions and the guidance the flags cannot express:

- one listener serves Reads and Uploads, and the default listener is loopback;
- the default Public Root is `pages-public/` under the current directory;
- direct TLS uses both certificate flags; a proxy keeps the loopback listener
  and forwards every method and path;
- the default CSP blocks scripts, and the flag replaces the whole policy;
- the health path is `/__pages/healthz`;
- the tokens file, certificate chain, and private key stay outside the Public
  Root, or the Service refuses to start;
- `serve` reads no config file; its inputs are flags and environment values;
- a missing tokens file starts read-only; `pages generate-token` enables
  remote Uploads;
- SIGHUP reloads token and certificate contents only; other changes restart;
- the contract link and the revision it was checked against.

`publish --help` is the method surface for an agent. It prints the flag
definitions and then the guidance a caller cannot derive from the flags:
which mode to choose, what each mode requires, the recommended command for
each, how to check the result, and the recovery for a missing Token or
Identity and for the two failures that happen after the Page is in place. The
local recommendation omits the Destination and shows the `pages-public/`
default output. It recommends HTTPS for a non-loopback Destination, names the
Service or its proxy in recovery, not a static host, states that a loopback
HTTP Destination is still remote mode and needs a Token, and points to
`pages serve --help` for the Service side.

The guidance lives in the binary rather than in a separate document, so it
cannot drift from the command it describes. It states the contract revision
it was checked against. Help writes to stdout and exits 0; the same text
accompanies a usage error on stderr.

Help guidance is a summary. It must not restate the whole contract: flag
precedence, exit codes, and the exact grammar stay in this document, and the
guidance links here instead of copying them.

## Examples
```text literal
pages serve

pages serve --destination /srv/pages/public --listen :443 --tls-cert /etc/pages/tls/fullchain.pem --tls-key /etc/pages/tls/privkey.pem

pages generate-token bumble

pages publish --file report.html --slug report

pages publish --file report.zip --slug report --dest https://pages.example.com --config publisher.json

pages publish --file report.zip --slug report --config <user-config-dir>/.pages/config.json

pages publish --help
```

## Status

Implemented. `serve` serves Reads and Uploads on one listener, defaults the
Destination to `pages-public/`, starts read-only with no Tokens, validates
listen and credential paths, supports file-based TLS and a CSP override,
reloads Tokens and certificates on SIGHUP, and prints the startup contract.
The root help and both subcommand help pages describe the self-hosted
Service.
