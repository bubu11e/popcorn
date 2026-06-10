// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (c) 2024-2026 Julien Girard

// Popcorn service worker: makes the app installable and usable offline. Served
// from the site root (see internal/web/handlers.go) so its scope is the whole
// origin.

var CACHE = 'popcorn-shell-v1';

// The app shell precached on install so the UI loads offline. Versioned assets
// (?v=) are picked up at runtime via stale-while-revalidate below.
var SHELL = [
    '/',
    '/static/css/main.css',
    '/static/images/favicon.svg',
    '/static/images/icon-192.png',
    '/static/images/icon-512.png',
    '/static/font/Raleway-Black.ttf',
    '/static/manifest.webmanifest'
];

self.addEventListener('install', function (event) {
    event.waitUntil(
        caches.open(CACHE)
            .then(function (cache) { return cache.addAll(SHELL); })
            .then(function () { return self.skipWaiting(); })
    );
});

self.addEventListener('activate', function (event) {
    event.waitUntil(
        caches.keys().then(function (keys) {
            return Promise.all(keys.filter(function (k) {
                return k !== CACHE;
            }).map(function (k) { return caches.delete(k); }));
        }).then(function () { return self.clients.claim(); })
    );
});

self.addEventListener('fetch', function (event) {
    var req = event.request;
    if (req.method !== 'GET') { return; }

    var url = new URL(req.url);
    if (url.origin !== self.location.origin) { return; }
    // Never intercept the health probe.
    if (url.pathname === '/health') { return; }

    // Navigations: network-first so fresh showtimes win, cached shell as fallback.
    if (req.mode === 'navigate') {
        event.respondWith(
            fetch(req).then(function (res) {
                if (res.ok) {
                    var copy = res.clone();
                    caches.open(CACHE).then(function (cache) { cache.put('/', copy); });
                }
                return res;
            }).catch(function () { return caches.match('/'); })
        );
        return;
    }

    // Static assets: stale-while-revalidate. Serve the cached copy instantly and
    // refresh it in the background, so new deploys propagate on the next load.
    if (url.pathname.indexOf('/static/') === 0) {
        event.respondWith(
            caches.open(CACHE).then(function (cache) {
                return cache.match(req).then(function (hit) {
                    var network = fetch(req).then(function (res) {
                        if (res.ok) { cache.put(req, res.clone()); }
                        return res;
                    }).catch(function () { return hit; });
                    return hit || network;
                });
            })
        );
    }
});
