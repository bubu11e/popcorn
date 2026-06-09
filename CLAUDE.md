# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Popcorn is a Go web application that aggregates cinema showtimes from the
allocine.fr API. It displays movie schedules for configured theaters with a rolling
7-day calendar view. Showtimes are refreshed in the background, so the data stays fresh
and a transient allocine.fr outage never crashes the server.

## Development Commands

```bash
# Run the dev server (uses the sample config)
go run . -config config.example.yaml

# Build a binary (templates and static assets are embedded via go:embed)
go build -o popcorn .

# Tests, vet, format check
go test ./... -cover
go vet ./...
gofmt -l .

# Pre-commit hooks (hadolint + generic hooks)
pre-commit run --all-files
```

## Docker

```bash
docker build -t popcorn .
docker run -p 5000:5000 popcorn        # uses baked-in config.yaml
docker run -p 5000:5000 -v $PWD/config.yaml:/app/config.yaml popcorn
```

## Architecture

See [docs/architecture.md](docs/architecture.md) for the full picture.

**Data flow:** a background `Refresher` recomputes a rolling date window from `time.Now()`
on every tick, fetches showtimes per theater/day via `allocine.Client`, aggregates them by
movie (sorted by `wantToSee`), and atomically swaps the result into a thread-safe `Store`.
The Gin HTTP server reads snapshots from the store. A failed refresh keeps the last good
snapshot (serve-stale); the server is available immediately on startup.

**Key files:**
- `main.go` — wiring, graceful shutdown, embedded assets.
- `config/config.go` — YAML + `POPCORN_*` env config with validation.
- `internal/allocine/` — `client.go` (HTTP, pagination, timeout, retry, defensive parsing)
  and `types.go` (`Movie`, `Showtime`, `Theater`).
- `internal/schedule/` — `aggregate.go`, `store.go`, `refresher.go`.
- `internal/web/` — `handlers.go` (`/` and `/health`) and `locale.go` (French labels).

**Configuration:** theaters and runtime settings live in `config.yaml` (env-overridable).
See [docs/configuration.md](docs/configuration.md). Theaters are no longer hardcoded.

**API integration:** `Client.GetShowtimes` fetches
`allocine.fr/_/showtimes/theater-{id}/d-{date}/p-{page}/`, following pagination. The
`no.showtime.error` / `next.showtime.on` messages are treated as "no screenings".

**Frontend:** Go `html/template` in `templates/` (`base.html` + `index.html`), static
assets in `static/`. Both are embedded into the binary.
