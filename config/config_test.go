// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (c) 2024-2026 Julien Girard

package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

const validYAML = `
theaters:
  - internal_id: W8560
    name: Grand Ecran
server:
  port: 8080
refresh:
  interval: 90m
  days: 5
`

func TestLoadAppliesDefaults(t *testing.T) {
	path := writeConfig(t, validYAML)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("host default = %q", cfg.Server.Host)
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("port = %d, want 8080 (from file)", cfg.Server.Port)
	}
	if cfg.Refresh.Interval != 90*time.Minute {
		t.Errorf("interval = %v", cfg.Refresh.Interval)
	}
	if cfg.Allocine.BaseURL != "https://www.allocine.fr" {
		t.Errorf("base_url default = %q", cfg.Allocine.BaseURL)
	}
}

func TestLoadEnvOverridesFile(t *testing.T) {
	path := writeConfig(t, validYAML)
	t.Setenv("POPCORN_PORT", "9999")
	t.Setenv("POPCORN_REFRESH_INTERVAL", "15m")
	t.Setenv("POPCORN_ALLOCINE_BASE_URL", "http://localhost:1234")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Server.Port != 9999 {
		t.Errorf("port = %d, want env override 9999", cfg.Server.Port)
	}
	if cfg.Refresh.Interval != 15*time.Minute {
		t.Errorf("interval = %v, want env override 15m", cfg.Refresh.Interval)
	}
	if cfg.Allocine.BaseURL != "http://localhost:1234" {
		t.Errorf("base_url = %q", cfg.Allocine.BaseURL)
	}
}

func TestLoadAllEnvOverrides(t *testing.T) {
	path := writeConfig(t, validYAML)
	t.Setenv("POPCORN_HOST", "127.0.0.1")
	t.Setenv("POPCORN_REFRESH_DAYS", "10")
	t.Setenv("POPCORN_LOG_LEVEL", "debug")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Server.Host != "127.0.0.1" {
		t.Errorf("host = %q", cfg.Server.Host)
	}
	if cfg.Refresh.Days != 10 {
		t.Errorf("days = %d, want 10", cfg.Refresh.Days)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("log_level = %q", cfg.LogLevel)
	}
}

func TestLoadInvalidEnv(t *testing.T) {
	cases := map[string]string{
		"POPCORN_PORT":             "notanumber",
		"POPCORN_REFRESH_DAYS":     "lots",
		"POPCORN_REFRESH_INTERVAL": "soon",
	}
	for key, bad := range cases {
		t.Run(key, func(t *testing.T) {
			path := writeConfig(t, validYAML)
			t.Setenv(key, bad)
			if _, err := Load(path); err == nil {
				t.Fatalf("expected error for %s=%q", key, bad)
			}
		})
	}
}

func TestValidationRangeErrors(t *testing.T) {
	cases := map[string]string{
		"bad port": "theaters:\n  - {internal_id: W1, name: X}\nserver:\n  port: 70000\n",
		"bad days": "theaters:\n  - {internal_id: W1, name: X}\nrefresh:\n  days: 99\n",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Load(writeConfig(t, body)); err == nil {
				t.Errorf("expected validation error for %q", name)
			}
		})
	}
}

func TestValidationErrors(t *testing.T) {
	cases := map[string]string{
		"no theaters":  "theaters: []\n",
		"missing id":   "theaters:\n  - name: X\n",
		"missing name": "theaters:\n  - internal_id: W1\n",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Load(writeConfig(t, body)); err == nil {
				t.Errorf("expected validation error for %q", name)
			}
		})
	}
}

func TestPushDefaultsDisabled(t *testing.T) {
	cfg, err := Load(writeConfig(t, validYAML))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Push.Enabled() {
		t.Error("push must be disabled when no VAPID keys are configured")
	}
	if cfg.Push.SubscriptionsFile != "subscriptions.json" {
		t.Errorf("subscriptions_file default = %q", cfg.Push.SubscriptionsFile)
	}
}

func TestPushEnabledViaEnv(t *testing.T) {
	path := writeConfig(t, validYAML)
	t.Setenv("POPCORN_VAPID_PUBLIC_KEY", "pub")
	t.Setenv("POPCORN_VAPID_PRIVATE_KEY", "priv")
	t.Setenv("POPCORN_VAPID_SUBJECT", "mailto:test@example.com")
	t.Setenv("POPCORN_PUSH_SUBSCRIPTIONS_FILE", "/data/subs.json")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.Push.Enabled() {
		t.Fatal("push should be enabled when both keys are set via env")
	}
	if cfg.Push.SubscriptionsFile != "/data/subs.json" {
		t.Errorf("subscriptions_file = %q, want env override", cfg.Push.SubscriptionsFile)
	}
}

func TestPushValidationErrors(t *testing.T) {
	cases := map[string]map[string]string{
		"only public key":  {"POPCORN_VAPID_PUBLIC_KEY": "pub"},
		"only private key": {"POPCORN_VAPID_PRIVATE_KEY": "priv"},
		"keys without subject": {
			"POPCORN_VAPID_PUBLIC_KEY":  "pub",
			"POPCORN_VAPID_PRIVATE_KEY": "priv",
		},
	}
	for name, env := range cases {
		t.Run(name, func(t *testing.T) {
			path := writeConfig(t, validYAML)
			for k, v := range env {
				t.Setenv(k, v)
			}
			if _, err := Load(path); err == nil {
				t.Fatalf("expected validation error for %q", name)
			}
		})
	}
}

func TestLoadMissingFile(t *testing.T) {
	if _, err := Load("/nonexistent/path/config.yaml"); err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestServerAddr(t *testing.T) {
	s := Server{Host: "127.0.0.1", Port: 5000}
	if s.Addr() != "127.0.0.1:5000" {
		t.Errorf("Addr() = %q", s.Addr())
	}
}
