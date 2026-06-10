// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (c) 2024-2026 Julien Girard

package main

import (
	"context"
	"log/slog"

	"github.com/bubu11e/popcorn/internal/push"
	"github.com/bubu11e/popcorn/internal/schedule"
)

// pushAnnouncer bridges the schedule package's new-movie hook to the push
// notifier: it turns the freshly-arrived movies into a digest notification. It
// satisfies schedule.Announcer.
type pushAnnouncer struct {
	notifier *push.Notifier
	logger   *slog.Logger
}

func newPushAnnouncer(n *push.Notifier, logger *slog.Logger) *pushAnnouncer {
	return &pushAnnouncer{notifier: n, logger: logger}
}

// AnnounceNewMovies sends one digest notification listing the new movie titles.
func (a *pushAnnouncer) AnnounceNewMovies(ctx context.Context, movies []schedule.MovieView) {
	titles := make([]string, len(movies))
	for i, m := range movies {
		titles[i] = m.Title
	}
	a.notifier.Notify(ctx, push.DigestPayload(titles).Marshal())
}
