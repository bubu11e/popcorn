// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (c) 2024-2026 Julien Girard

package push

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
)

// testSubscription builds a subscription with a valid P-256 public key and a
// 16-byte auth secret, so webpush can actually encrypt a payload for it. The
// endpoint points at the caller-supplied URL (a test push service).
func testSubscription(t *testing.T, endpoint string) Subscription {
	t.Helper()
	priv, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	auth := make([]byte, 16)
	if _, err := rand.Read(auth); err != nil {
		t.Fatalf("read auth: %v", err)
	}
	return Subscription{
		Endpoint: endpoint,
		Keys: Keys{
			P256dh: base64.RawURLEncoding.EncodeToString(priv.PublicKey().Bytes()),
			Auth:   base64.RawURLEncoding.EncodeToString(auth),
		},
	}
}

func newTestNotifier(t *testing.T, store *SubscriptionStore) *Notifier {
	t.Helper()
	priv, pub, err := GenerateVAPIDKeys()
	if err != nil {
		t.Fatalf("vapid: %v", err)
	}
	return NewNotifier(store, pub, priv, "mailto:test@example.com", nil)
}

func TestNotifierDeliversToEverySubscription(t *testing.T) {
	var mu sync.Mutex
	hits := map[string]int{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hits[r.URL.Path]++
		mu.Unlock()
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	store := NewSubscriptionStore(filepath.Join(t.TempDir(), "subs.json"))
	_ = store.Add(testSubscription(t, srv.URL+"/a"))
	_ = store.Add(testSubscription(t, srv.URL+"/b"))

	newTestNotifier(t, store).Notify(context.Background(), []byte(`{"title":"hi"}`))

	mu.Lock()
	defer mu.Unlock()
	if hits["/a"] != 1 || hits["/b"] != 1 {
		t.Fatalf("expected one delivery per subscription, got %v", hits)
	}
}

func TestNotifierPrunesGoneSubscriptions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/gone" {
			w.WriteHeader(http.StatusGone) // 410: subscription expired
			return
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	store := NewSubscriptionStore(filepath.Join(t.TempDir(), "subs.json"))
	_ = store.Add(testSubscription(t, srv.URL+"/live"))
	_ = store.Add(testSubscription(t, srv.URL+"/gone"))

	newTestNotifier(t, store).Notify(context.Background(), []byte(`{"title":"hi"}`))

	if store.Len() != 1 {
		t.Fatalf("Len = %d, want 1 after pruning the gone subscription", store.Len())
	}
	if got := store.All()[0].Endpoint; got != srv.URL+"/live" {
		t.Fatalf("surviving endpoint = %q, want the live one", got)
	}
}

func TestNotifierKeepsSubscriptionOnTransientError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests) // 429: not gone, must not prune
	}))
	defer srv.Close()

	store := NewSubscriptionStore(filepath.Join(t.TempDir(), "subs.json"))
	_ = store.Add(testSubscription(t, srv.URL+"/a"))

	newTestNotifier(t, store).Notify(context.Background(), []byte(`{"title":"hi"}`))

	if store.Len() != 1 {
		t.Fatalf("Len = %d, want 1: a 429 is transient, not a reason to prune", store.Len())
	}
}

func TestNotifierAccessors(t *testing.T) {
	store := NewSubscriptionStore(filepath.Join(t.TempDir(), "subs.json"))
	n := NewNotifier(store, "pub-key", "priv-key", "mailto:test@example.com", nil)

	if n.PublicKey() != "pub-key" {
		t.Fatalf("PublicKey = %q, want pub-key", n.PublicKey())
	}
	if err := n.Add(Subscription{Endpoint: "https://push.example/a"}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if store.Len() != 1 {
		t.Fatal("Add should reach the store")
	}
	if err := n.Remove("https://push.example/a"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if store.Len() != 0 {
		t.Fatal("Remove should reach the store")
	}
}

func TestNotifierWithoutSubscriptionsSendsNothing(t *testing.T) {
	var called atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called.Store(true)
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	store := NewSubscriptionStore(filepath.Join(t.TempDir(), "subs.json"))
	newTestNotifier(t, store).Notify(context.Background(), []byte(`{"title":"hi"}`))

	if called.Load() {
		t.Fatal("no subscriptions means no request should leave the process")
	}
}

func TestNotifierSurvivesAnUnreachableEndpoint(t *testing.T) {
	var reached atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		reached.Add(1)
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	store := NewSubscriptionStore(filepath.Join(t.TempDir(), "subs.json"))
	_ = store.Add(testSubscription(t, "http://127.0.0.1:1/dead")) // closed port
	_ = store.Add(testSubscription(t, srv.URL+"/live"))

	newTestNotifier(t, store).Notify(context.Background(), []byte(`{"title":"hi"}`))

	if reached.Load() != 1 {
		t.Fatalf("live endpoint hit %d times, want 1: one dead peer must not block the rest", reached.Load())
	}
	if store.Len() != 2 {
		t.Fatalf("Len = %d, want 2: a transport failure is not proof the subscription is gone", store.Len())
	}
}

func TestEndpointHostRedactsThePerDeviceToken(t *testing.T) {
	// Push endpoints carry a device secret in the path; logs must keep only the
	// host.
	const endpoint = "https://fcm.googleapis.com/fcm/send/SECRET-DEVICE-TOKEN?k=v"
	if got := endpointHost(endpoint); got != "fcm.googleapis.com" {
		t.Errorf("endpointHost = %q, want the bare host", got)
	}
	if got := endpointHost("://not a url"); got != "invalid" {
		t.Errorf("endpointHost of an unparseable endpoint = %q, want %q", got, "invalid")
	}
}

func TestNotifierEnabled(t *testing.T) {
	var nilNotifier *Notifier
	if nilNotifier.Enabled() {
		t.Fatal("nil notifier must report disabled")
	}
	if NewNotifier(nil, "", "", "", nil).Enabled() {
		t.Fatal("notifier without keys must report disabled")
	}
}
