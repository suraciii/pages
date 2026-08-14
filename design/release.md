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

One manually dispatched GitHub Actions workflow owns the release sequence.
The required `version` input is a stable semantic version with a `v` prefix,
such as `v0.0.1`. Dispatching the workflow from `main` is the maintainer's
explicit approval to publish that version from the captured `main` commit. The
approval confirms that the manual deployment acceptance and document review
are complete. A push or pull request must not publish a release.

## Semantics

### Candidate

Before a tag is created, all of these conditions must hold for one commit on
`main`:

- The workflow was dispatched from `main`, and the captured commit still
  identifies `origin/main` before the tag is created.
- The requested version is valid, and its tag does not exist.
- The required GitHub checks pass for that commit.
- `make ci` and `go test -race -count=1 ./...` pass.
- The CLI builds for Linux, macOS, and Windows on `amd64` and `arm64`.
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

1. Merge the release preparation and wait for the exact `main` commit's
   required GitHub checks. Complete the Caddy deployment acceptance and the
   product and design document review.
2. Dispatch the Release workflow from `main` with the version to publish:

   ```text literal
   gh workflow run release.yml --ref main -f version=v0.0.1
   ```

3. Validate the request and run the deterministic candidate conditions before
   the workflow creates public state.
4. Create and push an annotated tag for the captured commit.
5. Verify the remote tag object and its peeled commit.
6. In an isolated install directory, install the exact tag with `go install`,
   run `pages --version`, and perform a minimal local Publish.
7. Create the GitHub release from the existing tag. Do not attach binaries or
   container images.
8. Read the published release and tag back from GitHub and record their exact
   identifiers.

`v0.0.1` is a normal GitHub release, not a GitHub prerelease. Its major-zero
semantic version already identifies it as initial development software.

### Failure

A failed condition before the tag is pushed stops the release without creating
public state. Fix the cause and validate a new exact candidate.

A pushed tag must not be moved or deleted. If exact-tag verification fails, the
workflow must not create the GitHub release. Fix the defect in a new version.

## Status

The `v0.0.1` candidate is in preparation. No public release tag exists.
