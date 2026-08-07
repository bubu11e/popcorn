# 2. Keep the `github.com/bubu11e/popcorn` module path

Date: 2026-08-02

## Status

Accepted

## Context

Every other service in the fleet declares its module as
`forgejo.home.mpli.fr/julien/<name>`, matching the canonical remote. Popcorn
declares `github.com/bubu11e/popcorn` and is the only one that does not, so each
repository audit flags it as drift.

It is not drift. Popcorn is the fleet's public repository: it is mirrored to
GitHub, and `.github/workflows/` runs pre-commit on external pull requests
precisely so an outside contributor can be reviewed without access to the
Forgejo pipeline. That contributor's `go build` has to resolve the module path,
and `forgejo.home.mpli.fr` does not exist outside the home network.

The canonical remote stays Forgejo either way. The module path is about who can
fetch the code, not about where it is pushed.

## Decision

Keep `github.com/bubu11e/popcorn`, and treat it as the documented exception to
the fleet standard rather than something to migrate. `CLAUDE.md` carries the
same statement so the exception is visible without opening `docs/adr/`.

The rule the exception follows: a module path names an address the code's
audience can actually reach. Private services can name the private forge because
nobody outside reaches for them; a public one cannot.

## Consequences

- `go get github.com/bubu11e/popcorn` works for anyone, and a fork builds without
  a `replace` directive or a `GOPRIVATE` entry.
- Import paths across the repo, the `-ldflags -X` target in the Dockerfile, and
  any pinned reference elsewhere all stay as they are. Migrating would have
  touched every file and broken outside forks for no gain.
- The audit will keep noticing the difference. This ADR is the answer to it, and
  the reason the finding is closed rather than fixed.
- If Popcorn ever stops being mirrored publicly, the reason for the exception is
  gone and it should move to `forgejo.home.mpli.fr/julien/popcorn` with the rest.
