# 🍿 Popcorn

Popcorn is a small Go web app that aggregates cinema showtimes from the
allociné.fr API and serves them as a rolling 7-day calendar for the theaters you
configure. Showtimes are refreshed in the background, so the data stays fresh and
a transient allociné outage never takes the site down.

The interface is in French; the codebase and documentation are in English.

## Features

- Rolling 7-day view, recomputed from the current date on every refresh.
- Per-movie cards: poster, director, runtime, genres, synopsis, and a
  trailer search link.
- Client-side **day switching** and **genre filtering**, both reflected in the
  URL (`?delta=`, `?genre=`) so any view is shareable.
- Background refresh with serve-stale: a failed fetch keeps the last good
  snapshot, and the server is available immediately on startup.
- Single self-contained binary — templates and static assets are embedded via
  `go:embed`.

## Quickstart

```bash
# Run the dev server against the sample config.
go run . -config config.example.yaml
# then open http://localhost:5000

# Build a standalone binary (assets are embedded).
go build -o popcorn .
./popcorn -config config.example.yaml
```

### Docker

```bash
docker build -t popcorn .

# Use the config baked into the image:
docker run -p 5000:5000 popcorn

# Or mount your own:
docker run -p 5000:5000 -v "$PWD/config.yaml:/app/config.yaml" popcorn
```

## Configuration

Theaters and runtime settings live in `config.yaml`; copy `config.example.yaml`
to get started. Every scalar can be overridden at runtime with a `POPCORN_*`
environment variable. See [docs/configuration.md](docs/configuration.md) for the
full reference.

```yaml
theaters:
  - internal_id: C0159        # allociné.fr theater id
    name: UGC Ciné Cité Les Halles
server:
  port: 5000                  # POPCORN_PORT
refresh:
  interval: 3h                # POPCORN_REFRESH_INTERVAL
  days: 7                     # POPCORN_REFRESH_DAYS
```

## Development

```bash
go test ./... -cover   # tests with coverage
go vet ./...           # static checks
gofmt -l .             # formatting (should print nothing)
pre-commit run --all-files
```

See [docs/architecture.md](docs/architecture.md) for the data flow and package
layout.

## License

Released under the [GNU General Public License v3.0 or later](LICENSE).

Popcorn began as a Go reimplementation inspired by
[grainParisArt](https://github.com/solene-drnx/grainParisArt-Public) by
[Solène](https://github.com/solene-drnx) and
[Mathias](https://github.com/MathiasDPX). The current codebase is an independent
rewrite.

The bundled fonts (Raleway and Montserrat) are third-party works redistributed
under the SIL Open Font License 1.1 — see
[static/font/LICENSE.txt](static/font/LICENSE.txt). They are not covered by the
GPLv3 license above.

Showtime data comes from allociné.fr via an unofficial API and belongs to its
respective owners; this project is unaffiliated with allociné.
