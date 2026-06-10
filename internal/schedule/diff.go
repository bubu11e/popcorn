// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (c) 2024-2026 Julien Girard

package schedule

import (
	"context"
	"sort"
)

// Announcer is notified when movies enter the window that were not present in
// the previous snapshot. It is intentionally minimal so the schedule package
// stays free of any push/transport concerns: main wires the concrete delivery.
type Announcer interface {
	AnnounceNewMovies(ctx context.Context, movies []MovieView)
}

// NewMovies returns the movies present in next but absent from prev, identified
// by title, deduplicated across days and ordered by popularity (most wanted
// first). With an empty prev every movie is "new", so callers must suppress the
// very first snapshot themselves (see Refresher).
func NewMovies(prev, next [][]MovieView) []MovieView {
	known := titleSet(prev)

	added := make(map[string]bool)
	out := make([]MovieView, 0)
	for _, day := range next {
		for _, m := range day {
			if known[m.Title] || added[m.Title] {
				continue
			}
			added[m.Title] = true
			out = append(out, m)
		}
	}

	sort.SliceStable(out, func(i, j int) bool {
		return out[i].WantToSee > out[j].WantToSee
	})
	return out
}

// titleSet collects every movie title across a snapshot's days.
func titleSet(days [][]MovieView) map[string]bool {
	set := make(map[string]bool)
	for _, day := range days {
		for _, m := range day {
			set[m.Title] = true
		}
	}
	return set
}
