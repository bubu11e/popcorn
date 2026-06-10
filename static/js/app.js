// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (c) 2024-2026 Julien Girard

// Registers the service worker (PWA install + offline) and, when push is
// configured server-side, wires the "activer les notifications" button.
(function () {
    if (!('serviceWorker' in navigator)) { return; }

    navigator.serviceWorker.register('/sw.js', { scope: '/' }).catch(function (err) {
        console.error('service worker registration failed', err);
    });

    var btn = document.getElementById('notify-btn');
    if (!btn) { return; } // push disabled server-side: nothing to wire.

    // Push needs the PushManager + Notification APIs; leave the button hidden
    // otherwise. When supported, reveal it (it ships hidden to avoid a flash on
    // browsers that can't use it).
    if (!('PushManager' in window) || !('Notification' in window)) {
        return;
    }
    btn.hidden = false;

    // VAPID public keys are base64url; PushManager wants a Uint8Array.
    function urlBase64ToUint8Array(value) {
        var padding = '='.repeat((4 - value.length % 4) % 4);
        var base64 = (value + padding).replace(/-/g, '+').replace(/_/g, '/');
        var raw = atob(base64);
        var out = new Uint8Array(raw.length);
        for (var i = 0; i < raw.length; i++) { out[i] = raw.charCodeAt(i); }
        return out;
    }

    function setState(state) {
        if (state === 'on') {
            btn.textContent = '🔔 Notifications activées';
            btn.disabled = true;
        } else if (state === 'blocked') {
            btn.textContent = '🔕 Notifications bloquées';
            btn.disabled = true;
        } else {
            btn.textContent = '🔔 Activer les notifications';
            btn.disabled = false;
        }
    }

    // Reflect the current state on load.
    navigator.serviceWorker.ready.then(function (reg) {
        return reg.pushManager.getSubscription();
    }).then(function (sub) {
        if (Notification.permission === 'denied') { setState('blocked'); }
        else { setState(sub ? 'on' : 'off'); }
    });

    btn.addEventListener('click', function () {
        btn.disabled = true;
        fetch('/push/vapid-public-key').then(function (res) {
            if (!res.ok) { throw new Error('push disabled'); }
            return res.text();
        }).then(function (key) {
            return Notification.requestPermission().then(function (perm) {
                if (perm !== 'granted') {
                    setState(perm === 'denied' ? 'blocked' : 'off');
                    return null;
                }
                return navigator.serviceWorker.ready.then(function (reg) {
                    return reg.pushManager.subscribe({
                        userVisibleOnly: true,
                        applicationServerKey: urlBase64ToUint8Array(key)
                    });
                });
            });
        }).then(function (sub) {
            if (!sub) { return; }
            return fetch('/push/subscribe', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(sub)
            }).then(function () { setState('on'); });
        }).catch(function (err) {
            console.error('enabling notifications failed', err);
            setState('off');
        });
    });
})();
