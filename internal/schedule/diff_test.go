// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (c) 2024-2026 Julien Girard

package schedule

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/bubu11e/popcorn/internal/allocine"
)

func day(titles ...string) []MovieView {
	views := make([]MovieView, len(titles))
	for i, t := range titles {
		views[i] = MovieView{Title: t}
	}
	return views
}

func titlesOf(movies []MovieView) []string {
	out := make([]string, len(movies))
	for i, m := range movies {
		out[i] = m.Title
	}
	return out
}

func TestNewMoviesReturnsOnlyAddedTitles(t *testing.T) {
	prev := [][]MovieView{day("A", "B"), day("A")}
	next := [][]MovieView{day("A", "B", "C"), day("C", "D")}

	got := titlesOf(NewMovies(prev, next))
	// C and D are new; C appears on two days but must be reported once.
	want := map[string]bool{"C": true, "D": true}
	if len(got) != 2 || !want[got[0]] || !want[got[1]] {
		t.Fatalf("NewMovies = %v, want exactly C and D", got)
	}
}

func TestNewMoviesOrdersByPopularity(t *testing.T) {
	prev := [][]MovieView{day("Old")}
	next := [][]MovieView{{
		{Title: "Niche", WantToSee: 5},
		{Title: "Blockbuster", WantToSee: 500},
	}}

	got := titlesOf(NewMovies(prev, next))
	if len(got) != 2 || got[0] != "Blockbuster" {
		t.Fatalf("NewMovies = %v, want most-wanted first (Blockbuster)", got)
	}
}

func TestNewMoviesEmptyWhenNothingAdded(t *testing.T) {
	prev := [][]MovieView{day("A", "B")}
	next := [][]MovieView{day("B", "A")}
	if got := NewMovies(prev, next); len(got) != 0 {
		t.Fatalf("NewMovies = %v, want empty when the catalogue is unchanged", got)
	}
}

// recordingAnnouncer captures the movies handed to it across refreshes.
type recordingAnnouncer struct {
	mu     sync.Mutex
	rounds [][]string
}

func (a *recordingAnnouncer) AnnounceNewMovies(_ context.Context, movies []MovieView) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.rounds = append(a.rounds, titlesOf(movies))
}

func TestRefresherSuppressesFirstSnapshotThenAnnounces(t *testing.T) {
	f := &fakeFetcher{fallback: []allocine.Showtime{sampleShowtime("First")}}
	store := NewStore()
	ann := &recordingAnnouncer{}
	r := NewRefresher(f, []allocine.Theater{{ID: "A"}}, 1, time.Hour, store, nil).WithAnnouncer(ann)

	// First refresh: cold start, nothing should be announced.
	r.Refresh(context.Background())

	// Second refresh now also surfaces "Second": only that one is announced.
	f.fallback = []allocine.Showtime{sampleShowtime("First"), sampleShowtime("Second")}
	r.Refresh(context.Background())

	ann.mu.Lock()
	defer ann.mu.Unlock()
	if len(ann.rounds) != 1 {
		t.Fatalf("announcer called %d times, want once (first snapshot suppressed)", len(ann.rounds))
	}
	if len(ann.rounds[0]) != 1 || ann.rounds[0][0] != "Second" {
		t.Fatalf("announced %v, want only [Second]", ann.rounds[0])
	}
}
