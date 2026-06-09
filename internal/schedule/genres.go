// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (c) 2024-2026 Julien Girard

package schedule

import "sort"

// CollectGenres returns the sorted, de-duplicated union of every genre present
// across all days. It is the catalogue that drives the genre filter chips, so
// the UI only ever offers genres that actually have screenings.
func CollectGenres(days [][]MovieView) []string {
	seen := make(map[string]struct{})
	for _, day := range days {
		for _, movie := range day {
			for _, genre := range movie.Genres {
				if genre == "" {
					continue
				}
				seen[genre] = struct{}{}
			}
		}
	}

	genres := make([]string, 0, len(seen))
	for genre := range seen {
		genres = append(genres, genre)
	}
	sort.Strings(genres)
	return genres
}
