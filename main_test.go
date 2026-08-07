// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (c) 2024-2026 Julien Girard

package main

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/bubu11e/popcorn/config"
	"github.com/bubu11e/popcorn/internal/push"
)

// testSubscription builds a subscription webpush can actually encrypt for: a
// real P-256 public key and a 16-byte auth secret, pointed at a test server.
func testSubscription(t *testing.T, endpoint string) push.Subscription {
	t.Helper()
	priv, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	auth := make([]byte, 16)
	if _, err := rand.Read(auth); err != nil {
		t.Fatalf("read auth: %v", err)
	}
	return push.Subscription{
		Endpoint: endpoint,
		Keys: push.Keys{
			P256dh: base64.RawURLEncoding.EncodeToString(priv.PublicKey().Bytes()),
			Auth:   base64.RawURLEncoding.EncodeToString(auth),
		},
	}
}

func vapidConfig(t *testing.T, subscriptionsFile string) config.Push {
	t.Helper()
	priv, pub, err := push.GenerateVAPIDKeys()
	if err != nil {
		t.Fatalf("vapid: %v", err)
	}
	return config.Push{
		Subject:           "mailto:test@example.com",
		PublicKey:         pub,
		PrivateKey:        priv,
		SubscriptionsFile: subscriptionsFile,
	}
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}

func TestSetupPushDisabledIsNotAnError(t *testing.T) {
	// No VAPID keys is a supported configuration: the PWA still installs and
	// works offline, it just never notifies.
	notifier, err := setupPush(config.Push{}, discardLogger())
	if err != nil {
		t.Fatalf("disabled push must not error: %v", err)
	}
	if notifier != nil {
		t.Fatal("disabled push must yield a nil notifier, or main would advertise push")
	}
}

func TestSetupPushDeliversToSubscriptionsThatSurvivedARestart(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	// Persist a subscription the way a previous run would have.
	path := filepath.Join(t.TempDir(), "subs.json")
	seed := push.NewSubscriptionStore(path)
	if err := seed.Add(testSubscription(t, srv.URL+"/device")); err != nil {
		t.Fatalf("seed: %v", err)
	}

	notifier, err := setupPush(vapidConfig(t, path), discardLogger())
	if err != nil {
		t.Fatalf("setupPush: %v", err)
	}
	if notifier == nil || !notifier.Enabled() {
		t.Fatal("configured VAPID keys must produce an enabled notifier")
	}

	notifier.Notify(context.Background(), []byte(`{"title":"hi"}`))
	if hits.Load() != 1 {
		t.Fatalf("deliveries = %d, want 1: the persisted subscription was not loaded", hits.Load())
	}
}

func TestSetupPushFailsOnACorruptSubscriptionsFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "subs.json")
	if err := os.WriteFile(path, []byte(`[{"endpoint":`), 0o600); err != nil {
		t.Fatal(err)
	}

	// Starting anyway would silently drop every subscriber, so this is fatal.
	if _, err := setupPush(vapidConfig(t, path), discardLogger()); err == nil {
		t.Fatal("expected an error for a corrupt subscriptions file")
	}
}

func TestPrintVAPIDKeysEmitsAUsableEnvPair(t *testing.T) {
	out := captureStdout(t, func() {
		if err := printVAPIDKeys(); err != nil {
			t.Fatalf("printVAPIDKeys: %v", err)
		}
	})

	vars := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		name, value, ok := strings.Cut(line, "=")
		if !ok {
			t.Fatalf("line %q is not a KEY=VALUE assignment", line)
		}
		vars[name] = value
	}

	pub, hasPub := vars["POPCORN_VAPID_PUBLIC_KEY"]
	priv, hasPriv := vars["POPCORN_VAPID_PRIVATE_KEY"]
	if !hasPub || !hasPriv {
		t.Fatalf("output must name both env vars, got %v", vars)
	}
	if pub == "" || priv == "" || pub == priv {
		t.Fatalf("keys look wrong: public=%q private=%q", pub, priv)
	}
	// The output is meant to be pasted straight into a secrets store, so the
	// pair it prints must be one the config layer accepts.
	if !(config.Push{PublicKey: pub, PrivateKey: priv}).Enabled() {
		t.Error("the printed pair should enable push when configured")
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	fn()
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func TestEnvOr(t *testing.T) {
	if got := envOr("POPCORN_TEST_UNSET_VAR", "fallback"); got != "fallback" {
		t.Errorf("unset = %q, want fallback", got)
	}

	t.Setenv("POPCORN_TEST_VAR", "from-env")
	if got := envOr("POPCORN_TEST_VAR", "fallback"); got != "from-env" {
		t.Errorf("set = %q, want from-env", got)
	}

	// An explicit empty value is a choice, not an absence.
	t.Setenv("POPCORN_TEST_VAR", "")
	if got := envOr("POPCORN_TEST_VAR", "fallback"); got != "" {
		t.Errorf("empty = %q, want the empty value to win over the fallback", got)
	}
}

func TestNewLoggerLevels(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		level       string
		lowestShown slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"WARN", slog.LevelWarn}, // case-insensitive
		{"error", slog.LevelError},
		{"nonsense", slog.LevelInfo}, // unknown values fall back to info
		{"", slog.LevelInfo},
	} {
		t.Run(tc.level, func(t *testing.T) {
			logger := newLogger(tc.level)
			if !logger.Enabled(ctx, tc.lowestShown) {
				t.Errorf("%v should be enabled at level %q", tc.lowestShown, tc.level)
			}
			if tc.lowestShown > slog.LevelDebug && logger.Enabled(ctx, tc.lowestShown-4) {
				t.Errorf("%v should be filtered out at level %q", tc.lowestShown-4, tc.level)
			}
		})
	}
}
