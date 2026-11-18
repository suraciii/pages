# Testing

The default test suite uses only the standard library and hermetic in-memory
resources. It must not access a physical file system, bind a network port,
start a process, or use an external service.

## Strategy

- File behavior uses an explicit file-system dependency. Production uses the
  operating-system implementation. Tests create one in-memory implementation
  per test and must not replace package globals. The in-memory implementation
  resolves relative paths against its configured working directory, as the
  operating-system implementation does.
- Server tests call `Server.ServeHTTP` with `httptest.ResponseRecorder` and
  in-memory request bodies. They inspect the in-memory Public Root.
- Read tests cover: the root Page Index, a Named Identity Index, a single-file
  Page, a zip Page asset, and `HEAD` responses. They cover trailing-slash
  redirects whose `Location` does not use the `Host` header, a directory
  without `index.html`, a file requested with a trailing slash, an unknown
  path, and a missing Page. They also cover `404` for `.pages`, every
  dot-prefixed segment, `.` and `..`, repeated separators, backslashes,
  invalid UTF-8, control characters, an invalid Named Identity, and an
  unsupported method with `Allow: GET, HEAD, POST`. A Page named `healthz`
  stays reachable at `/healthz/` and is not shadowed by health.
- Read representation tests cover `text/html; charset=utf-8`, the standard
  extension mapping, `application/octet-stream` for an unknown extension,
  `Content-Length`, and the absence of compression, range handling, and
  `304` responses.
- Confinement tests use an injected file-system implementation that reports a
  symlink component or a special file; the Service must return `404` without
  serving the target. A Read must not change the Public Root.
- Header tests assert the reference security headers on `200`, `301`, `404`,
  `405`, health, and Upload responses. A CSP override replaces the policy
  exactly; an empty value and a value with a line break are usage errors.
- The health route `GET /__pages/healthz` and `HEAD /__pages/healthz` returns
  `200` without authentication. A different path under `__pages` returns
  `404`.
- TLS tests cover a certificate without a key, a key without a certificate,
  an unreadable or malformed pair, and a successful pair. Certificate reload
  is tested through an injected loader: a successful reload replaces the
  pair, and a failed reload keeps the previous pair. No test binds a port.
- The startup warning for a non-loopback plaintext listener is tested through
  an injected logger.
- Client tests use an injected `http.RoundTripper`. They must not use
  `httptest.NewServer`, `net.Listen`, or another real socket.
- Tests that combine the client and server route requests through a scripted
  in-process Transport. They do not resolve DNS or cross a process boundary.
- Every rejection test must first publish a valid Page and then assert
  that a rejected Upload leaves the existing files unchanged.
- Zip tests build archives in memory with `archive/zip` and cover: a valid
  directory page, a missing root `index.html`, directory entries that are
  skipped, symlink entries, path traversal names, absolute names,
  non-canonical names, backslashes, leading-dot names, non-UTF-8 names,
  exact and case-insensitive duplicate names, too many entries, and an
  oversized uncompressed total.
- Content-Type tests cover HTML with no parameter, HTML with only
  `charset=utf-8`, parameter-free zip, and rejection of every other parameter.
- Rejected HTML Upload tests assert that the existing Page and staging area
  are unchanged.
- Local Publish validation tests cover an empty file and a file that is not
  valid UTF-8. Both are usage errors and must leave the existing Page
  unchanged, matching the remote-mode rule.
- Failure-after-effect tests cover the two partial cases. A verification
  failure after a `204` reports the failure while the Page stays in place. A
  Page Index refresh failure after a successful swap reports the failure
  while the Page stays in place. Neither case may claim the Page is absent.
- Swap tests cover: first publish, replace of a directory page, the
  replacement of a zip Page with a single-file Page, the brief-absence window
  is not asserted but the end state is, Public Root and final Page directories
  use mode `0755` while `.pages/` uses mode `0700`, and startup recovery restores
  `old-<rand>-<slug>` under the Default or Named Identity staging scope when
  the target is missing and clears other staging leftovers. Recovery must
  preserve a displaced Page when it cannot inspect the target.
