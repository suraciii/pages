# Agent Instructions

## Project Context

- Treat [ROADMAP.md](ROADMAP.md) as the source of truth for implementation
  progress.
- Treat [CONTEXT.md](CONTEXT.md) as the source of truth for product language.
  Use its terms.
- Work within the current product contract. Do not add placeholder, partial,
  fake, or speculative features.

## Engineering Principles

- Choose the simplest design that fully meets the current requirements.
- Keep modules small and keep different concerns separate.
- Check the standard library before adding a dependency.
- Keep the write path atomic and the identity boundary explicit. The server
  derives identity from the verified token only.
- Keep the upload server on loopback. A static file server hosts the
  public reads.

## Architecture Constraints

- Follow [design/architecture.md](design/architecture.md) when changing the
  upload path or the publish contract.
- Follow [design/testing.md](design/testing.md) when changing tests.

## Documentation

- Write or update the product or design spec before implementation.
- Use `docs/` for product requirements and user language.
- Use `design/` for technical design.
- Follow [docs/AGENTS.md](docs/AGENTS.md) before editing `docs/`.
- Follow [design/AGENTS.md](design/AGENTS.md) before editing `design/`.
- When a document differs from the implementation, add a clear `Gap` section.

## Collaboration

- Work in a separate worktree.
- Work in reviewed batches. Stop after each batch and report the result.
- Keep changes within the assigned files and preserve unrelated changes.
- Write commit messages in simple English. State the actual change.
- Before handoff, inspect the diff and report the verification result.

## Verification

- Before handoff, run the repository gate: `make ci`.
- Report failed tests, missing tools, and environment limits. Do not hide them.
