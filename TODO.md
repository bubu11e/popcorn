# Repository Health TODO

**Date:** 2026-08-02
**Stack:** Go 1.26 (Gin, `html/template` + `static/`)

Audit against the conventions shared by the sibling services. This repo is the
furthest from the shared shape: the observability surface is largely absent.

## Critical (Security)

- [x] **No `gitleaks` hook in `.pre-commit-config.yaml`** -- secret scanning is the
      primary control against committing a key or token, and the global house rule
      is "never commit secrets". This repo generates VAPID keypairs, so it handles
      key material by design.
  - Add after `check-added-large-files`:
    ```yaml
    - repo: https://github.com/gitleaks/gitleaks
      rev: v8.30.1
      hooks:
        - id: gitleaks
    ```

## High (Setup)

- [ ] **No `/metrics` endpoint and no `metrics/` package** -- every other service
      exposes Prometheus metrics: HTTP count and latency labelled by the Gin route
      template, plus `<name>_build_info` and `<name>_ready`. This service is the
      only one an operator cannot scrape.
- [ ] **No `/ready` endpoint** -- only `/live` exists, so an orchestrator has no way
      to hold traffic while the service is not yet functional.
- [ ] **No `/version` endpoint and no `version/` package** -- the deployed image
      cannot report which commit it is running.
- [x] **`.woodpecker.yml` does not pass `build_args_from_env: [CI_COMMIT_SHA]`** --
      the direct consequence of the item above: even once `version/` exists, the
      build context has no `.git`, so the commit must be injected via
      `-ldflags -X .../version.Commit=${COMMIT}`. Fix both together.
- [ ] **Test coverage is 76.3%, below the house >= 80%** -- `config`,
      `internal/allocine`, `internal/push`, `internal/schedule` and `internal/web`
      all sit under the bar.

## Medium (Maintenance)

- [ ] **No `CONTEXT.md`** -- five of six siblings carry a domain glossary. This repo
      has real domain vocabulary (releases, watchlist, notification windows) that
      currently lives only in code.
- [ ] **No `docs/adr/`** -- the non-obvious decisions here (server-rendered
      templates instead of an embedded SPA; scraping instead of an API) are exactly
      what an ADR is for, and they are currently undocumented.
- [ ] **Record why the module path stays on `github.com/bubu11e/popcorn`** --
      **decided 2026-08-02:** the fleet standard is
      `forgejo.home.mpli.fr/julien/<name>`, and this repo is the deliberate
      exception because it is the public one. Nothing to migrate here; the chore is
      writing the exception down (a line in `CLAUDE.md`, or an ADR once
      `docs/adr/` exists) so the next audit does not flag it as drift again.
- [ ] **Add `.dockerignore`** -- the build context ships `data/` and the local binary
      into the daemon.
- [ ] **Add the `check-merge-conflict` pre-commit hook** -- alongside the existing
      `pre-commit-hooks` entries:
      ```yaml
      - id: check-merge-conflict
      ```
- [ ] **Add `.editorconfig`** -- copy `crate/.editorconfig` verbatim so the fleet
      shares one indent/EOL policy: 4-space default, tabs for Go, 2 for
      YAML/JSON/TOML, and no trailing-whitespace trimming in Markdown.
- [ ] **`CLAUDE.md` does not state the TDD / >= 80% coverage rule** that three
      siblings state explicitly. Either adopt it here or drop the claim fleet-wide.
- [ ] **No `worker/`** -- fine if the scheduling model genuinely differs, but confirm
      it is a decision and not drift, and record it if so.

`LICENSE` (GPL-3.0) is already present here, so nothing to do for it.

## Low (Review, likely intentional)

- [ ] **`fmt.Printf` at `main.go:155`** -- the only non-`slog` output in the repo. It
      prints a generated VAPID keypair, including the private key, to stdout. That is
      the normal UX for a keygen subcommand, so this is probably correct; confirm the
      command is never run under a log collector that would persist the private key.
