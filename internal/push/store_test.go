// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (c) 2024-2026 Julien Girard

package push

import (
	"path/filepath"
	"testing"
)

func TestSubscriptionStoreAddDedupsByEndpoint(t *testing.T) {
	store := NewSubscriptionStore(filepath.Join(t.TempDir(), "subs.json"))

	if err := store.Add(Subscription{Endpoint: "https://push.example/a", Keys: Keys{Auth: "x", P256dh: "y"}}); err != nil {
		t.Fatalf("add: %v", err)
	}
	// Same endpoint, different keys: must replace, not duplicate.
	if err := store.Add(Subscription{Endpoint: "https://push.example/a", Keys: Keys{Auth: "z", P256dh: "w"}}); err != nil {
		t.Fatalf("add again: %v", err)
	}

	if got := store.Len(); got != 1 {
		t.Fatalf("Len = %d, want 1 after re-subscribe", got)
	}
	if got := store.All()[0].Keys.Auth; got != "z" {
		t.Fatalf("Auth = %q, want the latest value %q", got, "z")
	}
}

func TestSubscriptionStoreAddRejectsEmptyEndpoint(t *testing.T) {
	store := NewSubscriptionStore(filepath.Join(t.TempDir(), "subs.json"))
	if err := store.Add(Subscription{}); err == nil {
		t.Fatal("expected error for empty endpoint, got nil")
	}
	if store.Len() != 0 {
		t.Fatal("empty subscription must not be stored")
	}
}

func TestSubscriptionStoreRemove(t *testing.T) {
	store := NewSubscriptionStore(filepath.Join(t.TempDir(), "subs.json"))
	_ = store.Add(Subscription{Endpoint: "https://push.example/a"})

	if err := store.Remove("https://push.example/a"); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if store.Len() != 0 {
		t.Fatalf("Len = %d, want 0 after remove", store.Len())
	}
	// Removing an unknown endpoint is a no-op, not an error.
	if err := store.Remove("https://push.example/missing"); err != nil {
		t.Fatalf("remove missing: %v", err)
	}
}

func TestSubscriptionStorePersistsAndReloads(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "subs.json")

	first := NewSubscriptionStore(path)
	_ = first.Add(Subscription{Endpoint: "https://push.example/a", Keys: Keys{Auth: "a1", P256dh: "p1"}})
	_ = first.Add(Subscription{Endpoint: "https://push.example/b", Keys: Keys{Auth: "a2", P256dh: "p2"}})

	// A fresh store backed by the same file must observe both subscriptions,
	// including the directory the first store had to create.
	second := NewSubscriptionStore(path)
	if err := second.Load(); err != nil {
		t.Fatalf("load: %v", err)
	}
	if got := second.Len(); got != 2 {
		t.Fatalf("Len after reload = %d, want 2", got)
	}
}

func TestSubscriptionStoreLoadMissingFileIsEmpty(t *testing.T) {
	store := NewSubscriptionStore(filepath.Join(t.TempDir(), "does-not-exist.json"))
	if err := store.Load(); err != nil {
		t.Fatalf("load missing file should not error: %v", err)
	}
	if store.Len() != 0 {
		t.Fatal("missing file should yield an empty store")
	}
}
