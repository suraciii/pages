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
  keep every Identity. A failed reload keeps the previous Tokens. Signal
  delivery belongs to the production process boundary and is not simulated.
- `GET /healthz` returns 200 without authentication.
- Size limits must stay configurable through `ServerConfig` so tests do not
  wait on real time or allocate large resources.
- Tests must not use `t.TempDir`, `os.CreateTemp`, physical file paths,
  `httptest.NewServer`, fixed ports, sleeps, retries, polling, or global test
  serialization.
- Test dependencies must be explicit and scoped to one test. Production
  defaults remain available without test-only global setters.
- CLI tests inject stdout, stderr, the environment, the working directory, and
  user config directory discovery. They cover root command dispatch and the
  minimal local publish command without a Destination.
- Skill command tests capture stdout and stderr in memory. They prove that the
  embedded output is a complete skill, help does not print the skill, invalid
  arguments are usage errors, and no process capability is consulted.

## Gate

`make ci` runs `fmt-check`, `tidy-check`, `vet`, and `test`. Every test command
must run under `timeout -k 10s` with a bounded overall duration. Run the gate
before handoff.
