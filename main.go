// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (c) 2024-2026 Julien Girard

// Command popcorn serves an aggregated cinema-showtimes calendar.
package main

import (
	"context"
	"embed"
	"flag"
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
	flag.Parse()

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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// The refresher populates the store in the background; the server starts
	// serving immediately (an empty page until the first refresh lands).
	go refresher.Run(ctx)

	srv, err := web.NewServer(store, cfg.Refresh.Days, templatesFS, staticFS)
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
