// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (c) 2024-2026 Julien Girard

// Package config loads application configuration from a YAML file with
// optional environment-variable overrides.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is the fully resolved application configuration.
type Config struct {
	Theaters []Theater `yaml:"theaters"`
	Server   Server    `yaml:"server"`
	Refresh  Refresh   `yaml:"refresh"`
	Allocine Allocine  `yaml:"allocine"`
	Push     Push      `yaml:"push"`
	LogLevel string    `yaml:"log_level"`
}

// Theater is a configured cinema. Name is the label shown in the UI.
type Theater struct {
	InternalID string `yaml:"internal_id"`
	Name       string `yaml:"name"`
}

// Server holds HTTP listener settings.
type Server struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

// Refresh controls the background showtime refresh.
type Refresh struct {
	Interval time.Duration `yaml:"interval"`
	Days     int           `yaml:"days"`
}

// Allocine controls the upstream API client.
type Allocine struct {
	BaseURL    string        `yaml:"base_url"`
	Timeout    time.Duration `yaml:"timeout"`
	MaxRetries int           `yaml:"max_retries"`
}

// Push controls Web Push notifications for newly-released movies. Notifications
// are sent only when both VAPID keys are set (see Enabled); otherwise the app
// still installs as a PWA and works offline, just without notifications. Keys
// are secrets and are normally supplied via POPCORN_VAPID_* env, not the file.
type Push struct {
	Subject           string `yaml:"subject"`            // VAPID contact, e.g. mailto:you@example.com
	PublicKey         string `yaml:"public_key"`         // base64url VAPID public key (shared with browsers)
	PrivateKey        string `yaml:"private_key"`        // base64url VAPID private key (secret)
	SubscriptionsFile string `yaml:"subscriptions_file"` // where subscriptions are persisted
}

// Enabled reports whether push notifications are configured. Both keys are
// required to sign and address push requests.
func (p Push) Enabled() bool {
	return p.PublicKey != "" && p.PrivateKey != ""
}

// Addr returns the host:port the server should listen on.
func (s Server) Addr() string {
	return fmt.Sprintf("%s:%d", s.Host, s.Port)
}

// defaults returns a Config pre-populated with sensible defaults. YAML values
// override these, and environment variables override YAML.
func defaults() Config {
	return Config{
		Server:   Server{Host: "0.0.0.0", Port: 5000},
		Refresh:  Refresh{Interval: 3 * time.Hour, Days: 7},
		Allocine: Allocine{BaseURL: "https://www.allocine.fr", Timeout: 10 * time.Second, MaxRetries: 3},
		Push:     Push{SubscriptionsFile: "subscriptions.json"},
		LogLevel: "info",
	}
}

// Load reads the YAML file at path (when non-empty), applies environment
// overrides, and validates the result.
func Load(path string) (Config, error) {
	cfg := defaults()

	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return Config{}, fmt.Errorf("read config %q: %w", path, err)
		}
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return Config{}, fmt.Errorf("parse config %q: %w", path, err)
		}
	}

	if err := cfg.applyEnv(); err != nil {
		return Config{}, err
	}
	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// applyEnv overrides scalar settings from POPCORN_* environment
// variables. Theaters are intentionally file-only (a list is awkward in env).
func (c *Config) applyEnv() error {
	if v, ok := os.LookupEnv("POPCORN_HOST"); ok {
		c.Server.Host = v
	}
	if v, ok := os.LookupEnv("POPCORN_PORT"); ok {
		port, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("POPCORN_PORT %q: %w", v, err)
		}
		c.Server.Port = port
	}
	if v, ok := os.LookupEnv("POPCORN_REFRESH_INTERVAL"); ok {
		d, err := time.ParseDuration(v)
		if err != nil {
			return fmt.Errorf("POPCORN_REFRESH_INTERVAL %q: %w", v, err)
		}
		c.Refresh.Interval = d
	}
	if v, ok := os.LookupEnv("POPCORN_REFRESH_DAYS"); ok {
		days, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("POPCORN_REFRESH_DAYS %q: %w", v, err)
		}
		c.Refresh.Days = days
	}
	if v, ok := os.LookupEnv("POPCORN_ALLOCINE_BASE_URL"); ok {
		c.Allocine.BaseURL = v
	}
	if v, ok := os.LookupEnv("POPCORN_LOG_LEVEL"); ok {
		c.LogLevel = v
	}
	// VAPID keys are secrets, so the env path is the expected source in
	// production (e.g. Woodpecker secrets); the file fields are mostly for dev.
	if v, ok := os.LookupEnv("POPCORN_VAPID_SUBJECT"); ok {
		c.Push.Subject = v
	}
	if v, ok := os.LookupEnv("POPCORN_VAPID_PUBLIC_KEY"); ok {
		c.Push.PublicKey = v
	}
	if v, ok := os.LookupEnv("POPCORN_VAPID_PRIVATE_KEY"); ok {
		c.Push.PrivateKey = v
	}
	if v, ok := os.LookupEnv("POPCORN_PUSH_SUBSCRIPTIONS_FILE"); ok {
		c.Push.SubscriptionsFile = v
	}
	return nil
}

func (c *Config) validate() error {
	if len(c.Theaters) == 0 {
		return fmt.Errorf("at least one theater must be configured")
	}
	for i, t := range c.Theaters {
		if t.InternalID == "" {
			return fmt.Errorf("theaters[%d]: internal_id is required", i)
		}
		if t.Name == "" {
			return fmt.Errorf("theaters[%d] (%s): name is required", i, t.InternalID)
		}
	}
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("server.port %d out of range", c.Server.Port)
	}
	if c.Refresh.Interval <= 0 {
		return fmt.Errorf("refresh.interval must be positive")
	}
	if c.Refresh.Days < 1 || c.Refresh.Days > 31 {
		return fmt.Errorf("refresh.days %d out of range (1-31)", c.Refresh.Days)
	}
	if c.Allocine.BaseURL == "" {
		return fmt.Errorf("allocine.base_url is required")
	}
	if err := c.Push.validate(); err != nil {
		return err
	}
	return nil
}

// validate guards against a half-configured push setup, which would silently
// disable notifications or fail to sign requests.
func (p Push) validate() error {
	if (p.PublicKey == "") != (p.PrivateKey == "") {
		return fmt.Errorf("push: public_key and private_key must both be set or both be empty")
	}
	if p.Enabled() {
		if p.Subject == "" {
			return fmt.Errorf("push: subject is required when VAPID keys are set")
		}
		if p.SubscriptionsFile == "" {
			return fmt.Errorf("push: subscriptions_file is required when push is enabled")
		}
	}
	return nil
}
