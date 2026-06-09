// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (c) 2024-2026 Julien Girard

package schedule

import (
	"testing"
	"time"

	"github.com/bubu11e/popcorn/internal/allocine"
)

func st(title string, want int, theater string, hour int) allocine.Showtime {
	return allocine.Showtime{
		StartsAt: time.Date(2026, 5, 29, hour, 0, 0, 0, time.Local),
		Theater:  allocine.Theater{Name: theater},
		Movie:    allocine.Movie{Title: title, WantToSee: want, Cast: []string{"A", "B"}, Genres: []string{"Comédie"}},
	}
}

func TestAggregateSortsByWantToSee(t *testing.T) {
	views := Aggregate([]allocine.Showtime{
		st("Low", 10, "Ciné 1", 14),
		st("High", 99, "Ciné 1", 16),
	})
	if len(views) != 2 {
		t.Fatalf("want 2 movies, got %d", len(views))
	}
	if views[0].Title != "High" {
		t.Errorf("expected most-wanted first, got %q", views[0].Title)
	}
}

func TestAggregateGroupsByTheaterAndSortsTimes(t *testing.T) {
	views := Aggregate([]allocine.Showtime{
		st("Movie", 5, "Ciné A", 20),
		st("Movie", 5, "Ciné A", 14),
		st("Movie", 5, "Ciné B", 18),
	})
	if len(views) != 1 {
		t.Fatalf("want 1 movie, got %d", len(views))
	}
	v := views[0]
	if len(v.Seances) != 2 {
		t.Fatalf("want 2 theaters, got %d", len(v.Seances))
	}
	// First theater (Ciné A) times sorted ascending.
	if got := v.Seances[0].Horaires; got[0] != "14:00" || got[1] != "20:00" {
		t.Errorf("horaires not sorted: %v", got)
	}
	// Casting and genres are flattened to comma-joined strings.
	if v.Casting != "A, B" {
		t.Errorf("casting = %q", v.Casting)
	}
}

func TestAggregateBuildsMovieURL(t *testing.T) {
	views := Aggregate([]allocine.Showtime{{
		StartsAt: time.Now(),
		Theater:  allocine.Theater{Name: "C"},
		Movie:    allocine.Movie{ID: 42, Title: "X"},
	}})
	want := "https://www.allocine.fr/film/fichefilm_gen_cfilm=42.html"
	if views[0].URL != want {
		t.Errorf("URL = %q, want %q", views[0].URL, want)
	}
}

func TestAggregateEmpty(t *testing.T) {
	if got := Aggregate(nil); len(got) != 0 {
		t.Errorf("want empty, got %d", len(got))
	}
}
