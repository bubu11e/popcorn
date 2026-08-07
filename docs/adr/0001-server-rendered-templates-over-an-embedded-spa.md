# 1. Render on the server with html/template instead of embedding an SPA

Date: 2026-08-02

## Status

Accepted

## Context

The house pattern for a Go service with a UI is a Vue 3 + Vite + vite-plugin-pwa
`frontend/` built into an `internal/spa/dist` and embedded with `go:embed`. The
scaffold assumes it, and most of the fleet follows it.

Popcorn is one screen. It shows a day of movie cards, a strip of seven days to
switch between, and a row of genre chips. The whole window is fetched by the
background refresher and already in memory when a request arrives, so there is
nothing to load asynchronously and no state to keep in sync with a server: the
page is a pure function of a snapshot the server already holds.

The two pieces of interactivity -- switching day and filtering by genre -- run
against markup that is already in the browser. Every day of the window is shipped
in the first response precisely so that switching costs no round-trip.

## Decision

Render server-side with Go `html/template` (`templates/base.html` +
`index.html`), and ship the client behaviour as plain CSS and one ES module under
`static/`. Both trees are embedded into the binary with `go:embed`.

The PWA parts -- `manifest.webmanifest` and `static/js/sw.js`, served from the
root so the worker controls "/" -- are hand-written rather than generated.

## Consequences

- No `node_modules`, no lockfile, no `npm ci` step in CI and no Node stage in the
  Dockerfile. `go build` produces the entire artifact, and `docker build` has one
  builder stage.
- No Renovate churn from a front-end dependency tree, which on a service this
  size would be most of the dependency updates it ever sees.
- Escaping is `html/template`'s contextual auto-escaping rather than a framework's
  binding layer. Allocine synopses and titles are third-party text rendered into
  HTML, so this is the control that matters most here.
- Without content-hashed filenames the service worker cannot be cache-first for
  the app shell without risking a stale `app.js` against newer markup. Instead
  the CSS content is hashed into an `assetVer` query token at startup, so a
  deploy busts the year-long static cache without weakening it.
- Genre filtering is client-side over `data-genres` slug attributes on the cards.
  That is a hand-written `querySelectorAll` loop where a framework would give a
  computed property; at this size it is 20 lines.
- There is no offline story for the data, only for the shell. The showtimes are
  only useful when the server that refreshed them is reachable, so that is not a
  gap this decision creates.
- If Popcorn ever grows real client state -- per-user watchlists, a booking flow,
  optimistic updates -- this is the decision to revisit. Nothing in it is load
  bearing beyond the templates and one module.
