// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (c) 2024-2026 Julien Girard

// Package schedule aggregates raw showtimes into the per-day view models the
// UI renders, and keeps them fresh via a background refresher.
package schedule

import (
	"fmt"
	"sort"

	"github.com/bubu11e/popcorn/internal/allocine"
)

// Slot is a single screening time and its booking link (empty if none).
type Slot struct {
	Heure string
	URL   string
}

// Seance is the list of screening times for one movie at one theater.
type Seance struct {
	Cinema   string
	Horaires []Slot
}

// MovieView is a single movie card as consumed by the template. Field names are
// kept French to match the template bindings.
type MovieView struct {
	Title     string
	VOTitle   string // original/international title, set only when it differs from Title
	Director  string
	Casting   string
	Genres    []string
	Duree     string
	Synopsis  string
	Affiche   string
	URL       string
	Trailer   string
	WantToSee int
	Seances   []Seance
}

// Aggregate groups showtimes by movie (then by theater) and returns the cards
// sorted by popularity (wantToSee) descending, mirroring the Python behaviour.
func Aggregate(showtimes []allocine.Showtime) []MovieView {
	type acc struct {
		view         *MovieView
		theaterIndex map[string]int // theater name -> index into view.Seances
	}

	byMovie := make(map[string]*acc)
	order := make([]string, 0) // preserve first-seen order before sorting

	for _, st := range showtimes {
		a, ok := byMovie[st.Movie.Title]
		if !ok {
			a = &acc{
				view: &MovieView{
					Title:     st.Movie.Title,
					VOTitle:   originalTitle(st.Movie.Title, st.Movie.OriginalTitle),
					Director:  st.Movie.Director,
					Casting:   join(st.Movie.Cast),
					Genres:    st.Movie.Genres,
					Duree:     st.Movie.Runtime,
					Synopsis:  st.Movie.Synopsis,
					Affiche:   st.Movie.Poster,
					URL:       fmt.Sprintf("https://www.allocine.fr/film/fichefilm_gen_cfilm=%d.html", st.Movie.ID),
					Trailer:   trailerSearchURL(st.Movie.Title, st.Movie.OriginalTitle),
					WantToSee: st.Movie.WantToSee,
				},
				theaterIndex: make(map[string]int),
			}
			byMovie[st.Movie.Title] = a
			order = append(order, st.Movie.Title)
		}

		idx, ok := a.theaterIndex[st.Theater.Name]
		if !ok {
			idx = len(a.view.Seances)
			a.view.Seances = append(a.view.Seances, Seance{Cinema: st.Theater.Name})
			a.theaterIndex[st.Theater.Name] = idx
		}
		a.view.Seances[idx].Horaires = append(a.view.Seances[idx].Horaires, Slot{
			Heure: st.StartsAt.Format("15:04"),
			URL:   st.Ticketing,
		})
	}

	views := make([]MovieView, 0, len(order))
	for _, title := range order {
		v := byMovie[title].view
		for i := range v.Seances {
			sort.Slice(v.Seances[i].Horaires, func(a, b int) bool {
				return v.Seances[i].Horaires[a].Heure < v.Seances[i].Horaires[b].Heure
			})
		}
		views = append(views, *v)
	}

	sort.SliceStable(views, func(i, j int) bool {
		return views[i].WantToSee > views[j].WantToSee
	})
	return views
}

func join(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += ", "
		}
		out += p
	}
	return out
}
