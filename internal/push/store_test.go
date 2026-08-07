// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (c) 2024-2026 Julien Girard

package push

import (
	"os"
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

func TestSubscriptionStoreLoadSkipsEntriesWithoutAnEndpoint(t *testing.T) {
	path := filepath.Join(t.TempDir(), "subs.json")
	body := `[{"endpoint":""},{"endpoint":"https://push.example/a","keys":{"auth":"a","p256dh":"p"}}]`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	store := NewSubscriptionStore(path)
	if err := store.Load(); err != nil {
		t.Fatalf("load: %v", err)
	}
	// An endpoint-less entry has no key and could never be delivered to.
	if store.Len() != 1 {
		t.Fatalf("Len = %d, want 1: the endpoint-less entry must be dropped", store.Len())
	}
}

func TestSubscriptionStoreLoadRejectsCorruptFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "subs.json")
	if err := os.WriteFile(path, []byte(`[{"endpoint":`), 0o600); err != nil {
		t.Fatal(err)
	}
	// Starting with a silently empty set would drop every subscriber without
	// anyone noticing, so a corrupt file must fail the load.
	if err := NewSubscriptionStore(path).Load(); err == nil {
		t.Fatal("expected an error for a corrupt subscriptions file")
	}
}

func TestSubscriptionStoreLoadReportsUnreadablePath(t *testing.T) {
	// A directory where the file should be: not "first run", a misconfiguration.
	dir := t.TempDir()
	if err := NewSubscriptionStore(dir).Load(); err == nil {
		t.Fatal("expected an error when the subscriptions path is a directory")
	}
}

func TestSubscriptionStoreAddReportsUnwritablePath(t *testing.T) {
	// The parent is a regular file, so the store cannot create its directory.
	base := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(base, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	store := NewSubscriptionStore(filepath.Join(base, "subs.json"))
	if err := store.Add(Subscription{Endpoint: "https://push.example/a"}); err == nil {
		t.Fatal("expected an error when the subscriptions file cannot be written")
	}
}

func TestSubscriptionStoreAddReportsReadOnlyDirectory(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	dir := filepath.Join(t.TempDir(), "readonly")
	if err := os.Mkdir(dir, 0o500); err != nil {
		t.Fatal(err)
	}

	store := NewSubscriptionStore(filepath.Join(dir, "subs.json"))
	if err := store.Add(Subscription{Endpoint: "https://push.example/a"}); err == nil {
		t.Fatal("expected an error when the target directory is not writable")
	}
}
