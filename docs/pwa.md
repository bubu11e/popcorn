# Installing Popcorn (Progressive Web App)

Popcorn is a Progressive Web App: you can install it on your phone, tablet, or
desktop and launch it like a native app. Once installed, the app shell and
static assets are cached, so the interface still loads when you are offline (the
showtimes themselves need a connection to refresh).

Installation requires a secure context: the site must be served over HTTPS
(`localhost` is exempt for local development).

## Install instructions

### iOS / iPadOS (Safari)

1. Open Popcorn in Safari.
2. Tap the Share button (the square with an upward arrow).
3. Choose "Add to Home Screen", then "Add".

### Android (Chrome)

1. Open Popcorn in Chrome.
2. Tap the menu (three dots) in the top-right corner.
3. Choose "Install app" (or "Add to Home screen") and confirm.

Chrome may also show an "Install" banner at the bottom of the screen.

### Desktop (Chrome, Edge)

Click the install icon at the right-hand end of the address bar (a monitor with
a downward arrow), or open the browser menu and choose "Install Popcorn".

## How it works

A [web app manifest](../static/manifest.webmanifest) declares the app name,
icons, and display mode, while a service worker (`static/js/sw.js`) precaches the
app shell on install. Navigations are network-first with the cached shell as a
fallback; other static assets use stale-while-revalidate so new deploys are
picked up on the next load. Both the manifest and the worker are embedded in the
binary via `go:embed`.

See [architecture.md](architecture.md) for where this fits in the wider design.
