// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (c) 2024-2026 Julien Girard

package schedule

import (
	"context"
	"log/slog"
	"time"

	"github.com/bubu11e/popcorn/internal/allocine"
)

// Fetcher retrieves showtimes for a theater on a given day. *allocine.Client
// satisfies it; tests substitute a fake.
type Fetcher interface {
	GetShowtimes(ctx context.Context, theater allocine.Theater, date time.Time) ([]allocine.Showtime, error)
}

// Refresher periodically rebuilds the store's snapshot. It recomputes the
// rolling date window from the current time on every cycle, so the "today"
// index never drifts regardless of how long the process runs.
type Refresher struct {
	fetcher  Fetcher
	theaters []allocine.Theater
	days     int
	interval time.Duration
	store    *Store
	logger   *slog.Logger
	now      func() time.Time // injectable clock for tests
}

// NewRefresher wires a Refresher.
func NewRefresher(f Fetcher, theaters []allocine.Theater, days int, interval time.Duration, store *Store, logger *slog.Logger) *Refresher {
	if logger == nil {
		logger = slog.Default()
	}
	return &Refresher{
		fetcher:  f,
		theaters: theaters,
		days:     days,
		interval: interval,
		store:    store,
		logger:   logger,
		now:      time.Now,
	}
}

// Run performs an initial refresh, then refreshes on every tick until the
// context is cancelled. It never returns an error: failures are logged and the
// last good snapshot is retained.
func (r *Refresher) Run(ctx context.Context) {
	r.Refresh(ctx)

	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			r.logger.Info("refresher stopping")
			return
		case <-ticker.C:
			r.Refresh(ctx)
		}
	}
}

// Refresh fetches the full window once and swaps it into the store. If every
// fetch fails, the store is left untouched so the previous data keeps serving.
func (r *Refresher) Refresh(ctx context.Context) {
	start := r.now()
	today := r.now()

	days := make([][]MovieView, r.days)
	var okCount, errCount, total int

	for offset := 0; offset < r.days; offset++ {
		date := today.AddDate(0, 0, offset)
		var showtimes []allocine.Showtime

		for _, theater := range r.theaters {
			st, err := r.fetcher.GetShowtimes(ctx, theater, date)
			if err != nil {
				errCount++
				r.logger.Warn("showtime fetch failed",
					"theater", theater.ID, "date", date.Format("2006-01-02"), "error", err)
				continue
			}
			okCount++
			showtimes = append(showtimes, st...)
		}

		views := Aggregate(showtimes)
		total += len(views)
		days[offset] = views
	}

	if okCount == 0 && errCount > 0 {
		r.logger.Error("refresh failed entirely; keeping last good snapshot",
			"errors", errCount)
		return
	}

	r.store.Replace(days)
	r.logger.Info("refresh complete",
		"movies", total, "ok", okCount, "errors", errCount,
		"duration", r.now().Sub(start).Round(time.Millisecond).String())
}
