// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (c) 2024-2026 Julien Girard

package push

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"

	webpush "github.com/SherClockHolmes/webpush-go"
)

// notificationTTL is how long (seconds) a push service keeps a message for a
// device that is currently offline. A day matches the freshness of the data.
const notificationTTL = 86400

// Notifier delivers Web Push messages to every stored subscription, signing them
// with the configured VAPID keys. A nil Notifier is a valid "push disabled"
// value: Enabled reports false and the web layer hides the notification UI.
type Notifier struct {
	store      *SubscriptionStore
	publicKey  string
	privateKey string
	subject    string // VAPID "sub": a mailto: or https: contact for the push service
	logger     *slog.Logger
	client     webpush.HTTPClient // injectable for tests; nil uses the default client
}

// NewNotifier wires a Notifier over the given store and VAPID credentials.
func NewNotifier(store *SubscriptionStore, publicKey, privateKey, subject string, logger *slog.Logger) *Notifier {
	if logger == nil {
		logger = slog.Default()
	}
	return &Notifier{
		store:      store,
		publicKey:  publicKey,
		privateKey: privateKey,
		subject:    subject,
		logger:     logger,
	}
}

// Enabled reports whether push delivery is configured. It is safe to call on a
// nil Notifier.
func (n *Notifier) Enabled() bool {
	return n != nil && n.publicKey != "" && n.privateKey != ""
}

// PublicKey returns the VAPID public key the browser needs to subscribe.
func (n *Notifier) PublicKey() string { return n.publicKey }

// Add stores a subscription.
func (n *Notifier) Add(sub Subscription) error { return n.store.Add(sub) }

// Remove drops a subscription by endpoint.
func (n *Notifier) Remove(endpoint string) error { return n.store.Remove(endpoint) }

// Notify sends the payload (an opaque JSON blob the service worker interprets)
// to every subscription. Subscriptions the push service reports as gone (404 or
// 410) are pruned. One failed delivery never blocks the others.
func (n *Notifier) Notify(ctx context.Context, payload []byte) {
	subs := n.store.All()
	if len(subs) == 0 {
		return
	}

	opts := &webpush.Options{
		Subscriber:      n.subject,
		VAPIDPublicKey:  n.publicKey,
		VAPIDPrivateKey: n.privateKey,
		TTL:             notificationTTL,
		HTTPClient:      n.client,
	}

	var sent, pruned int
	for i := range subs {
		sub := subs[i]
		resp, err := webpush.SendNotificationWithContext(ctx, payload, &sub, opts)
		if err != nil {
			n.logger.Warn("push send failed", "endpoint", endpointHost(sub.Endpoint), "error", err)
			continue
		}
		_ = resp.Body.Close()

		switch {
		case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone:
			if err := n.store.Remove(sub.Endpoint); err == nil {
				pruned++
			}
		case resp.StatusCode >= 400:
			n.logger.Warn("push endpoint rejected", "endpoint", endpointHost(sub.Endpoint), "status", resp.StatusCode)
		default:
			sent++
		}
	}

	n.logger.Info("push notifications delivered", "sent", sent, "pruned", pruned, "total", len(subs))
}

// endpointHost returns just the host of a push endpoint, so logs never leak the
// per-device secret token carried in the endpoint path/query.
func endpointHost(endpoint string) string {
	u, err := url.Parse(endpoint)
	if err != nil {
		return "invalid"
	}
	return u.Host
}
