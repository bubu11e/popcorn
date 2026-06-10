# Progressive Web App and push notifications

Popcorn is an installable Progressive Web App (PWA): on a phone you can "Add to
Home Screen" and launch it like a native app, and the app shell keeps working
offline. When configured, it can also send a push notification when new movies
enter the rolling window.

Push is **optional**. Without VAPID keys the app still installs and works
offline — only the notification opt-in button is hidden.

## How it works

- **Installable:** `static/manifest.webmanifest` (name, icons, `display:
  standalone`, theme colour) plus a service worker served from the site root at
  `/sw.js`. Serving the worker from the root — not `/static/` — gives it the
  whole-origin scope it needs (via the `Service-Worker-Allowed: /` header).
- **Offline:** the service worker precaches the shell on install, serves static
  assets stale-while-revalidate, and falls back to the cached page when a
  navigation fails.
- **New-movie push:** on each refresh the `Refresher` diffs the new snapshot
  against the previous one (`schedule.NewMovies`). Titles that just appeared are
  handed to the push `Notifier`, which sends a single French digest
  notification ("Nouveaux films à l'affiche…") to every stored subscription. The
  very first snapshot after startup is never announced (everything would look
  new).

Subscriptions are persisted to a JSON file (`push.subscriptions_file`), so they
survive restarts — this is the only stateful part of Popcorn.

## Requirements

- **HTTPS in production.** Service workers and the Push API only work in a
  secure context. `localhost` is exempt, so local development needs no TLS, but a
  deployed instance must sit behind HTTPS (typically a reverse proxy).
- **iOS / iPadOS:** push works only after the user has *installed* the PWA to the
  Home Screen (Safari 16.4+); it does not work in a regular browser tab.

## Setup

1. Generate a VAPID key pair:

   ```bash
   go run . -genvapid
   # POPCORN_VAPID_PUBLIC_KEY=...
   # POPCORN_VAPID_PRIVATE_KEY=...
   ```

2. Provide the keys and a contact subject. The private key is a secret, so
   prefer environment variables (e.g. a Woodpecker secret) over the YAML file:

   ```bash
   export POPCORN_VAPID_PUBLIC_KEY=...
   export POPCORN_VAPID_PRIVATE_KEY=...
   export POPCORN_VAPID_SUBJECT=mailto:you@example.com
   ```

3. (Optional) Choose where subscriptions are stored. The Docker image defaults
   to `/app/data/subscriptions.json` and declares `/app/data` as a volume:

   ```bash
   docker run -p 5000:5000 \
     -e POPCORN_VAPID_PUBLIC_KEY=... \
     -e POPCORN_VAPID_PRIVATE_KEY=... \
     -e POPCORN_VAPID_SUBJECT=mailto:you@example.com \
     -v popcorn-data:/app/data \
     popcorn
   ```

On startup the log line `push notifications enabled` confirms the keys were
accepted. Visit the site over HTTPS, click **Activer les notifications**, and
grant permission. A misconfigured pair (only one key, or keys without a subject)
aborts startup with a clear error.

## Notes and non-goals

- Notifications are global: every subscriber is told about every new movie. There
  is no per-theater or per-genre filtering yet.
- Rotating VAPID keys invalidates existing subscriptions; users must re-enable
  notifications.
