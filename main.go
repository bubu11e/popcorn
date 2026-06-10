// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (c) 2024-2026 Julien Girard

// Command popcorn serves an aggregated cinema-showtimes calendar.
package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/bubu11e/popcorn/config"
	"github.com/bubu11e/popcorn/internal/allocine"
	"github.com/bubu11e/popcorn/internal/push"
	"github.com/bubu11e/popcorn/internal/schedule"
	"github.com/bubu11e/popcorn/internal/web"
)

//go:embed templates/*.html
var templatesEmbed embed.FS

//go:embed static
var staticEmbed embed.FS

func main() {
	configPath := flag.String("config", envOr("POPCORN_CONFIG", "config.yaml"),
		"path to the YAML config file")
	genVAPID := flag.Bool("genvapid", false,
		"generate a VAPID key pair (for push notifications) and exit")
	flag.Parse()

	if *genVAPID {
		if err := printVAPIDKeys(); err != nil {
			slog.Error("generate VAPID keys", "error", err)
			os.Exit(1)
		}
		return
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("load config", "error", err)
		os.Exit(1)
	}

	logger := newLogger(cfg.LogLevel)
	slog.SetDefault(logger)

	templatesFS, err := fs.Sub(templatesEmbed, "templates")
	if err != nil {
		logger.Error("mount templates", "error", err)
		os.Exit(1)
	}
	staticFS, err := fs.Sub(staticEmbed, "static")
	if err != nil {
		logger.Error("mount static", "error", err)
		os.Exit(1)
	}

	theaters := make([]allocine.Theater, len(cfg.Theaters))
	for i, t := range cfg.Theaters {
		theaters[i] = allocine.Theater{ID: t.InternalID, Name: t.Name}
	}

	client := allocine.NewClient(cfg.Allocine.BaseURL, cfg.Allocine.Timeout, cfg.Allocine.MaxRetries, logger)
	store := schedule.NewStore()
	refresher := schedule.NewRefresher(client, theaters, cfg.Refresh.Days, cfg.Refresh.Interval, store, logger)

	// Push is optional: without VAPID keys the app still installs as a PWA and
	// works offline, just without new-movie notifications.
	notifier, err := setupPush(cfg.Push, logger)
	if err != nil {
		logger.Error("set up push", "error", err)
		os.Exit(1)
	}
	if notifier != nil {
		refresher.WithAnnouncer(newPushAnnouncer(notifier, logger))
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// The refresher populates the store in the background; the server starts
	// serving immediately (an empty page until the first refresh lands).
	go refresher.Run(ctx)

	// Pass the notifier only when enabled, so a nil *push.Notifier never becomes
	// a non-nil interface (which would falsely report push as available).
	var pushSvc web.PushService
	if notifier != nil {
		pushSvc = notifier
	}

	srv, err := web.NewServer(store, cfg.Refresh.Days, templatesFS, staticFS, pushSvc)
	if err != nil {
		logger.Error("build server", "error", err)
		os.Exit(1)
	}

	httpServer := &http.Server{Addr: cfg.Server.Addr(), Handler: srv.Handler()}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			logger.Error("graceful shutdown", "error", err)
		}
	}()

	logger.Info("server starting", "addr", cfg.Server.Addr(),
		"theaters", len(theaters), "days", cfg.Refresh.Days, "refresh", cfg.Refresh.Interval.String())
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("server failed", "error", err)
		os.Exit(1)
	}
	logger.Info("server stopped")
}

// setupPush builds a notifier from the push config, loading any persisted
// subscriptions. It returns (nil, nil) when push is disabled (no VAPID keys),
// which the caller treats as "PWA without notifications".
func setupPush(cfg config.Push, logger *slog.Logger) (*push.Notifier, error) {
	if !cfg.Enabled() {
		logger.Info("push notifications disabled (no VAPID keys configured)")
		return nil, nil
	}

	subs := push.NewSubscriptionStore(cfg.SubscriptionsFile)
	if err := subs.Load(); err != nil {
		return nil, fmt.Errorf("load subscriptions from %q: %w", cfg.SubscriptionsFile, err)
	}

	logger.Info("push notifications enabled",
		"subscriptions", subs.Len(), "file", cfg.SubscriptionsFile)
	return push.NewNotifier(subs, cfg.PublicKey, cfg.PrivateKey, cfg.Subject, logger), nil
}

// printVAPIDKeys generates a fresh VAPID key pair and prints it as env-var
// assignments, ready to drop into a secrets store or .env file.
func printVAPIDKeys() error {
	priv, pub, err := push.GenerateVAPIDKeys()
	if err != nil {
		return err
	}
	fmt.Printf("POPCORN_VAPID_PUBLIC_KEY=%s\nPOPCORN_VAPID_PRIVATE_KEY=%s\n", pub, priv)
	return nil
}

func envOr(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}

func newLogger(level string) *slog.Logger {
	var lvl slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: lvl}))
}
