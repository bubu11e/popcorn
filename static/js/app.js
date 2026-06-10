// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (c) 2024-2026 Julien Girard

// Registers the service worker, which makes Popcorn installable (PWA) and lets
// the app shell work offline.
(function () {
    if (!('serviceWorker' in navigator)) { return; }

    navigator.serviceWorker.register('/sw.js', { scope: '/' }).catch(function (err) {
        console.error('service worker registration failed', err);
    });
})();
