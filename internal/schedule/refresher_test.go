// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (c) 2024-2026 Julien Girard

package schedule

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bubu11e/popcorn/internal/allocine"
)

// fakeFetcher returns canned showtimes or an error, recording call count.
type fakeFetcher struct {
	calls    atomic.Int32
	err      error
	byDate   map[string][]allocine.Showtime
	fallback []allocine.Showtime
}

func (f *fakeFetcher) GetShowtimes(_ context.Context, theater allocine.Theater, date time.Time) ([]allocine.Showtime, error) {
	f.calls.Add(1)
	if f.err != nil {
		return nil, f.err
	}
	if f.byDate != nil {
		return f.byDate[date.Format("2006-01-02")], nil
	}
	return f.fallback, nil
}

func sampleShowtime(title string) allocine.Showtime {
	return allocine.Showtime{
		StartsAt: time.Date(2026, 5, 29, 14, 0, 0, 0, time.Local),
		Theater:  allocine.Theater{Name: "Ciné"},
		Movie:    allocine.Movie{Title: title, WantToSee: 1},
	}
}

func TestRefreshPopulatesStore(t *testing.T) {
	f := &fakeFetcher{fallback: []allocine.Showtime{sampleShowtime("Film")}}
	store := NewStore()
	r := NewRefresher(f, []allocine.Theater{{ID: "A"}}, 3, time.Hour, store, nil)

	r.Refresh(context.Background())

	if !store.Loaded() {
		t.Fatal("store should be loaded after a successful refresh")
	}
	for d := 0; d < 3; d++ {
		if len(store.Day(d)) != 1 {
			t.Errorf("day %d: want 1 movie, got %d", d, len(store.Day(d)))
		}
	}
	// 3 days x 1 theater.
	if f.calls.Load() != 3 {
		t.Errorf("want 3 fetches, got %d", f.calls.Load())
	}
}

func TestRefreshKeepsLastGoodOnTotalFailure(t *testing.T) {
	store := NewStore()
	good := &fakeFetcher{fallback: []allocine.Showtime{sampleShowtime("Film")}}
	r := NewRefresher(good, []allocine.Theater{{ID: "A"}}, 2, time.Hour, store, nil)
	r.Refresh(context.Background())

	// Now every fetch fails: the store must retain the previous snapshot.
	r.fetcher = &fakeFetcher{err: errors.New("allocine down")}
	r.Refresh(context.Background())

	if !store.Loaded() {
		t.Fatal("store should still report loaded")
	}
	if len(store.Day(0)) != 1 {
		t.Errorf("last-good data lost: day 0 has %d movies", len(store.Day(0)))
	}
}

func TestRefreshEmptyStoreOnFirstFailure(t *testing.T) {
	store := NewStore()
	r := NewRefresher(&fakeFetcher{err: errors.New("down")}, []allocine.Theater{{ID: "A"}}, 2, time.Hour, store, nil)
	r.Refresh(context.Background())

	if store.Loaded() {
		t.Error("store must not be marked loaded when the first refresh fails entirely")
	}
	if store.Day(0) != nil {
		t.Error("expected nil day data before any successful refresh")
	}
}

func TestRefreshUsesRollingWindowFromNow(t *testing.T) {
	f := &fakeFetcher{byDate: map[string][]allocine.Showtime{
		"2026-05-29": {sampleShowtime("Today")},
		"2026-05-30": {sampleShowtime("Tomorrow")},
	}}
	store := NewStore()
	r := NewRefresher(f, []allocine.Theater{{ID: "A"}}, 2, time.Hour, store, nil)
	r.now = func() time.Time { return time.Date(2026, 5, 29, 9, 0, 0, 0, time.Local) }

	r.Refresh(context.Background())

	if got := store.Day(0); len(got) != 1 || got[0].Title != "Today" {
		t.Errorf("day 0 = %+v, want Today", got)
	}
	if got := store.Day(1); len(got) != 1 || got[0].Title != "Tomorrow" {
		t.Errorf("day 1 = %+v, want Tomorrow", got)
	}
}

func TestRunStopsOnContextCancel(t *testing.T) {
	f := &fakeFetcher{fallback: []allocine.Showtime{sampleShowtime("Film")}}
	store := NewStore()
	r := NewRefresher(f, []allocine.Theater{{ID: "A"}}, 1, time.Hour, store, nil)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { r.Run(ctx); close(done) }()

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after context cancel")
	}
}
