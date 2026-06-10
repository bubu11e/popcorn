// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (c) 2024-2026 Julien Girard

package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bubu11e/popcorn/internal/push"
	"github.com/bubu11e/popcorn/internal/schedule"
)

// fakePush is a minimal in-memory PushService for handler tests.
type fakePush struct {
	enabled   bool
	publicKey string
	added     []push.Subscription
	removed   []string
}

func (f *fakePush) Enabled() bool     { return f.enabled }
func (f *fakePush) PublicKey() string { return f.publicKey }
func (f *fakePush) Add(sub push.Subscription) error {
	f.added = append(f.added, sub)
	return nil
}
func (f *fakePush) Remove(endpoint string) error {
	f.removed = append(f.removed, endpoint)
	return nil
}

func doJSON(srv *Server, method, target, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	srv.Handler().ServeHTTP(rec, req)
	return rec
}

func TestServiceWorkerServedFromRootWithScopeHeader(t *testing.T) {
	srv := newTestServer(t, schedule.NewStore(), 7)
	rec := doGet(srv, "/sw.js")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if got := rec.Header().Get("Service-Worker-Allowed"); got != "/" {
		t.Errorf("Service-Worker-Allowed = %q, want / (root scope)", got)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/javascript") {
		t.Errorf("Content-Type = %q, want text/javascript", ct)
	}
}

func TestVapidPublicKeyDisabledReturns404(t *testing.T) {
	srv := newTestServerWithPush(t, schedule.NewStore(), 7, &fakePush{enabled: false})
	if rec := doGet(srv, "/push/vapid-public-key"); rec.Code != http.StatusNotFound {
		t.Errorf("disabled push: status = %d, want 404", rec.Code)
	}
}

func TestVapidPublicKeyEnabled(t *testing.T) {
	srv := newTestServerWithPush(t, schedule.NewStore(), 7, &fakePush{enabled: true, publicKey: "PUBKEY"})
	rec := doGet(srv, "/push/vapid-public-key")
	if rec.Code != http.StatusOK || rec.Body.String() != "PUBKEY" {
		t.Errorf("got %d %q, want 200 PUBKEY", rec.Code, rec.Body.String())
	}
}

func TestSubscribeStoresSubscription(t *testing.T) {
	fp := &fakePush{enabled: true}
	srv := newTestServerWithPush(t, schedule.NewStore(), 7, fp)

	body := `{"endpoint":"https://push.example/abc","keys":{"p256dh":"k","auth":"a"}}`
	rec := doJSON(srv, http.MethodPost, "/push/subscribe", body)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", rec.Code)
	}
	if len(fp.added) != 1 || fp.added[0].Endpoint != "https://push.example/abc" {
		t.Errorf("subscription not stored: %+v", fp.added)
	}
}

func TestSubscribeRejectsInvalidBody(t *testing.T) {
	fp := &fakePush{enabled: true}
	srv := newTestServerWithPush(t, schedule.NewStore(), 7, fp)

	for _, body := range []string{`not json`, `{"keys":{}}`} { // malformed, and missing endpoint
		rec := doJSON(srv, http.MethodPost, "/push/subscribe", body)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("body %q: status = %d, want 400", body, rec.Code)
		}
	}
	if len(fp.added) != 0 {
		t.Errorf("invalid bodies must not be stored: %+v", fp.added)
	}
}

func TestUnsubscribeRemovesSubscription(t *testing.T) {
	fp := &fakePush{enabled: true}
	srv := newTestServerWithPush(t, schedule.NewStore(), 7, fp)

	rec := doJSON(srv, http.MethodPost, "/push/unsubscribe", `{"endpoint":"https://push.example/abc"}`)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if len(fp.removed) != 1 || fp.removed[0] != "https://push.example/abc" {
		t.Errorf("endpoint not removed: %+v", fp.removed)
	}
}

func TestSubscribeDisabledReturns404(t *testing.T) {
	srv := newTestServerWithPush(t, schedule.NewStore(), 7, &fakePush{enabled: false})
	rec := doJSON(srv, http.MethodPost, "/push/subscribe", `{"endpoint":"x"}`)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 when push disabled", rec.Code)
	}
}
