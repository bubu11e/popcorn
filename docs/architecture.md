# Architecture

Popcorn aggregates cinema showtimes from the unofficial allocine.fr API and
serves a 7-day calendar. This document describes the Go implementation.

## Data flow

```
                +-------------------+
   time.Ticker  |    Refresher      |  recomputes the rolling window from
   (interval) ->|  (schedule pkg)   |  time.Now() every cycle
                +---------+---------+
                          | for each day x theater
                          v
                +-------------------+
                | allocine.Client   |  HTTP GET /_/showtimes/... with timeout,
                | (allocine pkg)    |  bounded retry, defensive JSON parsing
                +---------+---------+
                          | []Showtime
                          v
                +-------------------+
                | Aggregate()       |  group by movie, then theater; sort by
                | (schedule pkg)    |  wantToSee desc
                +---------+---------+
                          | [][]MovieView (indexed by day offset)
                          v
                +-------------------+
                |      Store        |  thread-safe snapshot (RWMutex); a failed
                | (schedule pkg)    |  refresh keeps the previous snapshot
                +---------+---------+
                          | read-only
                          v
                +-------------------+
   HTTP GET / ->|  web.Server (Gin) |  renders embedded html/template
   HTTP /health|                   |
                +-------------------+
```

## Packages

- `config` — loads `config.yaml`, applies `POPCORN_*` env overrides, validates.
- `internal/allocine` — API client and domain models (`Movie`, `Showtime`, `Theater`).
  `client.go` handles pagination, per-request timeout, retry with backoff on 5xx /
  transport errors, and tolerant parsing (missing `message` key, `no.showtime.error`,
  nil cast/credits, missing poster).
- `internal/schedule` — `Aggregate` builds view models; `Store` holds the current
  snapshot; `Refresher` keeps it fresh in the background.
- `internal/web` — Gin handlers for `/`, `/health`, and the service worker (`/sw.js`,
  served from root so its scope is the whole origin); French locale helpers; embedded
  templates and static assets (incl. the web app manifest and icons).
- `main.go` — wiring: load config, build the client, start the refresher goroutine and
  HTTP server, handle graceful shutdown (SIGINT/SIGTERM). The container HEALTHCHECK
  probes `/health` via wget.

## Progressive Web App

Popcorn is installable and works offline: `static/manifest.webmanifest` plus a
service worker (`static/js/sw.js`) precache the app shell and serve static assets with
stale-while-revalidate, while navigations are network-first with a cached fallback. The
manifest, worker, and generated icons live in `static/` and are embedded via `go:embed`.

## Reliability and resilience

- **No drift / staleness:** data is not fetched just once at startup. The
  `Refresher` recomputes the date window from `time.Now()` on every tick, so day 0 is
  always today and new showtimes are picked up within `refresh.interval`.
- **Resilience (no startup crash):** the server starts serving immediately and the
  refresher runs in the background. A failed theater/day fetch is logged and skipped; a
  fully failed refresh retains the last good snapshot. The process never exits because
  allocine is briefly unavailable, and `/health` is up from the start.
- **Configurability:** theaters and runtime settings live in `config.yaml` with env
  overrides — see [configuration.md](configuration.md).

## Templates

Rendered with Go `html/template`. `base.html` defines the `base` template with a
`body` block; `index.html` defines `body`. Both are embedded via `go:embed` so the
binary is self-contained.

The server ships every day's cards at once and switches between them client-side for
instant day-switching (no round-trip). The **genre filter** follows the same model:
the handler computes the distinct genre catalogue (`schedule.CollectGenres`) and passes
it as chips, while each card carries its genres as accent-proof slugs (`web.slugify`,
exposed to templates via the `genreSlugs` func). Selecting a chip filters the visible
panel in the browser and is mirrored in the `?genre=` query param, so the server stays
stateless about the active filter.

Each card also links to a **trailer**. The showtimes API exposes no trailer URL (only a
`hasTrailer` flag), so `schedule.trailerSearchURL` builds a YouTube search link from the
movie title. Foreign films — detected by `schedule.originalTitle` when the allocine
`originalTitle` differs from the French `title` — bias the query towards the VOSTFR cut
(original audio, French subtitles) and additionally show the international title under the
French one on the card.
