// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (c) 2024-2026 Julien Girard

// Package push stores Web Push subscriptions and delivers notifications to them.
// It is the first stateful piece of Popcorn: subscriptions are persisted to a
// JSON file so they survive restarts. Delivery is optional — without VAPID keys
// the rest of the app (including PWA install and offline) works unchanged.
package push

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	webpush "github.com/SherClockHolmes/webpush-go"
)

// Subscription is a browser Push API subscription. It mirrors the JSON shape the
// browser produces via PushSubscription.toJSON(), so it binds straight from the
// subscribe request body.
type Subscription = webpush.Subscription

// Keys are the base64url-encoded p256dh/auth values from the subscription.
type Keys = webpush.Keys

// errEmptyEndpoint is returned when a subscription has no endpoint, which would
// otherwise produce an unusable, unkeyable entry.
var errEmptyEndpoint = errors.New("push: subscription endpoint is empty")

// SubscriptionStore is a thread-safe set of subscriptions keyed by endpoint
// (which dedups re-subscriptions from the same browser), backed by a JSON file.
type SubscriptionStore struct {
	mu   sync.RWMutex
	subs map[string]Subscription

	path   string
	saveMu sync.Mutex // serializes disk writes so concurrent saves stay consistent
}

// NewSubscriptionStore returns an empty store backed by the file at path. Call
// Load to populate it from any existing file.
func NewSubscriptionStore(path string) *SubscriptionStore {
	return &SubscriptionStore{subs: make(map[string]Subscription), path: path}
}

// Load reads subscriptions from the backing file. A missing file is treated as
// an empty set, not an error (first run).
func (s *SubscriptionStore) Load() error {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}

	var list []Subscription
	if err := json.Unmarshal(data, &list); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for _, sub := range list {
		if sub.Endpoint != "" {
			s.subs[sub.Endpoint] = sub
		}
	}
	return nil
}

// Add stores a subscription (replacing any with the same endpoint) and flushes
// the set to disk.
func (s *SubscriptionStore) Add(sub Subscription) error {
	if sub.Endpoint == "" {
		return errEmptyEndpoint
	}
	s.mu.Lock()
	s.subs[sub.Endpoint] = sub
	s.mu.Unlock()
	return s.save()
}

// Remove drops the subscription with the given endpoint and flushes to disk. A
// missing endpoint is a no-op (no write).
func (s *SubscriptionStore) Remove(endpoint string) error {
	s.mu.Lock()
	_, existed := s.subs[endpoint]
	delete(s.subs, endpoint)
	s.mu.Unlock()
	if !existed {
		return nil
	}
	return s.save()
}

// All returns a snapshot of the current subscriptions.
func (s *SubscriptionStore) All() []Subscription {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Subscription, 0, len(s.subs))
	for _, sub := range s.subs {
		out = append(out, sub)
	}
	return out
}

// Len reports how many subscriptions are currently stored.
func (s *SubscriptionStore) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.subs)
}

// save writes the current set atomically: a temp file in the target directory is
// renamed over the destination, so a crash mid-write never leaves a torn file.
func (s *SubscriptionStore) save() error {
	s.saveMu.Lock()
	defer s.saveMu.Unlock()

	data, err := json.MarshalIndent(s.All(), "", "  ")
	if err != nil {
		return err
	}

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, ".subscriptions-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }() // no-op once the rename succeeds

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, s.path)
}
