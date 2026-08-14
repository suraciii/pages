# Release

## Design Drivers

- A Go module tag is public and can be cached by module proxies. A published
  tag must identify one immutable commit.
- Release evidence must identify the exact candidate. Evidence from an older
  commit does not apply after `main` changes.
- pages uses source distribution. The project must not add artifact tooling
  when the release does not publish built artifacts.

## Model

A software release has three parts:

- one annotated semantic-version tag with the `v` prefix;
- one GitHub release that points to that tag;
- the source archives that GitHub creates for the tag.

The CLI is distributed as a Go module. The project does not publish prebuilt
binaries or container images. Users may build the included Dockerfile from a
tagged source checkout.

## Semantics

### Candidate

Before a tag is created, all of these conditions must hold for one commit on
`main`:

- The local branch and `origin/main` identify the same commit, and the worktree
  is clean.
- The required GitHub checks pass for that commit.
- `make ci` and `go test -race -count=1 ./...` pass.
- The CLI builds for the supported Linux, macOS, and Windows targets.
- The Dockerfile builds successfully. The build must not push an image.
- Local Publish and the Caddy static-host contract pass a deployment
  acceptance check. The Page returns `200`, `/.pages/*` returns `404`, and the
  required security headers are present.
- Product and design documents have no unresolved `Gap` that affects the
  release.

Release acceptance may use an isolated operating-system directory and
loopback services. It is separate from the hermetic test suite in
[testing.md](testing.md).

### Sequence

1. Record the candidate commit and its successful checks.
2. Get explicit maintainer approval for the public release.
3. Create and push an annotated tag for the approved commit.
4. Verify the remote tag object and its peeled commit.
5. In an isolated install directory, install the exact tag with `go install`,
   run `pages --version`, and perform a minimal local Publish.
6. Create the GitHub release from the existing tag. Do not attach binaries or
   container images.
7. Read the published release and tag back from GitHub and record their exact
   identifiers.

`v0.0.1` is a normal GitHub release, not a GitHub prerelease. Its major-zero
semantic version already identifies it as initial development software.

### Failure

A failed candidate condition stops the release. Fix the cause and validate a
new exact candidate.

A pushed tag must not be moved or deleted. If verification of a pushed tag
finds a defect, do not create or complete the GitHub release. Fix the defect in
a new version.

## Status

The `v0.0.1` candidate is in preparation. No public release tag exists.
