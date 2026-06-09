// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (c) 2024-2026 Julien Girard

package schedule

import (
	"reflect"
	"testing"
)

func TestCollectGenres(t *testing.T) {
	days := [][]MovieView{
		{
			{Title: "A", Genres: []string{"Comédie", "Romance"}},
			{Title: "B", Genres: []string{"Drame"}},
		},
		{
			{Title: "C", Genres: []string{"Romance", ""}}, // duplicate + blank
			{Title: "D", Genres: nil},
		},
	}

	got := CollectGenres(days)
	want := []string{"Comédie", "Drame", "Romance"} // sorted, deduped, no blanks
	if !reflect.DeepEqual(got, want) {
		t.Errorf("CollectGenres = %v, want %v", got, want)
	}
}

func TestCollectGenresEmpty(t *testing.T) {
	if got := CollectGenres(nil); len(got) != 0 {
		t.Errorf("want empty, got %v", got)
	}
}