- Scope tests publish the same Slug under the Default Identity and one Named
  Identity, then assert that `/slug/` and `/@identity/slug/` do not overlap.
- Token tests cover pure-secret Default Identity Tokens,
  `identity.secret` Named Identity Tokens, the structured tokens file, and
  token reload behavior. They also prove that concurrent calls in one process
  keep every Identity, and that a loopback request without a Token is `401`:
  authentication never uses the source address. A missing or empty tokens file
  starts with no Tokens; remote Uploads return `401` while local mode still
  works. An unreadable or malformed tokens file exits 1. A failed reload keeps
  the previous Tokens. Signal delivery belongs to the production process
  boundary and is not simulated.
- Page Index tests cover the Default Identity label, newest-first Page order,
  Slug order for equal file times, Named Identity order, empty scopes, ignored
  invalid or incomplete entries, relative links, startup recovery of
  interrupted Index replacement, shared Identity navigation on every Index,
  current-scope links, the Published Time rendered next to each Page link, and
  refresh after local and Service Publish.
  File times are supplied by an in-memory file-system test dependency.
- Page Record tests cover the `slug` and `published` fields with the exact
  RFC 3339 value, the same order and membership as the Index of the same
  scope, the empty `pages` array for an empty scope, and refusal to replace
  an existing `index.json` that is not a Page Record. A Record carries no
  scope field, and a foreign file that embeds the `pages` key inside a nested
  object is not a Record. A Record and its Index are read from one refresh
  and compared.
- Size limits must stay configurable through `ServerConfig` so tests do not
  wait on real time or allocate large resources.
- Tests must not use `t.TempDir`, `os.CreateTemp`, physical file paths,
  `httptest.NewServer`, fixed ports, sleeps, retries, polling, or global test
  serialization.
- Test dependencies must be explicit and scoped to one test. Production
  defaults remain available without test-only global setters.
- CLI tests inject stdout, stderr, the environment, the working directory, and
  user config directory discovery. They cover root command dispatch and the
  minimal local publish command without a Destination, which asserts
  `pages-public/report/` under the injected working directory. The root version
  test injects a fixed build version and asserts the exact stdout contract.
- Serve CLI tests cover: the TLS flag pair, a missing or malformed certificate
  pair, listen formats (missing port, port 0, service name, URL, path), an
  empty or multiline Content Security Policy, a non-positive upload limit,
  malformed environment values, and a tokens or TLS path inside the Public
  Root, including through a symlinked root. Invalid inputs exit 2; missing
  certificate files, unreadable or malformed token files, containment
  failures, root failures, and bind failures exit 1.
- Startup output tests use injected writers. They assert that the startup line
  carries the listener, root, tokens file, limit, Token count (`tokens=<n>`),
  `tls` state, and `csp` state, and never a Token. The plaintext warning
  appears only for a non-loopback listener without TLS. A no-token start logs
  `tokens=0` and the remote-disabled warning. The successful bind belongs to
  release acceptance, not this suite.
- Reload tests cover tokens and certificate independence: one failed reload
  keeps that input and logs, while a successful reload of the other input still
  applies.
- Help tests assert that `pages publish --help` prints the mode, Identity, and
  recovery guidance along with the flags, recommends HTTPS, names the Service
  rather than a static host, states that a loopback HTTP Destination still
  needs a Token, shows the `pages-public/` local default, and points to
  `pages serve --help`, and that the same guidance reaches stderr with a usage
  error. The `serve` notes also accompany a serve usage error.
  `serve --help` covers the Service model, default root, TLS or proxy, CSP,
  health, credential paths, token recovery, and restart/reload behavior. The
  guidance text is a constant, so a test reads it directly rather than
  duplicating it.

## Gate

`make ci` runs `fmt-check`, `tidy-check`, `vet`, and `test`. Every test command
must run under `timeout -k 10s` with a bounded overall duration. Run the gate
before handoff.

## Status

Implemented. The suite covers the Read path and headers, health, no-token
startup, CLI validation and startup output, TLS loading and reload, token
reload, help guidance, and the existing Upload and Publish paths.
