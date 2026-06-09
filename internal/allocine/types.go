// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (c) 2024-2026 Julien Girard

// Package allocine is a client for the unofficial allocine.fr showtimes API.
package allocine

import (
	"strings"
	"time"
)

const posterFallback = "/static/images/poster-placeholder.svg"

// Theater identifies a cinema by its allocine internal id and a display name.
type Theater struct {
	ID   string
	Name string
}

// Movie holds the subset of allocine movie fields the UI needs.
type Movie struct {
	ID            int
	Title         string
	OriginalTitle string
	Runtime       string
	Synopsis      string
	Genres        []string
	Cast          []string
	Director      string
	Poster        string
	WantToSee     int
}

// Showtime is a single screening of a movie at a theater.
type Showtime struct {
	StartsAt time.Time
	Theater  Theater
	Movie    Movie
}

// apiResponse mirrors the JSON returned by the showtimes endpoint. Only the
// fields we consume are declared; the rest are ignored by the decoder.
type apiResponse struct {
	Error      any         `json:"error"`
	Message    *string     `json:"message"`
	Results    []apiResult `json:"results"`
	Pagination struct {
		Page       string `json:"page"`
		TotalPages int    `json:"totalPages"`
	} `json:"pagination"`
}

type apiResult struct {
	Movie     apiMovie               `json:"movie"`
	Showtimes map[string][]apiSeance `json:"showtimes"`
}

// startsAtLayout matches allocine's timezone-less ISO timestamps, e.g.
// "2026-05-29T14:10:00". They are local (Europe/Paris) wall-clock times.
const startsAtLayout = "2006-01-02T15:04:05"

type apiSeance struct {
	StartsAt string `json:"startsAt"`
}

// toShowtime parses the screening time and binds it to its theater and movie.
func (s apiSeance) toShowtime(theater Theater, movie Movie) (Showtime, error) {
	startsAt, err := time.ParseInLocation(startsAtLayout, s.StartsAt, time.Local)
	if err != nil {
		return Showtime{}, err
	}
	return Showtime{StartsAt: startsAt, Theater: theater, Movie: movie}, nil
}

type apiMovie struct {
	InternalID    int    `json:"internalId"`
	Title         string `json:"title"`
	OriginalTitle string `json:"originalTitle"`
	Runtime       string `json:"runtime"`
	Synopsis      string `json:"synopsis"`
	Genres        []struct {
		Translate string `json:"translate"`
	} `json:"genres"`
	Stats struct {
		WantToSeeCount int `json:"wantToSeeCount"`
	} `json:"stats"`
	Poster *struct {
		URL string `json:"url"`
	} `json:"poster"`
	Credits []struct {
		Person *apiPerson `json:"person"`
	} `json:"credits"`
	Cast struct {
		Edges []struct {
			Node struct {
				Actor *apiPerson `json:"actor"`
			} `json:"node"`
		} `json:"edges"`
	} `json:"cast"`
}

type apiPerson struct {
	FirstName *string `json:"firstName"`
	LastName  *string `json:"lastName"`
}

// fullName joins first and last name, tolerating nil components.
func (p apiPerson) fullName() string {
	var first, last string
	if p.FirstName != nil {
		first = *p.FirstName
	}
	if p.LastName != nil {
		last = *p.LastName
	}
	return strings.TrimSpace(first + " " + last)
}

// toMovie converts the API payload into the domain Movie, applying nil-safe
// fallbacks for missing or malformed fields.
func (m apiMovie) toMovie() Movie {
	poster := posterFallback
	if m.Poster != nil && m.Poster.URL != "" {
		poster = m.Poster.URL
	}

	genres := make([]string, 0, len(m.Genres))
	for _, g := range m.Genres {
		genres = append(genres, g.Translate)
	}

	cast := make([]string, 0, len(m.Cast.Edges))
	for _, edge := range m.Cast.Edges {
		if edge.Node.Actor == nil {
			continue
		}
		cast = append(cast, edge.Node.Actor.fullName())
	}

	director := "Inconnu"
	if len(m.Credits) > 0 && m.Credits[0].Person != nil {
		director = m.Credits[0].Person.fullName()
	}

	return Movie{
		ID:            m.InternalID,
		Title:         m.Title,
		OriginalTitle: m.OriginalTitle,
		Runtime:       m.Runtime,
		Synopsis:      m.Synopsis,
		Genres:        genres,
		Cast:          cast,
		Director:      director,
		Poster:        poster,
		WantToSee:     m.Stats.WantToSeeCount,
	}
}
