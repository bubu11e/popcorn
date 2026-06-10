// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (c) 2024-2026 Julien Girard

// Command pushtest is a throwaway dev utility: it loads the persisted push
// subscriptions and sends one digest notification, so you can exercise the real
// server -> push-service -> browser delivery path on demand. Run it with the
// same POPCORN_VAPID_* / POPCORN_PUSH_SUBSCRIPTIONS_FILE env as the server.
package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/bubu11e/popcorn/internal/push"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	store := push.NewSubscriptionStore(os.Getenv("POPCORN_PUSH_SUBSCRIPTIONS_FILE"))
	if err := store.Load(); err != nil {
		logger.Error("load subscriptions", "error", err)
		os.Exit(1)
	}
	logger.Info("loaded subscriptions", "count", store.Len())

	notifier := push.NewNotifier(store,
		os.Getenv("POPCORN_VAPID_PUBLIC_KEY"),
		os.Getenv("POPCORN_VAPID_PRIVATE_KEY"),
		os.Getenv("POPCORN_VAPID_SUBJECT"),
		logger)

	payload := push.DigestPayload([]string{"Dune", "Wicked", "Conclave", "Anora"})
	notifier.Notify(context.Background(), payload.Marshal())
}
