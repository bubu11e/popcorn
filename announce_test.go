// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (c) 2024-2026 Julien Girard

package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/bubu11e/popcorn/internal/push"
	"github.com/bubu11e/popcorn/internal/schedule"
)

func TestPushAnnouncerSendsOneDigestPerBatch(t *testing.T) {
	var deliveries atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		deliveries.Add(1)
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	cfg := vapidConfig(t, filepath.Join(t.TempDir(), "subs.json"))
	store := push.NewSubscriptionStore(cfg.SubscriptionsFile)
	if err := store.Add(testSubscription(t, srv.URL+"/device")); err != nil {
		t.Fatalf("add subscription: %v", err)
	}
	notifier := push.NewNotifier(store, cfg.PublicKey, cfg.PrivateKey, cfg.Subject, discardLogger())

	newPushAnnouncer(notifier, discardLogger()).AnnounceNewMovies(context.Background(),
		[]schedule.MovieView{{Title: "Film A"}, {Title: "Film B"}, {Title: "Film C"}})

	// Three new movies are one piece of news, not three buzzes on the phone.
	if got := deliveries.Load(); got != 1 {
		t.Fatalf("deliveries = %d, want 1 digest for the whole batch", got)
	}
}

func TestPushAnnouncerReachesEverySubscribedDevice(t *testing.T) {
	var deliveries atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		deliveries.Add(1)
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	cfg := vapidConfig(t, filepath.Join(t.TempDir(), "subs.json"))
	store := push.NewSubscriptionStore(cfg.SubscriptionsFile)
	for _, path := range []string{"/phone", "/laptop"} {
		if err := store.Add(testSubscription(t, srv.URL+path)); err != nil {
			t.Fatalf("add subscription: %v", err)
		}
	}
	notifier := push.NewNotifier(store, cfg.PublicKey, cfg.PrivateKey, cfg.Subject, discardLogger())

	newPushAnnouncer(notifier, discardLogger()).AnnounceNewMovies(context.Background(),
		[]schedule.MovieView{{Title: "Film A"}})

	if got := deliveries.Load(); got != 2 {
		t.Fatalf("deliveries = %d, want one per subscribed device", got)
	}
}
